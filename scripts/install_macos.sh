#!/bin/bash
# NeoChat macOS Installer
# Run this once on Trinity's MacBook. Everything after is zero-touch.

set -euo pipefail

INSTALL_DIR="/usr/local/bin"
PLIST_LABEL="com.neotrinity.chatd"
PLIST_PATH="$HOME/Library/LaunchAgents/${PLIST_LABEL}.plist"
LOG_DIR="$HOME/.config/neochat"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_DIR="$(cd "${SCRIPT_DIR}/../bin" && pwd)"

# ── Preflight ────────────────────────────────────────────────────────────────
echo "NeoChat installer starting..."

if [[ "$(uname -m)" != "arm64" ]]; then
  echo "WARNING: This installer is for Apple Silicon (arm64). Your arch: $(uname -m)"
  echo "         The pre-built binaries may not work. Proceed anyway? [y/N]"
  read -r reply
  [[ "$reply" =~ ^[Yy]$ ]] || exit 1
fi

# ── Install binaries ─────────────────────────────────────────────────────────
echo "Installing binaries to $INSTALL_DIR..."
sudo mkdir -p "$INSTALL_DIR"

for binary in chat chatd; do
  if [[ ! -f "${BIN_DIR}/${binary}-macos" ]]; then
    echo "ERROR: ${BIN_DIR}/${binary}-macos not found."
    echo "       Please build it first: GOOS=darwin GOARCH=arm64 go build -o bin/${binary}-macos ./cmd/${binary}"
    exit 1
  fi

  # Strip Gatekeeper quarantine xattr before installing
  xattr -d com.apple.quarantine "${BIN_DIR}/${binary}-macos" 2>/dev/null || true

  sudo install -m 0755 "${BIN_DIR}/${binary}-macos" "${INSTALL_DIR}/${binary}"
  echo "  Installed: ${INSTALL_DIR}/${binary}"
done

# ── Create log and config directory ─────────────────────────────────────────
mkdir -p "$LOG_DIR"

# ── Unload previous version if running ──────────────────────────────────────
if launchctl list | grep -q "$PLIST_LABEL" 2>/dev/null; then
  echo "Stopping existing NeoChat daemon..."
  launchctl unload "$PLIST_PATH" 2>/dev/null || true
fi

# ── Write launchd plist ──────────────────────────────────────────────────────
# ThrottleInterval: if chatd crashes within 10 seconds of starting,
# launchd waits 10 seconds before retrying. Prevents 100% CPU spin
# on a startup bug and lets us diagnose via the log file.
mkdir -p "$(dirname "$PLIST_PATH")"
cat > "$PLIST_PATH" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple Computer//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${PLIST_LABEL}</string>

    <key>ProgramArguments</key>
    <array>
        <string>${INSTALL_DIR}/chatd</string>
    </array>

    <!-- NEOCHAT_ALIAS tells chatd which identity to use on Tailscale -->
    <key>EnvironmentVariables</key>
    <dict>
        <key>NEOCHAT_ALIAS</key>
        <string>Trinity</string>
        <key>HOME</key>
        <string>${HOME}</string>
    </dict>

    <key>RunAtLoad</key>
    <true/>

    <!-- Keep daemon alive after crashes -->
    <key>KeepAlive</key>
    <true/>

    <!-- Wait 10s before restarting after a crash. Prevents CPU spin
         on a boot-loop and gives the log file time to flush. -->
    <key>ThrottleInterval</key>
    <integer>10</integer>

    <!-- Pipe output to the state dir so you can tail it via autopilot -->
    <key>StandardOutPath</key>
    <string>${LOG_DIR}/chatd.log</string>
    <key>StandardErrorPath</key>
    <string>${LOG_DIR}/chatd.err</string>
</dict>
</plist>
EOF

echo "LaunchAgent plist written to $PLIST_PATH"

# ── Load the daemon ──────────────────────────────────────────────────────────
launchctl load -w "$PLIST_PATH"
echo "Daemon loaded into launchd."

# ── Health check ─────────────────────────────────────────────────────────────
# Wait up to 15 seconds for chatd to write its health file.
echo "Waiting for daemon to become healthy..."
HEALTH_FILE="/tmp/neochat_health"
for i in $(seq 1 15); do
  if [[ -f "$HEALTH_FILE" ]]; then
    echo "  Daemon is healthy: $(cat "$HEALTH_FILE")"
    break
  fi
  sleep 1
  if [[ $i -eq 15 ]]; then
    echo "  WARNING: Daemon did not report healthy within 15 seconds."
    echo "  Check logs: cat ${LOG_DIR}/chatd.log"
    echo "  Check errors: cat ${LOG_DIR}/chatd.err"
  fi
done

echo ""
echo "────────────────────────────────────────────"
echo "  NeoChat installed successfully."
echo "  Run 'chat' to open the messaging interface."
echo "────────────────────────────────────────────"
