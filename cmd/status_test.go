package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/bawdo/mailpitter/internal/app"
)

func TestRenderStatusHumanIncludesDomain(t *testing.T) {
	var buf bytes.Buffer
	rep := app.StatusReport{Container: "running", APIReachable: true, MailpitVersion: "1.21.0", Messages: 3, InstallMode: "persistent", Domain: "mailpit.test"}
	if err := renderStatus(&buf, rep, false); err != nil {
		t.Fatalf("renderStatus: %v", err)
	}
	if !strings.Contains(buf.String(), "mailpit.test") {
		t.Errorf("missing domain in human output:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "running") {
		t.Errorf("missing container state:\n%s", buf.String())
	}
}

func TestRenderStatusJSON(t *testing.T) {
	var buf bytes.Buffer
	rep := app.StatusReport{Container: "running"}
	if err := renderStatus(&buf, rep, true); err != nil {
		t.Fatalf("renderStatus: %v", err)
	}
	if !strings.Contains(buf.String(), `"container": "running"`) {
		t.Errorf("not JSON:\n%s", buf.String())
	}
}
