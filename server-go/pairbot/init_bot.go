package pairbot

import (
	"context"
	"github.com/julienschmidt/httprouter"
	"github.com/sneat-games/pair-matching/server-go/pairbot/pairrouting"
	"github.com/sneat-games/pair-matching/server-go/pairbot/platforms/pairtgbot"
	"github.com/sneat-games/pair-matching/server-go/pairsecrets"
	"github.com/strongo/app"
	"github.com/strongo/bots-framework/core"
	"github.com/strongo/bots-framework/platforms/telegram"
)

func InitBot(httpRouter *httprouter.Router, botHost bots.BotHost, appContext bots.BotAppContext) error {
	gaSettings := bots.AnalyticsSettings{
		GaTrackingID: pairsecrets.GaTrackingID,
		// Enabled: func(r *http.Request) bool {
		// 	return false
		// },
	}

	driver := bots.NewBotDriver(gaSettings, appContext, botHost,
		"Please report any issues to @trakhimenok",
	)
	// routing.WebhooksRouter

	if driver == nil {
		panic("driver == nil")
	}

	newTranslator := func(c context.Context) strongo.Translator {
		return strongo.NewMapTranslator(c, nil)
	}

	driver.RegisterWebhookHandlers(httpRouter, "/bot",
		telegram.NewTelegramWebhookHandler(
			func(c context.Context) bots.SettingsBy {
				return pairtgbot.Bots(c, strongo.EnvProduction, pairrouting.WebhooksRouter) // gaestandard.GetEnvironment(c)
			}, // Maps of bots by code, language, token, etc...
			newTranslator, // Creates newTranslator that gets a context.Context (for logging purpose)
		),
	)

	return nil
}
