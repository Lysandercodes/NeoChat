package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/NeoTrinity/chat/internal/autopilot"
	"github.com/NeoTrinity/chat/internal/db"
	"github.com/NeoTrinity/chat/internal/ipc"
	"github.com/NeoTrinity/chat/internal/network"
)

// healthFile is written on successful boot so CI and monitoring tools
// can verify the daemon came up cleanly.
const healthFile = "/tmp/neochat_health"

func main() {
	// Global panic catcher — logs the stack trace and restarts cleanly
	// instead of dying silently and confusing launchd.
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[chatd] PANIC: %v\n%s", r, debug.Stack())
			os.Exit(1) // launchd KeepAlive will relaunch us
		}
	}()

	// Direct all output to a persistent log file so you can read it via autopilot.
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Cannot determine home directory: %v", err)
	}
	stateDir := filepath.Join(homeDir, ".config", "neochat")
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		log.Fatalf("Cannot create state directory: %v", err)
	}

	logPath := filepath.Join(stateDir, "chatd.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		log.Fatalf("Cannot open log file: %v", err)
	}
	defer logFile.Close()
	log.SetOutput(logFile)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	log.Println("─────────────────────────────────────────")
	log.Printf("[chatd] Starting up. PID %d\n", os.Getpid())

	// Open the local SQLite database.
	database, err := db.Open(filepath.Join(stateDir, "neochat.db"))
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	// Resolve alias from environment (set in the launchd plist).
	alias := os.Getenv("NEOCHAT_ALIAS")
	if alias == "" {
		alias = "neochat"
	}
	log.Printf("[chatd] Identity: %s\n", alias)

	// Start embedded Tailscale node.
	tsNode, err := network.StartTailscale(stateDir, alias)
	if err != nil {
		log.Fatalf("Failed to start Tailscale: %v", err)
	}
	defer tsNode.Close()

	// Start the Autopilot listener on the Tailscale interface only.
	apListener, err := tsNode.Listen(autopilot.AutopilotPort)
	if err != nil {
		log.Fatalf("Failed to start autopilot listener: %v", err)
	}
	go safeGo("autopilot", func() { autopilot.ListenAndServe(apListener) })
	log.Printf("[chatd] Autopilot listening on Tailscale port %d\n", autopilot.AutopilotPort)

	// Start the P2P sync server on the Tailscale interface.
	p2pServer := network.NewP2PServer(tsNode, 7122, database, alias)
	if err := p2pServer.Start(); err != nil {
		log.Fatalf("Failed to start P2P sync server: %v", err)
	}

	// Start the IPC server for the local chat UI.
	ipcServer, err := ipc.StartIPCServer()
	if err != nil {
		log.Fatalf("Failed to start IPC server: %v", err)
	}
	_ = ipcServer

	// ── Boot successful ─────────────────────────────
	// Write a health file with the current timestamp.
	// CI and monitoring check for this file.
	writeHealth(healthFile)
	log.Println("[chatd] All subsystems started. Daemon healthy.")
	fmt.Println("NeoChat daemon running. Tailscale, sync, and autopilot active.")

	// ── Wait for shutdown signal ─────────────────────
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)
	sig := <-sigChan
	log.Printf("[chatd] Received signal %s. Shutting down cleanly.\n", sig)
	os.Remove(healthFile)
}

// safeGo wraps a goroutine with a panic-recovery that logs and restarts it.
// This prevents a single failing goroutine (e.g. a bad sync packet) from
// killing the entire daemon.
func safeGo(name string, fn func()) {
	for {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[chatd] goroutine '%s' panicked: %v\n%s",
						name, r, debug.Stack())
				}
			}()
			fn()
		}()
		log.Printf("[chatd] goroutine '%s' exited unexpectedly. Restarting in 2s.\n", name)
		time.Sleep(2 * time.Second)
	}
}

func writeHealth(path string) {
	content := fmt.Sprintf("ok pid=%d started=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339))
	_ = os.WriteFile(path, []byte(content), 0644)
}
