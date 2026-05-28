package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/bawdo/mailpitter/internal/app"
)

var resetFlags struct {
	confirm string
}

func newResetCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "reset",
		Short: "Tear down container + data volume (requires CONFIRM=yes)",
		Long: `Stops mailpit, removes the container, and removes the data volume.
Refuses without CONFIRM=yes. Registered consumers in state are preserved.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if v := os.Getenv("CONFIRM"); v != "" {
				resetFlags.confirm = v
			}
			if err := validateResetArgs(); err != nil {
				return err
			}
			return app.New().Reset(cmd.Context())
		},
	}
	c.Flags().StringVar(&resetFlags.confirm, "confirm", "", `Set to "yes" to confirm`)
	return c
}

func validateResetArgs() error {
	if resetFlags.confirm != "yes" {
		return app.InvalidArgsf("refusing to reset without --confirm=yes (or CONFIRM=yes via make)")
	}
	return nil
}
