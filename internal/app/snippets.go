package app

import (
	"fmt"
	"strings"
)

// MailVars selects the variable naming convention emitted in snippets.
type MailVars int

const (
	MailVarsDjango MailVars = iota
	MailVarsLaravel
)

// ParseMailVars accepts "django" or "laravel".
func ParseMailVars(s string) (MailVars, error) {
	switch strings.ToLower(s) {
	case "django", "":
		return MailVarsDjango, nil
	case "laravel":
		return MailVarsLaravel, nil
	default:
		return 0, fmt.Errorf("unknown --mail-vars %q (want django or laravel)", s)
	}
}

// RenderSnippet returns the multi-line copy-pasteable snippet shown after
// a successful register.
func RenderSnippet(name, service string, mv MailVars) string {
	var env string
	switch mv {
	case MailVarsLaravel:
		env = "MAIL_HOST=mailpit.test\n" +
			"MAIL_PORT=1025\n" +
			"MAIL_ENCRYPTION=\n" +
			"MAIL_USERNAME=\n" +
			"MAIL_PASSWORD=\n"
	default:
		env = "EMAIL_HOST=mailpit.test\n" +
			"EMAIL_PORT=1025\n" +
			"EMAIL_USE_TLS=False\n" +
			"EMAIL_USE_SSL=False\n"
	}

	svc := service
	if svc == "" {
		svc = "<your-service>"
	}

	return fmt.Sprintf(`# Add to .env:
%s
# Add to docker-compose.yml under services.%s:
extra_hosts:
  - "mailpit.test:host-gateway"

# To stop using the shared mailpit:
mailpitter deregister %s
`, env, svc, name)
}
