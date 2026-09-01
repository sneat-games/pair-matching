---
format: https://specscore.md/feature-specification
status: Draft
---

# Feature: Telegram Pair-Matching bot (Solo, vs Bot, vs Humans)

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/sneat-games/pair-matching/spec/features/telegram-pair-matching-bot?op=explore) | [Edit](https://specscore.studio/app/github.com/sneat-games/pair-matching/spec/features/telegram-pair-matching-bot?op=edit) | [Ask question](https://specscore.studio/app/github.com/sneat-games/pair-matching/spec/features/telegram-pair-matching-bot?op=ask) | [Request change](https://specscore.studio/app/github.com/sneat-games/pair-matching/spec/features/telegram-pair-matching-bot?op=request-change) |
**Status:** Draft
**Source Ideas:** pair-matching-game

## Summary

Play **Pair-Matching** (a memory/concentration game) inside Telegram through SneatBot's `/games`.

Three founder-specified modes are on offer: **Solo** (one human, no bot,
state carried in the message's `callback_data`), **vs Bot** (one human plus
one bot, a server-side session driven by a timer), or **vs Humans** (2–8
humans, no bot, a server-side session). All three modes share exactly the
same flip rules and the same secrecy guarantees — no turn order, independent per-player pending picks,
any-player sniping credited to the flipper, and an unmatched cell's pair id
never reaching a client before it is actually flipped — because every mode
calls through to this repo's `pair-matching-rules` engine (Solo) or
`pair-matching-sessions` composition (vs Bot, vs Humans) rather than
re-implementing any of it. This document follows the
[SpecScore feature specification](https://specscore.md/feature-specification).

## Problem

This Feature previously described a narrower, and in one important respect
incorrect, shape: a title of "vs robot, state in callback data," a single v1
mode (solo human vs. a robot, per the portfolio's default
`REQ:solo-vs-robot-v1`), a robot memory FIFO carried inside `callback_data`
alongside the board, and a strict "first pick, second pick resolves and
passes the turn" flip model. Two things have since been corrected by the
founder, both recorded in `pair-matching-rules` and `pair-matching-sessions`:

1. **"Solo" is not a synonym for "vs the robot."** Pair Matching, alongside
   GreedGame, is a named exception to the portfolio's Solo-only default
   (`sneat-co/backstage`'s `spec/features/games/README.md`). It ships three
   independent modes — Solo (a human playing alone, no bot at all — the
   original 2018 mode), vs Bot (human + bot), and vs Humans (2–8 humans, no
   bot) — not one mode with a misleading name.
2. **The flip rules themselves were wrong in the earlier draft.** There is no
   turn order and no single shared pending pick; any seated player, human or
   bot, may flip at any moment, and a flip resolves against *any* seated
   player's exposed pending pick, crediting the flipper — restoring the
   archived 2018 original's actual behavior (see
   `pair-matching-rules`' conformance testing). The bot's difficulty memory
   no longer lives in `callback_data` at all: it is now `Player.Memory`,
   held server-side, because a bot ever appears only inside a stored mode.

This Feature's job is narrower than it might look: it does not re-derive any
rule (that is `pair-matching-rules`/`pair-matching-sessions`'s job) — it
specifies how a Telegram bot surface must call through to them, per mode,
and the shared rendering rules the founder has settled on. See "Superseded
vs. reworded from the prior draft" in this repository's PR description for
the exact requirement-by-requirement mapping from the old text to this one.

## Behavior

### Three modes

#### REQ: three-founder-specified-modes

The bot MUST offer exactly the three founder-specified modes when starting a
new Pair-Matching game: **Solo** (one human, no bot), **vs Bot** (one human
plus one bot), and **vs Humans** (2–8 humans, no bot). The mode-selection UI
MUST NOT present "Solo" and "vs Bot" as the same choice under different
labels — they are different modes with different transports (see
`REQ: mode-determines-transport`).

#### REQ: mode-determines-transport

Solo MUST use `pair-matching-rules`' callback-data snapshot
(`GameState.Encode`/`Decode`, `LayoutSeedDerived`) and MUST NOT create any
server-side game record. vs Bot and vs Humans MUST use
`pair-matching-sessions`' stored-game API
(`CreateVsBotGame`/`CreateVsHumansGame`, `Flip`, `RobotMove`, `GetView`) and
MUST NOT attempt to fit their state into `callback_data`. A mode never mixes
the two transports.

### Engine & session reuse

#### REQ: engine-is-single-source-of-rules

All Pair-Matching rules — layout, reveal/match/mismatch resolution,
scoring, completion, and every robot strategy — MUST come from
`pair-matching-rules` (Solo) or `pair-matching-sessions` (vs Bot, vs
Humans). The bot layer MUST NOT re-implement or duplicate any game rule; per
`sneat-co/backstage`'s `REQ:engine-per-repo`, that layer lives entirely in
`sneat-co/sneat-bots` and `sneat-co/sneat-go`, outside this repo (see
Architecture below), and contains no independent match-detection,
turn-resolution, or scoring logic of its own.

### Solo (callback-data) surface

#### REQ: solo-uses-seed-derived-layout-only

A Solo game MUST be created with `pairgame.LayoutSeedDerived`, never
`LayoutInline` — an inline layout is fully readable by anyone who can read
the button's `callback_data`, a full spoiler for a memory game (see
`pair-matching-rules`' `REQ: unmatched-pair-id-never-leaves-server`). New
Solo games MUST offer only the presets in `pairgame.Sizes` that
`NewSoloGame` accepts for the chosen layout mode — a size `NewSoloGame`
would reject with `ErrSoloBoardTooLarge` MUST NOT be offered.

#### REQ: solo-secret-is-host-configuration

The host bot MUST provision the `PAIR_MATCHING_GAME_SECRET` used by
`DeriveFaces` as ordinary configuration (CI and Cloud Run), never as
per-game persistence, and owns its rotation. Rotating the secret is an
accepted operational consequence, not a bug: every in-flight Solo game's
`callback_data` stops decoding into the true layout the moment the old
secret is gone, which is fine because a Solo game *is* the message — there
is no resume surface a rotation needs to preserve.

#### REQ: solo-callback-data-carries-complete-state

Every board button's `callback_data` MUST encode the complete
`GameState.Encode()` snapshot, with room reserved for the host's own command
prefix and target-cell address (`pairgame.HostPrefixReserveBytes`), staying
within Telegram's 64-byte `callback_data` limit. `pair-matching-rules`'
`REQ: solo-board-size-gated-by-budget` already guarantees this fits for
every size `NewSoloGame` accepts; this Feature's job is to actually deliver
that snapshot inside a real button and decode it back on every tap
(`GameState.Decode` as `Encode`'s exact inverse) — not to re-measure the
budget itself.

#### REQ: solo-no-server-persistence

The bot MUST NOT persist a Solo game server-side — no datastore/Firestore
record, no per-chat "current game" blob. Accepted consequences: no move
history beyond the current snapshot, no "resume"/history surface, no
per-player lock on a shared or forwarded message, and no concurrency guard.
This constraint is Solo-only — it does not extend to vs Bot or vs Humans,
which are deliberately stored (see `REQ: mode-determines-transport`).

#### REQ: solo-illegal-tap-and-game-over-are-noops

Tapping an already-matched cell or the player's own already-pending cell
MUST leave the rendered board unchanged (the engine's `ErrCellAlreadyMatched`
/ `ErrCellIsPending`). Once every pair is matched, the board MUST render as
finished — the player's final pair count and a "start a new game" affordance
— and tapping any cell on a finished board MUST be a no-op
(`ErrGameOver`).

### vs Bot / vs Humans (stored) surface

#### REQ: vs-bot-timer-drives-robot-move

A vs-Bot game's bot seat MUST be advanced by a scheduled/delayed task that
calls `pairsession.RobotMove` — never by the bot reacting to a human's tap.
That task MUST re-enqueue itself once per tick while the game is
`StatusActive` and a bot is seated, and MUST stop reliably once the game
reaches `StatusFinished` (no further ticks fire after completion). Per
`pair-matching-sessions`' `REQ: robot-move-one-flip-per-call`, each tick
flips exactly one card — a deliberate pacing choice: completing a pair still
costs the bot a minimum of two ticks, keeping its play visible and beatable
rather than appearing to complete pairs instantly.

#### REQ: vs-humans-join-invite-and-start

A vs-Humans game MUST let the host invite others (a shareable deep link, a
group-chat join button, or both) while the game is in `StatusLobby`, and
MUST let the host (or whoever the product's UI designates) trigger
`pairsession.StartGame` once at least two players have joined. The bot MUST
surface `pairsession`'s lobby errors (`ErrGameNotInLobby`,
`ErrNotEnoughPlayers`, `ErrTooManyPlayers`) as user-facing feedback rather
than swallowing them.

#### REQ: anchored-message-per-mode-shape

A group-chat vs-Bot/vs-Humans game MUST render into exactly ONE shared
status message, edited in place as players join, flip, and finish
(`pairsession.SetGroupMessage`). A private-invite vs-Humans game MUST
instead give each player their own anchored board message, edited in place
independently (`pairsession.SetPlayerMessage`). The bot's broadcast/render
step MUST skip any player whose `ChatID` or `MessageID` is still zero
(`pair-matching-sessions`' `REQ: zero-is-the-unanchored-sentinel`) rather
than attempting delivery to an address that does not exist yet.

### Rendering (shared across every mode, founder-ruled — not open for
### re-litigation)

#### REQ: matched-face-else-hidden-glyph

A cell MUST show its true face (emoji) only once its pair has been matched;
every other cell — including one that has been publicly revealed by a flip
that did not match — MUST render as a plain hidden glyph, not its face.
"Revealed" (has appeared in the public log) and "shows its face to every
viewer" are different things: only "matched" earns the latter.

#### REQ: no-ownership-markers-or-card-back-art

The board MUST NOT render any per-cell ownership marker (e.g. "matched by
player X" badges on the tile itself) and MUST NOT use bespoke card-back art
beyond the one plain hidden glyph — both were explicitly rejected by the
founder. Who owns a matched pair is communicated only through the
scoreboard and the reveal-log messages, never on the tile.

#### REQ: own-pending-shown-solo-and-vs-bot-only

Solo and vs-Bot boards, each having exactly one human audience, MAY also
show that human's own currently-pending cell on the board itself. A
vs-Humans board, being one shared message read by every seated player, MUST
NOT show any player's pending pick on the board — the public reveal log
(rendered as chat messages, per `pair-matching-rules`' `REQ:
public-reveal-log`) is the only channel through which a vs-Humans player
learns what is currently exposed.

#### REQ: face-palette-is-a-fixed-emoji-set-permuted-per-seed

A rendered face MUST be drawn from a fixed emoji palette (48 entries, one
per possible pair id at the largest board preset), permuted once per game
from the game's own seed so repeated games do not always show the same
emoji for the same pair id. This mapping is independent of, and in addition
to, `pair-matching-rules`' own cell-layout shuffle (which decides which
*cell* holds which pair id, not which *emoji* a pair id displays as).

### Architecture: the three-repo split

#### REQ: bot-commands-behind-a-host-port

Pair Matching's bot commands MUST live in `sneat-co/sneat-bots`, calling
this repo's `pair-matching-rules`/`pair-matching-sessions` packages only
through a host-port interface the bot package itself declares (e.g.
`extensions/sneatbot/cmds4sneatbot.PairService`), never a direct dependency
on a concrete database. This indirection exists purely to avoid a
`sneat-bots` → `sneat-go` import cycle — the same shape GreedGame already
uses (`sneat-bots/extensions/greedgame/bot/cmds4greedgamebot.Service`).

#### REQ: sneat-go-wire-and-configure-only

`sneat-co/sneat-go` (`pkg/modules/pairmatching`) MUST do nothing but bind
that port to the concrete `dal.DB` (and, for Solo, the
`PAIR_MATCHING_GAME_SECRET` configuration) — mirroring
`sneat-go/pkg/modules/greedgame`'s `botService`. It MUST NOT contain any
game logic, session logic, or rendering decision of its own; per that
repo's own constitution, it is wire-and-configure only.

## Architecture & Components

- **`pair-matching-rules` / `pair-matching-sessions`** (this repo, both
  Features above) — the sole source of rules, layout, secrecy, and session
  composition for every mode. This Feature calls them; it never re-derives
  them.
- **Pair-Matching bot commands** (`sneat-co/sneat-bots`, not yet built) —
  own the Telegram-specific surface for all three modes: rendering a
  `GameState`/`View` to a `bots-go-core/botkb` inline keyboard, the mode
  picker, the vs-Humans invite/join UI, and the group-vs-private-invite
  anchored-message logic — the same shape as `sneat-games/reversi`'s
  `revplay` package for Solo, plus GreedGame's bot-command shape for the
  stored modes. Declares and implements the host-port `PairService`
  interface described in `REQ: bot-commands-behind-a-host-port`.
- **SneatBot host wiring** (`sneat-co/sneat-go`, `pkg/modules/pairmatching`,
  not yet built, WIRE-AND-CONFIGURE-ONLY per that repo's own constitution) —
  registers the `botsfw.Command`, owns and rotates the `DeriveFaces` secret
  and the stored-mode database wiring as ordinary configuration, and returns
  the bot's `botmsg.MessageFromBot`. This Feature's engine and session work
  does not touch `sneat-co/sneat-go`.

## Testing strategy

This repo's own test suites are the regression guard for every guarantee
this Feature depends on but does not itself re-test:

- `pair-matching-rules`' `conformance_2018_open_test.go`,
  `TestSoloBudgetMatrix`/`TestSoloAt8x8FitsComfortably`, and the
  `TestDeriveFaces*` secrecy tests back `REQ:engine-is-single-source-of-rules`,
  `REQ:solo-uses-seed-derived-layout-only`, and
  `REQ:solo-callback-data-carries-complete-state`.
- `pair-matching-sessions`' `session_test.go` — in particular
  `TestRobotMove_FlipsExactlyOneCardPerCall`, `TestGetView_UnrevealedCellsStayHidden`,
  `TestSetGroupMessage`, `TestSetPlayerMessage`, and `TestSettersDoNotTouchRuleState`
  — back `REQ:vs-bot-timer-drives-robot-move`, `REQ:vs-humans-join-invite-and-start`,
  and `REQ:anchored-message-per-mode-shape`.
- The **rendering** REQs' security property (an unmatched cell's pair id
  never reaching a client) is guarded at both boundaries once the bot layer
  exists: `TestRenderPairStoredBoard_UnmatchedCellsLeakNoLayout` in
  `sneat-co/sneat-bots` and `TestToCellViews_NeverLeaksAnUnmatchedCellsPairID`
  in `sneat-co/sneat-go`, alongside this repo's own
  `TestGetView_UnrevealedCellsStayHidden` (`pair-matching-sessions`) that the
  same views are built from.
- When the bot layer is built (out of this repo), add: a new-game test per
  mode asserting the chosen mode/size round-trips into the first rendered
  message; a legal-tap test proving exactly the engine's/session's outcome
  is reflected; an illegal-tap no-op test; a robot-tick test proving exactly
  one flip occurs per invocation and the schedule stops at
  `StatusFinished`; a vs-Humans join/start test; and a rendering test
  proving `REQ:matched-face-else-hidden-glyph` /
  `REQ:no-ownership-markers-or-card-back-art` /
  `REQ:own-pending-shown-solo-and-vs-bot-only` hold for each mode.

## Not Doing / Out of Scope

- Any game rule, layout derivation, or session/persistence logic of its
  own — all owned by `pair-matching-rules` / `pair-matching-sessions`; see
  `REQ:engine-is-single-source-of-rules`.
- The actual bot-command code (`sneat-co/sneat-bots`) and host wiring
  (`sneat-co/sneat-go`) — this Feature specifies the contract they must
  satisfy; building them is separate, later work in those repos.
- A bot seat joining a vs-Humans game as an extra player — not supported by
  `pair-matching-sessions` today; see that Feature's Open Questions.
- Reaping/expiring an abandoned vs-Humans lobby or an abandoned active
  game — see `pair-matching-sessions`' Open Questions.
- Capping or windowing the public reveal log for rendering — see
  `pair-matching-sessions`' Open Questions; until decided, this Feature
  assumes the full log is available to render, however large.
- Staking or wagering through the shared game-coins economy — see the
  parent Idea's Open Questions.
- A UI for choosing vs-Bot difficulty `N` beyond "the host offers some
  choices" — the specific presented tiers (e.g. Easy/Medium/Hard labels
  mapped to `Player.Memory` values) are a host/product decision, not an
  engine or session one.

## Acceptance Criteria

### AC: mode-picker-offers-exactly-the-three-modes
**Requirements:** telegram-pair-matching-bot#req:three-founder-specified-modes

**Given** a player starting a new Pair-Matching game
**When** they are asked to choose a mode
**Then** they are offered exactly Solo, vs Bot, and vs Humans, distinctly
labeled, with no mode presented as a variant of another.

### AC: each-mode-uses-its-own-transport-only
**Requirements:** telegram-pair-matching-bot#req:mode-determines-transport

**Given** a game in progress in each of the three modes
**When** its state storage is inspected
**Then** the Solo game has no server-side record and its full state is in
the last rendered message's `callback_data`; the vs-Bot and vs-Humans games
each have exactly one server-side record and their rendered messages carry
no game state of their own beyond an opaque game/message reference.

### AC: bot-layer-contains-no-rule-logic
**Requirements:** telegram-pair-matching-bot#req:engine-is-single-source-of-rules

**Given** the Pair-Matching bot-command code
**When** a reveal is validated, applied, scored, or a robot reply is
computed
**Then** it is done by calling `pair-matching-rules` or
`pair-matching-sessions`, and the bot layer contains no independent
match-detection, turn-resolution, or scoring logic.

### AC: solo-game-never-uses-an-inline-layout
**Requirements:** telegram-pair-matching-bot#req:solo-uses-seed-derived-layout-only

**Given** a new Solo game at any offered board size
**When** its snapshot is inspected
**Then** its `Mode` is `LayoutSeedDerived`, and no board size that
`NewSoloGame` would reject as `ErrSoloBoardTooLarge` was ever offered to the
player.

### AC: secret-rotation-is-config-not-a-migration
**Requirements:** telegram-pair-matching-bot#req:solo-secret-is-host-configuration

**Given** `PAIR_MATCHING_GAME_SECRET` is rotated in host configuration
**When** an in-flight Solo game's button, encoded under the old secret, is
tapped again
**Then** the host treats this as an ordinary decode/derivation mismatch (a
fresh game is the expected recovery), not as a data-migration incident.

### AC: solo-round-trips-the-full-snapshot-in-one-button
**Requirements:** telegram-pair-matching-bot#req:solo-callback-data-carries-complete-state

**Given** a Solo game's rendered board
**When** any cell's button is tapped
**Then** the host decodes the complete prior snapshot from that single
button's `callback_data`, applies the tap via the engine, and re-renders —
with no server-side lookup involved at any point.

### AC: no-solo-records-written
**Requirements:** telegram-pair-matching-bot#req:solo-no-server-persistence

**Given** a full Solo game played from start to finish in a chat
**When** the game runs
**Then** no game-state records are written to any datastore and no per-chat
game blob is stored.

### AC: solo-illegal-tap-and-finished-board-are-noops
**Requirements:** telegram-pair-matching-bot#req:solo-illegal-tap-and-game-over-are-noops

**Given** a Solo board with at least one matched pair
**When** an already-matched cell, or the player's own pending cell, is
tapped again — and, separately, when any cell on a finished board is tapped
**Then** the rendered state is unchanged from before the tap in both cases,
and a finished board additionally shows the final pair count and a
"start a new game" affordance.

### AC: bot-tick-stops-exactly-at-finish
**Requirements:** telegram-pair-matching-bot#req:vs-bot-timer-drives-robot-move

**Given** an active vs-Bot game whose scheduled task is ticking
**When** the game reaches `StatusFinished` on some tick
**Then** no further tick calls `RobotMove` against that game afterward, and
every prior tick flipped exactly one card.

### AC: vs-humans-cannot-start-below-two-players
**Requirements:** telegram-pair-matching-bot#req:vs-humans-join-invite-and-start

**Given** a vs-Humans lobby with zero or one joined player
**When** the host attempts to start the game
**Then** the attempt is rejected and surfaced to the host as user-facing
feedback (not a silent failure), and the game remains in its lobby.

### AC: broadcast-skips-unanchored-recipients
**Requirements:** telegram-pair-matching-bot#req:anchored-message-per-mode-shape

**Given** a vs-Humans game with one player who has not yet had a message
anchored for them (`ChatID` or `MessageID` still zero)
**When** the game's state changes and a broadcast/render step runs
**Then** every anchored recipient's message is edited in place, and no
delivery is attempted to the unanchored player.

### AC: unmatched-flipped-cell-still-shows-hidden
**Requirements:** telegram-pair-matching-bot#req:matched-face-else-hidden-glyph

**Given** a cell that has been flipped (and so appears in the public log)
but whose pair is not yet matched
**When** the board is rendered
**Then** that cell still shows the plain hidden glyph, not its face — only a
matched pair's two cells show their true face.

### AC: no-per-cell-ownership-marker-appears
**Requirements:** telegram-pair-matching-bot#req:no-ownership-markers-or-card-back-art

**Given** a matched pair on the board
**When** the board is rendered
**Then** the matched cells show only their face — no badge, marker, or
distinct card-back art indicating who claimed them; ownership is visible
only in the scoreboard and reveal-log messages.

### AC: vs-humans-board-never-shows-a-pending-pick
**Requirements:** telegram-pair-matching-bot#req:own-pending-shown-solo-and-vs-bot-only

**Given** a vs-Humans game where a player currently has a pending pick
**When** the one shared board message is rendered
**Then** no player's pending pick is shown on the board itself; a Solo or
vs-Bot board, by contrast, may show that game's own single human's pending
pick.

### AC: face-emoji-vary-across-games-with-the-same-pair-id
**Requirements:** telegram-pair-matching-bot#req:face-palette-is-a-fixed-emoji-set-permuted-per-seed

**Given** two different games (different seeds) that both need to render
pair id 0
**When** each game's face palette is derived from its own seed
**Then** the two games are not guaranteed to display the same emoji for
pair id 0 — the palette-to-pair-id mapping is permuted per game.

### AC: bot-package-only-reaches-storage-through-the-port
**Requirements:** telegram-pair-matching-bot#req:bot-commands-behind-a-host-port

**Given** the Pair-Matching bot-command package in `sneat-co/sneat-bots`
**When** its imports are inspected
**Then** it depends on a host-port interface it declares itself, and has no
direct import of a concrete database package or of `sneat-co/sneat-go`.

### AC: sneat-go-module-contains-no-game-logic
**Requirements:** telegram-pair-matching-bot#req:sneat-go-wire-and-configure-only

**Given** `sneat-co/sneat-go`'s `pkg/modules/pairmatching` package
**When** its contents are inspected
**Then** it contains only wiring (binding the port to a concrete `dal.DB`
and to `PAIR_MATCHING_GAME_SECRET` configuration) and no game rule, session
rule, or rendering decision of its own.

## Open Questions

- Which specific `N` (`Player.Memory`) values the host should expose as
  named vs-Bot difficulty tiers (e.g. Easy/Medium/Hard) is a product
  decision for whoever builds the SneatBot wiring in `sneat-co/sneat-go` —
  not decided here.
- Exactly which invite mechanism(s) — a deep link, an inline "invite"
  button inside a group chat, or both — a vs-Humans lobby should offer is a
  product/UX decision for the bot-command build, not decided here.

---
*This document follows the https://specscore.md/feature-specification*
