package browserstate

import "testing"

func TestOpenerCommandUsesPlatformOpeners(t *testing.T) {
	tests := []struct {
		goos string
		name string
		args []string
	}{
		{goos: "darwin", name: "open", args: []string{"https://go.dev/doc/"}},
		{goos: "windows", name: "rundll32", args: []string{"url.dll,FileProtocolHandler", "https://go.dev/doc/"}},
		{goos: "linux", name: "xdg-open", args: []string{"https://go.dev/doc/"}},
	}

	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			name, args, err := openerCommand(tt.goos, "https://go.dev/doc/")
			if err != nil {
				t.Fatalf("openerCommand returned error: %v", err)
			}
			if name != tt.name {
				t.Fatalf("name = %q, want %q", name, tt.name)
			}
			if len(args) != len(tt.args) {
				t.Fatalf("args = %#v", args)
			}
			for i := range args {
				if args[i] != tt.args[i] {
					t.Fatalf("args = %#v, want %#v", args, tt.args)
				}
			}
		})
	}
}

func TestOpenerCommandRejectsInvalidURL(t *testing.T) {
	_, _, err := openerCommand("darwin", "file:///tmp/session.html")
	if err == nil {
		t.Fatal("openerCommand returned nil error for invalid URL")
	}
	if err.Error() != `unsupported scheme "file"` {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestOpenerCommandRejectsUnsupportedOS(t *testing.T) {
	_, _, err := openerCommand("plan9", "https://go.dev/doc/")
	if err == nil {
		t.Fatal("openerCommand returned nil error for unsupported OS")
	}
	if err.Error() != "browser restore is not supported on plan9" {
		t.Fatalf("error = %q", err.Error())
	}
}
