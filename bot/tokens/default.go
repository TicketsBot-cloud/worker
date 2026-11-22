package tokens

import (
	"context"
	"sync/atomic"
)

var defaultResolver atomic.Pointer[Resolver]

// SetDefault installs the process wide resolver. It is called once at startup.
func SetDefault(r *Resolver) {
	defaultResolver.Store(r)
}

// Resolve uses the process wide resolver.
func Resolve(ctx context.Context, botId uint64, isWhitelabel bool) (string, error) {
	r := defaultResolver.Load()
	if r == nil {
		return "", ErrNotConfigured
	}

	return r.Resolve(ctx, botId, isWhitelabel)
}

// Invalidate uses the process wide resolver and is a no-op when none is set.
func Invalidate(botId uint64) {
	if r := defaultResolver.Load(); r != nil {
		r.Invalidate(botId)
	}
}
