package cmd

import "testing"

func TestStartCmdShort(t *testing.T) {
	c := newStartCmd()
	if c.Use != "start" {
		t.Errorf("Use = %q", c.Use)
	}
}
