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
- `relay send --to URL`
- HTTP transport with validated session storage
- inbox listing for received sessions
- destination restore

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

## Phase 8: Device Discovery and Pairing

Goal: make the network flow usable and trustworthy.

- `relay devices`
- mDNS discovery
- verification codes
- trusted device storage
- TLS encryption

## Phase 9: Broader Context

Goal: restore more of the user's working environment.

- browser tabs
- terminal directories
- project history
- background service
- simple desktop UI
