// Package dal4pairgame is the dalgo persistence layer for the STORED
// Pair-Matching modes — vs-Bot (one human + one bot) and vs-Humans (2..8
// humans): the DBO (data-bearing-object) shape and the key builder +
// get/save helpers. Solo never persists a session at all — it round-trips
// entirely through Telegram callback_data (see server-go/pairgame's
// snapshot.go) — so there is no "solo" Status or Mode value here.
//
// This package holds no game rules — those live in the parent pairgame
// package (Flip, MemoryStrategy, ...), which a session-composing sibling
// package is the intended caller of, alongside this one. Deliberately, this
// package does not import pairgame at all (mirroring
// sneat-games/greed-game's dal4greedgame, which does not import greedgame
// either): the DBO's field types are plain built-ins, so persistence stays
// decoupled from the rules engine's own types and neither package needs the
// other to compile or test in isolation.
package dal4pairgame

import "time"

// ExtensionID namespaces Pair-Matching's stored-mode records under
// ext/pairmatching/... (a convention shared with other Sneat extensions:
// ext/<extID>/<collection>/<id>).
const ExtensionID = "pairmatching"

// GamesCollection holds one document per stored (vs-Bot or vs-Humans) game.
const GamesCollection = "games"

// Status is a GameDbo's lifecycle state.
type Status string

const (
	// StatusLobby: the game has been created and is accepting joins; no
	// board has been dealt yet. vs-Bot games skip this state entirely —
	// both seats (the host and the bot) are known at creation time, so
	// there is nothing to wait on; only vs-Humans games spend time here.
	StatusLobby Status = "lobby"

	// StatusActive: the board is dealt and players may flip cards.
	StatusActive Status = "active"

	// StatusFinished: every pair has been matched — a terminal state.
	StatusFinished Status = "finished"
)

// Mode is which of the two STORED founder-specified modes this game is.
// Solo is deliberately not a value here — it is never stored; see the
// package doc.
type Mode string

const (
	// ModeVsBot: one human + one bot, seated at creation time.
	ModeVsBot Mode = "vs_bot"
	// ModeVsHumans: 2..8 humans, seated via Join before StatusActive.
	ModeVsHumans Mode = "vs_humans"
)

// PlayerDbo is one seat in a stored game — a human (UserID set, IsBot
// false) or a bot (UserID empty, IsBot true). Field names/shapes mirror
// pairgame.Player (ID, IsBot, Pending, Score, Memory) plus the identity/
// display fields a stored, multi-actor game additionally needs (UserID,
// Name, ChatID) that pairgame.Player has no reason to carry — that package
// is pure rules and never touches an application-level identity.
type PlayerDbo struct {
	// ID is this seat's PlayerID (1..N, seating order) — see
	// pairgame.PlayerID. Stored as a plain uint8 so this package does not
	// need to import pairgame for its own DBO type (see the package doc).
	ID uint8 `json:"id" firestore:"id"`

	// UserID is the host's application-level user ID. Empty for a bot seat.
	UserID string `json:"userID,omitempty" firestore:"userID,omitempty"`

	// IsBot is true for a bot seat.
	IsBot bool `json:"isBot,omitempty" firestore:"isBot,omitempty"`

	// Name is a display name captured at join time, used to render status
	// messages and the public reveal log without a live user lookup. Empty
	// for a bot (the render layer supplies its own bot display name).
	Name string `json:"name,omitempty" firestore:"name,omitempty"`

	// ChatID is a human player's private Telegram chat ID with the game's
	// bot, captured opportunistically the first time they interact with the
	// bot in a DM. Zero means the bot cannot yet DM this player directly.
	// Always zero for a bot seat.
	ChatID int64 `json:"chatID,omitempty" firestore:"chatID,omitempty"`

	// MessageID is this player's own anchored private board message, edited
	// in place as the game progresses — the private-invite vs-Humans
	// counterpart of GameDbo.MessageID, since there each player has their
	// OWN board message rather than one shared group message. Zero means
	// this player has not yet had a board message posted for them. Always
	// zero for a bot seat.
	MessageID int `json:"messageID,omitempty" firestore:"messageID,omitempty"`

	// Pending mirrors pairgame.Player.Pending: this seat's own currently
	// open first pick, or -1 for none.
	Pending int `json:"pending" firestore:"pending"`

	// Score mirrors pairgame.Player.Score: pairs this seat has personally
	// claimed.
	Score int `json:"score,omitempty" firestore:"score,omitempty"`

	// Memory mirrors pairgame.Player.Memory: a bot's difficulty dial (how
	// many recent public Log entries it may consult). 0 for a human seat.
	Memory int `json:"memory,omitempty" firestore:"memory,omitempty"`
}

// RevealDbo is one persisted entry of the public reveal log — the stored
// mirror of pairgame.Reveal.
type RevealDbo struct {
	// By is the PlayerID (see PlayerDbo.ID) who performed this flip.
	By uint8 `json:"by" firestore:"by"`
	// Cell is the cell index that was flipped.
	Cell int `json:"cell" firestore:"cell"`
	// PairID is the pair id under that cell — public the moment it is
	// flipped, matched or not.
	PairID uint8 `json:"pairID" firestore:"pairID"`
	// Matched is true if this flip completed a pair.
	Matched bool `json:"matched,omitempty" firestore:"matched,omitempty"`
}

// GameDbo is the persisted state of one stored (vs-Bot or vs-Humans) game:
// its board, its seats, and its public reveal history. Field names mirror
// pairgame.GameState (SizeIndex, Mode [LayoutMode], Seed, Faces, PairOwner,
// Players, Log) plus the session/lifecycle metadata a stored game
// additionally needs that the pure rules engine has no reason to carry
// (HostUserID, Mode [dal4pairgame.Mode — which of the two stored kinds this
// is, NOT pairgame.LayoutMode], ChatID, MessageID, Status, CreatedAt).
//
// Faces is always stored explicitly (the persistence-layer equivalent of
// pairgame.LayoutInline): a stored game has no need for
// pairgame.LayoutSeedDerived's HMAC-derived secrecy trick, since the server
// already fully controls what a view exposes to which viewer (see the
// session-composing sibling package's view helpers) — the anti-tamper
// property that trick buys a client-held callback_data snapshot is simply
// not a concern for a record only the server ever reads or writes.
type GameDbo struct {
	// HostUserID is the player who created the game (first to join).
	HostUserID string `json:"hostUserID" firestore:"hostUserID"`

	// Mode is which of the two stored kinds this game is (vs-Bot or
	// vs-Humans) — see the Mode type. Not to be confused with the board
	// layout mode pairgame.GameState.Mode selects between; a stored game's
	// board layout mode is always LayoutInline (see the struct doc).
	Mode Mode `json:"mode" firestore:"mode"`

	// ChatID is the Telegram group chat this game is anchored to, or 0 for
	// a private-invite game (host + friends, coordinated entirely by DM).
	ChatID int64 `json:"chatID,omitempty" firestore:"chatID,omitempty"`

	// MessageID is the group's single dedicated status message, edited in
	// place as players join/flip/finish. Meaningless when ChatID is 0.
	MessageID int `json:"messageID,omitempty" firestore:"messageID,omitempty"`

	// SizeIndex selects the board preset — see pairgame.Sizes.
	SizeIndex uint8 `json:"sizeIndex" firestore:"sizeIndex"`

	// Faces is the explicit per-cell face/pair-id layout, dealt once at
	// StatusActive and never re-dealt — see the struct doc on why a stored
	// game always uses the LayoutInline shape.
	Faces []uint8 `json:"faces,omitempty" firestore:"faces,omitempty"`

	// PairOwner records, per pair id, which seat (PlayerDbo.ID, 0 = nobody)
	// has matched it.
	PairOwner []uint8 `json:"pairOwner,omitempty" firestore:"pairOwner,omitempty"`

	// Players are this game's seats, in seating order.
	Players []PlayerDbo `json:"players" firestore:"players"`

	// Log is the append-only public reveal history — see RevealDbo.
	Log []RevealDbo `json:"log,omitempty" firestore:"log,omitempty"`

	// Status is the game's lifecycle state.
	Status Status `json:"status" firestore:"status"`

	// CreatedAt records game creation for housekeeping/ordering.
	CreatedAt time.Time `json:"createdAt" firestore:"createdAt"`
}

// Validate satisfies dalgo's optional ValidatableRecord hook, so a
// structurally broken game can never be persisted. It holds no game RULE
// (nothing about Flip resolution, scoring, or the reveal log lives here —
// see the package doc) — only the structural invariants a document must
// satisfy to be a well-formed record at all.
func (v *GameDbo) Validate() error {
	if v.HostUserID == "" {
		return errMissingField("hostUserID")
	}
	switch v.Mode {
	case ModeVsBot, ModeVsHumans:
	default:
		return errInvalidMode(v.Mode)
	}
	if len(v.Players) == 0 {
		return errMissingField("players")
	}
	seenIDs := make(map[uint8]struct{}, len(v.Players))
	seenUsers := make(map[string]struct{}, len(v.Players))
	bots := 0
	for i, p := range v.Players {
		if p.ID == 0 {
			return errIndexedField("players", i, "id")
		}
		if _, dup := seenIDs[p.ID]; dup {
			return errDuplicatePlayerID(p.ID)
		}
		seenIDs[p.ID] = struct{}{}
		if p.IsBot {
			bots++
			continue
		}
		if p.UserID == "" {
			return errIndexedField("players", i, "userID")
		}
		if _, dup := seenUsers[p.UserID]; dup {
			return errDuplicatePlayer(p.UserID)
		}
		seenUsers[p.UserID] = struct{}{}
	}
	if v.Mode == ModeVsBot && bots != 1 {
		return errWrongBotCount(v.Mode, bots)
	}
	if v.Mode == ModeVsHumans && bots != 0 {
		return errWrongBotCount(v.Mode, bots)
	}
	switch v.Status {
	case StatusLobby, StatusActive, StatusFinished:
	default:
		return errInvalidStatus(v.Status)
	}
	return nil
}
