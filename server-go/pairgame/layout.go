package pairgame

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"math/rand"
)

// LayoutMode selects how a Snapshot's card layout is carried on the wire.
// Both modes produce the same thing — a []uint8 of length Size.Cells(),
// each entry a face/pair id in [0, Size.Pairs()), each id appearing exactly
// twice — but they differ in cost and in whether the layout is visible to
// whoever can read the callback_data.
type LayoutMode uint8

const (
	// LayoutInline stores every cell's face id directly in the payload
	// (Size.Cells() * ceil(log2(Size.Pairs())) bits). It needs no secret and
	// no host round-trip to reconstruct, but it also means the *entire
	// board* — which card is where — is readable by anyone who can read the
	// callback_data (a third-party Telegram client, or direct Bot API
	// access; callback_data is not rendered to the user in the standard
	// Telegram UI, but it is not confidential). For a memory game, where the
	// hidden layout *is* the game, that is a full spoiler. It is also the
	// more expensive of the two modes and does not fit every board size —
	// see snapshot_test.go's measured table.
	LayoutInline LayoutMode = 0

	// LayoutSeedDerived stores only a 32-bit public seed. The actual layout
	// is derived as HMAC-SHA256(secret, version‖sizeIndex‖seed) — see
	// DeriveFaces — where secret is a bot-side value from host configuration
	// that never appears in the payload. Recomputing the layout therefore
	// requires the secret, so a party who can only read callback_data sees
	// an opaque seed, not the board. This is the recommended mode: it is
	// both the cheaper encoding (a fixed 32 bits beats an explicit layout at
	// every board size in Sizes) and the only one that keeps the layout
	// secret. See DeriveFaces's doc comment for the real trade-off this
	// introduces (secret rotation invalidates in-flight games).
	LayoutSeedDerived LayoutMode = 1
)

// shuffledFaces returns a deterministic pairing-shuffle of `pairs` face ids
// (each in [0, pairs), each appearing exactly twice) across pairs*2 cells,
// using rnd as the entropy source. Both ShuffleFaces and DeriveFaces funnel
// through this — they differ only in how rnd's seed is produced (an
// unkeyed public seed vs. an HMAC digest of a bot-side secret).
func shuffledFaces(rnd *rand.Rand, pairs int) []uint8 {
	faces := make([]uint8, pairs*2)
	for i := 0; i < pairs; i++ {
		faces[i] = uint8(i)
		faces[i+pairs] = uint8(i)
	}
	rnd.Shuffle(len(faces), func(i, j int) { faces[i], faces[j] = faces[j], faces[i] })
	return faces
}

// ShuffleFaces deterministically lays out `pairs` pairs from a public,
// unkeyed 32-bit seed. Two calls with the same (seed, pairs) always produce
// the same layout — this is what LayoutInline's explicit array is generated
// from, and it is also what a would-be cheater could reproduce themselves if
// a bare seed (no secret) were ever put on the wire in place of the explicit
// layout; that combination is deliberately not offered as a Snapshot mode
// (see LayoutSeedDerived's doc comment).
func ShuffleFaces(seed uint32, pairs int) []uint8 {
	return shuffledFaces(rand.New(rand.NewSource(int64(seed))), pairs) //nolint:gosec // deterministic by design, not a security boundary here
}

// DeriveFaces lays out `pairs` pairs from a public 32-bit seed and a
// bot-side secret that is never carried in the payload. It is the engine
// half of REQ:state-in-callback-data's "seed-derived" mode: the host (in
// sneat-co/sneat-go, out of scope for this repo) owns the secret's storage,
// rotation and lifecycle as ordinary configuration — never per-game
// persistence — and calls DeriveFaces fresh on every callback to
// reconstruct the same layout the game started with.
//
// version and sizeIndex are mixed into the HMAC input alongside seed so that
// (a) a future change to this derivation can be introduced under a new
// version without silently reinterpreting old payloads under the new rule,
// and (b) the same seed produces different layouts at different board
// sizes (no cross-size collisions).
//
// Trade-off this introduces, called out explicitly per the brief: rotating
// the secret invalidates every in-flight game (every open message's
// callback_data starts decoding into nonsense the moment the old secret is
// gone). That is acceptable under REQ:state-in-callback-data — the game
// *is* the message, there is no resume/history surface to preserve — but it
// is a real operational consequence the host owner should know before
// rotating.
func DeriveFaces(secret []byte, version, sizeIndex uint8, seed uint32) []uint8 {
	pairs := Sizes[sizeIndex].Pairs()
	mac := hmac.New(sha256.New, secret)
	var msg [6]byte
	msg[0] = version
	msg[1] = sizeIndex
	binary.BigEndian.PutUint32(msg[2:], seed)
	mac.Write(msg[:])
	digest := mac.Sum(nil)
	// The digest is 32 bytes of good entropy; the low 8 bytes are ample to
	// seed math/rand's Fisher-Yates for boards this small (<=64 cells).
	// Without `secret`, digest — and so the resulting layout — is
	// infeasible to reproduce; that unpredictability, not the shuffle
	// itself, is what keeps the layout hidden from a callback_data reader.
	derivedSeed := int64(binary.BigEndian.Uint64(digest[:8]))
	return shuffledFaces(rand.New(rand.NewSource(derivedSeed)), pairs) //nolint:gosec // seeded from an HMAC digest, not used as the security primitive itself
}
