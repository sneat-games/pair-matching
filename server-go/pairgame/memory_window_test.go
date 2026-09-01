package pairgame

import (
	"math/rand"
	"testing"
)

// TestMemoryWindowNeverTargetsAMatchedPair is the N-player rewrite's
// successor to the old engine's "memory FIFO never holds a matched cell"
// evidence. That engine maintained a per-robot FIFO that pruned matched
// cells on every match; this one instead derives a bot's memory fresh from
// the shared public Log on every Choose call, filtering out anything whose
// pair GameState.PairOwner already shows matched — see robot.go's
// MemoryStrategy and TestMemoryStrategy_IgnoresAMatchedPairInTheWindow for
// the direct unit-level proof of that filter. This test broadens that
// single-call evidence to full games: it plays many complete games — two
// bots, each acting in strict alternation purely to keep the scenario easy
// to reason about (the engine itself imposes no such order — see Flip's
// doc comment) — across every board preset and a range of Memory depths,
// and asserts after every single Flip that no cell belonging to an
// already-matched pair remains reachable through MemoryStrategy: choosing
// again immediately never panics and never targets a stale, already-owned
// pair.
func TestMemoryWindowNeverTargetsAMatchedPair(t *testing.T) {
	for sizeIdx, size := range Sizes {
		for _, memory := range dedupInts(0, 3, size.Cells()) {
			setup := []PlayerSetup{
				{IsBot: true, Memory: memory},
				{IsBot: true, Memory: memory},
			}
			g, err := NewGame(LayoutInline, sizeIdx, uint32(sizeIdx*1009+memory), nil, setup)
			if err != nil {
				t.Fatalf("NewGame(size=%dx%d, memory=%d): %v", size.Width, size.Height, memory, err)
			}
			rnd := rand.New(rand.NewSource(int64(sizeIdx*31 + memory)))
			mover := MemoryStrategy{Fallback: RandomMover{Rand: rnd}}
			budget := 80 * size.Cells()
			if budget < 800 {
				budget = 800
			}
			by := PlayerID(1)
			for i := 0; !g.IsComplete(); i++ {
				if i > budget {
					t.Fatalf("size=%dx%d memory=%d: game did not complete within a generous move budget", size.Width, size.Height, memory)
				}
				cell := mover.Choose(g, g.Faces, by)
				if g.PairOwner[g.Faces[cell]] != NoPlayer {
					t.Fatalf("size=%dx%d memory=%d move#%d: Choose(by=%d) returned cell %d, whose pair %d is already matched",
						size.Width, size.Height, memory, i, by, cell, g.Faces[cell])
				}
				if _, err := Flip(&g, nil, by, cell); err != nil {
					t.Fatalf("Flip(%d, by=%d): %v", cell, by, err)
				}
				by = otherOf(by)
			}
		}
	}
}

// otherOf alternates between the two test bots' PlayerIDs (1 and 2) — a
// test-only convenience, not an engine concept; see Flip's doc comment on
// there being no turn order.
func otherOf(by PlayerID) PlayerID {
	if by == 1 {
		return 2
	}
	return 1
}

func dedupInts(vs ...int) []int {
	seen := make(map[int]bool, len(vs))
	out := make([]int, 0, len(vs))
	for _, v := range vs {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}
