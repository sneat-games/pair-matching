package paircommands

import (
	"bytes"
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/prizarena/prizarena-public/pamodels"
	"github.com/prizarena/turnbased"
	"github.com/sneat-games/pair-matching/server-go/pairdal"
	"github.com/sneat-games/pair-matching/server-go/pairgame"
	"github.com/sneat-games/pair-matching/server-go/pairmodels"
	"github.com/strongo/bots-api-telegram"
	"github.com/strongo/bots-framework/core"
	"github.com/strongo/bots-framework/platforms/telegram"
	"github.com/strongo/db"
	"github.com/strongo/log"
	"net/url"
	"strconv"
	"time"
)

const openCellCommandCode = "open"

func openCellCallbackData(ca turnbased.CellAddress, playersCount int, boardID, userID, lang string) string {
	s := new(bytes.Buffer)
	s.WriteString(openCellCommandCode)
	fmt.Fprintf(s, "?c=%v&p=%v&l=%v", ca, playersCount, lang)
	// fmt.Fprintf(s, "?c=%v&p=%v&b=%v&l=%v", ca, playersCount, boardID, lang)
	if playersCount == 1 {
		s.WriteString("&u=" + userID)
	}
	if boardID != "" {
		s.WriteString("&b=" + boardID)
	}
	return s.String()
}

var openCellCommand = bots.NewCallbackCommand(openCellCommandCode, openCellCallbackAction)

func openCellCallbackAction(whc bots.WebhookContext, callbackUrl *url.URL) (m bots.MessageFromBot, err error) {
	c := whc.Context()
	log.Debugf(c, "openCellCommand.CallbackAction()")

	q := callbackUrl.Query()
	if err = whc.SetLocale(q.Get("l")); err != nil {
		return
	}

	var data updateDbContext

	if data.ca, err = getCellAddress(q, "c"); err != nil {
		err = errors.WithMessage(err, "parameter 'c' is not a valid cell address")
		return
	}

	if data.playersCount, err = strconv.Atoi(q.Get("p")); err != nil {
		err = errors.WithMessage(err, "parameter 'p' (e.g. playersCount) is not an integer")
		return
	} else if data.playersCount < 0 {
		err = fmt.Errorf("bad request: playersCount expected to be 0 or greater, got: %v", data.playersCount)
		return
	}

	if data.board.ID = q.Get("b"); data.board.ID == "" {
		if tgCallbackQuery, ok := whc.Input().(telegram.TgWebhookCallbackQuery); ok {
			data.board.ID = tgCallbackQuery.GetInlineMessageID()
		}
		if data.board.ID == "" {
			err = errors.New("bad request: parameter 'b' e.g. board ID is required")
			return
		}
	}

	userID := whc.AppUserStrID()

	isNewUser := data.playersCount == 0 || q.Get("u") != userID

	if data.playersCount > 1 {
		if err = pairdal.DB.Get(c, &data.board); err != nil {
			return
		}
		data.playerEntityHolders = make([]db.EntityHolder, 0, len(data.board.UserIDs)+1)
		for _, boardUserID := range data.board.UserIDs {
			if boardUserID == userID {
				isNewUser = false
			} else {
				data.playerEntityHolders = append(data.playerEntityHolders, &pairmodels.PairsPlayer{StringID: db.NewStrID(data.board.ID + ":" + boardUserID)})
			}
		}
	} else if isNewUser {
		data.playerEntityHolders = make([]db.EntityHolder, 0, data.playersCount+1)
	} else {
		data.playerEntityHolders = make([]db.EntityHolder, 0, 1)
	}

	if data.playersCount > 1 {
		if isNewUser {
			err = pairdal.DB.RunInTransaction(c, func(tc context.Context) (err error) {
				if err = pairdal.DB.Get(tc, &data.board); err != nil {
					return
				}
				data.userName, data.board.BoardEntityBase, err = turnbased.BoardUsersManagers{}.AddUserToBoard(
					c, userID, data.userName, data.board.PairsBoardEntity.BoardEntityBase, whc.GetAppUser)
				if err != nil {
					return
				}
				if err = pairdal.DB.Update(tc, &data.board); err != nil {
					return
				}
				return
			}, db.SingleGroupTransaction)
			if err != nil {
				return
			}
		} else {
			for i, uID := range data.board.UserIDs {
				if uID == userID {
					data.userName = data.board.UserNames[i]
				}
			}
		}
	}

	if len(data.playerEntityHolders) > 0 {
		if err = pairdal.DB.GetMulti(c, data.playerEntityHolders); err != nil {
			return
		}
	}

	data.player.ID = data.board.ID + ":" + userID

	// =[ Start of transaction ]=
	txOptions := db.CrossGroupTransaction
	if data.playersCount > 1 {
		txOptions = db.SingleGroupTransaction
	} else {
		txOptions = db.CrossGroupTransaction
	}

	if err = pairdal.DB.RunInTransaction(c, func(c context.Context) error {
		isNewUser, err = openCellUpdateDB(&data)
		return err
	}, txOptions); err != nil {
		return
	}
	// =[ End of transaction ]=

	// if player.OpenX == x && player.OpenY == y {
	// 	m, err = renderPairsBoardMessage(whc, nil, board, []pairmodels.PairsPlayer{player})
	// 	return
	// }
	log.Debugf(c, "isAlreadyCompleted: %v, isAlreadyMatched: %v, isAlreadyOpen: %v", data.isAlreadyCompleted, data.isAlreadyMatched, data.isAlreadyOpen)
	switch true {
	case data.isAlreadyCompleted:
		// nothing special
	case data.isAlreadyMatched:
		m.BotMessage = telegram.CallbackAnswer(tgbotapi.AnswerCallbackQueryConfig{
			Text:      "This cell is already matched",
			CacheTime: 10,
		})
		if _, err = whc.Responder().SendMessage(c, m, bots.BotAPISendMessageOverHTTPS); err != nil {
			log.Errorf(c, "Failed to send already matched alert: %v", err)
			err = nil
		}
	}
	if data.isAlreadyOpen {
		m.BotMessage = telegram.CallbackAnswer(tgbotapi.AnswerCallbackQueryConfig{
			Text:      "This cell is already open by you",
			CacheTime: 10,
		})
		if _, err = whc.Responder().SendMessage(c, m, bots.BotAPISendMessageOverHTTPS); err != nil {
			log.Errorf(c, "Failed to send already matched alert: %v", err)
			err = nil
		}
	}
	tournament := pamodels.Tournament{StringID: db.NewStrID(data.board.TournamentID)}

	return renderPairsBoardMessage(c, whc, tournament, data.board, data.matchedTile, userID, data.players)
}

type updateDbContext struct {
	c                   context.Context // Non transactional context
	tc                  context.Context // Transactional context
	ca                  turnbased.CellAddress
	userID, userName    string
	board               pairmodels.PairsBoard
	addUserToBoard      func() (err error)
	player              pairmodels.PairsPlayer
	players             []pairmodels.PairsPlayer
	playersCount        int
	playerEntityHolders []db.EntityHolder
	matchedTile         string
	isAlreadyMatched    bool
	isAlreadyOpen       bool
	isAlreadyCompleted  bool
}

func openCellUpdateDB(data *updateDbContext) (isNewUser bool, err error) {
	newPlayerEntity := func() *pairmodels.PairsPlayerEntity {
		return &pairmodels.PairsPlayerEntity{
			PlayerCreated: time.Now(),
			UserName:      data.userName,
		}
	}

	var firstPlayer pairmodels.PairsPlayer

	if data.playersCount < 2 {
		if err = pairdal.DB.Get(data.c, &data.board); err != nil && !db.IsNotFound(err) {
			return
		}
		isNewUser = true
		for i, uID := range data.board.UserIDs {
			if uID == data.userID {
				isNewUser = false
				if data.userName == "" {
					data.userName = data.board.UserNames[i]
				}
				break
			}
		}
		log.Debugf(data.c, "isNewUser=%v, len(board.UserIDs)=%v", isNewUser, len(data.board.UserIDs))
		if isNewUser {
			if len(data.board.UserIDs) == 1 {
				firstPlayer.ID = pairmodels.NewPlayerID(data.board.ID, data.board.UserIDs[0])
				if err = pairdal.DB.Get(data.c, &firstPlayer); err == nil { // Let's lock the first player entity
					log.Warningf(data.c, "Unexpected but not critical: first user has PairsPlayer entity")
				} else if db.IsNotFound(err) { // This is the expected case
					err = nil
					entityCopy := data.board.PairsPlayerEntity
					entityCopy.UserName = data.board.UserNames[0]
					firstPlayer.PairsPlayerEntity = &entityCopy
				} else { // some real error returned
					return
				}
				data.board.PairsPlayerEntity = pairmodels.PairsPlayerEntity{} // Cleanup board from single player
				data.playerEntityHolders = append(data.playerEntityHolders, &firstPlayer)
			}
			data.addUserToBoard()
		}

		switch len(data.board.UserIDs) {
		case 0:
			data.player.PairsPlayerEntity = newPlayerEntity()
		case 1:
			if data.board.UserIDs[0] == data.userID {
				data.player.PairsPlayerEntity = &data.board.PairsPlayerEntity
				data.player.UserName = data.board.UserNames[0]
			}
		}
	}

	if data.player.PairsPlayerEntity == nil {
		if err = pairdal.DB.Get(data.tc, &data.player); err != nil && !db.IsNotFound(err) {
			return
		}
		if db.IsNotFound(err) {
			err = nil
			log.Debugf(data.c, "we got a new user for the board, so create player.PairsPlayerEntity")
			data.player.ID = pairmodels.NewPlayerID(data.board.ID, data.userID)
			data.player.PairsPlayerEntity = newPlayerEntity()
		}
	}

	// Populate players in same order as joined board to keep displaying in same order.
	{
		data.players = make([]pairmodels.PairsPlayer, len(data.board.UserIDs))
		var playersSetCount int
		for i, uID := range data.board.UserIDs {
			if uID == data.userID {
				data.players[i] = data.player
				log.Debugf(data.c, "uID == userID: %v", uID)
				playersSetCount++
			} else {
				for _, eh := range data.playerEntityHolders {
					if p := eh.(*pairmodels.PairsPlayer); p.UserID() == uID {
						log.Debugf(data.c, "p.UserID() == uID: %v", uID)
						data.players[i] = *p
						playersSetCount++
					}
				}
			}
		}
		if playersSetCount != len(data.players) {
			err = fmt.Errorf("playersSetCount != len(players): %v != %v, len(playerEntityHolders)=%v, player.ID=%v, board.UserIDs=%v", playersSetCount, len(data.players), len(data.playerEntityHolders), data.player.ID, data.board.UserIDs)
			return
		}
	}

	var changed bool
	// var changedPlayers []pairmodels.PairsPlayer
	log.Debugf(data.c, "len(players): %v; Board: %v; players[0].MatchedItems: %v", len(data.players), data.board.Cells, data.players[0].MatchedItems)
	// ===================================================================================================
	if changed, data.matchedTile, _, err = pairgame.OpenCell(data.board.PairsBoardEntity, data.ca, data.player, data.players); err != nil {
		// ================================================================================================
		switch err {
		case pairgame.ErrAlreadyOpen:
			data.isAlreadyOpen = true
			err = nil
		case pairgame.ErrAlreadyMatched:
			data.isAlreadyMatched = true
			err = nil
		case pairgame.ErrBoardIsCompleted:
			data.isAlreadyCompleted = true
			err = nil
		}
	}
	if data.userName != "" && data.player.UserName != data.userName {
		data.player.UserName = data.userName
		changed = true
	}
	if changed {
		if data.playersCount < 2 && len(data.players) == 1 {
			data.board.PairsPlayerEntity.UserName = ""
			if err = pairdal.DB.Update(data.c, &data.board); err != nil && !db.IsNotFound(err) {
				return
			}
			data.board.PairsPlayerEntity.UserName = data.board.UserNames[0]
		} else if data.playersCount < 2 && len(data.players) > 1 {
			var entitiesToUpdate []db.EntityHolder
			if firstPlayer.ID == "" {
				entitiesToUpdate = []db.EntityHolder{&data.board, &data.player}
			} else {
				entitiesToUpdate = []db.EntityHolder{&data.board, &data.player, &firstPlayer}
			}
			if err = pairdal.DB.UpdateMulti(data.c, entitiesToUpdate); err != nil && !db.IsNotFound(err) {
				return
			}
		} else if data.playersCount > 1 && len(data.players) > 1 {
			if err = pairdal.DB.Update(data.tc, &data.player); err != nil && !db.IsNotFound(err) {
				return
			}
		} else {
			err = fmt.Errorf("reached unexpected branch: playersCount=%v && len(players)=%v", data.playersCount, len(data.players))
			return
		}
	}
	return
}

func getSize(v url.Values, p string) (size turnbased.Size, err error) {
	var ca turnbased.CellAddress
	if ca, err = getCellAddress(v, p); err != nil {
		return
	}
	return turnbased.Size(ca), nil
}
func getCellAddress(v url.Values, p string) (ca turnbased.CellAddress, err error) {
	s := v.Get(p)
	if len(s) != 2 {
		err = fmt.Errorf("unexpected length of '%v' parameter: %v", p, len(s))
		return
	}
	if s[0] < 'A' || s[0] > 'Z' {
		err = fmt.Errorf("unexpected X value of '%v' parameter: %v", p, s)
		return
	}
	if s[1] < '0' || s[1] > '9' {
		err = fmt.Errorf("unexpected Y value of '%v' parameter: %v", p, s)
		return
	}
	ca = turnbased.CellAddress(s)
	return
}
