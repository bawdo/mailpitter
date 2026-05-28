package app

import (
	"context"
	"strings"
	"testing"

	"github.com/bawdo/mailpitter/internal/docker"
)

// These tests pin the user-visible output of state-changing commands.
// Previously install/start/stop were silent on success, which made it
// impossible for users to tell whether caddy_shack registration had
// actually happened. Each successful caddy_shack action should now
// announce itself on the notices writer (stderr in production).

func TestInstallEmitsCaddyShackRegisterNotice(t *testing.T) {
	a, _, _, notices := newTestAppWithNotices(t)
	if err := a.Install(context.Background(), InstallInput{}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	got := notices.String()
	if !strings.Contains(got, "registered mailpit.test with caddy_shack") {
		t.Errorf("notices missing register confirmation:\n%s", got)
	}
}

func TestStartEmitsCaddyShackRegisterNotice(t *testing.T) {
	a, fd, _, notices := newTestAppWithNotices(t)
	fd.States[ContainerName] = docker.StateStopped
	if err := a.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	got := notices.String()
	if !strings.Contains(got, "registered mailpit.test with caddy_shack") {
		t.Errorf("notices missing register confirmation:\n%s", got)
	}
}

func TestStopEmitsCaddyShackDeregisterNotice(t *testing.T) {
	a, fd, _, notices := newTestAppWithNotices(t)
	fd.States[ContainerName] = docker.StateRunning
	if err := a.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	got := notices.String()
	if !strings.Contains(got, "deregistered mailpit.test from caddy_shack") {
		t.Errorf("notices missing deregister confirmation:\n%s", got)
	}
}
