package paircommands

import (
	"github.com/strongo/bots-framework/core"
	"testing"
)

func TestRegisterPairCommands(t *testing.T) {
	router := bots.NewWebhookRouter(map[bots.WebhookInputType][]bots.Command{}, nil)
	RegisterPairCommands(router)
	if router.CommandsCount() == 0 {
		t.Fatal("router.CommandsCount() == 0")
	}
}
