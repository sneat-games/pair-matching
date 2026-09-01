package pairgame

import "errors"

var (
	// ErrGameOver is returned by Flip once every pair is matched.
	ErrGameOver = errors.New("pairgame: game is already complete")
	// ErrInvalidCell is returned for a cell index outside the board.
	ErrInvalidCell = errors.New("pairgame: invalid cell index")
	// ErrInvalidSizeIndex is returned by NewGame for a sizeIndex outside
	// Sizes.
	ErrInvalidSizeIndex = errors.New("pairgame: invalid board size index")
	// ErrInvalidPlayerCount is returned by NewGame when the number of seats
	// requested is fewer than 1 or more than MaxPlayers.
	ErrInvalidPlayerCount = errors.New("pairgame: invalid number of players")
	// ErrInvalidMemory is returned by NewGame for a negative PlayerSetup.Memory.
	ErrInvalidMemory = errors.New("pairgame: bot memory must not be negative")
	// ErrUnknownPlayer is returned by Flip when `by` does not identify a
	// seated player in this GameState.
	ErrUnknownPlayer = errors.New("pairgame: unknown player id")
	// ErrCellAlreadyMatched is returned when flipping a cell whose pair is
	// already matched — by anyone, not just the acting player (see Flip's
	// doc comment on sniping an opponent's exposed pair).
	ErrCellAlreadyMatched = errors.New("pairgame: cell is already matched")
	// ErrCellIsPending is returned when a player flips the cell that is
	// already THEIR OWN pending pick (another player may independently hold
	// the same cell pending at the same time — that is not this error).
	ErrCellIsPending = errors.New("pairgame: cell is already this player's pending pick")
	// ErrSoloBoardTooLarge is returned by NewSoloGame for a (mode, sizeIndex)
	// whose solo snapshot would not fit the callback-data budget — see
	// Fits.
	ErrSoloBoardTooLarge = errors.New("pairgame: this board size does not fit the solo callback-data budget under this layout mode")
	// ErrNotSoloGame is returned by Encode for any GameState that is not the
	// solo shape (exactly one human player, no bot) — see Encode's doc
	// comment. The stored modes (vs-bot, vs-humans) never round-trip through
	// callback_data at all; see dal4pairgame.
	ErrNotSoloGame = errors.New("pairgame: Encode only supports a solo (single human player, no bot) game state")
	// ErrInvalidSnapshot is returned by Decode for a payload that fails to
	// parse: an unsupported version, a size index outside Sizes, or a
	// payload that ends before a field Decode expected to find has been
	// fully read. Decode performs no further semantic validation beyond
	// what each field's fixed width already constrains — e.g. it does not
	// check that a decoded pending pick could ever have arisen from real
	// gameplay.
	ErrInvalidSnapshot = errors.New("pairgame: invalid snapshot payload")
)
