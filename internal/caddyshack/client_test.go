package caddyshack

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeRunner struct {
	gotCmd     string
	gotArgs    []string
	stdout     string
	stderr     string
	exitCode   int
	returnErr  error
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
		{3, ErrDomainConflict},
		{4, ErrDockerUnreachable},
		{5, ErrCaddyNotRunning},
		{6, ErrNotInstalled},
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

func TestStatusSuccess(t *testing.T) {
	payload := `{"container":"running","admin_api":{"reachable":true},"backends":[{"name":"mailpit","domain":"mailpit.test","upstream":"localhost:8025"}]}`
	r := &fakeRunner{exitCode: 0, stdout: payload}
	c := newClientWithRunner(r.run)
	st, err := c.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Container != "running" {
		t.Errorf("Container = %q, want %q", st.Container, "running")
	}
	if !st.AdminAPI.Reachable {
		t.Errorf("AdminAPI.Reachable = false, want true")
	}
	if len(st.Backends) != 1 || st.Backends[0].Name != "mailpit" {
		t.Errorf("Backends = %+v", st.Backends)
	}
	if strings.Join(r.gotArgs, " ") != "status --json" {
		t.Errorf("args = %v", r.gotArgs)
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

func TestStatusInvalidJSON(t *testing.T) {
	r := &fakeRunner{exitCode: 0, stdout: "not json at all"}
	c := newClientWithRunner(r.run)
	_, err := c.Status(context.Background())
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}
