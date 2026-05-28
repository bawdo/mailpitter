// Package exitcode owns the sentinel errors that map to documented CLI
// exit codes. Wrap a sentinel with fmt.Errorf("%w: %s", ErrXxx, detail)
// and cmd/root.go's classifyError will translate it to the right exit
// code on the way out.
//
// The set of codes is part of the CLI's public contract - keep it stable
// and document additions in README.md.
package exitcode

import "errors"

// Sentinel errors. Add new ones here AND extend the switch in From below
// AND document the new code in README.md.
var (
	ErrInvalidArgs         = errors.New("invalid arguments")
	ErrPrerequisite        = errors.New("prerequisite missing")
	ErrDockerUnreachable   = errors.New("docker daemon unreachable")
	ErrMailpitNotRunning   = errors.New("mailpit container not running")
	ErrMailpitNotInstalled = errors.New("mailpit container not installed")
	ErrStateCorrupt        = errors.New("state file corrupt")
	ErrStateLockContention = errors.New("state lock held by another mailpitter process")
	ErrCaddyShackFailure   = errors.New("caddy_shack returned non-zero")
)

// From maps an error to its documented exit code. See README.md for the
// full table.
func From(err error) int {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, ErrInvalidArgs):
		return 2
	case errors.Is(err, ErrPrerequisite):
		return 3
	case errors.Is(err, ErrDockerUnreachable):
		return 4
	case errors.Is(err, ErrMailpitNotRunning):
		return 5
	case errors.Is(err, ErrMailpitNotInstalled):
		return 6
	case errors.Is(err, ErrStateCorrupt):
		return 7
	case errors.Is(err, ErrStateLockContention):
		return 8
	case errors.Is(err, ErrCaddyShackFailure):
		return 9
	default:
		return 1
	}
}
