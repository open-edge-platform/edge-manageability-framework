package main

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// =============================================================================
// Server configuration & request/response types
// =============================================================================

// ServerConfig holds the HTTP server settings (from CLI flags)
type ServerConfig struct {
	ListenAddr    string // e.g. ":8080"
	MPSHost       string // default MPS host
	DeviceGUID    string // default device GUID
	AMTUser       string // default AMT username
	AMTPort       int    // default AMT SOL port
	AllowedOrigin string // CORS origin
}

// ConnectRequest is the JSON body for POST /api/connect
type ConnectRequest struct {
	MPSHost    string `json:"mpsHost"`
	DeviceGUID string `json:"deviceGuid"`
	Port       int    `json:"port"`
	Mode       string `json:"mode"`
	JWTToken   string `json:"jwtToken"`
	AMTUser    string `json:"amtUser"`
	AMTPass    string `json:"amtPass"`
}

// StatusResponse is the JSON response for GET /api/status
type StatusResponse struct {
	State   string   `json:"state"`   // disconnected, connecting, authenticating, active, error
	Logs    []string `json:"logs"`    // recent protocol log messages
	Device  string   `json:"device"`  // connected device GUID
	MPSHost string   `json:"mpsHost"` // connected MPS host
}

// RedirectTokenResponse represents the MPS API response for redirect token
type RedirectTokenResponse struct {
	Token string `json:"token"`
}

// =============================================================================
// SOLServer — manages session lifecycle & exposes HTTP/WS endpoints
// =============================================================================

type SOLServer struct {
	config   ServerConfig
	session  *SOLSession
	mu       sync.RWMutex
	logs     []string
	logsMu   sync.Mutex
	upgrader websocket.Upgrader
}

// =============================================================================
// SOLSession — AMT SOL protocol state machine + browser relay
// =============================================================================

type SOLSession struct {
	// AMT/MPS side
	mpsConn     *websocket.Conn
	mu          sync.Mutex
	amtSequence uint32
	user        string
	pass        string
	authURI     string
	deviceGUID  string
	mpsHost     string

	// Browser side — terminal WebSocket
	browserConn *websocket.Conn
	browserMu   sync.Mutex

	// State tracking
	state   string // disconnected, connecting, authenticating, active, error
	stateMu sync.RWMutex

	// Log callback
	logFn func(string)

	// Done channel to signal session teardown
	done chan struct{}
}

// =============================================================================
// Binary helpers
// =============================================================================

func intToLE(v uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	return b
}

func shortToLE(v uint16) []byte {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, v)
	return b
}

func hexMD5(str string) string {
	h := md5.Sum([]byte(str))
	return hex.EncodeToString(h[:])
}

func generateRandomNonce(byteLen int) string {
	b := make([]byte, byteLen)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// =============================================================================
// SOLSession methods
// =============================================================================

func (s *SOLSession) log(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Print(msg)
	if s.logFn != nil {
		s.logFn(msg)
	}
}

func (s *SOLSession) setState(state string) {
	s.stateMu.Lock()
	s.state = state
	s.stateMu.Unlock()
}

func (s *SOLSession) getState() string {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.state
}

func (s *SOLSession) nextSequence() uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	seq := s.amtSequence
	s.amtSequence++
	return seq
}

// sendToMPS sends a binary message to the MPS WebSocket (AMT device side)
func (s *SOLSession) sendToMPS(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mpsConn.WriteMessage(websocket.BinaryMessage, data)
}

// sendToBrowser sends terminal text to the connected browser WebSocket
func (s *SOLSession) sendToBrowser(text string) {
	s.browserMu.Lock()
	defer s.browserMu.Unlock()
	if s.browserConn != nil {
		if err := s.browserConn.WriteMessage(websocket.TextMessage, []byte(text)); err != nil {
			s.log("[WARN] Failed to send to browser: %v", err)
		}
	}
}

// sendSOLData wraps terminal data in an AMT SOL data frame (0x28) and sends it
func (s *SOLSession) sendSOLData(data string) error {
	seq := s.nextSequence()
	frame := []byte{0x28, 0x00, 0x00, 0x00}
	frame = append(frame, intToLE(seq)...)
	frame = append(frame, shortToLE(uint16(len(data)))...)
	frame = append(frame, []byte(data)...)
	return s.sendToMPS(frame)
}

// sendDigestAuthInitial sends the initial digest auth request (method 4)
func (s *SOLSession) sendDigestAuthInitial() error {
	user := s.user
	uri := s.authURI
	dataLen := uint32(len(user) + len(uri) + 8)
	msg := []byte{0x13, 0x00, 0x00, 0x00, 0x04}
	msg = append(msg, intToLE(dataLen)...)
	msg = append(msg, byte(len(user)))
	msg = append(msg, []byte(user)...)
	msg = append(msg, 0x00, 0x00)
	msg = append(msg, byte(len(uri)))
	msg = append(msg, []byte(uri)...)
	msg = append(msg, 0x00, 0x00, 0x00, 0x00)
	s.log("[AUTH] Sending Digest Auth initial (user=%q)", user)
	return s.sendToMPS(msg)
}

// sendDigestAuthResponse computes and sends the digest auth response (RFC 2617)
func (s *SOLSession) sendDigestAuthResponse(realm, nonce, qop string) error {
	user := s.user
	pass := s.pass
	uri := s.authURI
	cnonce := generateRandomNonce(16)
	snc := "00000002"
	ha1 := hexMD5(user + ":" + realm + ":" + pass)
	ha2 := hexMD5("POST:" + uri)
	// AMT digest format: space before last colon (non-standard)
	responseStr := ha1 + ":" + nonce + ":" + snc + ":" + cnonce + ":" + qop + " :" + ha2
	digest := hexMD5(responseStr)
	s.log("[AUTH] Digest computed (HA1=%s)", ha1[:8]+"...")

	totalLen := len(user) + len(realm) + len(nonce) + len(uri) +
		len(cnonce) + len(snc) + len(digest) + len(qop) + 8
	msg := []byte{0x13, 0x00, 0x00, 0x00, 0x04}
	msg = append(msg, intToLE(uint32(totalLen))...)
	msg = append(msg, byte(len(user)))
	msg = append(msg, []byte(user)...)
	msg = append(msg, byte(len(realm)))
	msg = append(msg, []byte(realm)...)
	msg = append(msg, byte(len(nonce)))
	msg = append(msg, []byte(nonce)...)
	msg = append(msg, byte(len(uri)))
	msg = append(msg, []byte(uri)...)
	msg = append(msg, byte(len(cnonce)))
	msg = append(msg, []byte(cnonce)...)
	msg = append(msg, byte(len(snc)))
	msg = append(msg, []byte(snc)...)
	msg = append(msg, byte(len(digest)))
	msg = append(msg, []byte(digest)...)
	msg = append(msg, byte(len(qop)))
	msg = append(msg, []byte(qop)...)
	s.log("[AUTH] Sending Digest Auth response (%d bytes)", len(msg))
	return s.sendToMPS(msg)
}

// sendSOLSettings sends the SOL configuration message (0x20) to the AMT device
func (s *SOLSession) sendSOLSettings() {
	seq := s.nextSequence()
	msg := []byte{0x20, 0x00, 0x00, 0x00}
	msg = append(msg, intToLE(seq)...)
	msg = append(msg, shortToLE(10000)...) // MaxTxBuffer
	msg = append(msg, shortToLE(100)...)   // TxTimeout
	msg = append(msg, shortToLE(0)...)     // TxOverflowTimeout
	msg = append(msg, shortToLE(10000)...) // RxTimeout
	msg = append(msg, shortToLE(100)...)   // RxFlushTimeout
	msg = append(msg, shortToLE(5000)...)  // Heartbeat every 5 seconds
	msg = append(msg, 0x00, 0x00, 0x00, 0x00)

	s.log("[*] Sending SOL settings...")
	if err := s.sendToMPS(msg); err != nil {
		s.log("[ERROR] Failed to send SOL settings: %v", err)
	}
}

// =============================================================================
// AMT protocol frame handler — processes a single frame, returns bytes consumed
// =============================================================================

func (s *SOLSession) handleMPSFrame(data []byte) int {
	if len(data) == 0 {
		return 0
	}

	cmd := data[0]

	switch cmd {
	case 0x11: // StartRedirectionSessionReply
		if len(data) < 13 {
			return len(data)
		}
		status := data[1]
		s.log("[PROTOCOL] StartRedirectionSessionReply: status=%d", status)
		if status != 0 {
			s.log("[ERROR] Session start failed with status %d", status)
			s.setState("error")
			return len(data)
		}
		oemLen := int(data[12])
		frameSize := 13 + oemLen
		if frameSize > len(data) {
			frameSize = len(data)
		}

		// Send authentication query
		authQuery := []byte{0x13, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
		s.log("[*] Querying authentication methods...")
		if err := s.sendToMPS(authQuery); err != nil {
			s.log("[ERROR] Failed to send auth query: %v", err)
		}
		return frameSize

	case 0x14: // AuthenticateSessionReply
		if len(data) < 9 {
			return len(data)
		}
		status := data[1]
		authType := data[4]
		authDataLen := int(binary.LittleEndian.Uint32(data[5:9]))
		frameSize := 9 + authDataLen
		if frameSize > len(data) {
			frameSize = len(data)
		}
		s.log("[AUTH] Reply: status=%d authType=%d dataLen=%d", status, authType, authDataLen)

		if status == 0 && authType == 0 {
			var authMethods []byte
			if len(data) >= 9+authDataLen {
				authMethods = data[9 : 9+authDataLen]
			}
			hasDigest := false
			for _, m := range authMethods {
				if m == 4 {
					hasDigest = true
					break
				}
			}
			if hasDigest {
				s.log("[AUTH] Digest Auth (method 4) required...")
				s.sendDigestAuthInitial()
			} else {
				s.sendSOLSettings()
			}
		} else if status == 0 {
			s.log("[OK] Authentication successful!")
			s.sendSOLSettings()
		} else if status == 1 && (authType == 3 || authType == 4) {
			if len(data) < 9+authDataLen {
				return frameSize
			}
			authData := data[9 : 9+authDataLen]
			curPtr := 0
			realmLen := int(authData[curPtr])
			curPtr++
			realm := string(authData[curPtr : curPtr+realmLen])
			curPtr += realmLen
			nonceLen := int(authData[curPtr])
			curPtr++
			nonce := string(authData[curPtr : curPtr+nonceLen])
			curPtr += nonceLen
			qop := ""
			if authType == 4 && curPtr < len(authData) {
				qopLen := int(authData[curPtr])
				curPtr++
				if curPtr+qopLen <= len(authData) {
					qop = string(authData[curPtr : curPtr+qopLen])
				}
			}
			s.log("[AUTH] Challenge: realm=%q nonce=%q qop=%q", realm, nonce, qop)
			s.sendDigestAuthResponse(realm, nonce, qop)
		} else {
			s.log("[ERROR] Authentication failed: status=%d authType=%d", status, authType)
			s.setState("error")
		}
		return frameSize

	case 0x21: // SOL Settings Response (24 bytes)
		frameSize := 24
		if frameSize > len(data) {
			frameSize = len(data)
		}
		s.log("[PROTOCOL] SOL Settings Response received")
		seq := s.nextSequence()
		finalizeMsg := []byte{0x27, 0x00, 0x00, 0x00}
		finalizeMsg = append(finalizeMsg, intToLE(seq)...)
		finalizeMsg = append(finalizeMsg, 0x00, 0x00, 0x1B, 0x00, 0x00, 0x00)
		s.sendToMPS(finalizeMsg)

		s.setState("active")
		s.log("========================================")
		s.log("  SOL SESSION ACTIVE")
		s.log("========================================")
		return frameSize

	case 0x29: // Serial Settings (10 bytes)
		frameSize := 10
		if frameSize > len(data) {
			frameSize = len(data)
		}
		return frameSize

	case 0x2A: // Incoming display data (terminal output from AMT)
		if len(data) < 10 {
			return len(data)
		}
		dataLen := int(data[8]) | int(data[9])<<8
		frameSize := 10 + dataLen
		if frameSize > len(data) {
			dataLen = len(data) - 10
			frameSize = len(data)
		}
		termData := string(data[10 : 10+dataLen])
		s.sendToBrowser(termData)
		return frameSize

	case 0x2B: // Keep alive (8 bytes)
		frameSize := 8
		if frameSize > len(data) {
			frameSize = len(data)
		}
		if len(data) >= 8 {
			pong := []byte{0x2B, 0x00, 0x00, 0x00}
			pong = append(pong, data[4:8]...)
			if err := s.sendToMPS(pong); err != nil {
				s.log("[WARN] Failed to send keepalive pong: %v", err)
			}
		}
		return frameSize

	default:
		if cmd == 0x00 {
			return 1
		}
		s.log("[PROTOCOL] Unknown command 0x%02X (%d bytes)", cmd, len(data))
		return len(data)
	}
}

// =============================================================================
// MPS reader goroutine — reads from MPS and dispatches to state machine
// =============================================================================

func (s *SOLSession) readFromMPS() {
	defer func() {
		select {
		case <-s.done:
		default:
			close(s.done)
		}
	}()

	for {
		_, message, err := s.mpsConn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				s.log("[ERROR] MPS read error: %v", err)
			}
			return
		}
		s.mpsConn.SetReadDeadline(time.Now().Add(60 * time.Second))

		if len(message) > 0 {
			offset := 0
			for offset < len(message) {
				consumed := s.handleMPSFrame(message[offset:])
				if consumed <= 0 {
					break
				}
				offset += consumed
			}
		}
	}
}

// =============================================================================
// Browser reader goroutine — reads keystrokes from browser WebSocket
// =============================================================================

func (s *SOLSession) readFromBrowser(conn *websocket.Conn) {
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				s.log("[WARN] Browser read error: %v", err)
			}
			s.browserMu.Lock()
			if s.browserConn == conn {
				s.browserConn = nil
			}
			s.browserMu.Unlock()
			return
		}
		if s.getState() == "active" {
			// Convert LF to CR for SOL compatibility (websocat sends \n, AMT expects \r)
			text := strings.ReplaceAll(string(message), "\n", "\r")
			if err := s.sendSOLData(text); err != nil {
				s.log("[ERROR] Failed to send SOL data: %v", err)
			}
		}
	}
}

// keepAlivePinger sends periodic WebSocket pings and SOL keepalive to MPS
func (s *SOLSession) keepAlivePinger() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.mu.Lock()
			err := s.mpsConn.WriteMessage(websocket.PingMessage, []byte("keepalive"))
			s.mu.Unlock()
			if err != nil {
				s.log("[WARN] MPS ping failed: %v", err)
				return
			}
			if s.getState() == "active" {
				seq := s.nextSequence()
				frame := []byte{0x28, 0x00, 0x00, 0x00}
				frame = append(frame, intToLE(seq)...)
				frame = append(frame, shortToLE(0)...)
				s.mu.Lock()
				err = s.mpsConn.WriteMessage(websocket.BinaryMessage, frame)
				s.mu.Unlock()
				if err != nil {
					s.log("[WARN] SOL keepalive frame failed: %v", err)
				}
			}
		}
	}
}

// Close tears down the SOL session
func (s *SOLSession) Close() {
	select {
	case <-s.done:
	default:
		close(s.done)
	}
	if s.mpsConn != nil {
		s.mpsConn.Close()
	}
	if s.browserConn != nil {
		s.browserConn.Close()
	}
	s.setState("disconnected")
}

// =============================================================================
// getMPSRedirectToken — retrieves WebSocket redirect token from MPS API
// =============================================================================

func getMPSRedirectToken(mpsHost, deviceGUID, keycloakToken string) (string, error) {
	url := fmt.Sprintf("https://%s/api/v1/authorize/redirection/%s", mpsHost, deviceGUID)

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.AddCookie(&http.Cookie{Name: "jwt", Value: keycloakToken})
	req.Header.Set("ActiveProjectID", "")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp RedirectTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}
	return tokenResp.Token, nil
}

// =============================================================================
// SOLServer: connectToMPS — creates MPS WebSocket and starts AMT protocol
// =============================================================================

func (srv *SOLServer) connectToMPS(req ConnectRequest) (*SOLSession, error) {
	srv.addLog("[*] Getting MPS redirect token...")
	redirectToken, err := getMPSRedirectToken(req.MPSHost, req.DeviceGUID, req.JWTToken)
	if err != nil {
		return nil, fmt.Errorf("redirect token failed: %w", err)
	}
	srv.addLog("[OK] Redirect token obtained")

	wsURL := fmt.Sprintf("wss://%s/relay/webrelay.ashx?p=2&host=%s&port=%d&tls=0&tls1only=0&mode=%s",
		req.MPSHost, req.DeviceGUID, req.Port, req.Mode)

	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	headers := http.Header{}
	headers.Set("Sec-WebSocket-Protocol", redirectToken)
	headers.Set("Cookie", fmt.Sprintf("jwt=%s", req.JWTToken))
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", req.JWTToken))

	srv.addLog(fmt.Sprintf("[*] Connecting to MPS: %s", wsURL))
	conn, resp, err := dialer.Dial(wsURL, headers)
	if err != nil {
		errMsg := fmt.Sprintf("WebSocket dial failed: %v", err)
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			errMsg += fmt.Sprintf(" (HTTP %d: %s)", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("%s", errMsg)
	}

	session := &SOLSession{
		mpsConn:    conn,
		user:       req.AMTUser,
		pass:       req.AMTPass,
		authURI:    "",
		deviceGUID: req.DeviceGUID,
		mpsHost:    req.MPSHost,
		state:      "authenticating",
		done:       make(chan struct{}),
		logFn:      srv.addLog,
	}

	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(appData string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Send StartRedirectionSession (SOL)
	solStartCmd := []byte{0x10, 0x00, 0x00, 0x00, 0x53, 0x4F, 0x4C, 0x20}
	srv.addLog("[*] Sending StartRedirectionSession (SOL)...")
	if err := session.sendToMPS(solStartCmd); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to send SOL start: %w", err)
	}

	go session.readFromMPS()
	go session.keepAlivePinger()

	return session, nil
}

// =============================================================================
// HTTP Handlers
// =============================================================================

func (srv *SOLServer) addLog(msg string) {
	srv.logsMu.Lock()
	defer srv.logsMu.Unlock()
	srv.logs = append(srv.logs, msg)
	if len(srv.logs) > 500 {
		srv.logs = srv.logs[len(srv.logs)-500:]
	}
}

func (srv *SOLServer) getLogs() []string {
	srv.logsMu.Lock()
	defer srv.logsMu.Unlock()
	result := make([]string, len(srv.logs))
	copy(result, srv.logs)
	return result
}

func (srv *SOLServer) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = srv.config.AllowedOrigin
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// handleConnect — POST /api/connect
func (srv *SOLServer) handleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ConnectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Apply defaults
	if req.MPSHost == "" {
		req.MPSHost = srv.config.MPSHost
	}
	if req.DeviceGUID == "" {
		req.DeviceGUID = srv.config.DeviceGUID
	}
	if req.Port == 0 {
		req.Port = srv.config.AMTPort
	}
	if req.Mode == "" {
		req.Mode = "sol"
	}
	if req.AMTUser == "" {
		req.AMTUser = srv.config.AMTUser
	}

	if req.JWTToken == "" {
		http.Error(w, "jwtToken is required", http.StatusBadRequest)
		return
	}
	if req.AMTPass == "" {
		http.Error(w, "amtPass is required", http.StatusBadRequest)
		return
	}

	// Disconnect existing session
	srv.mu.Lock()
	if srv.session != nil {
		srv.session.Close()
		srv.session = nil
	}
	srv.mu.Unlock()

	session, err := srv.connectToMPS(req)
	if err != nil {
		srv.addLog("[ERROR] " + err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	srv.mu.Lock()
	srv.session = session
	srv.mu.Unlock()

	// Clean up when session ends
	go func() {
		<-session.done
		srv.mu.Lock()
		if srv.session == session {
			srv.session = nil
		}
		srv.mu.Unlock()
		srv.addLog("[*] Session ended")
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "connecting"})
}

// handleDisconnect — POST /api/disconnect
func (srv *SOLServer) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	srv.mu.Lock()
	if srv.session != nil {
		srv.session.Close()
		srv.session = nil
	}
	srv.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "disconnected"})
}

// handleStatus — GET /api/status
func (srv *SOLServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	srv.mu.RLock()
	session := srv.session
	srv.mu.RUnlock()

	resp := StatusResponse{
		State: "disconnected",
		Logs:  srv.getLogs(),
	}

	if session != nil {
		resp.State = session.getState()
		resp.Device = session.deviceGUID
		resp.MPSHost = session.mpsHost
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleTerminalWS — WS /ws/terminal
func (srv *SOLServer) handleTerminalWS(w http.ResponseWriter, r *http.Request) {
	conn, err := srv.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ERROR] WebSocket upgrade failed: %v", err)
		return
	}

	srv.mu.RLock()
	session := srv.session
	srv.mu.RUnlock()

	if session == nil || session.getState() == "disconnected" || session.getState() == "error" {
		conn.WriteMessage(websocket.TextMessage, []byte("[ERROR] No active SOL session. Call POST /api/connect first.\r\n"))
		conn.Close()
		return
	}

	// Attach browser connection to session
	session.browserMu.Lock()
	oldConn := session.browserConn
	session.browserConn = conn
	session.browserMu.Unlock()
	if oldConn != nil {
		oldConn.Close()
	}

	srv.addLog("[OK] Browser/terminal client connected via WebSocket")

	// Send CR to wake terminal prompt
	if session.getState() == "active" {
		if err := session.sendSOLData("\r"); err != nil {
			srv.addLog("[WARN] Failed to send wake CR: " + err.Error())
		}
	}

	// Browser ping/pong keepalive
	conn.SetReadDeadline(time.Now().Add(120 * time.Second))
	conn.SetPongHandler(func(appData string) error {
		conn.SetReadDeadline(time.Now().Add(120 * time.Second))
		return nil
	})

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-session.done:
				return
			case <-ticker.C:
				session.browserMu.Lock()
				currentConn := session.browserConn
				session.browserMu.Unlock()
				if currentConn != conn {
					return
				}
				if err := conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(5*time.Second)); err != nil {
					return
				}
			}
		}
	}()

	// Read keystrokes from browser/websocat and send to AMT
	session.readFromBrowser(conn)
	srv.addLog("[*] Browser/terminal client disconnected")
}

// =============================================================================
// main — HTTP server
// =============================================================================

func main() {
	config := ServerConfig{}
	flag.StringVar(&config.ListenAddr, "listen", ":8080", "HTTP server listen address")
	flag.StringVar(&config.MPSHost, "mps-host", "mps-wss.orch-10-139-218-35.pid.infra-host.com", "Default MPS host")
	flag.StringVar(&config.DeviceGUID, "guid", "89174ecf-31c3-22e3-5f8d-48210b509c73", "Default device GUID")
	flag.StringVar(&config.AMTUser, "amt-user", "admin", "Default AMT username")
	flag.IntVar(&config.AMTPort, "amt-port", 16994, "Default AMT SOL port")
	flag.StringVar(&config.AllowedOrigin, "cors-origin", "http://localhost:4200", "Allowed CORS origin")
	flag.Parse()

	srv := &SOLServer{
		config: config,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/connect", srv.handleConnect)
	mux.HandleFunc("/api/disconnect", srv.handleDisconnect)
	mux.HandleFunc("/api/status", srv.handleStatus)
	mux.HandleFunc("/ws/terminal", srv.handleTerminalWS)

	// Landing page
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<!DOCTYPE html><html><body style="background:#1a1b26;color:#a9b1d6;font-family:monospace;padding:40px">
<h1>AMT SOL Server (SOL-POC-NEW)</h1>
<p>API Endpoints:</p>
<ul>
<li><b>POST /api/connect</b> — Start SOL session<br>
    Body: {"jwtToken":"...", "amtPass":"...", "mpsHost":"...", "deviceGuid":"...", "amtUser":"admin", "port":16994}</li>
<li><b>POST /api/disconnect</b> — End SOL session</li>
<li><b>GET /api/status</b> — Get session status and logs</li>
<li><b>WS /ws/terminal</b> — WebSocket for interactive terminal I/O (use with websocat)</li>
</ul>
<h3>Quick Test with websocat:</h3>
<pre>
# 1. Start SOL session:
curl -s -X POST http://localhost:8080/api/connect \
  -H 'Content-Type: application/json' \
  -d '{"jwtToken":"YOUR_JWT","amtPass":"YOUR_AMT_PASS"}'

# 2. Connect interactive terminal:
websocat ws://localhost:8080/ws/terminal

# 3. Check status:
curl -s http://localhost:8080/api/status | python3 -m json.tool

# 4. Disconnect:
curl -s -X POST http://localhost:8080/api/disconnect
</pre>
</body></html>`)
			return
		}
		http.NotFound(w, r)
	})

	handler := srv.corsMiddleware(mux)

	log.Printf("========================================================================")
	log.Printf("  AMT SOL Server (SOL-POC-NEW)")
	log.Printf("========================================================================")
	log.Printf("  Listen:       %s", config.ListenAddr)
	log.Printf("  MPS Host:     %s", config.MPSHost)
	log.Printf("  Device GUID:  %s", config.DeviceGUID)
	log.Printf("  CORS Origin:  %s", config.AllowedOrigin)
	log.Printf("========================================================================")
	log.Printf("  Landing:  http://localhost%s/", config.ListenAddr)
	log.Printf("  Status:   http://localhost%s/api/status", config.ListenAddr)
	log.Printf("  Terminal: ws://localhost%s/ws/terminal", config.ListenAddr)
	log.Printf("========================================================================")

	if err := http.ListenAndServe(config.ListenAddr, handler); err != nil {
		log.Fatalf("[ERROR] Server failed: %v", err)
		os.Exit(1)
	}
}
