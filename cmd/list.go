package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/bawdo/mailpitter/internal/app"
)

var listFlags struct {
	asJSON bool
}

func newListCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "list",
		Short: "List registered consumers",
		RunE: func(cmd *cobra.Command, _ []string) error {
			items, err := app.New().List(cmd.Context())
			if err != nil {
				return err
			}
			return renderList(cmd.OutOrStdout(), items, listFlags.asJSON)
		},
	}
	c.Flags().BoolVar(&listFlags.asJSON, "json", false, "Output as JSON")
	return c
}

func renderList(w io.Writer, items []app.Listing, asJSON bool) error {
	if asJSON {
		out, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(w, string(out))
		return nil
	}
	if len(items) == 0 {
		_, _ = fmt.Fprintln(w, "no consumers registered")
		return nil
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "NAME\tSERVICE\tREGISTERED")
	for _, it := range items {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", it.Name, it.Service, it.RegisteredAt)
	}
	return tw.Flush()
}
