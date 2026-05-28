// Package app is the orchestrator behind every CLI command. Convention:
// cmd/ files are thin (Cobra wiring, flag parsing, delegation). All
// business logic lives here so it can be unit-tested without spinning up
// Cobra. Side-effects live behind interfaces in internal/docker,
// internal/mailpitapi, internal/caddyshack.
package app

import (
	"context"
	"io"
	"os"

	"github.com/bawdo/mailpitter/internal/caddyshack"
	"github.com/bawdo/mailpitter/internal/docker"
	"github.com/bawdo/mailpitter/internal/mailpitapi"
)

// App owns the runtime dependencies the commands share.
type App struct {
	out     io.Writer
	docker  docker.Client
	caddy   CaddyShack
	mailpit MailpitAPI
	paths   Paths
}

// CaddyShack is the subset of internal/caddyshack the app uses. Stated
// as an interface here so tests can swap it.
type CaddyShack interface {
	Installed() error
	Register(ctx context.Context, name, domain string, port int) error
	Deregister(ctx context.Context, name string) error
	Status(ctx context.Context) (caddyshack.Status, error)
	List(ctx context.Context) ([]caddyshack.Backend, error)
}

// MailpitAPI is the subset of internal/mailpitapi the app uses.
type MailpitAPI interface {
	Info() (mailpitapi.Info, error)
}

// Options is the test-friendly constructor input. Zero-valued fields
// fall back to production defaults.
type Options struct {
	Out     io.Writer
	Docker  docker.Client
	Caddy   CaddyShack
	Mailpit MailpitAPI
	Paths   *Paths
}

// New builds an App with production defaults.
func New() *App { return NewWithOptions(Options{}) }

// NewWithOptions builds an App with explicit options, substituting
// production defaults for any zero-valued field.
func NewWithOptions(opts Options) *App {
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	if opts.Docker == nil {
		opts.Docker = docker.NewCLI()
	}
	if opts.Caddy == nil {
		opts.Caddy = caddyshack.New()
	}
	if opts.Mailpit == nil {
		opts.Mailpit = mailpitapi.New(UIBaseURL, nil)
	}
	paths := DefaultPaths()
	if opts.Paths != nil {
		paths = *opts.Paths
	}
	return &App{
		out:     opts.Out,
		docker:  opts.Docker,
		caddy:   opts.Caddy,
		mailpit: opts.Mailpit,
		paths:   paths,
	}
}

// Out returns the writer App was constructed with.
func (a *App) Out() io.Writer { return a.out }
