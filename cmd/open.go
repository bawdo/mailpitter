package cmd

import (
	"github.com/spf13/cobra"

	"github.com/bawdo/mailpitter/internal/app"
)

func newOpenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open",
		Short: "Open https://mailpit.test in the system browser",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return app.New().Open(cmd.Context())
		},
	}
}
