package pairgame

import (
	"bytes"
	"github.com/pkg/errors"
	"github.com/prizarena/turnbased"
	"github.com/sneat-games/pair-matching/server-go/pairmodels"
	"strings"
)

var (
	ErrAlreadyMatched   = errors.New("already matched")
	ErrAlreadyOpen      = errors.New("already open")
	ErrBoardIsCompleted = errors.New("board is already completed")
)

func OpenCell(
	board *pairmodels.PairsBoardEntity, ca turnbased.CellAddress, player pairmodels.PairsPlayer, players []pairmodels.PairsPlayer,
) (
	changed bool, matchedTile string, changedPlayers []pairmodels.PairsPlayer, err error,
) {
	if ca == "" {
		panic("Cell address is required to open a cell")
	}
	if len(players) == 0 {
		panic("len(players) == 0")
		// players = []pairmodels.PairsPlayer{player}
	}
	if board.IsCompleted(players) {
		err = ErrBoardIsCompleted
		return
	}
	currentlyOpened := board.GetCell(ca)

	if player.Open1 != "" && player.Open2 != "" {
		// If player has 2 tiles opened close them before opening next one
		changed = true
		player.Open1 = ""
		player.Open2 = ""
	}

	playerChanged := func(p pairmodels.PairsPlayer) {
		for _, cp := range changedPlayers {
			if cp.ID == p.ID {
				return
			}
		}
		changedPlayers = append(changedPlayers, p)
	}

	{ // Close already matched cells
		allAlreadyMatchedItems := func() string {
			var s bytes.Buffer
			for _, p := range players {
				s.WriteString(p.MatchedItems)
			}
			return s.String()
		}()
		closeAlreadyMatched := func(p pairmodels.PairsPlayer) (pChanged bool) {
			if p.Open1 != "" && strings.Contains(allAlreadyMatchedItems, board.GetCell(p.Open1)) {
				p.Open1 = ""
				pChanged = true
			}
			if p.Open2 != "" && strings.Contains(allAlreadyMatchedItems, board.GetCell(p.Open2)) {
				p.Open2 = ""
				pChanged = true
			}
			return
		}
		changed = closeAlreadyMatched(player) || changed
		for _, p := range players {
			if closeAlreadyMatched(p) {
				changedPlayers = append(changedPlayers, p)
			}
		}
	}

	atLeastOneMatched := false

	for _, p := range players {
		if p.IsAlreadyMatched(currentlyOpened) {
			err = ErrAlreadyMatched
			if p.Open1 != "" && board.GetCell(p.Open1) == currentlyOpened {
				p.Open1 = ""
				playerChanged(p)
			}
			if p.Open2 != "" && board.GetCell(p.Open2) == currentlyOpened {
				p.Open2 = ""
				playerChanged(p)
			}
			return
		}

		match := func(openN int, openField turnbased.CellAddress) (matched bool) {
			if p.ID == player.ID && (p.Open1 == ca || p.Open2 == ca) {
				err = ErrAlreadyOpen
				return
			}
			if matched = board.GetCell(openField) == currentlyOpened; matched {
				playerChanged(p)
			}
			return
		}

		isMatched := p.Open1 != "" && match(1, p.Open1)
		if err != nil {
			return
		}
		if !isMatched {
			isMatched = p.Open2 != "" && match(2, p.Open2)
			if err != nil {
				return
			}
		}

		if isMatched {
			atLeastOneMatched = true
			// Do not break as theoretically in case of race could match more than 1.
		}
	}

	player.FlipsCount++

	if atLeastOneMatched {
		changed = true
		if player.Open1 == "" {
			player.Open1 = ca
		} else if player.Open2 == "" {
			player.Open2 = ca
		}
		player.MatchedCount++
		if player.MatchedItems == "" {
			player.MatchedItems += currentlyOpened
		} else {
			player.MatchedItems += "," + currentlyOpened
		}
		matchedTile = currentlyOpened
		return
	}

	if player.Open1 == "" {
		changed = true
		player.Open1 = ca
	} else if player.Open2 == "" {
		changed = true
		player.Open2 = ca

		// This check is just in case. Actually should be caught by match() function above.
		// if alreadyOpened := board.GetCell(player.Open1); alreadyOpened == currentlyOpened {
		// 	player.Open2 = ca
		// 	player.MatchedCount++
		// 	player.MatchedItems += string(currentlyOpened)
		// 	return
		// }
	} else if player.Open1 != "" && player.Open2 != "" {
		changed = true
		player.Open1 = ca
		player.Open2 = ""
	} else {
		panic("should not be here")
	}
	return
}
