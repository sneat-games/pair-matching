package pairgame

import (
	"fmt"
	"testing"
)

// --- 2018 conformance harness: server-go/pairmodels/board_test.go ---------
//
// Ports TestShuffle and its verifyBoard helper (sneat-games/pair-matching,
// branch archive/2018-original-engine, commit 9f2a078,
// server-go/pairmodels/board_test.go) onto the rewritten engine's
// ShuffleFaces, the LayoutInline counterpart of the original's
// pairmodels.NewCells(width, height) -- both produce a fresh shuffled
// layout for a board of that shape (see layout.go's LayoutMode doc
// comment). Only TestShuffle/verifyBoard are in scope, per the brief;
// board_test.go's DrawBoard/GetCell/IsCompleted cases test delivery-layer
// concerns already exercised elsewhere and are not ported here.
//
// verify2018Board deliberately mirrors the ORIGINAL verifyBoard's exact two
// checks -- no face appears more than twice; the total item count is
// exactly pairs*2 -- including its (err error) return shape, rather than
// reusing this package's own (already correct, and broader) assertValidLayout
// in layout_test.go. The point of this file is to prove the HISTORICAL
// assertion is live against the rewrite, not to lean on unrelated coverage.
// That original helper is exactly where the repo has form: a bare
// fmt.Errorf(...) with no `return` once made the item-count check silently
// unreachable for eight years (fixed before this engine's archived branch
// was pushed -- see that branch's commit message -- but this file proves
// ITS OWN port of the check is live, not the original's history).
// TestConformance2018_VerifyBoardHelperCatchesADuplicate below drives it
// with a synthetically invalid layout to prove that; the ShuffleFaces-level
// mutation used to prove the same for the real (non-synthetic) assertion is
// recorded in the PR description, not committed here.
func verify2018Board(pairs int, faces []uint8) error {
	counts := make(map[uint8]int, pairs)
	for _, f := range faces {
		counts[f]++
		if counts[f] > 2 {
			return fmt.Errorf("more than 2 items of face %d", f)
		}
	}
	if len(faces) != pairs*2 {
		return fmt.Errorf("expected %d items, got %d", pairs*2, len(faces))
	}
	return nil
}

// TestConformance2018_ShuffleProducesAValidBoard ports TestShuffle's three
// cases -- test(1,2,2), test(2,3,4), test(3,8,8) -- onto ShuffleFaces at the
// equivalent presets already in Sizes: {2,2} (index 0), {3,4} (index 2 --
// note TestShuffle passed x,y positionally as width,height directly, unlike
// open_test.go's "D3" size-string encoding, so this {3,4} genuinely is
// Sizes[2] with no transposition), {8,8} (index 8). Seed 2018 is arbitrary
// but fixed, for a reproducible failure message.
func TestConformance2018_ShuffleProducesAValidBoard(t *testing.T) {
	for _, size := range []Size{{Width: 2, Height: 2}, {Width: 3, Height: 4}, {Width: 8, Height: 8}} {
		faces := ShuffleFaces(2018, size.Pairs())
		if err := verify2018Board(size.Pairs(), faces); err != nil {
			t.Errorf("%+v shuffling: %v", size, err)
		}
	}
}

// TestConformance2018_VerifyBoardHelperCatchesADuplicate proves
// verify2018Board can actually fail -- the exact property the original
// verifyBoard lost for eight years. Independent of the mutation testing
// performed (and reverted) against ShuffleFaces itself for this harness's
// failure-injection requirement.
func TestConformance2018_VerifyBoardHelperCatchesADuplicate(t *testing.T) {
	if err := verify2018Board(2, []uint8{0, 0, 0, 1}); err == nil {
		t.Fatal("verify2018Board did not report an error for a face appearing 3 times")
	}
}
