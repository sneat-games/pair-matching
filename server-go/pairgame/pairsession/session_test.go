package pairsession

import (
	"errors"
	"testing"

	"github.com/sneat-games/pair-matching/server-go/pairgame"
	"github.com/sneat-games/pair-matching/server-go/pairgame/dal4pairgame"
)

// findPair returns two distinct cell indices that share a face — mirrors
// pairgame's own test helper of the same name, duplicated here since it is
// unexported there.
func findPair(faces []uint8) (a, b int) {
	seen := make(map[uint8]int, len(faces))
	for i, f := range faces {
		if j, ok := seen[f]; ok {
			return j, i
		}
		seen[f] = i
	}
	panic("no pair found")
}

func TestCreateVsBotGame_DealsAndActivatesImmediately(t *testing.T) {
	ctx, db := newMemoryDB(t)
	gameID, err := CreateVsBotGame(ctx, db, 3, PlayerRef{UserID: "host", Name: "Host", ChatID: 111}, 4, 999)
	if err != nil {
		t.Fatalf("CreateVsBotGame: %v", err)
	}
	if gameID == "" {
		t.Fatal("expected a non-empty gameID")
	}

	entry, err := dal4pairgame.GetGame(ctx, db, gameID)
	if err != nil {
		t.Fatalf("GetGame: %v", err)
	}
	d := entry.Data
	if d.Mode != dal4pairgame.ModeVsBot {
		t.Fatalf("Mode = %v, want ModeVsBot", d.Mode)
	}
	if d.Status != dal4pairgame.StatusActive {
		t.Fatalf("Status = %v, want StatusActive (vs-bot needs no lobby)", d.Status)
	}
	if d.ChatID != 999 {
		t.Fatalf("ChatID = %d, want 999", d.ChatID)
	}
	if len(d.Players) != 2 || d.Players[0].UserID != "host" || !d.Players[1].IsBot || d.Players[1].Memory != 4 {
		t.Fatalf("unexpected players: %+v", d.Players)
	}
	if len(d.Faces) != pairgame.Sizes[3].Cells() {
		t.Fatalf("Faces has %d entries, want %d (board dealt immediately)", len(d.Faces), pairgame.Sizes[3].Cells())
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("created game fails its own Validate(): %v", err)
	}
}

func TestCreateVsBotGame_RequiresHostUserID(t *testing.T) {
	ctx, db := newMemoryDB(t)
	if _, err := CreateVsBotGame(ctx, db, 0, PlayerRef{}, 0, 0); err == nil {
		t.Fatal("expected an error for a missing host.UserID")
	}
}

func TestCreateVsHumansGame_SeedsALobbyWithNoBoardYet(t *testing.T) {
	ctx, db := newMemoryDB(t)
	gameID, err := CreateVsHumansGame(ctx, db, 0, PlayerRef{UserID: "host"}, 0)
	if err != nil {
		t.Fatalf("CreateVsHumansGame: %v", err)
	}
	entry, err := dal4pairgame.GetGame(ctx, db, gameID)
	if err != nil {
		t.Fatalf("GetGame: %v", err)
	}
	d := entry.Data
	if d.Mode != dal4pairgame.ModeVsHumans {
		t.Fatalf("Mode = %v, want ModeVsHumans", d.Mode)
	}
	if d.Status != dal4pairgame.StatusLobby {
		t.Fatalf("Status = %v, want StatusLobby", d.Status)
	}
	if len(d.Faces) != 0 {
		t.Fatalf("Faces = %v, want empty (not dealt until StartGame)", d.Faces)
	}
	if len(d.Players) != 1 {
		t.Fatalf("Players = %v, want exactly the host", d.Players)
	}
}

func TestJoinGame_AddsPlayerAndIsIdempotent(t *testing.T) {
	ctx, db := newMemoryDB(t)
	gameID, _ := CreateVsHumansGame(ctx, db, 0, PlayerRef{UserID: "host"}, 0)

	if err := JoinGame(ctx, db, gameID, PlayerRef{UserID: "p2", Name: "Bob"}); err != nil {
		t.Fatalf("JoinGame: %v", err)
	}
	entry, _ := dal4pairgame.GetGame(ctx, db, gameID)
	if len(entry.Data.Players) != 2 || entry.Data.Players[1].UserID != "p2" || entry.Data.Players[1].ID != 2 {
		t.Fatalf("unexpected players after join: %+v", entry.Data.Players)
	}

	// Re-joining is a no-op that still captures a freshly-seen ChatID.
	if err := JoinGame(ctx, db, gameID, PlayerRef{UserID: "p2", ChatID: 42}); err != nil {
		t.Fatalf("re-JoinGame: %v", err)
	}
	entry, _ = dal4pairgame.GetGame(ctx, db, gameID)
	if len(entry.Data.Players) != 2 {
		t.Fatalf("re-join added a duplicate seat: %+v", entry.Data.Players)
	}
	if entry.Data.Players[1].ChatID != 42 {
		t.Fatalf("ChatID = %d, want 42 (captured on re-join)", entry.Data.Players[1].ChatID)
	}
}

func TestJoinGame_RejectsVsBotGame(t *testing.T) {
	ctx, db := newMemoryDB(t)
	gameID, _ := CreateVsBotGame(ctx, db, 0, PlayerRef{UserID: "host"}, 0, 0)
	if err := JoinGame(ctx, db, gameID, PlayerRef{UserID: "p2"}); err != ErrWrongModeForJoin {
		t.Errorf("JoinGame(vs-bot game) = %v, want ErrWrongModeForJoin", err)
	}
}

func TestJoinGame_RejectsOverflow(t *testing.T) {
	ctx, db := newMemoryDB(t)
	gameID, _ := CreateVsHumansGame(ctx, db, 3, PlayerRef{UserID: "u0"}, 0)
	for i := 1; i < pairgame.MaxPlayers; i++ {
		if err := JoinGame(ctx, db, gameID, PlayerRef{UserID: idFor(i)}); err != nil {
			t.Fatalf("JoinGame(%d): %v", i, err)
		}
	}
	if err := JoinGame(ctx, db, gameID, PlayerRef{UserID: "one-too-many"}); err != ErrTooManyPlayers {
		t.Errorf("JoinGame beyond MaxPlayers = %v, want ErrTooManyPlayers", err)
	}
}

func TestJoinGame_RejectsAfterGameStarted(t *testing.T) {
	ctx, db := newMemoryDB(t)
	gameID, _ := CreateVsHumansGame(ctx, db, 0, PlayerRef{UserID: "host"}, 0)
	if err := JoinGame(ctx, db, gameID, PlayerRef{UserID: "p2"}); err != nil {
		t.Fatalf("JoinGame: %v", err)
	}
	if err := StartGame(ctx, db, gameID); err != nil {
		t.Fatalf("StartGame: %v", err)
	}
	if err := JoinGame(ctx, db, gameID, PlayerRef{UserID: "late"}); err != ErrGameNotInLobby {
		t.Errorf("JoinGame(after start) = %v, want ErrGameNotInLobby", err)
	}
}

func TestStartGame_RequiresTwoPlayers(t *testing.T) {
	ctx, db := newMemoryDB(t)
	gameID, _ := CreateVsHumansGame(ctx, db, 0, PlayerRef{UserID: "host"}, 0)
	if err := StartGame(ctx, db, gameID); err != ErrNotEnoughPlayers {
		t.Errorf("StartGame(1 player) = %v, want ErrNotEnoughPlayers", err)
	}
}

func TestStartGame_DealsTheBoard(t *testing.T) {
	ctx, db := newMemoryDB(t)
	gameID, _ := CreateVsHumansGame(ctx, db, 3, PlayerRef{UserID: "host"}, 0)
	if err := JoinGame(ctx, db, gameID, PlayerRef{UserID: "p2"}); err != nil {
		t.Fatalf("JoinGame: %v", err)
	}
	if err := StartGame(ctx, db, gameID); err != nil {
		t.Fatalf("StartGame: %v", err)
	}
	entry, _ := dal4pairgame.GetGame(ctx, db, gameID)
	if entry.Data.Status != dal4pairgame.StatusActive {
		t.Fatalf("Status = %v, want StatusActive", entry.Data.Status)
	}
	if len(entry.Data.Faces) != pairgame.Sizes[3].Cells() {
		t.Fatalf("Faces has %d entries, want %d", len(entry.Data.Faces), pairgame.Sizes[3].Cells())
	}
}

func TestFlip_MatchAndSnipeEndToEnd(t *testing.T) {
	ctx, db := newMemoryDB(t)
	gameID, _ := CreateVsHumansGame(ctx, db, 3, PlayerRef{UserID: "alice"}, 0)
	if err := JoinGame(ctx, db, gameID, PlayerRef{UserID: "bob"}); err != nil {
		t.Fatalf("JoinGame: %v", err)
	}
	if err := StartGame(ctx, db, gameID); err != nil {
		t.Fatalf("StartGame: %v", err)
	}

	entry, _ := dal4pairgame.GetGame(ctx, db, gameID)
	a, b := findPair(entry.Data.Faces)

	// alice opens 'a' as her first pick.
	outcome, err := Flip(ctx, db, gameID, "alice", a)
	if err != nil {
		t.Fatalf("Flip(alice, a): %v", err)
	}
	if outcome.Matched {
		t.Fatalf("first pick should never match: %+v", outcome)
	}

	// bob snipes the pair in a SINGLE flip of 'b' — under the founder's
	// any-player-may-match rule, he does not need to first flip 'a'
	// himself; alice's still-exposed pending 'a' is enough.
	outcome, err = Flip(ctx, db, gameID, "bob", b)
	if err != nil {
		t.Fatalf("Flip(bob, b): %v", err)
	}
	if !outcome.Matched || outcome.MatchedBy != 2 {
		t.Fatalf("outcome = %+v, want bob (PlayerID 2) to claim the pair", outcome)
	}

	view, err := GetView(ctx, db, gameID)
	if err != nil {
		t.Fatalf("GetView: %v", err)
	}
	if view.Players[1].Score != 1 {
		t.Fatalf("bob's Score in the view = %d, want 1", view.Players[1].Score)
	}
	if view.Players[0].Score != 0 {
		t.Fatalf("alice's Score in the view = %d, want 0", view.Players[0].Score)
	}
	if !view.Cells[a].Matched || view.Cells[a].MatchedBy != 2 {
		t.Fatalf("Cells[a] = %+v, want Matched by PlayerID 2", view.Cells[a])
	}
}

func TestFlip_RejectsUnknownPlayer(t *testing.T) {
	ctx, db := newMemoryDB(t)
	gameID, _ := CreateVsBotGame(ctx, db, 0, PlayerRef{UserID: "host"}, 0, 0)
	if _, err := Flip(ctx, db, gameID, "stranger", 0); err != ErrPlayerNotInGame {
		t.Errorf("Flip(unknown userID) = %v, want ErrPlayerNotInGame", err)
	}
}

func TestFlip_RejectsBeforeGameStarted(t *testing.T) {
	ctx, db := newMemoryDB(t)
	gameID, _ := CreateVsHumansGame(ctx, db, 0, PlayerRef{UserID: "host"}, 0)
	if _, err := Flip(ctx, db, gameID, "host", 0); err != ErrGameNotActive {
		t.Errorf("Flip(lobby game) = %v, want ErrGameNotActive", err)
	}
}

func TestFlip_MarksGameFinishedOnLastMatch(t *testing.T) {
	ctx, db := newMemoryDB(t)
	gameID, _ := CreateVsBotGame(ctx, db, 0, PlayerRef{UserID: "host"}, 0, 0) // 2x2: exactly 1 pair... wait Sizes[0] is 2 pairs
	entry, _ := dal4pairgame.GetGame(ctx, db, gameID)
	faces := entry.Data.Faces
	pairs := pairgame.Sizes[0].Pairs()
	claimed := 0
	seenAt := make(map[uint8]int, pairs)
	for cell, pairID := range faces {
		if first, ok := seenAt[pairID]; ok {
			if _, err := Flip(ctx, db, gameID, "host", first); err != nil {
				t.Fatalf("Flip(%d): %v", first, err)
			}
			outcome, err := Flip(ctx, db, gameID, "host", cell)
			if err != nil {
				t.Fatalf("Flip(%d): %v", cell, err)
			}
			if !outcome.Matched {
				t.Fatalf("expected a match for pair %d", pairID)
			}
			claimed++
			continue
		}
		seenAt[pairID] = cell
	}
	if claimed != pairs {
		t.Fatalf("claimed %d pairs, want %d", claimed, pairs)
	}
	entry, _ = dal4pairgame.GetGame(ctx, db, gameID)
	if entry.Data.Status != dal4pairgame.StatusFinished {
		t.Fatalf("Status = %v, want StatusFinished", entry.Data.Status)
	}
}

func TestRobotMove_FlipsExactlyOneCardPerCall(t *testing.T) {
	ctx, db := newMemoryDB(t)
	gameID, _ := CreateVsBotGame(ctx, db, 3, PlayerRef{UserID: "host"}, 8, 0)

	outcome, err := RobotMove(ctx, db, gameID)
	if err != nil {
		t.Fatalf("RobotMove: %v", err)
	}
	view, err := GetView(ctx, db, gameID)
	if err != nil {
		t.Fatalf("GetView: %v", err)
	}
	if len(view.Log) != 1 {
		t.Fatalf("Log has %d entries after one RobotMove call, want exactly 1", len(view.Log))
	}
	if view.Log[0].By != 2 {
		t.Fatalf("Log[0].By = %d, want the bot's PlayerID 2", view.Log[0].By)
	}
	if outcome.Matched {
		t.Fatalf("a lone first pick should never match: %+v", outcome)
	}

	// A second call flips exactly one more card (the bot's own second pick).
	if _, err := RobotMove(ctx, db, gameID); err != nil {
		t.Fatalf("RobotMove (2nd call): %v", err)
	}
	view, _ = GetView(ctx, db, gameID)
	if len(view.Log) != 2 {
		t.Fatalf("Log has %d entries after two RobotMove calls, want exactly 2", len(view.Log))
	}
}

func TestRobotMove_RejectsNonBotGame(t *testing.T) {
	ctx, db := newMemoryDB(t)
	gameID, _ := CreateVsHumansGame(ctx, db, 0, PlayerRef{UserID: "host"}, 0)
	if err := JoinGame(ctx, db, gameID, PlayerRef{UserID: "p2"}); err != nil {
		t.Fatalf("JoinGame: %v", err)
	}
	if err := StartGame(ctx, db, gameID); err != nil {
		t.Fatalf("StartGame: %v", err)
	}
	if _, err := RobotMove(ctx, db, gameID); err != ErrNoBotInGame {
		t.Errorf("RobotMove(vs-humans game) = %v, want ErrNoBotInGame", err)
	}
}

func TestGetView_NotFound(t *testing.T) {
	ctx, db := newMemoryDB(t)
	if _, err := GetView(ctx, db, "missing"); err != ErrGameNotFound {
		t.Errorf("GetView(missing) = %v, want ErrGameNotFound", err)
	}
}

func TestJoinGame_NotFound(t *testing.T) {
	ctx, db := newMemoryDB(t)
	if err := JoinGame(ctx, db, "missing", PlayerRef{UserID: "p"}); err != ErrGameNotFound {
		t.Errorf("JoinGame(missing) = %v, want ErrGameNotFound", err)
	}
}

func TestJoinGame_RequiresUserID(t *testing.T) {
	ctx, db := newMemoryDB(t)
	gameID, _ := CreateVsHumansGame(ctx, db, 0, PlayerRef{UserID: "host"}, 0)
	if err := JoinGame(ctx, db, gameID, PlayerRef{}); err == nil {
		t.Fatal("expected an error for a missing player.UserID")
	}
}

func TestStartGame_NotFound(t *testing.T) {
	ctx, db := newMemoryDB(t)
	if err := StartGame(ctx, db, "missing"); err != ErrGameNotFound {
		t.Errorf("StartGame(missing) = %v, want ErrGameNotFound", err)
	}
}

func TestStartGame_RejectsVsBotGame(t *testing.T) {
	ctx, db := newMemoryDB(t)
	gameID, _ := CreateVsBotGame(ctx, db, 0, PlayerRef{UserID: "host"}, 0, 0)
	if err := StartGame(ctx, db, gameID); err != ErrWrongModeForStart {
		t.Errorf("StartGame(vs-bot game) = %v, want ErrWrongModeForStart", err)
	}
}

func TestStartGame_RejectsAlreadyActive(t *testing.T) {
	ctx, db := newMemoryDB(t)
	gameID, _ := CreateVsHumansGame(ctx, db, 0, PlayerRef{UserID: "host"}, 0)
	if err := JoinGame(ctx, db, gameID, PlayerRef{UserID: "p2"}); err != nil {
		t.Fatalf("JoinGame: %v", err)
	}
	if err := StartGame(ctx, db, gameID); err != nil {
		t.Fatalf("StartGame: %v", err)
	}
	if err := StartGame(ctx, db, gameID); err != ErrGameNotInLobby {
		t.Errorf("StartGame(already active) = %v, want ErrGameNotInLobby", err)
	}
}

func TestCreateVsHumansGame_RejectsInvalidSizeIndex(t *testing.T) {
	ctx, db := newMemoryDB(t)
	if _, err := CreateVsHumansGame(ctx, db, len(pairgame.Sizes), PlayerRef{UserID: "host"}, 0); err != pairgame.ErrInvalidSizeIndex {
		t.Errorf("CreateVsHumansGame(bad sizeIndex) = %v, want ErrInvalidSizeIndex", err)
	}
}

func TestCreateVsHumansGame_RequiresHostUserID(t *testing.T) {
	ctx, db := newMemoryDB(t)
	if _, err := CreateVsHumansGame(ctx, db, 0, PlayerRef{}, 0); err == nil {
		t.Fatal("expected an error for a missing host.UserID")
	}
}

func TestFlip_NotFound(t *testing.T) {
	ctx, db := newMemoryDB(t)
	if _, err := Flip(ctx, db, "missing", "host", 0); err != ErrGameNotFound {
		t.Errorf("Flip(missing game) = %v, want ErrGameNotFound", err)
	}
}

func TestRobotMove_NotFound(t *testing.T) {
	ctx, db := newMemoryDB(t)
	if _, err := RobotMove(ctx, db, "missing"); err != ErrGameNotFound {
		t.Errorf("RobotMove(missing game) = %v, want ErrGameNotFound", err)
	}
}

func TestRobotMove_RejectsBeforeGameActive(t *testing.T) {
	ctx, db := newMemoryDB(t)
	gameID, _ := CreateVsHumansGame(ctx, db, 0, PlayerRef{UserID: "host"}, 0)
	if err := JoinGame(ctx, db, gameID, PlayerRef{UserID: "p2"}); err != nil {
		t.Fatalf("JoinGame: %v", err)
	}
	// Still Lobby: not vs-bot mode, so ErrNoBotInGame fires before the
	// active-status check even applies.
	if _, err := RobotMove(ctx, db, gameID); err != ErrNoBotInGame {
		t.Errorf("RobotMove(lobby vs-humans game) = %v, want ErrNoBotInGame", err)
	}
}

func TestCreateVsBotGame_RejectsInvalidSizeIndex(t *testing.T) {
	ctx, db := newMemoryDB(t)
	if _, err := CreateVsBotGame(ctx, db, len(pairgame.Sizes), PlayerRef{UserID: "host"}, 0, 0); err == nil {
		t.Fatal("expected an error for an invalid sizeIndex")
	}
}

func TestRobotMove_RejectsAfterGameFinished(t *testing.T) {
	ctx, db := newMemoryDB(t)
	gameID, _ := CreateVsBotGame(ctx, db, 0, PlayerRef{UserID: "host"}, 8, 0) // 2x2: 2 pairs
	for i := 0; i < 40; i++ {
		view, err := GetView(ctx, db, gameID)
		if err != nil {
			t.Fatalf("GetView: %v", err)
		}
		if view.Complete {
			break
		}
		if _, err := RobotMove(ctx, db, gameID); err != nil {
			t.Fatalf("RobotMove: %v", err)
		}
	}
	view, err := GetView(ctx, db, gameID)
	if err != nil {
		t.Fatalf("GetView: %v", err)
	}
	if !view.Complete {
		t.Fatalf("game did not complete via RobotMove within a generous budget")
	}
	if _, err := RobotMove(ctx, db, gameID); err != ErrGameNotActive {
		t.Errorf("RobotMove(finished game) = %v, want ErrGameNotActive", err)
	}
}

func TestNewGameID_ProducesDistinctIDs(t *testing.T) {
	a := NewGameID()
	b := NewGameID()
	if a == "" || b == "" {
		t.Fatal("NewGameID returned an empty string")
	}
	if a == b {
		t.Fatal("two calls to NewGameID produced the same ID")
	}
}

func TestGetView_UnrevealedCellsStayHidden(t *testing.T) {
	ctx, db := newMemoryDB(t)
	gameID, _ := CreateVsBotGame(ctx, db, 3, PlayerRef{UserID: "host"}, 0, 0)
	view, err := GetView(ctx, db, gameID)
	if err != nil {
		t.Fatalf("GetView: %v", err)
	}
	for _, cv := range view.Cells {
		if cv.Revealed {
			t.Fatalf("Cell %d is Revealed before any flip has happened", cv.Cell)
		}
	}
}

// idFor generates distinct userIDs for the overflow test.
func idFor(i int) string {
	return string(rune('a' + i))
}

func TestSetGroupMessage(t *testing.T) {
	ctx, db := newMemoryDB(t)
	gameID, _ := CreateVsHumansGame(ctx, db, 0, PlayerRef{UserID: "host"}, 123)

	if err := SetGroupMessage(ctx, db, gameID, 456); err != nil {
		t.Fatalf("SetGroupMessage: %v", err)
	}
	entry, _ := dal4pairgame.GetGame(ctx, db, gameID)
	if entry.Data.MessageID != 456 {
		t.Fatalf("MessageID = %d, want 456", entry.Data.MessageID)
	}
	view, err := GetView(ctx, db, gameID)
	if err != nil {
		t.Fatalf("GetView: %v", err)
	}
	if view.ChatID != 123 || view.MessageID != 456 {
		t.Fatalf("view.ChatID/MessageID = %d/%d, want 123/456", view.ChatID, view.MessageID)
	}

	// Idempotent: setting the same value again is a no-op success.
	if err := SetGroupMessage(ctx, db, gameID, 456); err != nil {
		t.Fatalf("SetGroupMessage (repeat): %v", err)
	}
	entry, _ = dal4pairgame.GetGame(ctx, db, gameID)
	if entry.Data.MessageID != 456 {
		t.Fatalf("MessageID after repeat = %d, want 456", entry.Data.MessageID)
	}
}

func TestSetGroupMessage_NotFound(t *testing.T) {
	ctx, db := newMemoryDB(t)
	if err := SetGroupMessage(ctx, db, "missing", 1); !errors.Is(err, ErrGameNotFound) {
		t.Fatalf("SetGroupMessage(missing): err = %v, want ErrGameNotFound", err)
	}
}

func TestSetPlayerChatID(t *testing.T) {
	ctx, db := newMemoryDB(t)
	gameID, _ := CreateVsHumansGame(ctx, db, 0, PlayerRef{UserID: "host"}, 0)
	if err := JoinGame(ctx, db, gameID, PlayerRef{UserID: "guest"}); err != nil {
		t.Fatalf("JoinGame: %v", err)
	}

	if err := SetPlayerChatID(ctx, db, gameID, "guest", 42); err != nil {
		t.Fatalf("SetPlayerChatID: %v", err)
	}
	entry, _ := dal4pairgame.GetGame(ctx, db, gameID)
	if entry.Data.Players[1].ChatID != 42 {
		t.Fatalf("ChatID = %d, want 42", entry.Data.Players[1].ChatID)
	}
	view, err := GetView(ctx, db, gameID)
	if err != nil {
		t.Fatalf("GetView: %v", err)
	}
	if view.Players[1].ChatID != 42 {
		t.Fatalf("view.Players[1].ChatID = %d, want 42", view.Players[1].ChatID)
	}

	// Idempotent: setting the same value again is a no-op success.
	if err := SetPlayerChatID(ctx, db, gameID, "guest", 42); err != nil {
		t.Fatalf("SetPlayerChatID (repeat): %v", err)
	}

	// Unknown player.
	if err := SetPlayerChatID(ctx, db, gameID, "stranger", 1); !errors.Is(err, ErrPlayerNotInGame) {
		t.Fatalf("SetPlayerChatID(stranger): err = %v, want ErrPlayerNotInGame", err)
	}

	// Zero chatID is a documented no-op, including for a non-existent game.
	if err := SetPlayerChatID(ctx, db, "missing-game", "guest", 0); err != nil {
		t.Fatalf("SetPlayerChatID with chatID=0 should be a no-op, got %v", err)
	}
}

func TestSetPlayerChatID_NotFound(t *testing.T) {
	ctx, db := newMemoryDB(t)
	if err := SetPlayerChatID(ctx, db, "missing", "guest", 1); !errors.Is(err, ErrGameNotFound) {
		t.Fatalf("SetPlayerChatID(missing game): err = %v, want ErrGameNotFound", err)
	}
}

func TestSetPlayerChatID_RejectsBotSeat(t *testing.T) {
	ctx, db := newMemoryDB(t)
	gameID, _ := CreateVsBotGame(ctx, db, 0, PlayerRef{UserID: "host"}, 0, 0)
	if err := SetPlayerChatID(ctx, db, gameID, "", 42); !errors.Is(err, ErrPlayerIsBot) {
		t.Fatalf("SetPlayerChatID(bot seat): err = %v, want ErrPlayerIsBot", err)
	}
	entry, _ := dal4pairgame.GetGame(ctx, db, gameID)
	if entry.Data.Players[1].ChatID != 0 {
		t.Fatalf("bot seat ChatID = %d, want 0 (untouched)", entry.Data.Players[1].ChatID)
	}
}

func TestSetPlayerMessage(t *testing.T) {
	ctx, db := newMemoryDB(t)
	gameID, _ := CreateVsHumansGame(ctx, db, 0, PlayerRef{UserID: "host"}, 0)
	if err := JoinGame(ctx, db, gameID, PlayerRef{UserID: "guest"}); err != nil {
		t.Fatalf("JoinGame: %v", err)
	}

	if err := SetPlayerMessage(ctx, db, gameID, "guest", 789); err != nil {
		t.Fatalf("SetPlayerMessage: %v", err)
	}
	entry, _ := dal4pairgame.GetGame(ctx, db, gameID)
	if entry.Data.Players[1].MessageID != 789 {
		t.Fatalf("MessageID = %d, want 789", entry.Data.Players[1].MessageID)
	}
	view, err := GetView(ctx, db, gameID)
	if err != nil {
		t.Fatalf("GetView: %v", err)
	}
	if view.Players[1].MessageID != 789 {
		t.Fatalf("view.Players[1].MessageID = %d, want 789", view.Players[1].MessageID)
	}
	// The host's own message must be untouched.
	if view.Players[0].MessageID != 0 {
		t.Fatalf("host's MessageID = %d, want 0 (untouched)", view.Players[0].MessageID)
	}

	// Idempotent: setting the same value again is a no-op success.
	if err := SetPlayerMessage(ctx, db, gameID, "guest", 789); err != nil {
		t.Fatalf("SetPlayerMessage (repeat): %v", err)
	}
	entry, _ = dal4pairgame.GetGame(ctx, db, gameID)
	if entry.Data.Players[1].MessageID != 789 {
		t.Fatalf("MessageID after repeat = %d, want 789", entry.Data.Players[1].MessageID)
	}

	// Unknown player.
	if err := SetPlayerMessage(ctx, db, gameID, "stranger", 1); !errors.Is(err, ErrPlayerNotInGame) {
		t.Fatalf("SetPlayerMessage(stranger): err = %v, want ErrPlayerNotInGame", err)
	}
}

func TestSetPlayerMessage_NotFound(t *testing.T) {
	ctx, db := newMemoryDB(t)
	if err := SetPlayerMessage(ctx, db, "missing", "guest", 1); !errors.Is(err, ErrGameNotFound) {
		t.Fatalf("SetPlayerMessage(missing game): err = %v, want ErrGameNotFound", err)
	}
}

func TestSetPlayerMessage_RejectsBotSeat(t *testing.T) {
	ctx, db := newMemoryDB(t)
	gameID, _ := CreateVsBotGame(ctx, db, 0, PlayerRef{UserID: "host"}, 0, 0)
	if err := SetPlayerMessage(ctx, db, gameID, "", 99); !errors.Is(err, ErrPlayerIsBot) {
		t.Fatalf("SetPlayerMessage(bot seat): err = %v, want ErrPlayerIsBot", err)
	}
	entry, _ := dal4pairgame.GetGame(ctx, db, gameID)
	if entry.Data.Players[1].MessageID != 0 {
		t.Fatalf("bot seat MessageID = %d, want 0 (untouched)", entry.Data.Players[1].MessageID)
	}
}

// TestSettersDoNotTouchRuleState guards against a session-layer setter
// accidentally reaching into PairOwner, Pending, Log, or Score — these are
// plain state recordings, not game moves (see the package doc on Flip/
// RobotMove owning all rule state).
func TestSettersDoNotTouchRuleState(t *testing.T) {
	ctx, db := newMemoryDB(t)
	gameID, _ := CreateVsHumansGame(ctx, db, 3, PlayerRef{UserID: "alice"}, 0)
	if err := JoinGame(ctx, db, gameID, PlayerRef{UserID: "bob"}); err != nil {
		t.Fatalf("JoinGame: %v", err)
	}
	if err := StartGame(ctx, db, gameID); err != nil {
		t.Fatalf("StartGame: %v", err)
	}
	before, err := GetView(ctx, db, gameID)
	if err != nil {
		t.Fatalf("GetView: %v", err)
	}

	if err := SetGroupMessage(ctx, db, gameID, 1); err != nil {
		t.Fatalf("SetGroupMessage: %v", err)
	}
	if err := SetPlayerChatID(ctx, db, gameID, "alice", 2); err != nil {
		t.Fatalf("SetPlayerChatID: %v", err)
	}
	if err := SetPlayerMessage(ctx, db, gameID, "bob", 3); err != nil {
		t.Fatalf("SetPlayerMessage: %v", err)
	}

	after, err := GetView(ctx, db, gameID)
	if err != nil {
		t.Fatalf("GetView: %v", err)
	}
	if len(after.Log) != len(before.Log) {
		t.Fatalf("Log changed: before=%d entries, after=%d", len(before.Log), len(after.Log))
	}
	for i := range after.Players {
		if after.Players[i].Score != before.Players[i].Score {
			t.Fatalf("Players[%d].Score changed: before=%d, after=%d", i, before.Players[i].Score, after.Players[i].Score)
		}
		if after.Players[i].Pending != before.Players[i].Pending {
			t.Fatalf("Players[%d].Pending changed: before=%d, after=%d", i, before.Players[i].Pending, after.Players[i].Pending)
		}
	}
	for i := range after.Cells {
		if after.Cells[i].Matched != before.Cells[i].Matched || after.Cells[i].MatchedBy != before.Cells[i].MatchedBy {
			t.Fatalf("Cells[%d] match state changed", i)
		}
	}
}
