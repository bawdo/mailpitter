package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bawdo/mailpitter/internal/app"
)

var registerFlags struct {
	service  string
	mailVars string
}

func newRegisterCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "register <name>",
		Short: "Register a consumer and print the .env + docker-compose snippet",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mv, err := app.ParseMailVars(registerFlags.mailVars)
			if err != nil {
				return app.InvalidArgsf("%v", err)
			}
			a := app.New()
			snippet, err := a.Register(cmd.Context(), app.RegisterInput{
				Name:     args[0],
				Service:  registerFlags.service,
				MailVars: mv,
			})
			if err != nil {
				return err
			}
			// Notice to stderr so `mailpitter register > .env-snippet` captures only the snippet.
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Consumer %q registered.\n", args[0])
			if snippet != "" {
				_, _ = fmt.Fprint(cmd.OutOrStdout(), snippet)
			}
			return nil
		},
	}
	c.Flags().StringVar(&registerFlags.service, "service", "", "Compose service that sends mail (optional, used in snippet)")
	c.Flags().StringVar(&registerFlags.mailVars, "mail-vars", "django", "Variable flavour: django or laravel")
	return c
}
