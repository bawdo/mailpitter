package app

import (
	"context"
	"errors"
	"testing"

	"github.com/bawdo/mailpitter/internal/docker"
	"github.com/bawdo/mailpitter/internal/mailpitapi"
)

type fakeMailpitWithInfo struct {
	info mailpitapi.Info
	err  error
}

func (f *fakeMailpitWithInfo) Info() (mailpitapi.Info, error) { return f.info, f.err }

func TestStatusHealthy(t *testing.T) {
	a, fd, _ := newTestApp(t)
	fd.States[ContainerName] = docker.StateRunning
	a.mailpit = &fakeMailpitWithInfo{info: mailpitapi.Info{Version: "1.21.0", Messages: 7}}

	rep, err := a.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if rep.Container != "running" {
		t.Errorf("Container = %q", rep.Container)
	}
	if !rep.APIReachable {
		t.Errorf("APIReachable should be true")
	}
	if rep.Messages != 7 || rep.MailpitVersion != "1.21.0" {
		t.Errorf("got %+v", rep)
	}
}

func TestStatusContainerStopped(t *testing.T) {
	a, fd, _ := newTestApp(t)
	fd.States[ContainerName] = docker.StateStopped
	a.mailpit = &fakeMailpitWithInfo{err: errors.New("no listener")}
	rep, err := a.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if rep.Container != "stopped" || rep.APIReachable {
		t.Errorf("got %+v", rep)
	}
}
