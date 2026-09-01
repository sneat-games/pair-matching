package dal4pairgame

import (
	"context"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/record"
)

// GameEntry is the typed dalgo envelope for a stored Pair-Matching game: ID
// (the gameID), Key, the underlying Record, and the strongly typed Data.
type GameEntry = record.DataWithID[string, *GameDbo]

// NewGameEntry builds an (as-yet-unpopulated) envelope for gameID, ready to
// be passed to db.Get / tx.Get / tx.Set.
func NewGameEntry(gameID string) GameEntry {
	return record.NewDataWithID(gameID, newGameKey(gameID), new(GameDbo))
}

// GetGame loads a game outside of any transaction (a plain read). Use
// GetGameTx to read inside an already-open dal.ReadwriteTransaction instead
// — never call GetGame from inside a transaction callback on the same DB:
// on a strict in-memory test DB (and most real backends) that deadlocks or
// violates the backend's read/write ordering rules.
func GetGame(ctx context.Context, db dal.DB, gameID string) (GameEntry, error) {
	entry := NewGameEntry(gameID)
	err := db.Get(ctx, entry.Record)
	return entry, err
}

// GetGameTx loads a game using an already-open read/write transaction's
// Get, so the read participates in that transaction.
func GetGameTx(ctx context.Context, tx dal.ReadSession, gameID string) (GameEntry, error) {
	entry := NewGameEntry(gameID)
	err := tx.Get(ctx, entry.Record)
	return entry, err
}

// SaveGame persists entry in its own read-write transaction (a plain,
// non-conditional write — callers needing get-then-set atomicity should use
// GetGameTx + tx.Set inside their own db.RunReadwriteTransaction instead, as
// the session-composing sibling package's Flip/Join/StartGame do).
func SaveGame(ctx context.Context, db dal.DB, entry GameEntry) error {
	return db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Set(ctx, entry.Record)
	})
}

// IsNotFound reports whether err represents a missing game document.
func IsNotFound(err error) bool {
	return record.IsNotFound(err)
}
