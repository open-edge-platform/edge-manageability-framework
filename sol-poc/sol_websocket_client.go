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
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)
/*
cd /home/seu/edge-manageability-framework/SOL-POC-NEW && go run sol_websocket_client.go \
  -token "$JWT_TOKEN" \
  -pass '3x@&BOmk88TL' \
  -linux-user "user" \
  -linux-pass "user1234" \
  -cmd "ls"
  */

// Config holds the WebSocket connection parameters
type Config struct {
	MPSHost       string
	DeviceGUID    string
	Port          int
	Protocol      int
	Mode          string
	KeycloakToken string
	Insecure      bool
	TestCmd       string
	AMTUser       string
	AMTPass       string
	LinuxUser     string
	LinuxPass     string
}

// RedirectTokenResponse represents the API response for redirect token
type RedirectTokenResponse struct {
	Token string `json:"token"`
}

// SOLSession manages the AMT SOL protocol state machine
type SOLSession struct {
	conn        *websocket.Conn
	mu          sync.Mutex
	amtSequence uint32
	solReady    chan struct{}
	output      strings.Builder
	outputMu    sync.Mutex
	user        string
	pass        string
	authURI     string
}

// intToLE writes a uint32 as 4 little-endian bytes
func intToLE(v uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	return b
}

// shortToLE writes a uint16 as 2 little-endian bytes
func shortToLE(v uint16) []byte {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, v)
	return b
}

// nextSequence returns the next AMT sequence number (thread-safe)
func (s *SOLSession) nextSequence() uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	seq := s.amtSequence
	s.amtSequence++
	return seq
}

// sendBinary sends a binary WebSocket message (thread-safe)
func (s *SOLSession) sendBinary(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn.WriteMessage(websocket.BinaryMessage, data)
}

// sendSOLData wraps terminal data in an AMT SOL data frame (0x28) and sends it
// Frame format from AMTRedirector.ts:
//
//	0x28 0x00 0x00 0x00 + IntToStrX(sequence) + ShortToStrX(data.length) + data
func (s *SOLSession) sendSOLData(data string) error {
	seq := s.nextSequence()
	frame := []byte{0x28, 0x00, 0x00, 0x00}
	frame = append(frame, intToLE(seq)...)
	frame = append(frame, shortToLE(uint16(len(data)))...)
	frame = append(frame, []byte(data)...)
	log.Printf("[SOL-TX] Sending %d bytes: %q", len(data), data)
	log.Printf("[SOL-TX] Frame hex: %s", hex.EncodeToString(frame))
	return s.sendBinary(frame)
}

// appendOutput collects SOL terminal output (thread-safe)
func (s *SOLSession) appendOutput(data string) {
	s.outputMu.Lock()
	defer s.outputMu.Unlock()
	s.output.WriteString(data)
}

// getOutput returns all collected SOL output so far
func (s *SOLSession) getOutput() string {
	s.outputMu.Lock()
	defer s.outputMu.Unlock()
	return s.output.String()
}

// hexMD5 returns the hex-encoded MD5 hash of the input string
func hexMD5(str string) string {
	h := md5.Sum([]byte(str))
	return hex.EncodeToString(h[:])
}

// generateRandomNonce generates a random hex nonce
func generateRandomNonce(byteLen int) string {
	b := make([]byte, byteLen)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
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
	log.Printf("[AUTH] Sending Digest Auth initial (user=%q): %s", user, hex.EncodeToString(msg))
	return s.sendBinary(msg)
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
	// AMT digest format (from AMTRedirector.ts): MD5(HA1:nonce:nc:cnonce:qop :HA2)
	// Note: AMT uses a space before the last colon (non-standard)
	responseStr := ha1 + ":" + nonce + ":" + snc + ":" + cnonce + ":" + qop + " :" + ha2
	digest := hexMD5(responseStr)
	log.Printf("[AUTH] HA1(%q) = %q", user+":"+realm+":***", ha1)
	log.Printf("[AUTH] HA2(%q) = %q", "POST:"+uri, ha2)
	log.Printf("[AUTH] Response(%q) = %q", responseStr, digest)
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
	log.Printf("[AUTH] Sending Digest Auth response (%d bytes)", len(msg))
	return s.sendBinary(msg)
}

// getMPSRedirectToken retrieves the WebSocket redirect token from MPS API
func getMPSRedirectToken(mpsHost, deviceGUID, keycloakToken string, insecure bool) (string, error) {
	url := fmt.Sprintf("https://%s/api/v1/authorize/redirection/%s", mpsHost, deviceGUID)
	log.Printf("[*] Redirect token URL: %s", url)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	if insecure {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Traefik middleware requires Keycloak JWT in a cookie named "jwt"
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

func main() {
	// Parse command-line flags
	config := Config{}
	flag.StringVar(&config.MPSHost, "host", "mps-wss.orch-10-139-218-35.pid.infra-host.com", "MPS WebSocket host")
	//flag.StringVar(&config.MPSHost, "host", "10.190.167.230", "MPS WebSocket host")
	flag.StringVar(&config.DeviceGUID, "guid", "89174ecf-31c3-22e3-5f8d-48210b509c73", "Device GUID")
	flag.StringVar(&config.KeycloakToken, "token", "", "Keycloak JWT authentication token (required)")
	flag.IntVar(&config.Port, "port", 16994, "SOL port (16994 for KVM, 16992 for SOL)")
	flag.IntVar(&config.Protocol, "protocol", 1, "Protocol type (1=SOL, 2=KVM, 3=IDER)")
	flag.StringVar(&config.Mode, "mode", "sol", "Connection mode")
	flag.BoolVar(&config.Insecure, "insecure", true, "Skip TLS certificate verification")
	flag.StringVar(&config.TestCmd, "cmd", "hostname", "Linux command to send after SOL session is established")
	flag.StringVar(&config.AMTUser, "user", "admin", "AMT device username for digest auth")
	flag.StringVar(&config.AMTPass, "pass", "", "AMT device password for digest auth")
	flag.StringVar(&config.LinuxUser, "linux-user", "", "Linux username for device login (if device is at login prompt)")
	flag.StringVar(&config.LinuxPass, "linux-pass", "", "Linux password for device login")
	flag.Parse()

	// Validate required parameters
	if config.KeycloakToken == "" {
		log.Fatal("[ERROR] Keycloak token is required. Use -token flag")
	}
	if config.AMTPass == "" {
		log.Fatal("[ERROR] AMT password is required. Use -pass flag. Get it from: kubectl exec -n orch-platform vault-0 -- vault kv get -field=password secret/amt-password")
	}

	// Get MPS redirect token for WebSocket authentication
	log.Printf("[*] Getting MPS redirect token...")
	redirectToken, err := getMPSRedirectToken(config.MPSHost, config.DeviceGUID, config.KeycloakToken, config.Insecure)
	if err != nil {
		log.Fatalf("[ERROR] Failed to get redirect token: %v", err)
	}
	log.Printf("[OK] Redirect token obtained")

	// Build WebSocket URL
	// From AMTRedirector.ts: p=2 always means "REDIRECTION session"
	wsURL := fmt.Sprintf("wss://%s/relay/webrelay.ashx?p=2&host=%s&port=%d&tls=0&tls1only=0&mode=%s",
		config.MPSHost,
		config.DeviceGUID,
		config.Port,
		config.Mode,
	)

	// Print configuration
	log.Printf("========================================================================")
	log.Printf("         AMT Serial-over-LAN (SOL) Client")
	log.Printf("========================================================================")
	log.Printf("  MPS Host:    %s", config.MPSHost)
	log.Printf("  Device GUID: %s", config.DeviceGUID)
	log.Printf("  SOL Port:    %d", config.Port)
	log.Printf("  Protocol:    %d (SOL=1, KVM=2, IDER=3)", config.Protocol)
	log.Printf("  Test Cmd:    %q", config.TestCmd)
	log.Printf("  WebSocket:   %s", wsURL)
	log.Printf("========================================================================")

	// Setup WebSocket dialer
	dialer := websocket.Dialer{}
	if config.Insecure {
		dialer.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	// MPS expects redirect token in Sec-WebSocket-Protocol header
	// Traefik middleware requires Keycloak JWT in cookie named "jwt"
	headers := http.Header{}
	headers.Add("Sec-WebSocket-Protocol", redirectToken)
	headers.Add("Cookie", fmt.Sprintf("jwt=%s", config.KeycloakToken))

	// Connect to WebSocket
	log.Printf("[*] Connecting to WebSocket...")
	conn, resp, err := dialer.Dial(wsURL, headers)
	if err != nil {
		log.Printf("[ERROR] Connection failed: %v", err)
		if resp != nil {
			log.Printf("   HTTP Status: %s", resp.Status)
			if resp.Body != nil {
				body, _ := io.ReadAll(resp.Body)
				if len(body) > 0 {
					log.Printf("   Response Body: %s", string(body))
				}
			}
		}
		os.Exit(1)
	}
	defer conn.Close()

	log.Printf("[OK] WebSocket connection ESTABLISHED!")
	log.Printf("   Local:  %s", conn.LocalAddr())
	log.Printf("   Remote: %s", conn.RemoteAddr())

	// Initialize SOL session state
	sol := &SOLSession{
		conn:     conn,
		solReady: make(chan struct{}),
		user:    config.AMTUser,
		pass:    config.AMTPass,
		authURI: "",
	}

	// Send StartRedirectionSession for SOL
	// From device-management-toolkit AMTRedirector.ts:
	//   SOL:  0x10 0x00 0x00 0x00 0x53 0x4F 0x4C 0x20  ("SOL ")
	//   KVM:  0x10 0x01 0x00 0x00 0x4B 0x56 0x4D 0x52  ("KVMR")
	solStartCmd := []byte{0x10, 0x00, 0x00, 0x00, 0x53, 0x4F, 0x4C, 0x20}
	log.Printf("[*] Sending StartRedirectionSession (SOL)...")
	err = sol.sendBinary(solStartCmd)
	if err != nil {
		log.Fatalf("[ERROR] Failed to send SOL start: %v", err)
	}
	log.Printf("[OK] SOL start command sent: %s", hex.EncodeToString(solStartCmd))

	// Setup graceful shutdown
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	done := make(chan struct{})

	// =========================================================================
	// Reader goroutine: handles the full AMT SOL protocol state machine
	// Protocol flow (from AMTRedirector.ts):
	//   1. → 0x10 StartRedirectionSession (SOL)
	//   2. ← 0x11 StartRedirectionSessionReply (status=0 means success)
	//   3. → 0x13 AuthenticateSession (query available auth methods)
	//   4. ← 0x14 AuthenticateSessionReply (status=0+authType=0 = success)
	//   5. → 0x20 SOL settings (MaxTxBuffer, timeouts, etc.)
	//   6. ← 0x21 Response to settings
	//   7. → 0x27 Finalize session setup
	//   8. SOL session active! Terminal data flows via 0x28 (TX) / 0x2A (RX)
	// =========================================================================
	go func() {
		defer close(done)
		msgCount := 0

		log.Printf("[*] Waiting for AMT protocol messages...")

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					log.Printf("[*] Connection closed normally")
				} else {
					log.Printf("[ERROR] Read error: %v", err)
				}
				return
			}

			msgCount++
			if len(message) == 0 {
				continue
			}

			// Log raw data for debugging
			if len(message) <= 128 {
				log.Printf("[MSG #%d] cmd=0x%02X size=%d hex=%s",
					msgCount, message[0], len(message), hex.EncodeToString(message))
			} else {
				log.Printf("[MSG #%d] cmd=0x%02X size=%d hex=%s...(truncated)",
					msgCount, message[0], len(message), hex.EncodeToString(message[:64]))
			}

			switch message[0] {

			case 0x11: // StartRedirectionSessionReply
				if len(message) < 4 {
					log.Printf("[ERROR] 0x11 message too short (%d bytes)", len(message))
					continue
				}
				status := message[1]
				log.Printf("[PROTOCOL] StartRedirectionSessionReply: status=%d", status)
				if status != 0 {
					log.Printf("[ERROR] Session start failed with status %d", status)
					return
				}
				// Parse OEM data length at byte 12
				if len(message) >= 13 {
					oemLen := int(message[12])
					log.Printf("   OEM data length: %d", oemLen)
				}
				// Send authentication query (0x13)
				authQuery := []byte{0x13, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
				log.Printf("[*] Sending AuthenticateSession query...")
				if err := sol.sendBinary(authQuery); err != nil {
					log.Printf("[ERROR] Failed to send auth query: %v", err)
					return
				}
				log.Printf("[OK] Auth query sent: %s", hex.EncodeToString(authQuery))

			case 0x14: // AuthenticateSessionReply
				if len(message) < 9 {
					log.Printf("[ERROR] 0x14 message too short (%d bytes)", len(message))
					continue
				}
				status := message[1]
				authType := message[4]
				authDataLen := int(binary.LittleEndian.Uint32(message[5:9]))
				log.Printf("[PROTOCOL] AuthenticateSessionReply: status=%d authType=%d dataLen=%d",
					status, authType, authDataLen)

				if status == 0 && authType == 0 {
					// Query response — check available auth methods
					var authMethods []byte
					if len(message) >= 9+authDataLen {
						authMethods = message[9 : 9+authDataLen]
						log.Printf("   Available auth methods: %v", authMethods)
					}
					// Check if Digest Auth (4) is required
					hasDigest := false
					for _, m := range authMethods {
						if m == 4 {
							hasDigest = true
							break
						}
					}
					if hasDigest {
						log.Printf("[AUTH] Digest Auth (method 4) required, sending initial request...")
						if err := sol.sendDigestAuthInitial(); err != nil {
							log.Printf("[ERROR] Failed to send digest auth initial: %v", err)
							return
						}
					} else {
						log.Printf("[OK] No digest auth required. Sending SOL settings...")
						sol.sendSOLSettings()
					}
				} else if status == 0 {
					// Auth completed successfully
					log.Printf("[OK] Authentication successful (authType=%d). Sending SOL settings...", authType)
					sol.sendSOLSettings()
				} else if status == 1 && (authType == 3 || authType == 4) {
					// Digest challenge from server — parse realm/nonce/qop
					log.Printf("[AUTH] Digest challenge received (authType=%d)", authType)
					if len(message) < 9+authDataLen {
						log.Printf("[ERROR] Challenge too short")
						return
					}
					authData := message[9 : 9+authDataLen]
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
					log.Printf("[AUTH] Challenge: realm=%q nonce=%q qop=%q", realm, nonce, qop)
					if err := sol.sendDigestAuthResponse(realm, nonce, qop); err != nil {
						log.Printf("[ERROR] Failed to send digest response: %v", err)
						return
					}
				} else {
					log.Printf("[ERROR] Authentication failed: status=%d authType=%d", status, authType)
					return
				}

			case 0x21: // Response to SOL settings
				if len(message) < 23 {
					log.Printf("[WARN] 0x21 message shorter than expected (%d bytes)", len(message))
				}
				log.Printf("[PROTOCOL] SOL Settings Response received")
				// Send session finalization (0x27) — from AMTRedirector.ts
				seq := sol.nextSequence()
				finalizeMsg := []byte{0x27, 0x00, 0x00, 0x00}
				finalizeMsg = append(finalizeMsg, intToLE(seq)...)
				finalizeMsg = append(finalizeMsg, 0x00, 0x00, 0x1B, 0x00, 0x00, 0x00)
				log.Printf("[*] Sending session finalization (0x27)...")
				if err := sol.sendBinary(finalizeMsg); err != nil {
					log.Printf("[ERROR] Failed to send finalization: %v", err)
					return
				}
				log.Printf("[OK] Session finalization sent")

				// SOL session is now active!
				log.Printf("")
				log.Printf("========================================================================")
				log.Printf("  SOL SESSION ACTIVE - Terminal data can now be sent/received")
				log.Printf("========================================================================")
				log.Printf("")

				// Signal that SOL is ready
				select {
				case <-sol.solReady:
					// already closed
				default:
					close(sol.solReady)
				}

			case 0x29: // Serial Settings
				if len(message) >= 10 {
					log.Printf("[PROTOCOL] Serial Settings received (10 bytes)")
				}

			case 0x2A: // Incoming display data (terminal output)
				if len(message) < 10 {
					log.Printf("[WARN] 0x2A message too short (%d bytes)", len(message))
					continue
				}
				// Data length is at bytes 8-9 (little-endian)
				dataLen := int(message[8]) | int(message[9])<<8
				if len(message) < 10+dataLen {
					log.Printf("[WARN] 0x2A truncated: expected %d data bytes, got %d",
						dataLen, len(message)-10)
					dataLen = len(message) - 10
				}
				termData := string(message[10 : 10+dataLen])
				sol.appendOutput(termData)
				log.Printf("[SOL-RX] Terminal output (%d bytes):", dataLen)
				log.Printf("   %s", strings.ReplaceAll(termData, "\n", "\n   "))

			case 0x2B: // Keep alive
				if len(message) >= 8 {
					log.Printf("[PROTOCOL] Keep alive received")
				}

			default:
				log.Printf("[PROTOCOL] Unknown command 0x%02X (%d bytes)", message[0], len(message))
			}
		}
	}()

	// =========================================================================
	// Command sender: waits for SOL session to be ready, then sends test command
	// =========================================================================
	go func() {
		// Wait for SOL session to be established (or timeout)
		select {
		case <-sol.solReady:
			log.Printf("[*] SOL session ready, preparing to send test command...")
		case <-time.After(30 * time.Second):
			log.Printf("[ERROR] Timeout waiting for SOL session to become ready")
			return
		case <-done:
			return
		}

		// Small delay to let the terminal settle
		time.Sleep(1 * time.Second)

		// Helper: wait until output contains a target string (or timeout)
		waitForOutput := func(target string, timeout time.Duration) bool {
			deadline := time.Now().Add(timeout)
			for time.Now().Before(deadline) {
				if strings.Contains(sol.getOutput(), target) {
					return true
				}
				time.Sleep(200 * time.Millisecond)
			}
			return false
		}

		// Send a CR to wake up the terminal / get to login prompt
		log.Printf("[*] Sending CR to wake terminal...")
		if err := sol.sendSOLData("\r"); err != nil {
			log.Printf("[ERROR] Failed to send CR: %v", err)
			return
		}
		time.Sleep(2 * time.Second)

		// If Linux credentials provided, log in first
		if config.LinuxUser != "" {
			log.Printf("[LOGIN] Logging in as %q...", config.LinuxUser)

			// Check if we're at a login prompt
			currentOutput := sol.getOutput()
			if strings.Contains(currentOutput, "login:") {
				log.Printf("[LOGIN] Login prompt detected")
			} else {
				log.Printf("[LOGIN] Waiting for login prompt...")
				if !waitForOutput("login:", 5*time.Second) {
					log.Printf("[LOGIN] No login prompt found, sending CR and retrying...")
					sol.sendSOLData("\r")
					if !waitForOutput("login:", 5*time.Second) {
						log.Printf("[WARN] Still no login prompt. Proceeding anyway...")
					}
				}
			}

			// Send username
			log.Printf("[LOGIN] Sending username: %q", config.LinuxUser)
			if err := sol.sendSOLData(config.LinuxUser + "\r"); err != nil {
				log.Printf("[ERROR] Failed to send username: %v", err)
				return
			}

			// Wait for password prompt
			log.Printf("[LOGIN] Waiting for password prompt...")
			if !waitForOutput("Password:", 10*time.Second) {
				log.Printf("[WARN] Password prompt not detected, sending password anyway...")
			}

			// Send password
			log.Printf("[LOGIN] Sending password...")
			if err := sol.sendSOLData(config.LinuxPass + "\r"); err != nil {
				log.Printf("[ERROR] Failed to send password: %v", err)
				return
			}

			// Wait for shell prompt ($ or # or ~)
			log.Printf("[LOGIN] Waiting for shell prompt...")
			time.Sleep(3 * time.Second)

			// Check if login succeeded
			loginOutput := sol.getOutput()
			if strings.Contains(loginOutput, "Login incorrect") {
				log.Printf("[ERROR] Login failed: incorrect credentials")
				return
			}
			log.Printf("[LOGIN] Login completed. Clearing output buffer...")

			// Clear the output buffer so we only capture command output
			sol.outputMu.Lock()
			sol.output.Reset()
			sol.outputMu.Unlock()
		}

		// Now send the actual command
		testCmd := config.TestCmd
		log.Printf("")
		log.Printf("[CMD] ========================================")
		log.Printf("[CMD] Executing: %q", testCmd)
		log.Printf("[CMD] ========================================")
		if err := sol.sendSOLData(testCmd + "\r"); err != nil {
			log.Printf("[ERROR] Failed to send command: %v", err)
			return
		}

		// Wait for output
		log.Printf("[CMD] Waiting for command output...")
		time.Sleep(5 * time.Second)

		// Print results
		output := sol.getOutput()
		log.Printf("")
		log.Printf("[CMD] ========================================")
		log.Printf("[CMD] COMMAND OUTPUT")
		log.Printf("[CMD] ========================================")
		if len(output) > 0 {
			for _, line := range strings.Split(output, "\n") {
				line = strings.TrimRight(line, "\r")
				if line != "" {
					log.Printf("[CMD]   %s", line)
				}
			}
		} else {
			log.Printf("[CMD]   (no output received)")
		}
		log.Printf("[CMD] ======================================== ")
	}()

	// Wait for interrupt or done signal
	select {
	case <-interrupt:
		log.Printf("")
		log.Printf("[*] Interrupt signal received")
	case <-done:
		log.Printf("")
		log.Printf("[*] Connection closed")
	}

	// Graceful shutdown
	log.Printf("[*] Shutting down gracefully...")
	conn.Close()
	log.Printf("[OK] Cleanup complete. Goodbye!")
}

// sendSOLSettings sends the SOL configuration message (0x20) to the AMT device
// From AMTRedirector.ts: MaxTxBuffer=10000, TxTimeout=100, TxOverflowTimeout=0,
// RxTimeout=10000, RxFlushTimeout=100, Heartbeat=0
func (s *SOLSession) sendSOLSettings() {
	seq := s.nextSequence()
	msg := []byte{0x20, 0x00, 0x00, 0x00}
	msg = append(msg, intToLE(seq)...)
	msg = append(msg, shortToLE(10000)...) // MaxTxBuffer
	msg = append(msg, shortToLE(100)...)   // TxTimeout
	msg = append(msg, shortToLE(0)...)     // TxOverflowTimeout
	msg = append(msg, shortToLE(10000)...) // RxTimeout
	msg = append(msg, shortToLE(100)...)   // RxFlushTimeout
	msg = append(msg, shortToLE(0)...)     // Heartbeat
	msg = append(msg, 0x00, 0x00, 0x00, 0x00)

	log.Printf("[*] Sending SOL settings (0x20): %s", hex.EncodeToString(msg))
	if err := s.sendBinary(msg); err != nil {
		log.Printf("[ERROR] Failed to send SOL settings: %v", err)
	} else {
		log.Printf("[OK] SOL settings sent")
	}
}
