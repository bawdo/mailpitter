package app

import (
	"context"
	"os/exec"
	"runtime"
)

// Open launches the system browser at https://mailpit.test. macOS uses
// `open`; other platforms return a friendly message (we don't try to
// guess between xdg-open variants).
func (a *App) Open(_ context.Context) error {
	url := "https://" + Domain
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Run()
	default:
		return Prerequisitef("auto-open not supported on %s; visit %s manually", runtime.GOOS, url)
	}
}
