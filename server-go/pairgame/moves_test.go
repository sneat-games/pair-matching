package pairgame

import "testing"

// newTestGame builds a small inline-layout game so tests can read Faces
// directly instead of threading a secret through every call.
func newTestGame(t *testing.T, sizeIndex int, seed uint32, n int) GameState {
	t.Helper()
	g, err := NewGame(LayoutInline, sizeIndex, seed, nil, n, Human)
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	return g
}

// findPair returns two distinct cell indices that share a face — useful for
// deterministically driving a match in a test regardless of the shuffle.
func findPair(faces []uint8) (a, b int) {
	seen := make(map[uint8]int, len(faces))
	for i, f := range faces {
		if j, ok := seen[f]; ok {
			return j, i
		}
		seen[f] = i
	}
	panic("no pair found")
}

// findMismatch returns two distinct cell indices with different faces.
func findMismatch(faces []uint8) (a, b int) {
	for i := range faces {
		for j := range faces {
			if i != j && faces[i] != faces[j] {
				return i, j
			}
		}
	}
	panic("no mismatch found")
}

// playToCompletion drives g to completion by always revealing a genuine
// matching pair each round (never a deliberate mismatch), so it always
// makes progress and finishes in exactly Sizes[g.SizeIndex].Pairs() rounds.
// A naive "always reveal the lowest-index legal cell" driver is NOT safe
// here: after any mismatch it can get stuck re-revealing the same two
// still-unmatched cells forever (turn flips, Pending resets to -1, the
// lowest legal cell is the same one again) without ever reaching the cells
// that would actually complete the board.
func playToCompletion(t *testing.T, g GameState) GameState {
	t.Helper()
	for !g.IsComplete() {
		a, b := -1, -1
		seenAt := make(map[uint8]int, len(g.PairOwner))
		for cell, pairID := range g.Faces {
			if g.PairOwner[pairID] != Unmatched {
				continue
			}
			if first, ok := seenAt[pairID]; ok {
				a, b = first, cell
				break
			}
			seenAt[pairID] = cell
		}
		if a < 0 {
			t.Fatalf("playToCompletion: no unmatched pair found but IsComplete() is false; state: %+v", g)
		}
		var err error
		g, _, err = g.Reveal(a, nil)
		if err != nil {
			t.Fatalf("playToCompletion: Reveal(%d): %v", a, err)
		}
		g, _, err = g.Reveal(b, nil)
		if err != nil {
			t.Fatalf("playToCompletion: Reveal(%d): %v", b, err)
		}
	}
	return g
}

func TestReveal_FirstPickSetsPending(t *testing.T) {
	g := newTestGame(t, 3, 1, 4) // 4x4
	next, outcome, err := g.Reveal(0, nil)
	if err != nil {
		t.Fatalf("Reveal: %v", err)
	}
	if next.Pending != 0 {
		t.Errorf("Pending = %d, want 0", next.Pending)
	}
	if outcome.Matched || outcome.TurnEnded || outcome.GameOver {
		t.Errorf("unexpected outcome on a first pick: %+v", outcome)
	}
	if next.Turn != g.Turn {
		t.Errorf("Turn changed on a first pick: %v -> %v", g.Turn, next.Turn)
	}
}

func TestReveal_MatchCreditsCurrentTurnAndKeepsIt(t *testing.T) {
	g := newTestGame(t, 3, 1, 4)
	a, b := findPair(g.Faces)

	g, _, err := g.Reveal(a, nil)
	if err != nil {
		t.Fatalf("first Reveal: %v", err)
	}
	next, outcome, err := g.Reveal(b, nil)
	if err != nil {
		t.Fatalf("second Reveal: %v", err)
	}
	if !outcome.Matched || outcome.MatchedBy != Human {
		t.Fatalf("outcome = %+v, want Matched by Human", outcome)
	}
	if next.Turn != Human {
		t.Errorf("Turn = %v after a match, want it to stay Human (match => go again)", next.Turn)
	}
	if next.Pending != -1 {
		t.Errorf("Pending = %d after resolving a pick, want -1", next.Pending)
	}
	if owner := next.PairOwner[g.Faces[a]]; owner != Human {
		t.Errorf("PairOwner[matched pair] = %v, want Human", owner)
	}
}

func TestReveal_MismatchEndsTurn(t *testing.T) {
	g := newTestGame(t, 3, 1, 4)
	a, b := findMismatch(g.Faces)

	g, _, err := g.Reveal(a, nil)
	if err != nil {
		t.Fatalf("first Reveal: %v", err)
	}
	next, outcome, err := g.Reveal(b, nil)
	if err != nil {
		t.Fatalf("second Reveal: %v", err)
	}
	if !outcome.TurnEnded || outcome.Matched {
		t.Fatalf("outcome = %+v, want TurnEnded and not Matched", outcome)
	}
	if next.Turn != Robot {
		t.Errorf("Turn = %v after a mismatch, want Robot", next.Turn)
	}
	if next.Pending != -1 {
		t.Errorf("Pending = %d after a mismatch, want -1", next.Pending)
	}
}

func TestReveal_ErrorsOnAlreadyMatchedCell(t *testing.T) {
	g := newTestGame(t, 3, 1, 4)
	a, b := findPair(g.Faces)
	g, _, _ = g.Reveal(a, nil)
	g, _, _ = g.Reveal(b, nil) // matches, frees a and b

	if _, _, err := g.Reveal(a, nil); err != ErrCellAlreadyMatched {
		t.Errorf("Reveal(already-matched cell) = %v, want ErrCellAlreadyMatched", err)
	}
}

func TestReveal_ErrorsOnPendingCellClickedAgain(t *testing.T) {
	g := newTestGame(t, 3, 1, 4)
	g, _, _ = g.Reveal(0, nil)
	if _, _, err := g.Reveal(0, nil); err != ErrCellIsPending {
		t.Errorf("Reveal(pending cell again) = %v, want ErrCellIsPending", err)
	}
}

func TestReveal_ErrorsOnInvalidCell(t *testing.T) {
	g := newTestGame(t, 3, 1, 4)
	for _, cell := range []int{-1, len(g.Faces)} {
		if _, _, err := g.Reveal(cell, nil); err != ErrInvalidCell {
			t.Errorf("Reveal(%d) = %v, want ErrInvalidCell", cell, err)
		}
	}
}

func TestReveal_ErrorsOnceGameOver(t *testing.T) {
	g := newTestGame(t, 0, 1, 0) // 2x2, the smallest board: 2 pairs
	g = playToCompletion(t, g)
	if _, _, err := g.Reveal(0, nil); err != ErrGameOver {
		t.Errorf("Reveal after completion = %v, want ErrGameOver", err)
	}
}

func TestReveal_MemoryFIFORespectsCapacityAndDedup(t *testing.T) {
	g := newTestGame(t, 3, 1, 2) // 4x4 board, memory capacity 2
	a, b := findMismatch(g.Faces)
	g, _, _ = g.Reveal(a, nil)
	g, _, _ = g.Reveal(b, nil) // mismatch, turn passes; a and b are remembered

	if len(g.Memory) != 2 {
		t.Fatalf("Memory = %v, want 2 entries (capacity N=2)", g.Memory)
	}

	// A third distinct reveal must evict the oldest (a), keeping capacity 2.
	third := -1
	for cell := range g.Faces {
		if cell != a && cell != b {
			third = cell
			break
		}
	}
	g, _, _ = g.Reveal(third, nil)
	if len(g.Memory) != 2 {
		t.Fatalf("Memory = %v, want capacity to stay at 2 after a 3rd distinct reveal", g.Memory)
	}
	for _, m := range g.Memory {
		if m == a {
			t.Errorf("Memory %v still holds the oldest cell %d past capacity 2", g.Memory, a)
		}
	}
}

func TestReveal_MatchForgetsBothCellsFromMemory(t *testing.T) {
	g := newTestGame(t, 3, 1, 4)
	a, b := findPair(g.Faces)
	g, _, _ = g.Reveal(a, nil)
	g, _, outcome := g.Reveal(b, nil)
	_ = outcome
	for _, m := range g.Memory {
		if m == a || m == b {
			t.Errorf("Memory %v still holds a matched cell (%d,%d)", g.Memory, a, b)
		}
	}
}

func TestReveal_ZeroCapacityMemoryStaysEmpty(t *testing.T) {
	g := newTestGame(t, 3, 1, 0)
	g, _, _ = g.Reveal(0, nil)
	if len(g.Memory) != 0 {
		t.Errorf("Memory = %v, want empty at N=0", g.Memory)
	}
}

func TestScoreTracksMatchedPairsPerOwner(t *testing.T) {
	g := newTestGame(t, 0, 1, 0) // 2x2: exactly 2 pairs
	g = playToCompletion(t, g)
	human, robot := g.Score()
	if human+robot != 2 {
		t.Fatalf("Score() = (%d, %d), want them to sum to 2 pairs", human, robot)
	}
}
