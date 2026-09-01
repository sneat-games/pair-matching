package pairgame

import (
	"fmt"
	"reflect"
	"testing"
)

// TestEncodeDecodeRoundTrip_AllSizesAndDifficulties is the compaction pass's
// direct evidence for Change 1 (size-proportional cell indices) and Change 2
// (bounded difficulty/filled-count fields): every field must survive an
// Encode/Decode round trip at every board preset, under both layout modes,
// at N=0, N=1 (whichever of those is <= MaxDifficulty) and N=MaxDifficulty —
// the three N values REQ:state-in-callback-data's brief called out
// explicitly. It builds GameStates directly (not via Reveal) so it exercises
// the wire format itself, independent of game-play reachability, and checks
// a Pending value at both ends of pendingFieldBits' range (the sentinel and
// the largest real cell index) since that boundary is exactly where an
// off-by-one in the new width would show up.
func TestEncodeDecodeRoundTrip_AllSizesAndDifficulties(t *testing.T) {
	for sizeIdx, size := range Sizes {
		cells := size.Cells()
		pairs := size.Pairs()
		for _, mode := range []LayoutMode{LayoutInline, LayoutSeedDerived} {
			max := MaxDifficulty(mode, sizeIdx)
			if max < 0 {
				continue // never fits even N=0 (only 8x8 under LayoutInline) -- nothing to round-trip
			}
			for _, n := range dedupInts(0, 1, max) {
				if n > max {
					continue // dedupInts(0,1,max) can offer 1 when max==0
				}
				for _, pendCase := range []struct {
					name    string
					pending int
				}{
					{"NoPending", -1},
					{"PendingAtLastCell", cells - 1},
				} {
					name := fmt.Sprintf("%dx%d/%s/N=%d/%s", size.Width, size.Height, modeName(mode), n, pendCase.name)
					t.Run(name, func(t *testing.T) {
						g := GameState{
							SizeIndex: uint8(sizeIdx),
							Mode:      mode,
							Turn:      Human,
							Pending:   pendCase.pending,
							N:         n,
							PairOwner: make([]Owner, pairs),
						}
						if n%2 == 1 {
							g.Turn = Robot
						}
						for i := range g.PairOwner {
							g.PairOwner[i] = Owner(i % 3) // cycles Unmatched/Human/Robot
						}
						if n > 0 {
							g.Memory = make([]int, n)
							for i := 0; i < n; i++ {
								g.Memory[i] = i % cells // distinct-ish, always a valid cell index
							}
						}
						if mode == LayoutInline {
							g.Faces = ShuffleFaces(uint32(sizeIdx*97+n+1), pairs)
						} else {
							g.Seed = uint32(sizeIdx*104729 + n)
						}

						encoded := g.Encode()
						if len(encoded) > MaxSnapshotBase64Chars {
							t.Fatalf("encoded length %d exceeds MaxSnapshotBase64Chars %d (MaxDifficulty is supposed to guarantee this)", len(encoded), MaxSnapshotBase64Chars)
						}
						got, err := Decode(encoded)
						if err != nil {
							t.Fatalf("Decode: %v", err)
						}

						if got.SizeIndex != g.SizeIndex {
							t.Errorf("SizeIndex = %d, want %d", got.SizeIndex, g.SizeIndex)
						}
						if got.Mode != g.Mode {
							t.Errorf("Mode = %v, want %v", got.Mode, g.Mode)
						}
						if got.Turn != g.Turn {
							t.Errorf("Turn = %v, want %v", got.Turn, g.Turn)
						}
						if got.Pending != g.Pending {
							t.Errorf("Pending = %d, want %d", got.Pending, g.Pending)
						}
						if got.N != g.N {
							t.Errorf("N = %d, want %d", got.N, g.N)
						}
						if !reflect.DeepEqual(got.Memory, g.Memory) {
							t.Errorf("Memory = %v, want %v", got.Memory, g.Memory)
						}
						if !reflect.DeepEqual(got.PairOwner, g.PairOwner) {
							t.Errorf("PairOwner = %v, want %v", got.PairOwner, g.PairOwner)
						}
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
}

func modeName(mode LayoutMode) string {
	if mode == LayoutInline {
		return "inline"
	}
	return "seed"
}
