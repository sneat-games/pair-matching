package pairgame

import "math/rand"

// Strategy picks a bot's next cell to flip, given the current state and the
// resolved board layout. A bot is a player like any other under exactly the
// same rules as a human (see Flip's doc comment) — Strategy has no special
// access: it may only use `by`'s own Pending (already reflecting the bot's
// own earlier pick this "round", if any) and GameState.Log, the same public
// information a real player watching the chat would have. The session layer
// drives a bot ONE Flip call per invocation (see the package's PR
// description on why: completing a pair deliberately takes a bot two ticks,
// which makes it visible and beatable rather than an instant, opaque snipe).
type Strategy interface {
	Choose(g GameState, faces []uint8, by PlayerID) int
}

// RandomMover picks uniformly among `by`'s currently legal cells, ignoring
// the public Log entirely. It is the Memory==0 case of the "remembers the
// last N public reveal-log entries" difficulty dial (see Player.Memory's
// doc comment): a no-memory, easy opponent. It also serves as
// MemoryStrategy's fallback whenever the log does not resolve a move
// outright.
//
// Leaving Rand nil is fine for a ONE-SHOT call (a single test assertion,
// say) but is a real footgun for a caller that invokes Choose repeatedly
// across many separate calls — e.g. driving a bot one Flip per invocation,
// which is exactly how the founder's rules require a bot to be driven (see
// Flip's doc comment): Choose falls back to a FRESH
// `rand.New(rand.NewSource(1))` on every call when Rand is nil, and
// `rand.New(rand.NewSource(1)).Intn(n)` is a deterministic function of n
// alone — so every such call picks the same relative position in that
// call's own legal-moves list, which can make a bot cycle between the same
// few cells indefinitely instead of ever finishing the board. A caller
// that calls Choose more than once across its own lifetime MUST supply a
// real, persistently-seeded Rand.
type RandomMover struct{ Rand *rand.Rand }

// Choose implements Strategy.
func (m RandomMover) Choose(g GameState, faces []uint8, by PlayerID) int {
	moves := g.LegalMoves(faces, by)
	if len(moves) == 0 {
		panic("pairgame: Choose called with no legal moves")
	}
	r := m.Rand
	if r == nil {
		r = rand.New(rand.NewSource(1)) //nolint:gosec // a casual game's move variety, not a security boundary
	}
	return moves[r.Intn(len(moves))]
}

// recentLogEntries returns the tail of g.Log a bot with this much memory
// capacity may consult: the most recent `memory` entries (all of them if
// the log is shorter), oldest first — the public-log equivalent of the old
// per-robot Memory FIFO. A stale entry (whose pair has since been claimed —
// by this bot, by another bot, or by any human) is filtered out by
// MemoryStrategy's callers below, not here, since "still unmatched" is a
// live GameState.PairOwner property Choose already has independently of
// which entries the window contains.
func recentLogEntries(g GameState, memory int) []Reveal {
	if memory <= 0 || len(g.Log) == 0 {
		return nil
	}
	start := len(g.Log) - memory
	if start < 0 {
		start = 0
	}
	return g.Log[start:]
}

// MemoryStrategy plays using the tail of the public reveal Log — the
// difficulty dial is `by`'s own Player.Memory, how many of the most
// recent public Log entries (from ANY player, human or bot — every flip is
// public per Flip's doc comment) it may consult:
//
//   - Second pick (Pending is set): if the remembered window holds another
//     still-unmatched cell whose face matches the pending pick's face,
//     take it — a guaranteed match. Otherwise fall back to Fallback.
//   - First pick (no Pending): if the remembered window holds two DISTINCT
//     cells that share a still-unmatched face — a pair the bot has seen
//     both halves of, whether from its own earlier flips or from watching
//     other players — open one of them; its second pick will then find the
//     guaranteed match above. Otherwise fall back to Fallback.
//
// A larger Memory keeps more of the public log in view, so this strategy
// finds guaranteed matches more often. At Memory == Sizes[SizeIndex].Cells()
// (or larger — recentLogEntries never returns more than exists) it can see
// the entire game's history and plays a perfect, effectively unbeatable
// game; at Memory == 0, the window is always empty and it degrades to pure
// Fallback. Every value in between is a real difficulty step, not just
// cosmetic — which is the point of exposing Memory as the difficulty
// control.
type MemoryStrategy struct {
	// Fallback is used whenever the log window does not resolve a move
	// (defaults to RandomMover{} when nil).
	Fallback Strategy
}

func (m MemoryStrategy) fallback() Strategy {
	if m.Fallback != nil {
		return m.Fallback
	}
	return RandomMover{}
}

// Choose implements Strategy.
func (m MemoryStrategy) Choose(g GameState, faces []uint8, by PlayerID) int {
	p, ok := g.Player(by)
	window := recentLogEntries(g, p.Memory)

	if ok && p.Pending >= 0 {
		pendingFace := faces[p.Pending]
		for i := len(window) - 1; i >= 0; i-- {
			e := window[i]
			if e.Cell == p.Pending || g.PairOwner[e.PairID] != NoPlayer {
				continue
			}
			if faces[e.Cell] == pendingFace {
				return e.Cell
			}
		}
		return m.fallback().Choose(g, faces, by)
	}

	seenAt := make(map[uint8]int, len(window))
	for _, e := range window {
		if g.PairOwner[e.PairID] != NoPlayer {
			continue
		}
		if first, already := seenAt[e.PairID]; already {
			if first != e.Cell {
				return e.Cell // second remembered sighting of this pair's face: open it
			}
			continue
		}
		seenAt[e.PairID] = e.Cell
	}
	return m.fallback().Choose(g, faces, by)
}
