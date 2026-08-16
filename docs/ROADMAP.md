# Roadmap

## Phase 1: Core CLI

Goal: create a real `relay` command.

- `relay capture`
- `relay restore`
- JSON session format
- useful error messages

## Phase 2: Git Capture

Goal: describe the current codebase.

- repository root
- repository name
- Git remote
- current branch
- current commit
- dirty state
- changed files

## Phase 3: VS Code Extension

Goal: capture editor state.

- current workspace
- open files
- active file
- cursor line and column
- command for restoring files and cursor

## Phase 4: Local Restore

Goal: prove StateRelay can rebuild a workspace on one machine.

- read `session.json`
- find or clone repository
- switch branch
- open VS Code
- reopen files
- restore cursor

## Phase 5: Network Handoff

Goal: move one session between two machines.

- `relay listen`
- `relay ping --to URL`
- `relay send --to URL`
- HTTP transport with validated session storage
- inbox listing for received sessions
- destination restore from the latest received session
- clone missing repositories during restore
- setup diagnostics with `relay doctor`

## Phase 6: Uncommitted Work

Goal: move unfinished code safely.

- detect modified and untracked files
- transfer file content or patch data
- verify hashes
- refuse unsafe overwrites

## Phase 7: Conflicts

Goal: protect destination changes.

- detect when source and destination both changed the same file
- offer conflict strategies
- keep both versions when uncertain
- preserve local files with `--conflict keep-both`

## Phase 8: Device Discovery and Pairing

Goal: make the network flow usable and trustworthy.

- `relay devices`
- mDNS discovery through advertised listeners
- local Ed25519 device identity
- verification codes with `relay pair-code`
- signed session files
- trusted device storage with `relay trust`
- trusted restore guard with `restore --require-trusted`
- HTTPS listener mode
- mutual TLS client certificate enforcement

## Phase 9: Project Quality

Goal: keep the project easy to verify and maintain.

- GitHub Actions for Go tests
- GitHub Actions for VS Code extension checks
- release build scripts
- SQLite-backed received-session history
- installer packaging

## Phase 10: Broader Context

Goal: restore more of the user's working environment.

- basic terminal working-directory capture and restore
- basic browser tab URL capture and restore
- automatic browser tab reopening
- richer terminal session restore
- richer project history
- background service
- simple desktop UI
