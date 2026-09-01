package pairgame

import "errors"

var (
	// ErrGameOver is returned by Reveal once every pair is matched.
	ErrGameOver = errors.New("pairgame: game is already complete")
	// ErrInvalidCell is returned for a cell index outside the board or a
	// sizeIndex outside Sizes.
	ErrInvalidCell = errors.New("pairgame: invalid cell index")
	// ErrCellAlreadyMatched is returned when revealing a cell whose pair is
	// already matched.
	ErrCellAlreadyMatched = errors.New("pairgame: cell is already matched")
	// ErrCellIsPending is returned when revealing the cell that is already
	// this turn's pending first pick.
	ErrCellIsPending = errors.New("pairgame: cell is already the pending pick")
	// ErrInvalidDifficulty is returned by NewGame when N would make the
	// worst-case snapshot exceed the configured budget — see MaxDifficulty.
	ErrInvalidDifficulty = errors.New("pairgame: difficulty N does not fit the callback-data budget")
	// ErrInvalidSnapshot is returned by Decode for a payload that is
	// malformed, truncated, or internally inconsistent (e.g. a pending pick
	// or memory entry pointing at an already-matched cell).
	ErrInvalidSnapshot = errors.New("pairgame: invalid snapshot payload")
)
