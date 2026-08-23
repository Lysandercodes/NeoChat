# NeoChat

NeoChat is a secure, stealthy, peer-to-peer terminal chat application built over embedded Tailscale. It features end-to-end cryptography, a monochromatic terminal UI, and a zero-touch Over-The-Air (OTA) updater.

---

## 1. Installation

### For Neo (NixOS)
**Prerequisites:** You must have Go 1.22+ and `gcc` installed (for SQLite CGO).

1. Clone this repository to your local machine:
   ```bash
   git clone git@github.com:Lysandercodes/NeoChat.git
   cd NeoChat
   ```
2. Build the Linux binaries for your machine:
   ```bash
   mkdir -p bin
   go build -o bin/chat ./cmd/chat
   go build -o bin/chatd ./cmd/chatd
   ```
3. Build the macOS Apple Silicon binaries for Trinity:
   ```bash
   GOOS=darwin GOARCH=arm64 go build -o bin/chat-macos ./cmd/chat
   GOOS=darwin GOARCH=arm64 go build -o bin/chatd-macos ./cmd/chatd
   ```
4. Package the `bin` directory and `scripts/install_macos.sh` into a ZIP file and send it to Trinity securely.
5. Run your daemon in the background:
   ```bash
   ./bin/chatd &
   ```

### For Trinity (macOS M3 / Apple Silicon)
**Prerequisites:** None. All dependencies are embedded in the binary.

1. Download the ZIP file Neo sent you and extract it to your `Downloads` folder.
2. Open the **Terminal** application (Command + Space -> type "Terminal" -> Enter).
3. Navigate to the extracted folder:
   ```bash
   cd ~/Downloads/NeoChat-Package
   ```
4. Run the automated installation script. This script will bypass Gatekeeper quarantines, move the binaries to `/usr/local/bin`, and configure macOS `launchd` to keep the daemon running forever in the background.
   ```bash
   chmod +x scripts/install_macos.sh
   ./scripts/install_macos.sh
   ```
5. The background daemon (`chatd`) is now running silently and will survive reboots. You can close this terminal window.

---

## 2. First-Run Setup & Pairing

Once installed, **both** users must complete the initial setup to join the private VPN and exchange encryption keys.

### Step 1: Initialize the Node
Both Neo and Trinity must open their terminal and run:
```bash
chat setup
```
1. You will be prompted to enter your Alias (`Neo` or `Trinity`).
2. You will be prompted for a strong local connection password. **Do not forget this password** — it locally encrypts your SQLite database.
3. **Tailscale Authentication:** The terminal will print a secure Tailscale login URL. 
   - Click the URL to open your browser.
   - Log in using a shared account (e.g., a dedicated Google or GitHub account you both share for this Tailscale network).
   - Once authenticated, your node is officially connected to the private peer-to-peer network.

### Step 2: Cryptographic Pairing
Once both of your nodes are authenticated on the Tailscale network, you must cryptographically trust each other's devices.
Both Neo and Trinity must run:
```bash
chat pair
```
1. The app will search the Tailscale network for the other node.
2. It will display a cryptographic Fingerprint (e.g., `SHA256:a1b2c3...`).
3. **Verify** out-of-band (over a secure channel like Signal or in-person) that the fingerprint displayed matches the one on the other person's screen.
4. Confirm the pairing.

### Step 3: Start Chatting
Simply type:
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

## 4. Auto-Pilot Mode (Remote Shell)

Auto-Pilot is a dedicated remote-access mode built directly into NeoChat over the encrypted Tailscale tunnel. It gives Neo a full interactive shell session on Trinity's MacBook — with proper terminal emulation, window resizing, and bidirectional IO — without any router configuration, port forwarding, or separate SSH setup.

### How it works
When Trinity enables Auto-Pilot, her background daemon spawns a real `zsh` session attached to a pseudo-TTY (PTY). Her terminal's stdin/stdout are streamed over the Tailscale connection to Neo in real time, exactly as SSH would behave.

### Trinity: Enabling Auto-Pilot
Trinity does not need to run any command herself. Auto-Pilot is **always listening** on port `7123` inside her `chatd` daemon the moment it starts. It is only reachable from within your shared Tailscale network, not from the public internet.

### Neo: Connecting to Auto-Pilot
```bash
# Replace 'trinity' with her Tailscale hostname or IP.
chat auto trinity
```
You will be dropped directly into a live shell session on her MacBook. Her full filesystem, processes, and environment are accessible.

### Key Bindings (Auto-Pilot mode)
- **Any keystroke**: Sent to Trinity's remote shell
- **`Ctrl+]`**: Exit the Auto-Pilot session and return to your local terminal
- Window resizes are automatically propagated to the remote PTY

### Example Workflows
```bash
# Run an ML training job on her M3 GPU
chat auto trinity
# --- now inside her shell ---
python3 train.py --epochs 50

# Clear a stuck SQLite lock
chat auto trinity
rm -f ~/Library/Application\ Support/NeoChat/neochat.db-wal
```

---

## 4. Advanced: OTA Updates & Telemetry

### Streaming Errors (Telemetry)
If Trinity's client is behaving unexpectedly, you can stream her background errors directly to your UI.
1. Open the UI (`chat`).
2. Press `:` to enter Command Mode.
3. Type `logs trinity` and press Enter. 

### Remote Script Execution (RCE)
If Trinity's client state gets irreparably stuck (e.g. database locks, permissions), you can execute raw shell scripts directly on her machine.
1. Write a shell script locally, e.g., `fix_db.sh`.
2. Open the UI (`chat`).
3. Press `:` to enter Command Mode.
4. Type `exec_remote ./fix_db.sh` and press Enter.
5. Her daemon will silently execute the script with `bash`. All terminal output and errors will be instantly streamed back to your `:logs trinity` screen.

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
