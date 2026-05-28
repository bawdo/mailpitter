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
	ErrInvalidArgs  = errors.New("invalid arguments")
	ErrPrerequisite = errors.New("prerequisite missing")
)

// From maps an error to its documented exit code:
//
//	0 - nil
//	1 - generic failure
//	2 - ErrInvalidArgs (or any error wrapping it)
//	3 - ErrPrerequisite (or any error wrapping it)
func From(err error) int {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, ErrInvalidArgs):
		return 2
	case errors.Is(err, ErrPrerequisite):
		return 3
	default:
		return 1
	}
}
