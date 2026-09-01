package pairgame

// FlipOutcome describes what happened as a result of one Flip call, for a
// caller (the render/robot/session layer) to react to.
type FlipOutcome struct {
	// Matched is true if this flip completed a pair — i.e. it found some
	// seated player (possibly the flipper themselves) already holding a
	// pending pick on this pair's other cell. See Flip's doc comment: the
	// FLIPPER is always credited, never the player whose pending pick was
	// matched.
	Matched bool
	// MatchedBy is who was credited with the match; only meaningful when
	// Matched is true. It always equals the `by` player passed to Flip.
	MatchedBy PlayerID
	// GameOver is true if this flip completed the last remaining pair.
	GameOver bool
	// Reveal is the public log entry this call appended to GameState.Log.
	Reveal Reveal
}

// Flip applies one player's cell flip to the state, in place (g is mutated
// through the pointer — this is one deliberate deviation from the brief's
// suggested `func Flip(g *GameState, by PlayerID, cell int) (FlipOutcome,
// error)` signature: `secret` is threaded through too, since FacesWith
// needs it to resolve LayoutSeedDerived boards, exactly as the old
// Reveal(cell, secret) did; see this package's PR description for why
// dropping it silently would have been wrong rather than merely simpler).
//
// # The rules (founder-specified, confirmed twice — see this package's PR
// # description; apply identically to every mode and to a bot exactly as
// # to a human — see robot.go)
//
//   - There is NO turn order. Any seated player may flip at any moment; the
//     faster player has the advantage, and that is intended.
//   - A cell belonging to an already-matched pair can never be flipped by
//     anyone (ErrCellAlreadyMatched) — checked first, before any pending
//     comparison.
//   - A player may not flip the exact cell that is already THEIR OWN
//     pending pick (ErrCellIsPending). Two different players holding the
//     same cell pending at the same time remains explicitly allowed — that
//     is not this error, and flipping a cell that is someone ELSE's
//     pending is a completely ordinary flip, resolved by the rule below
//     like any other.
//   - The flipped cell's pair id is checked against the pending pick of
//     EVERY seated player, including the flipper's own (this is the
//     founder-confirmed rule, matching the archived 2018 original: see
//     conformance_2018_open_test.go's step 6, and its header comment on
//     why the rewrite's earlier same-player-only version was wrong). If
//     any player holds a pending cell that (a) is not the cell just
//     flipped and (b) shares its pair id, the pair is matched — and
//     ALWAYS credited to the FLIPPER, never to the player whose pending
//     pick it was. Exposing a card is genuinely risky: anyone, including a
//     bot, may snipe it in a single flip. If more than one seated player
//     happens to hold a matching pending pick at once (only possible when
//     several players share a pending on the exact same cell — see
//     above), the search walks Players in ascending PlayerID order and the
//     first match found is used; ErrCellAlreadyMatched already guarantees
//     the pair is still unmatched at this point, so "a match was found"
//     and "the pair is claimable" are the same fact.
//   - On a match: the pair is marked owned by the flipper, who scores +1.
//     EVERY player whose pending pick pointed at either of this pair's two
//     cells — the one just flipped, and the partner cell that was
//     pending, including the flipper's own pending if they had one — is
//     cleared to "none". This is a deliberate generalization beyond just
//     clearing the single matched holder: it is what keeps the invariant
//     "no Player.Pending ever references an already-matched cell" true
//     for every seated player, not only the one credited, in the case
//     where a cell's pending was shared by more than one player.
//   - On NO match: the flipped cell becomes the flipper's new pending,
//     REPLACING whatever they had pending before (there is no separate
//     "first pick" vs "second pick" state any more — every flip is this
//     same check-then-set operation, exactly matching the archived 2018
//     original's OpenCell, which let a player keep opening fresh cells
//     indefinitely with no alternation and no "must resolve before trying
//     again" constraint — see conformance_2018_open_test.go).
//   - Every successful flip — matched or not — is appended to the public
//     Log (see Reveal). This is the shared memory of the game: robot.go's
//     Strategy reads it, and a render layer replays it as messages so
//     human players can remember what was opened, since the board itself
//     only shows matched cells and each player's own currently-pending
//     cell (see the package doc).
func Flip(g *GameState, secret []byte, by PlayerID, cell int) (FlipOutcome, error) {
	if g.IsComplete() {
		return FlipOutcome{}, ErrGameOver
	}
	pi := g.playerIndex(by)
	if pi < 0 {
		return FlipOutcome{}, ErrUnknownPlayer
	}
	faces := g.FacesWith(secret)
	if cell < 0 || cell >= len(faces) {
		return FlipOutcome{}, ErrInvalidCell
	}
	pairID := faces[cell]
	if g.PairOwner[pairID] != NoPlayer {
		return FlipOutcome{}, ErrCellAlreadyMatched
	}
	p := &g.Players[pi]
	if cell == p.Pending {
		return FlipOutcome{}, ErrCellIsPending
	}

	// Find the lowest-PlayerID seated player (the flipper included) whose
	// pending pick is this pair's OTHER cell — see the doc comment above.
	holderIdx := -1
	for i := range g.Players {
		if g.Players[i].Pending >= 0 && g.Players[i].Pending != cell && faces[g.Players[i].Pending] == pairID {
			holderIdx = i
			break
		}
	}

	rev := Reveal{By: by, Cell: cell, PairID: pairID}

	if holderIdx < 0 {
		// No match: replace the flipper's own pending outright.
		p.Pending = cell
		g.Log = append(g.Log, rev)
		return FlipOutcome{Reveal: rev}, nil
	}

	// Match: the flipper scores, never the pending holder. Clear every
	// player's pending that pointed at either of this pair's two cells —
	// see the doc comment above on why this is not limited to just the
	// one matched holder.
	partner := g.Players[holderIdx].Pending
	g.PairOwner[pairID] = by
	p.Score++
	for i := range g.Players {
		if g.Players[i].Pending == cell || g.Players[i].Pending == partner {
			g.Players[i].Pending = -1
		}
	}

	rev.Matched = true
	g.Log = append(g.Log, rev)
	outcome := FlipOutcome{Matched: true, MatchedBy: by, Reveal: rev}
	if g.IsComplete() {
		outcome.GameOver = true
	}
	return outcome, nil
}
