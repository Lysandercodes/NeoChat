package main

import (
	"fmt"
	"os"

	"github.com/NeoTrinity/chat/internal/autopilot"
	"github.com/NeoTrinity/chat/internal/tui"
)

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "auto" {
		// Autopilot mode: chat auto <peer-tailscale-hostname-or-ip>
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: chat auto <tailscale-peer-address>")
			fmt.Fprintln(os.Stderr, "Example: chat auto trinity")
			os.Exit(1)
		}
		peerAddr := os.Args[2]
		if err := autopilot.Connect(peerAddr); err != nil {
			fmt.Fprintf(os.Stderr, "Autopilot error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Normal mode: launch the Bubbletea chat UI.
	if err := tui.StartTUI(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running chat UI: %v\n", err)
		os.Exit(1)
	}
}
