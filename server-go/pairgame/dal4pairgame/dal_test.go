package dal4pairgame

import (
	"context"
	"errors"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/record"
)

func TestNewGameEntry_KeyShape(t *testing.T) {
	entry := NewGameEntry("g1")
	if entry.ID != "g1" {
		t.Fatalf("ID = %q, want g1", entry.ID)
	}
	want := "ext/pairmatching/games/g1"
	if got := entry.Key.String(); got != want {
		t.Fatalf("Key.String() = %q, want %q", got, want)
	}
}

func TestSaveAndGetGame_RoundTrip(t *testing.T) {
	ctx, db := newMemoryDB(t)

	entry := NewGameEntry("g1")
	entry.Data.HostUserID = "u1"
	entry.Data.Mode = ModeVsHumans
	entry.Data.Status = StatusLobby
	entry.Data.Players = []PlayerDbo{{ID: 1, UserID: "u1", Name: "Alice", Pending: -1}}

	if err := SaveGame(ctx, db, entry); err != nil {
		t.Fatalf("SaveGame: %v", err)
	}

	loaded, err := GetGame(ctx, db, "g1")
	if err != nil {
		t.Fatalf("GetGame: %v", err)
	}
	if loaded.Data.HostUserID != "u1" || len(loaded.Data.Players) != 1 || loaded.Data.Players[0].Name != "Alice" {
		t.Fatalf("loaded game mismatch: %+v", loaded.Data)
	}
}

func TestGetGameTx_ParticipatesInTransaction(t *testing.T) {
	ctx, db := newMemoryDB(t)
	entry := NewGameEntry("g1")
	entry.Data.HostUserID = "u1"
	entry.Data.Mode = ModeVsBot
	entry.Data.Status = StatusActive
	entry.Data.Players = []PlayerDbo{
		{ID: 1, UserID: "u1", Pending: -1},
		{ID: 2, IsBot: true, Pending: -1},
	}
	if err := SaveGame(ctx, db, entry); err != nil {
		t.Fatalf("SaveGame: %v", err)
	}

	err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		loaded, gErr := GetGameTx(ctx, tx, "g1")
		if gErr != nil {
			return gErr
		}
		if loaded.Data.HostUserID != "u1" {
			t.Fatalf("HostUserID = %q, want u1", loaded.Data.HostUserID)
		}
		loaded.Data.Players[0].Score = 3
		return tx.Set(ctx, loaded.Record)
	})
	if err != nil {
		t.Fatalf("RunReadwriteTransaction: %v", err)
	}

	loaded, err := GetGame(ctx, db, "g1")
	if err != nil {
		t.Fatalf("GetGame: %v", err)
	}
	if loaded.Data.Players[0].Score != 3 {
		t.Fatalf("Players[0].Score = %d, want 3 (set via GetGameTx + tx.Set)", loaded.Data.Players[0].Score)
	}

	// Not-found also propagates correctly through GetGameTx.
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		_, gErr := GetGameTx(ctx, tx, "missing")
		return gErr
	})
	if !IsNotFound(err) {
		t.Fatalf("GetGameTx(missing): IsNotFound(%v) = false, want true", err)
	}
}

func TestGetGame_NotFound(t *testing.T) {
	ctx, db := newMemoryDB(t)
	_, err := GetGame(ctx, db, "missing")
	if err == nil {
		t.Fatal("expected an error for a missing game")
	}
	if !IsNotFound(err) {
		t.Fatalf("IsNotFound(%v) = false, want true", err)
	}
	if !errors.Is(err, record.ErrRecordNotFound) {
		t.Fatalf("errors.Is(err, record.ErrRecordNotFound) = false")
	}
}

func TestGameDbo_Validate(t *testing.T) {
	validVsHumans := func() *GameDbo {
		return &GameDbo{
			HostUserID: "u1",
			Mode:       ModeVsHumans,
			Status:     StatusLobby,
			Players: []PlayerDbo{
				{ID: 1, UserID: "u1", Pending: -1},
				{ID: 2, UserID: "u2", Pending: -1},
			},
		}
	}
	if err := validVsHumans().Validate(); err != nil {
		t.Fatalf("expected valid vs-humans game to pass, got %v", err)
	}

	validVsBot := func() *GameDbo {
		return &GameDbo{
			HostUserID: "u1",
			Mode:       ModeVsBot,
			Status:     StatusActive,
			Players: []PlayerDbo{
				{ID: 1, UserID: "u1", Pending: -1},
				{ID: 2, IsBot: true, Pending: -1, Memory: 4},
			},
		}
	}
	if err := validVsBot().Validate(); err != nil {
		t.Fatalf("expected valid vs-bot game to pass, got %v", err)
	}

	noHost := validVsHumans()
	noHost.HostUserID = ""
	if err := noHost.Validate(); err == nil {
		t.Fatal("expected error for missing HostUserID")
	}

	badMode := validVsHumans()
	badMode.Mode = Mode("bogus")
	if err := badMode.Validate(); err == nil {
		t.Fatal("expected error for an invalid mode")
	}

	noPlayers := validVsHumans()
	noPlayers.Players = nil
	if err := noPlayers.Validate(); err == nil {
		t.Fatal("expected error for empty Players")
	}

	zeroID := validVsHumans()
	zeroID.Players = []PlayerDbo{{ID: 0, UserID: "u1"}}
	if err := zeroID.Validate(); err == nil {
		t.Fatal("expected error for a player with ID 0")
	}

	dupID := validVsHumans()
	dupID.Players = []PlayerDbo{{ID: 1, UserID: "u1"}, {ID: 1, UserID: "u2"}}
	if err := dupID.Validate(); err == nil {
		t.Fatal("expected error for duplicate player id")
	}

	blankUserID := validVsHumans()
	blankUserID.Players = []PlayerDbo{{ID: 1, UserID: ""}}
	if err := blankUserID.Validate(); err == nil {
		t.Fatal("expected error for a human player with an empty UserID")
	}

	dupUser := validVsHumans()
	dupUser.Players = []PlayerDbo{{ID: 1, UserID: "u1"}, {ID: 2, UserID: "u1"}}
	if err := dupUser.Validate(); err == nil {
		t.Fatal("expected error for duplicate player UserID")
	}

	vsBotNoBot := validVsBot()
	vsBotNoBot.Players = []PlayerDbo{{ID: 1, UserID: "u1"}, {ID: 2, UserID: "u2"}}
	if err := vsBotNoBot.Validate(); err == nil {
		t.Fatal("expected error for vs-bot mode with zero bots")
	}

	vsBotTwoBots := validVsBot()
	vsBotTwoBots.Players = []PlayerDbo{{ID: 1, IsBot: true}, {ID: 2, IsBot: true}}
	if err := vsBotTwoBots.Validate(); err == nil {
		t.Fatal("expected error for vs-bot mode with two bots")
	}

	vsHumansWithBot := validVsHumans()
	vsHumansWithBot.Players = append(vsHumansWithBot.Players, PlayerDbo{ID: 3, IsBot: true})
	if err := vsHumansWithBot.Validate(); err == nil {
		t.Fatal("expected error for vs-humans mode with a bot seated")
	}

	badStatus := validVsHumans()
	badStatus.Status = Status("bogus")
	if err := badStatus.Validate(); err == nil {
		t.Fatal("expected error for an invalid status")
	}
}
