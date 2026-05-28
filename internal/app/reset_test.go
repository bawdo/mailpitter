package app

import (
	"context"
	"testing"

	"github.com/bawdo/mailpitter/internal/docker"
	"github.com/bawdo/mailpitter/internal/state"
)

func TestResetWipesContainerAndVolume(t *testing.T) {
	a, fd, _ := newTestApp(t)
	if err := a.Install(context.Background(), InstallInput{}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if err := a.Reset(context.Background()); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	if _, ok := fd.States[ContainerName]; ok && fd.States[ContainerName] != docker.StateAbsent {
		t.Errorf("container should be gone, got %v", fd.States[ContainerName])
	}
	if fd.Volumes[DataVolume] {
		t.Errorf("volume should be removed")
	}
	s, _ := state.Load(a.paths.StatePath)
	if s.Install != nil {
		t.Errorf("install block should be cleared, got %+v", s.Install)
	}
}

func TestResetPreservesConsumers(t *testing.T) {
	a, _, _ := newTestApp(t)
	if err := a.Install(context.Background(), InstallInput{}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if _, err := a.Register(context.Background(), RegisterInput{Name: "taiga", Service: "taiga-back"}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := a.Reset(context.Background()); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	s, _ := state.Load(a.paths.StatePath)
	if _, ok := s.Consumers["taiga"]; !ok {
		t.Errorf("consumers should be preserved across reset")
	}
}
