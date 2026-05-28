package cmd

import (
	"github.com/spf13/cobra"

	"github.com/bawdo/mailpitter/internal/app"
)

var installFlags struct {
	ephemeral bool
}

func newInstallCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "install",
		Short: "Install and start the local mailpit container",
		RunE: func(cmd *cobra.Command, _ []string) error {
			a := app.New()
			return a.Install(cmd.Context(), app.InstallInput{Ephemeral: installFlags.ephemeral})
		},
	}
	c.Flags().BoolVar(&installFlags.ephemeral, "ephemeral", false, "Skip data volume (inbox wiped on container recreate)")
	return c
}
