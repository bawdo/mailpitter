package app

import (
	"errors"
	"testing"

	"github.com/bawdo/mailpitter/internal/docker"
	"github.com/bawdo/mailpitter/internal/mailpitapi"
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
