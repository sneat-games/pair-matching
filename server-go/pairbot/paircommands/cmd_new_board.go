package paircommands

import (
	"bytes"
	"context"
	"fmt"
	"github.com/prizarena/prizarena-public/pamodels"
	"github.com/prizarena/turnbased"
	"github.com/sneat-games/pair-matching/server-go/pairdal"
	"github.com/sneat-games/pair-matching/server-go/pairmodels"
	"github.com/strongo/bots-framework/core"
	"github.com/strongo/bots-framework/platforms/telegram"
	"github.com/strongo/db"
	"github.com/strongo/log"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const newBoardCommandCode = "new"

func getNewBoardCallbackData(width, height, maxUsersLimit int, tournamentID, lang string) string {
	s := new(bytes.Buffer)
	fmt.Fprintf(s, "new?s=%v&l=%v", turnbased.NewSize(width, height), lang)
	if tournamentID != "" {
		if i := strings.Index(tournamentID, pamodels.TournamentIDSeparator); i >= 0 {
			tournamentID = tournamentID[i+1:]
		}
		fmt.Fprint(s, "&t="+tournamentID)
	}
	if maxUsersLimit > 0 {
		fmt.Fprint(s, "&max="+strconv.Itoa(maxUsersLimit))
	} else if maxUsersLimit < 0 {
		panic(fmt.Sprintf("maxUsersLimit < 0: %v", maxUsersLimit))
	}
	return s.String()
}

var newBoardCommand = bots.NewCallbackCommand(
	newBoardCommandCode,
	func(whc bots.WebhookContext, callbackUrl *url.URL) (m bots.MessageFromBot, err error) {
		c := whc.Context()
		q := callbackUrl.Query()
		var size turnbased.Size
		if size, err = getSize(q, "s"); err != nil {
			return
		}

		var maxUsersLimit int
		if s := q.Get("max"); s != "" {
			if maxUsersLimit, err = strconv.Atoi(s); err != nil {
				return
			}
			log.Debugf(c, "maxUsersLimit: %v", maxUsersLimit)
		} else {
			log.Debugf(c, "No maxUsersLimit")
		}

		if err = whc.SetLocale(q.Get("l")); err != nil {
			return
		}

		var board pairmodels.PairsBoard
		tgCallbackQuery := whc.Input().(telegram.TgWebhookCallbackQuery)
		board.ID = tgCallbackQuery.GetInlineMessageID()
		if board.ID == "" { // Inside bot single-player mode
			board.ID = tgCallbackQuery.GetID()
		}

		tournamentID := q.Get("t")
		if i := strings.Index(tournamentID, pamodels.TournamentIDSeparator); i >= 0 { // Just in case
			tournamentID = tournamentID[i+1:]
		}

		userID := whc.AppUserStrID()
		// var botAppUser bots.BotAppUser
		// if botAppUser, err = whc.GetAppUser(); err != nil {
		// 	return
		// }
		err = pairdal.DB.RunInTransaction(c, func(tc context.Context) (err error) {
			if err = pairdal.DB.Get(tc, &board); err != nil && !db.IsNotFound(err) {
				return
			}
			var changed bool
			if err == nil { // Existing entity
				log.Debugf(c, "Existing board entity")
				if boardUsersCount := len(board.UserIDs); boardUsersCount > 1 {
					log.Debugf(c, "Will delete %v player entities", boardUsersCount)
					players := make([]db.EntityHolder, boardUsersCount)
					for i, userID := range board.UserIDs {
						players[i] = &pairmodels.PairsPlayer{StringID: db.NewStrID(pairmodels.NewPlayerID(board.ID, userID))}
					}
					if err = pairdal.DB.DeleteMulti(tc, players); err != nil {
						return
					}
				}
				now := time.Now()
				if board.Created.Before(now.Add(-time.Second * 2)) {
					board.Created = now
					board.Size = size
					board.Cells = pairmodels.NewCells(size.Width(), size.Height())
					board.PairsPlayerEntity = pairmodels.PairsPlayerEntity{}
					board.UserIDs = []string{}
					board.UserNames = []string{}
					board.UserWins = []int{}
					board.UsersMax = maxUsersLimit
					changed = true
				}
			} else if db.IsNotFound(err) {
				log.Debugf(c, "New board entity")
				changed = true
				board.PairsBoardEntity = &pairmodels.PairsBoardEntity{
					BoardEntityBase: turnbased.BoardEntityBase{
						Created:       time.Now(),
						CreatorUserID: userID,
						UsersMax:      maxUsersLimit,
						TournamentID:  tournamentID,
					},
					Size:  size,
					Cells: pairmodels.NewCells(size.Width(), size.Height()),
				}
			}
			// if !slices.IsInStringSlice(userID, board.UserIDs) {
			// 	changed = true
			// 	board.AddUser(userID, botAppUser.(*pairmodels.UserEntity).GetFullName())
			// }
			if changed {
				if err = pairdal.DB.Update(tc, &board); err != nil {
					return
				}
			}
			return
		}, db.CrossGroupTransaction)
		if err != nil {
			return
		}
		// TODO: check and notify if another user already selected different board size.
		tournament := pamodels.Tournament{StringID: db.NewStrID(board.TournamentID)}
		m, err = renderPairsBoardMessage(c, whc, tournament, board, "", userID, nil)
		return
	},
)
