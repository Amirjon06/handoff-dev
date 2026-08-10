# StateRelay

Continue a development workspace on another computer exactly where you stopped.

StateRelay is a cross-platform workspace handoff tool for developers. Instead of syncing only committed files, it captures enough context to rebuild your working environment on another machine:

- Git repository, remote, branch, and commit
- Open editor files and cursor positions
- Uncommitted work
- Browser tabs
- Terminal working directories

The first implementation milestone is intentionally small:

```bash
relay capture --path /path/to/project --out session.json
relay restore session.json
```

This repository starts with the product definition and roadmap first. The code should grow in small, reviewable milestones so the Git history shows how StateRelay was actually built.

## Why Build This

People who use multiple computers often waste time rebuilding context:

- finding the right repo
- checking out the right branch
- reopening editor files
- remembering cursor positions
- reopening docs and local app tabs
- moving uncommitted changes carefully

Git handles versioned source code well, but it does not preserve the surrounding development state. StateRelay fills that gap.

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

## Planned Commands

```bash
relay capture [--path PATH] [--out FILE] [--pretty]
relay restore SESSION_FILE
relay version
```

Example capture output:

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
    "repository": "GhostMirror",
    "remote": "git@github.com:amir/GhostMirror.git",
    "branch": "feature-login",
    "commit": "3b8e9f1...",
    "path": "/Users/amir/projects/GhostMirror",
    "dirty": true,
    "changed_files": [
      {
        "path": "main.py",
        "status": "modified"
      }
    ]
  }
}
```

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
