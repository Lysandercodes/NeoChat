package network

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
)

// SyncMessage represents a P2P payload between NeoChat instances.
type SyncMessage struct {
	Type    string `json:"type"` // e.g., "handshake", "message_sync", "ota_update", "telemetry"
	Payload []byte `json:"payload"`
}

// OTAPayload represents the over-the-air update structure.
type OTAPayload struct {
	Version      string `json:"version"`
	ChatBinary   []byte `json:"chat_binary"`
	ChatdBinary  []byte `json:"chatd_binary"`
}

// P2PServer handles incoming peer connections.
type P2PServer struct {
	tsNode *TSNode
	port   int
}

func NewP2PServer(node *TSNode, port int) *P2PServer {
	return &P2PServer{
		tsNode: node,
		port:   port,
	}
}

// Start begins listening for peer connections.
func (s *P2PServer) Start() error {
	ln, err := s.tsNode.Listen(s.port)
	if err != nil {
		return err
	}

	log.Printf("P2P Server listening on tailscale port %d\n", s.port)

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				log.Printf("Error accepting connection: %v\n", err)
				continue
			}
			go s.handleConnection(conn)
		}
	}()

	return nil
}

func (s *P2PServer) handleConnection(conn net.Conn) {
	defer conn.Close()
	log.Printf("Peer connected: %s\n", conn.RemoteAddr().String())

	decoder := json.NewDecoder(conn)
	for {
		var msg SyncMessage
		if err := decoder.Decode(&msg); err != nil {
			if err != io.EOF {
				log.Printf("Error decoding message: %v\n", err)
			}
			break
		}
		
		s.processMessage(conn, msg)
	}
}

func (s *P2PServer) processMessage(conn net.Conn, msg SyncMessage) {
	switch msg.Type {
	case "handshake":
		fmt.Println("Received handshake from peer. Comparing High-Water Marks...")
		// Compare timestamps, diff generation logic here
		
	case "message_sync":
		fmt.Println("Received message sync payload. Executing INSERT ON CONFLICT DO UPDATE...")
		// Upsert logic here to guarantee no gaps
		
	case "telemetry":
		fmt.Println("Received remote error telemetry stream.")
		// Notify UI to print to :logs
		
	case "ota_update":
		fmt.Println("CRITICAL: Received Over-The-Air update payload!")
		s.handleOTAUpdate(msg.Payload)
		
	default:
		log.Printf("Unknown message type: %s\n", msg.Type)
	}
}

func (s *P2PServer) handleOTAUpdate(payload []byte) {
	var ota OTAPayload
	if err := json.Unmarshal(payload, &ota); err != nil {
		log.Printf("Failed to decode OTA payload: %v\n", err)
		return
	}
	
	log.Printf("Applying OTA Update version %s...\n", ota.Version)
	
	// 1. Write new binaries to disk (replacing old ones)
	err1 := os.WriteFile("/usr/local/bin/chat", ota.ChatBinary, 0755)
	err2 := os.WriteFile("/usr/local/bin/chatd", ota.ChatdBinary, 0755)
	
	if err1 != nil || err2 != nil {
		log.Printf("OTA Error writing binaries: %v, %v\n", err1, err2)
		return
	}
	
	log.Println("Binaries updated on disk. Executing self-restart via Launchd...")
	
	// 2. Self-Restart (assuming macOS launchd)
	cmd := exec.Command("launchctl", "stop", "com.neotrinity.chatd")
	_ = cmd.Run() // the service is set to KeepAlive=true, so it will instantly restart the new binary.
}
