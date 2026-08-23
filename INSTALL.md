# NeoChat Installation & Pairing Guide

Welcome to NeoChat. This guide covers how to install, configure, and pair NeoChat between NixOS and macOS.

## Prerequisites
- Both devices must have internet access.
- NeoChat utilizes an embedded Tailscale node (`tsnet`), so you **do not** need the OS-level Tailscale app installed, but you do need an active Tailscale network/account to authenticate the nodes to.

---

## 1. Installation

### NixOS (Neo)
You can install NeoChat directly from the repository using Nix flakes:

```bash
nix profile install github:NeoTrinity/chat
```
*(Alternatively, clone the repository and run `go build -o chat ./cmd/chat` and `go build -o chatd ./cmd/chatd`)*

Next, enable the background service:
```bash
# Assuming the Nix derivation provides the systemd unit:
systemctl --user enable --now neochatd.service
```

### macOS (Trinity)
NeoChat is distributed outside the Mac App Store. 
1. Download the `NeoChat.pkg` or the binary.
2. If macOS warns you that the app is from an "unidentified developer" (Gatekeeper):
   - Open **System Settings** > **Privacy & Security**.
   - Scroll down and click **Open Anyway** next to NeoChat.
   - Alternatively, from the terminal, remove the quarantine attribute:
     ```bash
     xattr -d com.apple.quarantine /path/to/chat
     xattr -d com.apple.quarantine /path/to/chatd
     ```
3. Run the setup script to install the `launchd` background service:
   ```bash
   ./scripts/install_macos.sh
   ```
   This will register `chatd` to start automatically in the background.

---

## 2. First-Run Setup & Pairing

Once installed, the setup process is entirely keyboard-driven.

### Step A: Setup
On **both** devices, run:
```bash
chat setup
```
1. Enter your Alias (e.g., `Neo` or `Trinity`).
2. Enter your connection password (used to encrypt local storage and the P2P connection vault).
3. The embedded Tailscale node will initialize and provide an authentication URL. **Open the provided link in your browser to authorize the device on your Tailscale network.**

### Step B: Pairing
Once both devices are authenticated to Tailscale, you need to pair them cryptographically.

On **Neo's** machine:
```bash
chat pair
```
It will output your local fingerprint, e.g., `7F3A 91BC 22D1...`

On **Trinity's** machine:
```bash
chat pair
```
It will output her fingerprint.

In the pairing UI, enter the other person's Tailscale IP or machine name, and **verify the fingerprint** matches exactly what they see on their screen.

Press `Enter` to trust the peer.

### Step C: Start Chatting
Once paired, simply run:
```bash
chat
```
The UI will open, synchronize securely over the background Tailscale connection, and you're ready to communicate.

## Usage Overview
- **j / k**: Navigate messages
- **i / Enter**: Compose message
- **a**: Attach file
- **/**: Search
- **Ctrl+\**: Emergency UI close (leaves connection active in background)
