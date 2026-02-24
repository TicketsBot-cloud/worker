package utils

import "math"

type colourEmoji struct {
	Emoji   string
	R, G, B float64
}

var colourEmojis = []colourEmoji{
	{"🔴", 221, 46, 68},
	{"🟠", 227, 137, 52},
	{"🟡", 253, 203, 88},
	{"🟢", 120, 177, 89},
	{"🔵", 85, 172, 238},
	{"🟣", 170, 142, 214},
	{"🟤", 166, 123, 91},
	{"⚫", 49, 55, 61},
	{"⚪", 230, 231, 232},
}

// ClosestColourEmoji returns the circle emoji closest to the given hex colour (e.g. 0xFF5733).
func ClosestColourEmoji(hex int) string {
	r := float64((hex >> 16) & 0xFF)
	g := float64((hex >> 8) & 0xFF)
	b := float64(hex & 0xFF)

	best := colourEmojis[0].Emoji
	bestDist := math.MaxFloat64

	for _, e := range colourEmojis {
		dist := (r-e.R)*(r-e.R) + (g-e.G)*(g-e.G) + (b-e.B)*(b-e.B)
		if dist < bestDist {
			bestDist = dist
			best = e.Emoji
		}
	}

	return best
}
