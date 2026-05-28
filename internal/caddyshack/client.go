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
// caddy_shack exit codes.
var (
	ErrNotInstalled      = errors.New("caddy_shack not installed")
	ErrDockerUnreachable = errors.New("caddy_shack: docker daemon unreachable")
	ErrCaddyNotRunning   = errors.New("caddy_shack: caddy container not running")
	ErrDomainConflict    = errors.New("caddy_shack: domain conflict")
	ErrShellOutFailed    = errors.New("caddy_shack: shell-out failed")
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

// Status holds the parsed output of `caddy_shack status --json`.
type Status struct {
	Container string        `json:"container"`
	AdminAPI  StatusAdminAPI `json:"admin_api"`
	Backends  []StatusBackend `json:"backends"`
}

// StatusAdminAPI reports whether the caddy admin API is reachable.
type StatusAdminAPI struct {
	Reachable bool `json:"reachable"`
}

// StatusBackend is one registered reverse-proxy backend.
type StatusBackend struct {
	Name     string `json:"name"`
	Domain   string `json:"domain"`
	Upstream string `json:"upstream"`
}

// Status calls `caddy_shack status --json` and returns the parsed result.
func (c *Client) Status(ctx context.Context) (Status, error) {
	stdout, stderr, exit, err := c.run(ctx, "caddy_shack", "status", "--json")
	if exit != 0 || err != nil {
		return Status{}, c.translate(exit, stderr, err)
	}
	var st Status
	if err := json.Unmarshal([]byte(stdout), &st); err != nil {
		return Status{}, fmt.Errorf("caddy_shack status: parse JSON: %w", err)
	}
	return st, nil
}

// translate maps caddy_shack's documented exit codes to package sentinels.
func (c *Client) translate(exit int, stderr string, runErr error) error {
	switch exit {
	case 0:
		return nil
	case 3:
		return fmt.Errorf("%w: %s", ErrDomainConflict, stderr)
	case 4:
		return fmt.Errorf("%w: %s", ErrDockerUnreachable, stderr)
	case 5:
		return fmt.Errorf("%w: %s", ErrCaddyNotRunning, stderr)
	case 6:
		return fmt.Errorf("%w: %s", ErrNotInstalled, stderr)
	default:
		return fmt.Errorf("%w: exit %d: %s: %v", ErrShellOutFailed, exit, stderr, runErr)
	}
}
