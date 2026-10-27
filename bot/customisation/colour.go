package customisation

import (
	"context"

	"github.com/TicketsBot-cloud/common/sentry"
	"github.com/TicketsBot-cloud/worker/bot/dbclient"
)

type Colour int16

func (c Colour) Int16() int16 {
	return int16(c)
}

func (c Colour) Default() int {
	return DefaultColours[c]
}

const (
	Green Colour = iota
	Red
	Orange
	Lime
	Blue
)

var DefaultColours = map[Colour]int{
	Green:  0x2ECC71,
	Red:    0xFC3F35,
	Orange: 16740864,
	Lime:   7658240,
	Blue:   472219,
}

func GetDefaultColour(colour Colour) int {
	return DefaultColours[colour]
}

func GetDefaultColours() map[Colour]*int {
	colours := make(map[Colour]*int, len(DefaultColours))
	for id, hex := range DefaultColours {
		colours[id] = &hex
	}

	return colours
}

func defaultColour(colour Colour) *int {
	hex := GetDefaultColour(colour)
	return &hex
}

func IsValidColour(colour Colour) bool {
	_, valid := DefaultColours[colour]
	return valid
}

func GetColours(ctx context.Context, guildId uint64) (map[Colour]*int, error) {
	raw, err := dbclient.Client.CustomColours.GetAll(ctx, guildId)
	if err != nil {
		return GetDefaultColours(), err
	}

	colours := GetDefaultColours()
	for id, hex := range raw {
		colours[Colour(id)] = hex
	}

	return colours, nil
}

// TODO: Premium check
func GetColourOrDefault(ctx context.Context, guildId uint64, colourCode Colour) *int {
	colour, ok, err := dbclient.Client.CustomColours.Get(ctx, guildId, colourCode.Int16())
	if err != nil {
		sentry.Error(err)
		return defaultColour(colourCode)
	}

	if !ok {
		return defaultColour(colourCode)
	}

	return colour
}
