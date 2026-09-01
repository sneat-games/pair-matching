package pairsession

import (
	"context"
	"fmt"
	"time"

	"github.com/dal-go/dalgo/dal"

	"github.com/sneat-games/pair-matching/server-go/pairgame"
	"github.com/sneat-games/pair-matching/server-go/pairgame/dal4pairgame"
)

// CreateVsBotGame creates a new vs-Bot game: host + one bot, both seats
// known up front, so the game is dealt (see the package doc on why always
// LayoutInline) and StatusActive immediately — there is no lobby to wait
// on. botMemory is the bot's difficulty dial (see pairgame.Player.Memory);
// 0 is a legal "remembers nothing" bot.
func CreateVsBotGame(ctx context.Context, db dal.DB, sizeIndex int, host PlayerRef, botMemory int, chatID int64) (gameID string, err error) {
	if host.UserID == "" {
		return "", fmt.Errorf("pairsession: CreateVsBotGame: host.UserID is required")
	}
	setup := []pairgame.PlayerSetup{{}, {IsBot: true, Memory: botMemory}}
	g, err := pairgame.NewGame(pairgame.LayoutInline, sizeIndex, randomSeed(), nil, setup)
	if err != nil {
		return "", fmt.Errorf("pairsession: CreateVsBotGame: %w", err)
	}

	gameID = NewGameID()
	entry := dal4pairgame.NewGameEntry(gameID)
	*entry.Data = dal4pairgame.GameDbo{
		HostUserID: host.UserID,
		Mode:       dal4pairgame.ModeVsBot,
		ChatID:     chatID,
		SizeIndex:  g.SizeIndex,
		Faces:      g.Faces,
		PairOwner:  make([]uint8, len(g.PairOwner)),
		Players: []dal4pairgame.PlayerDbo{
			{ID: 1, UserID: host.UserID, Name: host.Name, ChatID: host.ChatID, Pending: -1},
			{ID: 2, IsBot: true, Pending: -1, Memory: botMemory},
		},
		Status:    dal4pairgame.StatusActive,
		CreatedAt: time.Now(),
	}
	if err = dal4pairgame.SaveGame(ctx, db, entry); err != nil {
		return "", fmt.Errorf("pairsession: CreateVsBotGame: save game %s: %w", gameID, err)
	}
	return gameID, nil
}

// CreateVsHumansGame creates a new vs-Humans game in its Lobby with host as
// its sole seated player. Other players join via JoinGame until the host
// (or whoever the caller's UI lets) calls StartGame, which deals the board
// once 2..MaxPlayers have joined.
func CreateVsHumansGame(ctx context.Context, db dal.DB, sizeIndex int, host PlayerRef, chatID int64) (gameID string, err error) {
	if host.UserID == "" {
		return "", fmt.Errorf("pairsession: CreateVsHumansGame: host.UserID is required")
	}
	if sizeIndex < 0 || sizeIndex >= len(pairgame.Sizes) {
		return "", pairgame.ErrInvalidSizeIndex
	}

	gameID = NewGameID()
	entry := dal4pairgame.NewGameEntry(gameID)
	*entry.Data = dal4pairgame.GameDbo{
		HostUserID: host.UserID,
		Mode:       dal4pairgame.ModeVsHumans,
		ChatID:     chatID,
		SizeIndex:  uint8(sizeIndex),
		Players: []dal4pairgame.PlayerDbo{
			{ID: 1, UserID: host.UserID, Name: host.Name, ChatID: host.ChatID, Pending: -1},
		},
		Status:    dal4pairgame.StatusLobby,
		CreatedAt: time.Now(),
	}
	if err = dal4pairgame.SaveGame(ctx, db, entry); err != nil {
		return "", fmt.Errorf("pairsession: CreateVsHumansGame: save game %s: %w", gameID, err)
	}
	return gameID, nil
}

// JoinGame adds player to gameID's vs-Humans lobby (ErrWrongModeForJoin
// against a vs-Bot game — its seats are fixed at creation). Re-joining
// (e.g. tapping an invite link twice) is a harmless no-op that still
// opportunistically captures a freshly-seen ChatID, mirroring greedgame's
// Join.
func JoinGame(ctx context.Context, db dal.DB, gameID string, player PlayerRef) error {
	if player.UserID == "" {
		return fmt.Errorf("pairsession: JoinGame: player.UserID is required")
	}
	return db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		entry, err := dal4pairgame.GetGameTx(ctx, tx, gameID)
		if err != nil {
			return mapNotFound(err)
		}
		d := entry.Data
		if d.Mode != dal4pairgame.ModeVsHumans {
			return ErrWrongModeForJoin
		}
		for i, p := range d.Players {
			if p.UserID == player.UserID {
				if player.ChatID != 0 {
					d.Players[i].ChatID = player.ChatID
				}
				return tx.Set(ctx, entry.Record)
			}
		}
		if d.Status != dal4pairgame.StatusLobby {
			return ErrGameNotInLobby
		}
		if len(d.Players) >= pairgame.MaxPlayers {
			return ErrTooManyPlayers
		}
		d.Players = append(d.Players, dal4pairgame.PlayerDbo{
			ID:      uint8(len(d.Players) + 1),
			UserID:  player.UserID,
			Name:    player.Name,
			ChatID:  player.ChatID,
			Pending: -1,
		})
		return tx.Set(ctx, entry.Record)
	})
}

// StartGame deals the board and transitions a vs-Humans game from Lobby to
// Active. Requires at least two seated players (ErrNotEnoughPlayers
// otherwise); ErrWrongModeForStart against a vs-Bot game, which is dealt
// and Active immediately at creation.
func StartGame(ctx context.Context, db dal.DB, gameID string) error {
	return db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		entry, err := dal4pairgame.GetGameTx(ctx, tx, gameID)
		if err != nil {
			return mapNotFound(err)
		}
		d := entry.Data
		if d.Mode != dal4pairgame.ModeVsHumans {
			return ErrWrongModeForStart
		}
		if d.Status != dal4pairgame.StatusLobby {
			return ErrGameNotInLobby
		}
		if len(d.Players) < 2 {
			return ErrNotEnoughPlayers
		}
		setup := make([]pairgame.PlayerSetup, len(d.Players))
		g, err := pairgame.NewGame(pairgame.LayoutInline, int(d.SizeIndex), randomSeed(), nil, setup)
		if err != nil {
			return fmt.Errorf("pairsession: StartGame: %w", err)
		}
		d.Faces = g.Faces
		d.PairOwner = make([]uint8, len(g.PairOwner))
		d.Status = dal4pairgame.StatusActive
		return tx.Set(ctx, entry.Record)
	})
}

// Flip applies one human player's cell flip to gameID: loads the game,
// resolves userID to its seat, calls pairgame.Flip, and persists the
// result — all inside one read-write transaction, so two flips racing
// against the same game can never both read the pre-flip state.
func Flip(ctx context.Context, db dal.DB, gameID string, userID string, cell int) (outcome pairgame.FlipOutcome, err error) {
	txErr := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		entry, gErr := dal4pairgame.GetGameTx(ctx, tx, gameID)
		if gErr != nil {
			return mapNotFound(gErr)
		}
		d := entry.Data
		if d.Status != dal4pairgame.StatusActive {
			return ErrGameNotActive
		}
		by := findPlayerIDByUserID(d.Players, userID)
		if by == pairgame.NoPlayer {
			return ErrPlayerNotInGame
		}

		g := toGameState(d)
		var flipErr error
		outcome, flipErr = pairgame.Flip(&g, nil, by, cell)
		if flipErr != nil {
			return flipErr
		}
		writeBackPlayers(g, d)
		writeBackBoard(g, d)
		if g.IsComplete() {
			d.Status = dal4pairgame.StatusFinished
		}
		return tx.Set(ctx, entry.Record)
	})
	if txErr != nil {
		return pairgame.FlipOutcome{}, txErr
	}
	return outcome, nil
}

// RobotMove drives a vs-Bot game's bot seat for exactly ONE Flip call
// (ErrNoBotInGame against anything else) — the session-layer half of the
// founder's "the bot flips one card per invocation" rule; pairgame's own
// Strategy has no concept of invocation count, so enforcing "one card,
// then stop" is entirely this function's responsibility: it is called
// once, it flips once, it returns. A caller driving a bot on a timer calls
// this once per tick.
//
// The bot's move is chosen by pairgame.MemoryStrategy, reading the STORED
// game's own public Log — exactly the same information a real player
// watching the chat would have; see pairgame's robot.go doc comment on why
// a bot gets no special access.
func RobotMove(ctx context.Context, db dal.DB, gameID string) (outcome pairgame.FlipOutcome, err error) {
	txErr := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		entry, gErr := dal4pairgame.GetGameTx(ctx, tx, gameID)
		if gErr != nil {
			return mapNotFound(gErr)
		}
		d := entry.Data
		if d.Mode != dal4pairgame.ModeVsBot {
			return ErrNoBotInGame
		}
		if d.Status != dal4pairgame.StatusActive {
			return ErrGameNotActive
		}

		g := toGameState(d)
		botID := pairgame.NoPlayer
		for _, p := range g.Players {
			if p.IsBot {
				botID = p.ID
				break
			}
		}
		if botID == pairgame.NoPlayer {
			return ErrNoBotInGame
		}

		strat := pairgame.MemoryStrategy{Fallback: pairgame.RandomMover{Rand: newMoveRand()}}
		cell := strat.Choose(g, g.Faces, botID)
		var flipErr error
		outcome, flipErr = pairgame.Flip(&g, nil, botID, cell)
		if flipErr != nil {
			return flipErr
		}
		writeBackPlayers(g, d)
		writeBackBoard(g, d)
		if g.IsComplete() {
			d.Status = dal4pairgame.StatusFinished
		}
		return tx.Set(ctx, entry.Record)
	})
	if txErr != nil {
		return pairgame.FlipOutcome{}, txErr
	}
	return outcome, nil
}

// SetGroupMessage records gameID's anchored group status message — the
// single message a group-chat game's host wiring edits in place as
// players join/flip/finish (see dal4pairgame.GameDbo.MessageID; the game
// must already carry the group's ChatID, set at creation time). Mirrors
// greedgame's SetGroupMessage. Idempotent: setting the same messageID
// again is a no-op success, not an error.
func SetGroupMessage(ctx context.Context, db dal.DB, gameID string, messageID int) error {
	return db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		entry, err := dal4pairgame.GetGameTx(ctx, tx, gameID)
		if err != nil {
			return mapNotFound(err)
		}
		if entry.Data.MessageID == messageID {
			return nil // already up to date; avoid a needless write
		}
		entry.Data.MessageID = messageID
		return tx.Set(ctx, entry.Record)
	})
}

// SetPlayerChatID opportunistically records userID's private Telegram chat
// ID with the game's bot (e.g. the first time they interact with the bot
// in a DM), so the bot can address them directly. Mirrors greedgame's
// SetPlayerChatID: a no-op if chatID is 0 (nothing learned yet) or if the
// stored value already matches. ErrPlayerNotInGame if userID is not a
// seated player; ErrPlayerIsBot if userID resolves to the game's bot seat
// (a bot seat never has a chat of its own — see
// dal4pairgame.PlayerDbo.ChatID).
func SetPlayerChatID(ctx context.Context, db dal.DB, gameID, userID string, chatID int64) error {
	if chatID == 0 {
		return nil
	}
	return db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		entry, err := dal4pairgame.GetGameTx(ctx, tx, gameID)
		if err != nil {
			return mapNotFound(err)
		}
		idx, pErr := findPlayerIndexByUserID(entry.Data.Players, userID)
		if pErr != nil {
			return pErr
		}
		if entry.Data.Players[idx].ChatID == chatID {
			return nil // already up to date; avoid a needless write
		}
		entry.Data.Players[idx].ChatID = chatID
		return tx.Set(ctx, entry.Record)
	})
}

// SetPlayerMessage records userID's own anchored private board message —
// the private-invite vs-Humans counterpart of SetGroupMessage, since there
// each player has their OWN board message to edit in place rather than one
// shared group message (see dal4pairgame.PlayerDbo.MessageID). Idempotent:
// setting the same messageID again is a no-op success. ErrPlayerNotInGame
// if userID is not a seated player; ErrPlayerIsBot if userID resolves to
// the game's bot seat (a bot seat never has a message of its own).
func SetPlayerMessage(ctx context.Context, db dal.DB, gameID, userID string, messageID int) error {
	return db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		entry, err := dal4pairgame.GetGameTx(ctx, tx, gameID)
		if err != nil {
			return mapNotFound(err)
		}
		idx, pErr := findPlayerIndexByUserID(entry.Data.Players, userID)
		if pErr != nil {
			return pErr
		}
		if entry.Data.Players[idx].MessageID == messageID {
			return nil // already up to date; avoid a needless write
		}
		entry.Data.Players[idx].MessageID = messageID
		return tx.Set(ctx, entry.Record)
	})
}

// GetView loads gameID and projects it into a read-only View (a plain,
// non-transactional read — see the package doc on why the result does not
// depend on who is asking).
func GetView(ctx context.Context, db dal.DB, gameID string) (View, error) {
	entry, err := dal4pairgame.GetGame(ctx, db, gameID)
	if err != nil {
		return View{}, mapNotFound(err)
	}
	d := entry.Data
	g := toGameState(d)
	return buildView(gameID, d, g), nil
}

func buildView(gameID string, d *dal4pairgame.GameDbo, g pairgame.GameState) View {
	revealedPairID := make(map[int]uint8, len(g.Log))
	for _, e := range g.Log {
		revealedPairID[e.Cell] = e.PairID
	}
	cells := make([]CellView, len(g.Faces))
	for cell := range g.Faces {
		cv := CellView{Cell: cell}
		if pairID, ok := revealedPairID[cell]; ok {
			cv.Revealed = true
			cv.PairID = pairID
			if owner := g.PairOwner[pairID]; owner != pairgame.NoPlayer {
				cv.Matched = true
				cv.MatchedBy = owner
			}
		}
		cells[cell] = cv
	}

	players := make([]PlayerView, len(g.Players))
	for i, p := range g.Players {
		players[i] = PlayerView{
			ID:        p.ID,
			UserID:    d.Players[i].UserID,
			Name:      d.Players[i].Name,
			IsBot:     p.IsBot,
			Score:     p.Score,
			Pending:   p.Pending,
			ChatID:    d.Players[i].ChatID,
			MessageID: d.Players[i].MessageID,
		}
	}

	return View{
		GameID:    gameID,
		SizeIndex: g.SizeIndex,
		Cells:     cells,
		Players:   players,
		Log:       g.Log,
		Complete:  g.IsComplete(),
		Winners:   g.Winners(),
		ChatID:    d.ChatID,
		MessageID: d.MessageID,
	}
}

func mapNotFound(err error) error {
	if dal4pairgame.IsNotFound(err) {
		return ErrGameNotFound
	}
	return err
}

func findPlayerIDByUserID(players []dal4pairgame.PlayerDbo, userID string) pairgame.PlayerID {
	for _, p := range players {
		if !p.IsBot && p.UserID == userID {
			return pairgame.PlayerID(p.ID)
		}
	}
	return pairgame.NoPlayer
}

// findPlayerIndexByUserID resolves userID to its index in players for
// SetPlayerChatID/SetPlayerMessage. Unlike findPlayerIDByUserID, it
// distinguishes "no such player" (ErrPlayerNotInGame) from "that player is
// the bot seat" (ErrPlayerIsBot) — a bot seat's UserID is always empty, so
// it can only ever match a caller-supplied userID of "", but the explicit
// IsBot check makes that rejection reason unambiguous rather than an
// incidental side effect of an empty-string comparison.
func findPlayerIndexByUserID(players []dal4pairgame.PlayerDbo, userID string) (int, error) {
	for i, p := range players {
		if p.UserID != userID {
			continue
		}
		if p.IsBot {
			return -1, ErrPlayerIsBot
		}
		return i, nil
	}
	return -1, ErrPlayerNotInGame
}
