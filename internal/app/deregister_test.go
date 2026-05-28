package app

import (
	"context"
	"testing"

	"github.com/bawdo/mailpitter/internal/state"
)

func TestDeregisterRemovesEntry(t *testing.T) {
	a, _, _ := newTestApp(t)
	if _, err := a.Register(context.Background(), RegisterInput{Name: "taiga", Service: "taiga-back"}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	notice, err := a.Deregister(context.Background(), "taiga")
	if err != nil {
		t.Fatalf("Deregister: %v", err)
	}
	if notice != "" {
		t.Errorf("known consumer should produce no notice, got %q", notice)
	}
	s, _ := state.Load(a.paths.StatePath)
	if _, present := s.Consumers["taiga"]; present {
		t.Errorf("entry not removed")
	}
}

func TestDeregisterUnknownReturnsNotice(t *testing.T) {
	a, _, _ := newTestApp(t)
	notice, err := a.Deregister(context.Background(), "ghost")
	if err != nil {
		t.Fatalf("Deregister: %v", err)
	}
	if notice == "" {
		t.Errorf("unknown consumer should produce a notice")
	}
}
