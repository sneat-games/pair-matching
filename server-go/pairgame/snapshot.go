package pairgame

import (
	"encoding/base64"
	"fmt"
	"math/bits"
)

// snapshotVersion is the wire format tag written into every solo snapshot.
// Bump it (and branch on it in Decode) if the field layout ever changes, so
// an old payload is rejected instead of silently misread.
//
// v2 is the N-player rewrite's solo-only wire format, unrelated to the old
// v1 two-player (Human vs Robot) format it replaces: v1 carried a shared
// Turn bit, a robot difficulty field, and a bounded robot-memory FIFO — all
// gone, since solo has neither a turn order nor a bot at all (see
// player.go/moves.go's doc comments and this package's PR description for
// the full rules rewrite). No migration path is needed for this bump, for
// the same reason v0->v1 needed none: the host-bot wiring that would put a
// real snapshot into a live Telegram callback_data does not exist yet
// (sneat-co/sneat-go has no reference to this module), so no v1 payload has
// ever left this repo either.
const snapshotVersion = 2

// Fixed field widths, in bits, for a solo GameState's wire encoding (see
// Encode/Decode). These do not depend on board size. cellIndexBits and
// pendingFieldBits below are NOT fixed — see their own doc comments.
const (
	versionBits    = 2
	layoutModeBits = 1
	sizeIndexBits  = 4 // up to 16 board presets; Sizes currently has 9
	seedBits       = 32
	// pairOwnerBits is 1, not 2 as the old two-player format needed: solo
	// has exactly one possible non-zero owner (the lone human player), so a
	// pair's on-wire state is fully described by a single matched/unmatched
	// bit. PairOwner itself decodes back to PlayerID 0 or 1 accordingly.
	pairOwnerBits = 1
)

// bitsForCount returns the minimum number of bits needed to represent every
// value in [0, n) — i.e. ceil(log2(n)), with bitsForCount(0 or 1) == 0.
func bitsForCount(n int) int {
	if n <= 1 {
		return 0
	}
	return bits.Len(uint(n - 1))
}

// pendingFieldBits returns the wire width for the solo player's Pending at a
// board with this many cells: enough to address any cell, plus one more
// value for the "no pending pick" sentinel (written/read as the value
// `cells` itself, one past the last real cell index) — ceil(log2(cells+1)).
func pendingFieldBits(cells int) int { return bitsForCount(cells + 1) }

// EncodedBitLen returns the exact wire size, in bits, of a solo snapshot for
// the given mode/board, BEFORE base64 — what Encode's bitWriter produces.
// Unlike the old two-player format, this does not depend on any run-time
// state (no robot memory FIFO, no per-game-length field): a solo snapshot's
// size is fully determined by (mode, sizeIndex) alone.
func EncodedBitLen(mode LayoutMode, sizeIndex int) int {
	pairs := Sizes[sizeIndex].Pairs()
	cells := Sizes[sizeIndex].Cells()

	total := versionBits + layoutModeBits + sizeIndexBits
	total += pendingFieldBits(cells)
	total += pairs * pairOwnerBits
	if mode == LayoutInline {
		total += cells * bitsForCount(pairs)
	} else {
		total += seedBits
	}
	return total
}

// EncodedByteLen is the raw (pre-base64) byte length for a given bit length.
func EncodedByteLen(bitLen int) int { return (bitLen + 7) / 8 }

// EncodedBase64Len is the callback-data character length Encode() actually
// produces for the given mode/board: base64.RawURLEncoding over
// EncodedByteLen(EncodedBitLen(...)) bytes.
func EncodedBase64Len(mode LayoutMode, sizeIndex int) int {
	return base64.RawURLEncoding.EncodedLen(EncodedByteLen(EncodedBitLen(mode, sizeIndex)))
}

// CallbackDataLimitBytes is Telegram's hard callback_data cap.
const CallbackDataLimitBytes = 64

// HostPrefixReserveBytes is how much of the 64-byte budget this engine
// reserves for what the *host* bot appends around the encoded snapshot: a
// command prefix identifying "this is a pair-matching callback" plus the
// target cell address for the specific button clicked — the same shape
// Reversi uses. This is this engine's own design margin, not a number any
// spec mandates.
const HostPrefixReserveBytes = 10

// MaxSnapshotBase64Chars is the engine's own budget for Encode()'s output,
// leaving HostPrefixReserveBytes of Telegram's 64-byte callback_data limit
// for the host's own prefix + target cell.
const MaxSnapshotBase64Chars = CallbackDataLimitBytes - HostPrefixReserveBytes

// Fits reports whether a solo snapshot at the given mode/board stays within
// MaxSnapshotBase64Chars.
func Fits(mode LayoutMode, sizeIndex int) bool {
	return EncodedBase64Len(mode, sizeIndex) <= MaxSnapshotBase64Chars
}

// isSolo reports whether g has the exact shape Encode requires: exactly one
// player, and that player is not a bot. Solo never carries a Log across the
// wire (see the package doc), so an in-memory Log — populated by whatever
// Flip call happened earlier in the same request before Encode is called —
// is not part of this check; Encode simply never writes it out.
func (g GameState) isSolo() bool {
	return len(g.Players) == 1 && !g.Players[0].IsBot
}

// Encode serialises a SOLO GameState (exactly one human player, no bot —
// see isSolo) into a compact, callback-safe string: a
// base64.RawURLEncoding bit-packed snapshot, following Reversi's precedent
// (see revplay's int64base64.go) using a real bit writer instead of
// byte-aligned fields because several of this game's fields are sub-byte.
// Decode is its exact inverse.
//
// A vs-bot or vs-humans GameState is never solo, and Encode returns
// ErrNotSoloGame for one rather than silently truncating its extra players,
// scores, or public Log to fit a transport that was never meant to carry
// them — those modes are STORED server-side; see dal4pairgame.
func (g GameState) Encode() (string, error) {
	if !g.isSolo() {
		return "", ErrNotSoloGame
	}
	cells := Sizes[g.SizeIndex].Cells()
	pending := g.Players[0].Pending

	w := &bitWriter{}
	w.writeBits(snapshotVersion, versionBits)
	w.writeBits(uint64(g.Mode), layoutModeBits)
	w.writeBits(uint64(g.SizeIndex), sizeIndexBits)

	pendSentinel := uint64(cells) // one past the last real cell index
	pendingVal := pendSentinel
	if pending >= 0 {
		pendingVal = uint64(pending)
	}
	w.writeBits(pendingVal, pendingFieldBits(cells))

	for _, o := range g.PairOwner {
		v := uint64(0)
		if o != NoPlayer {
			v = 1
		}
		w.writeBits(v, pairOwnerBits)
	}

	if g.Mode == LayoutInline {
		bpc := bitsForCount(len(g.PairOwner))
		for _, f := range g.Faces {
			w.writeBits(uint64(f), bpc)
		}
	} else {
		w.writeBits(uint64(g.Seed), seedBits)
	}

	return base64.RawURLEncoding.EncodeToString(w.bytes()), nil
}

// Decode is Encode's exact inverse: it always reconstructs the solo shape —
// a single player (PlayerID 1, IsBot false, Memory 0), Score recomputed
// from the decoded PairOwner (every matched pair belongs to that one
// player, so Score is simply how many are matched — not itself stored on
// the wire), and a nil Log (solo carries none — see the package doc).
//
// Decode does not need the LayoutSeedDerived secret: it only recovers the
// raw fields (including the public Seed), never the board layout itself —
// call FacesWith(secret) once you have a decoded GameState if you need the
// actual faces.
func Decode(s string) (GameState, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return GameState{}, fmt.Errorf("%w: %v", ErrInvalidSnapshot, err)
	}
	r := &bitReader{buf: raw}

	readField := func(n int, what string) (uint64, error) {
		v, err := r.readBits(n)
		if err != nil {
			return 0, fmt.Errorf("%w: reading %s: %v", ErrInvalidSnapshot, what, err)
		}
		return v, nil
	}

	version, err := readField(versionBits, "version")
	if err != nil {
		return GameState{}, err
	}
	if version != snapshotVersion {
		return GameState{}, fmt.Errorf("%w: unsupported version %d", ErrInvalidSnapshot, version)
	}

	modeVal, err := readField(layoutModeBits, "layout mode")
	if err != nil {
		return GameState{}, err
	}
	mode := LayoutMode(modeVal)

	sizeIdxVal, err := readField(sizeIndexBits, "size index")
	if err != nil {
		return GameState{}, err
	}
	if int(sizeIdxVal) >= len(Sizes) {
		return GameState{}, fmt.Errorf("%w: size index %d out of range", ErrInvalidSnapshot, sizeIdxVal)
	}
	sizeIndex := int(sizeIdxVal)
	cells := Sizes[sizeIndex].Cells()

	pendingVal, err := readField(pendingFieldBits(cells), "pending")
	if err != nil {
		return GameState{}, err
	}
	pending := -1
	if int(pendingVal) != cells { // cells itself is the "no pending pick" sentinel
		pending = int(pendingVal)
	}

	pairs := Sizes[sizeIndex].Pairs()
	pairOwner := make([]PlayerID, pairs)
	score := 0
	for i := range pairOwner {
		v, err := readField(pairOwnerBits, "pair owner")
		if err != nil {
			return GameState{}, err
		}
		if v != 0 {
			pairOwner[i] = PlayerID(1)
			score++
		}
	}

	g := GameState{
		SizeIndex: uint8(sizeIndex),
		Mode:      mode,
		PairOwner: pairOwner,
		Players: []Player{{
			ID:      1,
			IsBot:   false,
			Pending: pending,
			Score:   score,
		}},
	}

	if mode == LayoutInline {
		bpc := bitsForCount(pairs)
		faces := make([]uint8, cells)
		for i := range faces {
			v, err := readField(bpc, "face")
			if err != nil {
				return GameState{}, err
			}
			faces[i] = uint8(v)
		}
		g.Faces = faces
	} else {
		seedVal, err := readField(seedBits, "seed")
		if err != nil {
			return GameState{}, err
		}
		g.Seed = uint32(seedVal)
	}

	return g, nil
}
