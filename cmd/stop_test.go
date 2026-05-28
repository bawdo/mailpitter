package cmd

import "testing"

func TestStopCmdShort(t *testing.T) {
	c := newStopCmd()
	if c.Use != "stop" {
		t.Errorf("Use = %q", c.Use)
	}
}
