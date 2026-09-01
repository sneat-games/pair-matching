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
const snapshotVersion = 0

// Field widths, in bits, for GameState's wire encoding (see Encode/Decode).
// pendingBits/memoryEntryBits/difficultyBits/filledBits share one width: a
// cell index and a memory-capacity count are both bounded by the largest
// board's cell count (64, for the 8x8 preset), so 7 bits (0-126, 127
// reserved as "none") comfortably covers every preset in Sizes with room to
// grow.
const (
	versionBits     = 2
	layoutModeBits  = 1
	sizeIndexBits   = 4 // up to 16 board presets; Sizes currently has 9
	turnBits        = 1
	pendingBits     = 7
	difficultyBits  = 7 // N: robot memory FIFO capacity ("difficulty")
	filledBits      = 7 // len(Memory) actually carried this snapshot
	memoryEntryBits = 7
	seedBits        = 32
	pairOwnerBits   = 2 // per pair: Unmatched(0)/Human(1)/Robot(2), 3 unused

	pendingSentinel = (1 << pendingBits) - 1 // 127: "no pending pick"
)

// bitsForCount returns the minimum number of bits needed to represent every
// value in [0, n) — i.e. ceil(log2(n)), with bitsForCount(0 or 1) == 0.
func bitsForCount(n int) int {
	if n <= 1 {
		return 0
	}
	return bits.Len(uint(n - 1))
}

// EncodedBitLen returns the exact wire size, in bits, of a snapshot for the
// given mode/board/memory-fill BEFORE base64 — i.e. what Encode's bitWriter
// would produce. It does not require a real GameState, so it is what the
// budget helpers (Fits, MaxDifficulty) and the size-vs-mode survey in
// snapshot_test.go use to evaluate a config without constructing one.
func EncodedBitLen(mode LayoutMode, sizeIndex, filled int) int {
	pairs := Sizes[sizeIndex].Pairs()
	cells := Sizes[sizeIndex].Cells()

	total := versionBits + layoutModeBits + sizeIndexBits + turnBits +
		pendingBits + difficultyBits + filledBits
	total += filled * memoryEntryBits
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
// happened) still fits the budget at the given mode/board. It returns -1 if
// even N == 0 does not fit (never happens for any preset in Sizes today —
// see snapshot_test.go). NewGame uses this to reject an unsafe N instead of
// producing a snapshot that later grows too large mid-game.
func MaxDifficulty(mode LayoutMode, sizeIndex int) int {
	cells := Sizes[sizeIndex].Cells()
	for n := cells; n >= 0; n-- {
		if Fits(mode, sizeIndex, n) {
			return n
		}
	}
	return -1
}

// Encode serialises the state into a compact, callback-safe string: a
// base64.RawURLEncoding (Reversi's precedent — see revgame's
// int64base64.go and revplay.Snapshot.Encode, which this engine's
// bit-packed variant follows in spirit, using a real bit writer instead of
// byte-aligned fields because several of this game's fields are sub-byte).
// Decode is its exact inverse.
func (g GameState) Encode() string {
	w := &bitWriter{}
	w.writeBits(snapshotVersion, versionBits)
	w.writeBits(uint64(g.Mode), layoutModeBits)
	w.writeBits(uint64(g.SizeIndex), sizeIndexBits)

	turnBit := uint64(0)
	if g.Turn == Robot {
		turnBit = 1
	}
	w.writeBits(turnBit, turnBits)

	pendingVal := uint64(pendingSentinel)
	if g.Pending >= 0 {
		pendingVal = uint64(g.Pending)
	}
	w.writeBits(pendingVal, pendingBits)

	w.writeBits(uint64(g.N), difficultyBits)
	w.writeBits(uint64(len(g.Memory)), filledBits)
	for _, m := range g.Memory {
		w.writeBits(uint64(m), memoryEntryBits)
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

	turnVal, err := readField(turnBits, "turn")
	if err != nil {
		return GameState{}, err
	}
	turn := Human
	if turnVal == 1 {
		turn = Robot
	}

	pendingVal, err := readField(pendingBits, "pending")
	if err != nil {
		return GameState{}, err
	}
	pending := -1
	if pendingVal != pendingSentinel {
		pending = int(pendingVal)
	}

	nVal, err := readField(difficultyBits, "difficulty")
	if err != nil {
		return GameState{}, err
	}

	filledVal, err := readField(filledBits, "memory length")
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
		memory = make([]int, filledVal)
		for i := range memory {
			v, err := readField(memoryEntryBits, "memory entry")
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
		cells := Sizes[sizeIndex].Cells()
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
