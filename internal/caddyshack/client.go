// Package caddyshack is a thin shell-out wrapper around the caddy_shack
// binary on PATH. Used by mailpitter's install/start/stop and doctor.
package caddyshack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
)

// Public sentinels callers match with errors.Is. Map to documented
// caddy_shack exit codes (see caddy_shack's README).
var (
	ErrNotInstalled        = errors.New("caddy_shack not installed")
	ErrUnhealthy           = errors.New("caddy_shack reports unhealthy")
	ErrDockerUnreachable   = errors.New("caddy_shack: docker daemon unreachable")
	ErrCaddyNotRunning     = errors.New("caddy_shack: caddy container not running")
	ErrDomainConflict      = errors.New("caddy_shack: domain conflict")
	ErrStateCorrupt        = errors.New("caddy_shack: state file corrupt")
	ErrStateLockContention = errors.New("caddy_shack: state lock held")
	ErrShellOutFailed      = errors.New("caddy_shack: shell-out failed")
)

// runFunc is the injectable executor. Production wires it to exec.CommandContext;
// tests substitute their own.
type runFunc func(ctx context.Context, name string, args ...string) (stdout, stderr string, exit int, err error)

// Client wraps the caddy_shack binary.
type Client struct {
	run runFunc
}

// New builds a production Client.
func New() *Client { return newClientWithRunner(runProcess) }

func newClientWithRunner(r runFunc) *Client { return &Client{run: r} }

// lookPath is exec.LookPath; tests substitute their own implementation.
var lookPath = exec.LookPath

// Installed returns nil when the caddy_shack binary is found on $PATH,
// or ErrNotInstalled otherwise. This is the right primitive for asking
// "is caddy_shack itself installed", distinct from asking caddy_shack
// to introspect its own container, which is what Status does.
func (c *Client) Installed() error {
	if _, err := lookPath("caddy_shack"); err != nil {
		return fmt.Errorf("%w: %v", ErrNotInstalled, err)
	}
	return nil
}

func runProcess(ctx context.Context, name string, args ...string) (string, string, int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exit := 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exit = ee.ExitCode()
		} else {
			return stdout.String(), stderr.String(), -1, err
		}
	}
	return stdout.String(), stderr.String(), exit, err
}

// Register calls `caddy_shack register <name> --port <port> --domain <domain>`.
func (c *Client) Register(ctx context.Context, name, domain string, port int) error {
	_, stderr, exit, err := c.run(ctx, "caddy_shack",
		"register", name, "--port", fmt.Sprintf("%d", port), "--domain", domain)
	if exit == 0 && err == nil {
		return nil
	}
	return c.translate(exit, stderr, err)
}

// Deregister calls `caddy_shack deregister <name>`.
func (c *Client) Deregister(ctx context.Context, name string) error {
	_, stderr, exit, err := c.run(ctx, "caddy_shack", "deregister", name)
	if exit == 0 && err == nil {
		return nil
	}
	return c.translate(exit, stderr, err)
}

// Status holds the parsed output of `caddy_shack status --json`. Field
// tags mirror caddy_shack's internal/app.StatusReport; keep them in sync
// with upstream. The realStatusJSON test constant pins this shape
// against real terminal output.
type Status struct {
	ContainerState string `json:"container_state"`
	AdminReachable bool   `json:"admin_reachable"`
	BackendCount   int    `json:"backend_count"`
	Healthy        bool   `json:"healthy"`
}

// Status calls `caddy_shack status --json` and returns the parsed result.
//
// caddy_shack legitimately exits 1 (ErrUnhealthy) when caddy is stopped
// or unreachable, while still emitting the full JSON report. When that
// happens, Status returns the parsed Status struct AND an ErrUnhealthy
// error so callers can inspect what's wrong rather than getting an
// opaque shell-out failure.
func (c *Client) Status(ctx context.Context) (Status, error) {
	stdout, stderr, exit, err := c.run(ctx, "caddy_shack", "status", "--json")

	var st Status
	parseErr := json.Unmarshal([]byte(stdout), &st)

	if exit == 0 && err == nil {
		if parseErr != nil {
			return Status{}, fmt.Errorf("caddy_shack status: parse JSON: %w", parseErr)
		}
		return st, nil
	}

	// Non-zero exit. If the JSON parsed cleanly, return it alongside the
	// translated error so callers see structured state.
	if parseErr == nil {
		return st, c.translate(exit, stderr, err)
	}
	return Status{}, c.translate(exit, stderr, err)
}

// Backend is one row of `caddy_shack list --json`. Field tags mirror
// caddy_shack's internal/app.Listing.
type Backend struct {
	Name         string `json:"name"`
	Domain       string `json:"domain"`
	Upstream     string `json:"upstream"`
	RegisteredAt string `json:"registered_at"`
}

// List calls `caddy_shack list --json` and returns the registered backends.
// caddy_shack's list command does not depend on caddy being running, so
// unhealthy-but-installed caddy_shack still returns the full list.
func (c *Client) List(ctx context.Context) ([]Backend, error) {
	stdout, stderr, exit, err := c.run(ctx, "caddy_shack", "list", "--json")
	if exit != 0 || err != nil {
		return nil, c.translate(exit, stderr, err)
	}
	var backends []Backend
	if err := json.Unmarshal([]byte(stdout), &backends); err != nil {
		return nil, fmt.Errorf("caddy_shack list: parse JSON: %w", err)
	}
	return backends, nil
}

// translate maps caddy_shack's documented exit codes to package sentinels.
func (c *Client) translate(exit int, stderr string, runErr error) error {
	switch exit {
	case 0:
		return nil
	case 1:
		return fmt.Errorf("%w: %s", ErrUnhealthy, stderr)
	case 3:
		return fmt.Errorf("%w: %s", ErrDomainConflict, stderr)
	case 4:
		return fmt.Errorf("%w: %s", ErrDockerUnreachable, stderr)
	case 5:
		return fmt.Errorf("%w: %s", ErrCaddyNotRunning, stderr)
	case 6:
		return fmt.Errorf("%w: %s", ErrNotInstalled, stderr)
	case 7:
		return fmt.Errorf("%w: %s", ErrStateCorrupt, stderr)
	case 8:
		return fmt.Errorf("%w: %s", ErrStateLockContention, stderr)
	default:
		return fmt.Errorf("%w: exit %d: %s: %v", ErrShellOutFailed, exit, stderr, runErr)
	}
}
