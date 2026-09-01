package pairgame

import (
	"encoding/base64"
	"testing"
)

func TestOwnerString(t *testing.T) {
	cases := map[Owner]string{Unmatched: "unmatched", Human: "human", Robot: "robot"}
	for o, want := range cases {
		if got := o.String(); got != want {
			t.Errorf("Owner(%d).String() = %q, want %q", o, got, want)
		}
	}
}

func TestOwnerOther(t *testing.T) {
	if Human.Other() != Robot {
		t.Errorf("Human.Other() = %v, want Robot", Human.Other())
	}
	if Robot.Other() != Human {
		t.Errorf("Robot.Other() = %v, want Human", Robot.Other())
	}
}

func TestOwnerOtherPanicsOnUnmatched(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Unmatched.Other() did not panic")
		}
	}()
	Unmatched.Other()
}

func TestNewGameRejectsInvalidSizeIndex(t *testing.T) {
	for _, idx := range []int{-1, len(Sizes)} {
		if _, err := NewGame(LayoutInline, idx, 1, nil, 0, Human); err != ErrInvalidCell {
			t.Errorf("NewGame(sizeIndex=%d) = %v, want ErrInvalidCell", idx, err)
		}
	}
}

func TestNewGameRejectsOversizedDifficulty(t *testing.T) {
	sizeIndex := 7 // 6x6, where inline mode's MaxDifficulty < Cells()
	max := MaxDifficulty(LayoutInline, sizeIndex)
	if _, err := NewGame(LayoutInline, sizeIndex, 1, nil, max+1, Human); err != ErrInvalidDifficulty {
		t.Errorf("NewGame(N=%d, over MaxDifficulty=%d) = %v, want ErrInvalidDifficulty", max+1, max, err)
	}
	if _, err := NewGame(LayoutInline, sizeIndex, 1, nil, -1, Human); err != ErrInvalidDifficulty {
		t.Errorf("NewGame(N=-1) = %v, want ErrInvalidDifficulty", err)
	}
	if _, err := NewGame(LayoutInline, sizeIndex, 1, nil, max, Human); err != nil {
		t.Errorf("NewGame(N=MaxDifficulty=%d) = %v, want success", max, err)
	}
}

func TestNewGamePanicsOnInvalidFirstMover(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewGame(first=Unmatched) did not panic")
		}
	}()
	_, _ = NewGame(LayoutInline, 0, 1, nil, 0, Unmatched)
}

func TestDecodeRejectsTruncatedPayload(t *testing.T) {
	full := mustEncodeSample(t)
	// Progressively truncate the base64 string; every prefix should either
	// decode to something shorter than intended (impossible here, since we
	// truncate to well before the end) or fail with ErrInvalidSnapshot —
	// never panic.
	for cut := 1; cut < 4; cut++ {
		s := full[:cut]
		if _, err := Decode(s); err == nil {
			t.Errorf("Decode(%q) (truncated) = nil error, want an error", s)
		}
	}
}

func TestDecodeRejectsUnknownSizeIndex(t *testing.T) {
	// Hand-build a header with a size index one past the end of Sizes.
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
	w.writeBits(snapshotVersion+1, versionBits) // only version 0 exists today
	s := base64.RawURLEncoding.EncodeToString(w.bytes())
	if _, err := Decode(s); err == nil {
		t.Error("Decode with an unsupported version = nil error, want an error")
	}
}

// TestScoreCreditsTheRobot forces one mismatch (flipping the turn to Robot)
// before playing the rest of the board out as guaranteed matches, which —
// since playToCompletion never itself causes a mismatch — all get credited
// to Robot. This exercises Score's Robot-counting branch, which the
// matches-only play-outs elsewhere (Turn never leaves Human) never reach.
func TestScoreCreditsTheRobot(t *testing.T) {
	g := newTestGame(t, 2, 5, 0) // 3x4: 6 pairs, room for a real mismatch
	a, b := findMismatch(g.Faces)
	var err error
	g, _, err = g.Reveal(a, nil)
	if err != nil {
		t.Fatalf("Reveal(a): %v", err)
	}
	g, _, err = g.Reveal(b, nil) // mismatch: turn passes to Robot
	if err != nil {
		t.Fatalf("Reveal(b): %v", err)
	}
	if g.Turn != Robot {
		t.Fatalf("Turn = %v after a mismatch, want Robot", g.Turn)
	}
	g = playToCompletion(t, g) // every remaining pair matches under Turn==Robot
	human, robot := g.Score()
	if human != 0 || robot != Sizes[2].Pairs() {
		t.Fatalf("Score() = (%d, %d), want (0, %d)", human, robot, Sizes[2].Pairs())
	}
}

// TestRandomMover_DefaultRandWhenNil covers RandomMover's zero-value path
// (Rand left nil), which every other RandomMover test avoids by supplying
// one explicitly for determinism.
func TestRandomMover_DefaultRandWhenNil(t *testing.T) {
	g := newTestGame(t, 3, 7, 0)
	cell := RandomMover{}.Choose(g, g.Faces)
	legal := false
	for _, m := range g.LegalMoves(g.Faces) {
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
	g := newTestGame(t, 3, 1, 2)
	return g.Encode()
}
