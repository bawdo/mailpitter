package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bawdo/mailpitter/internal/docker"
	"github.com/bawdo/mailpitter/internal/exitcode"
	"github.com/bawdo/mailpitter/internal/mailpitapi"
	"github.com/bawdo/mailpitter/internal/state"
)

// seedInstall writes an install record so Upgrade sees an installed system.
func seedInstall(t *testing.T, a *App, mode state.Mode) {
	t.Helper()
	s, err := state.Load(a.paths.StatePath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	s.Install = &state.Install{Mode: mode, Image: Image, InstalledAt: "2026-01-01T00:00:00Z"}
	if err := state.Save(a.paths.StatePath, s); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func TestUpgradeNotInstalled(t *testing.T) {
	a, _, _ := newTestApp(t)
	err := a.Upgrade(context.Background())
	if !errors.Is(err, exitcode.ErrMailpitNotInstalled) {
		t.Errorf("expected ErrMailpitNotInstalled, got %v", err)
	}
}

func TestUpgradeContainerAbsent(t *testing.T) {
	a, _, _ := newTestApp(t)
	seedInstall(t, a, state.ModePersistent)
	// container state defaults to absent in the fake
	err := a.Upgrade(context.Background())
	if !errors.Is(err, exitcode.ErrMailpitNotInstalled) {
		t.Errorf("expected ErrMailpitNotInstalled, got %v", err)
	}
}

func TestUpgradeDaemonDown(t *testing.T) {
	a, fd, _ := newTestApp(t)
	fd.DaemonErr = errors.New("daemon down")
	err := a.Upgrade(context.Background())
	if !errors.Is(err, exitcode.ErrDockerUnreachable) {
		t.Errorf("expected ErrDockerUnreachable, got %v", err)
	}
}

func TestUpgradeAlreadyLatest(t *testing.T) {
	a, fd, fc, notices := newTestAppWithNotices(t)
	seedInstall(t, a, state.ModePersistent)
	fd.States[ContainerName] = docker.StateRunning
	fd.ImageIDs[LatestImage] = "sha256:same"
	fd.ContainerImageIDs[ContainerName] = "sha256:same"
	a.mailpit = &fakeMailpitWithInfo{info: mailpitapi.Info{Version: "1.27.3"}}

	if err := a.Upgrade(context.Background()); err != nil {
		t.Fatalf("Upgrade: %v", err)
	}
	for _, c := range fd.Calls {
		if c == "Remove:"+ContainerName || c == "CreateAndStart:"+ContainerName {
			t.Errorf("already-latest upgrade must not recreate; calls=%v", fd.Calls)
		}
	}
	if len(fc.registers) != 0 {
		t.Errorf("already-latest upgrade must not re-register, got %v", fc.registers)
	}
	if !strings.Contains(notices.String(), "already up to date (1.27.3)") {
		t.Errorf("notices = %q", notices.String())
	}
	loaded, _ := state.Load(a.paths.StatePath)
	if loaded.Install.Image != Image {
		t.Errorf("state image should be unchanged, got %q", loaded.Install.Image)
	}
}

func TestUpgradePersistentRecreate(t *testing.T) {
	a, fd, fc, notices := newTestAppWithNotices(t)
	seedInstall(t, a, state.ModePersistent)
	fd.States[ContainerName] = docker.StateRunning
	fd.ImageIDs[LatestImage] = "sha256:new"
	fd.ContainerImageIDs[ContainerName] = "sha256:old"
	a.mailpit = &fakeMailpitWithInfo{info: mailpitapi.Info{Version: "1.27.3"}}

	if err := a.Upgrade(context.Background()); err != nil {
		t.Fatalf("Upgrade: %v", err)
	}
	if !fd.Images[LatestImage] {
		t.Errorf("latest image should have been pulled; calls=%v", fd.Calls)
	}
	if fd.LastSpec.Image != LatestImage {
		t.Errorf("container recreated with image %q, want %q", fd.LastSpec.Image, LatestImage)
	}
	if len(fd.LastSpec.Volumes) != 1 {
		t.Errorf("persistent recreate should mount the volume, got %d", len(fd.LastSpec.Volumes))
	}
	if !fd.Volumes[DataVolume] {
		t.Errorf("data volume should be ensured")
	}
	if len(fc.registers) != 1 || fc.registers[0] != "mailpit|mailpit.test" {
		t.Errorf("caddy register = %v", fc.registers)
	}
	loaded, _ := state.Load(a.paths.StatePath)
	if loaded.Install.Image != LatestImage {
		t.Errorf("state image = %q, want %q", loaded.Install.Image, LatestImage)
	}
	out := notices.String()
	if !strings.Contains(out, "upgraded to 1.27.3") || !strings.Contains(out, "registered mailpit.test") {
		t.Errorf("notices = %q", out)
	}
}

func TestUpgradeEphemeralRecreate(t *testing.T) {
	a, fd, _, _ := newTestAppWithNotices(t)
	seedInstall(t, a, state.ModeEphemeral)
	fd.States[ContainerName] = docker.StateRunning
	fd.ImageIDs[LatestImage] = "sha256:new"
	fd.ContainerImageIDs[ContainerName] = "sha256:old"
	a.mailpit = &fakeMailpitWithInfo{info: mailpitapi.Info{Version: "1.27.3"}}

	if err := a.Upgrade(context.Background()); err != nil {
		t.Fatalf("Upgrade: %v", err)
	}
	if fd.Volumes[DataVolume] {
		t.Errorf("ephemeral recreate should not ensure a volume")
	}
	if len(fd.LastSpec.Volumes) != 0 {
		t.Errorf("ephemeral recreate should mount no volumes, got %d", len(fd.LastSpec.Volumes))
	}
	if fd.LastSpec.Env != nil {
		t.Errorf("ephemeral recreate should have nil env, got %v", fd.LastSpec.Env)
	}
}

func TestUpgradeCaddyFailure(t *testing.T) {
	a, fd, fc, _ := newTestAppWithNotices(t)
	seedInstall(t, a, state.ModePersistent)
	fd.States[ContainerName] = docker.StateRunning
	fd.ImageIDs[LatestImage] = "sha256:new"
	fd.ContainerImageIDs[ContainerName] = "sha256:old"
	fc.registerErr = errors.New("caddy boom")
	a.mailpit = &fakeMailpitWithInfo{info: mailpitapi.Info{Version: "1.27.3"}}

	err := a.Upgrade(context.Background())
	if !errors.Is(err, exitcode.ErrCaddyShackFailure) {
		t.Errorf("expected ErrCaddyShackFailure, got %v", err)
	}
}

func TestUpgradeVersionUnknownFallback(t *testing.T) {
	a, fd, _, notices := newTestAppWithNotices(t)
	seedInstall(t, a, state.ModePersistent)
	fd.States[ContainerName] = docker.StateRunning
	fd.ImageIDs[LatestImage] = "sha256:new"
	fd.ContainerImageIDs[ContainerName] = "sha256:old"
	a.mailpit = &fakeMailpitWithInfo{err: errors.New("not ready")}

	if err := a.Upgrade(context.Background()); err != nil {
		t.Fatalf("Upgrade: %v", err)
	}
	if !strings.Contains(notices.String(), "upgrade complete; run mailpitter status") {
		t.Errorf("notices = %q", notices.String())
	}
}
