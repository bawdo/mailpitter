package app

import (
	"context"
	"sort"

	"github.com/bawdo/mailpitter/internal/state"
)

// Listing is one row of `mailpitter list`.
type Listing struct {
	Name         string `json:"name"`
	Service      string `json:"service"`
	RegisteredAt string `json:"registered_at"`
}

// List returns the registered consumers sorted alphabetically by name.
func (a *App) List(_ context.Context) ([]Listing, error) {
	s, err := state.Load(a.paths.StatePath)
	if err != nil {
		if state.IsCorrupt(err) {
			return nil, StateCorruptf("%v", err)
		}
		return nil, err
	}
	names := make([]string, 0, len(s.Consumers))
	for n := range s.Consumers {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]Listing, 0, len(names))
	for _, n := range names {
		c := s.Consumers[n]
		out = append(out, Listing{Name: n, Service: c.Service, RegisteredAt: c.RegisteredAt})
	}
	return out, nil
}
