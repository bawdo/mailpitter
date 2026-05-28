package app

import (
	"context"
	"errors"
	"testing"

	"github.com/bawdo/mailpitter/internal/docker"
	"github.com/bawdo/mailpitter/internal/exitcode"
)

func TestStartFromStopped(t *testing.T) {
	a, fd, fc := newTestApp(t)
	fd.States[ContainerName] = docker.StateStopped
	if err := a.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if fd.States[ContainerName] != docker.StateRunning {
		t.Errorf("not started")
	}
	if len(fc.registers) != 1 {
		t.Errorf("caddy_shack should be re-registered, got %d", len(fc.registers))
	}
}

func TestStartWhenAbsentErrors(t *testing.T) {
	a, fd, _ := newTestApp(t)
	fd.States[ContainerName] = docker.StateAbsent
	err := a.Start(context.Background())
	if !errors.Is(err, exitcode.ErrMailpitNotInstalled) {
		t.Errorf("got %v", err)
	}
}

func TestStartIdempotentWhenRunning(t *testing.T) {
	a, fd, fc := newTestApp(t)
	fd.States[ContainerName] = docker.StateRunning
	if err := a.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// No docker Start call; only caddy_shack re-register.
	for _, c := range fd.Calls {
		if c == "Start:"+ContainerName {
			t.Errorf("should not Start an already-running container")
		}
	}
	if len(fc.registers) != 1 {
		t.Errorf("caddy_shack should be re-registered for idempotency, got %d", len(fc.registers))
	}
}

func TestStopDeregistersThenStops(t *testing.T) {
	a, fd, fc := newTestApp(t)
	fd.States[ContainerName] = docker.StateRunning
	if err := a.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if len(fc.deregisters) != 1 {
		t.Errorf("caddy_shack deregister not called: %v", fc.deregisters)
	}
	if fd.States[ContainerName] != docker.StateStopped {
		t.Errorf("not stopped")
	}
}

func TestStopWhenAlreadyStopped(t *testing.T) {
	a, fd, fc := newTestApp(t)
	fd.States[ContainerName] = docker.StateStopped
	if err := a.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if len(fc.deregisters) != 1 {
		t.Errorf("caddy_shack deregister should still happen for safety: %v", fc.deregisters)
	}
}

func TestRestartGoesStopThenStart(t *testing.T) {
	a, fd, fc := newTestApp(t)
	fd.States[ContainerName] = docker.StateRunning
	if err := a.Restart(context.Background()); err != nil {
		t.Fatalf("Restart: %v", err)
	}
	if fd.States[ContainerName] != docker.StateRunning {
		t.Errorf("should end running")
	}
	if len(fc.deregisters) != 1 || len(fc.registers) != 1 {
		t.Errorf("expected exactly one deregister and one register, got d=%d r=%d",
			len(fc.deregisters), len(fc.registers))
	}
}
