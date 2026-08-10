# Architecture

StateRelay is built around a small Go agent that talks to local integrations and peer devices.

```text
            StateRelay Agent
                   |
      +------------+------------+
      |                         |
 VS Code extension       Browser extension
      |                         |
      +------------+------------+
                   |
              session JSON
                   |
            local storage
                   |
           encrypted network
                   |
            destination agent
```

## Core Ideas

The Go agent owns:

- command-line interface
- Git inspection
- session serialization
- local storage
- networking
- pairing and encryption
- restore orchestration

The VS Code extension owns:

- open file list
- active file
- cursor position
- workspace folder
- later, editor restore commands

The browser extension owns:

- open URLs
- active tab
- window/tab grouping metadata

## MVP Boundary

Version `0.1` should only prove this loop:

```text
capture Git state
    -> write session JSON
    -> read session JSON
    -> print or perform restore actions
```

After that, the next valuable step is VS Code state capture.

