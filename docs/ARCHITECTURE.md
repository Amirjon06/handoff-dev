# Architecture

StateRelay is built around a small Go CLI and local state files that can later grow into a background agent.

```text
              relay CLI
                   |
      +------------+-------------+
      |            |             |
 VS Code state  Terminal state  Browser state
      |            |             |
      +------------+-------------+
                   |
              session JSON
                   |
            local inbox
                   |
            HTTP transfer
                   |
            destination CLI
```

## Core Ideas

The Go agent owns:

- command-line interface
- Git inspection
- session serialization
- session signing and verification
- local state file reads and writes
- local device identity
- pair verification codes
- trusted-device fingerprint storage
- trusted restore enforcement
- mDNS device discovery
- HTTPS listener mode
- HTTP networking
- restore orchestration

The VS Code extension owns:

- open file list
- active file
- cursor position
- workspace folder
- later, editor restore commands

The browser extension owns:

- future automatic tab capture
- future tab reopening
- future window/tab grouping metadata

The current Go CLI can record browser tab URLs manually with `relay browser --url`.

## MVP Boundary

Version `0.1` proves this loop:

```text
capture workspace state
    -> write session JSON
    -> read session JSON
    -> transfer over HTTP
    -> validate and apply restore state
```

Device discovery, encrypted transport, and automatic browser restore are intentionally outside the current boundary. Pair verification codes and trusted-device storage are present as groundwork for that later transport layer.
