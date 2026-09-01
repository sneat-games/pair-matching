---
format: https://specscore.md/feature-specification
status: Draft
---

# Feature: Telegram Pair-Matching bot (vs robot, state in callback data)

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/sneat-games/pair-matching/spec/features/telegram-pair-matching-bot?op=explore) | [Edit](https://specscore.studio/app/github.com/sneat-games/pair-matching/spec/features/telegram-pair-matching-bot?op=edit) | [Ask question](https://specscore.studio/app/github.com/sneat-games/pair-matching/spec/features/telegram-pair-matching-bot?op=ask) | [Request change](https://specscore.studio/app/github.com/sneat-games/pair-matching/spec/features/telegram-pair-matching-bot?op=request-change) |
**Status:** Draft
**Source Ideas:** —

## Summary

Play **Pair-Matching** (a memory/concentration game) against a robotic opponent
inside a single Telegram chat, with the robot's difficulty controlled by how many
recently-seen cards it remembers. The **complete game state — including the
robot's memory — is carried in each button's `callback_data`**, so there is no
server-side game storage. All rules, the card layout, and the robot's strategies
come from this repo's `server-go/pairgame` engine; the bot layer holds no game
logic. v1 is single-player vs the robot; the play surface is designed to be
reused by any Sneat bot (first consumer: SneatBot's `/games`, per
`sneat-co/backstage`'s `spec/features/games/README.md`).

This document follows the [SpecScore feature specification](https://specscore.md/feature-specification).

## Problem

Pair-Matching already exists here as a standalone 2018 Google App Engine app
(`prizarena/pair-matching`), storing board and player state in a `strongo/db`
datastore behind `strongo/bots-framework`-era bot wiring. That entire delivery
and persistence stack — `pairbot`, `pairapp`, `pairgaeroot`, `pairsecrets`,
`pairdal`/`pairgaedal`, `pairtrans`, and the `google.golang.org/appengine/
datastore`-tagged `pairmodels` structs — is dead: it depends on archived
predecessor packages, and more fundamentally it assumes a server-side game
record, which `sneat-co/backstage`'s games portfolio feature explicitly rules
out (`spec/features/games/README.md`'s REQ:state-in-callback-data). Reviving
that deployment is explicitly **not** what "revive" means here.

What is worth keeping is the 2018 game's *proven logic* — pairing/shuffle and
match-detection — which server-go/pairgame's rewrite carries forward, adapted to
a stateless, callback-data-only model with no `players[]` array (v1 is solo vs
one robot, not the original's N-player race) and no datastore coupling at all.

The genuinely new problem this Feature's engine had to solve, that Reversi's
callback-data precedent (`sneat-games/reversi`) did not: **a memory game's
hidden information is the game.** Reversi has no secret state, so an inline
board (its bitboards, in the clear) is fine. Pair-Matching's card layout must
stay hidden from anyone who can read `callback_data` (not shown in Telegram's
standard UI, but not confidential either — a third-party client or direct Bot
API access can read it), or the "game" is just clicking through a spoiled
board. See REQ:seed-derived-layout below for how the engine solves this.

## Behavior

### Engine reuse

#### REQ: engine-is-single-source-of-rules

All Pair-Matching rules — the card layout, reveal/match/mismatch resolution,
turn handling, completion, scoring, and every robot strategy — MUST come from
the `server-go/pairgame` engine (`GameState`, `Reveal`, `Strategy`
implementations). The bot/rendering layer MUST NOT re-implement or duplicate
any game rule; per `sneat-co/backstage`'s REQ:engine-per-repo, that host layer
lives entirely in `sneat-co/sneat-go`, outside this repo.

### Card layout & secrecy

#### REQ: seed-derived-layout

The card layout MUST be carried as `pairgame.LayoutSeedDerived`: a 32-bit public
seed in the snapshot, with the actual per-cell layout derived on demand via
`pairgame.DeriveFaces(secret, version, sizeIndex, seed)` — a keyed HMAC-SHA256
derivation using a bot-side secret from host configuration that is never itself
carried in `callback_data`. `pairgame.LayoutInline` (the explicit per-cell array)
MUST NOT be used for a live game: it is kept in the engine only as the
losing-alternative baseline that `server-go/pairgame/snapshot_test.go`'s budget
survey measured against, because an inline layout is fully readable by anyone
who can read the button's `callback_data` — a full spoiler for a memory game —
and because it costs more bytes at every board size (see
REQ:callback-data-budget). Rotating the host's secret is an accepted
consequence: it invalidates every in-flight game's `callback_data` the moment
the old secret is gone, which is fine under REQ:no-server-persistence (the game
*is* the message; there is no resume surface to preserve across a rotation).

### Board sizes & difficulty

#### REQ: supported-board-sizes

New games MUST offer only the presets in `pairgame.Sizes` (2x2 through 8x8), not
arbitrary dimensions, so a board shape is addressable on the wire by a small
index. Starting a new game MUST let the human choose both the board size and
the robot's difficulty `N` (see REQ:memory-difficulty), and `NewGame` MUST
reject an `N` that would not fit REQ:callback-data-budget at that size rather
than silently producing a game that could outgrow the budget mid-play.

#### REQ: memory-difficulty

The robot's difficulty MUST be a single tunable parameter `N`: the capacity of a
FIFO of the most recently revealed, still-unresolved cells — remembered as cell
*indices* only, never card faces (a face is always re-derivable from the
layout, so remembering "cell 7" costs the same regardless of how many distinct
faces the board has). `N == 0` MUST play a uniformly random legal move
(`pairgame.RandomMover`, an easy/no-memory opponent). `N ==` the board's cell
count MUST play a perfect-memory, effectively unbeatable game
(`pairgame.MemoryStrategy` never has to forget anything). Values in between
MUST be real difficulty steps, not cosmetic — `MemoryStrategy` uses whatever is
still in memory to take a guaranteed match when it can and falls back to random
otherwise, so a larger `N` measurably finds more guaranteed matches.

#### REQ: robot-memory-in-callback-data

The robot's memory FIFO (its `N` and its currently-held cell indices) MUST be
carried in `callback_data` alongside the rest of the game state, per
`sneat-co/backstage`'s REQ:state-in-callback-data — it is game state, not
server-side persistence, so keeping the bot stateless requires it to live in
the payload like everything else. `Reveal` MUST update it on every successful
reveal (by either side) and MUST prune a cell out of memory once its pair
resolves, so a completed pair's slot does not waste FIFO capacity.

### State in callback data

#### REQ: callback-data-budget

Every board button's `callback_data` MUST encode the complete game snapshot —
layout mode, board size, seed, turn, pending pick, per-pair ownership, and the
robot's `N` + memory — using `GameState.Encode()`'s bit-packed
`base64.RawURLEncoding`, following `sneat-games/reversi`'s `int64base64.go`
precedent (a real bit writer is used here rather than reversi's byte-aligned
fields, because several of this game's fields are sub-byte). The **worst case**
(memory FIFO full, `filled == N`) MUST stay within Telegram's 64-byte
`callback_data` limit after reserving room for the host's own command prefix
and target-cell address (`pairgame.HostPrefixReserveBytes`, mirroring how
Reversi's host embeds its own prefix around `Snapshot.Encode()`) — not merely
the common case. `GameState.Decode()` MUST be `Encode()`'s exact inverse.

#### REQ: no-server-persistence

The bot MUST NOT persist game state server-side (no datastore/Firestore
records, no per-chat "current game" blob — and, per REQ:engine-per-repo,
`server-go/pairgame` has no such storage at all). Board and robot-memory state
live only in the rendered message's `callback_data`. Accepted v1 consequences:
no move history survives beyond the current snapshot, no "resume"/history
surface, no per-player lock on a shared or forwarded message, and no
concurrency guard.

### Playing a move

#### REQ: reveal-then-resolve

Tapping a legal (unmatched, not-currently-pending) cell MUST call
`GameState.Reveal`. A first pick MUST record it as pending and change nothing
else. A second pick MUST resolve against the pending pick: a match MUST credit
the pair to whoever is on turn and leave the turn unchanged (the matching side
goes again); a mismatch MUST clear the pending pick and pass the turn. After a
human's move resolves (whether by mismatch passing the turn, or a match handing
the human another pick), if it is now the robot's turn the host MUST drive the
robot's own reveal(s) via its configured `pairgame.Strategy`
(`RandomMover` or `MemoryStrategy`) before re-rendering, exactly as `Reveal`
would process a human tap.

#### REQ: illegal-move-noop

Tapping an already-matched cell or the already-pending cell MUST leave the
board state unchanged and re-render the same board (`ErrCellAlreadyMatched` /
`ErrCellIsPending`); no robot move is triggered.

#### REQ: game-over

Once every pair is matched, the board MUST render as finished — showing each
side's final pair count (`GameState.Score`) and the winner (or a tie) — with a
button to start a new game. Tapping any cell on a finished board MUST be a
no-op (`ErrGameOver`).

## Architecture & Components

- **`pairgame` engine** (existing, `server-go/pairgame`) — kept as the sole
  source of rules, layout derivation, and robot strategies. Its public API used
  by the play layer: `Sizes`, `NewGame`, `GameState.Reveal`, `GameState.Encode`
  / `Decode`, `GameState.FacesWith`, `GameState.Score`, `RandomMover`,
  `MemoryStrategy`.
- **Pair-Matching play layer** (this repo, not yet built — out of scope for the
  engine work this Feature currently describes) — would render a `GameState` to
  a `bots-go-core/botkb` inline keyboard and own nothing but that rendering, the
  same shape as `sneat-games/reversi`'s `revplay` package.
- **Host-bot wiring** (out of this repo — SneatBot in `sneat-co/sneat-go`,
  WIRE-AND-CONFIGURE-ONLY per that repo's own constitution) — registers the
  `botsfw.Command`, owns and rotates the `DeriveFaces` secret as ordinary
  configuration, calls the play layer, and returns its
  `botmsg.MessageFromBot`. This Feature's engine work does not touch
  `sneat-co/sneat-go`.

## Testing strategy

- Keep `pairgame`'s engine tests green (rules, layout, robot strategies already
  covered — see `server-go/pairgame/*_test.go`).
- `TestSnapshotBudgetMatrix` (`server-go/pairgame/snapshot_test.go`) is the
  living measurement behind REQ:callback-data-budget: it reports, per board
  size and layout mode, the actual `Encode()` length at `N ∈ {0, 4, cells}` and
  fails if a config `Fits()` claims true but the real prefixed total would
  exceed 64 bytes. Re-run it whenever a field width changes.
- `TestEncodeDecodeRoundTrip_*` prove `Encode`/`Decode` round-trip every field
  that matters for gameplay, for both layout modes.
- `TestDeriveFacesRequiresTheSecret` / `TestDeriveFacesWrongSecretGivesWrongLayout`
  are the secrecy property behind REQ:seed-derived-layout: decoding with the
  wrong secret must not reproduce the true layout.
- When the play layer is built, add: a new-game test asserting the chosen
  size/N round-trip into the first rendered snapshot; a legal-tap test proving
  exactly the engine's `Reveal` outcome is reflected; an illegal-tap no-op
  test; and a robot-turn test proving the configured `Strategy` drives the
  reply move(s).

## Not Doing / Out of Scope (v1)

- Player-vs-player (two Telegram users), invites, matchmaking, cross-chat turn
  notifications — per `sneat-co/backstage`'s REQ:solo-vs-robot-v1.
- Move history / transcript beyond the current snapshot; "resume game" / "my
  games" lists; a per-player lock on a shared or forwarded game message — per
  REQ:no-server-persistence.
- `pairgame.LayoutInline` in a live game (kept only as the engine's measured
  losing alternative — see REQ:seed-derived-layout).
- The play layer (keyboard rendering) and the SneatBot host wiring itself —
  this Feature's current scope is the engine + its callback-data encoding;
  wiring is a separate, later step in `sneat-co/sneat-go`.
- A UI for choosing difficulty `N` beyond "the host offers some choices" — the
  specific presented tiers (e.g. Easy/Medium/Hard labels mapped to N values)
  are a host/product decision, not an engine one.

## Acceptance Criteria

### AC: rules-come-from-engine
**Requirements:** telegram-pair-matching-bot#req:engine-is-single-source-of-rules

**Given** the Pair-Matching bot play layer
**When** a reveal is validated, applied, scored, or a robot reply is computed
**Then** it is done by calling the `pairgame` engine, and the play layer
contains no independent match-detection, turn-resolution, or scoring logic.

### AC: layout-is-not-inline-in-a-live-game
**Requirements:** telegram-pair-matching-bot#req:seed-derived-layout

**Given** a new game is started
**When** its snapshot is inspected
**Then** its `Mode` is `LayoutSeedDerived`, `Faces` is nil, and only a 32-bit
`Seed` is present — the per-cell layout is absent from the payload and can only
be reconstructed by calling `DeriveFaces` with the host's secret.

### AC: difficulty-selects-a-real-strategy
**Requirements:** telegram-pair-matching-bot#req:memory-difficulty

**Given** a chosen difficulty `N`
**When** the robot is asked to move with `N == 0`, a mid-range `N`, and
`N ==` the board's cell count
**Then** `N == 0` never takes a guaranteed match it could not know about,
`N ==` cell count always takes a guaranteed match when the board's history
makes one available, and a mid-range `N` does so only when the relevant cells
are still within its remembered window.

### AC: robot-memory-round-trips-in-callback-data
**Requirements:** telegram-pair-matching-bot#req:robot-memory-in-callback-data

**Given** a game where the robot has partially filled its memory FIFO
**When** the snapshot is encoded into `callback_data` and then decoded
**Then** the decoded `N` and the exact remembered cell indices equal the
originals, and no separate server-side record was consulted to reconstruct
them.

### AC: worst-case-snapshot-fits-the-budget
**Requirements:** telegram-pair-matching-bot#req:callback-data-budget

**Given** every board preset in `pairgame.Sizes`, at its `MaxDifficulty` for
`LayoutSeedDerived` (the worst case: memory FIFO full)
**When** the snapshot is encoded
**Then** the encoded `callback_data`, plus `HostPrefixReserveBytes` reserved for
the host's own prefix and target-cell address, is at most 64 bytes — measured
by `TestSnapshotBudgetMatrix`, not estimated.

### AC: no-records-written
**Requirements:** telegram-pair-matching-bot#req:no-server-persistence

**Given** a full Pair-Matching game played from start to finish in a chat
**When** the game runs
**Then** no game-state records are written to any datastore and no per-chat
game blob is stored — the state, including the robot's memory, exists only in
the message's `callback_data`.

### AC: match-keeps-turn-mismatch-passes-it
**Requirements:** telegram-pair-matching-bot#req:reveal-then-resolve

**Given** a pending first pick
**When** the second pick is revealed
**Then** a matching pick credits the pair to whoever was on turn and leaves the
turn unchanged, and a non-matching pick clears the pending pick and passes the
turn to the other side.

### AC: illegal-tap-is-a-noop
**Requirements:** telegram-pair-matching-bot#req:illegal-move-noop

**Given** a board with at least one already-matched pair
**When** an already-matched cell, or the currently-pending cell, is tapped
again
**Then** the returned state is unchanged from before the tap and no robot move
is triggered.

### AC: finished-board-shows-result-and-blocks-further-taps
**Requirements:** telegram-pair-matching-bot#req:game-over

**Given** every pair on the board has been matched
**When** the board is rendered, and separately when any cell is tapped again
**Then** it renders each side's final pair count and the winner (or a tie) with
a "start a new game" affordance, and the tap returns `ErrGameOver` without
changing state.

## Open Questions

- Which specific `N` values the host should expose as named difficulty tiers
  (e.g. Easy/Medium/Hard) is a product decision for whoever builds the
  SneatBot wiring in `sneat-co/sneat-go` — not decided here. `pairgame` only
  guarantees that any `N` up to `MaxDifficulty(mode, sizeIndex)` is safe to
  offer.

---
*This document follows the https://specscore.md/feature-specification*
