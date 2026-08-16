# StateRelay

**Cross-platform distributed workspace state synchronization for developers.**

StateRelay captures the development state surrounding a Git repository and transfers it between machines, allowing developers to resume work with the same repository, branch, uncommitted changes, editor context, terminal locations, and browser state.

Built with **Go, TypeScript, Git, SQLite, mDNS, and TLS**, StateRelay combines portable session artifacts, local-network discovery, authenticated device communication, and editor integration into a cross-platform workspace handoff system.

---

## The Problem

Git synchronizes committed source code extremely well.

It does not synchronize the complete state of an active development environment.

When moving from one machine to another, developers still have to reconstruct:

* The correct repository and branch
* Uncommitted work
* Open editor files
* Cursor and selection positions
* Terminal working directories
* Browser tabs and documentation
* Local workspace context

StateRelay treats this surrounding context as **portable workspace state**.

```text
Source Machine                         Destination Machine

Git repository ───────┐                ┌────── Git repository
Branch + commit ──────┤                ├────── Branch + commit
Uncommitted files ────┤                ├────── Uncommitted files
Editor state ─────────┤   StateRelay   ├────── Editor state
Terminal state ───────┼───────────────>├────── Terminal state
Browser state ────────┤                ├────── Browser state
Device identity ──────┘                └────── Trusted restore
```

---

## Core Capabilities

### Workspace State Capture

The Go CLI captures repository and workspace metadata into portable JSON session artifacts, including:

* Repository name and root
* Git remote
* Active branch
* Current commit
* Dirty working-tree state
* Uncommitted text-file snapshots
* Editor state
* Terminal working directories
* Browser URLs

Captured file contents include SHA-256 hashes for integrity validation.

### Cross-Machine Transfer

StateRelay provides a local-network handoff layer supporting:

* HTTP and HTTPS transfer
* mDNS / DNS-SD device discovery
* Device fingerprints
* Ed25519 identities
* Signed session artifacts
* Mutual TLS client authentication
* Trusted-device enforcement
* Pair verification codes

This allows StateRelay-capable machines on the same network to discover one another and exchange workspace session metadata without requiring a centralized cloud service.

### Safe State Restoration

Before modifying a destination workspace, StateRelay validates the incoming session and target repository.

Restore functionality includes:

* Repository validation
* Branch and commit validation
* Missing-repository cloning
* Session-signature verification
* SHA-256 file-integrity verification
* Dirty-working-tree protection
* Dry-run restoration
* Trusted-device requirements
* Conflict preservation with `keep-both`
* Restoration from the newest received session

### VS Code Integration

The TypeScript VS Code extension captures development context including:

* Workspace folder
* Active file
* Open text documents
* Dirty-file state
* Cursor and selection positions

Captured editor metadata is incorporated into StateRelay session artifacts and can be restored on the destination workspace.

### Terminal & Browser Context

StateRelay records terminal working directories and browser URLs alongside repository state.

Terminal state can be restored as shell navigation commands, while captured browser URLs can be reopened using the system browser.

### Session History

Received handoffs are indexed locally using **SQLite**, providing searchable metadata about previous sessions including repository, branch, commit, dirty state, and changed-file counts.

---

## Tech Stack

| Component                  | Technology               |
| -------------------------- | ------------------------ |
| Core CLI & transfer system | Go                       |
| Editor integration         | TypeScript + VS Code API |
| Repository state           | Git                      |
| Session representation     | JSON                     |
| Local persistence          | SQLite                   |
| Device discovery           | mDNS / DNS-SD            |
| Device identity            | Ed25519                  |
| Integrity verification     | SHA-256                  |
| Secure transport           | TLS / mutual TLS         |
| Network transport          | HTTP / HTTPS             |
| CI                         | GitHub Actions           |
| Target platforms           | Linux, macOS, Windows    |

---

## Quick Start

### Clone

```bash
git clone https://github.com/Amirjon06/handoff-dev.git
cd handoff-dev
```

### Test

```bash
go test ./...
```

### Verify the CLI

```bash
go run ./cmd/relay version
```

### Capture a workspace

```bash
go run ./cmd/relay capture --path . --out session.json
```

### Preview a restore

```bash
go run ./cmd/relay restore --apply --dry-run session.json
```

### Restore

```bash
go run ./cmd/relay restore --apply session.json
```

---

## Cross-Machine Handoff

### Destination

Start a StateRelay listener:

```bash
go run ./cmd/relay listen \
  --addr 0.0.0.0:8765 \
  --inbox .staterelay/inbox \
  --advertise \
  --path .
```

### Source

Discover available StateRelay devices:

```bash
go run ./cmd/relay devices --timeout 2s
```

Check connectivity:

```bash
go run ./cmd/relay ping --to http://DESTINATION_IP:8765
```

Send the captured workspace:

```bash
go run ./cmd/relay send \
  --to http://DESTINATION_IP:8765 \
  session.json
```

### Destination

Inspect received sessions:

```bash
go run ./cmd/relay inbox --inbox .staterelay/inbox
```

Restore the latest session:

```bash
go run ./cmd/relay restore \
  --inbox .staterelay/inbox \
  --apply \
  latest
```

---

## Device Identity & Trust

Create an Ed25519 identity:

```bash
go run ./cmd/relay identity --path . --name my-laptop
```

Trust another device:

```bash
go run ./cmd/relay trust add \
  --path . \
  --name desktop \
  --fingerprint <DEVICE_FINGERPRINT>
```

Verify the pairing code:

```bash
go run ./cmd/relay pair-code \
  --path . \
  --peer-fingerprint <DEVICE_FINGERPRINT>
```

Require trusted sessions during restoration:

```bash
go run ./cmd/relay restore \
  --require-trusted \
  session.json
```

---

## Mutual TLS

StateRelay can require authenticated clients when receiving workspace state.

```bash
go run ./cmd/relay listen \
  --addr 0.0.0.0:8765 \
  --inbox .staterelay/inbox \
  --advertise \
  --tls \
  --require-client-cert \
  --path .
```

A trusted source can connect using its StateRelay client certificate:

```bash
go run ./cmd/relay send \
  --to https://DESTINATION_IP:8765 \
  --client-cert \
  --path . \
  session.json
```

---

## VS Code Extension

Build the extension:

```bash
cd extensions/vscode
npm install
npm run compile
```

Available commands:

```text
StateRelay: Capture Editor State
StateRelay: Restore Editor State
```

Editor state is stored locally under:

```text
.staterelay/editor-state.json
```

and incorporated into StateRelay session artifacts.

---

## Restore Safety

StateRelay is intentionally conservative when applying remote workspace state.

Run a dry restore:

```bash
go run ./cmd/relay restore --apply --dry-run session.json
```

Preserve conflicting versions:

```bash
go run ./cmd/relay restore \
  --apply \
  --conflict keep-both \
  session.json
```

StateRelay validates session structure, repository state, signatures, and captured content hashes before applying workspace changes.

---

## Architecture

```text
             Workspace Sources
                    │
        ┌───────────┼───────────┐
        │           │           │
       Git       VS Code     Terminal/
        │        Extension     Browser
        │           │           │
        └───────────┼───────────┘
                    ▼
             ┌─────────────┐
             │ StateRelay  │
             │   Go CLI    │
             └──────┬──────┘
                    │
                    ▼
             JSON Session
                    │
             Ed25519 Signature
                    │
                    ▼
             Device Discovery
                 (mDNS)
                    │
                    ▼
              HTTP / HTTPS
                  + mTLS
                    │
                    ▼
          Destination StateRelay
                    │
             ┌──────┴──────┐
             │             │
           SQLite        Inbox
           History       Sessions
             │             │
             └──────┬──────┘
                    ▼
                  Restore
```

---

## Development Status

StateRelay currently implements the core workspace-handoff pipeline:

```text
Capture
   ↓
Serialize
   ↓
Sign & Validate
   ↓
Discover Destination
   ↓
Transfer
   ↓
Persist Session
   ↓
Validate Destination
   ↓
Restore
```

The project remains under active development. Current work is focused on expanding the handoff experience beyond the CLI and making cross-device synchronization increasingly automatic.

---

## Roadmap

* Richer device-pairing UX
* Expanded browser integration
* Expanded terminal integration
* Background service/agent
* Richer workspace history
* Desktop interface
* Installer packaging
* Package-manager distribution
* Additional cross-platform testing and integration

---

## License

StateRelay is licensed under the [MIT License](LICENSE).

---

## Author

**Amirjon Abdunayimov**

[GitHub](https://github.com/Amirjon06) · [Portfolio](https://amirjonabd.com)

---

<p align="center">
  <strong>Move computers. Keep your context.</strong>
</p>
