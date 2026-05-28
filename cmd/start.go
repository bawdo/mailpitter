package cmd

import (
	"github.com/spf13/cobra"

	"github.com/bawdo/mailpitter/internal/app"
)

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the mailpit container (idempotent)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return app.New().Start(cmd.Context())
		},
	}
}
