package app

import (
	"context"
	"errors"
	"testing"

	"github.com/bawdo/mailpitter/internal/docker"
)

func TestDoctorReportContainsAllChecks(t *testing.T) {
	a, fd, _ := newTestApp(t)
	fd.States[ContainerName] = docker.StateRunning
	a.mailpit = &fakeMailpitWithInfo{}

	rep, err := a.Doctor(context.Background())
	if err != nil {
		t.Fatalf("Doctor: %v", err)
	}
	want := []string{
		"docker-daemon",
		"caddy_shack-installed",
		"caddy_shack-running",
		"mailpit-image",
		"mailpit-container",
		"mailpit-api",
		"port-1025",
		"port-8025",
		"caddy-route",
		"install-mode",
	}
	if len(rep.Checks) < len(want) {
		t.Fatalf("checks = %d, want >= %d", len(rep.Checks), len(want))
	}
	got := map[string]bool{}
	for _, c := range rep.Checks {
		got[c.Name] = true
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("missing check %q", name)
		}
	}
}

func TestDoctorSummaryCountsFailures(t *testing.T) {
	a, fd, _ := newTestApp(t)
	fd.DaemonErr = errors.New("docker down")
	fd.States[ContainerName] = docker.StateAbsent
	rep, err := a.Doctor(context.Background())
	if err != nil {
		t.Fatalf("Doctor: %v", err)
	}
	if rep.Summary.Fail < 2 {
		t.Errorf("summary should count multiple FAILs, got %+v", rep.Summary)
	}
}
