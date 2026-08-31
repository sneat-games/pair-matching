package pairtgbot

import (
	"context"
	"github.com/sneat-games/pair-matching/server-go/pairsecrets"
	"github.com/strongo/app"
	"github.com/strongo/bots-framework/core"
	"github.com/strongo/bots-framework/platforms/telegram"
	"github.com/strongo/log"
)

var botsBy bots.SettingsBy

func Bots(c context.Context, env strongo.Environment, router bots.WebhooksRouter) bots.SettingsBy {
	if len(botsBy.ByCode) == 0 {
		routerByProfile := func(profile string) bots.WebhooksRouter {
			return router // We have single profile for now
		}

		switch env {
		case strongo.EnvProduction:
			botsBy = bots.NewBotSettingsBy(routerByProfile,
				telegram.NewTelegramBot(strongo.EnvProduction, "PairMatching",
					pairsecrets.TelegramProdBot, pairsecrets.TelegramProdToken,
					"", "", pairsecrets.GaTrackingID, strongo.LocaleEnUS),
			)
		default:
			log.Errorf(c, "Unknown environment: %v=%v", env, strongo.EnvironmentNames[env])
		}
	}
	return botsBy
}
