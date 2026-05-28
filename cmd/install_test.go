package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestInstallCmdParsesEphemeralFlag(t *testing.T) {
	c := newInstallCmd()
	var buf bytes.Buffer
	c.SetOut(&buf)
	c.SetErr(&buf)
	if err := c.ParseFlags([]string{"--ephemeral"}); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	if !installFlags.ephemeral {
		t.Errorf("--ephemeral not parsed")
	}
	if !strings.Contains(c.Short, "Install") {
		t.Errorf("Short missing 'Install': %q", c.Short)
	}
}
