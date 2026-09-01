package pairsession

import "errors"

// Sentinel errors returned by the session API. Use errors.Is to test.
var (
	// ErrGameNotFound is returned when gameID does not identify a game.
	ErrGameNotFound = errors.New("pairsession: game not found")

	// ErrGameNotInLobby is returned by JoinGame/StartGame when the game has
	// already started (or finished) — vs-humans only accepts new joins, and
	// only needs starting, while still in its Lobby.
	ErrGameNotInLobby = errors.New("pairsession: game is no longer accepting new players")

	// ErrGameNotActive is returned by Flip/RobotMove when the game has not
	// been started yet (still in the lobby) or has already finished.
	ErrGameNotActive = errors.New("pairsession: game is not accepting flips right now")

	// ErrNotEnoughPlayers is returned by StartGame when fewer than two
	// players have joined.
	ErrNotEnoughPlayers = errors.New("pairsession: at least 2 players are required to start")

	// ErrTooManyPlayers is returned by JoinGame once a vs-humans game
	// already has pairgame.MaxPlayers seated.
	ErrTooManyPlayers = errors.New("pairsession: this game already has the maximum number of players")

	// ErrPlayerNotInGame is returned when userID is not a seated player.
	ErrPlayerNotInGame = errors.New("pairsession: you are not a player in this game")

	// ErrWrongModeForJoin is returned by JoinGame against a vs-Bot game —
	// its two seats are both fixed at creation; there is nothing to join.
	ErrWrongModeForJoin = errors.New("pairsession: only a vs-humans game accepts new players")

	// ErrWrongModeForStart is returned by StartGame against a vs-Bot game —
	// it is dealt and Active immediately at creation; there is no lobby to
	// start.
	ErrWrongModeForStart = errors.New("pairsession: only a vs-humans game needs to be started")

	// ErrNoBotInGame is returned by RobotMove against anything but a
	// vs-Bot game.
	ErrNoBotInGame = errors.New("pairsession: this game has no bot seat")
)
