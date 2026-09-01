package pairgame

import "math/rand"

// Strategy picks the robot's next cell to reveal, given the current state
// (Pending already reflects the robot's own earlier pick this turn, if any)
// and the resolved board layout. It never sees the secret directly — only
// whichever faces the caller already resolved via FacesWith — so a Strategy
// cannot see more of the board than a real player legitimately could from
// what has already been revealed plus its own memory.
type Strategy interface {
	Choose(g GameState, faces []uint8) int
}

// RandomMover picks uniformly among the currently legal cells, ignoring
// Memory entirely. It is the N==0 case of the "remembers the last N cards"
// difficulty dial (see GameState.N's doc comment): a no-memory, easy
// opponent. It also serves as MemoryStrategy's fallback whenever memory does
// not resolve a move outright.
type RandomMover struct{ Rand *rand.Rand }

// Choose implements Strategy.
func (m RandomMover) Choose(g GameState, faces []uint8) int {
	moves := g.LegalMoves(faces)
	if len(moves) == 0 {
		panic("pairgame: Choose called with no legal moves")
	}
	r := m.Rand
	if r == nil {
		r = rand.New(rand.NewSource(1)) //nolint:gosec // a casual game's move variety, not a security boundary
	}
	return moves[r.Intn(len(moves))]
}

// MemoryStrategy plays using GameState.Memory, the bounded FIFO of the last
// N revealed-but-unresolved cells (N == GameState.N, the founder-specified
// difficulty dial — see GameState.N's doc comment):
//
//   - Second pick of a turn (Pending is set): if Memory holds another cell
//     whose face matches the pending pick's face, take it — a guaranteed
//     match. Otherwise fall back to Fallback.
//   - First pick of a turn (no Pending): if Memory holds two cells that
//     share a face — a pair the robot has already seen both halves of —
//     open one of them; its second pick will then find the guaranteed
//     match above. Otherwise fall back to Fallback.
//
// A larger N keeps more of the board in memory, so this strategy finds
// guaranteed matches more often. At N == Sizes[SizeIndex].Cells() (memory
// never has to forget anything) it plays a perfect, effectively unbeatable
// game; at N == 0, Memory is always empty and it degrades to pure Fallback.
// Every N in between is a real difficulty step, not just cosmetic — which is
// the point of exposing N as the difficulty control.
type MemoryStrategy struct {
	// Fallback is used whenever memory does not resolve a move (defaults to
	// RandomMover{} when nil).
	Fallback Strategy
}

func (m MemoryStrategy) fallback() Strategy {
	if m.Fallback != nil {
		return m.Fallback
	}
	return RandomMover{}
}

// Choose implements Strategy.
func (m MemoryStrategy) Choose(g GameState, faces []uint8) int {
	if g.Pending >= 0 {
		pendingFace := faces[g.Pending]
		for _, cell := range g.Memory {
			if cell != g.Pending && faces[cell] == pendingFace && g.PairOwner[pendingFace] == Unmatched {
				return cell
			}
		}
		return m.fallback().Choose(g, faces)
	}

	seenAt := make(map[uint8]int, len(g.Memory))
	for _, cell := range g.Memory {
		face := faces[cell]
		if g.PairOwner[face] != Unmatched {
			continue
		}
		if _, already := seenAt[face]; already {
			return cell // second remembered sighting of this face: open it
		}
		seenAt[face] = cell
	}
	return m.fallback().Choose(g, faces)
}
