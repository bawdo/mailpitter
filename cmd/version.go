package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bawdo/mailpitter/internal/version"
)

// newVersionCmd wires `mailpitter version`. Prints the version line
// followed by optional commit (with " (dirty)" suffix when the build tree
// was dirty) and tag lines. Values come from internal/version.Resolve,
// which prefers ldflags-set values and falls back to runtime build info.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the mailpitter version, commit, and tag",
		RunE: func(cmd *cobra.Command, _ []string) error {
			info := version.Resolve()
			w := cmd.OutOrStdout()
			_, _ = fmt.Fprintf(w, "mailpitter %s\n", info.Version)
			if info.Commit != "" {
				suffix := ""
				if info.Dirty {
					suffix = " (dirty)"
				}
				_, _ = fmt.Fprintf(w, "  commit: %s%s\n", info.Commit, suffix)
			}
			if info.Tag != "" {
				_, _ = fmt.Fprintf(w, "  tag:    %s\n", info.Tag)
			}
			return nil
		},
	}
}
