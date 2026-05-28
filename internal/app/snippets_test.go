package app

import (
	"strings"
	"testing"
)

func TestSnippetDjangoIncludesEmailHost(t *testing.T) {
	out := RenderSnippet("taiga", "taiga-back", MailVarsDjango)
	if !strings.Contains(out, "EMAIL_HOST=mailpit.test") {
		t.Errorf("missing EMAIL_HOST=mailpit.test:\n%s", out)
	}
	if !strings.Contains(out, "EMAIL_PORT=1025") {
		t.Errorf("missing EMAIL_PORT=1025:\n%s", out)
	}
	if !strings.Contains(out, `"mailpit.test:host-gateway"`) {
		t.Errorf("missing extra_hosts line:\n%s", out)
	}
	if !strings.Contains(out, "services.taiga-back") {
		t.Errorf("should reference the named service:\n%s", out)
	}
}

func TestSnippetLaravelUsesMailVars(t *testing.T) {
	out := RenderSnippet("bookstack", "bookstack", MailVarsLaravel)
	if !strings.Contains(out, "MAIL_HOST=mailpit.test") {
		t.Errorf("missing MAIL_HOST:\n%s", out)
	}
	if strings.Contains(out, "EMAIL_HOST") {
		t.Errorf("should not contain EMAIL_HOST in laravel mode:\n%s", out)
	}
}

func TestParseMailVars(t *testing.T) {
	cases := map[string]MailVars{
		"django":  MailVarsDjango,
		"laravel": MailVarsLaravel,
	}
	for in, want := range cases {
		got, err := ParseMailVars(in)
		if err != nil || got != want {
			t.Errorf("ParseMailVars(%q) = (%v, %v)", in, got, err)
		}
	}
	if _, err := ParseMailVars("oops"); err == nil {
		t.Errorf("ParseMailVars(oops) should error")
	}
}
