package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Amirjon06/handoff-dev/internal/browserstate"
	"github.com/Amirjon06/handoff-dev/internal/deviceidentity"
	"github.com/Amirjon06/handoff-dev/internal/discovery"
	"github.com/Amirjon06/handoff-dev/internal/editorstate"
	"github.com/Amirjon06/handoff-dev/internal/gitstate"
	"github.com/Amirjon06/handoff-dev/internal/history"
	"github.com/Amirjon06/handoff-dev/internal/paircode"
	"github.com/Amirjon06/handoff-dev/internal/session"
	"github.com/Amirjon06/handoff-dev/internal/terminalstate"
	"github.com/Amirjon06/handoff-dev/internal/tlsidentity"
	"github.com/Amirjon06/handoff-dev/internal/transport"
	"github.com/Amirjon06/handoff-dev/internal/truststore"
)

const version = "0.1.0-dev"

var sendSession = transport.Client{}.Send
var pingListener = transport.Client{}.Ping
var advertiseDevice = discovery.Advertise
var discoverDevices = discovery.Lookup
var openBrowserURL = browserstate.OpenURL

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "relay: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		printHelp(stdout)
		return nil
	}

	switch args[0] {
	case "capture":
		return runCapture(ctx, args[1:], stdout)
	case "restore":
		return runRestore(ctx, args[1:], stdout)
	case "send":
		return runSend(ctx, args[1:], stdout)
	case "ping":
		return runPing(ctx, args[1:], stdout)
	case "listen":
		return runListen(ctx, args[1:], stdout)
	case "devices":
		return runDevices(ctx, args[1:], stdout)
	case "inbox":
		return runInbox(ctx, args[1:], stdout)
	case "history":
		return runHistory(ctx, args[1:], stdout)
	case "doctor":
		return runDoctor(ctx, args[1:], stdout)
	case "terminal":
		return runTerminal(ctx, args[1:], stdout)
	case "browser":
		return runBrowser(ctx, args[1:], stdout)
	case "identity":
		return runIdentity(ctx, args[1:], stdout)
	case "pair-code":
		return runPairCode(ctx, args[1:], stdout)
	case "trust":
		return runTrust(ctx, args[1:], stdout)
	case "version":
		fmt.Fprintln(stdout, version)
		return nil
	case "help", "-h", "--help":
		printHelp(stdout)
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runPairCode(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("pair-code", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	path := fs.String("path", ".", "workspace path")
	peerFingerprint := fs.String("peer-fingerprint", "", "peer device fingerprint")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("pair-code does not accept positional arguments")
	}
	if strings.TrimSpace(*peerFingerprint) == "" {
		return fmt.Errorf("pair-code requires --peer-fingerprint")
	}

	root, err := gitstate.Root(ctx, *path)
	if err != nil {
		return fmt.Errorf("find workspace root: %w", err)
	}
	identity, err := deviceidentity.Load(deviceidentity.Path(root))
	if os.IsNotExist(err) {
		return fmt.Errorf("device identity missing; run relay identity --path %s first", root)
	}
	if err != nil {
		return err
	}
	code, err := paircode.Code(identity.Fingerprint, *peerFingerprint)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Pair verification code: %s\n", code)
	fmt.Fprintf(stdout, "Local fingerprint: %s\n", identity.Fingerprint)
	fmt.Fprintf(stdout, "Peer fingerprint: %s\n", strings.ToLower(strings.TrimSpace(*peerFingerprint)))
	return nil
}

func runTrust(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("trust requires a subcommand: add, list, or remove")
	}
	switch args[0] {
	case "add":
		return runTrustAdd(ctx, args[1:], stdout)
	case "list":
		return runTrustList(ctx, args[1:], stdout)
	case "remove":
		return runTrustRemove(ctx, args[1:], stdout)
	default:
		return fmt.Errorf("unknown trust subcommand %q", args[0])
	}
}

func runTrustAdd(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("trust add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	path := fs.String("path", ".", "workspace path")
	name := fs.String("name", "", "trusted device name")
	fingerprint := fs.String("fingerprint", "", "trusted device fingerprint")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("trust add does not accept positional arguments")
	}

	root, err := gitstate.Root(ctx, *path)
	if err != nil {
		return fmt.Errorf("find workspace root: %w", err)
	}
	storePath := truststore.Path(root)
	store, err := truststore.Load(storePath)
	if err != nil {
		return err
	}
	store, added, err := truststore.Add(store, *name, *fingerprint, time.Now())
	if err != nil {
		return err
	}
	if err := truststore.Save(storePath, store); err != nil {
		return err
	}
	if added {
		fmt.Fprintln(stdout, "Added trusted device")
	} else {
		fmt.Fprintln(stdout, "Updated trusted device")
	}
	fmt.Fprintf(stdout, "Name: %s\n", strings.TrimSpace(*name))
	fmt.Fprintf(stdout, "Fingerprint: %s\n", strings.ToLower(strings.TrimSpace(*fingerprint)))
	fmt.Fprintf(stdout, "Path: %s\n", storePath)
	return nil
}

func runTrustList(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("trust list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	path := fs.String("path", ".", "workspace path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("trust list does not accept positional arguments")
	}

	root, err := gitstate.Root(ctx, *path)
	if err != nil {
		return fmt.Errorf("find workspace root: %w", err)
	}
	store, err := truststore.Load(truststore.Path(root))
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Trusted devices: %d\n", len(store.Devices))
	if len(store.Devices) == 0 {
		fmt.Fprintln(stdout, "No trusted devices")
		return nil
	}
	for _, device := range store.Devices {
		fmt.Fprintf(stdout, "- %s %s\n", device.Name, device.Fingerprint)
	}
	return nil
}

func runTrustRemove(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("trust remove", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	path := fs.String("path", ".", "workspace path")
	fingerprint := fs.String("fingerprint", "", "trusted device fingerprint")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("trust remove does not accept positional arguments")
	}

	root, err := gitstate.Root(ctx, *path)
	if err != nil {
		return fmt.Errorf("find workspace root: %w", err)
	}
	storePath := truststore.Path(root)
	store, err := truststore.Load(storePath)
	if err != nil {
		return err
	}
	store, removed, err := truststore.Remove(store, *fingerprint)
	if err != nil {
		return err
	}
	if err := truststore.Save(storePath, store); err != nil {
		return err
	}
	if removed {
		fmt.Fprintln(stdout, "Removed trusted device")
	} else {
		fmt.Fprintln(stdout, "Trusted device not found")
	}
	fmt.Fprintf(stdout, "Fingerprint: %s\n", strings.ToLower(strings.TrimSpace(*fingerprint)))
	return nil
}

func runIdentity(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("identity", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	path := fs.String("path", ".", "workspace path")
	name := fs.String("name", "", "device name for a new identity")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("identity does not accept positional arguments")
	}

	root, err := gitstate.Root(ctx, *path)
	if err != nil {
		return fmt.Errorf("find workspace root: %w", err)
	}
	deviceName := strings.TrimSpace(*name)
	if deviceName == "" {
		deviceName, err = os.Hostname()
		if err != nil {
			return fmt.Errorf("read hostname: %w", err)
		}
	}

	identity, created, err := deviceidentity.LoadOrCreate(deviceidentity.Path(root), deviceName, time.Now())
	if err != nil {
		return err
	}
	if created {
		fmt.Fprintln(stdout, "Created device identity")
	} else {
		fmt.Fprintln(stdout, "Loaded device identity")
	}
	fmt.Fprintf(stdout, "Name: %s\n", identity.Name)
	fmt.Fprintf(stdout, "Fingerprint: %s\n", identity.Fingerprint)
	fmt.Fprintf(stdout, "Path: %s\n", deviceidentity.Path(root))
	return nil
}

type stringListFlag []string

func (f *stringListFlag) String() string {
	if f == nil {
		return ""
	}
	return strings.Join(*f, ",")
}

func (f *stringListFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func runBrowser(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("browser", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	path := fs.String("path", ".", "workspace path")
	restore := fs.Bool("restore", false, "restore recorded browser URLs")
	openTabs := fs.Bool("open", false, "open restored browser URLs")
	var urls stringListFlag
	fs.Var(&urls, "url", "browser URL to record; repeat for multiple tabs")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("browser does not accept positional arguments")
	}
	if *openTabs && !*restore {
		return fmt.Errorf("browser --open requires --restore")
	}
	if *restore && len(urls) > 0 {
		return fmt.Errorf("browser --restore cannot be combined with --url")
	}

	root, err := gitstate.Root(ctx, *path)
	if err != nil {
		return fmt.Errorf("find workspace root: %w", err)
	}
	if *restore {
		state, err := browserstate.ReadWorkspace(root)
		if err != nil {
			return err
		}
		if state == nil {
			return fmt.Errorf("browser state missing; run relay browser --url URL first")
		}
		fmt.Fprintf(stdout, "Browser tabs: %d\n", len(state.Tabs))
		for _, tab := range state.Tabs {
			fmt.Fprintf(stdout, "URL: %s\n", tab.URL)
			if *openTabs {
				if err := openBrowserURL(ctx, tab.URL); err != nil {
					return err
				}
			}
		}
		if *openTabs {
			fmt.Fprintf(stdout, "Opened browser tabs: %d\n", len(state.Tabs))
		}
		return nil
	}
	state, err := browserstate.Capture(urls, time.Now())
	if err != nil {
		return err
	}
	if err := browserstate.WriteWorkspace(root, &state); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Captured browser state to %s\n", filepath.Join(root, ".staterelay", "browser-state.json"))
	fmt.Fprintf(stdout, "Browser tabs: %d\n", len(state.Tabs))
	return nil
}

func runTerminal(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("terminal", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	path := fs.String("path", ".", "workspace path")
	cwd := fs.String("cwd", "", "terminal working directory to record")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("terminal does not accept positional arguments")
	}

	root, err := gitstate.Root(ctx, *path)
	if err != nil {
		return fmt.Errorf("find workspace root: %w", err)
	}
	state, err := terminalstate.Capture(root, *cwd, time.Now())
	if err != nil {
		return err
	}
	if err := terminalstate.WriteWorkspace(root, &state); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Captured terminal state to %s\n", filepath.Join(root, ".staterelay", "terminal-state.json"))
	fmt.Fprintf(stdout, "Working directories: %d\n", len(state.WorkingDirectories))
	return nil
}

func runDoctor(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	path := fs.String("path", ".", "project path to inspect")
	inbox := fs.String("inbox", filepath.Join(".staterelay", "inbox"), "directory for received sessions")
	historyPath := fs.String("history", history.Path("."), "SQLite history database path")
	target := fs.String("to", "", "optional HTTP listener to check")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("doctor does not accept positional arguments")
	}

	state, err := gitstate.Capture(ctx, *path)
	if err != nil {
		return fmt.Errorf("check repository: %w", err)
	}
	entries, err := transport.FileInbox{Dir: *inbox}.List(ctx)
	if err != nil {
		return fmt.Errorf("check inbox: %w", err)
	}

	fmt.Fprintln(stdout, "StateRelay doctor")
	fmt.Fprintln(stdout, "Repository: ok")
	fmt.Fprintf(stdout, "  root: %s\n", state.Root)
	fmt.Fprintf(stdout, "  branch: %s\n", state.Branch)
	fmt.Fprintf(stdout, "  commit: %s\n", shortHash(state.Commit))
	fmt.Fprintf(stdout, "  dirty: %t\n", state.Dirty)
	fmt.Fprintln(stdout, "Inbox: ok")
	fmt.Fprintf(stdout, "  path: %s\n", *inbox)
	fmt.Fprintf(stdout, "  sessions: %d\n", len(entries))
	historyStore, err := history.Open(*historyPath)
	if err != nil {
		return fmt.Errorf("check history: %w", err)
	}
	historyCount, err := historyStore.Count(ctx)
	closeErr := historyStore.Close()
	if err != nil {
		return fmt.Errorf("check history: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("close history: %w", closeErr)
	}
	fmt.Fprintln(stdout, "History: ok")
	fmt.Fprintf(stdout, "  path: %s\n", *historyPath)
	fmt.Fprintf(stdout, "  sessions: %d\n", historyCount)
	identity, err := deviceidentity.Load(deviceidentity.Path(state.Root))
	if err == nil {
		fmt.Fprintln(stdout, "Identity: ok")
		fmt.Fprintf(stdout, "  name: %s\n", identity.Name)
		fmt.Fprintf(stdout, "  fingerprint: %s\n", identity.Fingerprint)
	} else if os.IsNotExist(err) {
		fmt.Fprintln(stdout, "Identity: missing")
	} else {
		return fmt.Errorf("check identity: %w", err)
	}
	store, err := truststore.Load(truststore.Path(state.Root))
	if err != nil {
		return fmt.Errorf("check trusted devices: %w", err)
	}
	fmt.Fprintln(stdout, "Trusted devices: ok")
	fmt.Fprintf(stdout, "  count: %d\n", len(store.Devices))

	if *target == "" {
		fmt.Fprintln(stdout, "Listener: skipped")
		return nil
	}
	health, err := pingListener(ctx, *target)
	if err != nil {
		return fmt.Errorf("check listener: %w", err)
	}
	fmt.Fprintln(stdout, "Listener: ok")
	fmt.Fprintf(stdout, "  target: %s\n", *target)
	fmt.Fprintf(stdout, "  service: %s\n", health.Service)
	return nil
}

func runPing(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("ping", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	target := fs.String("to", "", "HTTP or HTTPS listener to check")
	insecureTLS := fs.Bool("insecure-tls", false, "allow a self-signed TLS listener certificate")
	clientCert := fs.Bool("client-cert", false, "send the local device identity certificate")
	path := fs.String("path", ".", "workspace path for client certificate identity")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *target == "" {
		return fmt.Errorf("ping requires --to")
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("ping does not accept positional arguments")
	}

	ping := pingListener
	if *insecureTLS || *clientCert {
		client, err := newTLSClient(ctx, *path, *clientCert, *insecureTLS)
		if err != nil {
			return err
		}
		ping = client.Ping
	}
	health, err := ping(ctx, *target)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Listener: %s\n", *target)
	fmt.Fprintf(stdout, "Status: %s\n", health.Status)
	fmt.Fprintf(stdout, "Service: %s\n", health.Service)
	return nil
}

func runSend(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("send", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	target := fs.String("to", "", "HTTP or HTTPS target to send the session to")
	insecureTLS := fs.Bool("insecure-tls", false, "allow a self-signed TLS listener certificate")
	clientCert := fs.Bool("client-cert", false, "send the local device identity certificate")
	path := fs.String("path", ".", "workspace path for client certificate identity")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *target == "" {
		return fmt.Errorf("send requires --to")
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("send requires a session JSON file")
	}

	captured, err := readSessionFile(fs.Arg(0))
	if err != nil {
		return err
	}

	send := sendSession
	if *insecureTLS || *clientCert {
		client, err := newTLSClient(ctx, *path, *clientCert, *insecureTLS)
		if err != nil {
			return err
		}
		send = client.Send
	}
	receipt, err := send(ctx, *target, captured)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Sent session to %s\n", *target)
	if receipt.ID != "" {
		fmt.Fprintf(stdout, "Receipt: %s\n", receipt.ID)
	}
	return nil
}

func runListen(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("listen", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	addr := fs.String("addr", "127.0.0.1:8765", "HTTP listen address")
	inbox := fs.String("inbox", filepath.Join(".staterelay", "inbox"), "directory for received sessions")
	historyPath := fs.String("history", history.Path("."), "SQLite history database path")
	advertise := fs.Bool("advertise", false, "publish this listener with mDNS")
	tlsEnabled := fs.Bool("tls", false, "serve HTTPS with the local device identity")
	requireClientCert := fs.Bool("require-client-cert", false, "require a trusted client certificate")
	path := fs.String("path", ".", "workspace path for device identity when advertising")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("listen does not accept positional arguments")
	}
	if *requireClientCert && !*tlsEnabled {
		return fmt.Errorf("listen --require-client-cert requires --tls")
	}
	var root string
	var identity *deviceidentity.Identity
	if *advertise || *tlsEnabled || *requireClientCert {
		loadedRoot, loaded, err := loadWorkspaceIdentity(ctx, *path)
		if err != nil {
			return err
		}
		root = loadedRoot
		identity = &loaded
	}
	var advertiseOptions discovery.AdvertiseOptions
	if *advertise {
		var err error
		advertiseOptions, err = newAdvertiseOptions(*identity, *addr, *tlsEnabled)
		if err != nil {
			return err
		}
	}
	var tlsConfig *tls.Config
	if *tlsEnabled {
		config, err := newListenerTLSConfig(*identity, root, *requireClientCert)
		if err != nil {
			return err
		}
		tlsConfig = config
	}

	server := &http.Server{
		Addr:              *addr,
		Handler:           transport.Handler(recordingInbox{files: transport.FileInbox{Dir: *inbox}, historyPath: *historyPath}),
		ReadHeaderTimeout: 5 * time.Second,
		TLSConfig:         tlsConfig,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	scheme := "http"
	if *tlsEnabled {
		scheme = "https"
	}
	fmt.Fprintf(stdout, "Listening on %s://%s\n", scheme, *addr)
	fmt.Fprintf(stdout, "Inbox: %s\n", *inbox)
	fmt.Fprintf(stdout, "History: %s\n", *historyPath)
	if *advertise {
		go func() {
			_ = advertiseDevice(ctx, advertiseOptions)
		}()
		fmt.Fprintf(stdout, "Advertising as %s\n", advertiseOptions.Name)
	}
	if *requireClientCert {
		fmt.Fprintln(stdout, "Client certificates: required")
	}
	var err error
	if *tlsEnabled {
		err = server.ListenAndServeTLS("", "")
	} else {
		err = server.ListenAndServe()
	}
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

type recordingInbox struct {
	files       transport.FileInbox
	historyPath string
}

func (r recordingInbox) Save(ctx context.Context, captured session.Session) (transport.Receipt, error) {
	receipt, err := r.files.Save(ctx, captured)
	if err != nil {
		return transport.Receipt{}, err
	}
	if strings.TrimSpace(r.historyPath) == "" {
		return receipt, nil
	}

	store, err := history.Open(r.historyPath)
	if err != nil {
		return transport.Receipt{}, fmt.Errorf("open history: %w", err)
	}
	defer store.Close()

	sessionPath := filepath.Join(r.files.Path(), receipt.ID+".json")
	event := history.NewReceivedEvent(receipt.ID, sessionPath, captured, time.Now())
	if err := store.Record(ctx, event); err != nil {
		return transport.Receipt{}, fmt.Errorf("record history: %w", err)
	}
	return receipt, nil
}

func runDevices(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("devices", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	timeout := fs.Duration("timeout", 2*time.Second, "mDNS discovery timeout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("devices does not accept positional arguments")
	}
	if *timeout <= 0 {
		return fmt.Errorf("devices timeout must be positive")
	}

	devices, err := discoverDevices(ctx, *timeout)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Discovered devices: %d\n", len(devices))
	if len(devices) == 0 {
		fmt.Fprintln(stdout, "No StateRelay devices found")
		return nil
	}
	for _, device := range devices {
		fmt.Fprintf(stdout, "- %s\n", device.Name)
		if device.Endpoint != "" {
			fmt.Fprintf(stdout, "  endpoint: %s\n", device.Endpoint)
		}
		if device.Fingerprint != "" {
			fmt.Fprintf(stdout, "  fingerprint: %s\n", device.Fingerprint)
		}
		if device.Version != "" {
			fmt.Fprintf(stdout, "  version: %s\n", device.Version)
		}
	}
	return nil
}

func loadWorkspaceIdentity(ctx context.Context, path string) (string, deviceidentity.Identity, error) {
	root, err := gitstate.Root(ctx, path)
	if err != nil {
		return "", deviceidentity.Identity{}, fmt.Errorf("find workspace root: %w", err)
	}
	identity, err := deviceidentity.Load(deviceidentity.Path(root))
	if os.IsNotExist(err) {
		return "", deviceidentity.Identity{}, fmt.Errorf("device identity missing; run relay identity --path %s first", root)
	}
	if err != nil {
		return "", deviceidentity.Identity{}, err
	}
	return root, identity, nil
}

func newTLSClient(ctx context.Context, path string, clientCert bool, insecureTLS bool) (transport.Client, error) {
	var certificates []tls.Certificate
	if clientCert {
		_, identity, err := loadWorkspaceIdentity(ctx, path)
		if err != nil {
			return transport.Client{}, err
		}
		cert, err := tlsidentity.Certificate(identity, time.Now())
		if err != nil {
			return transport.Client{}, fmt.Errorf("create client TLS certificate: %w", err)
		}
		certificates = []tls.Certificate{cert}
	}
	return transport.TLSClient(certificates, insecureTLS), nil
}

func newListenerTLSConfig(identity deviceidentity.Identity, root string, requireClientCert bool) (*tls.Config, error) {
	cert, err := tlsidentity.Certificate(identity, time.Now())
	if err != nil {
		return nil, fmt.Errorf("create TLS certificate: %w", err)
	}
	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	if !requireClientCert {
		return config, nil
	}

	store, err := truststore.Load(truststore.Path(root))
	if err != nil {
		return nil, fmt.Errorf("read trusted devices: %w", err)
	}
	config.ClientAuth = tls.RequireAnyClientCert
	config.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		if len(rawCerts) == 0 {
			return fmt.Errorf("client certificate is required")
		}
		cert, err := x509.ParseCertificate(rawCerts[0])
		if err != nil {
			return fmt.Errorf("parse client certificate: %w", err)
		}
		fingerprint, err := tlsidentity.Fingerprint(cert)
		if err != nil {
			return err
		}
		ok, err := truststore.Contains(store, fingerprint)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("client certificate %s is not trusted", fingerprint)
		}
		return nil
	}
	return config, nil
}

func newAdvertiseOptions(identity deviceidentity.Identity, addr string, tlsEnabled bool) (discovery.AdvertiseOptions, error) {
	port, err := listenPort(addr)
	if err != nil {
		return discovery.AdvertiseOptions{}, err
	}
	scheme := "http"
	if tlsEnabled {
		scheme = "https"
	}
	return discovery.AdvertiseOptions{
		Instance:    identity.Name,
		Name:        identity.Name,
		Fingerprint: identity.Fingerprint,
		Version:     version,
		Scheme:      scheme,
		Port:        port,
	}, nil
}

func listenPort(addr string) (int, error) {
	_, portText, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, fmt.Errorf("parse listen address %q: %w", addr, err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return 0, fmt.Errorf("parse listen port %q: %w", portText, err)
	}
	if port <= 0 || port > 65535 {
		return 0, fmt.Errorf("listen port must be between 1 and 65535")
	}
	return port, nil
}

func runInbox(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("inbox", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	inbox := fs.String("inbox", filepath.Join(".staterelay", "inbox"), "directory for received sessions")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("inbox does not accept positional arguments")
	}

	entries, err := transport.FileInbox{Dir: *inbox}.List(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Inbox: %s\n", *inbox)
	if len(entries) == 0 {
		fmt.Fprintln(stdout, "No received sessions")
		return nil
	}
	for _, entry := range entries {
		captured := entry.Session
		fmt.Fprintf(stdout, "%s\n", entry.Path)
		fmt.Fprintf(stdout, "  repo: %s\n", captured.Git.Name)
		fmt.Fprintf(stdout, "  branch: %s\n", captured.Git.Branch)
		fmt.Fprintf(stdout, "  commit: %s\n", shortHash(captured.Git.Commit))
		fmt.Fprintf(stdout, "  dirty: %t\n", captured.Git.Dirty)
		fmt.Fprintf(stdout, "  changed files: %d\n", len(captured.Git.ChangedFiles))
		printSessionSignature(stdout, captured, "  ")
		if captured.Editor != nil {
			fmt.Fprintf(stdout, "  editor files: %d\n", len(captured.Editor.OpenFiles))
		}
		if captured.Terminal != nil {
			fmt.Fprintf(stdout, "  terminal directories: %d\n", len(captured.Terminal.WorkingDirectories))
		}
		if captured.Browser != nil {
			fmt.Fprintf(stdout, "  browser tabs: %d\n", len(captured.Browser.Tabs))
		}
	}
	return nil
}

func runHistory(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("history", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	historyPath := fs.String("history", history.Path("."), "SQLite history database path")
	limit := fs.Int("limit", 20, "maximum sessions to show")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("history does not accept positional arguments")
	}
	if *limit <= 0 {
		return fmt.Errorf("history limit must be positive")
	}

	store, err := history.Open(*historyPath)
	if err != nil {
		return err
	}
	defer store.Close()

	events, err := store.List(ctx, *limit)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "History: %s\n", *historyPath)
	if len(events) == 0 {
		fmt.Fprintln(stdout, "No sessions recorded")
		return nil
	}
	for _, event := range events {
		fmt.Fprintf(stdout, "%s %s %s\n", event.StoredAt.Format(time.RFC3339), event.Direction, event.ID)
		fmt.Fprintf(stdout, "  repo: %s\n", event.RepoName)
		fmt.Fprintf(stdout, "  branch: %s\n", event.RepoBranch)
		fmt.Fprintf(stdout, "  commit: %s\n", shortHash(event.RepoCommit))
		fmt.Fprintf(stdout, "  dirty: %t\n", event.Dirty)
		fmt.Fprintf(stdout, "  changed files: %d\n", event.ChangedFiles)
		if event.SignerFingerprint != "" {
			fmt.Fprintf(stdout, "  signer: %s (%s)\n", event.SignerName, shortHash(event.SignerFingerprint))
		}
		if event.SessionPath != "" {
			fmt.Fprintf(stdout, "  file: %s\n", event.SessionPath)
		}
	}
	return nil
}

func runCapture(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("capture", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	path := fs.String("path", ".", "project path to inspect")
	jsonOutput := fs.Bool("json", false, "write captured session as JSON")
	out := fs.String("out", "", "write captured session JSON to this file")

	if err := fs.Parse(args); err != nil {
		return err
	}

	state, err := gitstate.Capture(ctx, *path)
	if err != nil {
		return err
	}
	editor, err := editorstate.ReadWorkspace(state.Root)
	if err != nil {
		return fmt.Errorf("read editor state: %w", err)
	}
	terminal, err := terminalstate.ReadWorkspace(state.Root)
	if err != nil {
		return fmt.Errorf("read terminal state: %w", err)
	}
	browser, err := browserstate.ReadWorkspace(state.Root)
	if err != nil {
		return fmt.Errorf("read browser state: %w", err)
	}

	if *jsonOutput || *out != "" {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unknown"
		}
		captured := session.NewWithWorkspace(hostname, state, editor, terminal, browser, time.Now())
		captured, err = signSessionFromWorkspace(captured, state.Root)
		if err != nil {
			return err
		}

		if *out == "" {
			return session.WriteJSON(stdout, captured)
		}

		file, err := os.Create(*out)
		if err != nil {
			return fmt.Errorf("create %s: %w", *out, err)
		}
		defer file.Close()

		if err := session.WriteJSON(file, captured); err != nil {
			return fmt.Errorf("write %s: %w", *out, err)
		}

		fmt.Fprintf(stdout, "Captured session to %s\n", *out)
		return nil
	}

	fmt.Fprintf(stdout, "Git root: %s\n", state.Root)
	fmt.Fprintf(stdout, "Git name: %s\n", state.Name)
	fmt.Fprintf(stdout, "Git remote: %s\n", state.Remote)
	fmt.Fprintf(stdout, "Git branch: %s\n", state.Branch)
	fmt.Fprintf(stdout, "Git commit: %s\n", state.Commit)
	fmt.Fprintf(stdout, "Git dirty: %t\n", state.Dirty)
	for _, file := range state.ChangedFiles {
		if file.ContentCaptured {
			fmt.Fprintf(stdout, "Changed file: %s %s (%d bytes captured, sha256 %s)\n", file.Status, file.Path, file.Size, shortHash(file.ContentSHA256))
			continue
		}
		fmt.Fprintf(stdout, "Changed file: %s %s (content not captured)\n", file.Status, file.Path)
	}
	printEditorState(stdout, editor)
	printTerminalState(stdout, terminal)
	printBrowserState(stdout, browser)
	return nil
}

func runRestore(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("restore", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	apply := fs.Bool("apply", false, "write captured file snapshots after validation")
	dryRun := fs.Bool("dry-run", false, "validate apply without writing captured file snapshots")
	path := fs.String("path", "", "project path to restore into")
	cloneDir := fs.String("clone-dir", "", "clone missing repository into this parent directory")
	inbox := fs.String("inbox", "", "directory for received sessions; use latest as the session name")
	conflict := fs.String("conflict", "reject", "conflict strategy: reject or keep-both")
	requireTrusted := fs.Bool("require-trusted", false, "require the session signer to exist in the local trust store")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *conflict != "reject" && *conflict != "keep-both" {
		return fmt.Errorf("unknown conflict strategy %q", *conflict)
	}

	if fs.NArg() != 1 {
		return fmt.Errorf("restore requires a session JSON file")
	}

	sessionPath, err := resolveRestoreSession(ctx, fs.Arg(0), *inbox)
	if err != nil {
		return err
	}

	captured, err := readSessionFile(sessionPath)
	if err != nil {
		return err
	}

	restorePath, cloned, err := resolveRestorePath(ctx, captured.Git, *path, *cloneDir)
	if err != nil {
		return err
	}

	verifiedRoot, err := gitstate.Root(ctx, restorePath)
	if err != nil {
		return fmt.Errorf("verify git root: %w", err)
	}
	if *path == "" && !samePath(verifiedRoot, captured.Git.Root) {
		if *cloneDir == "" {
			return fmt.Errorf("session root %s resolved to %s", captured.Git.Root, verifiedRoot)
		}
	}
	if *requireTrusted {
		if err := verifyTrustedSession(captured, verifiedRoot); err != nil {
			return err
		}
	}

	currentBranch, err := gitstate.Branch(ctx, verifiedRoot)
	if err != nil {
		return fmt.Errorf("verify git branch: %w", err)
	}
	if currentBranch != captured.Git.Branch {
		return fmt.Errorf("session branch %s does not match current branch %s", captured.Git.Branch, currentBranch)
	}

	currentCommit, err := gitstate.Commit(ctx, verifiedRoot)
	if err != nil {
		return fmt.Errorf("verify git commit: %w", err)
	}
	if currentCommit != captured.Git.Commit {
		return fmt.Errorf("session commit %s does not match current commit %s", captured.Git.Commit, currentCommit)
	}

	if *apply {
		currentState, err := gitstate.Capture(ctx, verifiedRoot)
		if err != nil {
			return fmt.Errorf("verify working tree: %w", err)
		}
		if currentState.Dirty && *conflict == "reject" {
			return fmt.Errorf("refusing to apply over dirty working tree: %s", formatChangedFiles(currentState.ChangedFiles))
		}
		if *dryRun {
			result, err := planApplyFiles(verifiedRoot, captured.Git.ChangedFiles, currentState.ChangedFiles, *conflict)
			if err != nil {
				return err
			}
			printApplyResult(stdout, "Would apply", result)
			printEditorApplyResult(stdout, "Would restore", captured.Editor)
			printTerminalApplyResult(stdout, "Would restore", captured.Terminal)
			printBrowserApplyResult(stdout, "Would restore", captured.Browser)
			return nil
		}
		result, err := applyChangedFiles(verifiedRoot, captured.Git.ChangedFiles, currentState.ChangedFiles, *conflict)
		if err != nil {
			return err
		}
		if err := editorstate.WriteWorkspace(verifiedRoot, captured.Editor); err != nil {
			return err
		}
		if err := terminalstate.WriteWorkspace(verifiedRoot, captured.Terminal); err != nil {
			return err
		}
		if err := browserstate.WriteWorkspace(verifiedRoot, captured.Browser); err != nil {
			return err
		}
		printApplyResult(stdout, "Applied", result)
		printEditorApplyResult(stdout, "Restored", captured.Editor)
		printTerminalApplyResult(stdout, "Restored", captured.Terminal)
		printBrowserApplyResult(stdout, "Restored", captured.Browser)
		return nil
	}

	fmt.Fprintln(stdout, "Restore plan")
	fmt.Fprintf(stdout, "Session file: %s\n", sessionPath)
	if cloned {
		fmt.Fprintf(stdout, "Cloned repository to: %s\n", verifiedRoot)
	}
	fmt.Fprintf(stdout, "Git root: %s\n", captured.Git.Root)
	if *path != "" {
		fmt.Fprintf(stdout, "Restore root: %s\n", verifiedRoot)
	}
	if *cloneDir != "" {
		fmt.Fprintf(stdout, "Restore root: %s\n", verifiedRoot)
	}
	fmt.Fprintf(stdout, "Git name: %s\n", captured.Git.Name)
	fmt.Fprintf(stdout, "Git remote: %s\n", captured.Git.Remote)
	fmt.Fprintf(stdout, "Git branch: %s\n", captured.Git.Branch)
	fmt.Fprintf(stdout, "Git commit: %s\n", captured.Git.Commit)
	fmt.Fprintf(stdout, "Git dirty: %t\n", captured.Git.Dirty)
	printSessionSignature(stdout, captured, "")
	if *requireTrusted {
		fmt.Fprintln(stdout, "Trusted signer: verified")
	}
	if len(captured.Git.ChangedFiles) > 0 {
		fmt.Fprintln(stdout, "Changed files:")
		for _, file := range captured.Git.ChangedFiles {
			if file.ContentCaptured {
				fmt.Fprintf(stdout, "- %s %s (%d bytes captured, sha256 %s)\n", file.Status, file.Path, file.Size, shortHash(file.ContentSHA256))
				continue
			}
			fmt.Fprintf(stdout, "- %s %s (content not captured)\n", file.Status, file.Path)
		}
	}
	printEditorState(stdout, captured.Editor)
	printTerminalState(stdout, captured.Terminal)
	printBrowserState(stdout, captured.Browser)
	return nil
}

func resolveRestorePath(ctx context.Context, git gitstate.State, path string, cloneDir string) (string, bool, error) {
	if path != "" && cloneDir != "" {
		return "", false, fmt.Errorf("restore cannot use --path and --clone-dir together")
	}
	if path != "" {
		return path, false, nil
	}
	if cloneDir == "" {
		return git.Root, false, nil
	}
	if !safeRepoName(git.Name) {
		return "", false, fmt.Errorf("unsafe repository name %q", git.Name)
	}

	destination := filepath.Join(cloneDir, git.Name)
	if _, err := os.Stat(destination); err == nil {
		return destination, false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", false, fmt.Errorf("inspect clone destination: %w", err)
	}

	if err := gitstate.Clone(ctx, git.Remote, git.Branch, destination); err != nil {
		return "", false, err
	}
	return destination, true, nil
}

func safeRepoName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return false
	}
	if strings.Contains(name, "/") || strings.Contains(name, `\`) {
		return false
	}
	return filepath.Clean(name) == name
}

func resolveRestoreSession(ctx context.Context, name string, inbox string) (string, error) {
	if inbox == "" && name != "latest" {
		return name, nil
	}

	inboxDir := inbox
	if inboxDir == "" {
		inboxDir = filepath.Join(".staterelay", "inbox")
	}

	entries, err := transport.FileInbox{Dir: inboxDir}.List(ctx)
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("no received sessions in %s", inboxDir)
	}
	if name == "latest" {
		return entries[0].Path, nil
	}
	for _, entry := range entries {
		if entry.Name == name || strings.TrimSuffix(entry.Name, ".json") == name {
			return entry.Path, nil
		}
	}
	return "", fmt.Errorf("received session %s not found in %s", name, inboxDir)
}

func readSessionFile(path string) (session.Session, error) {
	file, err := os.Open(path)
	if err != nil {
		return session.Session{}, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	captured, err := session.ReadJSON(file)
	if err != nil {
		return session.Session{}, fmt.Errorf("read session: %w", err)
	}
	return captured, nil
}

func signSessionFromWorkspace(captured session.Session, root string) (session.Session, error) {
	identity, err := deviceidentity.Load(deviceidentity.Path(root))
	if os.IsNotExist(err) {
		return captured, nil
	}
	if err != nil {
		return session.Session{}, fmt.Errorf("read device identity: %w", err)
	}
	signed, err := session.Sign(captured, identity)
	if err != nil {
		return session.Session{}, fmt.Errorf("sign session: %w", err)
	}
	return signed, nil
}

func verifyTrustedSession(captured session.Session, root string) error {
	if captured.Signature == nil {
		return fmt.Errorf("session is unsigned")
	}
	store, err := truststore.Load(truststore.Path(root))
	if err != nil {
		return fmt.Errorf("read trusted devices: %w", err)
	}
	ok, err := truststore.Contains(store, captured.Signature.Fingerprint)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("session signer %s is not trusted", captured.Signature.Fingerprint)
	}
	return nil
}

func formatChangedFiles(files []gitstate.ChangedFile) string {
	if len(files) == 0 {
		return "unknown local changes"
	}

	parts := make([]string, 0, len(files))
	for _, file := range files {
		parts = append(parts, file.Status+" "+file.Path)
	}
	return strings.Join(parts, ", ")
}

type applyResult struct {
	applied   []gitstate.ChangedFile
	skipped   []gitstate.ChangedFile
	conflicts []conflictCopy
}

type conflictCopy struct {
	file gitstate.ChangedFile
	path string
}

func planApplyFiles(root string, files []gitstate.ChangedFile, localFiles []gitstate.ChangedFile, conflict string) (applyResult, error) {
	var result applyResult
	localChanged := changedPathSet(localFiles)

	for _, file := range files {
		if !file.ContentCaptured {
			result.skipped = append(result.skipped, file)
			continue
		}

		path, ok := safePath(root, file.Path)
		if !ok {
			return result, fmt.Errorf("unsafe changed file path %s", file.Path)
		}
		if file.ContentSHA256 == "" {
			return result, fmt.Errorf("changed file %s is missing content hash", file.Path)
		}
		if sha256Hex([]byte(file.Content)) != file.ContentSHA256 {
			return result, fmt.Errorf("changed file %s content hash mismatch", file.Path)
		}
		if conflict == "keep-both" && localChanged[file.Path] {
			copyPath, err := nextConflictCopyPath(path)
			if err != nil {
				return result, err
			}
			result.conflicts = append(result.conflicts, conflictCopy{file: file, path: copyPath})
			continue
		}
		result.applied = append(result.applied, file)
	}

	return result, nil
}

func applyChangedFiles(root string, files []gitstate.ChangedFile, localFiles []gitstate.ChangedFile, conflict string) (applyResult, error) {
	result, err := planApplyFiles(root, files, localFiles, conflict)
	if err != nil {
		return result, err
	}

	for _, file := range result.applied {
		path, _ := safePath(root, file.Path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return result, fmt.Errorf("create parent directory for %s: %w", file.Path, err)
		}
		if err := os.WriteFile(path, []byte(file.Content), 0o644); err != nil {
			return result, fmt.Errorf("write changed file %s: %w", file.Path, err)
		}
	}
	for _, copy := range result.conflicts {
		if err := os.MkdirAll(filepath.Dir(copy.path), 0o755); err != nil {
			return result, fmt.Errorf("create parent directory for conflict copy %s: %w", copy.file.Path, err)
		}
		if err := os.WriteFile(copy.path, []byte(copy.file.Content), 0o644); err != nil {
			return result, fmt.Errorf("write conflict copy for %s: %w", copy.file.Path, err)
		}
	}

	return result, nil
}

func printApplyResult(stdout io.Writer, action string, result applyResult) {
	fmt.Fprintf(stdout, "%s %d changed file(s)\n", action, len(result.applied))
	if len(result.skipped) > 0 {
		fmt.Fprintf(stdout, "Skipped %d changed file(s) without captured content: %s\n", len(result.skipped), formatChangedFiles(result.skipped))
	}
	if len(result.conflicts) > 0 {
		conflictAction := "Kept"
		if strings.HasPrefix(action, "Would ") {
			conflictAction = "Would keep"
		}
		fmt.Fprintf(stdout, "%s %d conflict copy/copies\n", conflictAction, len(result.conflicts))
		for _, copy := range result.conflicts {
			fmt.Fprintf(stdout, "Conflict copy: %s -> %s\n", copy.file.Path, copy.path)
		}
	}
}

func changedPathSet(files []gitstate.ChangedFile) map[string]bool {
	paths := make(map[string]bool, len(files))
	for _, file := range files {
		paths[file.Path] = true
	}
	return paths
}

func nextConflictCopyPath(path string) (string, error) {
	first := path + ".staterelay-source"
	for attempt := 0; attempt < 1000; attempt++ {
		candidate := first
		if attempt > 0 {
			candidate = fmt.Sprintf("%s.%d", first, attempt+1)
		}
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		} else if err != nil {
			return "", fmt.Errorf("inspect conflict copy path %s: %w", candidate, err)
		}
	}
	return "", fmt.Errorf("could not allocate conflict copy path for %s", path)
}

func printEditorState(stdout io.Writer, editor *editorstate.State) {
	if editor == nil {
		return
	}
	fmt.Fprintf(stdout, "Editor state: %d open file(s)\n", len(editor.OpenFiles))
	if editor.ActiveFile != nil {
		fmt.Fprintf(stdout, "Active editor file: %s\n", *editor.ActiveFile)
	}
}

func printEditorApplyResult(stdout io.Writer, action string, editor *editorstate.State) {
	if editor == nil {
		return
	}
	fmt.Fprintf(stdout, "%s editor state: %d open file(s)\n", action, len(editor.OpenFiles))
}

func printTerminalState(stdout io.Writer, terminal *terminalstate.State) {
	if terminal == nil {
		return
	}
	fmt.Fprintf(stdout, "Terminal directories: %d\n", len(terminal.WorkingDirectories))
}

func printTerminalApplyResult(stdout io.Writer, action string, terminal *terminalstate.State) {
	if terminal == nil {
		return
	}
	fmt.Fprintf(stdout, "%s terminal directories: %d\n", action, len(terminal.WorkingDirectories))
}

func printBrowserState(stdout io.Writer, browser *browserstate.State) {
	if browser == nil {
		return
	}
	fmt.Fprintf(stdout, "Browser tabs: %d\n", len(browser.Tabs))
}

func printBrowserApplyResult(stdout io.Writer, action string, browser *browserstate.State) {
	if browser == nil {
		return
	}
	fmt.Fprintf(stdout, "%s browser tabs: %d\n", action, len(browser.Tabs))
}

func printSessionSignature(stdout io.Writer, captured session.Session, indent string) {
	if captured.Signature == nil {
		fmt.Fprintf(stdout, "%sSignature: unsigned\n", indent)
		return
	}
	fmt.Fprintf(stdout, "%sSignature: signed by %s (%s)\n", indent, captured.Signature.DeviceName, shortHash(captured.Signature.Fingerprint))
}

func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func shortHash(hash string) string {
	if len(hash) <= 12 {
		return hash
	}
	return hash[:12]
}

func safePath(root string, path string) (string, bool) {
	cleanPath := filepath.Clean(path)
	if filepath.IsAbs(cleanPath) || cleanPath == ".." || strings.HasPrefix(cleanPath, "../") {
		return "", false
	}
	return filepath.Join(root, cleanPath), true
}

func samePath(left string, right string) bool {
	leftInfo, leftErr := os.Stat(left)
	rightInfo, rightErr := os.Stat(right)
	if leftErr == nil && rightErr == nil {
		return os.SameFile(leftInfo, rightInfo)
	}

	return left == right
}

func printHelp(w io.Writer) {
	fmt.Fprintln(w, "StateRelay")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  relay capture [--path PATH] [--json] [--out FILE]")
	fmt.Fprintln(w, "  relay restore [--path PATH] [--clone-dir DIR] [--inbox DIR] [--apply] [--dry-run] [--conflict reject|keep-both] [--require-trusted] SESSION_FILE|latest")
	fmt.Fprintln(w, "  relay listen [--addr ADDR] [--inbox DIR] [--history DB] [--advertise] [--tls] [--require-client-cert] [--path PATH]")
	fmt.Fprintln(w, "  relay devices [--timeout DURATION]")
	fmt.Fprintln(w, "  relay inbox [--inbox DIR]")
	fmt.Fprintln(w, "  relay history [--history DB] [--limit N]")
	fmt.Fprintln(w, "  relay doctor [--path PATH] [--inbox DIR] [--history DB] [--to URL]")
	fmt.Fprintln(w, "  relay terminal [--path PATH] [--cwd DIR]")
	fmt.Fprintln(w, "  relay browser [--path PATH] --url URL [--url URL...]")
	fmt.Fprintln(w, "  relay browser [--path PATH] --restore [--open]")
	fmt.Fprintln(w, "  relay identity [--path PATH] [--name NAME]")
	fmt.Fprintln(w, "  relay pair-code --peer-fingerprint FINGERPRINT [--path PATH]")
	fmt.Fprintln(w, "  relay trust add --name NAME --fingerprint FINGERPRINT [--path PATH]")
	fmt.Fprintln(w, "  relay trust list [--path PATH]")
	fmt.Fprintln(w, "  relay trust remove --fingerprint FINGERPRINT [--path PATH]")
	fmt.Fprintln(w, "  relay ping --to URL [--insecure-tls] [--client-cert] [--path PATH]")
	fmt.Fprintln(w, "  relay send --to URL [--insecure-tls] [--client-cert] [--path PATH] SESSION_FILE")
	fmt.Fprintln(w, "  relay version")
}
