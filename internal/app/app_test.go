package app

import (
	"bytes"
	"os"
	"testing"
)

func TestNewUsesStdout(t *testing.T) {
	a := New()
	if a.Out() != os.Stdout {
		t.Errorf("New().Out(): want os.Stdout, got %v", a.Out())
	}
}

func TestNewWithOptionsHonoursExplicitOut(t *testing.T) {
	var buf bytes.Buffer
	a := NewWithOptions(Options{Out: &buf})
	if a.Out() != &buf {
		t.Errorf("Out not honoured: got %p, want %p", a.Out(), &buf)
	}
}

func TestNewWithOptionsZeroValueFallsBackToStdout(t *testing.T) {
	a := NewWithOptions(Options{})
	if a.Out() != os.Stdout {
		t.Errorf("zero-valued Out should fall back to stdout, got %v", a.Out())
	}
}
