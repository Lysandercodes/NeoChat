package main

import (
	"fmt"
	"os"

	"github.com/NeoTrinity/chat/internal/tui"
)

func main() {
	if err := tui.StartTUI(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running chat UI: %v\n", err)
		os.Exit(1)
	}
}
