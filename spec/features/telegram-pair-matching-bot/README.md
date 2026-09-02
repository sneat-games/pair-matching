---
format: https://specscore.md/feature-specification
status: Stable
---

# Feature: Telegram Pair-Matching bot (Solo, vs Bot, vs Humans)

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/sneat-games/pair-matching/spec/features/telegram-pair-matching-bot?op=explore) | [Edit](https://specscore.studio/app/github.com/sneat-games/pair-matching/spec/features/telegram-pair-matching-bot?op=edit) | [Ask question](https://specscore.studio/app/github.com/sneat-games/pair-matching/spec/features/telegram-pair-matching-bot?op=ask) | [Request change](https://specscore.studio/app/github.com/sneat-games/pair-matching/spec/features/telegram-pair-matching-bot?op=request-change) |
**Status:** Stable
**Source Ideas:** pair-matching-game

## Summary

Play **Pair-Matching** (a memory/concentration game) inside Telegram through SneatBot's `/games`.

Three founder-specified modes are on offer: **Solo** (one human, no bot,
state carried in the message's `callback_data`), **vs Bot** (one human plus
one bot, a server-side session driven by a timer), or **vs Humans** (2–8
humans, no bot, a server-side session). All three modes share exactly the
same flip rules — no turn order, independent per-player pending picks,
any-player sniping credited to the flipper — and the same board-display
model: a card is visible face-up while it is open (matched, or someone's
current pending pick) and goes face-down again the moment it stops being
open, with no record of who opened it ever shown. Every mode calls through
to this repo's `pair-matching-rules` engine (Solo) or `pair-matching-sessions`
composition (vs Bot, vs Humans) rather than re-implementing any of it. The
bot-command surface (`sneat-co/sneat-bots`) and its host wiring
(`sneat-co/sneat-go`) are shipped and in production for all three modes;
this Feature specifies the contract they implement. This document follows
the [SpecScore feature specification](https://specscore.md/feature-specification).

## Problem

This Feature has gone through two rounds of correction since it first
existed as "vs robot, state in callback data." Both rounds are recorded
below so the requirement mapping stays auditable rather than living only in
a PR description that nobody reads two months later.

**First round (three modes, and the real flip rules).** The original draft
assumed a single v1 mode (solo human vs. a robot), a robot memory FIFO
carried inside `callback_data`, and a strict "first pick, second pick
resolves and passes the turn" flip model. The founder corrected two things:
"Solo" is not a synonym for "vs the robot" — Pair Matching, alongside
GreedGame, ships three independent modes (Solo, vs Bot, vs Humans), not one
mode with a misleading name — and the flip rules themselves were wrong:
there is no turn order and no single shared pending pick; any seated
player, human or bot, may flip at any moment, and a flip resolves against
*any* seated player's exposed pending pick, crediting the flipper,
restoring the archived 2018 original's actual behavior (see
`pair-matching-rules`' conformance testing).

**Second round (the board-display model).** The founder subsequently
overrode the board-rendering model this Feature had specified after the
first round — including a "no pending pick shown on a vs-Humans board"
privacy rule this document had itself invented and presented as
founder-ruled, which the founder never actually said. Verbatim, on why
attribution does not matter: *"I don't think we need to show who opened
what as it does not affect game who opened - opens cards are shared."* On
what to keep visible, choosing explicitly between "board only" and "board
plus a written history," the founder chose **board only**: *"Face-up on the
board = open right now, or matched. When an unmatched card flips back
down, it's gone from view and you have to remember it. This is what makes
it a memory game."* This Feature's rendering requirements below implement
that model; **this document previously mislabeled requirements the founder
never actually ruled on as "founder-ruled — not open for re-litigation."**
Every requirement below states its provenance individually now: a founder
quote, or an implementation fact about the shipped `sneat-co/sneat-bots` /
`sneat-co/sneat-go` code that is not itself a founder ruling and remains
open to revision.

This Feature's job stays narrower than it might look: it does not re-derive
any game rule (that is `pair-matching-rules`/`pair-matching-sessions`'s
job) — it specifies how the Telegram bot surface must call through to
them, per mode, and the shared rendering rules the founder has settled on.

### Requirement mapping across both drafts

| Prior requirement | Disposition | Current status |
|---|---|---|
| `REQ: no-server-persistence` (original draft, global claim) | Superseded — the claim was global; only Solo is persistence-free | Reworded, Solo-scoped: `REQ: solo-no-server-persistence` below |
| `REQ: robot-memory-in-callback-data` (original draft) | Superseded outright | No replacement in this Feature — the bot's difficulty now lives server-side as `Player.Memory`, entirely `pair-matching-rules`'/`pair-matching-sessions`' scope |
| `REQ: reveal-then-resolve` (original draft) | Superseded outright — the turn-alternation model it described never matched the archived 2018 original | Replaced conceptually by `pair-matching-rules`' `REQ: flip-matches-any-pending-credits-flipper` / `REQ: mismatch-replaces-own-pending` |
| `AC: match-keeps-turn-mismatch-passes-it` (original draft) | Superseded — "keeps/passes the turn" is not a concept in the current rules | No replacement |
| `REQ: seed-derived-layout`, `REQ: callback-data-budget`, `REQ: engine-is-single-source-of-rules`, `REQ: illegal-move-noop`, `REQ: game-over` (original draft) | Reworded, re-scoped to Solo specifically and re-pointed at `pair-matching-rules`/`pair-matching-sessions` rather than restated | `REQ: solo-uses-seed-derived-layout-only`, `REQ: solo-callback-data-carries-complete-state`, `REQ: engine-is-single-source-of-rules`, `REQ: solo-illegal-tap-and-game-over-are-noops` below |
| `REQ: supported-board-sizes`, `REQ: memory-difficulty` (original draft) | Folded/delegated | `pair-matching-rules`' `REQ: memory-window-difficulty`; `pair-matching-sessions`' `CreateVsBotGame` |
| `REQ: matched-face-else-hidden-glyph` (this Feature's prior rewrite) | Superseded by the founder's board-display ruling (second round, above) | `REQ: open-cards-are-shared-no-attribution` below |
| `REQ: own-pending-shown-solo-and-vs-bot-only` (this Feature's prior rewrite) | Superseded — **retracted outright**: this document invented it and mislabeled it founder-ruled; the founder never ruled on a vs-Humans pending-privacy carve-out | Deleted, no replacement — pending picks are now shared everywhere, see `REQ: open-cards-are-shared-no-attribution` |
| `REQ: face-palette-is-a-fixed-emoji-set-permuted-per-seed` (this Feature's prior rewrite) | Reworded — its rationale was wrong (largest preset is 32 pairs, not 48) and it was mislabeled founder-ruled when it is an implementation fact | Same slug below, corrected body and provenance |
| `REQ: vs-bot-timer-drives-robot-move` (this Feature's prior rewrite) | Reworded — the "completing a pair costs a minimum of two ticks" inference was false | Same slug below, claim removed |
| `REQ: solo-uses-seed-derived-layout-only` (this Feature's prior rewrite) | Reworded — named an enforcement path (`NewSoloGame`'s `ErrSoloBoardTooLarge`) the shipped code does not use | Same slug below, actual gate described |
| `REQ: sneat-go-wire-and-configure-only` (this Feature's prior rewrite) | Reworded — falsely claimed `sneat-go` binds `PAIR_MATCHING_GAME_SECRET` at runtime | Same slug below, corrected |
| `REQ: vs-humans-join-invite-and-start` (this Feature's prior rewrite) | Reworded — hedged ("a deep link, a join button, or both") on something already shipped and decided | Same slug below, states the shipped fact (both) |
| `AC: unmatched-flipped-cell-still-shows-hidden` (this Feature's prior rewrite) | Superseded — asserted the pre-founder-ruling rendering model | Replaced by ACs under `REQ: open-cards-are-shared-no-attribution` |
| `AC: vs-humans-board-never-shows-a-pending-pick` (this Feature's prior rewrite) | Superseded — deleted with its REQ | No replacement |

## Behavior

### Three modes

#### REQ: three-founder-specified-modes

The bot MUST offer exactly the three founder-specified modes when starting a
new Pair-Matching game: **Solo** (one human, no bot), **vs Bot** (one human
plus one bot), and **vs Humans** (2–8 humans, no bot). The mode-selection UI
MUST NOT present "Solo" and "vs Bot" as the same choice under different
labels — they are different modes with different transports (see
`REQ: mode-determines-transport`). *Founder-ruled (first round, above).*

#### REQ: mode-determines-transport

Solo MUST use `pair-matching-rules`' callback-data snapshot
(`GameState.Encode`/`Decode`, `LayoutSeedDerived`) and MUST NOT create any
server-side game record. vs Bot and vs Humans MUST use
`pair-matching-sessions`' stored-game API
(`CreateVsBotGame`/`CreateVsHumansGame`, `Flip`, `RobotMove`, `GetView`) and
MUST NOT attempt to fit their state into `callback_data`. A mode never mixes
the two transports. *Founder-ruled (first round, above), via
`sneat-co/backstage`'s multi-actor carve-out.*

### Engine & session reuse

#### REQ: engine-is-single-source-of-rules

All Pair-Matching rules — layout, reveal/match/mismatch resolution,
scoring, completion, and every robot strategy — MUST come from
`pair-matching-rules` (Solo) or `pair-matching-sessions` (vs Bot, vs
Humans). The bot layer MUST NOT re-implement or duplicate any game rule; per
`sneat-co/backstage`'s `REQ:engine-per-repo`, that layer lives entirely in
`sneat-co/sneat-bots` and `sneat-co/sneat-go`, outside this repo (see
Architecture below), and contains no independent match-detection,
turn-resolution, or scoring logic of its own. *Architectural policy, this
Feature's own — not itself a specific founder quote.*

### Solo (callback-data) surface

#### REQ: solo-uses-seed-derived-layout-only

A Solo game MUST be created with `pairgame.LayoutSeedDerived`, never
`LayoutInline` — an inline layout is fully readable by anyone who can read
the button's `callback_data`, a full spoiler for a memory game (see
`pair-matching-rules`' `REQ: unmatched-pair-id-never-leaves-server`).

**The actual gate, stated honestly:** the shipped `sneat-co/sneat-bots`
Solo path (`pairSoloCallbackAction`) enforces this by construction — it
calls `pairgame.NewGame` with `pairgame.LayoutSeedDerived` hardcoded as a
literal argument, never a caller-selectable value — rather than by calling
`pairgame.NewSoloGame`, whose own `ErrSoloBoardTooLarge` rejection path is
therefore never reached from this code path. The board-size picker
(`pairSizePicker`) also currently offers every preset in `pairgame.Sizes`
unfiltered by any budget check. This has no live consequence today only
because `LayoutSeedDerived` fits comfortably at every preset in `Sizes`
(see `pair-matching-rules`' `TestSoloBudgetMatrix`) — if a future preset or
layout mode did not fit, this gate's absence would let a broken game start.

#### REQ: solo-secret-is-host-configuration

The `PAIR_MATCHING_GAME_SECRET` used by `DeriveFaces` MUST be provisioned as
ordinary configuration (CI and Cloud Run), never as per-game persistence.
Rotating it is an accepted operational consequence, not a bug: every
in-flight Solo game's `callback_data` stops decoding into the true layout
the moment the old secret is gone.

**Silent corruption, not graceful degradation — stated honestly:**
`GameState.Decode` never touches the secret at all (it recovers only the
public `Seed`, not the layout — see `pair-matching-rules`' `Decode` doc
comment), and a Solo snapshot carries no MAC over its own fields (see
`pair-matching-rules`' `REQ: solo-snapshot-is-unauthenticated`). There is
therefore no detection path for a rotation at all: after rotation, an
in-flight Solo game's board silently continues rendering — every tap still
decodes and re-encodes successfully — on a scrambled layout, with nothing
telling the player their game just broke. This is NOT "the host treats it
as an ordinary mismatch and offers a fresh game" (an earlier version of
this REQ's own Acceptance Criteria claimed exactly that detection, which
does not exist); it is silent, undetected corruption. Whether that is
acceptable is a real product question, not resolved here.

#### REQ: solo-callback-data-carries-complete-state

Every board button's `callback_data` MUST encode the complete
`GameState.Encode()` snapshot, with room reserved for the host's own command
prefix and target-cell address (`pairgame.HostPrefixReserveBytes`), staying
within Telegram's 64-byte `callback_data` limit. `pair-matching-rules`'
`REQ: solo-board-size-gated-by-budget` already guarantees this fits for
every size the engine's own `NewSoloGame` would accept; this Feature's job
is to actually deliver that snapshot inside a real button and decode it
back on every tap (`GameState.Decode` as `Encode`'s exact inverse).

#### REQ: solo-no-server-persistence

The bot MUST NOT persist a Solo game server-side — no datastore/Firestore
record, no per-chat "current game" blob. Accepted consequences: no move
history beyond the current snapshot, no "resume"/history surface, no
per-player lock on a shared or forwarded message, no concurrency guard, and
— per `pair-matching-rules`' `REQ: solo-snapshot-is-unauthenticated` — no
integrity check on a hand-crafted `callback_data` payload: a player who
constructs their own snapshot string can make the decoded game claim any
board state they like (every pair pre-matched, for instance). This
constraint is Solo-only — it does not extend to vs Bot or vs Humans, whose
state a client never directly controls (see `REQ: mode-determines-transport`).

#### REQ: solo-illegal-tap-and-game-over-are-noops

Tapping an already-matched cell or the player's own already-pending cell
MUST leave the rendered board unchanged (the engine's `ErrCellAlreadyMatched`
/ `ErrCellIsPending`). Once every pair is matched, the board MUST render as
finished — the player's final pair count and a "start a new game" affordance
— and tapping any cell on a finished board MUST be a no-op (`ErrGameOver`).

### vs Bot / vs Humans (stored) surface

#### REQ: vs-bot-timer-drives-robot-move

A vs-Bot game's bot seat MUST be advanced by a scheduled/delayed task that
calls `pairsession.RobotMove` — never by the bot reacting to a human's tap.
That task MUST re-enqueue itself once per tick while the game is
`StatusActive` and a bot is seated, and MUST stop reliably once the game
reaches `StatusFinished` (no further ticks fire after completion). Per
`pair-matching-sessions`' `REQ: robot-move-one-flip-per-call`, each tick
flips exactly one card — a deliberate pacing choice so the bot's play is
visible rather than resolving between renders. **This does NOT mean
completing a pair costs the bot two ticks:** a single tick's `RobotMove`
call can complete a pair outright whenever `MemoryStrategy`'s snipe check
finds a live exposure (see `pair-matching-rules`' `REQ:
memory-strategy-prefers-sniping-then-pairing-then-random`), exactly as any
human flip can.

#### REQ: vs-bot-difficulty-is-two-tiers

The vs-Bot difficulty picker MUST offer exactly two tiers — shipped as
"🤖 AI" and "🎲 Random" — mapped respectively to `Player.Memory` values of
`1000` and `0`. `1000` is comfortably above the longest reveal log any
supported board can produce (every preset tops out at 32 pairs / 64 cells),
so "AI" difficulty effectively remembers the entire public log rather than
a bounded tail of it. This closes what an earlier draft of this Feature
left as an Open Question ("which `N` values the host should expose") —
these two are the shipped answer, not a placeholder pending a future
product decision.

#### REQ: vs-humans-join-invite-and-start

A vs-Humans game MUST offer BOTH invite mechanisms while in `StatusLobby` —
a shareable deep link (`t.me/SneatBot?start=pmjoin_<gameID>`) and an
in-chat "🙋 Join" button — not a choice between them; a private-invite game
additionally surfaces the deep link in its lobby card text for the host to
forward. The bot MUST let the host (or whoever the product's UI designates)
trigger `pairsession.StartGame` once at least two players have joined, and
MUST surface `pairsession`'s lobby errors (`ErrGameNotInLobby`,
`ErrNotEnoughPlayers`, `ErrTooManyPlayers`) as user-facing feedback rather
than swallowing them. This closes what an earlier draft of this Feature
left as an Open Question ("exactly which invite mechanism") — both is the
shipped answer.

#### REQ: anchored-message-per-mode-shape

A group-chat vs-Bot/vs-Humans game MUST render into exactly ONE shared
status message, edited in place as players join, flip, and finish
(`pairsession.SetGroupMessage`). A private-invite vs-Humans game MUST
instead give each player their own anchored board message, edited in place
independently (`pairsession.SetPlayerMessage`). The bot's broadcast/render
step MUST skip any player whose `ChatID` or `MessageID` is still zero
(`pair-matching-sessions`' `REQ: zero-is-the-unanchored-sentinel`) rather
than attempting delivery to an address that does not exist yet.

### Rendering (shared across every mode)

Every requirement in this section names its own provenance individually —
see the Problem section above on why that discipline matters here
specifically.

#### REQ: open-cards-are-shared-no-attribution

**Founder-ruled, quoted verbatim in the Problem section above.** A cell
MUST show its true face when EITHER its pair is matched OR it is any
seated player's CURRENT pending pick (`Pending == cell` for some player,
right now) — every currently-open card is visible to every player,
regardless of who opened it or which mode is being played. No attribution
of who opened, or who currently holds, a pending pick MUST ever be shown —
not on the board, not in any accompanying message, in any mode including
vs-Humans. When an unmatched pending pick is later replaced (the flipper
opens something else) or resolved (matched, by anyone), the cell MUST go
back to the hidden glyph — this is what makes it a memory game rather than
a checklist.

#### REQ: pending-drives-visibility-not-revealed

**A real trap for an implementer, stated explicitly** (mirrors
`pair-matching-sessions`' identically-named warning on its own `View`,
repeated here because it is the render layer that will actually get this
wrong). `pairsession.CellView.Revealed` means the cell has EVER been named
by a `Log` entry, and stays `true` forever once set. It is NOT the same
thing as "currently open." A render layer that shows a cell's face whenever
`Revealed` is true — instead of whenever `Matched` or some player's
`Pending` currently points at it — will leave every card ever flipped
permanently face-up, destroying the memory game `REQ:
open-cards-are-shared-no-attribution` exists to implement. "Open right now"
MUST be computed from `Matched` OR "some player's current `Pending` equals
this cell," never from `Revealed`.

#### REQ: no-reveal-log-rendering

**Founder-ruled** (second round, above: the founder's own reasoning was
that attribution "does not affect game," so nothing built to show it is
needed). The public reveal log MUST NOT be rendered to any player in any
form, in any mode — no "Alice opened B3: 🍇" message, no per-flip
announcement, no "Recently:" list. This is a deletion from what this
Feature previously required (a rendered log WAS required, with
attribution, before the founder's second-round ruling) — see the
requirement mapping table above.

#### REQ: reveal-log-still-feeds-robot-memory

**Removing the log's DISPLAY MUST NOT remove the log ITSELF, stated
explicitly because this is an easy thing to get wrong while implementing
`REQ: no-reveal-log-rendering` above.** `GameState.Log` and its stored
mirror, `pairsession.View.Log`, MUST continue to exist and continue to be
populated exactly as `pair-matching-rules` already specifies
(`REQ: public-reveal-log`) — this is what `MemoryStrategy` reads
(`REQ: memory-window-difficulty`, `REQ:
memory-strategy-prefers-sniping-then-pairing-then-random`). A future
implementer removing the log's rendering MUST NOT also stop populating or
delete the underlying log — doing so would silently reduce every bot above
`Memory == 0` to `RandomMover`'s behavior with no visible symptom beyond
"the AI difficulty stopped feeling different from Random."

#### REQ: no-ownership-markers-or-card-back-art

**Founder-ruled**, and independently corroborated by the shipped code's own
history: `sneat-co/sneat-bots` briefly added per-cell "matched by player X"
badges and removed them (commit history: "fix(sneatbot): plain card faces,
no per-cell ownership markers"), with the shipped code's own comment
recording that "the founder explicitly rejected them and this must not be
reintroduced in any form." The board MUST NOT render any per-cell ownership
marker and MUST NOT use bespoke card-back art beyond the one plain hidden
glyph. Who owns a matched pair is communicated only through the scoreboard,
never on the tile.

#### REQ: face-palette-is-a-fixed-emoji-set-permuted-per-seed

**Implementation fact about the shipped `sneat-co/sneat-bots` code, NOT a
founder ruling** — an earlier draft of this REQ mislabeled it as such.
A rendered face is drawn from a fixed 48-entry emoji palette
(`pairFacePalette`), permuted once per game via a seed-keyed Fisher-Yates
shuffle (`math/rand/v2`'s PCG, deliberately not legacy `math/rand` — see
below) so repeated games do not always show the same emoji for the same
pair id. **Correcting this REQ's own prior rationale:** 48 is deliberate
headroom, not "one entry per possible pair id at the largest preset" — the
largest shipped preset (8×8) has 32 pairs, not 48, and `pairgame.MaxPairs`
(63) is a materially larger, currently-unstated ceiling that a 48-entry
palette does NOT cover; a future preset above 48 pairs would need the
palette extended. For the stored modes, which persist no shuffle seed of
their own (a stored game deals `LayoutInline` and stores the explicit
`Faces` directly — see `pair-matching-sessions`), the render-layer seed is
`stableSeedFor(gameID)`, a deterministic hash of the game ID computed in
`sneat-co/sneat-go` purely for this presentation purpose — it is unrelated
to, and carries none of the security weight of, `pair-matching-rules`'
board-layout seed. Using `math/rand/v2`'s PCG rather than legacy
`math/rand` here is deliberate: legacy `math/rand`'s `Seed(int64)` reduces
its argument modulo 2³¹−1, which would make two seeds that far apart
collide on an identical permutation — a cosmetic defect for this palette
shuffle (not a secrecy one; see `pair-matching-rules`' Testing strategy for
the same class of defect mattering a great deal more in `DeriveFaces`).

### Architecture: the three-repo split

#### REQ: bot-commands-behind-a-host-port

Pair Matching's bot commands MUST live in `sneat-co/sneat-bots`, calling
this repo's `pair-matching-rules`/`pair-matching-sessions` packages only
through a host-port interface the bot package itself declares
(`extensions/sneatbot/cmds4sneatbot.PairService`), never a direct dependency
on a concrete database. This indirection exists purely to avoid a
`sneat-bots` → `sneat-go` import cycle — the same shape GreedGame already
uses (`sneat-bots/extensions/greedgame/bot/cmds4greedgamebot.Service`).

#### REQ: sneat-go-wire-and-configure-only

`sneat-co/sneat-go` (`pkg/modules/pairmatching`) MUST do nothing but bind
the `PairService` port to `pairsession` and a concrete `dal.DB` (via
`facade.GetSneatDB`), translating types and mapping sentinel errors —
mirroring `sneat-go/pkg/modules/greedgame`'s `botService`. It MUST NOT
contain any game logic, session logic, or rendering decision of its own;
per that repo's own constitution, it is wire-and-configure only.

**Correcting this REQ's own prior claim about the secret:**
`sneat-co/sneat-go` does NOT bind `PAIR_MATCHING_GAME_SECRET` at runtime —
`sneat-co/sneat-bots` reads it directly via `os.Getenv` at request time
(`getPairMatchingSecret`), independent of any `sneat-go` wiring.
`sneat-co/sneat-go`'s only relationship to that secret is provisioning it
as a Cloud Run environment variable in its own deploy workflow
(`.github/workflows/deploy-cloudrun.yml`) — infrastructure provisioning,
not a runtime binding this package performs.

## Architecture & Components

- **`pair-matching-rules` / `pair-matching-sessions`** (this repo, both
  Features above) — the sole source of rules, layout, secrecy, and session
  composition for every mode. This Feature calls them; it never re-derives
  them.
- **Pair-Matching bot commands** (`sneat-co/sneat-bots`,
  `extensions/sneatbot/cmds4sneatbot`, shipped and merged — PRs #152–#156
  there) — `pair_command.go` (mode/difficulty/size pickers, the Solo
  callback-data flow, and the stored-mode create/join/start/flip
  handlers), `pair_render.go` (every board/lobby/result rendering path,
  the face palette, and the cell-glyph logic), `pair_deps.go` (the
  `PairService` host-port interface, `PairGameView`/`PairPlayerView`/
  `PairCellView`, `PairMode`, the sentinel errors), `pair_delayers.go`
  (the bot-tick and private-invite-broadcast delayed tasks). Implements
  the host-port `PairService` interface described in
  `REQ: bot-commands-behind-a-host-port`.
- **SneatBot host wiring** (`sneat-co/sneat-go`, `pkg/modules/pairmatching`,
  shipped and merged, WIRE-AND-CONFIGURE-ONLY per that repo's own
  constitution) — `bot_service.go` (`botService`, the `PairService`
  implementation: type translation, error mapping, the difficulty→`Memory`
  constant, `stableSeedFor`), `bot_profile.go` (registers the delayed-task
  workers). Registered inside the existing `@SneatBot` profile (Pair
  Matching contributes no bot profile of its own, unlike GreedGame — see
  `register/pairmatching.go`). This Feature's engine and session work in
  the current repo does not touch `sneat-co/sneat-go`.

## Testing strategy

This repo's own test suites are the regression guard for every rule this
Feature depends on but does not itself re-implement; the shipped bot-layer
test suites (named below by file and function, in `sneat-co/sneat-bots`
and `sneat-co/sneat-go`) are the regression guard for this Feature's own
requirements. **Caveat, stated honestly:** the tests named under Rendering
below cover the CURRENT shipped display model (attribution shown, vs-Humans
pending hidden) — the model `REQ: open-cards-are-shared-no-attribution`
and its sibling REQs above now supersede. Implementing that change is
in-flight in parallel in `sneat-co/sneat-bots`/`sneat-co/sneat-go` as this
Feature is written; the named tests are what exist today and what will
need rewriting, not a claim that the new model is already tested.

- `pair-matching-rules`' `conformance_2018_open_test.go`,
  `TestSoloBudgetMatrix`/`TestSoloAt8x8FitsComfortably`, and the
  `TestDeriveFaces*` tests back `REQ:engine-is-single-source-of-rules` and
  `REQ:solo-callback-data-carries-complete-state`.
- **Three modes / transport:** `TestPairModePicker_HasExpectedButtons`,
  `TestPairCallbackAction_ModePicker` (`sneat-bots/pair_render_test.go`,
  `pair_command_test.go`) back `REQ:three-founder-specified-modes` and
  `REQ:mode-determines-transport`.
- **Solo:** `TestPairSoloCallbackAction_NewGame`,
  `TestPairSoloCallbackAction_SizePicker`,
  `TestPairSoloCallbackAction_InvalidSizeFallsBackToPicker`,
  `TestGetPairMatchingSecret_UnsetEnvReturnsError`/`_SetEnvReturnsValue`,
  `TestPairEntry_SecretUnset`/`_SecretSet`,
  `TestPairMoveCallbackData_RoundTrips`,
  `TestPairMoveCallbackData_FitsCallbackBudget`,
  `TestPairMoveCallbackAction_LegalTapAdvancesBoard`,
  `TestPairMoveCallbackAction_MismatchLeavesBoardPlayable`,
  `TestPairMoveCallbackAction_IllegalTapIsNoOp`,
  `TestPairMoveCallbackAction_ExpiredSnapshot`,
  `TestPairMoveCallbackAction_BadCellCharIsExpired`,
  `TestRenderPairBoard_GameOverShowsNewGame` (all `sneat-bots`) back
  `REQ:solo-uses-seed-derived-layout-only`,
  `REQ:solo-secret-is-host-configuration`,
  `REQ:solo-callback-data-carries-complete-state`, and
  `REQ:solo-illegal-tap-and-game-over-are-noops`. No test in this suite
  exercises `NewSoloGame`'s `ErrSoloBoardTooLarge` path, matching
  `REQ:solo-uses-seed-derived-layout-only`'s own note that this path is
  not reached from the shipped Solo flow.
- **vs Bot:** `TestPairVsBotCallbackAction_ServiceUnavailable`,
  `TestPairVsBotCallbackAction_DifficultyThenSizePicker`,
  `TestPairVsBotCallbackAction_CreatesGameAndSchedulesBotTick`,
  `TestPairVsBotDifficultyPicker_HasExpectedButtons` (`sneat-bots`) back
  `REQ:vs-bot-difficulty-is-two-tiers`. `TestDelayedPairBotFlip_ReEnqueuesWhileUnfinished`,
  `TestDelayedPairBotFlip_StopsReEnqueueingOnceFinished`,
  `TestDelayedPairBotFlip_StopsReEnqueueingWhenBotLeftPlay`,
  `TestDelayedPairBotFlip_ErrorStopsTheChainWithoutFailingTheTask`,
  `TestDelayPairBotFlip_NilDelayerErrors` (`sneat-bots/pair_delayers_test.go`)
  back `REQ:vs-bot-timer-drives-robot-move`.
- **vs Humans:** `TestPairVsHumansCallbackAction_CreatesLobby`,
  `TestPairVsHumansCallbackAction_SizePicker`,
  `TestPairVsHumansCallbackAction_InvalidSizeFallsBackToPicker`,
  `TestPairJoinCallbackAction_JoinsAndReRendersLobby`,
  `TestPairJoinCallbackAction_MissingGameID`,
  `TestPairJoinCallbackAction_ErrorMapsToFriendlyMessage`,
  `TestPairStartCallbackAction_HostOnly`,
  `TestPairStartCallbackAction_NotEnoughPlayers`,
  `TestPairStartCallbackAction_StartsAndRendersBoard`,
  `TestHandlePairJoinDeepLink_ServiceUnavailable`,
  `TestHandlePairJoinDeepLink_EmptyGameID`,
  `TestHandlePairJoinDeepLink_JoinsAndRendersLobby`,
  `TestPairInviteLink_ContainsGameID`,
  `TestPairJoinStartHandler_MatchesOnlyItsOwnPrefix` (`sneat-bots`) back
  `REQ:vs-humans-join-invite-and-start`.
- **Anchored messages:** `TestPairBroadcast_GroupEditsTheOneSharedMessage`,
  `TestPairBroadcast_GroupNoOpWhenNotYetAnchored`,
  `TestPairBroadcast_PrivateInviteEditsEveryOtherAnchoredPlayer`,
  `TestPairBroadcast_ExcludeEmptyEditsEveryAnchoredPlayer`,
  `TestPairBroadcast_SkipsTheBotPlayer`,
  `TestPairCreateStoredGame_GroupChatAnchorsHostChatIDToGroup`,
  `TestPairCreateStoredGame_PrivateChatCapturesHostChatID`,
  `TestPairEnsureAnchor_NoOpWhenAlreadyAnchored`
  (`sneat-bots/pair_delayers_test.go`, `pair_command_test.go`) back
  `REQ:anchored-message-per-mode-shape`.
- **Rendering (current shipped model — see this section's caveat above):**
  `TestRenderPairBoard_PendingCellIsVisible` (Solo; already consistent with
  matched-or-pending face-up, no change needed there once vs-Bot/vs-Humans
  catch up); `TestRenderPairStoredBoard_MatchedCellShowsFace`,
  `TestRenderPairStoredBoard_NeverShowsUnmatchedCellFace`,
  `TestRenderPairStoredBoard_UnmatchedCellsLeakNoLayout` (the SECURITY
  non-interference test, still fully valid under the new model — matched
  cells still show, unmatched-and-not-currently-pending cells still must
  not leak); `TestRenderPairStoredBoard_ShowsScoresAndLog`,
  `TestRenderPairStoredBoard_PendingOnlyShownForVsBot`,
  `TestRenderPairStoredBoard_ShowsOwnPendingForVsBot` assert the display
  model `REQ:open-cards-are-shared-no-attribution` and `REQ:
  no-reveal-log-rendering` now supersede (log rendering, and vs-Bot-only
  pending visibility) — these three will need rewriting or retiring, not
  merely re-reading, as part of the in-flight parallel implementation.
  `TestPairFacePalette_NoDuplicates`, `TestPairFaceEmoji_StableForSameSeedAndPair`,
  `TestPairFaceEmoji_DifferentSeedsLookDifferent`,
  `TestPairFaceEmoji_EveryBoardSizeHasEnoughDistinctFaces` back
  `REQ:face-palette-is-a-fixed-emoji-set-permuted-per-seed`.
- **Architecture:** `TestSetPairService_WiresPackageVar`,
  `TestPairGameView_MatchedPairsAndTotalPairs`, `TestPairGameView_PlayerByID`,
  `TestPairGameView_HasBotInPlay` (`sneat-bots/pair_deps_test.go`) back
  `REQ:bot-commands-behind-a-host-port`. `TestToCellViews_NeverLeaksAnUnmatchedCellsPairID`,
  `TestBotServiceVsBotLifecycle`, `TestBotServiceVsHumansLifecycle`,
  `TestBotServiceMapsSessionErrorsOntoBotSentinels`,
  `TestBotServiceSettersReachTheRealEngine`, `TestCreateGame_RejectsSoloMode`,
  `TestStableSeedForIsDeterministicPerGame` (`sneat-go/pkg/modules/pairmatching`)
  back `REQ:sneat-go-wire-and-configure-only`. `TestToPlayerViews_PendingOnlyPopulatedForVsBot`
  in the same package also asserts the model being superseded (see the
  Rendering note above) and will need the same treatment.

## Not Doing / Out of Scope

- Any game rule, layout derivation, or session/persistence logic of its
  own — all owned by `pair-matching-rules` / `pair-matching-sessions`; see
  `REQ:engine-is-single-source-of-rules`.
- A bot seat joining a vs-Humans game as an extra player — not supported by
  `pair-matching-sessions` today; see that Feature's Open Questions.
- Reaping/expiring an abandoned vs-Humans lobby or an abandoned active
  game — see `pair-matching-sessions`' Open Questions.
- Capping or windowing the public reveal log — see `pair-matching-sessions`'
  Open Questions; unaffected by this Feature's rendering changes above,
  since the log's storage and the bot's use of it as memory both continue
  unchanged (`REQ:reveal-log-still-feeds-robot-memory`) — only its display
  is removed.
- Staking or wagering through the shared game-coins economy — see the
  parent Idea's Open Questions.

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

### AC: solo-is-hardcoded-to-seed-derived-not-budget-gated
**Requirements:** telegram-pair-matching-bot#req:solo-uses-seed-derived-layout-only

**Given** the shipped `pairSoloCallbackAction`
**When** its call into the engine is inspected
**Then** it passes `pairgame.LayoutSeedDerived` as a literal, never a
caller-selectable value, and it calls `pairgame.NewGame` rather than
`NewSoloGame` — so `ErrSoloBoardTooLarge` is never returned from this path,
and every preset in `pairgame.Sizes` is offered by the size picker
unfiltered.

### AC: secret-rotation-corrupts-silently
**Requirements:** telegram-pair-matching-bot#req:solo-secret-is-host-configuration

**Given** `PAIR_MATCHING_GAME_SECRET` is rotated in host configuration
**When** an in-flight Solo game's button, encoded under the old secret, is
tapped again
**Then** `Decode` succeeds (it never touches the secret), the board renders
from the now-wrong `DeriveFaces` output, and nothing in the request path
detects or reports the mismatch — this is silent corruption, not a
recoverable "expired game" message.

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
game blob is stored, and a hand-crafted `callback_data` payload that
`Decode`s successfully is accepted and rendered as-is, with no check that
it could have arisen from real gameplay.

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
a tick whose `RobotMove` call finds a live snipeable exposure can complete
a pair on that single tick — completing a pair is not guaranteed to take
two ticks.

### AC: vs-bot-difficulty-picker-has-exactly-two-tiers
**Requirements:** telegram-pair-matching-bot#req:vs-bot-difficulty-is-two-tiers

**Given** a player starting a vs-Bot game
**When** they are asked to choose the bot's difficulty
**Then** they see exactly two options ("🤖 AI", "🎲 Random"), mapping to
`Player.Memory` values of `1000` and `0` respectively — not a range or a
third tier.

### AC: vs-humans-offers-both-invite-mechanisms
**Requirements:** telegram-pair-matching-bot#req:vs-humans-join-invite-and-start

**Given** a vs-Humans lobby
**When** a player looks for a way to invite others
**Then** both a `t.me/SneatBot?start=pmjoin_<gameID>` deep link and an
in-chat "🙋 Join" button are available — not one or the other depending on
context — and starting the game with fewer than two seated players, or
joining/starting a vs-Bot game, is rejected with user-facing feedback.

### AC: broadcast-skips-unanchored-recipients
**Requirements:** telegram-pair-matching-bot#req:anchored-message-per-mode-shape

**Given** a vs-Humans game with one player who has not yet had a message
anchored for them (`ChatID` or `MessageID` still zero)
**When** the game's state changes and a broadcast/render step runs
**Then** every anchored recipient's message is edited in place, and no
delivery is attempted to the unanchored player.

### AC: any-players-current-pending-pick-shows-face-up
**Requirements:** telegram-pair-matching-bot#req:open-cards-are-shared-no-attribution

**Given** a vs-Humans game where player A currently has a pending pick on
cell X, and player B (not A) is the one viewing the shared board
**When** the board is rendered for B
**Then** cell X shows its true face — visible to B exactly as it is to A —
with no indication anywhere that A (rather than any other player) is the
one holding it pending.

### AC: pending-pick-replaced-goes-back-to-hidden
**Requirements:** telegram-pair-matching-bot#req:open-cards-are-shared-no-attribution, telegram-pair-matching-bot#req:pending-drives-visibility-not-revealed

**Given** a cell that was a player's pending pick (and so showed its face),
then that player flipped a different, non-matching cell, replacing their
pending pick
**When** the board is next rendered
**Then** the now-former-pending cell shows the hidden glyph again, even
though it has been `Revealed` (per `pairsession.CellView.Revealed`) and
will remain `Revealed` forever — the render decision uses current
`Pending`/`Matched`, not `Revealed`.

### AC: no-recent-opens-message-is-ever-sent
**Requirements:** telegram-pair-matching-bot#req:no-reveal-log-rendering

**Given** any game, in any mode, mid-play
**When** any message is rendered — the board, the score line, or any
notification
**Then** no per-flip attribution ("X opened Y") or reveal-log listing
appears anywhere in it.

### AC: bot-memory-strategy-is-unaffected-by-removing-log-display
**Requirements:** telegram-pair-matching-bot#req:reveal-log-still-feeds-robot-memory

**Given** a vs-Bot game with `Memory > 0` and no reveal-log rendering
anywhere in its messages
**When** the bot's `RobotMove` is driven
**Then** `MemoryStrategy` still snipes a live exposure and still opens a
remembered pairing exactly as it would if the log were being rendered —
`GameState.Log`/`pairsession.View.Log` is still fully populated underneath,
only its display was removed.

### AC: no-per-cell-ownership-marker-appears
**Requirements:** telegram-pair-matching-bot#req:no-ownership-markers-or-card-back-art

**Given** a matched pair on the board
**When** the board is rendered
**Then** the matched cells show only their face — no badge, marker, or
distinct card-back art indicating who claimed them; ownership is visible
only in the scoreboard.

### AC: face-emoji-vary-across-games-with-the-same-pair-id
**Requirements:** telegram-pair-matching-bot#req:face-palette-is-a-fixed-emoji-set-permuted-per-seed

**Given** two different games (different seeds) that both need to render
pair id 0
**When** each game's face palette is derived from its own seed
**Then** the two games are not guaranteed to display the same emoji for
pair id 0 — the palette-to-pair-id mapping is permuted per game, and a
board whose pair count exceeds the 48-entry palette (above `MaxPairs`'
theoretical ceiling of 63, though no shipped preset reaches even 48) is out
of this palette's current coverage.

### AC: bot-package-only-reaches-storage-through-the-port
**Requirements:** telegram-pair-matching-bot#req:bot-commands-behind-a-host-port

**Given** the Pair-Matching bot-command package in `sneat-co/sneat-bots`
**When** its imports are inspected
**Then** it depends on a host-port interface it declares itself, and has no
direct import of a concrete database package or of `sneat-co/sneat-go`.

### AC: sneat-go-module-contains-only-wiring-and-does-not-bind-the-secret
**Requirements:** telegram-pair-matching-bot#req:sneat-go-wire-and-configure-only

**Given** `sneat-co/sneat-go`'s `pkg/modules/pairmatching` package
**When** its contents are inspected
**Then** it contains only wiring (binding the `PairService` port to
`pairsession` and a concrete `dal.DB`, plus type/error translation) and no
game rule, session rule, or rendering decision of its own — and, separately,
it contains no code that reads or binds `PAIR_MATCHING_GAME_SECRET` at
runtime; that secret is provisioned only as a Cloud Run environment
variable in this repo's deploy workflow and read directly by
`sneat-co/sneat-bots` via `os.Getenv`.

## Open Questions

None remaining that are specific to this Feature. The two previously open
here — which vs-Bot difficulty tiers to expose, and which vs-Humans invite
mechanism(s) to offer — are answered by shipped code (see
`REQ:vs-bot-difficulty-is-two-tiers` and `REQ:vs-humans-join-invite-and-start`
above) and are no longer open. Remaining open questions for the game as a
whole are tracked in `pair-matching-sessions` (reveal-log length in
storage, abandoned-lobby expiry, a bot joining a multi-human game) and this
repo's `spec/ideas/pair-matching-game.md` (the shared game-coins economy).

---
*This document follows the https://specscore.md/feature-specification*
