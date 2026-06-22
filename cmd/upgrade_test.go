package cmd

import "testing"

func TestUpgradeCmdShort(t *testing.T) {
	c := newUpgradeCmd()
	if c.Use != "upgrade" {
		t.Errorf("Use = %q", c.Use)
	}
}

func TestUpgradeCmdRegistered(t *testing.T) {
	root := newRootCmd()
	for _, c := range root.Commands() {
		if c.Use == "upgrade" {
			return
		}
	}
	t.Errorf("upgrade command not registered on root")
}
