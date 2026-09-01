package pairgame

import (
	"fmt"
	"reflect"
	"testing"
)

// TestSoloBudgetMatrix is the REQ:state-in-callback-data gate for the solo
// mode's snapshot, under both LayoutInline and LayoutSeedDerived, for
// every board preset. Unlike the old two-player format, a solo snapshot's
// size does not depend on any run-time state (no robot memory FIFO to
// worry about at N=0/typical/worst-case) — it is fully determined by
// (mode, sizeIndex), so this measures exactly one row per (size, mode).
// Run with `go test -v -run TestSoloBudgetMatrix` to see the full table.
func TestSoloBudgetMatrix(t *testing.T) {
	modes := []struct {
		mode LayoutMode
		name string
	}{
		{LayoutInline, "inline"},
		{LayoutSeedDerived, "seed-derived"},
	}

	t.Logf("%-8s %-6s %-12s %4s %4s %6s %5s %s",
		"size", "cells", "mode", "bits", "bytes", "b64ch", "fits", "margin-to-64")

	for sizeIdx, size := range Sizes {
		for _, m := range modes {
			bits := EncodedBitLen(m.mode, sizeIdx)
			bytes := EncodedByteLen(bits)
			b64 := EncodedBase64Len(m.mode, sizeIdx)
			fits := Fits(m.mode, sizeIdx)
			totalWithPrefix := b64 + HostPrefixReserveBytes
			t.Logf("%-8s %-6d %-12s %4d %4d %6d %5v %d/%d",
				fmt.Sprintf("%dx%d", size.Width, size.Height), size.Cells(), m.name,
				bits, bytes, b64, fits, totalWithPrefix, CallbackDataLimitBytes)

			if totalWithPrefix > CallbackDataLimitBytes && fits {
				t.Errorf("%dx%d %s: Fits()=true but b64(%d)+prefix(%d)=%d exceeds the real Telegram limit %d",
					size.Width, size.Height, m.name, b64, HostPrefixReserveBytes, totalWithPrefix, CallbackDataLimitBytes)
			}
		}
	}
}

// TestSoloAt8x8FitsComfortably is the specific measurement the brief asked
// for: an 8x8 board (the largest preset, 32 pairs) under the mode a real
// host would actually use (LayoutSeedDerived — see NewSoloGame's doc
// comment on why LayoutInline is unusable at this size) fits with room to
// spare, and is smaller than the pre-rewrite two-player engine's smallest
// possible snapshot at the same size (that engine's own
// TestSnapshotBudgetMatrix measured LayoutSeedDerived/N=0 at 8x8 needing
// more bytes than this solo format needs even before accounting for the
// N-player rewrite dropping the turn bit, the difficulty field, and the
// robot-memory FIFO entirely).
func TestSoloAt8x8FitsComfortably(t *testing.T) {
	const sizeIdx = 8 // 8x8: 32 pairs, 64 cells
	if Sizes[sizeIdx].Width != 8 || Sizes[sizeIdx].Height != 8 {
		t.Fatalf("Sizes[%d] = %+v, this test assumes it is 8x8", sizeIdx, Sizes[sizeIdx])
	}
	b64 := EncodedBase64Len(LayoutSeedDerived, sizeIdx)
	t.Logf("8x8 solo LayoutSeedDerived snapshot: %d base64 chars (budget %d, Telegram limit %d)",
		b64, MaxSnapshotBase64Chars, CallbackDataLimitBytes)
	if !Fits(LayoutSeedDerived, sizeIdx) {
		t.Fatalf("8x8 LayoutSeedDerived solo snapshot does not fit MaxSnapshotBase64Chars=%d (got %d chars)", MaxSnapshotBase64Chars, b64)
	}
	if b64 > MaxSnapshotBase64Chars/2 {
		t.Errorf("8x8 solo snapshot (%d chars) is not \"comfortably\" under budget (%d chars) — expected well under half", b64, MaxSnapshotBase64Chars)
	}
}

// TestEncodeDecodeRoundTrip_Inline proves a real solo LayoutInline snapshot
// (explicit board, a pending pick, some matched pairs) survives an
// Encode/Decode round trip byte-for-byte in every field that matters for
// gameplay.
func TestEncodeDecodeRoundTrip_Inline(t *testing.T) {
	g, err := NewSoloGame(LayoutInline, 3, 99, nil) // 4x4
	if err != nil {
		t.Fatalf("NewSoloGame: %v", err)
	}
	a, b := findPair(g.Faces)
	if _, err := Flip(&g, nil, 1, a); err != nil {
		t.Fatalf("Flip(a): %v", err)
	}
	if _, err := Flip(&g, nil, 1, b); err != nil {
		t.Fatalf("Flip(b): %v", err)
	}
	// Leave a fresh pending pick unresolved to exercise that field too.
	c, _ := findMismatch(g.Faces)
	for c == a || c == b {
		c++
		c %= len(g.Faces)
	}
	if _, err := Flip(&g, nil, 1, c); err != nil {
		t.Fatalf("Flip(c): %v", err)
	}

	encoded, err := g.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	t.Logf("encoded (%d chars): %s", len(encoded), encoded)
	if len(encoded) > MaxSnapshotBase64Chars {
		t.Errorf("encoded length %d exceeds MaxSnapshotBase64Chars %d", len(encoded), MaxSnapshotBase64Chars)
	}

	got, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	assertSameGameplayState(t, g, got)
	if !reflect.DeepEqual(got.Faces, g.Faces) {
		t.Errorf("decoded Faces = %v, want %v", got.Faces, g.Faces)
	}
}

// TestEncodeDecodeRoundTrip_SeedDerived proves the same for
// LayoutSeedDerived, including that the decoded Seed lets FacesWith
// reconstruct the identical layout given the same secret.
func TestEncodeDecodeRoundTrip_SeedDerived(t *testing.T) {
	secret := []byte("host-bot-secret")
	g, err := NewSoloGame(LayoutSeedDerived, 7, 4242, secret) // 6x6
	if err != nil {
		t.Fatalf("NewSoloGame: %v", err)
	}
	faces := g.FacesWith(secret)
	a, b := findPair(faces)
	if _, err := Flip(&g, secret, 1, a); err != nil {
		t.Fatalf("Flip(a): %v", err)
	}
	if _, err := Flip(&g, secret, 1, b); err != nil {
		t.Fatalf("Flip(b): %v", err)
	}

	encoded, err := g.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	t.Logf("encoded (%d chars): %s", len(encoded), encoded)
	if len(encoded) > MaxSnapshotBase64Chars {
		t.Errorf("encoded length %d exceeds MaxSnapshotBase64Chars %d", len(encoded), MaxSnapshotBase64Chars)
	}

	got, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	assertSameGameplayState(t, g, got)
	if got.Seed != g.Seed {
		t.Errorf("decoded Seed = %d, want %d", got.Seed, g.Seed)
	}
	if got.Faces != nil {
		t.Errorf("decoded Faces = %v, want nil under LayoutSeedDerived", got.Faces)
	}
	if !reflect.DeepEqual(got.FacesWith(secret), faces) {
		t.Errorf("FacesWith(secret) after decode does not match the original layout")
	}
}

// TestDeriveFacesWrongSecretGivesWrongLayout demonstrates the secrecy
// property end-to-end through the wire format: decoding a real snapshot
// with the wrong secret does not error (there is nothing in the payload to
// validate against) but does NOT reproduce the true layout either.
func TestDeriveFacesWrongSecretGivesWrongLayout(t *testing.T) {
	secret := []byte("real-secret")
	g, err := NewSoloGame(LayoutSeedDerived, 3, 123, secret)
	if err != nil {
		t.Fatalf("NewSoloGame: %v", err)
	}
	trueFaces := g.FacesWith(secret)

	encoded, err := g.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	guessedFaces := decoded.FacesWith([]byte("wrong-secret"))
	if reflect.DeepEqual(guessedFaces, trueFaces) {
		t.Fatal("decoding with the wrong secret reproduced the true layout — secrecy property broken")
	}
}

// TestEncodeDecodeRoundTrip_AllSizes exercises Encode/Decode at every board
// preset that fits, under both layout modes, at a NoPending state, a
// pending-at-last-cell state, and a partially-matched state — the wire
// format is otherwise state-independent (see EncodedBitLen's doc comment),
// so this is the width-boundary sweep that the old format needed a
// per-difficulty loop for.
func TestEncodeDecodeRoundTrip_AllSizes(t *testing.T) {
	for sizeIdx, size := range Sizes {
		cells := size.Cells()
		pairs := size.Pairs()
		for _, mode := range []LayoutMode{LayoutInline, LayoutSeedDerived} {
			if !Fits(mode, sizeIdx) {
				continue
			}
			for _, pendCase := range []struct {
				name    string
				pending int
			}{
				{"NoPending", -1},
				{"PendingAtLastCell", cells - 1},
			} {
				name := fmt.Sprintf("%dx%d/%s/%s", size.Width, size.Height, modeName(mode), pendCase.name)
				t.Run(name, func(t *testing.T) {
					g := GameState{
						SizeIndex: uint8(sizeIdx),
						Mode:      mode,
						PairOwner: make([]PlayerID, pairs),
						Players:   []Player{{ID: 1, Pending: pendCase.pending}},
					}
					for i := range g.PairOwner {
						if i%2 == 0 {
							g.PairOwner[i] = 1 // cycles Unmatched/Matched
							g.Players[0].Score++
						}
					}
					if mode == LayoutInline {
						g.Faces = ShuffleFaces(uint32(sizeIdx*97+1), pairs)
					} else {
						g.Seed = uint32(sizeIdx*104729 + 7)
					}

					encoded, err := g.Encode()
					if err != nil {
						t.Fatalf("Encode: %v", err)
					}
					if len(encoded) > MaxSnapshotBase64Chars {
						t.Fatalf("encoded length %d exceeds MaxSnapshotBase64Chars %d", len(encoded), MaxSnapshotBase64Chars)
					}
					got, err := Decode(encoded)
					if err != nil {
						t.Fatalf("Decode: %v", err)
					}
					assertSameGameplayState(t, g, got)
					if mode == LayoutInline {
						if !reflect.DeepEqual(got.Faces, g.Faces) {
							t.Errorf("Faces = %v, want %v", got.Faces, g.Faces)
						}
					} else if got.Seed != g.Seed {
						t.Errorf("Seed = %d, want %d", got.Seed, g.Seed)
					}
				})
			}
		}
	}
}

func modeName(mode LayoutMode) string {
	if mode == LayoutInline {
		return "inline"
	}
	return "seed"
}

func assertSameGameplayState(t *testing.T, want, got GameState) {
	t.Helper()
	if got.SizeIndex != want.SizeIndex {
		t.Errorf("SizeIndex = %d, want %d", got.SizeIndex, want.SizeIndex)
	}
	if got.Mode != want.Mode {
		t.Errorf("Mode = %v, want %v", got.Mode, want.Mode)
	}
	if len(got.Players) != 1 || len(want.Players) != 1 {
		t.Fatalf("solo state must decode to exactly 1 player: got %d, want %d", len(got.Players), len(want.Players))
	}
	if got.Players[0].Pending != want.Players[0].Pending {
		t.Errorf("Pending = %d, want %d", got.Players[0].Pending, want.Players[0].Pending)
	}
	if got.Players[0].Score != want.Players[0].Score {
		t.Errorf("Score = %d, want %d", got.Players[0].Score, want.Players[0].Score)
	}
	if !reflect.DeepEqual(got.PairOwner, want.PairOwner) {
		t.Errorf("PairOwner = %v, want %v", got.PairOwner, want.PairOwner)
	}
}
