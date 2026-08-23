package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/NeoTrinity/chat/internal/ipc"
)

func main() {
	// Initialize IPC Server
	ipcServer, err := ipc.StartIPCServer()
	if err != nil {
		log.Fatalf("Failed to start IPC server: %v", err)
	}
	_ = ipcServer // hold reference

	fmt.Println("NeoChat daemon (chatd) is running in the background.")
	
	// Wait for termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("Shutting down chatd...")
}
