package tokens

import (
	"context"
	"fmt"

	"github.com/TicketsBot-cloud/common/encryption"
	"github.com/TicketsBot-cloud/database"
)

type whitelabelStore interface {
	GetByBotId(ctx context.Context, botId uint64) (database.WhitelabelBot, error)
}

// NewDBLoader reads a whitelabel row and opens its token. The legacy plaintext
// column wins while it is populated, so a pod that predates encryption cannot
// leave a stale sealed copy in charge.
func NewDBLoader(store whitelabelStore, ring *encryption.Keyring) Loader {
	return func(ctx context.Context, botId uint64) (string, error) {
		bot, err := store.GetByBotId(ctx, botId)
		if err != nil {
			return "", fmt.Errorf("loading whitelabel bot: %w", err)
		}

		if bot.BotId == 0 {
			return "", ErrWhitelabelMissing
		}

		token, err := ring.ResolveStored(bot.BotId, bot.Token, bot.TokenCiphertext, bot.TokenKeyVersion)
		if err != nil {
			return "", fmt.Errorf("reading stored token: %w", err)
		}

		return token, nil
	}
}
