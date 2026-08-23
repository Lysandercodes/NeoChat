package autopilot

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"
)

const AutopilotPort = 7123

// Frame wraps data for the autopilot wire protocol.
type Frame struct {
	Type    string `json:"type"` // "data", "resize", "keepalive"
	Payload []byte `json:"payload"`
	Rows    uint16 `json:"rows,omitempty"`
	Cols    uint16 `json:"cols,omitempty"`
}

// ─────────────────────────────────────────────
//  LISTENER SIDE (Trinity's machine)
//  Spawns a shell attached to a pseudo-TTY and
//  bridges its IO to the incoming TCP connection.
// ─────────────────────────────────────────────

// ListenAndServe waits for autopilot connections on the given listener.
// The caller is responsible for binding the listener to the Tailscale-only
// interface so it is never reachable from the public internet or local LAN.
func ListenAndServe(ln net.Listener) {
	log.Println("[autopilot] Waiting for Neo to connect on Tailscale...")
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("[autopilot] Accept error: %v\n", err)
			time.Sleep(time.Second)
			continue
		}
		log.Printf("[autopilot] Neo connected from %s\n", conn.RemoteAddr())
		go serveShell(conn)
	}
}

func serveShell(conn net.Conn) {
	defer conn.Close()

	// Detect shell: prefer zsh on macOS, fall back to bash.
	shell := "/bin/zsh"
	if _, err := os.Stat(shell); os.IsNotExist(err) {
		shell = "/bin/bash"
	}

	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	// Allocate a PTY — this is what makes it a real interactive session.
	ptmx, err := pty.Start(cmd)
	if err != nil {
		log.Printf("[autopilot] Failed to start PTY: %v\n", err)
		return
	}
	defer ptmx.Close()

	// Forward SIGWINCH so resize events from Neo propagate to the shell.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			_ = pty.InheritSize(os.Stdin, ptmx)
		}
	}()
	ch <- syscall.SIGWINCH // set initial size

	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)

	// PTY output → conn (shell output back to Neo)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				f := Frame{Type: "data", Payload: buf[:n]}
				if encErr := enc.Encode(f); encErr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// conn input → PTY (Neo's keystrokes and resize events)
	for {
		var f Frame
		if err := dec.Decode(&f); err != nil {
			if err != io.EOF {
				log.Printf("[autopilot] Decode error: %v\n", err)
			}
			break
		}
		switch f.Type {
		case "data":
			ptmx.Write(f.Payload)
		case "resize":
			_ = pty.Setsize(ptmx, &pty.Winsize{Rows: f.Rows, Cols: f.Cols})
		case "keepalive":
			// no-op
		}
	}

	cmd.Process.Kill()
	log.Println("[autopilot] Shell session ended.")
}

// ─────────────────────────────────────────────
//  CLIENT SIDE (Neo's machine)
//  Dials Trinity's autopilot listener over
//  Tailscale and bridges Neo's raw terminal.
// ─────────────────────────────────────────────

// Connect dials the remote autopilot listener and runs an interactive session.
func Connect(addr string) error {
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", addr, AutopilotPort))
	if err != nil {
		return fmt.Errorf("autopilot: dial failed: %w", err)
	}
	defer conn.Close()

	fmt.Printf("\n[autopilot] Connected to %s. Press Ctrl+] to exit.\n\n", addr)

	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)

	// Put Neo's terminal in raw mode using the portable x/term package.
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("autopilot: failed to set raw terminal: %w", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	// Send initial window size.
	if rows, cols, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
		_ = enc.Encode(Frame{Type: "resize", Rows: uint16(rows), Cols: uint16(cols)})
	}

	// Forward SIGWINCH (terminal resize) to the remote PTY.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	go func() {
		for range sigCh {
			if rows, cols, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
				_ = enc.Encode(Frame{Type: "resize", Rows: uint16(rows), Cols: uint16(cols)})
			}
		}
	}()

	done := make(chan struct{})

	// conn → stdout (shell output from Trinity's machine)
	go func() {
		defer close(done)
		for {
			var f Frame
			if err := dec.Decode(&f); err != nil {
				return
			}
			if f.Type == "data" {
				os.Stdout.Write(f.Payload)
			}
		}
	}()

	// stdin → conn (Neo's keystrokes; Ctrl+] = 0x1D exits)
	go func() {
		buf := make([]byte, 256)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				for _, b := range buf[:n] {
					if b == 0x1D { // Ctrl+]
						conn.Close()
						return
					}
				}
				_ = enc.Encode(Frame{Type: "data", Payload: buf[:n]})
			}
			if err != nil {
				return
			}
		}
	}()

	<-done
	fmt.Println("\n[autopilot] Session closed.")
	return nil
}
