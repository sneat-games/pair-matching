package pairgame

import "testing"

// --- 2018 conformance harness: server-go/pairgame/open_test.go -------------
//
// This file ports the rule-behaviour assertions of the original 2018
// engine's TestOpenCell (sneat-games/pair-matching, branch
// archive/2018-original-engine, commit 9f2a078,
// server-go/pairgame/open_test.go) onto the rewritten engine's Reveal. The
// founder revived this game because "it really worked" -- this file is
// what turns that claim about the ORIGINAL board-game rules into evidence
// against the rewrite, not a restoration of the old API.
//
// Address/size mapping -- verified against the original's own dependency
// (github.com/prizarena/turnbased@v0.0.1's table.go), not merely inferred
// from test behaviour, and cross-checked against pairmodels/board.go's
// GetCell:
//   - A Size string is turnbased.NewSize(width, height): the first rune is
//     'A'+width-1, the rest is the height digit(s). So "D3" decodes to
//     width=4 ('D'-'A'+1), height=3 -- a 4-column, 3-row, 12-cell board
//     (matching the original's "Cells: 1,2,3,4,5,6,6,5,4,3,2,1", 12 values).
//     This is NOT the {Width:3,Height:4} preset already in this package's
//     Sizes (index 2) -- that preset is transposed relative to "D3".
//   - A CellAddress string is "<col-letter><row-digit>": X()=letter-'A'
//     (0-indexed column), Y()=digit-'1' (0-indexed row). GetCell resolves
//     an address via board.Size.Width()*Y()+X() (board.go), i.e. row-major
//     using the board's WIDTH -- identical to turnbased.CellAddress.Index.
//   - The rewritten engine has no coordinate/address type at all -- Reveal
//     takes a flat `cell int` -- so cellD3 below reproduces that same
//     row-major formula (width=4, the "D3" board's actual width) to convert
//     each "A2"-style address from the original test into the correct flat
//     index. Getting the width wrong here (e.g. using 3, the transposed
//     preset's width) would silently exercise the wrong cells; cellD3's
//     values were checked by hand against board.go's GetCell for every
//     address the original test uses (A1, A2, A3, B2, C2, D3) before this
//     file was written -- see the mapping table in newD3Game's comment.
//
// The rewritten engine models a cell's face as a pair id (Faces []uint8,
// each id 0..pairs-1 appearing exactly twice) rather than a display string,
// so the original's "Cells: 1,2,3,4,5,6,6,5,4,3,2,1" is carried over as
// pair ids 0..5 in the same positions (display value v -> pair id v-1)
// under LayoutInline -- the "measured baseline" explicit-layout mode this
// engine already offers (GameState's fields are exported and Mode ==
// LayoutInline reads Faces directly; see state.go's FacesWith). No
// production API, test hook, or exported seam was added to build this.
//
// # What could not be ported, and the divergence this surfaces
//
// The original TestOpenCell's case_1 is NOT a strict alternating-turn,
// two-picks-per-turn sequence: it drives player p1 through FIVE consecutive
// opens (including a mismatch at steps 1-2 followed immediately by a THIRD
// p1 open at step 3, with no intervening p2 move), then has p2 (step 6)
// complete a match against a cell p1 left open from step 5 -- crediting p2
// for matching p1's own still-pending pick, again with no alternation.
// OpenCell (open.go in the archived engine) supports this because it takes
// an explicit `player` argument and checks every player's Open1/Open2
// fields for a match on every call, with a comment noting matches are not
// even deduplicated "in case of race" -- this reads as plumbing for
// asynchronous, out-of-order Telegram callback delivery, not a deliberate
// gameplay rule (nothing in the test names or board narrative suggests
// "steal your opponent's exposed card" was an intended mechanic).
//
// The rewritten engine's Reveal has no `player`/acting-side argument at
// all: it is a pure function of GameState, and the ONLY player who can ever
// complete a pending pick is whoever currently holds g.Turn (Turn flips
// only on a mismatch; a match keeps it). There is structurally no way to
// call Reveal "as the other side" while it is not that side's turn, and no
// way to leave a mismatched pick "still open" for a future pick to land on
// -- the second pick always resolves Pending immediately, win or lose (see
// moves.go's Reveal doc comment: "classic concentration/memory-match
// rules"). So:
//
//   - Steps 1-2 (A2/A3 mismatch) and step 3 (B2, same player p1 again) could
//     NOT be chained into one scenario: under the rewritten engine's rules
//     the mismatch at steps 1-2 would flip the turn to Robot, so Robot --
//     not Human -- would be the one legally able to open B2 next, which
//     would then credit the step 3-4 match to the wrong side relative to
//     the original. Ported below as two independent, freshly-started
//     scenarios instead (TestConformance2018_OpenCell_SameTurnMismatchEndsTurn
//     for steps 1-2; TestConformance2018_OpenCell_SameTurnMatchThenContinue
//     for steps 3-4-5, which chain legitimately because turn-keeping on a
//     match IS common to both engines).
//   - Step 6 (p2 matching p1's still-open A1) has no rewritten-engine
//     equivalent whatsoever and is NOT ported. Not because it's expensive
//     to set up, but because it is not expressible: Reveal has no
//     "acting side" parameter to force, and adding one purely to make this
//     test possible would be exactly the kind of test-only production seam
//     the brief asks NOT to add speculatively.
//
// I believe the rewritten engine's strict alternation is the correct
// behaviour to keep going forward (it is what "classic concentration"
// means, and the original's own code comments point at concurrency
// plumbing rather than intentional design) -- but this is a real rules
// difference the founder should confirm, not something silently dropped;
// see the PR description for the same note in one place.

// newD3Game builds the original test case_1's board -- Size "D3" (4 cols x
// 3 rows), Cells "1,2,3,4,5,6,6,5,4,3,2,1" -- as a fresh Human-to-move
// LayoutInline game. Row-major with width=4, display value v -> pair id
// v-1:
//
//	row1 (y=0): A1=1 B1=2 C1=3 D1=4   -> faces[0..3]  = 0,1,2,3
//	row2 (y=1): A2=5 B2=6 C2=6 D2=5   -> faces[4..7]  = 4,5,5,4
//	row3 (y=2): A3=4 B3=3 C3=2 D3=1   -> faces[8..11] = 3,2,1,0
//
// SizeIndex points at Sizes[2] (3x4=12 cells/6 pairs) purely to size
// PairOwner correctly -- Reveal never consults Width/Height, only
// len(Faces) and PairOwner, so the transposed shape is immaterial here.
func newD3Game(t *testing.T) GameState {
	t.Helper()
	return GameState{
		SizeIndex: 2,
		Mode:      LayoutInline,
		Faces:     []uint8{0, 1, 2, 3, 4, 5, 5, 4, 3, 2, 1, 0},
		PairOwner: make([]Owner, 6),
		Pending:   -1,
		Turn:      Human,
	}
}

// cellD3 converts a "<col-letter><row-digit>" address (as used by the
// original test, e.g. "A2") on the 4-wide/3-tall "D3" board into the
// rewritten engine's flat cell index: (row-1)*width + (col-'A'), width=4 --
// see this file's header comment for how that formula was derived and
// cross-checked against the original's own turnbased.CellAddress.Index and
// pairmodels board.go's GetCell.
func cellD3(col byte, row int) int {
	const width = 4
	return (row-1)*width + int(col-'A')
}

// TestConformance2018_OpenCell_SameTurnMismatchEndsTurn ports case_1's
// steps 1-2: p1 opens "A2" (value 5) then "A3" (value 4) -- different
// values, no match. The original only asserted Open1/Open2/MatchedCount
// stayed at their non-matching values; the rewritten engine additionally
// resolves the pick immediately and flips the turn (see this file's header
// comment on why that representational difference is not a scoring
// difference).
func TestConformance2018_OpenCell_SameTurnMismatchEndsTurn(t *testing.T) {
	g := newD3Game(t)
	a2 := cellD3('A', 2) // value 5
	a3 := cellD3('A', 3) // value 4

	g, _, err := g.Reveal(a2, nil)
	if err != nil {
		t.Fatalf("Reveal(A2): %v", err)
	}
	g, outcome, err := g.Reveal(a3, nil)
	if err != nil {
		t.Fatalf("Reveal(A3): %v", err)
	}
	if outcome.Matched || !outcome.TurnEnded {
		t.Fatalf("outcome = %+v, want a mismatch (TurnEnded, not Matched) -- old: no MatchedCount change", outcome)
	}
	if g.Turn != Robot {
		t.Fatalf("Turn = %v after the mismatch, want Robot", g.Turn)
	}
	if human, robot := g.Score(); human != 0 || robot != 0 {
		t.Fatalf("Score = (%d,%d), want (0,0): a mismatch credits nobody -- old: both players' MatchedCount stayed 0", human, robot)
	}
}

// TestConformance2018_OpenCell_SameTurnMatchThenContinue ports case_1's
// steps 3-4-5 as one continuous scenario -- this chains legitimately
// (unlike steps 1-2-3) because turn-keeping on a match is a rule both
// engines share:
//
//	step 3: p1 opens "B2" (value 6)                         -> first pick
//	step 4: p1 opens "C2" (value 6) -- matches B2            -> old:
//	        Open1="B2" Open2="C2" MatchedItems="6" MatchedCount=1
//	step 5: p1 opens "A1" (value 1) -- a fresh first pick     -> old:
//	        Open1="A1" Open2="" MatchedItems="6" MatchedCount=1 (unchanged)
func TestConformance2018_OpenCell_SameTurnMatchThenContinue(t *testing.T) {
	g := newD3Game(t)
	b2 := cellD3('B', 2) // value 6
	c2 := cellD3('C', 2) // value 6
	a1 := cellD3('A', 1) // value 1

	g, outcome, err := g.Reveal(b2, nil) // old step 3
	if err != nil {
		t.Fatalf("Reveal(B2): %v", err)
	}
	if outcome != (RevealOutcome{}) {
		t.Fatalf("first pick outcome = %+v, want the zero value", outcome)
	}

	g, outcome, err = g.Reveal(c2, nil) // old step 4
	if err != nil {
		t.Fatalf("Reveal(C2): %v", err)
	}
	if !outcome.Matched || outcome.MatchedBy != Human {
		t.Fatalf("outcome = %+v, want Matched by Human -- old: MatchedItems=\"6\" MatchedCount=1", outcome)
	}
	if g.Turn != Human {
		t.Fatalf("Turn = %v after the match, want Human to keep the turn -- old: same player free to continue", g.Turn)
	}
	if owner := g.PairOwner[g.Faces[b2]]; owner != Human {
		t.Fatalf("PairOwner[value-6 pair] = %v, want Human", owner)
	}
	if human, _ := g.Score(); human != 1 {
		t.Fatalf("Score human = %d, want 1 -- old: MatchedCount=1", human)
	}

	g, outcome, err = g.Reveal(a1, nil) // old step 5
	if err != nil {
		t.Fatalf("Reveal(A1): %v", err)
	}
	if outcome != (RevealOutcome{}) {
		t.Fatalf("third pick outcome = %+v, want the zero value (a fresh first pick, no new match)", outcome)
	}
	if g.Pending != a1 {
		t.Fatalf("Pending = %d, want %d (A1 is the new first pick) -- old: Open1=\"A1\" Open2=\"\"", g.Pending, a1)
	}
	if human, _ := g.Score(); human != 1 {
		t.Fatalf("Score human = %d after the 3rd pick, want still 1 -- old: MatchedItems/MatchedCount unchanged (\"6\"/1)", human)
	}
}

// TestConformance2018_OpenCell_FirstPickOfGameNeverMatches ports case_2: a
// distinct-faces board where the very first reveal of the game cannot
// match anything yet. Directly portable -- a first pick is a first pick in
// both engines. Original Cells: "🚂,🚷,🚂,🚍,🚺,🚄,🚒,🚷,🚍,🚄,🚒,🚺" (Size
// "D3"); in first-appearance order that is pair ids
// 0,1,0,2,3,4,5,1,2,4,5,3. Old step: p1 opens "A1" -> Open1="A1"
// MatchedItems="" MatchedCount=0.
func TestConformance2018_OpenCell_FirstPickOfGameNeverMatches(t *testing.T) {
	g := GameState{
		SizeIndex: 2,
		Mode:      LayoutInline,
		Faces:     []uint8{0, 1, 0, 2, 3, 4, 5, 1, 2, 4, 5, 3},
		PairOwner: make([]Owner, 6),
		Pending:   -1,
		Turn:      Human,
	}
	a1 := cellD3('A', 1)

	g, outcome, err := g.Reveal(a1, nil)
	if err != nil {
		t.Fatalf("Reveal(A1): %v", err)
	}
	if outcome != (RevealOutcome{}) {
		t.Fatalf("outcome = %+v, want the zero value -- old: MatchedItems=\"\" MatchedCount=0", outcome)
	}
	if g.Pending != a1 {
		t.Fatalf("Pending = %d, want %d -- old: Open1=\"A1\"", g.Pending, a1)
	}
	if human, robot := g.Score(); human != 0 || robot != 0 {
		t.Fatalf("Score = (%d,%d), want (0,0)", human, robot)
	}
}
