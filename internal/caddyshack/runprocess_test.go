package caddyshack

import (
	"context"
	"testing"
)

// runProcess is the production runner (the exec seam that the fakeRunner
// stands in for elsewhere). Exercise it directly against real, portable
// commands so its exit-code and error handling are covered.
func TestRunProcessSuccess(t *testing.T) {
	stdout, _, exit, err := runProcess(context.Background(), "echo", "hello")
	if err != nil {
		t.Fatalf("runProcess echo: %v", err)
	}
	if exit != 0 {
		t.Errorf("exit = %d, want 0", exit)
	}
	if stdout != "hello\n" {
		t.Errorf("stdout = %q, want %q", stdout, "hello\n")
	}
}

func TestRunProcessNonZeroExit(t *testing.T) {
	// `sh -c 'exit 3'` runs and exits non-zero: an *exec.ExitError whose
	// code runProcess extracts.
	_, _, exit, err := runProcess(context.Background(), "sh", "-c", "exit 3")
	if err == nil {
		t.Fatal("expected an error for non-zero exit")
	}
	if exit != 3 {
		t.Errorf("exit = %d, want 3", exit)
	}
}

func TestRunProcessCannotStart(t *testing.T) {
	// A binary that does not exist fails before running: not an ExitError,
	// so runProcess reports exit -1.
	_, _, exit, err := runProcess(context.Background(), "/nonexistent/caddy_shack-stub")
	if err == nil {
		t.Fatal("expected an error when the binary cannot be run")
	}
	if exit != -1 {
		t.Errorf("exit = %d, want -1", exit)
	}
}
