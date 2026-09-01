package pairgame

// PlayerID identifies a participant within one game. NoPlayer (0) means
// "nobody" — an unmatched pair, or the acting side wherever a PlayerID field
// can legitimately be absent. Real players are numbered 1..N in seating
// order (join order for vs-humans, creation order for solo/vs-bot); a
// PlayerID is only ever meaningful relative to the GameState.Players slice
// that assigned it.
//
// There is deliberately no "whose turn is it" concept anywhere in this
// package — see Flip's doc comment. PlayerID exists to attribute a Pending
// pick, a PairOwner entry, and a Reveal log entry to a specific participant,
// not to gate who may act next.
type PlayerID uint8

// NoPlayer is the zero PlayerID: an unmatched pair (PairOwner's default), or
// "nobody" wherever a PlayerID field can be absent.
const NoPlayer PlayerID = 0

// MaxPlayers is the founder-specified ceiling for vs-humans (2..8 humans,
// per the mode brief). It is a game-design limit, not a wire-format one —
// PlayerID (uint8) could address up to 255 without changing type — so
// NewGame enforces it directly rather than deriving it from a field width.
const MaxPlayers = 8

// Player is one participant in a game: a human or a bot, each tracking
// their own independent pending pick and score. Under the N-player rules
// there is no shared/global pending cell and no turn order — every player's
// Pending is advanced only by their own Flip calls (see Flip).
type Player struct {
	// ID is this player's seat number, 1..N within the owning GameState.
	ID PlayerID
	// IsBot is true for a bot seat. A bot is a player like any other under
	// exactly these same rules — see robot.go's Strategy, which the session
	// layer drives one Flip call at a time.
	IsBot bool
	// Pending is this player's own currently-open first pick this "round"
	// (their own, not shared — see the package doc), or -1 if none.
	Pending int
	// Score is how many pairs this player has personally claimed.
	Score int
	// Memory is a bot's difficulty dial: how many of the most recent public
	// Log entries (see GameState.Log) it may consult when choosing a move —
	// see robot.go's MemoryStrategy. 0 for a human player (ignored) and for
	// a "remembers nothing" bot (RandomMover-equivalent difficulty).
	Memory int
}

// PlayerSetup describes one seat to fill when starting a new game (see
// NewGame). NewGame assigns PlayerIDs 1..len(setup) in slice order and
// initializes each Player's Pending to "none" (-1) and Score to 0 — callers
// supply only what varies per seat.
type PlayerSetup struct {
	// IsBot seats a bot at this position.
	IsBot bool
	// Memory is the bot's difficulty dial (see Player.Memory). Ignored
	// (forced to 0) for a human seat.
	Memory int
}
