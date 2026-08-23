#!/bin/bash
set -e

echo "Installing NeoChat Daemon for macOS..."

INSTALL_DIR="/usr/local/bin"
PLIST_PATH="$HOME/Library/LaunchAgents/com.neotrinity.chatd.plist"

mkdir -p "$INSTALL_DIR"
cp ../bin/chat "$INSTALL_DIR/chat"
cp ../bin/chatd "$INSTALL_DIR/chatd"

cat <<EOF > "$PLIST_PATH"
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple Computer//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.neotrinity.chatd</string>
    <key>ProgramArguments</key>
    <array>
        <string>$INSTALL_DIR/chatd</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/chatd.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/chatd.err</string>
</dict>
</plist>
EOF

launchctl load -w "$PLIST_PATH"

echo "NeoChat installed successfully. Run 'chat' to start."
