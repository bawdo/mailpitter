package app

import (
	"bytes"
	"context"
	"runtime"
	"strings"
	"testing"
)

func TestOpenOnNonDarwinPrintsURLReturnsNil(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("non-darwin behaviour test")
	}
	var buf bytes.Buffer
	a := NewWithOptions(Options{Out: &buf})
	if err := a.Open(context.Background()); err != nil {
		t.Fatalf("Open: got error %v, want nil", err)
	}
	if !strings.Contains(buf.String(), "mailpit.test") {
		t.Errorf("output should mention mailpit.test, got %q", buf.String())
	}
}
