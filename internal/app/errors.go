package app

import (
	"fmt"

	"github.com/bawdo/mailpitter/internal/exitcode"
)

// InvalidArgsf returns an error wrapping exitcode.ErrInvalidArgs.
func InvalidArgsf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", exitcode.ErrInvalidArgs, fmt.Sprintf(format, args...))
}

// Prerequisitef wraps exitcode.ErrPrerequisite with detail.
func Prerequisitef(format string, args ...any) error {
	return fmt.Errorf("%w: %s", exitcode.ErrPrerequisite, fmt.Sprintf(format, args...))
}

// DockerUnreachablef wraps exitcode.ErrDockerUnreachable.
func DockerUnreachablef(format string, args ...any) error {
	return fmt.Errorf("%w: %s", exitcode.ErrDockerUnreachable, fmt.Sprintf(format, args...))
}

// MailpitNotRunningf wraps exitcode.ErrMailpitNotRunning.
func MailpitNotRunningf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", exitcode.ErrMailpitNotRunning, fmt.Sprintf(format, args...))
}

// MailpitNotInstalledf wraps exitcode.ErrMailpitNotInstalled.
func MailpitNotInstalledf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", exitcode.ErrMailpitNotInstalled, fmt.Sprintf(format, args...))
}

// StateCorruptf wraps exitcode.ErrStateCorrupt.
func StateCorruptf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", exitcode.ErrStateCorrupt, fmt.Sprintf(format, args...))
}

// StateLockf wraps exitcode.ErrStateLockContention.
func StateLockf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", exitcode.ErrStateLockContention, fmt.Sprintf(format, args...))
}

// CaddyShackf wraps exitcode.ErrCaddyShackFailure with the underlying
// error chain preserved.
func CaddyShackf(err error, format string, args ...any) error {
	return fmt.Errorf("%w: %s: %v", exitcode.ErrCaddyShackFailure, fmt.Sprintf(format, args...), err)
}
