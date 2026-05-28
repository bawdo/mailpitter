//go:build integration

// Package integration is the home for slow/external tests that should
// not run on every `go test ./...`. They live behind the `integration`
// build tag and run via `make integration` (or the CI job of the same
// name).
//
// Conventions:
//   - Every file in this package starts with `//go:build integration`.
//   - Tests that need a missing dependency (docker, a local service,
//     network) should t.Skip with a helpful message rather than fail.
//   - Use t.TempDir, t.Cleanup, and t.Helper as you would in unit tests.
package integration

import "testing"

// TestIntegrationScaffoldRuns is the smoke test that proves the build
// tag, package layout, and CI wiring all work. Replace it with real
// integration tests as the project grows; keep at least one passing
// test so the CI job has something to run.
func TestIntegrationScaffoldRuns(t *testing.T) {
	t.Log("integration test scaffold OK")
}
