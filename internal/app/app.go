// Package app is the orchestrator behind every CLI command.
//
// Convention: cmd/ files are thin (Cobra wiring, flag parsing,
// delegation). All business logic - validation, I/O, state mutation -
// lives here so it can be unit-tested without spinning up Cobra. A cmd
// function should look like:
//
//	RunE: func(cmd *cobra.Command, args []string) error {
//	    a := app.New()
//	    return a.DoTheThing(cmd.Context(), parseInput(args))
//	}
//
// Errors returned from App methods that wrap an exitcode sentinel
// (see internal/exitcode) will translate to the right exit code on
// the way out.
package app

import (
	"io"
	"os"
)

// App owns the runtime dependencies the commands share. No domain
// methods ship by default - add fields and methods as you wire up
// your first real command (a logger, a config, an HTTP client, etc.).
type App struct {
	out io.Writer
}

// Options is the test-friendly constructor input. Zero-valued fields
// fall back to production defaults; tests fill in only the bits they
// care about.
type Options struct {
	// Out is where user-facing command output goes. Defaults to
	// os.Stdout when nil.
	Out io.Writer
}

// New builds an App with default dependencies. Production callers use
// this; tests use NewWithOptions.
func New() *App {
	return NewWithOptions(Options{})
}

// NewWithOptions builds an App from explicit options, substituting
// production defaults for any zero-valued field.
func NewWithOptions(opts Options) *App {
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	return &App{out: opts.Out}
}

// Out returns the writer App was constructed with. cmd/ render helpers
// use this so test output goes to the same buffer the test injected.
func (a *App) Out() io.Writer { return a.out }
