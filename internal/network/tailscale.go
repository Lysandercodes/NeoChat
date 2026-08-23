package network

import (
	"fmt"
	"log"
	"net"
	"path/filepath"

	"tailscale.com/tsnet"
)

// TSNode wraps a tsnet.Server.
type TSNode struct {
	server *tsnet.Server
}

// StartTailscale initializes the embedded Tailscale node.
// It stores its state in the given stateDir.
func StartTailscale(stateDir string, hostname string) (*TSNode, error) {
	tsDir := filepath.Join(stateDir, "tailscale")

	s := &tsnet.Server{
		Hostname: hostname,
		Dir:      tsDir,
		Logf:     func(format string, args ...any) {}, // Silence noisy logs for UI sake
	}

	if err := s.Start(); err != nil {
		return nil, fmt.Errorf("failed to start tailscale node: %w", err)
	}

	log.Printf("Embedded Tailscale node started. Hostname: %s\n", hostname)
	return &TSNode{server: s}, nil
}

// Listen creates a TCP listener on the Tailscale network on the specified port.
func (n *TSNode) Listen(port int) (net.Listener, error) {
	addr := fmt.Sprintf(":%d", port)
	ln, err := n.server.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on tailscale network: %w", err)
	}
	return ln, nil
}

// Dial connects to a peer on the Tailscale network.
func (n *TSNode) Dial(host string, port int) (net.Conn, error) {
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := n.server.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial peer %s: %w", host, err)
	}
	return conn, nil
}

// Close gracefully shuts down the Tailscale node.
func (n *TSNode) Close() error {
	return n.server.Close()
}
