# Demo

This walkthrough sends one workspace session from a source machine to a destination machine on the same local network.

Replace `DESTINATION_IP` with the IP address of the receiving machine.

## 1. Start the Receiver

On the destination machine:

```bash
git clone https://github.com/Amirjon06/handoff-dev.git
cd handoff-dev
go run ./cmd/relay listen --addr 0.0.0.0:8765 --inbox .staterelay/inbox
```

Keep this terminal open.

## 2. Capture a Session

On the source machine:

```bash
cd handoff-dev
go run ./cmd/relay capture --path . --out session.json
```

## 3. Check the Receiver

From the source machine:

```bash
go run ./cmd/relay ping --to http://DESTINATION_IP:8765
```

Expected result:

```text
Status: ok
Service: staterelay
```

## 4. Send the Session

From the source machine:

```bash
go run ./cmd/relay send --to http://DESTINATION_IP:8765 session.json
```

The command prints a receipt ID when the receiver stores the session.

## 5. Inspect Received Sessions

On the destination machine, in a second terminal:

```bash
cd handoff-dev
go run ./cmd/relay inbox --inbox .staterelay/inbox
go run ./cmd/relay history --history .staterelay/history.db
```

## 6. Preview Restore

Restore the newest received session without writing files:

```bash
go run ./cmd/relay restore --inbox .staterelay/inbox --apply --dry-run latest
```

Or restore a specific session from history:

```bash
go run ./cmd/relay restore --history .staterelay/history.db --apply --dry-run SESSION_ID
```

## 7. Apply Restore

When the preview looks right:

```bash
go run ./cmd/relay restore --inbox .staterelay/inbox --apply latest
```

For local changes, preserve incoming copies instead of overwriting:

```bash
go run ./cmd/relay restore --inbox .staterelay/inbox --apply --conflict keep-both latest
```

## Optional Trusted Device Flow

Create identities on both machines:

```bash
go run ./cmd/relay identity --path . --name my-device
```

Compare fingerprints, then verify a pair code:

```bash
go run ./cmd/relay pair-code --path . --peer-fingerprint PEER_FINGERPRINT
```

Trust the peer only after the code matches:

```bash
go run ./cmd/relay pair --path . --name peer-device --fingerprint PEER_FINGERPRINT --code PAIR_CODE
```

Use trusted restore when needed:

```bash
go run ./cmd/relay restore --require-trusted session.json
```
