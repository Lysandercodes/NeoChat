package ipc

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
)

const SocketPath = "/tmp/neochatd.sock"

// Message represents a command sent over the local unix socket.
type Message struct {
	Command string `json:"command"`
	Payload string `json:"payload"`
}

// Server listens for connections from the UI.
type Server struct {
	listener net.Listener
}

// StartIPCServer creates a unix socket server for chatd.
func StartIPCServer() (*Server, error) {
	os.Remove(SocketPath)
	ln, err := net.Listen("unix", SocketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to start IPC server: %w", err)
	}

	s := &Server{listener: ln}
	go s.acceptConnections()
	return s, nil
}

func (s *Server) acceptConnections() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			continue
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	decoder := json.NewDecoder(conn)
	for {
		var msg Message
		if err := decoder.Decode(&msg); err != nil {
			break
		}
		// Handle command from UI
		fmt.Printf("Received IPC Command: %s\n", msg.Command)
	}
}

// Client is used by the chat UI to send commands to chatd.
type Client struct {
	conn net.Conn
}

func NewClient() (*Client, error) {
	conn, err := net.Dial("unix", SocketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to chatd: %w", err)
	}
	return &Client{conn: conn}, nil
}

func (c *Client) SendCommand(cmd, payload string) error {
	msg := Message{Command: cmd, Payload: payload}
	encoder := json.NewEncoder(c.conn)
	return encoder.Encode(msg)
}

func (c *Client) Close() {
	c.conn.Close()
}
