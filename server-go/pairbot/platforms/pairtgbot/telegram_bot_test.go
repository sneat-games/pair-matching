package pairtgbot

import (
	"github.com/strongo/app"
	"github.com/strongo/bots-framework/core"
	"testing"
)

func TestBots(t *testing.T) {
	Bots(nil, strongo.EnvProduction, bots.WebhooksRouter{})
}
