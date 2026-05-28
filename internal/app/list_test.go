package app

import (
	"context"
	"testing"
)

func TestListEmpty(t *testing.T) {
	a, _, _ := newTestApp(t)
	items, err := a.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected empty, got %v", items)
	}
}

func TestListReturnsRegisteredConsumersSorted(t *testing.T) {
	a, _, _ := newTestApp(t)
	_, _ = a.Register(context.Background(), RegisterInput{Name: "taiga", Service: "taiga-back"})
	_, _ = a.Register(context.Background(), RegisterInput{Name: "bookstack", Service: "bookstack"})
	items, err := a.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 2 || items[0].Name != "bookstack" || items[1].Name != "taiga" {
		t.Errorf("not sorted alphabetically: %+v", items)
	}
}
