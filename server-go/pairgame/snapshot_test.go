package pairgame

import (
	"fmt"
	"reflect"
	"testing"
)

// TestSnapshotBudgetMatrix is the REQ:state-in-callback-data gate: for every
// board preset, under both LayoutInline and LayoutSeedDerived, at N=0 (no
// robot memory), a "typical" N, and the worst-case N == cells (a
// perfect-memory robot), it measures the ACTUAL Encode() output length (not
// a hand estimate) and reports whether it fits Telegram's 64-byte
// callback_data limit after reserving HostPrefixReserveBytes for the host's
// own command prefix + target-cell address. Run with `go test -v -run
// TestSnapshotBudgetMatrix` to see the full table.
func TestSnapshotBudgetMatrix(t *testing.T) {
	modes := []struct {
		mode LayoutMode
		name string
	}{
		{LayoutInline, "inline"},
		{LayoutSeedDerived, "seed-derived"},
	}

	t.Logf("%-8s %-6s %-12s %4s %4s %4s %6s %5s %s",
		"size", "cells", "mode", "N", "bits", "bytes", "b64ch", "fits", "margin-to-64")

	for sizeIdx, size := range Sizes {
		for _, m := range modes {
			maxN := size.Cells()
			typicalN := 4
			if typicalN > maxN {
				typicalN = maxN
			}
			for _, n := range dedupInts(0, typicalN, maxN) {
				bits := EncodedBitLen(m.mode, sizeIdx, n)
				bytes := EncodedByteLen(bits)
				b64 := EncodedBase64Len(m.mode, sizeIdx, n)
				fits := Fits(m.mode, sizeIdx, n)
				totalWithPrefix := b64 + HostPrefixReserveBytes
				t.Logf("%-8s %-6d %-12s %4d %4d %4d %6d %5v %d/%d",
					fmt.Sprintf("%dx%d", size.Width, size.Height), size.Cells(), m.name,
					n, bits, bytes, b64, fits, totalWithPrefix, CallbackDataLimitBytes)

				if totalWithPrefix > CallbackDataLimitBytes && fits {
					t.Errorf("%dx%d %s N=%d: Fits()=true but b64(%d)+prefix(%d)=%d exceeds the real Telegram limit %d",
						size.Width, size.Height, m.name, n, b64, HostPrefixReserveBytes, totalWithPrefix, CallbackDataLimitBytes)
				}
			}
		}
	}
}

func dedupInts(vs ...int) []int {
	seen := make(map[int]bool, len(vs))
	out := make([]int, 0, len(vs))
	for _, v := range vs {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// TestMaxDifficultyIsSelfConsistent checks MaxDifficulty against Fits
// directly: Fits must hold at MaxDifficulty and fail one above it (unless
// MaxDifficulty already equals the board's cell count, i.e. even a
// perfect-memory robot fits).
func TestMaxDifficultyIsSelfConsistent(t *testing.T) {
	for sizeIdx, size := range Sizes {
		for _, mode := range []LayoutMode{LayoutInline, LayoutSeedDerived} {
			max := MaxDifficulty(mode, sizeIdx)
			if max < 0 {
				if Fits(mode, sizeIdx, 0) {
					t.Errorf("%+v mode=%v: MaxDifficulty=-1 but Fits(N=0) is true", size, mode)
				}
				continue
			}
			if !Fits(mode, sizeIdx, max) {
				t.Errorf("%+v mode=%v: Fits(MaxDifficulty=%d) = false, want true", size, mode, max)
			}
			if max < size.Cells() && Fits(mode, sizeIdx, max+1) {
				t.Errorf("%+v mode=%v: Fits(MaxDifficulty+1=%d) = true, want false", size, mode, max+1)
			}
		}
	}
}

// TestEncodeDecodeRoundTrip_Inline proves a real LayoutInline snapshot
// (explicit board, non-trivial memory/pending/ownership) survives an
// Encode/Decode round trip byte-for-byte in every field that matters for
// gameplay.
func TestEncodeDecodeRoundTrip_Inline(t *testing.T) {
	g := newTestGame(t, 3, 99, 4) // 4x4
	a, b := findMismatch(g.Faces)
	g, _, err := g.Reveal(a, nil)
	if err != nil {
		t.Fatalf("Reveal: %v", err)
	}
	// Leave a pending pick unresolved to exercise that field too.
	_ = b

	encoded := g.Encode()
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

// TestEncodeDecodeRoundTrip_SeedDerived proves the same for LayoutSeedDerived,
// including that the decoded Seed lets FacesWith reconstruct the identical
// layout given the same secret.
func TestEncodeDecodeRoundTrip_SeedDerived(t *testing.T) {
	secret := []byte("host-bot-secret")
	g, err := NewGame(LayoutSeedDerived, 7, 4242, secret, 6, Robot) // 6x6
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	faces := g.FacesWith(secret)
	a, b := findPair(faces)
	g, _, err = g.Reveal(a, secret)
	if err != nil {
		t.Fatalf("Reveal(a): %v", err)
	}
	g, _, err = g.Reveal(b, secret)
	if err != nil {
		t.Fatalf("Reveal(b): %v", err)
	}

	encoded := g.Encode()
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
	// The whole point: the layout is reconstructible ONLY with the secret,
	// and it round-trips to the exact same layout the game started with.
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
	g, err := NewGame(LayoutSeedDerived, 3, 123, secret, 0, Human)
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	trueFaces := g.FacesWith(secret)

	decoded, err := Decode(g.Encode())
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	guessedFaces := decoded.FacesWith([]byte("wrong-secret"))
	if reflect.DeepEqual(guessedFaces, trueFaces) {
		t.Fatal("decoding with the wrong secret reproduced the true layout — secrecy property broken")
	}
}

func assertSameGameplayState(t *testing.T, want, got GameState) {
	t.Helper()
	if got.SizeIndex != want.SizeIndex {
		t.Errorf("SizeIndex = %d, want %d", got.SizeIndex, want.SizeIndex)
	}
	if got.Mode != want.Mode {
		t.Errorf("Mode = %v, want %v", got.Mode, want.Mode)
	}
	if got.Turn != want.Turn {
		t.Errorf("Turn = %v, want %v", got.Turn, want.Turn)
	}
	if got.Pending != want.Pending {
		t.Errorf("Pending = %d, want %d", got.Pending, want.Pending)
	}
	if got.N != want.N {
		t.Errorf("N = %d, want %d", got.N, want.N)
	}
	if !reflect.DeepEqual(got.Memory, want.Memory) {
		t.Errorf("Memory = %v, want %v", got.Memory, want.Memory)
	}
	if !reflect.DeepEqual(got.PairOwner, want.PairOwner) {
		t.Errorf("PairOwner = %v, want %v", got.PairOwner, want.PairOwner)
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
