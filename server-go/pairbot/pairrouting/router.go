package pairrouting

import (
	"github.com/sneat-games/pair-matching/server-go/pairbot/paircommands"
	"github.com/strongo/bots-framework/core"
)

var WebhooksRouter = bots.NewWebhookRouter(
	map[bots.WebhookInputType][]bots.Command{},
	func() string { return "Oops..." },
)

func init() {
	paircommands.RegisterPairCommands(WebhooksRouter)
}
