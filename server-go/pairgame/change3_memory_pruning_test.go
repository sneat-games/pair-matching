package pairgame

import (
	"math/rand"
	"testing"
)

// TestMemoryNeverHoldsAMatchedCell is the evidence for the callback-data
// compaction pass's Change 3 ("prune memory to unmatched cells"). It plays
// many full games driven by MemoryStrategy at the board's own N (so Memory
// genuinely fills and drains the way a real robot difficulty setting would,
// unlike a pure random driver, which — having no memory to exploit — can
// take an unbounded number of reveals to finish) across every board preset
// and a range of N, and asserts after EVERY single Reveal call that
// g.Memory contains no cell whose pair is already matched.
//
// This test does not fail before or after Change 3 — see moves.go's
// rememberCell/forgetCell: forgetCell(next.Memory, cell) and
// forgetCell(next.Memory, g.Pending) already run on every successful match,
// unconditionally, and a pairID can only ever transition Unmatched->Matched
// via those exact two cells (Reveal rejects re-revealing an already-matched
// cell, so a stale matched entry can never be introduced any other way).
// That is: Change 3, as specified ("drop matched cells from the FIFO"), was
// already fully implemented and already covered by
// TestReveal_MatchForgetsBothCellsFromMemory before this compaction pass
// touched anything. This test exists to broaden that single-match unit
// test's evidence to full games across every board size/difficulty, which
// is also the evidence behind this pass's PR report: there is no measurable
// robot difficulty shift from Change 3, because there is no behavioural
// change to measure — Memory already excluded matched cells by construction.
func TestMemoryNeverHoldsAMatchedCell(t *testing.T) {
	for sizeIdx, size := range Sizes {
		for _, n := range dedupInts(0, 3, size.Cells()) {
			max := MaxDifficulty(LayoutInline, sizeIdx)
			if max < 0 || n > max {
				continue
			}
			g, err := NewGame(LayoutInline, sizeIdx, uint32(sizeIdx*1009+n), nil, n, Human)
			if err != nil {
				t.Fatalf("NewGame(size=%dx%d, n=%d): %v", size.Width, size.Height, n, err)
			}
			rnd := rand.New(rand.NewSource(int64(sizeIdx*31 + n)))
			mover := MemoryStrategy{Fallback: RandomMover{Rand: rnd}}
			budget := 50 * size.Cells()
			if budget < 500 {
				budget = 500
			}
			for i := 0; !g.IsComplete(); i++ {
				if i > budget {
					t.Fatalf("size=%dx%d n=%d: game did not complete within a generous move budget", size.Width, size.Height, n)
				}
				cell := mover.Choose(g, g.Faces)
				var err error
				g, _, err = g.Reveal(cell, nil)
				if err != nil {
					t.Fatalf("Reveal(%d): %v", cell, err)
				}
				for _, m := range g.Memory {
					pairID := g.Faces[m]
					if g.PairOwner[pairID] != Unmatched {
						t.Fatalf("size=%dx%d n=%d move#%d: Memory %v holds cell %d whose pair %d is already %v",
							size.Width, size.Height, n, i, g.Memory, m, pairID, g.PairOwner[pairID])
					}
				}
			}
		}
	}
}
