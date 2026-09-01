package pairgame

import "testing"

// newTestGame builds a small inline-layout game with the given number of
// human seats so tests can read Faces directly instead of threading a
// secret through every call.
func newTestGame(t *testing.T, sizeIndex int, seed uint32, players int) GameState {
	t.Helper()
	setup := make([]PlayerSetup, players)
	g, err := NewGame(LayoutInline, sizeIndex, seed, nil, setup)
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

// playToCompletion drives g to completion by having player 1 alone reveal
// every genuine matching pair in turn (never a deliberate mismatch), so it
// always makes progress and finishes in exactly Sizes[g.SizeIndex].Pairs()
// rounds — every pair credited to player 1.
func playToCompletion(t *testing.T, g GameState) GameState {
	t.Helper()
	for !g.IsComplete() {
		a, b := -1, -1
		seenAt := make(map[uint8]int, len(g.PairOwner))
		for cell, pairID := range g.Faces {
			if g.PairOwner[pairID] != NoPlayer {
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
		if _, err := Flip(&g, nil, 1, a); err != nil {
			t.Fatalf("playToCompletion: Flip(%d): %v", a, err)
		}
		if _, err := Flip(&g, nil, 1, b); err != nil {
			t.Fatalf("playToCompletion: Flip(%d): %v", b, err)
		}
	}
	return g
}

func TestFlip_FirstPickSetsPending(t *testing.T) {
	g := newTestGame(t, 3, 1, 2) // 4x4, 2 players
	outcome, err := Flip(&g, nil, 1, 0)
	if err != nil {
		t.Fatalf("Flip: %v", err)
	}
	p, _ := g.Player(1)
	if p.Pending != 0 {
		t.Errorf("Pending = %d, want 0", p.Pending)
	}
	if outcome.Matched || outcome.GameOver {
		t.Errorf("unexpected outcome on a first pick: %+v", outcome)
	}
	if outcome.Reveal != (Reveal{By: 1, Cell: 0, PairID: g.Faces[0], Matched: false}) {
		t.Errorf("Reveal = %+v, want a first-pick log entry", outcome.Reveal)
	}
	if len(g.Log) != 1 || g.Log[0] != outcome.Reveal {
		t.Errorf("Log = %v, want exactly the returned Reveal", g.Log)
	}
}

func TestFlip_MatchCreditsTheActingPlayerAndClearsPending(t *testing.T) {
	g := newTestGame(t, 3, 1, 2)
	a, b := findPair(g.Faces)

	if _, err := Flip(&g, nil, 1, a); err != nil {
		t.Fatalf("first Flip: %v", err)
	}
	outcome, err := Flip(&g, nil, 1, b)
	if err != nil {
		t.Fatalf("second Flip: %v", err)
	}
	if !outcome.Matched || outcome.MatchedBy != 1 {
		t.Fatalf("outcome = %+v, want Matched by player 1", outcome)
	}
	p, _ := g.Player(1)
	if p.Pending != -1 {
		t.Errorf("Pending = %d after resolving a pick, want -1", p.Pending)
	}
	if p.Score != 1 {
		t.Errorf("Score = %d, want 1", p.Score)
	}
	if owner := g.PairOwner[g.Faces[a]]; owner != 1 {
		t.Errorf("PairOwner[matched pair] = %v, want player 1", owner)
	}
}

func TestFlip_MismatchClearsPendingWithoutScoring(t *testing.T) {
	g := newTestGame(t, 3, 1, 2)
	a, b := findMismatch(g.Faces)

	if _, err := Flip(&g, nil, 1, a); err != nil {
		t.Fatalf("first Flip: %v", err)
	}
	outcome, err := Flip(&g, nil, 1, b)
	if err != nil {
		t.Fatalf("second Flip: %v", err)
	}
	if outcome.Matched {
		t.Fatalf("outcome = %+v, want not Matched", outcome)
	}
	p, _ := g.Player(1)
	if p.Pending != -1 {
		t.Errorf("Pending = %d after a mismatch, want -1", p.Pending)
	}
	if p.Score != 0 {
		t.Errorf("Score = %d after a mismatch, want 0", p.Score)
	}
}

// TestFlip_NoTurnOrder is the core rule-difference test: with no turn order,
// player 2 may act immediately after player 1's first pick without player 1
// ever having "ended their turn" — there is no such concept any more.
func TestFlip_NoTurnOrder(t *testing.T) {
	g := newTestGame(t, 3, 1, 2)
	a, _ := findMismatch(g.Faces)

	if _, err := Flip(&g, nil, 1, a); err != nil {
		t.Fatalf("player 1 Flip: %v", err)
	}
	// Player 2 acts next, with player 1's pick still pending — this must be
	// legal under the new rules (it was structurally impossible before).
	other := -1
	for cell := range g.Faces {
		if cell != a {
			other = cell
			break
		}
	}
	if _, err := Flip(&g, nil, 2, other); err != nil {
		t.Fatalf("player 2 Flip while player 1 still has a pending pick: %v", err)
	}
	p1, _ := g.Player(1)
	if p1.Pending != a {
		t.Errorf("player 1's Pending changed as a side effect of player 2's flip: %d, want %d", p1.Pending, a)
	}
}

// TestFlip_TwoPlayersMayShareAPendingCell proves the founder's explicit
// "two players may hold the same cell pending at the same time" rule.
func TestFlip_TwoPlayersMayShareAPendingCell(t *testing.T) {
	g := newTestGame(t, 3, 1, 2)
	if _, err := Flip(&g, nil, 1, 5); err != nil {
		t.Fatalf("player 1 Flip: %v", err)
	}
	if _, err := Flip(&g, nil, 2, 5); err != nil {
		t.Fatalf("player 2 Flip on the same cell: %v", err)
	}
	p1, _ := g.Player(1)
	p2, _ := g.Player(2)
	if p1.Pending != 5 || p2.Pending != 5 {
		t.Fatalf("both players should have Pending=5, got p1=%d p2=%d", p1.Pending, p2.Pending)
	}
}

// TestFlip_AnyPlayerMayClaimAnOpponentsExposedPair is the founder's
// "sniping" rule: player 1 opens both halves of a pair as two separate
// first picks (never matching themselves because each Flip is player 1's
// own second pick only against player 1's own pending — so opening both
// halves without an intervening resolution requires two different actors).
// Concretely: player 1 opens cell a (first pick). Player 2 opens cell b,
// the SAME pair's other cell, as player 2's OWN first pick — no match yet,
// since match resolution only compares an acting player's own two picks.
// Player 2 then opens some third cell as their second pick (a deliberate
// mismatch, so player 2 does NOT claim the pair themselves) — proving the
// pair sits fully revealed (both halves publicly logged) but still
// unmatched. Player 1 (who still holds cell a pending) now opens cell b as
// their own second pick and claims it — the snipe.
func TestFlip_AnyPlayerMayClaimAnOpponentsExposedPair(t *testing.T) {
	g := newTestGame(t, 3, 1, 2) // 4x4: 16 cells, plenty of room
	a, b := findPair(g.Faces)

	if _, err := Flip(&g, nil, 1, a); err != nil {
		t.Fatalf("player1 opens a: %v", err)
	}
	if _, err := Flip(&g, nil, 2, b); err != nil {
		t.Fatalf("player2 opens b (same pair, their own first pick): %v", err)
	}
	if owner := g.PairOwner[g.Faces[a]]; owner != NoPlayer {
		t.Fatalf("pair should still be unmatched after two different players each hold one half pending, got owner=%v", owner)
	}

	// Player 1 claims it by flipping cell b as their own second pick.
	outcome, err := Flip(&g, nil, 1, b)
	if err != nil {
		t.Fatalf("player1 snipes b: %v", err)
	}
	if !outcome.Matched || outcome.MatchedBy != 1 {
		t.Fatalf("outcome = %+v, want player 1 to claim the pair", outcome)
	}
	p2, _ := g.Player(2)
	if p2.Score != 0 {
		t.Errorf("player 2 Score = %d, want 0 (they never completed their own pick)", p2.Score)
	}
}

// TestFlip_StalePendingSelfCorrectsOnNextFlip is the founder's "pending can
// go stale" rule: player 2 holds cell b pending (the partner of a pair
// player 1 then completes by claiming both halves themselves). Player 2's
// very next flip — necessarily some other cell, since b is now unflippable
// — must simply fail to match (no panic, no special-cased "stale" error)
// and clear player 2's pending, exactly like an ordinary mismatch.
func TestFlip_StalePendingSelfCorrectsOnNextFlip(t *testing.T) {
	g := newTestGame(t, 3, 1, 2) // 4x4: 16 cells, room for a 3rd untouched cell
	a, b := findPair(g.Faces)

	if _, err := Flip(&g, nil, 2, b); err != nil {
		t.Fatalf("player2 opens b: %v", err)
	}
	// Player 1 completes the same pair independently, stealing both halves
	// out from under player 2's still-pending b.
	if _, err := Flip(&g, nil, 1, a); err != nil {
		t.Fatalf("player1 opens a: %v", err)
	}
	if _, err := Flip(&g, nil, 1, b); err != nil {
		t.Fatalf("player1 claims b as their own second pick: %v", err)
	}
	if owner := g.PairOwner[g.Faces[a]]; owner != 1 {
		t.Fatalf("pair owner = %v, want player 1", owner)
	}

	// Player 2's pending (b) is now stale: b itself is unflippable, so their
	// very next flip is necessarily some other cell — which must resolve as
	// an ordinary, unremarkable mismatch against the stale pending face.
	d := -1
	for cell := range g.Faces {
		if cell != a && cell != b {
			d = cell
			break
		}
	}
	outcome, err := Flip(&g, nil, 2, d)
	if err != nil {
		t.Fatalf("player2's next flip on a stale pending: %v", err)
	}
	if outcome.Matched {
		t.Fatalf("outcome = %+v, want no match (stale pending can never spuriously match)", outcome)
	}
	p2, _ := g.Player(2)
	if p2.Pending != -1 {
		t.Errorf("player2 Pending = %d after the stale pick resolved, want -1", p2.Pending)
	}
	if p2.Score != 0 {
		t.Errorf("player2 Score = %d, want 0", p2.Score)
	}
}

func TestFlip_ErrorsOnAlreadyMatchedCell(t *testing.T) {
	g := newTestGame(t, 3, 1, 1)
	a, b := findPair(g.Faces)
	if _, err := Flip(&g, nil, 1, a); err != nil {
		t.Fatalf("Flip(a): %v", err)
	}
	if _, err := Flip(&g, nil, 1, b); err != nil {
		t.Fatalf("Flip(b): %v", err)
	}
	if _, err := Flip(&g, nil, 1, a); err != ErrCellAlreadyMatched {
		t.Errorf("Flip(already-matched cell) = %v, want ErrCellAlreadyMatched", err)
	}
}

func TestFlip_ErrorsOnOwnPendingCellClickedAgain(t *testing.T) {
	g := newTestGame(t, 3, 1, 1)
	if _, err := Flip(&g, nil, 1, 0); err != nil {
		t.Fatalf("Flip(0): %v", err)
	}
	if _, err := Flip(&g, nil, 1, 0); err != ErrCellIsPending {
		t.Errorf("Flip(own pending cell again) = %v, want ErrCellIsPending", err)
	}
}

func TestFlip_ErrorsOnInvalidCell(t *testing.T) {
	g := newTestGame(t, 3, 1, 1)
	for _, cell := range []int{-1, len(g.Faces)} {
		if _, err := Flip(&g, nil, 1, cell); err != ErrInvalidCell {
			t.Errorf("Flip(%d) = %v, want ErrInvalidCell", cell, err)
		}
	}
}

func TestFlip_ErrorsOnUnknownPlayer(t *testing.T) {
	g := newTestGame(t, 3, 1, 2)
	if _, err := Flip(&g, nil, 99, 0); err != ErrUnknownPlayer {
		t.Errorf("Flip(unknown player) = %v, want ErrUnknownPlayer", err)
	}
	if _, err := Flip(&g, nil, NoPlayer, 0); err != ErrUnknownPlayer {
		t.Errorf("Flip(NoPlayer) = %v, want ErrUnknownPlayer", err)
	}
}

func TestFlip_ErrorsOnceGameOver(t *testing.T) {
	g := newTestGame(t, 0, 1, 1) // 2x2, the smallest board: 2 pairs
	g = playToCompletion(t, g)
	if _, err := Flip(&g, nil, 1, 0); err != ErrGameOver {
		t.Errorf("Flip after completion = %v, want ErrGameOver", err)
	}
}

func TestWinners_SingleWinner(t *testing.T) {
	g := newTestGame(t, 0, 1, 2)
	g.Players[0].Score = 2
	g.Players[1].Score = 0
	winners := g.Winners()
	if len(winners) != 1 || winners[0] != 1 {
		t.Errorf("Winners() = %v, want [1]", winners)
	}
}

func TestWinners_TieIsRepresentable(t *testing.T) {
	g := newTestGame(t, 0, 1, 3)
	g.Players[0].Score = 1
	g.Players[1].Score = 1
	g.Players[2].Score = 0
	winners := g.Winners()
	if len(winners) != 2 || winners[0] != 1 || winners[1] != 2 {
		t.Errorf("Winners() = %v, want [1 2]", winners)
	}
}

func TestLegalMoves_ExcludesOnlyOwnPending(t *testing.T) {
	g := newTestGame(t, 3, 1, 2)
	if _, err := Flip(&g, nil, 1, 3); err != nil {
		t.Fatalf("Flip: %v", err)
	}
	movesFor1 := g.LegalMoves(g.Faces, 1)
	for _, m := range movesFor1 {
		if m == 3 {
			t.Fatalf("LegalMoves(by=1) should exclude player 1's own pending cell 3, got %v", movesFor1)
		}
	}
	movesFor2 := g.LegalMoves(g.Faces, 2)
	found := false
	for _, m := range movesFor2 {
		if m == 3 {
			found = true
		}
	}
	if !found {
		t.Fatalf("LegalMoves(by=2) should still offer cell 3 (player 1's pending does not block player 2), got %v", movesFor2)
	}
}
