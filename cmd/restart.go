package cmd

import (
	"github.com/spf13/cobra"

	"github.com/bawdo/mailpitter/internal/app"
)

func newRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Restart the mailpit container",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return app.New().Restart(cmd.Context())
		},
	}
}
