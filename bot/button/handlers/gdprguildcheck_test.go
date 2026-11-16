package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/TicketsBot-cloud/gdl/objects/guild"
	"github.com/TicketsBot-cloud/gdl/rest"
	"github.com/TicketsBot-cloud/gdl/rest/ratelimit"
	"github.com/TicketsBot-cloud/gdl/rest/request"
	"github.com/TicketsBot-cloud/worker"
	"github.com/stretchr/testify/require"
)

func TestCheckOwnedGuilds(t *testing.T) {
	owners := map[uint64]uint64{1: 7, 2: 8, 4: 7}
	var fetches atomic.Int32

	fetchGuild = func(_ context.Context, _ string, _ *ratelimit.Ratelimiter, guildId uint64) (guild.Guild, error) {
		fetches.Add(1)
		switch guildId {
		case 3:
			return guild.Guild{}, request.RestError{StatusCode: http.StatusForbidden}
		case 5:
			return guild.Guild{}, errors.New("timeout")
		}
		return guild.Guild{Id: guildId, Name: fmt.Sprintf("g%d", guildId), OwnerId: owners[guildId]}, nil
	}
	t.Cleanup(func() { fetchGuild = rest.GetGuild })

	ctx, w, cachedOwned := context.Background(), &worker.Context{}, []uint64{1, 2, 3, 4, 5}

	names, notOwned, unreachable, err := checkOwnedGuilds(ctx, w, 7, []uint64{4, 9, 1}, cachedOwned)
	require.NoError(t, err)
	require.Nil(t, names)
	require.Equal(t, []uint64{9}, notOwned)
	require.Nil(t, unreachable)
	require.Zero(t, fetches.Load())

	names, notOwned, unreachable, err = checkOwnedGuilds(ctx, w, 7, []uint64{4, 3, 2, 1}, cachedOwned)
	require.NoError(t, err)
	require.Equal(t, map[uint64]string{4: "g4", 1: "g1"}, names)
	require.Equal(t, []uint64{2}, notOwned)
	require.Equal(t, []uint64{3}, unreachable)

	_, _, _, err = checkOwnedGuilds(ctx, w, 7, []uint64{1, 5}, cachedOwned)
	require.EqualError(t, err, "timeout")
}
