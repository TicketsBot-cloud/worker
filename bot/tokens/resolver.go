// Package tokens resolves the Discord token for a bot id. Whitelabel tokens
// are stored encrypted, so every read goes through here and is cached in
// process for a short time rather than decrypted per event.
package tokens

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// DefaultTTL bounds how long a token survives if an invalidation message is
// missed, for example during a Redis pub/sub reconnect.
const DefaultTTL = 5 * time.Minute

var (
	ErrNotConfigured     = errors.New("tokens: resolver not configured")
	ErrMainTokenUnset    = errors.New("tokens: main bot token is not configured")
	ErrWhitelabelMissing = errors.New("tokens: whitelabel bot not found")
)

// Loader fetches the current token for a whitelabel bot. It must return an
// error rather than a fallback value when the token cannot be produced.
type Loader func(ctx context.Context, botId uint64) (string, error)

type entry struct {
	token     string
	expiresAt time.Time
}

type Resolver struct {
	mainToken string
	load      Loader
	ttl       time.Duration
	now       func() time.Time
	// sweepEvery is how often expired entries are purged. It matches the TTL,
	// so an entry outlives its expiry by at most one extra TTL.
	sweepEvery time.Duration

	mu      sync.Mutex
	entries map[uint64]entry
	// generation advances on every invalidation. A load that started before an
	// invalidation must not store its result, or it would resurrect the token
	// that was just revoked.
	generation uint64
}

func NewResolver(mainToken string, load Loader, ttl time.Duration) *Resolver {
	if ttl <= 0 {
		ttl = DefaultTTL
	}

	return &Resolver{
		mainToken:  mainToken,
		load:       load,
		ttl:        ttl,
		now:        time.Now,
		entries:    make(map[uint64]entry),
		sweepEvery: ttl,
	}
}

// Resolve returns the token for a bot. A whitelabel bot never falls back to
// the main token: a failure to resolve is returned to the caller.
func (r *Resolver) Resolve(ctx context.Context, botId uint64, isWhitelabel bool) (string, error) {
	if !isWhitelabel {
		if r.mainToken == "" {
			return "", ErrMainTokenUnset
		}

		return r.mainToken, nil
	}

	r.mu.Lock()
	if e, ok := r.entries[botId]; ok {
		if r.now().Before(e.expiresAt) {
			r.mu.Unlock()
			return e.token, nil
		}

		delete(r.entries, botId)
	}
	generation := r.generation
	r.mu.Unlock()

	token, err := r.load(ctx, botId)
	if err != nil {
		return "", fmt.Errorf("resolving token for bot %d: %w", botId, err)
	}

	r.mu.Lock()
	if r.generation == generation {
		r.entries[botId] = entry{token: token, expiresAt: r.now().Add(r.ttl)}
	}
	r.mu.Unlock()

	return token, nil
}

// Invalidate drops the cached token for a bot and discards the result of any
// load that is currently in flight.
func (r *Resolver) Invalidate(botId uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.generation++
	delete(r.entries, botId)
}

// sweep drops every expired entry so decrypted tokens do not stay in memory
// for bots that are never looked up again.
func (r *Resolver) sweep() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now()
	for id, e := range r.entries {
		if !now.Before(e.expiresAt) {
			delete(r.entries, id)
		}
	}
}

// runSweeper purges expired entries on a ticker until ctx is cancelled. The
// returned channel closes once the goroutine has exited.
func (r *Resolver) runSweeper(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	ticker := time.NewTicker(r.sweepEvery)

	go func() {
		defer close(done)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.sweep()
			}
		}
	}()

	return done
}
