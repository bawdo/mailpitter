package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/bawdo/mailpitter/internal/exitcode"
)

func TestResetCmdRefusesWithoutConfirm(t *testing.T) {
	c := newResetCmd()
	var buf bytes.Buffer
	c.SetOut(&buf)
	c.SetErr(&buf)
	c.SetArgs([]string{})
	// Ensure no stale flag value from previous tests.
	resetFlags.confirm = ""
	// Ensure no stale env var.
	t.Setenv("CONFIRM", "")
	err := c.Execute()
	if !errors.Is(err, exitcode.ErrInvalidArgs) {
		t.Errorf("want ErrInvalidArgs, got %v", err)
	}
	if !strings.Contains(buf.String(), "CONFIRM") {
		t.Errorf("error message should mention CONFIRM:\n%s", buf.String())
	}
}

func TestResetCmdAcceptsConfirmYes(t *testing.T) {
	resetFlags.confirm = "yes"
	if err := validateResetArgs(); err != nil {
		t.Errorf("validateResetArgs(yes): %v", err)
	}
}
