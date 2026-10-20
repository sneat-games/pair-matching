package pairsession

import (
	cryptorand "crypto/rand"
	"encoding/base32"
	"encoding/binary"
	mathrand "math/rand"
	"strings"
)

// gameIDEncoding renders a random game ID using Telegram start-parameter-
// safe characters only (Telegram restricts /start payloads to
// [A-Za-z0-9_-]). Crockford's alphabet (no padding) satisfies that and
// avoids the visually ambiguous characters a human might need to retype —
// the same choice sneat-games/greed-game's NewGameID makes.
var gameIDEncoding = base32.NewEncoding("0123456789ABCDEFGHJKMNPQRSTVWXYZ").WithPadding(base32.NoPadding)

// gameIDBytes is the amount of randomness backing a game ID: 10 bytes (80
// bits) is comfortably collision-resistant for this game's scale.
const gameIDBytes = 10

// NewGameID returns a new random, URL/deep-link-safe game ID.
func NewGameID() string {
	b := make([]byte, gameIDBytes)
	_, _ = cryptorand.Read(b)
	return strings.ToLower(gameIDEncoding.EncodeToString(b))
}

// randomSeed returns a fresh, unpredictable 32-bit seed for
// pairgame.ShuffleFaces — used at deal time (game creation for vs-Bot,
// StartGame for vs-Humans) so a stored game's board layout is not
// guessable from, say, the game ID or creation time. See the package doc
// on why a stored game always deals via ShuffleFaces (LayoutInline) rather
// than DeriveFaces (LayoutSeedDerived) — this seed is never itself secret,
// it just needs to be unpredictable at deal time.
func randomSeed() uint32 {
	var b [4]byte
	_, _ = cryptorand.Read(b[:])
	return binary.BigEndian.Uint32(b[:])
}

// newMoveRand returns a *math/rand.Rand freshly seeded from crypto/rand —
// what RobotMove passes as pairgame.RandomMover's Rand.
//
// This matters more than it looks: pairgame.RandomMover's OWN zero-value
// fallback (Rand left nil) creates `rand.New(rand.NewSource(1))` — a fixed
// seed — FRESH on every single Choose call, which was fine for that
// package's own tests (each explicitly supplies a seeded Rand, or only
// calls Choose once) but is a real production bug for a caller like
// RobotMove that calls Choose repeatedly across many separate
// RobotMove invocations: `rand.New(rand.NewSource(1)).Intn(n)` is a
// deterministic function of n alone, so every fallback-to-random call
// picks the SAME relative position in that call's legal-moves list every
// time — a bot self-playing an otherwise-empty board can cycle between
// the same two positions indefinitely instead of ever completing it. This
// was caught by TestRobotMove_RejectsAfterGameFinished failing to
// terminate within a generous move budget; see this package's PR
// description.
func newMoveRand() *mathrand.Rand {
	var b [8]byte
	_, _ = cryptorand.Read(b[:])
	return mathrand.New(mathrand.NewSource(int64(binary.BigEndian.Uint64(b[:])))) //nolint:gosec // move variety for a casual bot, not a security boundary
}
