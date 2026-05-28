package cmd

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/bawdo/mailpitter/internal/app"
)

var statusFlags struct {
	asJSON bool
}

func newStatusCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "status",
		Short: "Report container + mailpit admin-API health",
		RunE: func(cmd *cobra.Command, _ []string) error {
			a := app.New()
			rep, err := a.Status(cmd.Context())
			if err != nil {
				return err
			}
			return renderStatus(cmd.OutOrStdout(), rep, statusFlags.asJSON)
		},
	}
	c.Flags().BoolVar(&statusFlags.asJSON, "json", false, "Output as JSON")
	return c
}

func renderStatus(w io.Writer, rep app.StatusReport, asJSON bool) error {
	if asJSON {
		out, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(w, string(out))
		return nil
	}
	_, _ = fmt.Fprintf(w, "domain:    %s\n", rep.Domain)
	_, _ = fmt.Fprintf(w, "container: %s\n", rep.Container)
	api := "unreachable"
	if rep.APIReachable {
		api = "ok"
	}
	_, _ = fmt.Fprintf(w, "api:       %s\n", api)
	if rep.MailpitVersion != "" {
		_, _ = fmt.Fprintf(w, "version:   %s\n", rep.MailpitVersion)
	}
	_, _ = fmt.Fprintf(w, "messages:  %d\n", rep.Messages)
	if rep.InstallMode != "" {
		_, _ = fmt.Fprintf(w, "mode:      %s\n", rep.InstallMode)
	}
	return nil
}
