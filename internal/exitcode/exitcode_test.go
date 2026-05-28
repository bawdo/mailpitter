package exitcode

import (
	"errors"
	"fmt"
	"testing"
)

func TestFromMapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, 0},
		{"generic", errors.New("boom"), 1},
		{"invalid args", fmt.Errorf("wrap: %w", ErrInvalidArgs), 2},
		{"prerequisite", fmt.Errorf("wrap: %w", ErrPrerequisite), 3},
		{"docker unreachable", fmt.Errorf("wrap: %w", ErrDockerUnreachable), 4},
		{"mailpit not running", fmt.Errorf("wrap: %w", ErrMailpitNotRunning), 5},
		{"mailpit not installed", fmt.Errorf("wrap: %w", ErrMailpitNotInstalled), 6},
		{"state corrupt", fmt.Errorf("wrap: %w", ErrStateCorrupt), 7},
		{"state lock contention", fmt.Errorf("wrap: %w", ErrStateLockContention), 8},
		{"caddy_shack failure", fmt.Errorf("wrap: %w", ErrCaddyShackFailure), 9},
		{"double-wrapped invalid args", fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", ErrInvalidArgs)), 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := From(tc.err)
			if got != tc.want {
				t.Errorf("From(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}
