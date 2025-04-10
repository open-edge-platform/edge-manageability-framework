package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"time"
)

func main() {
	// CIRA server address (replace with your server's address and port)
	// serverAddress := "mps-node.kind.internal:4433"
	serverAddress := "172.18.255.237:4433"
	// serverAddress := "localhost:4433"

	// TLS configuration
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true, // Set to false in production if you have a valid certificate
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS13,
	}

	// Dial the server
	fmt.Printf("Connecting to CIRA server at %s...\n", serverAddress)
	conn, err := tls.DialWithDialer(&net.Dialer{
		Timeout: 5 * time.Second,
	}, "tcp", serverAddress, tlsConfig)
	if err != nil {
		log.Fatalf("Failed to connect to CIRA server at %s: %v\n", serverAddress, err)
	}
	defer conn.Close()

	fmt.Println("Connected to CIRA server successfully!")

	// Send a sample CIRA handshake message (replace with actual CIRA protocol handshake if needed)
	message := []byte("CIRA handshake message")
	_, err = conn.Write(message)
	if err != nil {
		log.Fatalf("Failed to send handshake message: %v\n", err)
	}
	fmt.Println("Handshake message sent successfully!")

	// Read the server's response
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		log.Fatalf("Failed to read server response: %v\n", err)
	}
	fmt.Printf("Received response from server: %s\n", string(buffer[:n]))
}
