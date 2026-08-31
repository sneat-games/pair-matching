package paircommands

import (
	"github.com/prizarena/prizarena-public/pabot"
	"github.com/prizarena/prizarena-public/pamodels"
	"github.com/sneat-games/pair-matching/server-go/pairsecrets"
	"github.com/sneat-games/pair-matching/server-go/pairtrans"
	"github.com/strongo/bots-api-telegram"
	"github.com/strongo/bots-framework/core"
	"github.com/strongo/bots-framework/platforms/telegram"
	"strings"
)

var inlineQueryCommand = bots.NewInlineQueryCommand(
	"inline-query",
	func(whc bots.WebhookContext) (m bots.MessageFromBot, err error) {
		tgInlineQuery := whc.Input().(telegram.TgWebhookInlineQuery)
		inlineQuery := pabot.InlineQueryContext{
			ID:   tgInlineQuery.GetInlineQueryID(),
			Text: strings.TrimSpace(tgInlineQuery.TgUpdate().InlineQuery.Query),
		}
		words := strings.Split(inlineQuery.Text, " ")

		removeLang := func() { // TODO: reuse? currently copy-pasted
			if len(words) == 1 {
				words = []string{}
			} else {
				words = words[1:]
			}
		}
		switch words[0] {
		case "ru":
			whc.SetLocale("ru-RU")
			removeLang()
		case "en":
			removeLang()
		}

		inlineQuery.Text = strings.Join(words, " ")

		switch {
		case strings.HasPrefix(inlineQuery.Text, "tournament?id="):
			// return inlineQueryTournament(whc, inlineQuery)
		case inlineQuery.Text == "" || inlineQuery.Text == "play" || strings.HasPrefix(inlineQuery.Text, "play?tournament="):
			return inlineQueryPlay(whc, inlineQuery)
		}
		return
	},
)

// func inlineQueryDefault(whc bots.WebhookContext, inlineQuery pabot.InlineQueryContext) (m bots.MessageFromBot, err error) {
// 	return
// }

func inlineQueryPlay(whc bots.WebhookContext, inlineQuery pabot.InlineQueryContext) (m bots.MessageFromBot, err error) {
	return pabot.ProcessInlineQueryTournament(whc, inlineQuery, pairsecrets.PrizarenaGameID, pairsecrets.PrizarenaToken, "tournament",
		func(tournament pamodels.Tournament) (m bots.MessageFromBot, err error) {
			// c := whc.Context()

			// translator := whc.BotAppContext().GetTranslator(c)

			newGameOption := func() tgbotapi.InlineQueryResultArticle {
				// t := strongo.NewSingleMapTranslator(strongo.LocalesByCode5[lang], translator)

				lang := whc.Locale().Code5

				articleID := "new_game?l=" + lang
				if tournament.ID != "" {
					articleID += "&t=" + tournament.ShortTournamentID()
				}

				var keyboard *tgbotapi.InlineKeyboardMarkup
				if tournament.ID == "" {
					keyboard = newNonTournamentBoardSizesKeyboards[lang]
				} else {
					keyboard = getNewPlayTgInlineKbMarkup(lang, tournament.ID, 0)
				}
				return tgbotapi.InlineQueryResultArticle{
					ID:          articleID,
					Type:        "article",
					Title:       whc.Translate(pairtrans.NewGameInlineTitle),
					Description: whc.Translate(pairtrans.NewGameInlineDescription),
					InputMessageContent: tgbotapi.InputTextMessageContent{
						Text:                  getNewPlayText(whc, tournament),
						ParseMode:             "HTML",
						DisableWebPagePreview: m.DisableWebPagePreview,
					},
					ReplyMarkup: keyboard,
				}
			}

			m.BotMessage = telegram.InlineBotMessage(tgbotapi.InlineConfig{
				InlineQueryID: inlineQuery.ID,
				Results: []interface{}{
					newGameOption(),
					// newGameOption("ru-RU"),
				},
				CacheTime: 10,
			})
			return
		})
	return
}
