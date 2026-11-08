package redis

import (
	"context"
	"fmt"
	"time"
)

const hiddenChannelExpiry = time.Hour * 24

func hiddenChannelKey(channelId uint64) string {
	return fmt.Sprintf("hiddenchannel:%d", channelId)
}

func SetChannelHidden(ctx context.Context, channelId uint64) error {
	return Client.Set(ctx, hiddenChannelKey(channelId), 1, hiddenChannelExpiry).Err()
}

func GetHiddenChannels(ctx context.Context, channelIds []uint64) (map[uint64]bool, error) {
	if len(channelIds) == 0 {
		return nil, nil
	}

	keys := make([]string, len(channelIds))
	for i, id := range channelIds {
		keys[i] = hiddenChannelKey(id)
	}

	res, err := Client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	hidden := make(map[uint64]bool)
	for i, value := range res {
		if value != nil {
			hidden[channelIds[i]] = true
		}
	}

	return hidden, nil
}
