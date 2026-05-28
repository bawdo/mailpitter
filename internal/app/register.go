package app

import (
	"context"
	"errors"
	"regexp"
	"time"

	"github.com/bawdo/mailpitter/internal/state"
)

// RegisterInput holds the fields the register command accepts.
type RegisterInput struct {
	Name     string
	Service  string
	MailVars MailVars
}

var consumerNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,30}$`)

// Register records a consumer and returns the copy-pasteable snippet. The
// snippet is empty when the call is a no-op (identical re-registration).
func (a *App) Register(_ context.Context, in RegisterInput) (string, error) {
	if !consumerNameRE.MatchString(in.Name) {
		return "", InvalidArgsf("name must match ^[a-z0-9][a-z0-9-]{0,30}$ (got %q)", in.Name)
	}

	lock, err := state.AcquireLock(a.paths.LockPath, 5*time.Second)
	if err != nil {
		if errors.Is(err, state.ErrLockTimeout) {
			return "", StateLockf("%v", err)
		}
		return "", err
	}
	defer func() { _ = lock.Release() }()

	s, err := state.Load(a.paths.StatePath)
	if err != nil {
		if state.IsCorrupt(err) {
			return "", StateCorruptf("%v", err)
		}
		return "", err
	}

	existing, present := s.Consumers[in.Name]
	if present && existing.Service == in.Service {
		return "", nil
	}

	s.Consumers[in.Name] = state.Consumer{
		Service:      in.Service,
		RegisteredAt: time.Now().Format(time.RFC3339),
	}
	if err := state.Save(a.paths.StatePath, s); err != nil {
		return "", err
	}
	return RenderSnippet(in.Name, in.Service, in.MailVars), nil
}
