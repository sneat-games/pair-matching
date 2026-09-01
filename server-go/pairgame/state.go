package pairgame

// GameState is the complete, self-contained state of one Pair-Matching game —
// everything Encode needs to round-trip it through Telegram callback data,
// and everything Reveal needs to apply a move. There is no server-side game
// storage: per REQ:state-in-callback-data, a decoded GameState plus the
// board's Faces (see FacesWith) is the entire game.
type GameState struct {
	// SizeIndex selects the board preset from Sizes.
	SizeIndex uint8
	// Mode selects how the layout is carried — see LayoutMode's doc comment.
	Mode LayoutMode
	// Seed is the public layout seed. Meaningful only when Mode ==
	// LayoutSeedDerived; ignored (and left zero) under LayoutInline, where
	// the layout is instead carried explicitly in Faces.
	Seed uint32
	// Faces is the explicit per-cell face/pair-id layout. Populated only
	// under LayoutInline; nil under LayoutSeedDerived, where it is instead
	// recomputed on demand — see FacesWith.
	Faces []uint8
	// PairOwner records, per pair id, who (if anyone) has matched it.
	// len(PairOwner) == Sizes[SizeIndex].Pairs().
	PairOwner []Owner
	// Pending is the cell index of the current turn's first pick this
	// round, or -1 if no pick is pending (the next Reveal will be a first
	// pick).
	Pending int
	// Turn is whose move it is: Human or Robot.
	Turn Owner
	// N is the robot's memory capacity — the difficulty dial. 0 means the
	// robot remembers nothing (a uniformly random mover); larger N lets it
	// recall more recently-seen cells, up to Sizes[SizeIndex].Cells() (an
	// unbeatable perfect-memory robot). See robot.go.
	N int
	// Memory is a FIFO of recently-revealed, still-unresolved cell indices
	// — the most recently revealed cell last — capped at N entries. It
	// stores indices, never face values: the face at any index is always
	// derivable from FacesWith, so remembering "cell 7" costs
	// ceil(log2(cells)) bits regardless of how many distinct faces the
	// board has. See Reveal for how it is maintained.
	Memory []int
}

// NewGame starts a fresh game: an empty board of matches, no pending pick,
// `first` to move, and a freshly generated layout.
//
// Under LayoutInline, `seed` deterministically generates the explicit layout
// via ShuffleFaces (secret is ignored). Under LayoutSeedDerived, `seed` is
// stored on the wire and the layout is derived on demand via DeriveFaces
// using `secret` (a bot-side value from host configuration — see
// DeriveFaces's doc comment; NewGame does not otherwise use or store it).
//
// NewGame rejects an `n` (robot memory capacity/difficulty) that would make
// a full-memory snapshot exceed the callback-data budget at this
// mode/board — see MaxDifficulty — returning ErrInvalidDifficulty rather
// than silently producing a game that could later grow too large mid-play.
func NewGame(mode LayoutMode, sizeIndex int, seed uint32, secret []byte, n int, first Owner) (GameState, error) {
	if sizeIndex < 0 || sizeIndex >= len(Sizes) {
		return GameState{}, ErrInvalidCell
	}
	if first != Human && first != Robot {
		panic("pairgame: NewGame's first mover must be Human or Robot")
	}
	if max := MaxDifficulty(mode, sizeIndex); n < 0 || n > max {
		return GameState{}, ErrInvalidDifficulty
	}

	g := GameState{
		SizeIndex: uint8(sizeIndex),
		Mode:      mode,
		Seed:      seed,
		PairOwner: make([]Owner, Sizes[sizeIndex].Pairs()),
		Pending:   -1,
		Turn:      first,
		N:         n,
	}
	if mode == LayoutInline {
		g.Faces = ShuffleFaces(seed, Sizes[sizeIndex].Pairs())
	} else {
		_ = secret // not needed until FacesWith/Reveal actually recompute the layout
	}
	return g, nil
}

// FacesWith returns the per-cell face/pair-id layout: Faces directly under
// LayoutInline, or DeriveFaces(secret, ...) under LayoutSeedDerived. secret
// is ignored under LayoutInline and may be nil there.
func (g GameState) FacesWith(secret []byte) []uint8 {
	if g.Mode == LayoutInline {
		return g.Faces
	}
	return DeriveFaces(secret, snapshotVersion, g.SizeIndex, g.Seed)
}

// IsComplete reports whether every pair has been matched.
func (g GameState) IsComplete() bool {
	for _, o := range g.PairOwner {
		if o == Unmatched {
			return false
		}
	}
	return true
}

// Score returns how many pairs each side has matched.
func (g GameState) Score() (human, robot int) {
	for _, o := range g.PairOwner {
		switch o {
		case Human:
			human++
		case Robot:
			robot++
		}
	}
	return
}

// LegalMoves returns the indices of cells that can currently be revealed:
// not already matched, and not the pending pick itself.
func (g GameState) LegalMoves(faces []uint8) []int {
	moves := make([]int, 0, len(faces))
	for cell, pairID := range faces {
		if g.PairOwner[pairID] == Unmatched && cell != g.Pending {
			moves = append(moves, cell)
		}
	}
	return moves
}
