---
format: https://specscore.md/feature-specification
status: Stable
---

# Feature: Pair Matching stored-mode sessions

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/sneat-games/pair-matching/spec/features/pair-matching-sessions?op=explore) | [Edit](https://specscore.studio/app/github.com/sneat-games/pair-matching/spec/features/pair-matching-sessions?op=edit) | [Ask question](https://specscore.studio/app/github.com/sneat-games/pair-matching/spec/features/pair-matching-sessions?op=ask) | [Request change](https://specscore.studio/app/github.com/sneat-games/pair-matching/spec/features/pair-matching-sessions?op=request-change) |
**Status:** Stable
**Source Ideas:** pair-matching-game

## Summary

Server-side persistence and session composition for the two stored Pair-Matching modes (vs Bot, vs Humans): the dal4pairgame DBO and dalgo persistence, and the pairsession API (create/join/start/flip/robot-move/view) plus anchored-message bookkeeping.

## Problem

`pair-matching-rules`' engine has no storage of its own and no concept of
Telegram, application users, or a background timer — by design (see that
Feature's Not Doing section). Two of this game's three founder-specified
modes need exactly those things: **vs Bot** because a bot that moves on its
own timer cannot be reconstructed from a single tapped button's payload the
way Solo's single-human-driven state can, and **vs Humans** because moves
arrive from more than one concurrent actor who each need to see one shared,
contended record rather than N independent copies riding in N different
messages. `sneat-co/backstage`'s `spec/features/games/README.md` names this
exact boundary (`REQ:state-in-callback-data`'s multi-actor carve-out) and
names GreedGame's `dal4greedgame` + session-composing package as the pattern
already proven for exactly this shape. This Feature is Pair Matching's
instance of that same pattern: it owns everything a stored game needs beyond
the pure rules — the record shape, the namespace, the create/join/start/flip
lifecycle, driving the bot one tick at a time, and the anchored-message
bookkeeping a bot's render layer relies on. This document follows the
[SpecScore feature specification](https://specscore.md/feature-specification).

## Behavior

### Persistence & namespace

#### REQ: namespaced-storage

A stored game MUST be a single document keyed
`ext/pairmatching/games/{gameID}` (`dal4pairgame.ExtensionID`,
`GamesCollection`), holding a `GameDbo` whose `Players`/`PairOwner`/`Log`
fields mirror `pairgame.GameState`'s shape plus the session/lifecycle
metadata the pure engine has no reason to carry (`HostUserID`, which of the
two stored kinds it is, `ChatID`/`MessageID`, `Status`, `CreatedAt`). A
stored game's board MUST always be dealt as `pairgame.LayoutInline` with an
explicit `Faces` array, generated once at deal time from a fresh
unpredictable seed (`crypto/rand`-sourced, never a value derived from the
game ID or creation time) — never `LayoutSeedDerived`: the HMAC-derivation
trick exists to hide a layout from a client that holds the payload itself,
and a stored game's board lives only in a server-side record no client ever
reads directly, so that trick buys nothing here (see `pair-matching-rules`'
`REQ: solo-secret-from-env`).

#### REQ: structural-validation-before-persist

Every write MUST pass `GameDbo.Validate()` before being persisted: a
non-empty `HostUserID`, a recognized `Mode`, at least one player, unique
player IDs and unique human `UserID`s, exactly one bot seat for `ModeVsBot`
and zero for `ModeVsHumans`, and a recognized `Status`. This is a structural
gate only — it holds no game rule (match resolution, scoring, turn
legality); those stay entirely in `pair-matching-rules`.

**What this gate does NOT check, stated explicitly:** `Validate()` does not
enforce `pairgame.MaxPlayers` (2..8 for `ModeVsHumans`) as a player-count
ceiling — nothing in `Validate()` itself rejects a `GameDbo` with more than
8 players. That ceiling is enforced only by `JoinGame`'s own
`ErrTooManyPlayers` check on the ordinary join path (see
`REQ:vs-humans-lobby-then-deal`). A `GameDbo` written by any other path —
today there is none in this package, but `Validate()` alone would not catch
one — would not be rejected for exceeding the ceiling.

### Creating & joining a game

#### REQ: vs-bot-dealt-immediately

`CreateVsBotGame` MUST seat the host and one bot (the bot's `Memory`
difficulty supplied by the caller) at creation, deal a fresh board
immediately, and persist the game as `StatusActive` — there is no lobby to
wait on, because both seats are already known.

#### REQ: vs-humans-lobby-then-deal

`CreateVsHumansGame` MUST create a game in `StatusLobby` with only the host
seated. `JoinGame` MUST append a new human seat while the game is in
`StatusLobby` and the seat count is below `pairgame.MaxPlayers`
(`ErrTooManyPlayers` otherwise, `ErrGameNotInLobby` once dealing has started),
and MUST be idempotent for a player who has already joined — re-joining is a
harmless no-op that still opportunistically captures a freshly-seen
`ChatID`. `StartGame` MUST require at least two seated players
(`ErrNotEnoughPlayers` otherwise), deal a fresh board, and transition the
game to `StatusActive`. Calling `JoinGame` or `StartGame` against a vs-Bot
game MUST be rejected (`ErrWrongModeForJoin` / `ErrWrongModeForStart`) — its
seats are fixed at creation and it is never in a lobby.

### Flip & robot-move mechanics

#### REQ: flip-transactional

`Flip` MUST load the game, resolve the calling `userID` to its seat
(`ErrPlayerNotInGame` if not seated), apply `pairgame.Flip`, and persist the
result inside one read-write transaction, so two flips racing against the
same game can never both read the pre-flip state. `Flip` MUST reject a call
against a game that is not `StatusActive` (`ErrGameNotActive`) — before it
has been dealt, or after it has finished.

#### REQ: robot-move-one-flip-per-call

`RobotMove` MUST drive the bot seat of a `ModeVsBot` game
(`ErrNoBotInGame` for anything else) that is `StatusActive`
(`ErrGameNotActive` otherwise) through exactly ONE `pairgame.Flip` call per
invocation, choosing the cell via `pairgame.MemoryStrategy` (falling back to
`RandomMover`) reading the stored game's own public `Log`, all inside one
read-write transaction. Each invocation MUST seed its `RandomMover` fallback
freshly from `crypto/rand` rather than relying on `pairgame.RandomMover`'s
own deterministic nil-`Rand` default, which — called repeatedly across many
separate `RobotMove` invocations, exactly how a timer-driven bot is
driven — would otherwise pick the same relative legal-move position every
time and could cycle the bot between the same cells indefinitely. A caller
drives the bot on its own timer by invoking `RobotMove` once per tick; one
card per tick is deliberate, so the bot's play is paced and visible rather
than resolving a whole turn silently between renders. This does NOT mean
completing a pair costs the bot two ticks: `MemoryStrategy.Choose` opens
with its snipe check (see `pair-matching-rules`' `REQ:
memory-strategy-prefers-sniping-then-pairing-then-random`), so a single
`RobotMove` call can complete a pair outright whenever a live snipeable
exposure is in the bot's remembered window — exactly as
`pair-matching-rules`' `REQ: bot-plays-by-identical-rules` already states
for any flipper, bot or human.

#### REQ: game-completion-transitions-status

When a `Flip` or `RobotMove` call completes the last remaining pair
(`pairgame.GameState.IsComplete`), the stored `Status` MUST transition to
`StatusFinished` in that same transaction — a terminal state after which
`Flip` and `RobotMove` both reject further calls via
`REQ:flip-transactional`'s and `REQ:robot-move-one-flip-per-call`'s
`StatusActive` checks.

### Read model & secrecy

#### REQ: view-exposes-only-revealed-cells

`GetView` MUST project a stored game into a read-only `View` whose
`CellView.PairID` is populated for a cell only once at least one `Log` entry
has ever named that cell (`Revealed == true`); a cell that has never been
flipped MUST NOT disclose its pair id anywhere in the `View`. This is the
stored-mode instance of `pair-matching-rules`' **SECURITY**
`REQ: unmatched-pair-id-never-leaves-server` — a downstream render layer
that builds a Telegram message straight from a `View` inherits this
guarantee for free.

**A real trap for an implementer, stated explicitly:** `Revealed` means
*ever* named by a `Log` entry, and stays `true` forever once set — it is
NOT the same thing as "currently open" (which the founder's board-display
model actually needs — see `telegram-pair-matching-bot`'s rendering
requirements). "Currently open" means a cell that is either matched, or
that some seated player's `Pending` points at right now; a cell can be
`Revealed` (once flipped, mismatched, and long since replaced by that
player's next pending pick) while being neither matched nor anyone's
current pending — and per the founder's decision, THAT cell must render
face-down again. A render layer that treats `CellView.Revealed` as "show
the face" would leave every card ever flipped permanently face-up and
destroy the memory game. `Revealed` exists on `CellView` as a historical
"has this pair's identity ever legitimately become public knowledge"
fact (harmless to expose at this engine-projection layer, since a
`Revealed` cell's pair id was already disclosed through the public `Log`)
— not as a rendering signal. A render layer's "is this cell currently
open" question is answered by matched-or-pending, computed from `Matched`
and each `PlayerView.Pending`, never from `Revealed`.

#### REQ: view-is-not-viewer-dependent

`GetView`'s output MUST NOT vary by who is asking. Under the founder's
board-display model (every open card is visible to every player, with no
attribution and no per-player privacy — see `telegram-pair-matching-bot`),
there is no hidden hand anywhere in this game, at any layer: every matched
pair and every player's current pending pick is public information to
every viewer. One `View` is therefore correct for every viewer of the same
game — this was already true of `GetView`'s shape before that display
model was settled (it has never taken a viewer parameter), and is now also
the *correct* behavior rather than merely a convenient one.

### Anchored messages

#### REQ: group-game-has-one-shared-message

A game anchored to a Telegram group (`GameDbo.ChatID != 0`) MUST have
exactly one shared status message, recorded via `SetGroupMessage`
(idempotent: setting the same `messageID` again is a no-op success) and
edited in place by the host's render layer as players join, flip, and
finish — never a separate message per player.

#### REQ: private-invite-gives-each-player-their-own-message

A private-invite game (`GameDbo.ChatID == 0`, coordinated by DM) MUST give
each seated human player their own anchored board message, recorded via
`SetPlayerMessage` per player (also idempotent), rather than one shared
message — there is no group chat to post a shared status into.

#### REQ: chat-id-captured-opportunistically

`SetPlayerChatID` MUST record a human player's private Telegram chat ID with
the game's bot the first time it becomes known, as a no-op for `chatID == 0`
or an already-current value, and MUST reject a bot seat
(`ErrPlayerIsBot`) — a bot seat never has a chat of its own.

#### REQ: zero-is-the-unanchored-sentinel

`ChatID == 0` MUST mean "no private chat known for this recipient yet," and
`MessageID == 0` MUST mean "no message anchored for this recipient yet," for
both the shared `GameDbo` fields and each `PlayerDbo`'s own fields. A host
broadcast/render layer relies on this zero-value sentinel as its signal to
skip that recipient rather than attempting delivery to an address that does
not exist yet — this Feature's scope is guaranteeing the sentinel's
semantics are exactly this, not the broadcast loop itself (which lives in
the out-of-repo bot layer — see `telegram-pair-matching-bot`).

## Architecture

- **`server-go/pairgame/dal4pairgame`** — the dalgo persistence layer: `dbo.go`
  (`GameDbo`, `PlayerDbo`, `RevealDbo`, `Status`, `Mode`, `Validate`),
  `keys.go` (the `ext/pairmatching/games/{gameID}` key builder), `dal.go`
  (`GetGame`/`GetGameTx`/`SaveGame`/`IsNotFound`), `errors.go`. Deliberately
  does not import `pairgame` — the DBO's field types are plain built-ins, so
  persistence stays decoupled from the rules engine's own types and each
  package compiles and tests independently, mirroring
  `sneat-games/greed-game`'s `dal4greedgame`.
- **`server-go/pairgame/pairsession`** — the session-composing layer:
  `session.go` (`CreateVsBotGame`, `CreateVsHumansGame`, `JoinGame`,
  `StartGame`, `Flip`, `RobotMove`, `GetView`, `SetGroupMessage`,
  `SetPlayerChatID`, `SetPlayerMessage`), `types.go` (`PlayerRef`,
  `CellView`, `PlayerView`, `View`), `convert.go` (`toGameState`,
  `writeBackPlayers`, `writeBackBoard` — the one-way projection into
  `pairgame.GameState` and the two write-back helpers that copy only what
  `Flip`/`RobotMove` can change back onto the persisted `GameDbo`, since
  `GameDbo.Players` carries identity fields `pairgame.Player` has no
  business knowing about), `ids.go` (`NewGameID`, `randomSeed`,
  `newMoveRand`), `errors.go`, `doc.go`. Takes `dal.DB` as an explicit
  parameter from its host and imports no concrete database driver — mirrors
  greedgame's own `CoinWallet`-as-a-port pattern, minus the wallet (this
  game has no economy to abstract over — see the parent Idea's Open
  Questions on that).
- Composes `pair-matching-rules`' `NewGame`/`Flip`/`Strategy` directly; owns
  no game rule of its own.

## Testing strategy

- `dal4pairgame/dal_test.go` covers the key shape
  (`TestNewGameEntry_KeyShape`), a plain save/get round trip
  (`TestSaveAndGetGame_RoundTrip`), transactional reads
  (`TestGetGameTx_ParticipatesInTransaction`), not-found handling
  (`TestGetGame_NotFound`), and `GameDbo.Validate`'s structural gate
  (`TestGameDbo_Validate`) — this is the regression guard for
  REQ:structural-validation-before-persist. `TestGameDbo_Validate` does not
  assert anything about a player count above `pairgame.MaxPlayers`, matching
  that REQ's own note that `Validate()` does not enforce that ceiling.
- `pairsession/session_test.go` is this Feature's main suite, one test per
  API behavior: `TestCreateVsBotGame_DealsAndActivatesImmediately` /
  `TestCreateVsHumansGame_SeedsALobbyWithNoBoardYet` for
  REQ:vs-bot-dealt-immediately / REQ:vs-humans-lobby-then-deal;
  `TestJoinGame_AddsPlayerAndIsIdempotent`,
  `TestJoinGame_RejectsVsBotGame`, `TestJoinGame_RejectsOverflow`,
  `TestJoinGame_RejectsAfterGameStarted`, `TestStartGame_RequiresTwoPlayers`,
  `TestStartGame_DealsTheBoard`, `TestStartGame_RejectsVsBotGame` for the
  rest of REQ:vs-humans-lobby-then-deal; `TestFlip_MatchAndSnipeEndToEnd`,
  `TestFlip_RejectsUnknownPlayer`, `TestFlip_RejectsBeforeGameStarted` for
  REQ:flip-transactional; `TestFlip_MarksGameFinishedOnLastMatch` for
  REQ:game-completion-transitions-status;
  `TestRobotMove_FlipsExactlyOneCardPerCall`,
  `TestRobotMove_RejectsNonBotGame`,
  `TestRobotMove_RejectsBeforeGameActive`,
  `TestRobotMove_RejectsAfterGameFinished` for
  REQ:robot-move-one-flip-per-call (that last test is the one that caught
  the deterministic-fallback-`Rand` bug `REQ:robot-move-one-flip-per-call`
  now documents — it failed to terminate within a generous move budget
  before `newMoveRand` was introduced); `TestGetView_UnrevealedCellsStayHidden`
  is this Feature's own in-repo regression guard for
  REQ:view-exposes-only-revealed-cells — but it currently only proves the
  trivial all-cells-hidden case (a freshly dealt game, zero flips). It does
  NOT yet prove non-interference on a partially-played board (some cells
  matched, some revealed-but-unmatched, some never touched) the way the AC
  below requires — that stronger variant does not exist in this package
  yet; see the AC's own note. The two downstream tests named in
  `telegram-pair-matching-bot`'s Testing strategy
  (`TestRenderPairStoredBoard_UnmatchedCellsLeakNoLayout` in
  `sneat-co/sneat-bots`, `TestToCellViews_NeverLeaksAnUnmatchedCellsPairID` in
  `sneat-co/sneat-go`) guard the same underlying property one layer further
  downstream, at the render/translation layer that consumes this Feature's
  `View`; `TestSetGroupMessage`, `TestSetGroupMessage_NotFound`,
  `TestSetPlayerChatID`, `TestSetPlayerChatID_RejectsBotSeat`,
  `TestSetPlayerMessage`, `TestSetPlayerMessage_RejectsBotSeat`, and
  `TestSettersDoNotTouchRuleState` cover the anchored-message REQs.
- `TestNewGameID_ProducesDistinctIDs` covers ID collision-resistance,
  ancillary to but not itself a named REQ above.

## Not Doing / Out of Scope

- Any game rule (match resolution, scoring, turn legality, robot strategy
  selection) — all of it is called through from `pair-matching-rules`; this
  Feature composes and persists, it does not decide.
- Solo — it never persists a session at all (see `pair-matching-rules`'
  `REQ: solo-only-codec`); there is nothing here for it to compose.
- The actual timer/scheduler that calls `RobotMove` once per tick, and the
  broadcast loop that reads `ChatID`/`MessageID` and skips zero values — both
  are host/bot-layer responsibilities (see `telegram-pair-matching-bot`'s
  Architecture section on the three-repo split); this Feature guarantees the
  contract (`REQ:robot-move-one-flip-per-call`, `REQ:zero-is-the-unanchored-sentinel`)
  those callers rely on, not the calling loop itself.
- The shared game-coins economy — Pair Matching does not stake or wager
  through this Feature; see the parent Idea's Open Questions.
- Reaping/expiring an abandoned lobby or an abandoned active game — see Open
  Questions below.

## Acceptance Criteria

### AC: stored-game-lives-under-the-extension-namespace
**Requirements:** pair-matching-sessions#req:namespaced-storage

**Given** any game created by `CreateVsBotGame` or `CreateVsHumansGame`
**When** its document key is inspected
**Then** it is `ext/pairmatching/games/{gameID}`, and its board is stored as
an explicit `LayoutInline` `Faces` array dealt from a fresh, unpredictable
seed.

### AC: malformed-game-cannot-be-persisted
**Requirements:** pair-matching-sessions#req:structural-validation-before-persist

**Given** a `GameDbo` missing a required field, carrying a duplicate player
ID or `UserID`, or seating the wrong bot count for its `Mode`
**When** `Validate()` is called (as every persist path does)
**Then** it returns a descriptive error and the record is not written.

### AC: validate-does-not-catch-an-oversized-player-list
**Requirements:** pair-matching-sessions#req:structural-validation-before-persist

**Given** a `GameDbo` constructed directly (not through `JoinGame`) with
more than 8 players, otherwise well-formed
**When** `Validate()` is called on it
**Then** it returns `nil` (no error) — `Validate()` alone does not reject an
oversized player list; only `JoinGame`'s own check does, on the ordinary
join path.

### AC: vs-bot-game-is-immediately-playable
**Requirements:** pair-matching-sessions#req:vs-bot-dealt-immediately

**Given** a call to `CreateVsBotGame`
**When** the returned game is loaded
**Then** it already has two seated players (host and bot), a dealt board,
and `StatusActive` — no separate "start" call is needed.

### AC: vs-humans-lobby-fills-then-deals
**Requirements:** pair-matching-sessions#req:vs-humans-lobby-then-deal

**Given** a `CreateVsHumansGame` call followed by two or more `JoinGame`
calls
**When** `StartGame` is then called
**Then** the game transitions from `StatusLobby` to `StatusActive` with a
freshly dealt board; calling `StartGame` with fewer than two seated players,
or calling `JoinGame`/`StartGame` against a vs-Bot game, is rejected instead.

### AC: concurrent-flips-cannot-race
**Requirements:** pair-matching-sessions#req:flip-transactional

**Given** an active game
**When** `Flip` is called for a seated player
**Then** the read of the pre-flip state and the write of the post-flip state
happen inside one transaction (`tx.Get` then `tx.Set` on the same
`dal.ReadwriteTransaction`), and a call from an unseated `userID`, or
against a game that is not `StatusActive`, is rejected instead. The
"cannot observe or act on the stale pre-flip state" half of this claim is
NOT independently exercised by anything in this repo today: the in-memory
`dal.DB` this package's tests run against (`testdb_test.go`) has no
documented contention semantics, so no test here can fail if that guarantee
were broken — this AC is verified by code inspection of the transaction
boundary, not by a concurrency test.

### AC: robot-move-flips-exactly-one-card
**Requirements:** pair-matching-sessions#req:robot-move-one-flip-per-call

**Given** an active vs-Bot game
**When** `RobotMove` is called repeatedly, once per simulated tick, until the
board is complete
**Then** each call flips exactly one card, the sequence terminates (it does
not cycle indefinitely between the same cells), and calling `RobotMove`
against a vs-Humans game, a not-yet-active game, or a finished game is
rejected instead.

### AC: last-match-finishes-the-game
**Requirements:** pair-matching-sessions#req:game-completion-transitions-status

**Given** a game with exactly one pair left unmatched
**When** a `Flip` or `RobotMove` call completes that last pair
**Then** the stored `Status` becomes `StatusFinished` in that same call, and
a further `Flip` or `RobotMove` call against the game is rejected.

### AC: never-flipped-cells-do-not-leak-through-the-view
**Requirements:** pair-matching-sessions#req:view-exposes-only-revealed-cells

**Given** a game with a mix of matched cells, revealed-but-unmatched cells,
and cells that have never been flipped
**When** every never-flipped cell's true `Faces` entry (its real pair id) is
overwritten with a poisoned sentinel value in the stored `GameDbo` before
calling `GetView` — the non-interference shape of `sneat-co/sneat-bots`'
`TestRenderPairStoredBoard_UnmatchedCellsLeakNoLayout`, adapted to this
package's `Revealed`-gated (not `Matched`-gated) contract
**Then** every never-flipped cell's `CellView` is byte-identical to what it
would have been with the true pair id in place (`Revealed == false`, and
`PairID` unaffected by the poisoned value) — proving `GetView` never reads
a never-flipped cell's true pair id into the `View` at all, not merely that
it happens not to. This AC is stronger than, and not yet satisfied by, the
existing `TestGetView_UnrevealedCellsStayHidden` (see Testing strategy
above) — that test only covers the trivial zero-flip case.

### AC: view-is-identical-for-every-caller
**Requirements:** pair-matching-sessions#req:view-is-not-viewer-dependent

**Given** `GetView`'s function signature
**When** it is inspected
**Then** it takes no viewer/caller identity parameter at all — this AC is
true by construction (there is no code path that could branch on a viewer
that was never passed in), not something a test can meaningfully fail;
recorded here for traceability to the REQ, not as a falsifiable check.

### AC: group-game-status-message-is-singular
**Requirements:** pair-matching-sessions#req:group-game-has-one-shared-message

**Given** a `CreateVsHumansGame` (or `CreateVsBotGame`) call with a non-zero
`chatID`
**When** `SetGroupMessage` is called, including a second time with the same
`messageID`
**Then** the game carries exactly one `MessageID`, and the repeat call is a
no-op success rather than a second write.

### AC: private-invite-players-each-get-their-own-message
**Requirements:** pair-matching-sessions#req:private-invite-gives-each-player-their-own-message

**Given** a `CreateVsHumansGame` call with `chatID == 0`
**When** `SetPlayerMessage` is called for two different seated players
**Then** each player's own `MessageID` is recorded independently, with no
shared `GameDbo.MessageID` involved.

### AC: bot-seat-rejects-chat-and-message-setters
**Requirements:** pair-matching-sessions#req:chat-id-captured-opportunistically

**Given** a vs-Bot game
**When** `SetPlayerChatID` or `SetPlayerMessage` is called for the bot's
seat
**Then** it returns `ErrPlayerIsBot` and no field on the bot's seat changes.

### AC: zero-chat-and-message-ids-mean-unanchored
**Requirements:** pair-matching-sessions#req:zero-is-the-unanchored-sentinel

**Given** a newly created game before any `SetGroupMessage`/`SetPlayerChatID`/
`SetPlayerMessage` call
**When** its `View` is inspected
**Then** every `ChatID` and `MessageID` field reads zero, correctly signaling
"nothing to deliver to here yet" to a caller that has not anchored anything.

## Open Questions

- The public reveal log is unbounded in this Feature's storage (`GameDbo.Log`
  grows by one entry per flip, never trimmed), but a downstream render layer
  puts it into a single Telegram message, which has a length limit. Does the
  log need capping or windowing for a long 8x8 (64-cell, 32-pair) game before
  that becomes a real rendering failure rather than a theoretical one?
- Should an abandoned `vs Humans` game — stuck in `StatusLobby` with nobody
  ever calling `StartGame`, or `StatusActive` with nobody ever flipping
  again — expire? Nothing in this Feature currently reaps either case; a
  game persists indefinitely once created.
- Should a bot ever be able to join a `vs Humans` game as an extra player?
  Today `REQ:structural-validation-before-persist` requires zero bots for
  `ModeVsHumans`, and there is no path to add one after creation.

---
*This document follows the https://specscore.md/feature-specification*
