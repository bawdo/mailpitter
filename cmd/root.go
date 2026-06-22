package cmd

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/bawdo/mailpitter/internal/exitcode"
)

// Execute runs the CLI. The returned int is the process exit code.
func Execute() int {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	err := newRootCmd().ExecuteContext(ctx)
	return classifyError(err)
}

// classifyError maps an error to documented exit codes. Delegates to
// exitcode.From so the sentinel-to-code map has a single source of truth.
// Document new codes in README.md when you add sentinels.
func classifyError(err error) int {
	return exitcode.From(err)
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "mailpitter",
		Short: "Manage one shared local mailpit instance for development",
		Long: `mailpitter manages a single locally-running mailpit container so multiple
projects can share one inbox. The UI is registered with caddy_shack and
reachable at https://mailpit.test. Consumer containers reach SMTP at
mailpit.test:1025 via Docker's host-gateway.

Designed to be called from project startup and teardown scripts:
  mailpitter register taiga --service taiga-back
  mailpitter deregister taiga

Exit codes (stable, script-friendly):
  0  success
  1  generic command failure
  2  invalid arguments
  3  prerequisite missing (e.g. caddy_shack not installed)
  4  docker daemon unreachable
  5  mailpit container not running
  6  mailpit container not installed
  7  state file corrupt - manual remediation required
  8  state lock held by another mailpitter process
  9  caddy_shack returned non-zero

See docs/superpowers/specs/2026-05-28-mailpitter-design.md for the
full design.`,
		SilenceUsage:  true,
		SilenceErrors: false,
	}
	root.AddCommand(newVersionCmd())
	root.AddCommand(newInstallCmd())
	root.AddCommand(newStartCmd())
	root.AddCommand(newStopCmd())
	root.AddCommand(newRestartCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newRegisterCmd())
	root.AddCommand(newDeregisterCmd())
	root.AddCommand(newListCmd())
	root.AddCommand(newOpenCmd())
	root.AddCommand(newResetCmd())
	root.AddCommand(newUpgradeCmd())
	applyHelpBareword(root)
	return root
}

func applyHelpBareword(cmd *cobra.Command) {
	orig := cmd.RunE
	if orig != nil {
		cmd.RunE = func(c *cobra.Command, args []string) error {
			if len(args) > 0 && args[0] == "help" {
				return c.Help()
			}
			return orig(c, args)
		}
	}
	for _, sub := range cmd.Commands() {
		applyHelpBareword(sub)
	}
}
