package app

import (
	"bytes"
	"os"
	"testing"

	"github.com/bawdo/mailpitter/internal/docker"
)

func TestNewUsesProductionDefaults(t *testing.T) {
	a := New()
	if a.Out() != os.Stdout {
		t.Errorf("Out: want os.Stdout, got %v", a.Out())
	}
	if a.docker == nil {
		t.Errorf("docker client should be set")
	}
	if a.caddy == nil {
		t.Errorf("caddy client should be set")
	}
	if a.mailpit == nil {
		t.Errorf("mailpit client should be set")
	}
	if a.paths.StatePath == "" {
		t.Errorf("paths should be populated")
	}
}

func TestNewWithOptionsHonoursOverrides(t *testing.T) {
	var buf bytes.Buffer
	fakeDocker := docker.NewFake()
	a := NewWithOptions(Options{
		Out:    &buf,
		Docker: fakeDocker,
	})
	if a.Out() != &buf {
		t.Errorf("Out not honoured")
	}
	if a.docker != fakeDocker {
		t.Errorf("Docker not honoured")
	}
}

func TestNewWithOptionsZeroValuesFallBack(t *testing.T) {
	a := NewWithOptions(Options{})
	if a.Out() != os.Stdout {
		t.Errorf("zero-valued Out should default to stdout")
	}
	if a.docker == nil {
		t.Errorf("zero-valued Docker should default")
	}
}
