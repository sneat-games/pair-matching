// Package pairgame is the Pair-Matching (memory/concentration) rules engine:
// board presets, the deterministic card layout, and the N-player win/match
// mechanics (Flip; see its doc comment for the full rules) shared
// identically by all three founder-specified modes — Solo (one human, no
// bot), vs-Bot (one human + one bot), and vs-Humans (2..8 humans, no bot).
// There is no turn order and no per-game persistence dependency in this
// package itself: it has no bot, host, or storage code of its own.
//
//   - Solo is the one mode small enough, and single-actor enough, to
//     round-trip through a Telegram callback_data button — see snapshot.go's
//     Encode/Decode and server-go/pairplay for the host-agnostic play layer
//     that would turn that into an actual Telegram game
//     (github.com/sneat-games/reversi's revgame/revplay split is the
//     precedent both this package and pairplay follow).
//   - vs-Bot and vs-Humans need server-side storage instead — a bot that
//     moves on its own timer, and moves arriving from more than one human
//     actor, cannot be reconstructed from a single tapped button's payload
//     the way solo's single-human-driven state can. See
//     server-go/pairgame/dal4pairgame for the persistence layer and its
//     sibling session-composing package for how the two are wired together.
package pairgame

// Size is a supported board preset: a fixed Width x Height combination.
// Only a short, curated list of presets is supported (see Sizes) rather than
// arbitrary dimensions, so a board shape can be identified on the wire by a
// small index instead of two separate numbers — see server-go/pairplay for
// why that matters (REQ:state-in-callback-data caps the encoded state at
// Telegram's 64-byte callback-data limit).
//
// Width*Height must be even (every card needs a partner) and Width*Height/2
// must be <= MaxPairs — see MaxPairs's doc comment for what actually bounds
// it.
type Size struct {
	Width  int
	Height int
}

// Cells returns the total number of cells (cards) on the board.
func (s Size) Cells() int { return s.Width * s.Height }

// Pairs returns the number of matching pairs on the board.
func (s Size) Pairs() int { return s.Cells() / 2 }

// MaxPairs is a documented ceiling on pair count, well below the two real
// constraints: a face/pair id must fit Faces' uint8 element type (Pairs() <=
// 256), and cell indexes (the pending-pick cell and each robot-memory entry)
// now cost ceil(log2(cells))-ish bits rather than a fixed width — see
// cellIndexBits and pendingFieldBits in snapshot.go — so there is no longer a
// hard bit-width wall at 126/127 cells the way there was before the
// callback-data compaction pass. The real ceiling in practice is the 64-byte
// callback-data budget itself (see snapshot_test.go's TestSnapshotBudgetMatrix
// and MaxDifficulty), which binds much earlier than either of those for
// every preset in Sizes; MaxPairs just keeps that budget-driven ceiling from
// being confused with a hard format limit.
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
