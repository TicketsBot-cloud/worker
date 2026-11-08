package utils

import (
	"context"
	"errors"
	"math/rand"
	"net/http"
	"time"

	"github.com/TicketsBot-cloud/gdl/objects/channel"
	"github.com/TicketsBot-cloud/gdl/rest"
	"github.com/TicketsBot-cloud/gdl/rest/request"
	w "github.com/TicketsBot-cloud/worker"
	"github.com/TicketsBot-cloud/worker/bot/redis"
)

const maxUnlistedChannelProbes = 100

func IsUnknownChannel(err error) bool {
	var restError request.RestError
	return errors.As(err, &restError) && (restError.StatusCode == http.StatusNotFound || restError.ApiError.Code == 10003)
}

func IsMissingAccess(err error) bool {
	var restError request.RestError
	return errors.As(err, &restError) && (restError.StatusCode == http.StatusForbidden || restError.ApiError.Code == 50001)
}

func RecacheGuildChannels(ctx context.Context, worker *w.Context, guildId uint64, probeBudget time.Duration, reprobeHidden bool) ([]channel.Channel, error) {
	channels, err := rest.GetGuildChannels(ctx, worker.Token, worker.RateLimiter, guildId)
	if err != nil {
		return nil, err
	}

	if len(channels) > 0 {
		if err := worker.Cache.StoreChannels(ctx, channels); err != nil {
			return nil, err
		}
	}

	if err := PruneUnlistedChannels(ctx, worker, guildId, channels, probeBudget, reprobeHidden); err != nil {
		return nil, err
	}

	return channels, nil
}

// Unlisted rows may just be hidden, so only ones confirmed gone are deleted
func PruneUnlistedChannels(ctx context.Context, worker *w.Context, guildId uint64, listed []channel.Channel, probeBudget time.Duration, reprobeHidden bool) error {
	cached, err := worker.Cache.GetGuildChannels(ctx, guildId)
	if err != nil {
		return err
	}

	listedIds := make(map[uint64]struct{}, len(listed))
	for _, ch := range listed {
		listedIds[ch.Id] = struct{}{}
	}

	var unlisted []channel.Channel
	for _, ch := range cached {
		if _, ok := listedIds[ch.Id]; ok {
			continue
		}

		if ch.Type == channel.ChannelTypeGuildPublicThread || ch.Type == channel.ChannelTypeGuildPrivateThread || ch.Type == channel.ChannelTypeGuildNewsThread {
			if err := worker.Cache.DeleteChannel(ctx, ch.Id); err != nil {
				return err
			}

			continue
		}

		unlisted = append(unlisted, ch)
	}

	if !reprobeHidden {
		ids := make([]uint64, len(unlisted))
		for i, ch := range unlisted {
			ids[i] = ch.Id
		}

		hidden, err := redis.GetHiddenChannels(ctx, ids)
		if err != nil {
			return err
		}

		unprobed := unlisted[:0]
		for _, ch := range unlisted {
			if !hidden[ch.Id] {
				unprobed = append(unprobed, ch)
			}
		}

		unlisted = unprobed
	}

	// Shuffled so repeated runs reach rows past the probe limit
	rand.Shuffle(len(unlisted), func(i, j int) {
		unlisted[i], unlisted[j] = unlisted[j], unlisted[i]
	})

	probeCtx, cancel := context.WithTimeout(ctx, probeBudget)
	defer cancel()

	for i, ch := range unlisted {
		if i >= maxUnlistedChannelProbes || probeCtx.Err() != nil {
			break
		}

		_, err := rest.GetChannel(probeCtx, worker.Token, worker.RateLimiter, ch.Id)
		switch {
		case IsUnknownChannel(err):
			if err := worker.Cache.DeleteChannel(ctx, ch.Id); err != nil {
				return err
			}
		case IsMissingAccess(err):
			if !ch.IsObfuscated() {
				ch.Flags |= channel.ChannelFlagObfuscated
				if err := worker.Cache.StoreChannel(ctx, ch); err != nil {
					return err
				}
			}

			if err := redis.SetChannelHidden(ctx, ch.Id); err != nil {
				return err
			}
		}
	}

	return nil
}
