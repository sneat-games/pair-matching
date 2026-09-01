package pairgame

import (
	"encoding/base64"
	"fmt"
	"math/bits"
)

// snapshotVersion is the wire format tag written into every Snapshot. Bump it
// (and branch on it in Decode) if the field layout ever changes, so an old
// payload is rejected instead of silently misread — this matters especially
// for LayoutSeedDerived, where DeriveFaces's derivation is itself part of
// the format.
//
// v1 (bumped from v0 by the callback-data compaction pass): cell indices
// (memory entries, the pending pick), the difficulty field and the filled
// field are no longer fixed 7-bit values — see cellIndexBits,
// pendingFieldBits, difficultyFieldBits and this file's Encode/Decode. A v0
// payload cannot be reinterpreted under these narrower, board-size-dependent
// widths, so Decode rejects anything but the current version outright rather
// than risk silently misparsing an old payload. No migration path is needed
// for this bump: the host-bot wiring that would put a real snapshot into a
// live Telegram callback_data does not exist yet (sneat-co/sneat-go has no
// reference to this module at all, and this repo has never tagged or
// released a version another repo could pin to — see the PR description for
// exactly what was checked), so no v0 payload has ever left this repo.
const snapshotVersion = 1

// Fixed field widths, in bits, for GameState's wire encoding (see
// Encode/Decode). These do not depend on board size or game state.
// cellIndexBits, pendingFieldBits, difficultyFieldBits and the per-game
// filled-count width below are NOT fixed — see their own doc comments.
const (
	versionBits    = 2
	layoutModeBits = 1
	sizeIndexBits  = 4 // up to 16 board presets; Sizes currently has 9
	turnBits       = 1
	seedBits       = 32
	pairOwnerBits  = 2 // per pair: Unmatched(0)/Human(1)/Robot(2), 3 unused
)

// bitsForCount returns the minimum number of bits needed to represent every
// value in [0, n) — i.e. ceil(log2(n)), with bitsForCount(0 or 1) == 0.
func bitsForCount(n int) int {
	if n <= 1 {
		return 0
	}
	return bits.Len(uint(n - 1))
}

// cellIndexBits returns the wire width for one robot-memory FIFO entry at a
// board with this many cells: just enough to address any cell, ceil(log2(
// cells)) — no "none" sentinel, because a memory entry's presence is already
// tracked by the separate filled-count field (see Encode/Decode), unlike the
// pending-pick field below. This is Change 1 of the callback-data compaction
// pass: cell indices used to be a fixed 7 bits everywhere (0-126, enough for
// the largest board, 8x8's 64 cells) even on a 4x4 board that only needs 4.
func cellIndexBits(cells int) int { return bitsForCount(cells) }

// pendingFieldBits returns the wire width for GameState.Pending at a board
// with this many cells: cellIndexBits(cells) worth of addresses plus one
// more value for the "no pending pick" sentinel (written/read as the value
// `cells` itself, one past the last real cell index) — ceil(log2(cells+1)).
// This is never wider, and for most board sizes is the same width, as the
// alternative the brief allowed (a separate presence bit alongside a plain
// cellIndexBits(cells)-wide value): ceil(log2(n+1)) <= ceil(log2(n))+1 always,
// with equality only when n+1 is a power of two.
func pendingFieldBits(cells int) int { return bitsForCount(cells + 1) }

// difficultyFieldEntry is one precomputed (mode, sizeIndex) row: the wire
// width used to encode GameState.N, and the MaxDifficulty ceiling that width
// was sized for. See computeDifficultyField for why the two must be solved
// together rather than picked independently.
type difficultyFieldEntry struct {
	bits int
	max  int // -1 if even N == 0 does not fit the budget
}

// difficultyTable[mode][sizeIndex] is computed once at package init by
// computeDifficultyField for every (mode, sizeIndex) pair in Sizes.
var difficultyTable = [2][]difficultyFieldEntry{}

func init() {
	for _, mode := range []LayoutMode{LayoutInline, LayoutSeedDerived} {
		row := make([]difficultyFieldEntry, len(Sizes))
		for i := range Sizes {
			bitsWidth, max := computeDifficultyField(mode, i)
			row[i] = difficultyFieldEntry{bits: bitsWidth, max: max}
		}
		difficultyTable[mode] = row
	}
}

// computeDifficultyField finds, for one (mode, sizeIndex), the fixed point of
// two interdependent quantities: the wire width used to encode N (Change 2
// of the callback-data compaction pass — N used to be a fixed 7 bits
// everywhere, even though it is already capped by MaxDifficulty), and
// MaxDifficulty itself computed AT that width. They are interdependent
// because narrowing the difficulty field frees bits that can raise the
// ceiling, and the ceiling in turn determines how narrow the field can
// safely be while still being able to represent it.
//
// This starts from the loosest safe bound — bitsForCount(cells+1), enough
// for any N up to a full-board perfect memory, since N can never physically
// exceed cells — and shrinks it a step at a time. dBits only ever decreases
// (each step recomputes it as bitsForCount(candidateMax+1), which cannot
// exceed the width just used to find that candidateMax), so this always
// terminates; the iteration cap below is a defensive assertion, not expected
// to ever fire for any preset in Sizes.
func computeDifficultyField(mode LayoutMode, sizeIndex int) (dBits, max int) {
	cells := Sizes[sizeIndex].Cells()
	dBits = bitsForCount(cells + 1)
	for iter := 0; ; iter++ {
		if iter > 64 {
			panic("pairgame: computeDifficultyField did not converge for " +
				itoa(int(mode)) + "/" + itoa(sizeIndex))
		}
		best := -1
		for n := cells; n >= 0; n-- {
			// A candidate n that dBits itself cannot represent is not a
			// legal answer regardless of whether it would otherwise fit.
			if bitsForCount(n+1) > dBits {
				continue
			}
			if fitsBits(encodedBitLenRaw(mode, sizeIndex, n, dBits)) {
				best = n
				break
			}
		}
		if best < 0 {
			return dBits, -1
		}
		want := bitsForCount(best + 1)
		if want == dBits {
			return dBits, best
		}
		dBits = want // shrink and retry: freed bits may raise the ceiling further
	}
}

// difficultyFieldBits returns the wire width used to encode GameState.N for
// this (mode, sizeIndex) — the width computeDifficultyField settled on.
func difficultyFieldBits(mode LayoutMode, sizeIndex int) int {
	return difficultyTable[mode][sizeIndex].bits
}

// fitsBits is Fits's check restated directly on a bit length, for use before
// a GameState/EncodedBitLen call makes sense (computeDifficultyField calls
// this while difficultyTable, which EncodedBitLen depends on, is still being
// built).
func fitsBits(bitLen int) bool {
	return base64.RawURLEncoding.EncodedLen(EncodedByteLen(bitLen)) <= MaxSnapshotBase64Chars
}

// EncodedBitLen returns the exact wire size, in bits, of a snapshot for the
// given mode/board/memory-fill BEFORE base64 — i.e. what Encode's bitWriter
// would produce for a GameState whose N and len(Memory) both equal `filled`
// (Fits and MaxDifficulty's worst-case query: a full memory). It does not
// require a real GameState, so it is what the budget helpers and the
// size-vs-mode survey in snapshot_test.go use to evaluate a config without
// constructing one.
func EncodedBitLen(mode LayoutMode, sizeIndex, filled int) int {
	return encodedBitLenRaw(mode, sizeIndex, filled, difficultyFieldBits(mode, sizeIndex))
}

// encodedBitLenRaw is EncodedBitLen's computation with an explicit
// difficulty-field width, so computeDifficultyField can evaluate candidate
// widths before difficultyTable (which the public EncodedBitLen reads) has
// been built. filled plays the double duty Fits/MaxDifficulty always give it
// here: both N and len(Memory) for this query.
func encodedBitLenRaw(mode LayoutMode, sizeIndex, filled, dBits int) int {
	pairs := Sizes[sizeIndex].Pairs()
	cells := Sizes[sizeIndex].Cells()

	total := versionBits + layoutModeBits + sizeIndexBits + turnBits
	total += pendingFieldBits(cells)
	total += dBits
	total += bitsForCount(filled + 1) // filled-count field: filled is in [0, N=filled] here
	total += filled * cellIndexBits(cells)
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
// produces for the given mode/board/memory-fill: base64.RawURLEncoding over
// EncodedByteLen(EncodedBitLen(...)) bytes.
func EncodedBase64Len(mode LayoutMode, sizeIndex, filled int) int {
	return base64.RawURLEncoding.EncodedLen(EncodedByteLen(EncodedBitLen(mode, sizeIndex, filled)))
}

// CallbackDataLimitBytes is Telegram's hard callback_data cap (REQ:
// state-in-callback-data).
const CallbackDataLimitBytes = 64

// HostPrefixReserveBytes is how much of the 64-byte budget this engine
// reserves for what the *host* bot appends around the encoded snapshot: a
// command prefix identifying "this is a pair-matching callback" plus the
// target cell address for the specific button clicked — the same shape
// Reversi uses (github.com/sneat-games/reversi's revplay.Render: "the host
// is expected to build the full callback_data itself, embedding s.Encode()
// and its own command prefix"). This is this engine's own design margin, not
// a number the spec mandates — the host (sneat-co/sneat-go, out of scope
// here) could get away with less.
const HostPrefixReserveBytes = 10

// MaxSnapshotBase64Chars is the engine's own budget for Encode()'s output,
// leaving HostPrefixReserveBytes of Telegram's 64-byte callback_data limit
// for the host's own prefix + target cell.
const MaxSnapshotBase64Chars = CallbackDataLimitBytes - HostPrefixReserveBytes

// Fits reports whether a snapshot at the given mode/board/memory-fill stays
// within MaxSnapshotBase64Chars.
func Fits(mode LayoutMode, sizeIndex, filled int) bool {
	return EncodedBase64Len(mode, sizeIndex, filled) <= MaxSnapshotBase64Chars
}

// MaxDifficulty returns the largest N (robot memory capacity) for which a
// FULL memory (filled == N — the worst case, reached once N reveals have
// happened) still fits the budget at the given mode/board, USING the
// difficulty field's own compacted width (difficultyFieldBits) — i.e. it is
// not a search independent of the encoding, but the same fixed point
// computeDifficultyField already solved at package init. It returns -1 if
// even N == 0 does not fit (only 8x8 under LayoutInline today — see
// snapshot_test.go). NewGame uses this to reject an unsafe N instead of
// producing a snapshot that later grows too large mid-game.
func MaxDifficulty(mode LayoutMode, sizeIndex int) int {
	return difficultyTable[mode][sizeIndex].max
}

// Encode serialises the state into a compact, callback-safe string: a
// base64.RawURLEncoding (Reversi's precedent — see revgame's
// int64base64.go and revplay.Snapshot.Encode, which this engine's
// bit-packed variant follows in spirit, using a real bit writer instead of
// byte-aligned fields because several of this game's fields are sub-byte).
// Decode is its exact inverse.
func (g GameState) Encode() string {
	cells := Sizes[g.SizeIndex].Cells()

	w := &bitWriter{}
	w.writeBits(snapshotVersion, versionBits)
	w.writeBits(uint64(g.Mode), layoutModeBits)
	w.writeBits(uint64(g.SizeIndex), sizeIndexBits)

	turnBit := uint64(0)
	if g.Turn == Robot {
		turnBit = 1
	}
	w.writeBits(turnBit, turnBits)

	pendSentinel := uint64(cells) // one past the last real cell index
	pendingVal := pendSentinel
	if g.Pending >= 0 {
		pendingVal = uint64(g.Pending)
	}
	w.writeBits(pendingVal, pendingFieldBits(cells))

	w.writeBits(uint64(g.N), difficultyFieldBits(g.Mode, int(g.SizeIndex)))
	// filledBits is bounded by N itself (Change 2: len(Memory) can never
	// exceed N — see rememberCell), not a fixed width.
	w.writeBits(uint64(len(g.Memory)), bitsForCount(g.N+1))
	cBits := cellIndexBits(cells)
	for _, m := range g.Memory {
		w.writeBits(uint64(m), cBits)
	}

	for _, o := range g.PairOwner {
		w.writeBits(uint64(o), pairOwnerBits)
	}

	if g.Mode == LayoutInline {
		bpc := bitsForCount(len(g.PairOwner))
		for _, f := range g.Faces {
			w.writeBits(uint64(f), bpc)
		}
	} else {
		w.writeBits(uint64(g.Seed), seedBits)
	}

	return base64.RawURLEncoding.EncodeToString(w.bytes())
}

// Decode is Encode's exact inverse. It does not need the LayoutSeedDerived
// secret: Decode only recovers the raw fields (including the public Seed),
// never the board layout itself — call FacesWith(secret) once you have a
// decoded GameState if you need the actual faces.
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

	turnVal, err := readField(turnBits, "turn")
	if err != nil {
		return GameState{}, err
	}
	turn := Human
	if turnVal == 1 {
		turn = Robot
	}

	pendingVal, err := readField(pendingFieldBits(cells), "pending")
	if err != nil {
		return GameState{}, err
	}
	pending := -1
	if int(pendingVal) != cells { // cells itself is the "no pending pick" sentinel
		pending = int(pendingVal)
	}

	nVal, err := readField(difficultyFieldBits(mode, sizeIndex), "difficulty")
	if err != nil {
		return GameState{}, err
	}

	// filledBits is bounded by the N just read, not a fixed width — see
	// Encode.
	filledVal, err := readField(bitsForCount(int(nVal)+1), "memory length")
	if err != nil {
		return GameState{}, err
	}
	// A nil Memory (never allocated because it has been empty since
	// creation, or emptied back out by forgetCell) and a non-nil empty one
	// are functionally identical, but reflect.DeepEqual treats them as
	// different — so decode straight back to nil at zero length rather than
	// make([]int, 0), to round-trip Reveal's actual nil-when-empty behaviour
	// (see moves.go's rememberCell/forgetCell) rather than merely a
	// same-length approximation of it.
	var memory []int
	if filledVal > 0 {
		cBits := cellIndexBits(cells)
		memory = make([]int, filledVal)
		for i := range memory {
			v, err := readField(cBits, "memory entry")
			if err != nil {
				return GameState{}, err
			}
			memory[i] = int(v)
		}
	}

	pairs := Sizes[sizeIndex].Pairs()
	pairOwner := make([]Owner, pairs)
	for i := range pairOwner {
		v, err := readField(pairOwnerBits, "pair owner")
		if err != nil {
			return GameState{}, err
		}
		pairOwner[i] = Owner(v)
	}

	g := GameState{
		SizeIndex: uint8(sizeIndex),
		Mode:      mode,
		Turn:      turn,
		Pending:   pending,
		N:         int(nVal),
		Memory:    memory,
		PairOwner: pairOwner,
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
