package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bawdo/mailpitter/internal/exitcode"
	"github.com/bawdo/mailpitter/internal/state"
)

func TestRegisterValidatesName(t *testing.T) {
	a, _, _ := newTestApp(t)
	for _, bad := range []string{"", "BadCase", "with space", strings.Repeat("a", 32)} {
		_, err := a.Register(context.Background(), RegisterInput{Name: bad})
		if !errors.Is(err, exitcode.ErrInvalidArgs) {
			t.Errorf("name %q: want ErrInvalidArgs, got %v", bad, err)
		}
	}
}

func TestRegisterWritesStateAndReturnsSnippet(t *testing.T) {
	a, _, _ := newTestApp(t)
	out, err := a.Register(context.Background(), RegisterInput{
		Name: "taiga", Service: "taiga-back", MailVars: MailVarsDjango,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if !strings.Contains(out, "EMAIL_HOST=mailpit.test") {
		t.Errorf("snippet missing env block:\n%s", out)
	}
	s, err := state.Load(a.paths.StatePath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	c, ok := s.Consumers["taiga"]
	if !ok {
		t.Fatalf("consumer not stored")
	}
	if c.Service != "taiga-back" {
		t.Errorf("service = %q", c.Service)
	}
}

func TestRegisterIdempotentNoOp(t *testing.T) {
	a, _, _ := newTestApp(t)
	if _, err := a.Register(context.Background(), RegisterInput{Name: "taiga", Service: "taiga-back"}); err != nil {
		t.Fatalf("first: %v", err)
	}
	out, err := a.Register(context.Background(), RegisterInput{Name: "taiga", Service: "taiga-back"})
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if out != "" {
		t.Errorf("idempotent re-register should return empty snippet, got:\n%s", out)
	}
}
