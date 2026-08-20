# StateRelay

StateRelay is a cross-platform workspace handoff tool for developers.

It captures the working state around a Git repository and moves it to another machine: branch, commit, uncommitted file snapshots, editor context, terminal directories, browser URLs, and trusted device metadata.

The goal is simple: switch computers without rebuilding your whole development context by hand.

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

## Tech Stack

- Go
- TypeScript
- Git
- SQLite
- mDNS
- TLS / mutual TLS
- GitHub Actions

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
