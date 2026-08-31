package paircommands

import (
	"github.com/prizarena/prizarena-public/pabot"
	"github.com/sneat-games/pair-matching/server-go/pairsecrets"
	"github.com/sneat-games/pair-matching/server-go/pairtrans"
	"github.com/strongo/bots-api-telegram"
	"github.com/strongo/bots-framework/core"
	"github.com/strongo/bots-framework/platforms/telegram"
	"github.com/strongo/log"
	"net/url"
)

const startCommandCommandCode = "start"

var startCommand = bots.Command{
	Code:           startCommandCommandCode,
	Commands:       []string{"/start"},
	Action:         startAction,
	CallbackAction: startCallbackAction,
}

func startCallbackAction(whc bots.WebhookContext, callbackUrl *url.URL) (m bots.MessageFromBot, err error) {
	c := whc.Context()
	q := callbackUrl.Query()
	lang := q.Get("l")
	switch lang {
	case "ru":
		lang = "ru-RU"
	case "en":
		lang = "en-US"
	default:
		m.BotMessage = telegram.CallbackAnswer(tgbotapi.AnswerCallbackQueryConfig{
			Text: "Unknown language: " + lang,
		})
		log.Errorf(whc.Context(), "Unknown language: "+lang)
		return
	}
	if lang != "" {
		chatEntity := whc.ChatEntity() // We need it to be loaded before changing current locale
		currentLang := q.Get("cl")
		currentLocaleCode5 := whc.Locale().Code5
		log.Debugf(whc.Context(), "query: %v, lang: %v, currentLang: %v, currentLocaleCode5: %v", q, lang, currentLang, currentLocaleCode5)
		if lang != currentLocaleCode5 {
			if err = whc.SetLocale(lang); err != nil {
				log.Errorf(c, "Failed to set current locale to %v: %v", lang, err)
				err = nil
			} else {
				if currentLocaleCode5 = whc.Locale().Code5; currentLocaleCode5 != lang {
					log.Errorf(c, "Locale not set, expected %v, got: %v", lang, currentLocaleCode5)
				}
				chatEntity.SetPreferredLanguage(lang)
			}
		}
		if lang == currentLang {
			m.BotMessage = telegram.CallbackAnswer(tgbotapi.AnswerCallbackQueryConfig{
				Text: "It is already current language",
			})
			return
		}
	}
	m, err = startAction(whc)
	m.IsEdit = true
	return
}

func startAction(whc bots.WebhookContext) (m bots.MessageFromBot, err error) {
	if m, err = pabot.OnStartIfTournamentLink(whc, pairsecrets.PrizarenaGameID, pairsecrets.PrizarenaToken); m.Text != "" || err != nil {
		return
	}
	m.Text = whc.Translate(pairtrans.OnStartWelcome)
	m.Format = bots.MessageFormatHTML
	switchInlinePlay := whc.Locale().Code5[:2]
	m.Keyboard = tgbotapi.NewInlineKeyboardMarkup(
		[]tgbotapi.InlineKeyboardButton{
			{Text: "Русский", CallbackData: "start?l=ru&cl=" + whc.Locale().Code5},
			{Text: "English", CallbackData: "start?l=en&cl=" + whc.Locale().Code5},
		},
		[]tgbotapi.InlineKeyboardButton{
			{Text: whc.Translate(pairtrans.SinglePlayer), CallbackData: newSinleplayerCommandCode},
		},
		[]tgbotapi.InlineKeyboardButton{
			{Text: whc.Translate(pairtrans.MultiPlayer), SwitchInlineQuery: &switchInlinePlay},
		},
		[]tgbotapi.InlineKeyboardButton{
			{Text: whc.Translate(pairtrans.Tournaments), CallbackData: "tournaments"},
		},
	)
	return
}
