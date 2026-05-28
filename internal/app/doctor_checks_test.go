package app

import (
	"errors"
	"testing"

	"github.com/bawdo/mailpitter/internal/caddyshack"
	"github.com/bawdo/mailpitter/internal/docker"
	"github.com/bawdo/mailpitter/internal/mailpitapi"
	"github.com/bawdo/mailpitter/internal/state"
)

func TestCheckDockerDaemonPass(t *testing.T) {
	a, _, _ := newTestApp(t)
	r := checkDockerDaemon(a)
	if r.Status != SeverityPass {
		t.Errorf("status = %v", r.Status)
	}
}

func TestCheckDockerDaemonFail(t *testing.T) {
	a, fd, _ := newTestApp(t)
	fd.DaemonErr = errors.New("not running")
	r := checkDockerDaemon(a)
	if r.Status != SeverityFail {
		t.Errorf("status = %v", r.Status)
	}
}

func TestCheckMailpitContainerStates(t *testing.T) {
	cases := []struct {
		state docker.ContainerState
		want  Severity
	}{
		{docker.StateRunning, SeverityPass},
		{docker.StateStopped, SeverityFail},
		{docker.StateAbsent, SeverityFail},
	}
	for _, tc := range cases {
		a, fd, _ := newTestApp(t)
		fd.States[ContainerName] = tc.state
		r := checkMailpitContainer(a)
		if r.Status != tc.want {
			t.Errorf("state %v: status = %v, want %v", tc.state, r.Status, tc.want)
		}
	}
}

func TestCheckMailpitAPIReachable(t *testing.T) {
	a, fd, _ := newTestApp(t)
	fd.States[ContainerName] = docker.StateRunning
	a.mailpit = &fakeMailpitWithInfo{info: mailpitapi.Info{Version: "1.21.0"}}
	r := checkMailpitAPI(a)
	if r.Status != SeverityPass {
		t.Errorf("status = %v", r.Status)
	}
}

func TestCheckMailpitAPISkippedWhenContainerDown(t *testing.T) {
	a, fd, _ := newTestApp(t)
	fd.States[ContainerName] = docker.StateAbsent
	r := checkMailpitAPI(a)
	if r.Status != SeveritySkip {
		t.Errorf("status = %v, want skip", r.Status)
	}
}

func TestCheckCaddyShackInstalledPass(t *testing.T) {
	a, _, _ := newTestApp(t)
	// fakeCaddy.Status returns Status{}, nil by default — no ErrNotInstalled.
	r := checkCaddyShackInstalled(a)
	if r.Status != SeverityPass {
		t.Errorf("status = %v, want PASS", r.Status)
	}
}

func TestCheckCaddyShackInstalledFail(t *testing.T) {
	a, _, fc := newTestApp(t)
	fc.statusErr = caddyshack.ErrNotInstalled
	r := checkCaddyShackInstalled(a)
	if r.Status != SeverityFail {
		t.Errorf("status = %v, want FAIL", r.Status)
	}
	if r.Summary != "caddy_shack not installed" {
		t.Errorf("summary = %q", r.Summary)
	}
}

func TestCheckCaddyShackRunningPass(t *testing.T) {
	a, _, fc := newTestApp(t)
	fc.status = caddyshack.Status{Container: "running", AdminAPI: caddyshack.StatusAdminAPI{Reachable: true}}
	r := checkCaddyShackRunning(a)
	if r.Status != SeverityPass {
		t.Errorf("status = %v, want PASS", r.Status)
	}
}

func TestCheckCaddyShackRunningFail(t *testing.T) {
	a, _, fc := newTestApp(t)
	fc.status = caddyshack.Status{Container: "stopped"}
	r := checkCaddyShackRunning(a)
	if r.Status != SeverityFail {
		t.Errorf("status = %v, want FAIL", r.Status)
	}
}

func TestCheckCaddyShackRunningSkipWhenNotInstalled(t *testing.T) {
	a, _, fc := newTestApp(t)
	fc.statusErr = caddyshack.ErrNotInstalled
	r := checkCaddyShackRunning(a)
	if r.Status != SeveritySkip {
		t.Errorf("status = %v, want SKIP", r.Status)
	}
}

func TestCheckMailpitImagePresent(t *testing.T) {
	a, fd, _ := newTestApp(t)
	fd.Images[Image] = true
	r := checkMailpitImage(a)
	if r.Status != SeverityPass {
		t.Errorf("status = %v, want PASS", r.Status)
	}
}

func TestCheckMailpitImageMissing(t *testing.T) {
	a, _, _ := newTestApp(t)
	r := checkMailpitImage(a)
	if r.Status != SeverityFail {
		t.Errorf("status = %v, want FAIL", r.Status)
	}
}

func TestCheckCaddyRoutePresent(t *testing.T) {
	a, _, fc := newTestApp(t)
	fc.status = caddyshack.Status{
		Container: "running",
		AdminAPI:  caddyshack.StatusAdminAPI{Reachable: true},
		Backends: []caddyshack.StatusBackend{
			{Name: "mailpit", Domain: Domain},
		},
	}
	r := checkCaddyRoute(a)
	if r.Status != SeverityPass {
		t.Errorf("status = %v, want PASS", r.Status)
	}
}

func TestCheckCaddyRouteMissing(t *testing.T) {
	a, _, fc := newTestApp(t)
	fc.status = caddyshack.Status{Container: "running", AdminAPI: caddyshack.StatusAdminAPI{Reachable: true}}
	r := checkCaddyRoute(a)
	if r.Status != SeverityFail {
		t.Errorf("status = %v, want FAIL", r.Status)
	}
}

func TestCheckInstallModeMatchSkip(t *testing.T) {
	a, _, _ := newTestApp(t)
	// No install recorded yet — state file absent.
	r := checkInstallModeMatch(a)
	if r.Status != SeveritySkip {
		t.Errorf("status = %v, want SKIP", r.Status)
	}
}

func TestCheckInstallModeMatchPass(t *testing.T) {
	a, _, _ := newTestApp(t)
	s := &state.State{
		SchemaVersion: state.SchemaVersion,
		Install:       &state.Install{Mode: state.ModePersistent, Image: Image},
		Consumers:     map[string]state.Consumer{},
	}
	if err := state.Save(a.paths.StatePath, s); err != nil {
		t.Fatalf("Save: %v", err)
	}
	r := checkInstallModeMatch(a)
	if r.Status != SeverityPass {
		t.Errorf("status = %v, want PASS", r.Status)
	}
}
