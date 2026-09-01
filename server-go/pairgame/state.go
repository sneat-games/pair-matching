package pairgame

// GameState is the complete state of one Pair-Matching game under the
// founder-specified N-player rules (see Flip's doc comment for the rules
// themselves):
//
//   - Solo (exactly one human player, no bot) round-trips through Telegram
//     callback_data via Encode/Decode — see snapshot.go.
//   - vs-Bot (one human + one bot) and vs-Humans (2..8 humans) are STORED
//     server-side (see server-go/pairgame/dal4pairgame): a bot that moves on
//     its own timer, and moves arriving from more than one actor, cannot
//     round-trip through a single tapped button's callback_data the way
//     solo's single-human-driven state can.
//
// GameState itself does not know which of the two transports applies to it
// — that is a property of how many/what kind of Players it carries, which
// Encode checks (see ErrNotSoloGame).
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
	// PairOwner records, per pair id, who (if anyone) has matched it —
	// NoPlayer for a still-unmatched pair. len(PairOwner) ==
	// Sizes[SizeIndex].Pairs().
	PairOwner []PlayerID
	// Players are this game's seats, 1..len(Players) — see Player and
	// NewGame. There is no turn order: any player may Flip at any moment,
	// and each tracks their own Pending pick independently.
	Players []Player
	// Log is the append-only public reveal history: one Reveal entry per
	// successful Flip call, in call order. Every flip is public per the
	// founder's rules — this IS the shared memory of the game (what
	// robot.go's Strategy reads, and what a render layer replays as
	// messages so human players can remember what was opened). Solo never
	// carries this across a callback_data round trip (see snapshot.go) —
	// a lone player already sees their own board directly and has no need
	// of a history feed.
	Log []Reveal
}

// Reveal is one public log entry: an appended, never-mutated record of one
// Flip call's result.
type Reveal struct {
	// By is the player who performed this flip.
	By PlayerID
	// Cell is the cell index that was flipped.
	Cell int
	// PairID is the pair id under that cell — public the moment it is
	// flipped, matched or not (per the founder's rules: every flip is
	// public knowledge, which is what makes sniping an opponent's exposed
	// pair possible).
	PairID uint8
	// Matched is true if this flip completed a pair (it was the acting
	// player's own second pick and it matched their pending first pick).
	Matched bool
}

// NewGame starts a fresh game: an empty board of matches, every seat's
// Pending set to "none", and a freshly generated layout.
//
// setup describes each seat to fill; NewGame assigns PlayerIDs 1..len(setup)
// in slice order (see PlayerSetup). It is mode-agnostic — solo (a single
// human setup), vs-bot (one human + one bot setup), and vs-humans (2..8
// human setups) all go through this one constructor; which shape a caller
// builds is a session-layer/caller decision, not something NewGame itself
// enforces beyond the universal 1..MaxPlayers bound.
//
// Under LayoutInline, `seed` deterministically generates the explicit layout
// via ShuffleFaces (secret is ignored). Under LayoutSeedDerived, `seed` is
// stored on the wire/record and the layout is derived on demand via
// DeriveFaces using `secret` — NewGame does not otherwise use or store it.
func NewGame(mode LayoutMode, sizeIndex int, seed uint32, secret []byte, setup []PlayerSetup) (GameState, error) {
	if sizeIndex < 0 || sizeIndex >= len(Sizes) {
		return GameState{}, ErrInvalidSizeIndex
	}
	if len(setup) < 1 || len(setup) > MaxPlayers {
		return GameState{}, ErrInvalidPlayerCount
	}
	players := make([]Player, len(setup))
	for i, s := range setup {
		if s.Memory < 0 {
			return GameState{}, ErrInvalidMemory
		}
		memory := s.Memory
		if !s.IsBot {
			memory = 0 // a human has no memory dial; never trust a caller-supplied value for it
		}
		players[i] = Player{
			ID:      PlayerID(i + 1),
			IsBot:   s.IsBot,
			Pending: -1,
			Memory:  memory,
		}
	}

	g := GameState{
		SizeIndex: uint8(sizeIndex),
		Mode:      mode,
		Seed:      seed,
		PairOwner: make([]PlayerID, Sizes[sizeIndex].Pairs()),
		Players:   players,
	}
	if mode == LayoutInline {
		g.Faces = ShuffleFaces(seed, Sizes[sizeIndex].Pairs())
	} else {
		_ = secret // not needed until FacesWith/Flip actually recompute the layout
	}
	return g, nil
}

// NewSoloGame starts a fresh Solo game: a single human player (PlayerID 1),
// no bot, ready to round-trip through Encode/Decode (see snapshot.go).
// It additionally rejects a (mode, sizeIndex) whose solo snapshot would not
// fit the callback-data budget (ErrSoloBoardTooLarge) — the caller should
// steer players away from that combination rather than let them start a
// game that could never be encoded back into a callback button in the
// first place. LayoutInline does not fit at Sizes' largest preset (8x8);
// LayoutSeedDerived fits every preset (see snapshot_test.go's measured
// table) and is the mode a real host should prefer for exactly that reason,
// same as the pre-rewrite engine recommended.
func NewSoloGame(mode LayoutMode, sizeIndex int, seed uint32, secret []byte) (GameState, error) {
	if sizeIndex < 0 || sizeIndex >= len(Sizes) {
		return GameState{}, ErrInvalidSizeIndex
	}
	if !Fits(mode, sizeIndex) {
		return GameState{}, ErrSoloBoardTooLarge
	}
	return NewGame(mode, sizeIndex, seed, secret, []PlayerSetup{{}})
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
		if o == NoPlayer {
			return false
		}
	}
	return true
}

// Winners returns the PlayerIDs with the highest Score, in ascending ID
// order. More than one entry represents a tie — ties are a normal, fully
// representable outcome under these rules, not an edge case to special-case
// away. Meaningful once IsComplete is true, but well-defined at any point
// (an in-progress "who's currently ahead" read).
func (g GameState) Winners() []PlayerID {
	best := -1
	for _, p := range g.Players {
		if p.Score > best {
			best = p.Score
		}
	}
	var winners []PlayerID
	for _, p := range g.Players {
		if p.Score == best {
			winners = append(winners, p.ID)
		}
	}
	return winners
}

// Player returns the player with this ID and whether it exists in g.
func (g GameState) Player(id PlayerID) (Player, bool) {
	if i := g.playerIndex(id); i >= 0 {
		return g.Players[i], true
	}
	return Player{}, false
}

// playerIndex returns the slice index of the player with this ID, or -1.
func (g GameState) playerIndex(id PlayerID) int {
	for i, p := range g.Players {
		if p.ID == id {
			return i
		}
	}
	return -1
}

// LegalMoves returns the indices of cells `by` may currently flip: not
// already matched, and not `by`'s OWN pending pick. Another player's
// pending pick on the same cell does not block it — see Flip's doc comment
// on simultaneous pending picks. If `by` is not a seated player, only the
// "not already matched" filter applies (there is no pending pick to
// exclude).
func (g GameState) LegalMoves(faces []uint8, by PlayerID) []int {
	pending := -1
	if p, ok := g.Player(by); ok {
		pending = p.Pending
	}
	moves := make([]int, 0, len(faces))
	for cell, pairID := range faces {
		if g.PairOwner[pairID] == NoPlayer && cell != pending {
			moves = append(moves, cell)
		}
	}
	return moves
}
