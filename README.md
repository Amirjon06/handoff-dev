# StateRelay

A high-performance, secure, peer-to-peer workspace orchestration engine built in Go. 

StateRelay atomizes and synchronizes ephemeral development environments across distinct physical machines. Instead of degrading productivity by manually committing unfinished work, StateRelay abstracts, packages, and cryptographically signs your entire runtime context—including Git state, dirty uncommitted files, IDE editor layouts, and terminal environments—and streams it securely over local networks.

## Features

- Go CLI for capture, restore, send, listen, and device trust
- JSON session files with SHA-256 content checks
- Signed sessions using local Ed25519 device identities
- Local network transfer over HTTP or HTTPS
- Optional mutual TLS for trusted devices
- mDNS device discovery
- SQLite history for received handoffs
- Restore from inbox, latest received session, or history ID
- VS Code extension for editor state capture and restore
- Terminal directory and browser URL capture
- Dry-run restore and conflict protection

## Architectural & Security Blueprint

StateRelay is engineered as a decoupled, multi-component distributed system:

* **Core Engine (Go):** Optimized CLI binary managing file system snapshots, compression, and network I/O.
* **Peer-to-Peer Topology (mDNS):** Implements zero-configuration local service discovery using multicast DNS, eliminating the need for a centralized control plane.
* **Zero-Trust Security Model:**
  * **Transport:** Mutual TLS (mTLS) ensuring end-to-end encryption and cryptographic identity verification between local nodes.
  * **Authentication:** Ed25519 public-key signatures validating session origins before execution.
  * **Data Integrity:** Strict SHA-256 content hashing to prevent payload corruption or tampering during transit.
* **State Management (SQLite):** Embedded transactional database capturing local handoff ledger history with strict ACID guarantees.
* **Orchestration Layer (TypeScript):** Deep VS Code API integration to serialize and deserialize UI state, active buffers, and cursor positions.

## Engineering Challenges & Deep Dives

### 1. Atomic State Restoration & Conflict Resolution
Restoring an uncommitted workspace onto a dirty target directory introduces the risk of state corruption. StateRelay resolves this by executing a multi-phase commit-like flow: evaluating local Git status, running dry-run differential checks, and implementing a strict `--conflict keep-both` isolation strategy to guarantee zero data loss.

### 2. High-Performance File Serialization
Streaming raw directory trees across local networks is bottlenecked by disk I/O and network overhead. StateRelay optimizes this by hashing file snapshots with SHA-256, indexing changes within a local SQLite instance, and only transmitting verified deltas.

## Install

Clone the repository:

```bash
git clone https://github.com/Amirjon06/handoff-dev.git
cd handoff-dev
```

Run the tests:

```bash
go test ./...
```

Check the CLI:

```bash
go run ./cmd/relay version
```

## Basic Usage

Capture the current workspace:

```bash
go run ./cmd/relay capture --path . --out session.json
```

Preview a restore:

```bash
go run ./cmd/relay restore --apply --dry-run session.json
```

Apply a restore:

```bash
go run ./cmd/relay restore --apply session.json
```

Start a receiver:

```bash
go run ./cmd/relay listen --addr 0.0.0.0:8765 --inbox .staterelay/inbox
```

Send a captured session:

```bash
go run ./cmd/relay send --to http://DESTINATION_IP:8765 session.json
```

Show received history:

```bash
go run ./cmd/relay history --history .staterelay/history.db
```

Restore a saved handoff from history:

```bash
go run ./cmd/relay restore --history .staterelay/history.db SESSION_ID
```

## Demo

See [docs/DEMO.md](docs/DEMO.md) for a full two-device walkthrough with exact commands.

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

## Safety

StateRelay validates repository state before restore. It checks the branch, commit, session structure, captured file hashes, and trusted signer settings when enabled.

Use dry-run mode before applying changes:

```bash
go run ./cmd/relay restore --apply --dry-run session.json
```

Use `--conflict keep-both` when you want incoming files preserved beside local changes.

## Project Status

StateRelay is at MVP v0.1.0. The core handoff path is implemented, tested, and usable from the CLI. Future work may add a background service, a desktop UI, and deeper editor/browser integrations.

## License

StateRelay is licensed under the [MIT License](LICENSE).

## Author

Amirjon Abdunayimov

[GitHub](https://github.com/Amirjon06) · [Portfolio](https://amirjonabd.com)
