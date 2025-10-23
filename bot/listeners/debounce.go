package listeners

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type ThreadMemberUpdateDebounce struct {
	mu     sync.Mutex
	timers map[string]*time.Timer
}

var threadUpdateDebounce = &ThreadMemberUpdateDebounce{
	timers: make(map[string]*time.Timer),
}

func (d *ThreadMemberUpdateDebounce) getKey(guildId, threadId uint64, messageId uint64) string {
	return fmt.Sprintf("%d:%d:%d", guildId, threadId, messageId)
}

func (d *ThreadMemberUpdateDebounce) Schedule(guildId, threadId, messageId uint64, fn func(context.Context) error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := d.getKey(guildId, threadId, messageId)

	if existingTimer, exists := d.timers[key]; exists {
		existingTimer.Stop()
	}

	timer := time.AfterFunc(500*time.Millisecond, func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
		defer cancel()

		if err := fn(ctx); err != nil {
			return
		}

		d.mu.Lock()
		delete(d.timers, key)
		d.mu.Unlock()
	})

	d.timers[key] = timer
}
