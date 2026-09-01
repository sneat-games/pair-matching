package pairgame

import (
	"math/rand"
	"testing"
)

func TestRandomMover_ChoosesALegalMove(t *testing.T) {
	g := newTestGame(t, 3, 7, 0)
	mover := RandomMover{Rand: rand.New(rand.NewSource(1))}
	for i := 0; i < 20; i++ {
		cell := mover.Choose(g, g.Faces)
		legal := false
		for _, m := range g.LegalMoves(g.Faces) {
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
	g := newTestGame(t, 0, 1, 0)
	g = playToCompletion(t, g)
	RandomMover{}.Choose(g, g.Faces)
}

// TestMemoryStrategy_TakesAGuaranteedSecondPick sets up a state where the
// robot's memory already holds the partner of its own pending pick, and
// checks it takes the guaranteed match instead of a fallback random cell.
func TestMemoryStrategy_TakesAGuaranteedSecondPick(t *testing.T) {
	g := newTestGame(t, 3, 7, 8) // memory capacity comfortably covers the board
	a, b := findPair(g.Faces)

	// Simulate the robot having seen cell b earlier (e.g. from a human
	// mismatch reveal), then opening cell a as its first pick this turn.
	g.Memory = []int{b}
	g.Turn = Robot
	next, _, err := g.Reveal(a, nil)
	if err != nil {
		t.Fatalf("Reveal(a): %v", err)
	}

	strat := MemoryStrategy{}
	got := strat.Choose(next, next.Faces)
	if got != b {
		t.Errorf("MemoryStrategy.Choose = %d, want the remembered partner %d", got, b)
	}
}

// TestMemoryStrategy_OpensARememberedPairOnFirstPick checks the first-pick
// path: if two cells in memory already share a face, MemoryStrategy should
// open one of them rather than falling back to random.
func TestMemoryStrategy_OpensARememberedPairOnFirstPick(t *testing.T) {
	g := newTestGame(t, 3, 7, 8)
	a, b := findPair(g.Faces)
	g.Memory = []int{a, b}
	g.Turn = Robot

	strat := MemoryStrategy{}
	got := strat.Choose(g, g.Faces)
	if got != a && got != b {
		t.Errorf("MemoryStrategy.Choose = %d, want one of the remembered pair {%d,%d}", got, a, b)
	}
}

// TestMemoryStrategy_FallsBackWithEmptyMemory checks that at N==0 (Memory
// always empty), MemoryStrategy behaves exactly like its Fallback.
func TestMemoryStrategy_FallsBackWithEmptyMemory(t *testing.T) {
	g := newTestGame(t, 3, 7, 0)
	strat := MemoryStrategy{Fallback: RandomMover{Rand: rand.New(rand.NewSource(3))}}
	cell := strat.Choose(g, g.Faces)
	legal := false
	for _, m := range g.LegalMoves(g.Faces) {
		if m == cell {
			legal = true
		}
	}
	if !legal {
		t.Fatalf("MemoryStrategy fallback returned %d, not a legal move", cell)
	}
}
