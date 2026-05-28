package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bawdo/mailpitter/internal/app"
)

func newDeregisterCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "deregister <name>",
		Short: "Deregister a consumer (idempotent; unknown names exit 0)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			notice, err := app.New().Deregister(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if notice != "" {
				_, _ = fmt.Fprint(cmd.ErrOrStderr(), notice)
			}
			return nil
		},
	}
}
