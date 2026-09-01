---
format: https://specscore.md/idea-specification
status: Specifying
---

# Idea: Pair Matching game

**Status:** Specifying
**Date:** 2026-09-01
**Owner:** alexander.trakhimenok@gmail.com
**Promotes To:** pair-matching-rules, pair-matching-sessions, telegram-pair-matching-bot
**Supersedes:** —
**Related Ideas:** —

## Problem Statement

How might we bring the Pair-Matching (memory/concentration) game back into Sneat bots — across Solo, vs Bot, and vs Humans — restoring the founder-confirmed 2018 flip rules that made the original 'really work', without reviving its dead Google App Engine + datastore delivery stack?

## Context

Pair Matching (a memory/concentration game) previously lived as a standalone 2018 Google App Engine app under prizarena/pair-matching (archived here on branch archive/2018-original-engine, commit 9f2a078), storing board and player state via strongo/db behind strongo/bots-framework-era bot wiring. The founder revived it because 'it really worked'. That delivery stack is dead — archived predecessor packages, server-side per-game records that sneat-co/backstage's games portfolio feature (spec/features/games/README.md) rules out by default. What survived the rewrite is the game's proven rules: no turn order, each player holds an independent pending pick, any flip may snipe any exposed pending pick and credits the flipper — restored and regression-guarded by server-go/pairgame/conformance_2018_open_test.go, which replays the original's TestOpenCell case_1 six steps end to end. Pair Matching is named in backstage alongside GreedGame as one of the two games exempted from the portfolio's Solo-only v1 default (REQ:solo-vs-robot-v1), because it ships three founder-ruled modes: Solo (one human, no bot, callback-data-only — the original 2018 mode), vs Bot (one human + one bot, server-side session), and vs Humans (2-8 humans, no bot, server-side session).

## Recommended Direction

Ship all three modes — Solo, vs Bot, vs Humans — as one coherent game under SneatBot's /games, built on this repo's existing server-go/pairgame rules engine plus its dal4pairgame/pairsession persistence layer for the two stored modes. The flip rules are identical in every mode (no turn order, independent per-player pending picks, any-player sniping credited to the flipper, a public append-only reveal log as the game's shared memory) — the only thing that changes between modes is where state lives: Solo rides in Telegram callback_data (pairgame.LayoutSeedDerived, HMAC-derived from PAIR_MATCHING_GAME_SECRET so an unmatched cell's pair id never leaves the server), while vs Bot and vs Humans are stored server-side under ext/pairmatching/games/{gameID} because a bot moving on its own timer and multiple concurrent human actors cannot round-trip through one tapped button. This mirrors GreedGame's already-proven three-repo split (engine+dal4*+session in the game's own repo, bot commands in sneat-co/sneat-bots behind a host-port interface, sneat-co/sneat-go doing nothing but wiring) rather than inventing a new architecture. Solo, vs Bot, and vs Humans all matter to the product the founder specified — none is a stretch goal added on top of a solo-only MVP.

## Alternatives Considered

- **Solo-only relaunch, matching Reversi/RPS's default REQ:solo-vs-robot-v1 shape.** Lost because backstage already names Pair Matching, alongside GreedGame, as an explicit exception to that default — the founder specified vs Bot and vs Humans as first-class modes, not a later stretch goal, and the original 2018 game's whole appeal ("it really worked") was its multi-player race, not a solo puzzle.
- **Strict-alternating-turn engine, one shared pending pick per game.** This was the rewrite's own first draft before the founder ruled on it. Lost twice, on the record: first when the founder confirmed there is no turn order at all, and again — prompted by the rewrite's own doc comments flagging the ambiguity — when the founder confirmed a flip may match ANY seated player's exposed pending pick, credited to the flipper. Both rulings restore the archived 2018 `OpenCell`'s actual behavior (see `conformance_2018_open_test.go`); the alternating-turn draft would have shipped a different, unproven game under the same name.
- **Keep the 2018 Google App Engine + Datastore delivery stack, port only what's necessary.** Lost because it depends entirely on archived `strongo/bots-framework`-era packages that no longer exist as live dependencies, and — more fundamentally — it assumes server-side persistence for every mode, which backstage's REQ:state-in-callback-data rules out as the Solo default. Reviving the deployment, rather than the rules, was never what "the founder revived it" meant.

## MVP Scope

One full Pair-Matching game played to completion, in each of the three modes, through SneatBot's /games — using the exact founder-restored 2018 flip rules (sniping included) and the exact secrecy properties (Solo's layout never leaves the server in the clear; a stored game's unmatched cells stay unrevealed to every viewer until actually flipped). Ships as: a Solo callback-data flow with a bot-side derivation secret in Cloud Run/CI config; a vs-Bot flow with the bot's memory-window difficulty dial and its one-flip-per-timer-tick pacing; a vs-Humans flow with a join lobby, 2-8 seats, and one shared anchored status message per group game (or one anchored message per player for a private invite). Not timeboxed by mode count — timeboxed by 'one clean playthrough of each already-founder-ruled mode', since the rules themselves are not being re-litigated here.

## Not Doing (and Why)

- Reviving the 2018 Google App Engine + Datastore delivery stack (pairbot/pairapp/pairgaeroot/pairsecrets/pairdal/pairgaedal/pairtrans, appengine-tagged pairmodels) — dead infra on archived predecessor packages, and it assumes server-side state for every mode, which contradicts REQ:state-in-callback-data's Solo default
- Per-cell ownership markers or bespoke card-back art on the board — the founder explicitly rejected both; a cell shows its face once matched, a plain hidden glyph otherwise, nothing else
- Real-money wagering or any non-coin stake on a game — Sneat's casual games stay non-gambling by construction; even joining the existing free, non-cashable game-coins economy (as GreedGame and Bidding Tic-Tac-Toe do) is an open product question here, not an assumption
- A bot seat joining a vs-Humans game as an extra player, or any matchmaking/ranking/tournament layer on top of a single game — today a game is exactly Solo, or one bot, or 2-8 humans, decided at creation

## Key Assumptions to Validate

| Tier | Assumption | How to validate |
|------|------------|-----------------|
| Must-be-true | The restored 2018 flip rules (no turn order, any-player sniping credited to the flipper) are what actually made the original game work, not just nostalgia for the app that carried them | Ship vs Bot and vs Humans and watch whether games are completed and replayed, not abandoned after the first sniped pair |
| Should-be-true | An unbounded public reveal log renders acceptably inside a single Telegram message, including at the 8x8/64-cell preset in a vs-Humans group game | Playtest an 8x8 vs-Humans game before deciding whether `telegram-pair-matching-bot`'s open question on capping/windowing the log needs an answer |
| Might-be-true | Players will want Pair Matching staked in the shared game-coins economy the way GreedGame and Bidding Tic-Tac-Toe are | Ask after vs-Bot/vs-Humans ship whether players request stakes, rather than pre-building wallet integration speculatively |

## SpecScore Integration

- **New Features this would create:** `pair-matching-rules` (the rules engine), `pair-matching-sessions` (stored-mode persistence), `telegram-pair-matching-bot` (the three-mode Telegram surface, rewriting the existing solo-vs-robot draft)
- **Existing Features affected:** `telegram-pair-matching-bot` (rewritten — its current title and several requirements describe a superseded vs-robot/callback-data-only design)
- **Dependencies:** `sneat-co/backstage`'s `spec/features/games/README.md` (portfolio-level REQ:engine-per-repo, REQ:state-in-callback-data, REQ:solo-vs-robot-v1); the GreedGame three-repo split this follows

## Open Questions

- Should Pair Matching participate in the shared game-coins economy (as GreedGame and Bidding Tic-Tac-Toe do)? It currently does not, and this Idea's "Not Doing" section treats that as undecided rather than ruled out. This is a portfolio/product-economics decision that spans more than one downstream Feature, so it is tracked here rather than under a single Feature's Open Questions.
