package pairapp

import (
	"context"
	"github.com/pkg/errors"
	"github.com/sneat-games/pair-matching/server-go/pairmodels"
	"github.com/sneat-games/pair-matching/server-go/pairtrans"
	"github.com/strongo/app"
	"github.com/strongo/bots-framework/core"
	"github.com/strongo/bots-framework/platforms/telegram"
	"reflect"
	"time"
)

type pairAppContext struct {
}

func (appCtx pairAppContext) AppUserEntityKind() string {
	return pairmodels.UserKind
}

func (appCtx pairAppContext) AppUserEntityType() reflect.Type {
	return reflect.TypeOf(&pairmodels.UserEntity{})
}

func (appCtx pairAppContext) NewBotAppUserEntity() bots.BotAppUser {
	return &pairmodels.UserEntity{
		DtCreated: time.Now(),
	}
}

func (appCtx pairAppContext) NewAppUserEntity() strongo.AppUser {
	return appCtx.NewBotAppUserEntity()
}

func (appCtx pairAppContext) GetTranslator(c context.Context) strongo.Translator {
	return strongo.NewMapTranslator(c, pairtrans.TRANS)
}

type LocalesProvider struct {
}

func (LocalesProvider) GetLocaleByCode5(code5 string) (strongo.Locale, error) {
	return strongo.LocaleEnUS, nil
}

func (appCtx pairAppContext) SupportedLocales() strongo.LocalesProvider {
	return PairLocalesProvider{}
}

type PairLocalesProvider struct {
}

func (PairLocalesProvider) GetLocaleByCode5(code5 string) (locale strongo.Locale, err error) {
	switch code5 {
	case strongo.LocaleCodeEnUS:
		return strongo.LocaleEnUS, nil
	case strongo.LocalCodeRuRu:
		return strongo.LocaleRuRu, nil
	default:
		return locale, errors.New("Unsupported locale: " + code5)
	}
}

var _ strongo.LocalesProvider = (*PairLocalesProvider)(nil)

func (appCtx pairAppContext) GetBotChatEntityFactory(platform string) func() bots.BotChat {
	switch platform {
	case telegram.PlatformID:
		return func() bots.BotChat {
			return &telegram.ChatEntity{
				TgChatEntityBase: *telegram.NewTelegramChatEntity(),
			}
		}
	default:
		panic("Unknown platform: " + platform)
	}
}

var _ bots.BotAppContext = (*pairAppContext)(nil)
