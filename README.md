# StateRelay

Cross-platform developer workspace handoff.

StateRelay lets you continue coding on another computer exactly where you stopped. The GitHub repository is named `handoff-dev`, while the product itself is called StateRelay.

## What It Does

StateRelay captures enough development context from one machine to recreate that workspace on another machine:

- Git repository name, remote, branch, and commit
- Open editor files and cursor positions
- Uncommitted work
- Browser tabs
- Terminal working directories

The goal is simple:

```text
MacBook
  |
  | relay send desktop
  v
Windows Desktop

same repo
same branch
same files
same cursor positions
same unfinished work
```

## Why It Exists

Developers who use multiple computers waste time rebuilding context every time they switch machines.

For example, moving from a laptop to a desktop often means manually:

- finding the right repo
- checking out the right branch
- reopening editor files
- remembering cursor positions
- reopening docs and local app tabs
- moving uncommitted changes carefully

Git handles committed source code well, but it does not preserve the surrounding development state. StateRelay fills that gap.

Think of it as Apple Handoff for developers, but across macOS, Windows, and Linux.

## Tech Stack

| Area | Technology | Why |
| --- | --- | --- |
| Core app | Go | Fast cross-platform CLI and background agent |
| Editor integration | TypeScript + VS Code API | Capture and restore editor state |
| Browser integration | TypeScript + browser extension APIs | Capture and reopen useful tabs |
| Session format | JSON | Portable, inspectable workspace metadata |
| Repository state | Git | Branch, commit, remote, and dirty file detection |
| Storage | SQLite | Local device, session, and transfer history |
| First transport | HTTP | Simple local-network transfer |
| Discovery | mDNS | Find nearby devices without typing IPs |
| Security | TLS + device pairing | Protect source code and trusted-device identity |

## Installation

StateRelay is in early development and is not packaged yet.

For now, build it from source:

```bash
git clone https://github.com/Amirjon06/handoff-dev.git
cd handoff-dev
go test ./...
go run ./cmd/relay version
```

Later releases should provide easier installation through package managers or install scripts.

## Usage

Current command:

```bash
go run ./cmd/relay version
go run ./cmd/relay capture --path .
go run ./cmd/relay capture --path . --json
go run ./cmd/relay capture --path . --out session.json
go run ./cmd/relay restore session.json
go run ./cmd/relay restore --apply --dry-run session.json
go run ./cmd/relay restore --apply session.json
```

Current capture output includes the Git repository root, repository name, origin remote, active branch, current commit, and dirty working tree status. Use `--json` to produce the first StateRelay session artifact. Restore rejects malformed session files before verifying that the captured Git root, branch, and commit match. Use `restore --apply --dry-run` to validate an apply without writing files. Use `restore --apply` to write captured text snapshots into a clean working tree; dirty destinations are rejected with the blocking file list, and uncaptured files are reported as skipped.

Planned commands:

```bash
relay capture [--path PATH] [--json]
relay capture [--path PATH] [--out FILE]
relay restore [--apply] [--dry-run] SESSION_FILE
relay version
```

Planned capture example:

```json
{
  "schema_version": 1,
  "captured_at": "2026-08-10T18:30:00Z",
  "source": {
    "hostname": "amir-macbook",
    "os": "darwin",
    "arch": "arm64"
  },
  "git": {
    "name": "GhostMirror",
    "remote": "git@github.com:amir/GhostMirror.git",
    "branch": "feature-login",
    "commit": "3b8e9f1...",
    "root": "/Users/amir/projects/GhostMirror",
    "dirty": true,
    "changed_files": [
      {
        "path": "main.py",
        "status": "modified",
        "size": 128,
        "content_captured": true,
        "content_encoding": "utf-8",
        "content": "def login():\n    pass\n"
      },
      {
        "path": "notes.txt",
        "status": "untracked",
        "size": 24,
        "content_captured": true,
        "content_encoding": "utf-8",
        "content": "remember auth edge case\n"
      }
    ]
  }
}
```

## Development Status

This repository is being built step by step. The early commits intentionally start with the problem statement, architecture, and a minimal Go CLI before adding capture, restore, networking, and editor integration.

## Roadmap

1. Capture Git repository, branch, commit, remote, and dirty files.
2. Define a stable session JSON schema.
3. Build a VS Code extension that captures open files, active file, and cursor position.
4. Merge editor state into `relay capture`.
5. Restore a captured session on the same computer.
6. Send a session over HTTP between two computers on the same network.
7. Restore a session on the destination computer.
8. Clone missing repositories automatically from the captured Git remote.
9. Transfer uncommitted work.
10. Add conflict detection instead of overwriting local changes.
11. Add mDNS device discovery with `relay devices`.
12. Add device pairing and trusted identities.
13. Encrypt transfers with TLS.
14. Persist sessions, devices, and transfer history in SQLite.
15. Add browser-tab handoff.
16. Add terminal-directory handoff.
17. Support Linux alongside macOS and Windows.
18. Run the Go agent as a background service.
19. Add a simple desktop UI that talks to the same Go backend.
20. Add cross-platform tests, CI, and installers.

## Planned Repository Layout

```text
cmd/relay/          CLI entry point
internal/gitstate/  Git capture logic
internal/session/   Session schema and JSON helpers
extensions/vscode/  Future VS Code extension
extensions/browser/ Future browser extension
docs/               Architecture and roadmap notes
```

## License

StateRelay is licensed under the MIT License. See [LICENSE](LICENSE).
