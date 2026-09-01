package pairgame

// FlipOutcome describes what happened as a result of one Flip call, for a
// caller (the render/robot/session layer) to react to.
type FlipOutcome struct {
	// Matched is true if this flip completed a pair (it was the acting
	// player's second pick and it matched their pending first pick).
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
// through the pointer — this is the one deliberate deviation from the
// brief's suggested `func Flip(g *GameState, by PlayerID, cell int)
// (FlipOutcome, error)` signature: `secret` is threaded through too, since
// FacesWith needs it to resolve LayoutSeedDerived boards, exactly as the
// old Reveal(cell, secret) did; see this package's PR description for why
// dropping it silently would have been wrong rather than merely simpler).
//
// # The rules (founder-specified; apply identically to every mode and to a
// # bot exactly as to a human — see robot.go)
//
//   - There is NO turn order. Any seated player may flip at any moment; the
//     faster player has the advantage, and that is intended.
//   - Each player has their OWN independent pending pick (Player.Pending).
//     Two different players may hold the same cell pending at the same
//     time — that is explicitly allowed; only a player flipping their OWN
//     already-pending cell again is rejected (ErrCellIsPending).
//   - A cell belonging to an already-matched pair can never be flipped by
//     anyone, first pick or second (ErrCellAlreadyMatched) — this is also
//     what makes a stale pending pick self-correcting: once a pair is
//     fully matched, BOTH of its cells become unflippable, so a player who
//     was still holding one of them pending can never re-complete it; their
//     very next flip (necessarily some other, still-legal cell) simply
//     fails to match against their now-stale pending face, and Pending
//     clears below exactly like any other mismatch. No separate staleness
//     check is needed — see the second-pick branch below.
//   - If the acting player has no pending pick: record `cell` as their
//     pending. Nothing else resolves yet.
//   - If the acting player has a pending pick: `cell` must differ from it
//     (ErrCellIsPending otherwise). If the two cells share a pair id — and
//     ErrCellAlreadyMatched above already guarantees that pair is still
//     unmatched — the player claims the pair and scores +1. Otherwise
//     nothing is claimed. Either way the player's pending clears.
//   - ANY player may claim ANY pair, including one whose cards were
//     revealed by an opponent (or by the acting player revealing both
//     halves of a pair themselves, first pick then second, same as
//     always). Sniping an opponent's exposed pair is a legitimate,
//     intended move, not an edge case to guard against.
//   - Every successful flip — first pick or second, matched or not — is
//     appended to the public Log (see Reveal). This is the shared memory
//     of the game: robot.go's Strategy reads it, and a render layer
//     replays it as messages so human players can remember what was
//     opened, since the board itself only shows matched cells and each
//     player's own currently-pending cell (see the package doc).
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

	if p.Pending < 0 {
		// First pick: nothing resolves yet.
		p.Pending = cell
		rev := Reveal{By: by, Cell: cell, PairID: pairID, Matched: false}
		g.Log = append(g.Log, rev)
		return FlipOutcome{Reveal: rev}, nil
	}

	// Second pick: resolve against this player's own pending first pick.
	// pendingPairID may be stale (its pair claimed by someone else since —
	// see the doc comment above); that self-corrects here without any
	// extra check, because a stale pendingPairID can never equal pairID:
	// if it did, cell and p.Pending would be the two cells of the SAME
	// still-unmatched pair (ErrCellAlreadyMatched above already proved
	// pairID's pair is unmatched), which is exactly the real-match case,
	// not a stale one.
	pendingPairID := faces[p.Pending]
	matched := pendingPairID == pairID
	p.Pending = -1

	rev := Reveal{By: by, Cell: cell, PairID: pairID, Matched: matched}
	g.Log = append(g.Log, rev)
	outcome := FlipOutcome{Reveal: rev}
	if matched {
		g.PairOwner[pairID] = by
		p.Score++
		outcome.Matched = true
		outcome.MatchedBy = by
		if g.IsComplete() {
			outcome.GameOver = true
		}
	}
	return outcome, nil
}
