package browserstate

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
)

func OpenURL(ctx context.Context, rawURL string) error {
	name, args, err := openerCommand(runtime.GOOS, rawURL)
	if err != nil {
		return err
	}
	if err := exec.CommandContext(ctx, name, args...).Start(); err != nil {
		return fmt.Errorf("open browser URL: %w", err)
	}
	return nil
}

func openerCommand(goos string, rawURL string) (string, []string, error) {
	if err := validateURL(rawURL); err != nil {
		return "", nil, err
	}
	switch goos {
	case "darwin":
		return "open", []string{rawURL}, nil
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", rawURL}, nil
	case "linux":
		return "xdg-open", []string{rawURL}, nil
	default:
		return "", nil, fmt.Errorf("browser restore is not supported on %s", goos)
	}
}
