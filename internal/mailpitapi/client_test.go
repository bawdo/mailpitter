package mailpitapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInfoReturnsParsedFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/info" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Version":"1.21.0","Messages":42}`))
	}))
	defer srv.Close()

	c := New(srv.URL, srv.Client())
	info, err := c.Info()
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Version != "1.21.0" {
		t.Errorf("Version = %q", info.Version)
	}
	if info.Messages != 42 {
		t.Errorf("Messages = %d", info.Messages)
	}
}

func TestInfoErrorsOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := New(srv.URL, srv.Client())
	if _, err := c.Info(); err == nil {
		t.Fatal("expected error")
	}
}
