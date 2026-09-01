---
format: https://specscore.md/feature-specification
status: Draft
---

# Feature: Pair Matching rules engine

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/sneat-games/pair-matching/spec/features/pair-matching-rules?op=explore) | [Edit](https://specscore.studio/app/github.com/sneat-games/pair-matching/spec/features/pair-matching-rules?op=edit) | [Ask question](https://specscore.studio/app/github.com/sneat-games/pair-matching/spec/features/pair-matching-rules?op=ask) | [Request change](https://specscore.studio/app/github.com/sneat-games/pair-matching/spec/features/pair-matching-rules?op=request-change) |
**Status:** Draft
**Source Ideas:** pair-matching-game

## Summary

The pairgame rules engine: flip resolution (no turn order, independent per-player pending picks, any-player sniping credited to the flipper), board-layout derivation and secrecy, the Solo snapshot codec, and the robot strategies shared identically by Solo, vs Bot, and vs Humans.

## Problem

An earlier draft of this rewrite imposed strict alternating turns and let only
the flipper complete their own pending pick — neither of which the archived
2018 original (`prizarena/pair-matching`, branch `archive/2018-original-engine`,
commit `9f2a078`) ever had. That draft's own doc comments flagged the
divergence as "a real rules difference the founder should confirm, not
something silently dropped." The founder has since confirmed, twice, that the
original's actual mechanic — no turn order, and a flip that can snipe *any*
seated player's exposed pending pick, crediting the flipper — is what shipped
in 2018 and what should ship again. Every mode this game offers (Solo, vs Bot,
vs Humans, per `sneat-co/backstage`'s `spec/features/games/README.md`) needs
this same mechanic, plus the board-layout secrecy and the Solo callback-data
codec that make Solo possible at all, defined exactly once so no consuming
layer — this repo's own `pairsession`, or the out-of-repo bot wiring — has to
re-derive or duplicate it. This document follows the
[SpecScore feature specification](https://specscore.md/feature-specification).

## Behavior

### Flip resolution (identical across every mode)

#### REQ: no-turn-order

There MUST be no turn order. Any seated player MAY call `Flip` at any moment;
a faster player having the advantage is the intended design, not a defect,
and matches the archived 2018 original.

#### REQ: independent-pending-picks

Each player MUST hold their own independent pending pick (`Player.Pending`).
Two different players MAY hold the same cell pending at the same time — that
is explicitly allowed, not a conflict to resolve.

#### REQ: flip-matches-any-pending-credits-flipper

A flip's cell MUST be checked against every seated player's pending pick,
including the flipper's own, not only the flipper's. If any player holds a
pending pick on the flipped cell's pair partner, the pair MUST be marked
matched and credited to the FLIPPER — never to the player whose pending pick
was matched. On a match, every seated player whose pending pointed at either
of the pair's two cells MUST be cleared to "none," not only the one credited.
This is the founder-restored 2018 behavior (see
`conformance_2018_open_test.go`'s step 6, a player sniping another's exposed
card) and is what makes exposing a card genuinely risky in every mode,
including vs a bot.

#### REQ: mismatch-replaces-own-pending

When a flip does not match any pending pick, the flipped cell MUST become the
flipper's new pending pick, replacing whatever they had pending before. There
is no separate "first pick" vs "second pick" state — every flip is this same
check-then-set operation.

#### REQ: matched-cell-immutable

A cell belonging to an already-matched pair MUST NOT be flippable by anyone
(`ErrCellAlreadyMatched`), checked before any pending comparison. This alone
is what makes a stale pending pick self-correcting: no separate staleness
check exists or is needed.

#### REQ: own-pending-flip-rejected

Flipping the exact cell that is already the flipper's OWN pending pick MUST
be rejected (`ErrCellIsPending`). This does not extend to another player
holding the same cell pending at the same time — that is a completely
ordinary flip, resolved like any other.

### Shared memory & game end

#### REQ: public-reveal-log

Every successful flip — matched or not — MUST be appended to an append-only
public `Log`, recording who flipped, which cell, its pair id, and whether it
matched. This log is the game's sole shared memory: what a bot's `Strategy`
may read, and what a render layer replays as messages so human players can
remember what has been opened.

#### REQ: completion-and-scoring

The game MUST end once every pair is matched (`IsComplete`). A player's score
is the count of pairs they have personally been credited with. The player(s)
with the highest score win (`Winners`); more than one entry is a tie, a
normal, fully representable outcome, not an edge case to special-case away.

### Bot fairness

#### REQ: bot-plays-by-identical-rules

A bot seat MUST be resolved through the exact same `Flip` function as a human
seat, with no special access: a `Strategy` may only consult its own player's
`Pending` and the public `Log` — the same information a real player watching
the chat would have. A bot differs from a human only in *how* it acts (driven
on a timer rather than in response to a tap), never in what it is allowed to
see or do, and it MUST be driven one `Flip` call per invocation — a single
call can complete a pair outright (sniping an exposed card, or completing the
bot's own already-open pick), so "two ticks to complete a pair" is not
guaranteed.

### Board layout & secrecy

#### REQ: two-layout-modes

A board layout MUST be carried as one of exactly two `LayoutMode`s:
`LayoutInline` (the explicit per-cell face/pair-id array, on the wire in the
clear) or `LayoutSeedDerived` (a 32-bit public seed plus a keyed derivation).
Both MUST produce the same shape: a `[]uint8` of length `Size.Cells()`, each
entry a pair id in `[0, Size.Pairs())` appearing exactly twice.

#### REQ: seed-derived-layout-is-pure-and-uncached

`DeriveFaces(secret, version, sizeIndex, seed)` MUST be a pure function,
recomputed on every call from its arguments alone — nothing about a derived
layout is ever cached in memory or persisted anywhere. `version` and
`sizeIndex` MUST be mixed into the derivation so a future format change is
introduced under a new version rather than silently reinterpreting old
payloads, and so the same seed never collides across board sizes.

#### REQ: solo-secret-from-env

The secret backing `LayoutSeedDerived` MUST come from the
`PAIR_MATCHING_GAME_SECRET` environment variable, provisioned as ordinary
host configuration (CI and Cloud Run) — never as per-game persistence. It is
a **Solo-only** concern: the stored modes (vs Bot, vs Humans) always deal a
fresh random seed under `LayoutInline` and never touch this secret (see
`pair-matching-sessions`). Rotating the secret is an accepted consequence: it
invalidates every in-flight Solo game's `callback_data` the moment the old
secret is gone, which is acceptable because the game *is* the message — there
is no resume surface to preserve across a rotation.

#### REQ: unmatched-pair-id-never-leaves-server

**SECURITY.** An unmatched cell's pair id MUST NOT appear in anything handed
to a client — callback data or a rendered message — before that cell has
actually been flipped. Callback data and rendered messages are delivered to
the client, so leaking an unmatched pair id would hand a third-party Telegram
client (or direct Bot API access) the entire solution. This is the reason
`LayoutSeedDerived` exists at all for Solo, and it binds the stored modes'
view/render layer identically even though they never touch the HMAC secret
(see `pair-matching-sessions`' `REQ: view-exposes-only-revealed-cells`).

### Solo state in callback data

#### REQ: solo-only-codec

`GameState.Encode`/`Decode` (wire format `snapshotVersion = 2`) MUST round-trip
only the Solo shape: exactly one human player, no bot, and no `Log` (Solo has
no shared-memory feed to carry — a lone player already sees their own board
directly). `Encode` MUST return `ErrNotSoloGame` for any other shape rather
than silently truncating extra players, scores, or the log to fit a transport
that was never meant to carry them.

#### REQ: solo-board-size-gated-by-budget

`NewSoloGame` MUST reject (`ErrSoloBoardTooLarge`) any `(mode, sizeIndex)`
combination whose `Encode()` output would not fit
`MaxSnapshotBase64Chars` — Telegram's 64-byte `callback_data` limit less
`HostPrefixReserveBytes` reserved for the host's own command prefix and
target-cell address — rather than letting a caller start a game that could
never be encoded back into a callback button. Measured
(`TestSoloBudgetMatrix`): 8x8 `LayoutSeedDerived` fits at 14 base64
characters (10 raw bytes before encoding), comfortably under budget; 8x8
`LayoutInline` does not fit (62 base64 characters) and is rejected.

### Robot difficulty

#### REQ: memory-window-difficulty

A bot's difficulty MUST be a single dial, `Player.Memory`: how many of the
most recent public `Log` entries — from any player, human or bot — it may
consult when choosing a move. `Memory == 0` MUST play a uniformly random
legal move (`RandomMover`), consulting the log not at all.

#### REQ: memory-strategy-prefers-sniping-then-pairing-then-random

`MemoryStrategy` MUST check, in this order, before choosing a cell: (1) a
live, still-unmatched exposure in its remembered window that some seated
player (including itself) currently has a pending pick on the partner
cell of — an immediate, guaranteed match ("sniping"), checked
most-recently-remembered first since it is the strongest and most
contestable move available; (2) two distinct remembered cells sharing a
still-unmatched face, neither currently pending — opening one is a bet on
completing it later; (3) otherwise, its configured `Fallback`
(`RandomMover` by default). A larger `Memory` therefore both spots more live
snipes and sets up more future pairings; every value between 0 and the
board's cell count is a real difficulty step, not a cosmetic one.

## Architecture

- **`server-go/pairgame`** (this Feature's whole scope) — a pure Go package
  with no bot, host, or storage code of its own: `state.go` (`GameState`,
  `NewGame`, `NewSoloGame`), `moves.go` (`Flip`, `FlipOutcome`), `player.go`
  (`Player`, `PlayerID`, `PlayerSetup`), `layout.go` (`LayoutMode`,
  `ShuffleFaces`, `DeriveFaces`), `snapshot.go` (`Encode`/`Decode`, the
  budget helpers), `robot.go` (`Strategy`, `RandomMover`, `MemoryStrategy`),
  `size.go` (`Sizes`, board presets), `bits.go` (the bit-packed reader/writer
  `Encode`/`Decode` use).
- Consumed directly by `pair-matching-sessions`' `pairsession` package for
  the two stored modes, and (via `Encode`/`Decode`) by whatever host-bot
  wiring implements Solo's callback-data flow — see
  `telegram-pair-matching-bot`. This package itself has no persistence
  dependency and no knowledge of Telegram.

## Testing strategy

- `conformance_2018_open_test.go` is the regression guard for the founder's
  flip-rule rulings: it ports the archived 2018 engine's `TestOpenCell`
  case_1 in full — six steps, one continuous game, including a player
  sniping another's still-exposed card — onto `Flip`, and a second case
  proving a game's first-ever pick can never match. Re-run this file first
  whenever `moves.go` changes.
- `conformance_2018_shuffle_test.go` carries forward the original's
  shuffle/pairing assertions.
- `moves_test.go` and `misc_test.go` cover `Flip`'s error paths
  (`ErrGameOver`, `ErrUnknownPlayer`, `ErrInvalidCell`,
  `ErrCellAlreadyMatched`, `ErrCellIsPending`) and `GameState`'s completion/
  scoring/winners logic beyond the conformance scenarios.
- `layout_test.go` covers `ShuffleFaces`/`DeriveFaces` shape guarantees
  (every pair id appears exactly twice) and determinism.
- `TestDeriveFacesRequiresTheSecret` / `TestDeriveFacesWrongSecretGivesWrongLayout`
  (`layout_test.go`) are the secrecy property behind
  REQ:seed-derived-layout-is-pure-and-uncached and
  REQ:unmatched-pair-id-never-leaves-server: decoding with the wrong secret
  must not reproduce the true layout.
- `snapshot_test.go`'s `TestSoloBudgetMatrix` is the living measurement
  behind REQ:solo-board-size-gated-by-budget — it reports, per board size and
  layout mode, the actual `Encode()` length and fails if `Fits()` claims true
  but the real prefixed total would exceed 64 bytes; `TestSoloAt8x8FitsComfortably`
  is the specific 8x8/`LayoutSeedDerived` measurement. `TestEncodeDecodeRoundTrip_*`
  prove `Encode`/`Decode` round-trip every field that matters, for both
  layout modes.
- `memory_window_test.go` and `robot_test.go` cover `MemoryStrategy`'s
  ordered decision (snipe, then pairing, then fallback) and `RandomMover`'s
  uniform-over-legal-moves behavior, including the documented footgun that a
  caller invoking `Choose` repeatedly MUST supply a real, persistently-seeded
  `Rand` rather than relying on the nil-`Rand` fallback.
- `bits_test.go` covers the underlying bit reader/writer in isolation.

## Not Doing / Out of Scope

- Any persistence — this package has no storage of its own for any mode; the
  two stored modes' persistence and session composition are
  `pair-matching-sessions`' scope, and Solo's only "storage" is the
  `callback_data` round trip `Encode`/`Decode` already cover here.
- Any bot/host wiring, rendering, or Telegram-specific concern — those
  belong to the out-of-repo bot layer (see `telegram-pair-matching-bot`'s
  Architecture section on the three-repo split). This package does not know
  it is being used by a Telegram bot at all.
- `MaxDifficulty`, the old two-player format's robot-memory budget cap — it
  does not exist in this rewrite. Bots never touch `callback_data`, since a
  bot only ever appears in a stored mode, so the constraint it used to bound
  no longer applies; a bot's `Memory` is unbounded at the engine level (the
  session layer may still choose to offer only certain values as UI tiers,
  which is out of this package's scope).
- Coordinate/address types (e.g. `"A2"`-style cell addresses) — `Flip` takes
  a flat `cell int`; any addressing scheme is a caller/render-layer concern.
- `vs-Humans`' join/lobby/seat-count enforcement (2..8 humans) —
  `NewGame` enforces only the universal `1..MaxPlayers` bound; which shape a
  caller builds (Solo, vs Bot, vs Humans) is a session-layer decision, see
  `pair-matching-sessions`.

## Acceptance Criteria

### AC: no-turn-order-any-player-may-flip
**Requirements:** pair-matching-rules#req:no-turn-order

**Given** a game with two or more seated players and no completed pairs
**When** the same player calls `Flip` several times in a row with no other
player acting in between
**Then** every call is accepted on its own merits (subject only to the other
flip rules) — there is no rejection for "it is not your turn."

### AC: two-players-may-share-a-pending-cell
**Requirements:** pair-matching-rules#req:independent-pending-picks

**Given** two seated players
**When** both flip the same still-unmatched cell in turn, each as their own
first pick
**Then** both flips succeed and each player's `Pending` independently records
that same cell — neither flip is rejected because the other player already
has it pending.

### AC: sniping-credits-the-flipper-not-the-exposer
**Requirements:** pair-matching-rules#req:flip-matches-any-pending-credits-flipper

**Given** player A has a pending pick exposed on a cell whose pair partner is
still unmatched
**When** player B (not A) flips that partner cell
**Then** the pair is marked matched, B's score increases by one, A's score is
unchanged, and both A's and B's pending picks pointing at either of the
pair's two cells are cleared to "none" — reproducing
`conformance_2018_open_test.go`'s step 6.

### AC: mismatch-replaces-the-flippers-pending
**Requirements:** pair-matching-rules#req:mismatch-replaces-own-pending

**Given** a player already has a pending pick
**When** they flip a different, non-matching cell
**Then** their `Pending` becomes the newly flipped cell, replacing the old
one outright, with no intermediate "cleared" state and no score change.

### AC: matched-cell-can-never-be-reflipped
**Requirements:** pair-matching-rules#req:matched-cell-immutable

**Given** a pair that has already been matched
**When** any seated player, including one who still has a stale pending pick
referencing one of that pair's cells, attempts to flip either of its cells
**Then** the call fails with `ErrCellAlreadyMatched` and no state changes.

### AC: flipping-your-own-pending-cell-is-rejected
**Requirements:** pair-matching-rules#req:own-pending-flip-rejected

**Given** a player has a pending pick on cell X
**When** that same player flips cell X again
**Then** the call fails with `ErrCellIsPending` and no state changes; a
*different* player flipping cell X at the same time is unaffected by this
rule.

### AC: every-successful-flip-is-logged
**Requirements:** pair-matching-rules#req:public-reveal-log

**Given** a game in progress
**When** any successful flip occurs, matched or not
**Then** exactly one new entry is appended to `Log` recording who flipped,
the cell, its pair id, and whether it matched — and no successful flip is
ever left unlogged.

### AC: game-ends-with-winners-or-a-tie
**Requirements:** pair-matching-rules#req:completion-and-scoring

**Given** a game where every pair has just been matched
**When** `IsComplete` and `Winners` are evaluated
**Then** `IsComplete` is true and `Winners` returns every player tied for the
highest score — one PlayerID for a clear winner, more than one for a tie.

### AC: bot-strategy-sees-only-public-information
**Requirements:** pair-matching-rules#req:bot-plays-by-identical-rules

**Given** a bot seat mid-game
**When** its `Strategy.Choose` is invoked
**Then** its decision is a function only of the bot's own `Pending` and the
public `Log` — the same `GameState` a human player could inspect from the
chat — and exactly one `Flip` call results per invocation.

### AC: both-layout-modes-yield-a-valid-shuffle
**Requirements:** pair-matching-rules#req:two-layout-modes

**Given** any preset in `Sizes`
**When** a layout is produced under `LayoutInline` (`ShuffleFaces`) and,
separately, under `LayoutSeedDerived` (`DeriveFaces`)
**Then** each produces a `[]uint8` of length `Size.Cells()` in which every
pair id in `[0, Size.Pairs())` appears exactly twice.

### AC: derive-faces-is-deterministic-and-uncached
**Requirements:** pair-matching-rules#req:seed-derived-layout-is-pure-and-uncached

**Given** the same `(secret, version, sizeIndex, seed)`
**When** `DeriveFaces` is called twice, independently, with nothing carried
over between calls
**Then** both calls return the identical layout — proving the derivation
needs no cache or persisted state to be reproducible.

### AC: wrong-secret-cannot-recover-the-layout
**Requirements:** pair-matching-rules#req:solo-secret-from-env, pair-matching-rules#req:unmatched-pair-id-never-leaves-server

**Given** a `LayoutSeedDerived` game's public `Seed`
**When** `DeriveFaces` is called with a secret other than the one the game
was created with
**Then** the resulting layout differs from the true one — a party who can
read the public seed but not the secret cannot recover the board.

### AC: solo-encode-rejects-non-solo-shapes
**Requirements:** pair-matching-rules#req:solo-only-codec

**Given** a `GameState` with more than one player, or a single bot player
**When** `Encode` is called
**Then** it returns `ErrNotSoloGame` rather than truncating any field to fit.

### AC: oversized-solo-board-is-rejected-at-creation
**Requirements:** pair-matching-rules#req:solo-board-size-gated-by-budget

**Given** the 8x8 preset under `LayoutInline`
**When** `NewSoloGame(LayoutInline, 8x8-index, ...)` is called
**Then** it returns `ErrSoloBoardTooLarge` without producing a game, and the
same call under `LayoutSeedDerived` succeeds and its `Encode()` output
measures 14 base64 characters, within `MaxSnapshotBase64Chars`.

### AC: memory-zero-never-snipes
**Requirements:** pair-matching-rules#req:memory-window-difficulty

**Given** a bot with `Memory == 0` and a live snipeable exposure on the
board
**When** the bot's `Strategy.Choose` is invoked
**Then** it never targets the exposed cell purposefully — its choice is
uniform over all currently legal cells, exactly `RandomMover`'s behavior.

### AC: memory-strategy-snipes-before-pairing-before-random
**Requirements:** pair-matching-rules#req:memory-strategy-prefers-sniping-then-pairing-then-random

**Given** a bot whose remembered window contains, separately: (a) a live
exposure it can snipe, (b) two remembered sightings of an unmatched pair with
neither currently pending, and (c) neither of the above
**Then** `MemoryStrategy.Choose` picks the snipe in case (a), the remembered
pairing in case (b), and falls back to `RandomMover` only in case (c).

## Open Questions

None specific to the rules engine itself. The founder-ruled ambiguities this
Feature previously carried (turn order, sniping) are resolved and
regression-guarded (see Testing strategy above); the remaining open
questions this game's build-out raises are tracked where they actually bind
— `pair-matching-sessions` (reveal-log length, abandoned-lobby expiry, a bot
joining a multi-human game) and this repo's `spec/ideas/pair-matching-game.md`
(the shared game-coins economy).

---
*This document follows the https://specscore.md/feature-specification*
