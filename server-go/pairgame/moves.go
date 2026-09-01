package pairgame

// RevealOutcome describes what happened as a result of one Reveal call, for
// a caller (the render/robot layer) to react to.
type RevealOutcome struct {
	// Matched is true if this reveal completed a pair (it was a second pick
	// and it matched the pending first pick).
	Matched bool
	// MatchedBy is who was credited with the match; only meaningful when
	// Matched is true.
	MatchedBy Owner
	// TurnEnded is true if this reveal was a second pick that did NOT
	// match, so the turn passed to the other side.
	TurnEnded bool
	// GameOver is true if this reveal completed the last remaining pair.
	GameOver bool
}

// Reveal applies one cell reveal to the state, following classic
// concentration/memory-match rules adapted to a click-through (no hidden
// "flip back after a delay") flow:
//
//   - The first pick of a turn is recorded as Pending; nothing else changes.
//   - The second pick is compared against Pending's face. A match credits
//     the pair to the current Turn, clears Pending, and leaves Turn
//     unchanged (the player who matched goes again) — the classic
//     concentration rule that rewards a good memory. A mismatch clears
//     Pending and flips Turn to the other side.
//
// Every successfully revealed cell (first or second pick, matched or not)
// is pushed into Memory — see the FIFO maintenance below — which is what
// robot.go's memory-bounded Strategy reads.
//
// secret is forwarded to FacesWith to resolve the board layout; it is
// ignored when g.Mode == LayoutInline.
func (g GameState) Reveal(cell int, secret []byte) (GameState, RevealOutcome, error) {
	if g.IsComplete() {
		return g, RevealOutcome{}, ErrGameOver
	}
	faces := g.FacesWith(secret)
	if cell < 0 || cell >= len(faces) {
		return g, RevealOutcome{}, ErrInvalidCell
	}
	pairID := int(faces[cell])
	if g.PairOwner[pairID] != Unmatched {
		return g, RevealOutcome{}, ErrCellAlreadyMatched
	}
	if cell == g.Pending {
		return g, RevealOutcome{}, ErrCellIsPending
	}

	next := g
	next.PairOwner = append([]Owner(nil), g.PairOwner...)
	next.Memory = rememberCell(g.Memory, g.N, cell)

	if g.Pending < 0 {
		// First pick this turn: nothing resolves yet.
		next.Pending = cell
		return next, RevealOutcome{}, nil
	}

	// Second pick: resolve against the pending first pick.
	pendingFace := int(faces[g.Pending])
	next.Pending = -1

	if pendingFace == pairID {
		next.PairOwner[pairID] = g.Turn
		next.Memory = forgetCell(next.Memory, cell)
		next.Memory = forgetCell(next.Memory, g.Pending)
		outcome := RevealOutcome{Matched: true, MatchedBy: g.Turn}
		if next.IsComplete() {
			outcome.GameOver = true
		}
		return next, outcome, nil
	}

	next.Turn = g.Turn.Other()
	return next, RevealOutcome{TurnEnded: true}, nil
}

// rememberCell pushes cell onto the memory FIFO (most-recent last),
// de-duplicating an already-remembered cell by moving it to the back
// instead of storing it twice, then trims from the front until the FIFO is
// back within capacity n. n == 0 means "remembers nothing" — the FIFO stays
// empty, which is what makes RandomMover's uniform-random placeholder
// correct for that difficulty level.
func rememberCell(memory []int, n, cell int) []int {
	if n <= 0 {
		return nil
	}
	next := make([]int, 0, len(memory)+1)
	for _, m := range memory {
		if m != cell {
			next = append(next, m)
		}
	}
	next = append(next, cell)
	if len(next) > n {
		next = next[len(next)-n:]
	}
	return next
}

// forgetCell removes cell from the memory FIFO, if present — used once a
// pair resolves, since a matched cell's face is already public via
// PairOwner and is no longer useful to remember.
func forgetCell(memory []int, cell int) []int {
	for i, m := range memory {
		if m == cell {
			return append(append([]int(nil), memory[:i]...), memory[i+1:]...)
		}
	}
	return memory
}
