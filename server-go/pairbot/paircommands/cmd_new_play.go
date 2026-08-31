package paircommands

import (
	"bytes"
	"fmt"
	"github.com/prizarena/prizarena-public/pamodels"
	"github.com/sneat-games/pair-matching/server-go/pairtrans"
	"github.com/strongo/app"
	"github.com/strongo/bots-api-telegram"
	"github.com/strongo/bots-framework/core"
	"net/url"
	"strconv"
)

const newSinleplayerCommandCode = "singleplayer"

var newPlayCommand = bots.Command{
	Code:     newSinleplayerCommandCode,
	Commands: []string{"/singleplayer"},
	Action: func(whc bots.WebhookContext) (m bots.MessageFromBot, err error) {
		return newPlayAction(whc, "", 1)
	},
	CallbackAction: func(whc bots.WebhookContext, callbackUrl *url.URL) (m bots.MessageFromBot, err error) {
		tournamentID := callbackUrl.Query().Get("t")
		return newPlayAction(whc, tournamentID, 1)
	},
}

func newPlayAction(whc bots.WebhookContext, tournamentID string, maxUsersLimit int) (m bots.MessageFromBot, err error) {
	var tournament pamodels.Tournament
	m.Text = getNewPlayText(whc, tournament)
	m.Format = bots.MessageFormatHTML
	m.Keyboard = getNewPlayTgInlineKbMarkup(whc.Locale().Code5, tournamentID, maxUsersLimit)
	return
}

func getNewPlayTgInlineKbMarkup(lang, tournamentID string, maxUsersLimit int) *tgbotapi.InlineKeyboardMarkup {
	sizeButton := func(width, height int) tgbotapi.InlineKeyboardButton {
		return tgbotapi.InlineKeyboardButton{
			Text:         fmt.Sprintf(strconv.Itoa(width) + "x" + strconv.Itoa(height)),
			CallbackData: getNewBoardCallbackData(width, height, maxUsersLimit, tournamentID, lang),
		}
	}
	return tgbotapi.NewInlineKeyboardMarkup(
		[]tgbotapi.InlineKeyboardButton{
			sizeButton(4, 2),
			sizeButton(4, 3),
			sizeButton(4, 4),
			sizeButton(5, 4),
		},
		[]tgbotapi.InlineKeyboardButton{
			sizeButton(6, 4),
			sizeButton(6, 5),
			sizeButton(6, 6),
		},
		[]tgbotapi.InlineKeyboardButton{
			sizeButton(7, 6),
			sizeButton(8, 6),
			sizeButton(8, 7),
			sizeButton(8, 8),
		},
		[]tgbotapi.InlineKeyboardButton{
			sizeButton(8, 9),
			sizeButton(8, 10),
			sizeButton(8, 11),
			sizeButton(8, 12),
		},
	)
}

var newNonTournamentBoardSizesKeyboards = map[string]*tgbotapi.InlineKeyboardMarkup{
	"en-US": getNewPlayTgInlineKbMarkup("en-US", "", 0),
	"ru-RU": getNewPlayTgInlineKbMarkup("ru-RU", "", 0),
}

func getNewPlayText(t strongo.SingleLocaleTranslator, tournament pamodels.Tournament) string {
	text := new(bytes.Buffer)
	text.WriteString(t.Translate(pairtrans.NewGameText))
	return text.String()
}
