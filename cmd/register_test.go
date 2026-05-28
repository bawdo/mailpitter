package cmd

import "testing"

func TestRegisterCmdRequiresName(t *testing.T) {
	c := newRegisterCmd()
	if c.Args == nil {
		t.Fatal("Args validator missing")
	}
	if err := c.Args(c, []string{}); err == nil {
		t.Errorf("zero args should fail validation")
	}
	if err := c.Args(c, []string{"taiga"}); err != nil {
		t.Errorf("one arg should pass, got %v", err)
	}
}
