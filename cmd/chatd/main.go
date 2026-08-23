package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/NeoTrinity/chat/internal/autopilot"
	"github.com/NeoTrinity/chat/internal/ipc"
	"github.com/NeoTrinity/chat/internal/network"
)

func main() {
	// Determine state directory.
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Cannot determine home directory: %v", err)
	}
	stateDir := filepath.Join(homeDir, ".config", "neochat")
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		log.Fatalf("Cannot create state directory: %v", err)
	}

	// Start embedded Tailscale node.
	// Hostname is used as the Tailscale machine name.
	// On Trinity's machine the binary is identical; the alias is set via config.
	hostname := os.Getenv("NEOCHAT_ALIAS")
	if hostname == "" {
		hostname = "neochat"
	}
	tsNode, err := network.StartTailscale(stateDir, hostname)
	if err != nil {
		log.Fatalf("Failed to start Tailscale: %v", err)
	}
	defer tsNode.Close()

	// Start the Autopilot listener *on the Tailscale interface only*.
	// This guarantees it is never reachable from the LAN or public internet.
	apListener, err := tsNode.Listen(autopilot.AutopilotPort)
	if err != nil {
		log.Fatalf("Failed to start autopilot listener: %v", err)
	}
	go autopilot.ListenAndServe(apListener)
	fmt.Printf("[chatd] Autopilot listener active on Tailscale port %d.\n", autopilot.AutopilotPort)

	// Start the P2P sync server on the Tailscale interface.
	p2pServer := network.NewP2PServer(tsNode, 7122)
	if err := p2pServer.Start(); err != nil {
		log.Fatalf("Failed to start P2P sync server: %v", err)
	}

	// Initialize IPC Server (Unix socket for chat UI ↔ chatd bridge).
	ipcServer, err := ipc.StartIPCServer()
	if err != nil {
		log.Fatalf("Failed to start IPC server: %v", err)
	}
	_ = ipcServer

	fmt.Println("[chatd] NeoChat daemon running. All listeners are Tailscale-only.")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-sigChan

	fmt.Println("[chatd] Shutting down...")
}
