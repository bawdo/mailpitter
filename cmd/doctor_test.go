package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/bawdo/mailpitter/internal/app"
)

func TestRenderDoctorHumanShowsTags(t *testing.T) {
	rep := app.DoctorReport{
		Checks: []app.CheckResult{
			{Name: "docker-daemon", Status: app.SeverityPass, Summary: "reachable"},
			{Name: "mailpit-container", Status: app.SeverityFail, Summary: "not installed",
				Fix: &app.FixHint{Label: "mailpitter install", Command: "mailpitter install"}},
		},
		Summary: app.DoctorSummary{Pass: 1, Fail: 1},
	}
	var buf bytes.Buffer
	renderDoctorHuman(&buf, rep)
	for _, want := range []string{"[PASS]", "[FAIL]", "docker-daemon", "mailpitter install"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("missing %q:\n%s", want, buf.String())
		}
	}
}

func TestRenderDoctorJSON(t *testing.T) {
	rep := app.DoctorReport{Checks: []app.CheckResult{{Name: "x", Status: app.SeverityPass}}}
	var buf bytes.Buffer
	renderDoctorJSON(&buf, rep)
	if !strings.Contains(buf.String(), `"name": "x"`) {
		t.Errorf("not JSON: %s", buf.String())
	}
}
