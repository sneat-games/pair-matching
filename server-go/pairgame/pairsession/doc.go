// Package pairsession is the host-agnostic session layer for the two
// STORED Pair-Matching modes — vs-Bot (one human + one bot) and vs-Humans
// (2..8 humans) — composing the pure rules engine
// (server-go/pairgame: NewGame, Flip, Strategy) with its persistence layer
// (server-go/pairgame/dal4pairgame) the same way
// sneat-games/greed-game/server-go/greedgame composes greedplay with
// dal4greedgame. It owns:
//
//   - creating a vs-Bot game (both seats known up front, dealt and Active
//     immediately) or a vs-Humans game (a Lobby that Join fills before
//     StartGame deals the board);
//   - Flip, which loads a game, applies pairgame.Flip for the acting human,
//     and persists the result — all inside one read-write transaction;
//   - RobotMove, which does the same for the game's bot seat, driving
//     pairgame.MemoryStrategy over the STORED game's own public Log and
//     flipping exactly ONE card per call (see RobotMove's doc comment);
//   - GetView, a read-only projection for a render/host layer.
//
// Solo is deliberately absent from this package: it never persists a
// session at all (see server-go/pairgame's snapshot.go and package doc) —
// there is nothing here for it to compose.
//
// # Host wiring
//
// This package takes its persistence (dal.DB) as an explicit parameter
// from the host — it imports no concrete database driver. A host (e.g.
// sneat-co/sneat-go) wires a real dal.DB and calls into this package;
// nothing in this package ever imports the host module. This mirrors
// greedgame's own CoinWallet-as-a-port pattern, minus the wallet: Pair-
// Matching has no economy to abstract over.
//
// # Why a stored game never uses LayoutSeedDerived
//
// pairgame.LayoutSeedDerived exists to hide the board layout from anyone
// who can read a Telegram callback_data payload — a real concern for Solo,
// which puts the whole game state on the wire. A stored game's board lives
// only in dal4pairgame.GameDbo.Faces, a server-side record no client ever
// reads directly; the server itself decides what a view exposes (see
// view.go), so the HMAC-derivation trick buys nothing here. Every game this
// package creates therefore uses pairgame.LayoutInline, with the actual
// per-cell layout generated once (a fresh random seed via ShuffleFaces) at
// deal time and stored explicitly.
//
// # Why every reveal is fully public in the stored view
//
// Per the founder's rules (see pairgame's Flip doc comment), every flip —
// first pick or second, matched or not — is logged publicly. That log
// entry already discloses the pair id under that cell to every player
// permanently, whether or not the pair is later matched. There is
// therefore no per-viewer secrecy left to enforce in view.go beyond "has
// any log entry ever mentioned this cell" — unlike, say, a game with a
// legitimately hidden hand, GetView's output does not depend on which
// player is asking.
package pairsession
