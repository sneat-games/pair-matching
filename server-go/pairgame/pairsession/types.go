package pairsession

import "github.com/sneat-games/pair-matching/server-go/pairgame"

// PlayerRef identifies one human participant for CreateVsBotGame,
// CreateVsHumansGame, and JoinGame — the session-layer counterpart of
// greedgame.Player.
type PlayerRef struct {
	// UserID is the host's application-level user ID.
	UserID string
	// Name is a display name (see dal4pairgame.PlayerDbo.Name).
	Name string
	// ChatID is the player's private Telegram chat ID with the game's bot,
	// if already known (e.g. they joined via a DM deep link). Zero when
	// unknown yet (e.g. they joined a group game via a callback tap). Hosts
	// that are not Telegram-backed can simply always pass 0.
	ChatID int64
}

// CellView is one board cell's public-knowledge rendering: whether ANY
// public reveal-log entry has ever mentioned it (see the package doc on
// why that alone is the whole secrecy model for a stored game), and — if
// so — its pair id and match state.
type CellView struct {
	// Cell is this cell's index.
	Cell int
	// Revealed is true once at least one Reveal log entry names this cell
	// — from that moment its PairID is public knowledge to every player,
	// matched or not.
	Revealed bool
	// PairID is the pair id under this cell. Only meaningful when Revealed.
	PairID uint8
	// Matched is true once this cell's pair has been claimed.
	Matched bool
	// MatchedBy is who claimed it. Only meaningful when Matched.
	MatchedBy pairgame.PlayerID
}

// PlayerView is one seat's public state, for rendering a scoreboard.
type PlayerView struct {
	ID      pairgame.PlayerID
	UserID  string // empty for a bot seat
	Name    string
	IsBot   bool
	Score   int
	Pending int // -1 if this seat currently has no pending pick
	// ChatID is this player's private Telegram chat ID with the bot, if
	// known (0 = unknown) — see dal4pairgame.PlayerDbo.ChatID and
	// SetPlayerChatID. Always 0 for a bot seat.
	ChatID int64
	// MessageID is this player's own anchored private board message, if
	// any (private-invite vs-Humans only; 0 = not yet anchored) — see
	// dal4pairgame.PlayerDbo.MessageID and SetPlayerMessage. Always 0 for a
	// bot seat.
	MessageID int
}

// View is a read-only rendering of one stored game, returned by GetView.
// It is NOT viewer-dependent (see the package doc): every cell that has
// ever been flipped by anyone is equally public to every player, so there
// is nothing here to vary per caller.
type View struct {
	GameID    string
	SizeIndex uint8
	Cells     []CellView
	Players   []PlayerView
	// Log is the full public reveal history, in call order — what a
	// render layer replays as messages (see pairgame's package doc).
	Log      []pairgame.Reveal
	Complete bool
	// Winners are the PlayerIDs with the highest score — see
	// pairgame.GameState.Winners. More than one entry is a tie.
	Winners []pairgame.PlayerID
	// ChatID/MessageID anchor the ONE shared group status message, if this
	// game was started in a group chat (ChatID == 0 for a private-invite
	// game — there, each player's own anchor lives on their PlayerView
	// instead; see dal4pairgame.GameDbo.ChatID/MessageID, SetGroupMessage,
	// and SetPlayerMessage).
	ChatID    int64
	MessageID int
}
