package cmd

import "testing"

func TestDeregisterCmdRequiresName(t *testing.T) {
	c := newDeregisterCmd()
	if err := c.Args(c, []string{}); err == nil {
		t.Errorf("zero args should fail validation")
	}
}
