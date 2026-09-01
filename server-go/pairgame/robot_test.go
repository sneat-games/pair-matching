package pairgame

import (
	"math/rand"
	"testing"
)

func TestRandomMover_ChoosesALegalMove(t *testing.T) {
	g := newTestGame(t, 3, 7, 1)
	mover := RandomMover{Rand: rand.New(rand.NewSource(1))}
	for i := 0; i < 20; i++ {
		cell := mover.Choose(g, g.Faces, 1)
		legal := false
		for _, m := range g.LegalMoves(g.Faces, 1) {
			if m == cell {
				legal = true
				break
			}
		}
		if !legal {
			t.Fatalf("RandomMover.Choose returned %d, not in LegalMoves()", cell)
		}
	}
}

func TestRandomMover_PanicsWithNoLegalMoves(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic when there are no legal moves left")
		}
	}()
	g := newTestGame(t, 0, 1, 1)
	g = playToCompletion(t, g)
	RandomMover{}.Choose(g, g.Faces, 1)
}

// TestMemoryStrategy_TakesAGuaranteedSecondPick sets up a state where the
// bot's own second pick is guaranteed by a cell in the public log (put
// there by a HUMAN's earlier mismatch reveal — every flip is public, so
// the bot does not need to have flipped it itself).
func TestMemoryStrategy_TakesAGuaranteedSecondPick(t *testing.T) {
	g := newTestGame(t, 3, 7, 2) // 4x4, player 1 human, player 2 bot
	a, b := findPair(g.Faces)

	// Player 1 (human) reveals cell b as part of an unrelated mismatch,
	// publishing it to the log.
	c, _ := findMismatchExcluding(g.Faces, a, b)
	if _, err := Flip(&g, nil, 1, b); err != nil {
		t.Fatalf("player1 opens b: %v", err)
	}
	if _, err := Flip(&g, nil, 1, c); err != nil {
		t.Fatalf("player1 opens c (mismatch): %v", err)
	}

	g.Players[1].Memory = 8 // comfortably covers the board
	if _, err := Flip(&g, nil, 2, a); err != nil {
		t.Fatalf("bot opens a: %v", err)
	}

	strat := MemoryStrategy{}
	got := strat.Choose(g, g.Faces, 2)
	if got != b {
		t.Errorf("MemoryStrategy.Choose = %d, want the remembered partner %d", got, b)
	}
}

// TestMemoryStrategy_OpensARememberedPairOnFirstPick checks the first-pick
// path: if two cells in the bot's memory window already share a still-
// unmatched face, MemoryStrategy should open one of them rather than
// falling back to random.
func TestMemoryStrategy_OpensARememberedPairOnFirstPick(t *testing.T) {
	g := newTestGame(t, 3, 7, 2)
	a, b := findPair(g.Faces)
	g.Log = []Reveal{
		{By: 1, Cell: a, PairID: g.Faces[a]},
		{By: 1, Cell: b, PairID: g.Faces[b]},
	}
	g.Players[1].Memory = 8

	strat := MemoryStrategy{}
	got := strat.Choose(g, g.Faces, 2)
	if got != a && got != b {
		t.Errorf("MemoryStrategy.Choose = %d, want one of the remembered pair {%d,%d}", got, a, b)
	}
}

// TestMemoryStrategy_IgnoresAMatchedPairInTheWindow proves memory does not
// try to re-open a pair the log window still mentions but that has since
// been matched (by anyone) — the live PairOwner check, not the window
// contents, is authoritative.
func TestMemoryStrategy_IgnoresAMatchedPairInTheWindow(t *testing.T) {
	g := newTestGame(t, 3, 7, 2)
	a, b := findPair(g.Faces)
	if _, err := Flip(&g, nil, 1, a); err != nil {
		t.Fatalf("Flip(a): %v", err)
	}
	if _, err := Flip(&g, nil, 1, b); err != nil {
		t.Fatalf("Flip(b): %v", err)
	} // pair (a,b) is now matched by player 1; both log entries still exist

	g.Players[1].Memory = 8
	strat := MemoryStrategy{Fallback: RandomMover{Rand: rand.New(rand.NewSource(5))}}
	got := strat.Choose(g, g.Faces, 2)
	if got == a || got == b {
		t.Errorf("MemoryStrategy.Choose = %d, must not re-target an already-matched pair", got)
	}
}

// TestMemoryStrategy_WindowRespectsMemoryCapacity proves a bot with a small
// Memory cannot exploit a pair revealed further back than its window.
func TestMemoryStrategy_WindowRespectsMemoryCapacity(t *testing.T) {
	g := newTestGame(t, 3, 7, 2) // 4x4
	a, b := findPair(g.Faces)
	c, d := findMismatchExcluding(g.Faces, a, b)

	if _, err := Flip(&g, nil, 1, a); err != nil {
		t.Fatalf("Flip(a): %v", err)
	}
	if _, err := Flip(&g, nil, 1, c); err != nil { // mismatch: clears player1's pending
		t.Fatalf("Flip(c): %v", err)
	}
	if _, err := Flip(&g, nil, 1, b); err != nil { // pushes 'a' out of a 1-entry window
		t.Fatalf("Flip(b): %v", err)
	}
	_ = d

	g.Players[1].Memory = 1 // window holds only the single most recent entry (b)
	const seed = 9
	strat := MemoryStrategy{Fallback: RandomMover{Rand: rand.New(rand.NewSource(seed))}}
	got := strat.Choose(g, g.Faces, 2)
	// A window that cannot see 'a' finds no guaranteed match, so it must
	// fall straight through to Fallback — proved by exact agreement with a
	// freshly-seeded RandomMover consuming the same single draw, not by a
	// weaker "got != a" check (which 'a' could also satisfy by pure chance
	// if the strategy fell back to random anyway).
	want := RandomMover{Rand: rand.New(rand.NewSource(seed))}.Choose(g, g.Faces, 2)
	if got != want {
		t.Errorf("MemoryStrategy.Choose = %d, want %d (a same-seeded RandomMover — the window should not have found a guaranteed match for 'a')", got, want)
	}
}

// TestMemoryStrategy_FallsBackWithNoMemory checks that at Memory==0 (window
// always empty), MemoryStrategy behaves exactly like its Fallback.
func TestMemoryStrategy_FallsBackWithNoMemory(t *testing.T) {
	g := newTestGame(t, 3, 7, 2)
	strat := MemoryStrategy{Fallback: RandomMover{Rand: rand.New(rand.NewSource(3))}}
	cell := strat.Choose(g, g.Faces, 2)
	legal := false
	for _, m := range g.LegalMoves(g.Faces, 2) {
		if m == cell {
			legal = true
		}
	}
	if !legal {
		t.Fatalf("MemoryStrategy fallback returned %d, not a legal move", cell)
	}
}

// findMismatchExcluding returns two distinct cell indices with different
// faces, neither equal to skipA or skipB — used to keep a mismatch scenario
// from accidentally touching the pair under test.
func findMismatchExcluding(faces []uint8, skipA, skipB int) (a, b int) {
	for i := range faces {
		if i == skipA || i == skipB {
			continue
		}
		for j := range faces {
			if j == skipA || j == skipB || i == j {
				continue
			}
			if faces[i] != faces[j] {
				return i, j
			}
		}
	}
	panic("no mismatch found excluding the given cells")
}
