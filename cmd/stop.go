package cmd

import (
	"github.com/spf13/cobra"

	"github.com/bawdo/mailpitter/internal/app"
)

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the mailpit container and deregister mailpit.test",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return app.New().Stop(cmd.Context())
		},
	}
}
