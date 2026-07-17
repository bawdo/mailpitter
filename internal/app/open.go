package app

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
)

// execCommand is a seam over exec.Command so tests can stub the browser
// launch instead of really opening one. Production uses exec.Command.
var execCommand = exec.Command

// goos is a seam over runtime.GOOS so tests can exercise both the darwin
// and non-darwin branches regardless of the host.
var goos = runtime.GOOS

// Open launches the system browser at https://mailpit.test. macOS uses
// `open`; other platforms return a friendly message (we don't try to
// guess between xdg-open variants).
func (a *App) Open(_ context.Context) error {
	url := "https://" + Domain
	switch goos {
	case "darwin":
		return execCommand("open", url).Run()
	default:
		_, _ = fmt.Fprintf(a.out, "auto-open not supported on %s; visit %s manually\n", goos, url)
		return nil
	}
}
