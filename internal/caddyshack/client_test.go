package caddyshack

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeRunner struct {
	gotCmd    string
	gotArgs   []string
	stdout    string
	stderr    string
	exitCode  int
	returnErr error
}

func (f *fakeRunner) run(_ context.Context, cmd string, args ...string) (stdout, stderr string, exit int, err error) {
	f.gotCmd = cmd
	f.gotArgs = args
	return f.stdout, f.stderr, f.exitCode, f.returnErr
}

func TestRegisterSuccess(t *testing.T) {
	r := &fakeRunner{exitCode: 0}
	c := newClientWithRunner(r.run)
	if err := c.Register(context.Background(), "mailpit", "mailpit.test", 8025); err != nil {
		t.Fatalf("Register: %v", err)
	}
	want := []string{"register", "mailpit", "--port", "8025", "--domain", "mailpit.test"}
	if strings.Join(r.gotArgs, " ") != strings.Join(want, " ") {
		t.Errorf("args = %v, want %v", r.gotArgs, want)
	}
}

func TestRegisterTranslatesExitCodes(t *testing.T) {
	cases := []struct {
		exit int
		is   error
	}{
		{1, ErrUnhealthy},
		{3, ErrDomainConflict},
		{4, ErrDockerUnreachable},
		{5, ErrCaddyNotRunning},
		{6, ErrNotInstalled},
		{7, ErrStateCorrupt},
		{8, ErrStateLockContention},
		{99, ErrShellOutFailed},
	}
	for _, tc := range cases {
		r := &fakeRunner{exitCode: tc.exit, returnErr: errors.New("exit")}
		c := newClientWithRunner(r.run)
		err := c.Register(context.Background(), "mailpit", "mailpit.test", 8025)
		if !errors.Is(err, tc.is) {
			t.Errorf("exit %d: got %v, want errors.Is(_, %v)", tc.exit, err, tc.is)
		}
	}
}

func TestDeregisterSuccess(t *testing.T) {
	r := &fakeRunner{exitCode: 0}
	c := newClientWithRunner(r.run)
	if err := c.Deregister(context.Background(), "mailpit"); err != nil {
		t.Fatalf("Deregister: %v", err)
	}
	if strings.Join(r.gotArgs, " ") != "deregister mailpit" {
		t.Errorf("args = %v", r.gotArgs)
	}
}

// realStatusJSON is the literal stdout of `caddy_shack status --json`
// recorded from a healthy install. Pinning the test to real bytes catches
// upstream shape drift that would otherwise leave our struct's fields
// silently zero-valued.
const realStatusJSON = `{
  "container_state": "running",
  "admin_reachable": true,
  "backend_count": 3,
  "healthy": true
}`

func TestStatusParsesRealOutput(t *testing.T) {
	r := &fakeRunner{exitCode: 0, stdout: realStatusJSON}
	c := newClientWithRunner(r.run)
	st, err := c.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.ContainerState != "running" {
		t.Errorf("ContainerState = %q, want %q", st.ContainerState, "running")
	}
	if !st.AdminReachable {
		t.Errorf("AdminReachable = false, want true")
	}
	if st.BackendCount != 3 {
		t.Errorf("BackendCount = %d, want 3", st.BackendCount)
	}
	if !st.Healthy {
		t.Errorf("Healthy = false, want true")
	}
	if strings.Join(r.gotArgs, " ") != "status --json" {
		t.Errorf("args = %v", r.gotArgs)
	}
}

// TestStatusReturnsParsedDataOnExitOne verifies that when caddy_shack
// reports unhealthy (exit 1) it still emits a valid JSON status report,
// and Status surfaces the parsed data alongside ErrUnhealthy so callers
// can inspect what's wrong rather than getting a generic "shell-out failed".
func TestStatusReturnsParsedDataOnExitOne(t *testing.T) {
	unhealthy := `{
  "container_state": "stopped",
  "admin_reachable": false,
  "backend_count": 0,
  "healthy": false
}`
	r := &fakeRunner{exitCode: 1, stdout: unhealthy, returnErr: errors.New("exit")}
	c := newClientWithRunner(r.run)
	st, err := c.Status(context.Background())
	if !errors.Is(err, ErrUnhealthy) {
		t.Fatalf("err = %v, want errors.Is(_, ErrUnhealthy)", err)
	}
	if st.ContainerState != "stopped" {
		t.Errorf("ContainerState = %q, want %q", st.ContainerState, "stopped")
	}
	if st.AdminReachable {
		t.Errorf("AdminReachable should be false")
	}
}

func TestStatusNotInstalled(t *testing.T) {
	r := &fakeRunner{exitCode: 6, returnErr: errors.New("exit")}
	c := newClientWithRunner(r.run)
	_, err := c.Status(context.Background())
	if !errors.Is(err, ErrNotInstalled) {
		t.Errorf("got %v, want errors.Is(_, ErrNotInstalled)", err)
	}
}

func TestStatusInvalidJSONOnExitZero(t *testing.T) {
	r := &fakeRunner{exitCode: 0, stdout: "not json at all"}
	c := newClientWithRunner(r.run)
	_, err := c.Status(context.Background())
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

// realListJSON is the literal stdout of `caddy_shack list --json`
// recorded from a real install. Same drift-protection rationale as
// realStatusJSON.
const realListJSON = `[
  {
    "name": "bookstack",
    "domain": "bookstack.test",
    "upstream": "host.docker.internal:8091",
    "registered_at": "2026-05-28T11:43:13+10:00"
  },
  {
    "name": "mailpit",
    "domain": "mailpit.test",
    "upstream": "host.docker.internal:8025",
    "registered_at": "2026-05-28T13:52:11+10:00"
  },
  {
    "name": "taiga",
    "domain": "taiga.test",
    "upstream": "host.docker.internal:8090",
    "registered_at": "2026-05-28T10:01:33+10:00"
  }
]`

func TestListParsesRealOutput(t *testing.T) {
	r := &fakeRunner{exitCode: 0, stdout: realListJSON}
	c := newClientWithRunner(r.run)
	backends, err := c.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(backends) != 3 {
		t.Fatalf("backends = %d, want 3", len(backends))
	}
	var mailpit *Backend
	for i := range backends {
		if backends[i].Name == "mailpit" {
			mailpit = &backends[i]
			break
		}
	}
	if mailpit == nil {
		t.Fatal("mailpit backend missing")
	}
	if mailpit.Domain != "mailpit.test" {
		t.Errorf("mailpit.Domain = %q", mailpit.Domain)
	}
	if mailpit.Upstream != "host.docker.internal:8025" {
		t.Errorf("mailpit.Upstream = %q", mailpit.Upstream)
	}
	if strings.Join(r.gotArgs, " ") != "list --json" {
		t.Errorf("args = %v", r.gotArgs)
	}
}

func TestListEmpty(t *testing.T) {
	r := &fakeRunner{exitCode: 0, stdout: "[]"}
	c := newClientWithRunner(r.run)
	backends, err := c.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(backends) != 0 {
		t.Errorf("expected empty, got %v", backends)
	}
}

func TestListNotInstalled(t *testing.T) {
	r := &fakeRunner{exitCode: 6, returnErr: errors.New("exit")}
	c := newClientWithRunner(r.run)
	_, err := c.List(context.Background())
	if !errors.Is(err, ErrNotInstalled) {
		t.Errorf("got %v, want errors.Is(_, ErrNotInstalled)", err)
	}
}

func TestInstalledReturnsNilWhenBinaryOnPath(t *testing.T) {
	prev := lookPath
	lookPath = func(name string) (string, error) {
		if name != "caddy_shack" {
			t.Errorf("LookPath called with %q", name)
		}
		return "/usr/local/bin/caddy_shack", nil
	}
	defer func() { lookPath = prev }()
	c := New()
	if err := c.Installed(); err != nil {
		t.Errorf("Installed: %v", err)
	}
}

func TestInstalledReturnsErrNotInstalledWhenBinaryMissing(t *testing.T) {
	prev := lookPath
	lookPath = func(string) (string, error) { return "", errors.New("not found") }
	defer func() { lookPath = prev }()
	c := New()
	err := c.Installed()
	if !errors.Is(err, ErrNotInstalled) {
		t.Errorf("got %v, want errors.Is(_, ErrNotInstalled)", err)
	}
}
