package pairgame

import "testing"

// --- 2018 conformance harness: server-go/pairgame/open_test.go -------------
//
// This file ports the rule-behaviour assertions of the original 2018
// engine's TestOpenCell (sneat-games/pair-matching, branch
// archive/2018-original-engine, commit 9f2a078,
// server-go/pairgame/open_test.go) onto the rewritten engine's Flip. The
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
//   - The rewritten engine has no coordinate/address type at all -- Flip
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
// # Full-fidelity port -- the alternation divergence is settled, not open
//
// An earlier version of this file could not port case_1 in full and said
// so explicitly: the REWRITE (not the original) had imposed strict
// alternating turns and a single acting side per pending pick, neither of
// which the archived 2018 OpenCell (open.go) ever had -- it takes an
// explicit `player` argument and checks EVERY player's own pending fields
// for a match on every call, with no alternation and no restriction on who
// may complete a pending pick. That file flagged this as "a real rules
// difference the founder should confirm, not something silently dropped."
//
// The founder has now confirmed it, twice: first that there is no turn
// order at all (any seated player may Flip at any moment, each with their
// own independent Pending), and second -- prompted by this file's own
// header, on exactly the ambiguity it raised -- that a flip matches against
// ANY seated player's exposed pending pick, not only the flipper's own,
// crediting the FLIPPER. Both rulings are exactly what the archived
// original's OpenCell already did. This file now ports case_1 in full,
// with no split into artificial sub-scenarios and no unported step:
//
//   - Steps 1-5 -- five consecutive p1 opens, including a mismatch (steps
//     1-2) immediately followed by a THIRD p1 open (step 3) with no
//     intervening p2 move -- port as ONE continuous scenario
//     (TestConformance2018_OpenCell_FiveConsecutiveOpensNoAlternation).
//     The rewrite's Flip has no turn to flip, so nothing forces a split.
//   - Step 6 -- p2 completing a match against p1's still-open A1, crediting
//     p2 rather than p1 -- ports directly as this same scenario's final
//     step: it is now exactly the founder-ruled sniping behaviour, and the
//     direct regression guard for it.

// newD3Game builds the original test case_1's board -- Size "D3" (4 cols x
// 3 rows), Cells "1,2,3,4,5,6,6,5,4,3,2,1" -- as a fresh two-human-player
// LayoutInline game (p1 = PlayerID 1, p2 = PlayerID 2, matching the
// original test's naming). Row-major with width=4, display value v -> pair
// id v-1:
//
//	row1 (y=0): A1=1 B1=2 C1=3 D1=4   -> faces[0..3]  = 0,1,2,3
//	row2 (y=1): A2=5 B2=6 C2=6 D2=5   -> faces[4..7]  = 4,5,5,4
//	row3 (y=2): A3=4 B3=3 C3=2 D3=1   -> faces[8..11] = 3,2,1,0
//
// SizeIndex points at Sizes[2] (3x4=12 cells/6 pairs) purely to size
// PairOwner correctly -- Flip never consults Width/Height, only len(Faces)
// and PairOwner, so the transposed shape is immaterial here.
func newD3Game(t *testing.T) GameState {
	t.Helper()
	g, err := NewGame(LayoutInline, 2, 0, nil, []PlayerSetup{{}, {}})
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	g.Faces = []uint8{0, 1, 2, 3, 4, 5, 5, 4, 3, 2, 1, 0}
	return g
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

// TestConformance2018_OpenCell_FiveConsecutiveOpensNoAlternation ports
// case_1 in full, six steps, one continuous game, exactly as the archived
// original ran it -- p1 (PlayerID 1) opens five cells back to back with no
// intervening p2 move, then p2 (PlayerID 2) claims p1's still-exposed A1
// in a single flip:
//
//	step 1: p1 opens "A2" (value 5)                    -> first pending, no match
//	step 2: p1 opens "A3" (value 4) -- mismatch          -> old: Open1/Open2/
//	        MatchedCount unchanged; rewrite: A3 REPLACES p1's pending outright
//	step 3: p1 opens "B2" (value 6)                      -> replaces pending again
//	step 4: p1 opens "C2" (value 6) -- matches B2         -> old: MatchedItems="6"
//	        MatchedCount=1; p1 scores, pending clears
//	step 5: p1 opens "A1" (value 1) -- a fresh pending    -> old: Open1="A1" Open2=""
//	step 6: p2 opens "D3" (value 1) -- matches p1's A1    -> founder-ruled: p2
//	        (the FLIPPER) scores, not p1; p1's exposed A1 is cleared
func TestConformance2018_OpenCell_FiveConsecutiveOpensNoAlternation(t *testing.T) {
	g := newD3Game(t)
	const p1, p2 PlayerID = 1, 2
	a2 := cellD3('A', 2) // value 5
	a3 := cellD3('A', 3) // value 4
	b2 := cellD3('B', 2) // value 6
	c2 := cellD3('C', 2) // value 6
	a1 := cellD3('A', 1) // value 1
	d3 := cellD3('D', 3) // value 1 -- A1's twin

	// step 1: p1 opens A2 -- first pending, no match possible yet.
	outcome, err := Flip(&g, nil, p1, a2)
	if err != nil {
		t.Fatalf("step1 Flip(p1, A2): %v", err)
	}
	if outcome.Matched {
		t.Fatalf("step1 outcome = %+v, want not Matched -- old: MatchedItems=\"\" MatchedCount=0", outcome)
	}
	if p, _ := g.Player(p1); p.Pending != a2 {
		t.Fatalf("step1 p1.Pending = %d, want %d (A2) -- old: Open1=\"A2\"", p.Pending, a2)
	}

	// step 2: p1 opens A3 -- mismatch (value 4 != value 5). The rewrite's
	// no-alternation rule REPLACES p1's pending with A3 outright (see
	// moves.go's Flip doc comment) rather than clearing it to "none" --
	// this is exactly what makes step 3's immediate third p1 open legal.
	outcome, err = Flip(&g, nil, p1, a3)
	if err != nil {
		t.Fatalf("step2 Flip(p1, A3): %v", err)
	}
	if outcome.Matched {
		t.Fatalf("step2 outcome = %+v, want not Matched -- old: no MatchedCount change", outcome)
	}
	if p, _ := g.Player(p1); p.Pending != a3 {
		t.Fatalf("step2 p1.Pending = %d, want %d (A3, replacing A2) -- old: Open1=\"A3\"", p.Pending, a3)
	}
	if human, robot := g.Players[0].Score, g.Players[1].Score; human != 0 || robot != 0 {
		t.Fatalf("step2 Score = (%d,%d), want (0,0): a mismatch credits nobody -- old: both players' MatchedCount stayed 0", human, robot)
	}

	// step 3: p1 opens B2 -- a THIRD consecutive p1 open, no p2 move in
	// between. Not expressible under the pre-ruling rewrite (it would have
	// been Robot's turn); trivially legal now.
	outcome, err = Flip(&g, nil, p1, b2)
	if err != nil {
		t.Fatalf("step3 Flip(p1, B2): %v", err)
	}
	if outcome.Matched {
		t.Fatalf("step3 outcome = %+v, want not Matched", outcome)
	}
	if p, _ := g.Player(p1); p.Pending != b2 {
		t.Fatalf("step3 p1.Pending = %d, want %d (B2) -- old: Open1=\"B2\"", p.Pending, b2)
	}

	// step 4: p1 opens C2 -- matches B2 (both value 6). p1 scores, keeps
	// no special "extra turn" status (there is no turn to keep), pending
	// clears.
	outcome, err = Flip(&g, nil, p1, c2)
	if err != nil {
		t.Fatalf("step4 Flip(p1, C2): %v", err)
	}
	if !outcome.Matched || outcome.MatchedBy != p1 {
		t.Fatalf("step4 outcome = %+v, want Matched by p1 -- old: MatchedItems=\"6\" MatchedCount=1", outcome)
	}
	if owner := g.PairOwner[g.Faces[b2]]; owner != p1 {
		t.Fatalf("step4 PairOwner[value-6 pair] = %v, want p1", owner)
	}
	if p, _ := g.Player(p1); p.Pending != -1 {
		t.Fatalf("step4 p1.Pending = %d after the match, want -1", p.Pending)
	}
	if p, _ := g.Player(p1); p.Score != 1 {
		t.Fatalf("step4 p1.Score = %d, want 1 -- old: MatchedCount=1", p.Score)
	}

	// step 5: p1 opens A1 -- a fresh pending (Pending was -1 after the
	// step 4 match, so this is a plain set, not a replace, but the
	// observable result is identical either way).
	outcome, err = Flip(&g, nil, p1, a1)
	if err != nil {
		t.Fatalf("step5 Flip(p1, A1): %v", err)
	}
	if outcome.Matched {
		t.Fatalf("step5 outcome = %+v, want not Matched (a fresh pending, no new match)", outcome)
	}
	if p, _ := g.Player(p1); p.Pending != a1 {
		t.Fatalf("step5 p1.Pending = %d, want %d (A1) -- old: Open1=\"A1\" Open2=\"\"", p.Pending, a1)
	}
	if p, _ := g.Player(p1); p.Score != 1 {
		t.Fatalf("step5 p1.Score = %d after the 5th open, want still 1 -- old: MatchedItems/MatchedCount unchanged (\"6\"/1)", p.Score)
	}

	// step 6: p2 opens D3 -- A1's twin (both value 1). Founder-ruled: the
	// FLIPPER (p2) is credited, not p1, whose exposed A1 sniped. p1's
	// pending is cleared as part of resolving the match (see Flip's doc
	// comment on clearing every player pointing at either matched cell).
	outcome, err = Flip(&g, nil, p2, d3)
	if err != nil {
		t.Fatalf("step6 Flip(p2, D3): %v", err)
	}
	if !outcome.Matched || outcome.MatchedBy != p2 {
		t.Fatalf("step6 outcome = %+v, want Matched by p2 -- founder-ruled: the flipper scores a sniped pair, not the exposer", outcome)
	}
	if owner := g.PairOwner[g.Faces[a1]]; owner != p2 {
		t.Fatalf("step6 PairOwner[value-1 pair] = %v, want p2", owner)
	}
	if p, _ := g.Player(p1); p.Pending != -1 {
		t.Fatalf("step6 p1.Pending = %d after being sniped, want -1", p.Pending)
	}
	if p1v, _ := g.Player(p1); p1v.Score != 1 {
		t.Fatalf("step6 p1.Score = %d, want still 1 -- p1 never completed their own exposed pick", p1v.Score)
	}
	if p2v, _ := g.Player(p2); p2v.Score != 1 {
		t.Fatalf("step6 p2.Score = %d, want 1 -- the flipper is credited", p2v.Score)
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
	g, err := NewGame(LayoutInline, 2, 0, nil, []PlayerSetup{{}, {}})
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	g.Faces = []uint8{0, 1, 0, 2, 3, 4, 5, 1, 2, 4, 5, 3}
	a1 := cellD3('A', 1)

	outcome, err := Flip(&g, nil, 1, a1)
	if err != nil {
		t.Fatalf("Flip(p1, A1): %v", err)
	}
	if outcome.Matched {
		t.Fatalf("outcome = %+v, want not Matched -- old: MatchedItems=\"\" MatchedCount=0", outcome)
	}
	if p, _ := g.Player(1); p.Pending != a1 {
		t.Fatalf("p1.Pending = %d, want %d -- old: Open1=\"A1\"", p.Pending, a1)
	}
	if human, robot := g.Players[0].Score, g.Players[1].Score; human != 0 || robot != 0 {
		t.Fatalf("Score = (%d,%d), want (0,0)", human, robot)
	}
}
