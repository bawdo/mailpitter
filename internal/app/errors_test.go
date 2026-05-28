package app

import (
	"errors"
	"testing"

	"github.com/bawdo/mailpitter/internal/exitcode"
)

func TestInvalidArgsfWrapsExitCode(t *testing.T) {
	err := InvalidArgsf("bad port %d", 70000)
	if !errors.Is(err, exitcode.ErrInvalidArgs) {
		t.Errorf("not wrapped: %v", err)
	}
	if err.Error() != "invalid arguments: bad port 70000" {
		t.Errorf("error text = %q", err.Error())
	}
}

func TestPrerequisitefWrapsExitCode(t *testing.T) {
	err := Prerequisitef("caddy_shack not on PATH")
	if !errors.Is(err, exitcode.ErrPrerequisite) {
		t.Errorf("not wrapped: %v", err)
	}
}

func TestMailpitNotInstalledfWrapsExitCode(t *testing.T) {
	err := MailpitNotInstalledf("no container %s", "mailpitter")
	if !errors.Is(err, exitcode.ErrMailpitNotInstalled) {
		t.Errorf("not wrapped: %v", err)
	}
}
