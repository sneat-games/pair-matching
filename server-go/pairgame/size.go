// Package pairgame is the Pair-Matching (memory/concentration) rules engine:
// board presets, the deterministic card layout, and pure win/match mechanics.
// It has no bot, host, or persistence dependencies — see server-go/pairplay
// for the host-agnostic play layer that turns this into a Telegram
// callback-data game (github.com/sneat-games/reversi's revgame/revplay split
// is the precedent this package follows).
package pairgame

// Size is a supported board preset: a fixed Width x Height combination.
// Only a short, curated list of presets is supported (see Sizes) rather than
// arbitrary dimensions, so a board shape can be identified on the wire by a
// small index instead of two separate numbers — see server-go/pairplay for
// why that matters (REQ:state-in-callback-data caps the encoded state at
// Telegram's 64-byte callback-data limit).
//
// Width*Height must be even (every card needs a partner) and Width*Height/2
// must be <= MaxPairs, the largest pair count server-go/pairplay can address
// with its 6-bit cell index.
type Size struct {
	Width  int
	Height int
}

// Cells returns the total number of cells (cards) on the board.
func (s Size) Cells() int { return s.Width * s.Height }

// Pairs returns the number of matching pairs on the board.
func (s Size) Pairs() int { return s.Cells() / 2 }

// MaxPairs is the largest pair count the Snapshot wire encoding can address:
// cell indexes (the pending-pick cell and each robot-memory entry) are
// carried in 7 bits (0-126, 127 reserved as the "none" sentinel), so Cells()
// must stay <= 126, i.e. Pairs() <= 63. Every preset in Sizes is comfortably
// inside that ceiling — the real ceiling in practice is the 64-byte
// callback-data budget (see snapshot_test.go), which binds much earlier.
const MaxPairs = 63

// Sizes is the ordered list of supported board presets. A preset's index in
// this slice IS its wire encoding (Snapshot.SizeIndex), so this order is part
// of the wire format: append new presets, never reorder or remove existing
// ones. 8x8 is included because it turns out to fit under the seed-derived
// layout encoding (with a capped difficulty N) even though it does not fit
// under the inline encoding — see snapshot_test.go's measured table.
var Sizes = []Size{
	{Width: 2, Height: 2}, // 0:  2 pairs,  4 cells
	{Width: 2, Height: 3}, // 1:  3 pairs,  6 cells
	{Width: 3, Height: 4}, // 2:  6 pairs, 12 cells
	{Width: 4, Height: 4}, // 3:  8 pairs, 16 cells
	{Width: 4, Height: 5}, // 4: 10 pairs, 20 cells
	{Width: 4, Height: 6}, // 5: 12 pairs, 24 cells
	{Width: 5, Height: 6}, // 6: 15 pairs, 30 cells
	{Width: 6, Height: 6}, // 7: 18 pairs, 36 cells
	{Width: 8, Height: 8}, // 8: 32 pairs, 64 cells
}

func init() {
	for i, s := range Sizes {
		if s.Cells()%2 != 0 {
			panic("pairgame: Sizes[" + itoa(i) + "] has an odd cell count")
		}
		if s.Pairs() > MaxPairs {
			panic("pairgame: Sizes[" + itoa(i) + "] exceeds MaxPairs")
		}
	}
}

// itoa avoids pulling in strconv just for a panic message in init().
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
