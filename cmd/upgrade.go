package cmd

import (
	"github.com/spf13/cobra"

	"github.com/bawdo/mailpitter/internal/app"
)

func newUpgradeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade the mailpit container to the latest image",
		Long: `Pulls axllent/mailpit:latest and, if it differs from the running
image, recreates the container against it. The data volume is preserved,
so stored mail survives. No-op when already up to date.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return app.New().Upgrade(cmd.Context())
		},
	}
}
