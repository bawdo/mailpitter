package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/bawdo/mailpitter/internal/app"
)

func TestRenderListHumanEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := renderList(&buf, nil, false); err != nil {
		t.Fatalf("renderList: %v", err)
	}
	if !strings.Contains(buf.String(), "no consumers registered") {
		t.Errorf("got %q", buf.String())
	}
}

func TestRenderListHumanRows(t *testing.T) {
	var buf bytes.Buffer
	items := []app.Listing{{Name: "taiga", Service: "taiga-back", RegisteredAt: "2026-05-28T12:00:00Z"}}
	if err := renderList(&buf, items, false); err != nil {
		t.Fatalf("renderList: %v", err)
	}
	if !strings.Contains(buf.String(), "taiga") || !strings.Contains(buf.String(), "NAME") {
		t.Errorf("got %q", buf.String())
	}
}

func TestRenderListJSON(t *testing.T) {
	var buf bytes.Buffer
	items := []app.Listing{{Name: "taiga"}}
	if err := renderList(&buf, items, true); err != nil {
		t.Fatalf("renderList: %v", err)
	}
	if !strings.Contains(buf.String(), `"name": "taiga"`) {
		t.Errorf("not JSON: %s", buf.String())
	}
}
