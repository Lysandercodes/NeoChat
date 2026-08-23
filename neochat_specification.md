# NeoChat — Specification v1.0 (Draft)

## 1. Identity & Architecture

| Device  | OS    | Alias     |
| ------- | ----- | --------- |
| Neo     | NixOS | `Neo`     |
| Trinity | macOS | `Trinity` |

**Architecture Diagram**
```text
Neo / NixOS                         Trinity / macOS
┌──────────────┐                   ┌──────────────┐
│  chat UI     │                   │  chat UI     │
└──────┬───────┘                   └──────┬───────┘
       │                                  │
┌──────▼───────┐     Tailscale/TCP   ┌────▼─────────┐
│    chatd     │◄───────────────────►│    chatd     │
└──────┬───────┘                     └────┬──────────┘
       │                                  │
   SQLite DB                          SQLite DB
       │                                  │
       └──────── automatic sync ──────────┘
```

Both users interact with NeoChat entirely through a Terminal interface (`chat`), powered by a background daemon (`chatd`). There is no GUI client. 

`chatd` handles:
* Automatic Tailscale-based connection
* End-to-end (E2E) encryption & decryption
* Background synchronization
* Local SQLite database management
* File reception & transfers
* Native background notifications
* Backups (on Neo's device)

---

## 2. Installation & Setup

### Neo — NixOS
```bash
nix profile install github:NeoTrinity/chat
chat setup
chat pair
```

### Trinity — macOS
Install the application/package once (which installs `chat`, `chatd`, launchd service, and notification resources).
```bash
chat setup
chat pair
```

### First-Run Setup & Pairing
Entirely keyboard-driven setup wizard prompting for:
* Alias
* Connection Password

To pair, users run `chat pair`, which displays their local public-key fingerprint.
Users verify and trust each other's fingerprints to establish the encrypted session.

---

## 3. UI Modes & Interface

The UI is built on a modal paradigm, similar to terminal editors (e.g., Vim).

* **NORMAL mode**: Navigate, read messages, perform quick actions.
* **INSERT mode**: Compose and edit messages.
* **COMMAND mode**: Execute less frequent application commands (e.g., `:sync`, `:settings`).

**Interface Layout**
```text
┌──────────────────────────────────────────────────────┐
│ Trinity ● online                         SYNCED ✓    │
├──────────────────────────────────────────────────────┤
│                                                      │
│  Trinity  21:31                                      │
│  Are you awake?                                      │
│                                                      │
│  Neo      21:32                                      │
│  Yeah                                                │
│                                                      │
├──────────────────────────────────────────────────────┤
│ INSERT │ > I'm here_                                 │
└──────────────────────────────────────────────────────┘
```

---

## 4. Keybindings & Navigation

Single-key commands are prioritized for speed. The keybindings below are the defaults, configurable via `:settings`.

### NORMAL Mode
| Key | Action |
| --- | --- |
| `j` / `k` | Message down / up |
| `h` / `l` | Older history / newer history |
| `g` / `G` | Oldest message / Newest message |
| `Ctrl+d` / `Ctrl+u` | Half page down / up |
| `Ctrl+f` / `Ctrl+b` | Full page down / up |
| `n` / `N` | Next / Previous unread message |
| `i` / `Enter` | Enter INSERT mode (compose) |
| `a` | Attach file |
| `/` | Search |
| `:` | Enter COMMAND mode |
| `?` | Open Help screen |
| `q` | Quit UI |
| `Ctrl+\` | **Emergency UI exit** (kills UI instantly, leaves daemon running) |

### INSERT Mode
| Key | Action |
| --- | --- |
| `Enter` | Send message |
| `Shift+Enter` | New line (for multiline drafts) |
| `Esc` | Return to NORMAL mode |
| `Ctrl+w` / `Ctrl+u`| Delete previous word / Clear line |
| `Ctrl+a` / `Ctrl+e`| Move to beginning / end of line |
| `Alt+←` / `Alt+→` | Move to previous / next word |
| `↑` / `↓` | Navigate draft history |

### COMMAND Mode
Triggered by `:`. Used for rare operations.
Examples: `:help`, `:settings`, `:status`, `:sync`, `:reset`, `:backup`, `:quit`

---

## 5. Message Actions & Editing

In NORMAL mode, pressing `Enter` on a selected message opens the action menu:

| Key | Action | Details |
| --- | --- | --- |
| `e` | Edit | Opens message in editor. `Ctrl+Enter` to save, `Esc` to cancel. Displays `[edited]`. |
| `d` | Delete | Prompts for confirmation. Deletes on both devices. No "message deleted" placeholder is left behind. |
| `r` | Reply | Quotes the message for context. |
| `c` | Copy | Copies message text to system clipboard. |
| `f` | Download | Saves attachment to `~/Downloads/`. |
| `q` / `Esc` | Close | Closes the action menu. |

---

## 6. Files & Attachments

* Press `a` in NORMAL mode to open the terminal file picker.
* **Navigation**: `j`/`k` (up/down), `h`/`l` (parent/enter directory), `Space` (select), `Enter` (confirm), `/` (search), `Esc` (cancel).
* Files are sent as attachments only and display inline as: `[file] report.pdf · 2.4 MB`.
* Press `f` on an attachment message to download it natively to the OS `Downloads` folder.
* No inline image/file rendering is performed.

---

## 7. Search & History

* **Search**: Press `/` to search local history across messages, dates, senders, and metadata.
  * `Enter` (open result), `n` / `N` (next/prev result), `Esc` (exit).
* **History**: Treated as an infinite scroll. Moving up (`k`) lazily loads older messages from SQLite.

---

## 8. Presence, Delivery & Notifications

### Connection & Sync
Status is visible in the header (e.g., `● online`, `↻ syncing`, `! error`).
Sync is automatic. If offline, messages are queued locally and sync when connection returns.

### Typing & Delivery
* **Typing Indicator**: Ephemeral, shown as `Trinity is typing...` (not stored).
* **Delivery Status**:
  * `✓` : Sent
  * `✓✓` : Delivered
  * `✓✓*`: Read (UI specific glyph to be determined)

### Notifications
`chatd` periodically checks for messages at fixed intervals (e.g., top of the hour).
Notifications are generic and obfuscated (no sender, no message snippet, normal-looking OS app identity) to protect privacy.

---

## 9. Encryption, Storage & Backups

**Storage Paths**:
```text
~/.chat/
├── db/chat.db
├── storage/encrypted-files/
├── backups/               # Versioned snapshots (Neo only)
├── keys/                  # Cryptographic identities
└── config/config.toml
```

**E2E Encryption**:
Covers message contents, metadata, files/filenames, database, local storage, and backups. The connection password is used to unlock the vault, but is not the encryption key itself.

**Backups**:
Neo's machine retains full versioned encrypted backups of the history. If a message is deleted and vanishes from the live DB, the historical snapshot retains it.
