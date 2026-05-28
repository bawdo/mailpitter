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
		Use:           "mailpitter",
		Short:         "(add a short description)",
		SilenceUsage:  true,
		SilenceErrors: false,
	}
	root.AddCommand(newVersionCmd())
	root.AddCommand(newInstallCmd())
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
