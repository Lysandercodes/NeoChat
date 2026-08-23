# NeoChat

NeoChat is a secure, stealthy, peer-to-peer terminal chat application built over embedded Tailscale. It features end-to-end cryptography, a monochromatic terminal UI, and a zero-touch Over-The-Air (OTA) updater.

---

## 1. Installation

### For Neo (NixOS)
1. Ensure your NixOS environment is configured for Go development.
2. Clone this repository:
   ```bash
   git clone git@github.com:Lysandercodes/NeoChat.git
   cd NeoChat
   ```
3. Build the binaries:
   ```bash
   go build -o bin/chat ./cmd/chat
   go build -o bin/chatd ./cmd/chatd
   ```
4. Run the daemon in the background (or configure a systemd user service for `chatd`).

### For Trinity (macOS M3 / Apple Silicon)
1. Neo will send you the compiled binaries or package. Download them to a folder.
2. Open your Terminal and run the installation script:
   ```bash
   chmod +x scripts/install_macos.sh
   ./scripts/install_macos.sh
   ```
   *Note: If macOS prevents the app from running due to an "unidentified developer," you may need to go to System Settings > Privacy & Security and click "Open Anyway" for NeoChat.*
3. The background daemon (`chatd`) is now running silently and will survive reboots via `launchd`.

---

## 2. First-Run Setup & Pairing

Once installed, **both** users must complete the initial setup.

1. **Setup Node:**
   ```bash
   chat setup
   ```
   - Enter your Alias (`Neo` or `Trinity`).
   - Enter a strong local connection password (this encrypts your local database).
   - **Important:** An authentication URL will be printed. Click it to authorize this embedded Tailscale node to your private Tailscale network.

2. **Pair Devices:**
   Once both nodes are on the Tailscale network, you must cryptographically trust each other.
   ```bash
   chat pair
   ```
   Exchange fingerprints and confirm the pairing.

3. **Start Chatting:**
   ```bash
   chat
   ```

---

## 3. UI Controls
The interface is strictly monochromatic.
- `j / k`: Navigate messages
- `i / Enter`: Compose message
- `a`: Attach file
- `/`: Search history
- `:`: Enter Command Mode
- `Ctrl+\`: Emergency Stealth Close (Instantly closes UI, leaves connection active)

---

## 4. Advanced: OTA Updates & Telemetry

### Streaming Errors (Telemetry)
If Trinity's client is behaving unexpectedly, you can stream her background errors directly to your UI.
1. Open the UI (`chat`).
2. Press `:` to enter Command Mode.
3. Type `logs trinity` and press Enter. 

### Zero-Touch Updates
Once you identify a bug and compile a fix, you can push the update to Trinity without her needing to use the terminal.
1. Compile the new Apple Silicon binaries:
   ```bash
   GOOS=darwin GOARCH=arm64 go build -o bin/chat-macos ./cmd/chat
   GOOS=darwin GOARCH=arm64 go build -o bin/chatd-macos ./cmd/chatd
   ```
2. Open the UI (`chat`).
3. Press `:` to enter Command Mode.
4. Type `push_update ./bin/chat-macos ./bin/chatd-macos v1.1` and press Enter.
5. Her daemon will silently receive the payload, replace its files, and instantly self-restart.
