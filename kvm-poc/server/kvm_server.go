package main

import (
	"bytes"
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
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// =============================================================================
// CHANGE 1: Server-oriented config instead of one-shot CLI config
// The server listens on a port and accepts connections from the Angular frontend.
// MPS/AMT connection details come via the REST API, not CLI flags.
// =============================================================================

// ServerConfig holds the HTTP server settings (from CLI flags)
type ServerConfig struct {
	ListenAddr    string // e.g. ":8080"
	MPSHost       string // default MPS host
	DeviceGUID    string // default device GUID
	AMTPort       int    // default AMT KVM port
	StaticDir     string // path to Angular dist/ folder
	AllowedOrigin string // CORS origin (e.g. "http://localhost:4200")
}

// ConnectRequest is the JSON body for POST /api/connect
// The Angular frontend sends this to initiate a KVM session.
type ConnectRequest struct {
	MPSHost    string `json:"mpsHost"`
	DeviceGUID string `json:"deviceGuid"`
	Port       int    `json:"port"`
	Mode       string `json:"mode"`
	JWTToken   string `json:"jwtToken"`
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

// ConsentCodeRequest is the JSON body for POST /api/consent/code
type ConsentCodeRequest struct {
	ConsentCode string `json:"consentCode"`
}

// ConsentResponse represents user consent API response from MPS
type ConsentResponse struct {
	Body struct {
		ReturnValue int `json:"ReturnValue"`
	} `json:"Body"`
}

// =============================================================================
// CHANGE 2: KVMServer — central struct that manages the session lifecycle
// In the client version, main() did everything sequentially. Now the server
// manages the session state and exposes it via HTTP/WebSocket handlers.
// =============================================================================

// KVMServer manages the AMT KVM connection and provides HTTP/WS endpoints
type KVMServer struct {
	config    ServerConfig
	session   *KVMSession   // nil when disconnected
	mu        sync.RWMutex  // protects session
	logs      []string      // recent log messages
	logsMu    sync.Mutex
	upgrader  websocket.Upgrader // for browser WebSocket upgrade
}

// =============================================================================
// CHANGE 3: KVMSession — mostly preserved from client, but now includes:
//   - browserConn: WebSocket connection TO the Angular frontend
//   - state tracking via a string field
//   - log callback instead of direct log.Printf
// =============================================================================

// KVMSession manages AMT protocol handshake then RFB relay
// Server handles: AMT RedirectStart, Authentication, Channel Open
// Then relays pure RFB protocol between browser and MPS
type KVMSession struct {
	// MPS side
	mpsConn   *websocket.Conn
	mpsMu     sync.Mutex

	// Browser side
	browserConn          *websocket.Conn
	browserMu            sync.Mutex
	pendingBrowserFrames [][]byte

	// AMT Protocol state machine
	amtState       string // "start", "auth", "channel", "active"
	amtSequence    uint32
	amtAccumulator string
	user           string // AMT username (usually "admin")
	pass           string // AMT password (empty for CCM)
	authURI        string // "/RedirectionService" (fallback)
	deviceGUID     string // Device GUID (used as auth URI in DMT)

	// State tracking
	state   string
	stateMu sync.RWMutex

	// Logging
	logFn func(string)

	// Shutdown coordination
	done chan struct{}
}

// =============================================================================
// Binary helpers for AMT protocol
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

// decodeUTF8Binary converts a UTF-8 encoded byte slice back to raw binary.
// MPS/AMT WebSocket text frames carry binary data UTF-8 encoded:
// each byte value 0x00-0x7F is verbatim, 0x80-0xFF are 2-byte UTF-8 sequences.
// This reverses that encoding so the Go→Browser relay sends raw binary bytes.
func decodeUTF8Binary(src []byte) []byte {
	dst := make([]byte, 0, len(src))
	for i := 0; i < len(src); {
		b := src[i]
		if b < 0x80 {
			dst = append(dst, b)
			i++
		} else if b&0xE0 == 0xC0 && i+1 < len(src) && src[i+1]&0xC0 == 0x80 {
			// 2-byte UTF-8 sequence: decode to single byte 0x80-0xFF
			dst = append(dst, (b&0x1F)<<6|(src[i+1]&0x3F))
			i += 2
		} else {
			// Unexpected byte: pass through as-is
			dst = append(dst, b)
			i++
		}
	}
	return dst
}

// encodeUTF8Binary converts raw binary bytes to UTF-8 encoded bytes for MPS.
// This is the reverse of decodeUTF8Binary — bytes 0x80-0xFF become 2-byte sequences.
func encodeUTF8Binary(src []byte) []byte {
	dst := make([]byte, 0, len(src)+len(src)/4)
	for _, b := range src {
		if b < 0x80 {
			dst = append(dst, b)
		} else {
			dst = append(dst, 0xC0|(b>>6), 0x80|(b&0x3F))
		}
	}
	return dst
}

// =============================================================================
// KVMSession methods — AMT protocol handling
// =============================================================================

func (s *KVMSession) log(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Print(msg)
	if s.logFn != nil {
		s.logFn(msg)
	}
}

func (s *KVMSession) setState(state string) {
	s.stateMu.Lock()
	s.state = state
	s.stateMu.Unlock()
}

func (s *KVMSession) getState() string {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.state
}

// sendToMPS sends data to MPS as a UTF-8 text frame.
// MPS/AMT expects WebSocket TEXT frames with binary values UTF-8 encoded —
// bytes 0x80-0xFF are represented as 2-byte UTF-8 sequences (matching DMT's
// String.fromCharCode / socketSend behaviour).
func (s *KVMSession) sendToMPS(data []byte) error {
	s.mpsMu.Lock()
	defer s.mpsMu.Unlock()
	return s.mpsConn.WriteMessage(websocket.TextMessage, encodeUTF8Binary(data))
}

// sendToBrowser relays binary data to Angular frontend (pure relay)
func (s *KVMSession) sendToBrowser(data []byte) {
	s.browserMu.Lock()
	defer s.browserMu.Unlock()
	if s.browserConn == nil {
		copied := make([]byte, len(data))
		copy(copied, data)
		s.pendingBrowserFrames = append(s.pendingBrowserFrames, copied)
		s.log("[RFB] Queued %d bytes for browser (browser websocket not attached yet)", len(data))
		return
	}
	if s.browserConn != nil {
		if err := s.browserConn.WriteMessage(websocket.BinaryMessage, data); err != nil {
			s.log("[WARN] Failed to send to browser: %v", err)
		}
	}
}

func (s *KVMSession) flushPendingBrowserFrames() {
	s.browserMu.Lock()
	defer s.browserMu.Unlock()

	if s.browserConn == nil || len(s.pendingBrowserFrames) == 0 {
		return
	}

	s.log("[RFB] Flushing %d queued browser frame(s)", len(s.pendingBrowserFrames))
	for _, frame := range s.pendingBrowserFrames {
		if err := s.browserConn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
			s.log("[WARN] Failed to flush queued frame to browser: %v", err)
			break
		}
	}
	s.pendingBrowserFrames = nil
}

// AMT protocol then RFB relay: MPS → Browser
// Go server handles: AMT handshake (0x10→0x11→0x13→0x14→0x40→0x41)
// After 0x41: Relay pure RFB binary to browser
func (s *KVMSession) readFromMPS() {
	defer func() {
		s.browserMu.Lock()
		if s.browserConn != nil {
			_ = s.browserConn.Close()
			s.browserConn = nil
		}
		s.browserMu.Unlock()

		select {
		case <-s.done:
		default:
			close(s.done)
		}
	}()

	s.log("[*] MPS reader started - AMT protocol mode")
	for {
		msgType, message, err := s.mpsConn.ReadMessage()
		if err != nil {
			if closeErr, ok := err.(*websocket.CloseError); ok {
				s.log("[WARN] MPS websocket closed: code=%d text=%q state=%s amtState=%s", closeErr.Code, closeErr.Text, s.getState(), s.amtState)
			} else {
				s.log("[ERROR] MPS read error: %v (state=%s amtState=%s)", err, s.getState(), s.amtState)
			}
			s.log("[*] Closing browser websocket because MPS session ended")
			s.setState("error")
			return
		}
		// Reset read deadline after every successful message — prevents timeout when screen
		// is static and MPS does not respond to WebSocket ping frames.
		_ = s.mpsConn.SetReadDeadline(time.Now().Add(120 * time.Second))

		// MPS/AMT sends RFB data as WebSocket TEXT frames containing UTF-8 encoded
		// binary. Each byte value 0x80-0xFF is encoded as a 2-byte UTF-8 sequence
		// (e.g. 0xFF → 0xC3 0xBF). We must decode back to raw binary before
		// forwarding to the browser's ArrayBuffer-based RFB parser.
		if msgType == websocket.TextMessage {
			message = decodeUTF8Binary(message)
		}

		// Check AMT state
		currentState := s.getState()

		if currentState == "active" {
			// After ChannelOpen (0x41), relay all data as pure RFB binary
			hexLimit := len(message)
			if hexLimit > 24 {
				hexLimit = 24
			}
			s.log("[RFB→Browser] %d bytes, first %d: %x", len(message), hexLimit, message[:hexLimit])
			s.sendToBrowser(message)
		} else {
			// Before ChannelOpen: Parse AMT protocol messages
			s.handleMPSMessage(message)
		}
	}
}

// handleMPSMessage - AMT protocol state machine
func (s *KVMSession) handleMPSMessage(msg []byte) {
	if len(msg) == 0 {
		return
	}

	msgType := msg[0]

	switch msgType {
	case 0x11: // StartRedirectionSessionReply
		s.log("[AMT] StartRedirectionSessionReply received")
		if len(msg) < 4 {
			s.log("[AMT] ✘ Reply too short")
			s.setState("error")
			return
		}
		status := msg[1]
		if status == 0 {
			s.log("[AMT] ✔ Session started - status=0")
			s.amtState = "auth"
			// Query for available authentication methods
			s.sendAuthQuery()
		} else {
			s.log("[AMT] ✘ Session start failed - status=%d", status)
			s.setState("error")
		}

	case 0x14: // AuthenticateSessionReply
		s.log("[AMT] AuthenticateSessionReply received")
		if len(msg) < 9 {
			s.log("[AMT] ✘ Auth reply too short: %d bytes", len(msg))
			return
		}

		status := msg[1]
		authType := msg[4]
		authDataLen := binary.LittleEndian.Uint32(msg[5:9])
		s.log("[AMT] Auth status=%d, type=%d, dataLen=%d", status, authType, authDataLen)

		if len(msg) < 9+int(authDataLen) {
			s.log("[AMT] ✘ Auth data incomplete")
			return
		}

		if authType == 0 && status == 0 {
			// Query response - check available auth methods
			s.log("[AMT] Auth query response received")
			authData := msg[9 : 9+authDataLen]
			s.log("[AMT] Available auth methods: %v", authData)
			
			// Check if Digest Auth (type 4) is available
			hasDigest := false
			for _, method := range authData {
				if method == 4 {
					hasDigest = true
					break
				}
			}
			
			if hasDigest {
				s.log("[AMT] Digest Auth (method 4) available - sending initial request")
				s.sendDigestAuthInitial()
			} else {
				s.log("[AMT] ✘ Digest auth not available, trying anyway")
				s.sendDigestAuthInitial()
			}
		} else if (authType == 3 || authType == 4) && status == 1 {
			// Digest challenge
			s.log("[AMT] Digest challenge received - parsing...")
			s.handleAuthChallenge(msg, authType, authDataLen)
		} else if status == 0 && (authType == 3 || authType == 4) {
			// Auth success!
			s.log("[AMT] ✔✔✔ AMT Authentication SUCCESS! ✔✔✔")
			s.amtState = "channel"
			s.setState("authenticating")
			s.sendChannelOpen()
		} else if authType == 0 && status == 1 {
			// Auth query failed - try digest anyway (some AMT versions)
			s.log("[AMT] Auth query failed - trying digest auth anyway")
			s.sendDigestAuthInitial()
		} else {
			s.log("[AMT] ✘ Auth failed - status=%d, type=%d", status, authType)
			s.setState("error")
		}

	case 0x41: // ChannelOpenConfirmation
		if len(msg) < 8 {
			s.log("[AMT] ✘ Channel Open response too short: %d bytes", len(msg))
			s.setState("error")
			return
		}

		s.log("[AMT] ✔ Channel Open Response received (0x41)")
		s.log("[AMT] ✔✔✔ Channel opened - KVM ACTIVE! ✔✔✔")
		s.amtState = "active"
		s.setState("active")

		// The AMT ChannelOpenConfirmation frame is 8 bytes long.
		// Any trailing bytes after that header belong to the initial RFB stream.
		if len(msg) > 8 {
			rfbData := msg[8:]
			s.log("[RFB] Received %d bytes with channel open", len(rfbData))
			s.sendToBrowser(rfbData)
		}

	default:
		s.log("[AMT] Unknown message type: 0x%02x (%d bytes)", msgType, len(msg))
	}
}

// handleAuthChallenge - Parse Digest auth challenge (status=1)
// DMT format: [authData] = [realmLen][realm][nonceLen][nonce][qopLen][qop]
func (s *KVMSession) handleAuthChallenge(msg []byte, authType uint8, authDataLen uint32) {
	authDataBuf := msg[9 : 9+authDataLen]
	curptr := 0

	// Parse Realm
	if curptr >= len(authDataBuf) {
		s.log("[AMT] ✘ Challenge data too short for realm")
		s.setState("error")
		return
	}
	realmLen := int(authDataBuf[curptr])
	curptr++
	if curptr+realmLen > len(authDataBuf) {
		s.log("[AMT] ✘ Realm length exceeds data")
		s.setState("error")
		return
	}
	realm := string(authDataBuf[curptr : curptr+realmLen])
	curptr += realmLen

	// Parse Nonce
	if curptr >= len(authDataBuf) {
		s.log("[AMT] ✘ Challenge data too short for nonce")
		s.setState("error")
		return
	}
	nonceLen := int(authDataBuf[curptr])
	curptr++
	if curptr+nonceLen > len(authDataBuf) {
		s.log("[AMT] ✘ Nonce length exceeds data")
		s.setState("error")
		return
	}
	nonce := string(authDataBuf[curptr : curptr+nonceLen])
	curptr += nonceLen

	// Parse QOP (only for authType 4)
	qop := ""
	if authType == 4 {
		if curptr >= len(authDataBuf) {
			s.log("[AMT] ✘ Challenge data too short for qop")
			s.setState("error")
			return
		}
		qopLen := int(authDataBuf[curptr])
		curptr++
		if curptr+qopLen > len(authDataBuf) {
			s.log("[AMT] ✘ QOP length exceeds data")
			s.setState("error")
			return
		}
		qop = string(authDataBuf[curptr : curptr+qopLen])
	}

	s.log("[AMT] Challenge parsed - realm=%s, nonce=%s, qop=%s", realm, nonce, qop)
	s.sendDigestAuthResponse(authType, realm, nonce, qop)
}

// =============================================================================
// AMT Protocol Sending Methods
// =============================================================================

// sendRedirectStartKVM - Initial AMT handshake (0x10, protocol=2, "KVMR")
func (s *KVMSession) sendRedirectStartKVM() {
	// Message format: [0x10][0x01][0x00][0x00]['K']['V']['M']['R']
	redirectStart := []byte{0x10, 0x01, 0x00, 0x00, 'K', 'V', 'M', 'R'}
	s.sendToMPS(redirectStart)
	s.log("[AMT] → RedirectStartKVM sent (8 bytes)")
}

// sendAuthQuery - Query available authentication methods (0x13 with authType=0)
func (s *KVMSession) sendAuthQuery() {
	// DMT format: [0x13][0x00][0x00][0x00][AuthType=0][AuthDataLen=0 (4 bytes)]
	authQuery := []byte{0x13, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	s.sendToMPS(authQuery)
	s.log("[AMT] → AuthenticateSession Query sent")
}

// sendDigestAuthInitial - Send initial Digest auth request (authType=4)
// DMT format: user.length + authUri.length + 8 bytes overhead
func (s *KVMSession) sendDigestAuthInitial() {
	username := s.user
	if username == "" {
		username = "admin"
	}
	
	uri := s.authURI
	if uri == "" {
		uri = "/RedirectionService"
	}
	
	// Calculate total auth data length
	authDataLen := len(username) + len(uri) + 8
	
	var buf bytes.Buffer
	// Message header
	buf.WriteByte(0x13) // AuthenticateSession
	buf.WriteByte(0x00) // Reserved
	buf.WriteByte(0x00) // Reserved
	buf.WriteByte(0x00) // Reserved
	buf.WriteByte(0x04) // AuthType = 4 (Digest)
	
	// AuthDataLen (little-endian)
	buf.Write(intToLE(uint32(authDataLen)))
	
	// Username
	buf.WriteByte(byte(len(username)))
	buf.WriteString(username)
	
	// Two zero bytes
	buf.WriteByte(0x00)
	buf.WriteByte(0x00)
	
	// URI
	buf.WriteByte(byte(len(uri)))
	buf.WriteString(uri)
	
	// Four zero bytes
	buf.WriteByte(0x00)
	buf.WriteByte(0x00)
	buf.WriteByte(0x00)
	buf.WriteByte(0x00)
	
	message := buf.Bytes()
	s.sendToMPS(message)
	s.log("[AMT] → Digest Auth Initial sent (%d bytes)", len(message))
}

// sendDigestAuthResponse - Send digest auth response after challenge
// DMT format matches their exact structure with all fields
func (s *KVMSession) sendDigestAuthResponse(authType uint8, realm, nonce, qop string) {
	username := s.user
	if username == "" {
		username = "admin"
	}
	
	password := s.pass // Empty for CCM mode
	
	// DMT uses device GUID as URI, not "/RedirectionService"
	uri := s.deviceGUID
	if uri == "" {
		uri = s.authURI
		if uri == "" {
			uri = "/RedirectionService"
		}
	}
	
	// Generate cnonce and nc (match DMT format)
	cnonce := generateRandomNonce(16) // 16 bytes = 32 hex chars (DMT format)
	nc := "00000001" // DMT uses 00000001
	
	// Compute MD5 digest (DMT format)
	// digest = MD5(MD5(user:realm:pass) + ":" + nonce + ":" + nc + ":" + cnonce + ":" + qop + ":" + MD5("POST:" + uri))
	ha1 := hexMD5(username + ":" + realm + ":" + password)
	ha2 := hexMD5("POST:" + uri)
	
	var extra string
	if authType == 4 {
		extra = nc + ":" + cnonce + ":" + qop + ":"
	} else {
		extra = ""
	}
	
	digest := hexMD5(ha1 + ":" + nonce + ":" + extra + ha2)
	
	// Calculate total length
	totallen := len(username) + len(realm) + len(nonce) + len(uri) + len(cnonce) + len(nc) + len(digest) + 7
	if authType == 4 {
		totallen += len(qop) + 1
	}
	
	// Build message
	var buf bytes.Buffer
	buf.WriteByte(0x13) // AuthenticateSession
	buf.WriteByte(0x00) // Reserved
	buf.WriteByte(0x00) // Reserved
	buf.WriteByte(0x00) // Reserved
	buf.WriteByte(authType) // AuthType (3 or 4)
	
	// Total length (little-endian)
	buf.Write(intToLE(uint32(totallen)))
	
	// Username
	buf.WriteByte(byte(len(username)))
	buf.WriteString(username)
	
	// Realm
	buf.WriteByte(byte(len(realm)))
	buf.WriteString(realm)
	
	// Nonce
	buf.WriteByte(byte(len(nonce)))
	buf.WriteString(nonce)
	
	// URI
	buf.WriteByte(byte(len(uri)))
	buf.WriteString(uri)
	
	// CNonce
	buf.WriteByte(byte(len(cnonce)))
	buf.WriteString(cnonce)
	
	// NC
	buf.WriteByte(byte(len(nc)))
	buf.WriteString(nc)
	
	// Digest
	buf.WriteByte(byte(len(digest)))
	buf.WriteString(digest)
	
	// QOP (only for authType 4)
	if authType == 4 {
		buf.WriteByte(byte(len(qop)))
		buf.WriteString(qop)
	}
	
	message := buf.Bytes()
	if err := s.sendToMPS(message); err != nil {
		s.log("[AMT] ✘ Failed to send digest response: %v", err)
		s.setState("error")
		return
	}
	s.log("[AMT] → Digest Auth Response sent (%d bytes)", len(message))
}

// sendChannelOpen - Open KVM channel (0x40)
func (s *KVMSession) sendChannelOpen() {
	// Message: [0x40][0x00][0x00][0x00][0x00][0x00][0x00][0x00]
	channelOpen := []byte{0x40, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	s.sendToMPS(channelOpen)
	s.log("[AMT] → Channel Open sent (0x40)")
}

// Pure binary relay: Browser → MPS  
// Browser sends AMT protocol + RFB (handshake, mouse/keyboard events)
func (s *KVMSession) readFromBrowser() {
	s.log("[*] Browser reader started")
	defer s.log("[*] Browser reader stopped")
	
	for {
		_, message, err := s.browserConn.ReadMessage()
		if err != nil {
			if closeErr, ok := err.(*websocket.CloseError); ok {
				s.log("[WARN] Browser websocket closed: code=%d text=%q state=%s amtState=%s", closeErr.Code, closeErr.Text, s.getState(), s.amtState)
			}
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				s.log("[WARN] Browser read error: %v (state=%s amtState=%s)", err, s.getState(), s.amtState)
			}
			// Clear browser connection, keep MPS session alive
			s.browserMu.Lock()
			s.browserConn = nil
			s.browserMu.Unlock()
			return
		}
		
		hexLimitB := len(message)
		if hexLimitB > 16 {
			hexLimitB = 16
		}
		s.log("[RFB] Browser→MPS %d bytes, hex: %x", len(message), message[:hexLimitB])

		// Pure binary relay - forward directly to MPS
		if err := s.sendToMPS(message); err != nil {
			s.log("[ERROR] Failed to forward to MPS: %v", err)
		}
	}
}

// keepAlivePinger sends periodic WebSocket pings to keep MPS connection alive
func (s *KVMSession) keepAlivePinger() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.mpsMu.Lock()
			err := s.mpsConn.WriteMessage(websocket.PingMessage, []byte("keepalive"))
			s.mpsMu.Unlock()
			if err != nil {
				s.log("[WARN] MPS ping failed: %v", err)
				return
			}
		}
	}
}

// Close tears down the KVM session
func (s *KVMSession) Close() {
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
// CHANGE 8: getMPSRedirectToken — UNCHANGED (just a standalone function)
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

// connectToMPS creates WebSocket to MPS and initiates AMT protocol
// Go server handles: AMT RedirectStart, Digest Auth, Channel Open (0x10→0x11→0x13→0x14→0x40→0x41)
// After 0x41: Relay pure RFB binary between browser and MPS
func (srv *KVMServer) connectToMPS(req ConnectRequest) (*KVMSession, error) {
	// Get redirect token
	srv.addLog("[*] Getting MPS redirect token...")
	redirectToken, err := getMPSRedirectToken(req.MPSHost, req.DeviceGUID, req.JWTToken)
	if err != nil {
		return nil, fmt.Errorf("redirect token failed: %w", err)
	}
	srv.addLog("[OK] Redirect token obtained")

	// Build WebSocket URL for KVM mode
	wsURL := fmt.Sprintf("wss://%s/relay/webrelay.ashx?p=2&host=%s&port=%d&tls=0&tls1only=0&mode=%s",
		req.MPSHost, req.DeviceGUID, req.Port, req.Mode)

	// Connect to MPS WebSocket
	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	headers := http.Header{}
	headers.Set("Sec-WebSocket-Protocol", redirectToken)
	headers.Set("Cookie", fmt.Sprintf("jwt=%s", req.JWTToken))
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", req.JWTToken))

	srv.addLog("[*] Connecting to MPS...")
	conn, resp, err := dialer.Dial(wsURL, headers)
	if err != nil {
		errMsg := fmt.Sprintf("Dial failed: %v", err)
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			errMsg += fmt.Sprintf(" (HTTP %d: %s)", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("%s", errMsg)
	}
	srv.addLog("[OK] Connected to MPS")

	// Create session - AMT protocol handler
	session := &KVMSession{
		mpsConn:     conn,
		state:       "connecting",
		amtState:    "start",
		amtSequence: 0,
		user:        "admin",              // AMT username
		pass:        "",                   // Empty for CCM mode
		authURI:     "/RedirectionService", // AMT auth URI (fallback)
		deviceGUID:  req.DeviceGUID,       // Store device GUID
		done:        make(chan struct{}),
		logFn:       srv.addLog,
	}

	// Configure keepalive — deadline is reset on every received message too (see readFromMPS).
	conn.SetReadDeadline(time.Now().Add(120 * time.Second))
	conn.SetPongHandler(func(appData string) error {
		conn.SetReadDeadline(time.Now().Add(120 * time.Second))
		return nil
	})

	// Start relay goroutines
	go session.readFromMPS()
	go session.keepAlivePinger()

	// Initiate AMT protocol handshake
	srv.addLog("[*] Initiating AMT KVM handshake...")
	session.sendRedirectStartKVM()

	return session, nil
}

// =============================================================================
// CHANGE 10: HTTP Handlers — entirely NEW (replaces the CLI-driven main())
// =============================================================================

func (srv *KVMServer) addLog(msg string) {
	srv.logsMu.Lock()
	defer srv.logsMu.Unlock()
	srv.logs = append(srv.logs, msg)
	if len(srv.logs) > 500 {
		srv.logs = srv.logs[len(srv.logs)-500:]
	}
}

func (srv *KVMServer) serverLog(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Print(msg)
	srv.addLog(msg)
}

func (srv *KVMServer) getLogs() []string {
	srv.logsMu.Lock()
	defer srv.logsMu.Unlock()
	result := make([]string, len(srv.logs))
	copy(result, srv.logs)
	return result
}

// corsMiddleware adds CORS headers so the Angular dev server can call the API
func (srv *KVMServer) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = srv.config.AllowedOrigin
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// handleConnect — POST /api/connect
// Receives connection config from Angular, connects to MPS, starts AMT protocol
func (srv *KVMServer) handleConnect(w http.ResponseWriter, r *http.Request) {
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
		req.Mode = "kvm"
	}

	// Validate
	if req.JWTToken == "" {
		http.Error(w, "jwtToken is required", http.StatusBadRequest)
		return
	}

	// Disconnect existing session if any
	srv.mu.Lock()
	if srv.session != nil {
		srv.session.Close()
		srv.session = nil
	}
	srv.mu.Unlock()

	// Connect
	session, err := srv.connectToMPS(req)
	if err != nil {
		srv.addLog("[ERROR] " + err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	srv.mu.Lock()
	srv.session = session
	srv.mu.Unlock()

	// When session ends, clean up
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
func (srv *KVMServer) handleDisconnect(w http.ResponseWriter, r *http.Request) {
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
func (srv *KVMServer) handleStatus(w http.ResponseWriter, r *http.Request) {
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
		resp.MPSHost = srv.config.MPSHost
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// =============================================================================
// Consent API Handlers — for CCM mode user consent flow
// =============================================================================

// handleConsent — GET/POST /api/consent/:deviceGuid
// GET = Request consent code display, POST = Submit consent code
func (srv *KVMServer) handleConsent(w http.ResponseWriter, r *http.Request) {
	srv.serverLog("[CONSENT] %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
	if r.Method == "GET" {
		srv.handleConsentRequest(w, r)
	} else if r.Method == "POST" {
		srv.handleConsentSubmit(w, r)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleConsentRequest — GET /api/consent/:device Guid
// Requests consent code display on AMT device screen
func (srv *KVMServer) handleConsentRequest(w http.ResponseWriter, r *http.Request) {

	// Extract device GUID from URL path
	deviceGUID := r.URL.Path[len("/api/consent/"):]
	
	// Get JWT token from header
	jwtToken := r.Header.Get("Authorization")
	if jwtToken == "" {
		srv.serverLog("[CONSENT] Missing Authorization header for GET %s", deviceGUID)
		http.Error(w, "Authorization required", http.StatusUnauthorized)
		return
	}
	if len(jwtToken) > 7 && jwtToken[:7] == "Bearer " {
		jwtToken = jwtToken[7:]
	}

	srv.serverLog("[CONSENT] Requesting consent code for device=%s via MPS=%s", deviceGUID, srv.config.MPSHost)

	// Call MPS API to request consent code
	apiURL := fmt.Sprintf("https://%s/api/v1/amt/userConsentCode/%s", srv.config.MPSHost, deviceGUID)
	
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		srv.serverLog("[CONSENT][ERROR] Failed to create consent request: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Authorization", "Bearer "+jwtToken)

	resp, err := client.Do(req)
	if err != nil {
		srv.serverLog("[CONSENT][ERROR] Failed to request consent code: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Read response body
	body, _ := io.ReadAll(resp.Body)
	srv.serverLog("[CONSENT] GET response status=%d body=%s", resp.StatusCode, string(body))
	
	if resp.StatusCode != http.StatusOK {
		srv.serverLog("[CONSENT][ERROR] MPS consent request failed: %s", string(body))
		http.Error(w, string(body), resp.StatusCode)
		return
	}

	var consentResp ConsentResponse
	if err := json.Unmarshal(body, &consentResp); err != nil {
		srv.serverLog("[CONSENT][ERROR] Failed to parse consent response: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if consentResp.Body.ReturnValue != 0 {
		srv.serverLog("[CONSENT][ERROR] Consent request failed with ReturnValue=%d", consentResp.Body.ReturnValue)
		http.Error(w, fmt.Sprintf("Consent request failed (ReturnValue: %d)", consentResp.Body.ReturnValue), http.StatusBadRequest)
		return
	}

	srv.serverLog("[CONSENT] Consent code requested successfully for device=%s", deviceGUID)
	
	// Return success
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Consent code displayed on device screen",
	})
}

// handleConsentSubmit — POST /api/consent/:deviceGuid
// Submits user-entered consent code for validation
func (srv *KVMServer) handleConsentSubmit(w http.ResponseWriter, r *http.Request) {

	// Extract device GUID from URL path
	deviceGUID := r.URL.Path[len("/api/consent/"):]
	
	// Get JWT token from header
	jwtToken := r.Header.Get("Authorization")
	if jwtToken == "" {
		srv.serverLog("[CONSENT] Missing Authorization header for POST %s", deviceGUID)
		http.Error(w, "Authorization required", http.StatusUnauthorized)
		return
	}
	if len(jwtToken) > 7 && jwtToken[:7] == "Bearer " {
		jwtToken = jwtToken[7:]
	}

	// Parse request body
	var req ConsentCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		srv.serverLog("[CONSENT][ERROR] Invalid consent submit JSON for device=%s: %v", deviceGUID, err)
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.ConsentCode == "" {
		srv.serverLog("[CONSENT][ERROR] Empty consent code for device=%s", deviceGUID)
		http.Error(w, "consentCode is required", http.StatusBadRequest)
		return
	}

	srv.serverLog("[CONSENT] Submitting consent code for device=%s code=%s", deviceGUID, req.ConsentCode)

	// Call MPS API to submit consent code
	apiURL := fmt.Sprintf("https://%s/api/v1/amt/userConsentCode/%s", srv.config.MPSHost, deviceGUID)
	
	payload := map[string]string{"consentCode": req.ConsentCode}
	jsonData, _ := json.Marshal(payload)

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		srv.serverLog("[CONSENT][ERROR] Failed to create consent submit request: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+jwtToken)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		srv.serverLog("[CONSENT][ERROR] Failed to submit consent code: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Read response
	body, _ := io.ReadAll(resp.Body)
	srv.serverLog("[CONSENT] POST response status=%d body=%s", resp.StatusCode, string(body))
	
	var consentResp ConsentResponse
	if err := json.Unmarshal(body, &consentResp); err != nil {
		srv.serverLog("[CONSENT][ERROR] Failed to parse consent submit response: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if consentResp.Body.ReturnValue == 0 {
		srv.serverLog("[CONSENT] Consent code accepted for device=%s", deviceGUID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Consent granted",
		})
	} else if consentResp.Body.ReturnValue == 2058 {
		srv.serverLog("[CONSENT][ERROR] Invalid consent code for device=%s", deviceGUID)
		http.Error(w, "Invalid consent code", http.StatusBadRequest)
	} else {
		srv.addLog(fmt.Sprintf("[ERROR] Consent submission failed (ReturnValue: %d)", consentResp.Body.ReturnValue))
		http.Error(w, fmt.Sprintf("Consent failed (ReturnValue: %d)", consentResp.Body.ReturnValue), http.StatusBadRequest)
	}
}

// =============================================================================
// CHANGE 11: handleKVMWebSocket — WebSocket endpoint for browser KVM viewer
// This is the key endpoint: browser connects here to relay KVM data
//
// Flow:
//   Browser (Canvas + RGB565 decoder) ←→ WS /ws/kvm ←→ KVMSession ←→ MPS ←→ AMT device
//
// Browser sends: Mouse/keyboard events (RFB protocol) → MPS
// AMT sends: Framebuffer updates (RFB protocol) → Browser for canvas rendering
// =============================================================================

func (srv *KVMServer) handleKVMWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP to WebSocket
	conn, err := srv.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ERROR] WebSocket upgrade failed: %v", err)
		return
	}

	srv.mu.RLock()
	session := srv.session
	srv.mu.RUnlock()

	if session == nil || session.getState() == "disconnected" || session.getState() == "error" {
		log.Printf("[WARN] KVM WebSocket connection rejected - no active session")
		conn.Close()
		return
	}

	// Attach browser connection to session
	session.browserMu.Lock()
	oldConn := session.browserConn
	session.browserConn = conn
	session.browserMu.Unlock()
	if oldConn != nil {
		oldConn.Close() // close previous browser connection gracefully
	}

	srv.addLog("[OK] KVM viewer connected")
	session.flushPendingBrowserFrames()

	// Set up browser WebSocket ping/pong to keep browser connection alive
	conn.SetReadDeadline(time.Now().Add(120 * time.Second))
	conn.SetPongHandler(func(appData string) error {
		conn.SetReadDeadline(time.Now().Add(120 * time.Second))
		return nil
	})

	// Ping browser periodically to keep its connection alive
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
					return // another browser took over
				}
				if err := conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(5*time.Second)); err != nil {
					return
				}
			}
		}
	}()

	// Relay binary data from browser to MPS (mouse/keyboard events, RFB protocol)
	session.readFromBrowser()

	srv.addLog("[*] KVM viewer disconnected")
}

// =============================================================================
// CHANGE 12: main() — completely rewritten
// Before: parsed flags, connected, ran protocol, sent test command, exited
// After:  parsed flags, starts HTTP server, waits for REST/WS connections
// =============================================================================

func main() {
	config := ServerConfig{}
	flag.StringVar(&config.ListenAddr, "listen", ":8080", "HTTP server listen address")
	flag.StringVar(&config.MPSHost, "mps-host", "mps-wss.orch-10-139-218-43.pid.infra-host.com", "MPS host")
	flag.StringVar(&config.DeviceGUID, "guid", "94e00576-d750-3391-de61-48210b50d802", "Device GUID")
	flag.IntVar(&config.AMTPort, "amt-port", 16994, "AMT KVM port")
	flag.StringVar(&config.StaticDir, "static", "", "Path to Angular dist/ folder to serve (optional)")
	flag.StringVar(&config.AllowedOrigin, "cors-origin", "http://localhost:4200", "Allowed CORS origin")
	flag.Parse()

	srv := &KVMServer{
		config: config,
		upgrader: websocket.Upgrader{
			// Allow connections from Angular dev server
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}

	// API routes
	mux := http.NewServeMux()
	mux.HandleFunc("/api/connect", srv.handleConnect)
	mux.HandleFunc("/api/disconnect", srv.handleDisconnect)
	mux.HandleFunc("/api/status", srv.handleStatus)
	mux.HandleFunc("/api/consent/", srv.handleConsent) // Handles both GET (request) and POST (submit)
	mux.HandleFunc("/ws/kvm", srv.handleKVMWebSocket)

	// Optionally serve Angular static files
	if config.StaticDir != "" {
		log.Printf("[*] Serving static files from %s", config.StaticDir)
		mux.Handle("/", http.FileServer(http.Dir(config.StaticDir)))
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprint(w, `<!DOCTYPE html><html><body style="background:#1a1b26;color:#a9b1d6;font-family:monospace;padding:40px">
<h1>AMT KVM Server</h1>
<p>API Endpoints:</p>
<ul>
<li>POST /api/connect — Start KVM session (JSON body: mpsHost, deviceGuid, jwtToken)</li>
<li>POST /api/disconnect — End KVM session</li>
<li>GET /api/status — Get session status and logs</li>
<li>WS /ws/kvm — WebSocket for KVM data relay</li>
</ul>
<p>Connect the Angular frontend at <a href="http://localhost:4200" style="color:#89b4fa">http://localhost:4200</a></p>
</body></html>`)
				return
			}
			http.NotFound(w, r)
		})
	}

	// Start server
	handler := srv.corsMiddleware(mux)

	log.Printf("========================================================================")
	log.Printf("  Intel AMT KVM Server (Pure Binary Relay)")
	log.Printf("========================================================================")
	log.Printf("  Listen:       %s", config.ListenAddr)
	log.Printf("  MPS Host:     %s", config.MPSHost)
	log.Printf("  Device GUID:  %s", config.DeviceGUID)
	log.Printf("  CORS Origin:  %s", config.AllowedOrigin)
	if config.StaticDir != "" {
		log.Printf("  Static Dir:   %s", config.StaticDir)
	}
	log.Printf("========================================================================")
	log.Printf("")
	log.Printf("  API:      http://localhost%s/api/status", config.ListenAddr)
	log.Printf("  KVM WebSocket: ws://localhost%s/ws/kvm", config.ListenAddr)
	log.Printf("")

	if err := http.ListenAndServe(config.ListenAddr, handler); err != nil {
		log.Fatalf("[ERROR] Server failed: %v", err)
		os.Exit(1)
	}
}
