package pairgame

import (
	"encoding/base64"
	"testing"
)

func TestNewGameRejectsInvalidSizeIndex(t *testing.T) {
	for _, idx := range []int{-1, len(Sizes)} {
		if _, err := NewGame(LayoutInline, idx, 1, nil, []PlayerSetup{{}}); err != ErrInvalidSizeIndex {
			t.Errorf("NewGame(sizeIndex=%d) = %v, want ErrInvalidSizeIndex", idx, err)
		}
	}
}

func TestNewGameRejectsInvalidPlayerCount(t *testing.T) {
	if _, err := NewGame(LayoutInline, 0, 1, nil, nil); err != ErrInvalidPlayerCount {
		t.Errorf("NewGame(0 players) = %v, want ErrInvalidPlayerCount", err)
	}
	tooMany := make([]PlayerSetup, MaxPlayers+1)
	if _, err := NewGame(LayoutInline, 0, 1, nil, tooMany); err != ErrInvalidPlayerCount {
		t.Errorf("NewGame(%d players) = %v, want ErrInvalidPlayerCount", len(tooMany), err)
	}
	maxSetup := make([]PlayerSetup, MaxPlayers)
	if _, err := NewGame(LayoutInline, 0, 1, nil, maxSetup); err != nil {
		t.Errorf("NewGame(MaxPlayers players) = %v, want success", err)
	}
}

func TestNewGameRejectsNegativeMemory(t *testing.T) {
	setup := []PlayerSetup{{IsBot: true, Memory: -1}}
	if _, err := NewGame(LayoutInline, 0, 1, nil, setup); err != ErrInvalidMemory {
		t.Errorf("NewGame(Memory=-1) = %v, want ErrInvalidMemory", err)
	}
}

func TestNewGameForcesHumanMemoryToZero(t *testing.T) {
	setup := []PlayerSetup{{IsBot: false, Memory: 99}}
	g, err := NewGame(LayoutInline, 0, 1, nil, setup)
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	if g.Players[0].Memory != 0 {
		t.Errorf("human Memory = %d, want 0 (caller-supplied value must be ignored)", g.Players[0].Memory)
	}
}

func TestNewGameAssignsSequentialPlayerIDs(t *testing.T) {
	setup := []PlayerSetup{{}, {IsBot: true, Memory: 3}, {}}
	g, err := NewGame(LayoutInline, 3, 1, nil, setup)
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	for i, p := range g.Players {
		if p.ID != PlayerID(i+1) {
			t.Errorf("Players[%d].ID = %d, want %d", i, p.ID, i+1)
		}
		if p.Pending != -1 {
			t.Errorf("Players[%d].Pending = %d, want -1", i, p.Pending)
		}
	}
	if !g.Players[1].IsBot || g.Players[1].Memory != 3 {
		t.Errorf("Players[1] = %+v, want the bot setup", g.Players[1])
	}
}

func TestNewSoloGame(t *testing.T) {
	g, err := NewSoloGame(LayoutInline, 3, 1, nil)
	if err != nil {
		t.Fatalf("NewSoloGame: %v", err)
	}
	if !g.isSolo() {
		t.Fatalf("NewSoloGame produced a non-solo shape: %+v", g.Players)
	}
	if g.Players[0].ID != 1 || g.Players[0].IsBot {
		t.Errorf("solo player = %+v, want a lone human, ID 1", g.Players[0])
	}
}

func TestNewSoloGameRejectsABoardThatDoesNotFit(t *testing.T) {
	// 8x8 under LayoutInline does not fit the solo callback-data budget —
	// see EncodedBitLen's doc comment and TestSoloBudgetMatrix.
	if _, err := NewSoloGame(LayoutInline, 8, 1, nil); err != ErrSoloBoardTooLarge {
		t.Errorf("NewSoloGame(LayoutInline, 8x8) = %v, want ErrSoloBoardTooLarge", err)
	}
	if _, err := NewSoloGame(LayoutSeedDerived, 8, 1, []byte("secret")); err != nil {
		t.Errorf("NewSoloGame(LayoutSeedDerived, 8x8) = %v, want success", err)
	}
}

func TestEncodeRejectsNonSoloState(t *testing.T) {
	g, err := NewGame(LayoutInline, 0, 1, nil, []PlayerSetup{{}, {IsBot: true}})
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	if _, err := g.Encode(); err != ErrNotSoloGame {
		t.Errorf("Encode(vs-bot state) = %v, want ErrNotSoloGame", err)
	}

	humans, err := NewGame(LayoutInline, 0, 1, nil, []PlayerSetup{{}, {}})
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	if _, err := humans.Encode(); err != ErrNotSoloGame {
		t.Errorf("Encode(vs-humans state) = %v, want ErrNotSoloGame", err)
	}
}

func TestDecodeRejectsTruncatedPayload(t *testing.T) {
	full := mustEncodeSample(t)
	for cut := 1; cut < 4; cut++ {
		s := full[:cut]
		if _, err := Decode(s); err == nil {
			t.Errorf("Decode(%q) (truncated) = nil error, want an error", s)
		}
	}
}

func TestDecodeRejectsUnknownSizeIndex(t *testing.T) {
	w := &bitWriter{}
	w.writeBits(snapshotVersion, versionBits)
	w.writeBits(uint64(LayoutInline), layoutModeBits)
	w.writeBits(uint64(len(Sizes)), sizeIndexBits) // out of range
	s := base64.RawURLEncoding.EncodeToString(w.bytes())
	if _, err := Decode(s); err == nil {
		t.Error("Decode with an out-of-range size index = nil error, want an error")
	}
}

func TestDecodeRejectsUnsupportedVersion(t *testing.T) {
	w := &bitWriter{}
	w.writeBits(snapshotVersion+1, versionBits)
	s := base64.RawURLEncoding.EncodeToString(w.bytes())
	if _, err := Decode(s); err == nil {
		t.Error("Decode with an unsupported version = nil error, want an error")
	}
}

func TestDecodeRejectsGarbage(t *testing.T) {
	if _, err := Decode("not-valid-base64-???"); err == nil {
		t.Error("Decode(garbage) = nil error, want ErrInvalidSnapshot")
	}
	if _, err := Decode(""); err == nil {
		t.Error("Decode(\"\") = nil error, want an error (too short for even the header)")
	}
}

// TestRandomMover_DefaultRandWhenNil covers RandomMover's zero-value path
// (Rand left nil), which every other RandomMover test avoids by supplying
// one explicitly for determinism.
func TestRandomMover_DefaultRandWhenNil(t *testing.T) {
	g := newTestGame(t, 3, 7, 1)
	cell := RandomMover{}.Choose(g, g.Faces, 1)
	legal := false
	for _, m := range g.LegalMoves(g.Faces, 1) {
		if m == cell {
			legal = true
		}
	}
	if !legal {
		t.Fatalf("RandomMover{}.Choose (nil Rand) returned %d, not a legal move", cell)
	}
}

func mustEncodeSample(t *testing.T) string {
	t.Helper()
	g, err := NewSoloGame(LayoutInline, 3, 1, nil)
	if err != nil {
		t.Fatalf("NewSoloGame: %v", err)
	}
	s, err := g.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return s
}

func TestNewSoloGame_InvalidSizeIndex(t *testing.T) {
	if _, err := NewSoloGame(LayoutInline, -1, 1, nil); err != ErrInvalidSizeIndex {
		t.Errorf("NewSoloGame(sizeIndex=-1) = %v, want ErrInvalidSizeIndex", err)
	}
	if _, err := NewSoloGame(LayoutInline, len(Sizes), 1, nil); err != ErrInvalidSizeIndex {
		t.Errorf("NewSoloGame(sizeIndex=%d) = %v, want ErrInvalidSizeIndex", len(Sizes), err)
	}
}

func TestGameState_Player_NotFound(t *testing.T) {
	g := newTestGame(t, 0, 1, 1)
	if _, ok := g.Player(PlayerID(99)); ok {
		t.Error("Player(99) returned ok=true, want false")
	}
}

func Test_itoa(t *testing.T) {
	if got := itoa(0); got != "0" {
		t.Errorf("itoa(0) = %q, want '0'", got)
	}
	if got := itoa(42); got != "42" {
		t.Errorf("itoa(42) = %q, want '42'", got)
	}
}

type dummyStrategy struct{}

func (dummyStrategy) Choose(GameState, []uint8, PlayerID) int { return 0 }

func TestMemoryStrategy_ExplicitFallback(t *testing.T) {
	m := MemoryStrategy{Fallback: dummyStrategy{}}
	if got := m.fallback(); got != (dummyStrategy{}) {
		t.Errorf("fallback() = %v, want dummyStrategy{}", got)
	}
}

func TestMemoryStrategy_NilFallback(t *testing.T) {
	m := MemoryStrategy{}
	if _, ok := m.fallback().(RandomMover); !ok {
		t.Errorf("fallback() = %v, want RandomMover", m.fallback())
	}
}

func TestValidateSizes_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for odd cells")
		}
	}()
	validateSizes([]Size{{Width: 3, Height: 3}})
}

func TestValidateSizes_ExceedsMaxPairs(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for exceeds MaxPairs")
		}
	}()
	validateSizes([]Size{{Width: 12, Height: 12}})
}

func TestDecode_TruncatedErrors(t *testing.T) {
	valid := mustEncodeSample(t)
	raw, err := base64.RawURLEncoding.DecodeString(valid)
	if err != nil {
		t.Fatalf("DecodeString: %v", err)
	}
	for i := 1; i < len(raw); i++ {
		truncated := base64.RawURLEncoding.EncodeToString(raw[:i])
		if _, err := Decode(truncated); err == nil {
			t.Errorf("Decode(truncated len %d) succeeded, want error", i)
		}
	}
	// Also test with seed-derived mode truncated
	soloSeed, err := NewSoloGame(LayoutSeedDerived, 0, 1, []byte("secret"))
	if err != nil {
		t.Fatalf("NewSoloGame: %v", err)
	}
	encodedSeed, err := soloSeed.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	rawSeed, _ := base64.RawURLEncoding.DecodeString(encodedSeed)
	for i := 1; i < len(rawSeed); i++ {
		truncated := base64.RawURLEncoding.EncodeToString(rawSeed[:i])
		if _, err := Decode(truncated); err == nil {
			t.Errorf("Decode(truncated seed len %d) succeeded, want error", i)
		}
	}
}
