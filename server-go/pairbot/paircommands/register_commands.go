package paircommands

import (
	"github.com/prizarena/prizarena-public/pabot"
	"github.com/sneat-games/pair-matching/server-go/pairsecrets"
	"github.com/strongo/bots-framework/core"
)

func RegisterPairCommands(router bots.WebhooksRouter) {
	router.RegisterCommands([]bots.Command{
		startCommand,
		inlineQueryCommand,
		openCellCommand,
		newBoardCommand,
		newPlayCommand,
	})

	pabot.InitPrizarenaInGameBot(pairsecrets.PrizarenaGameID, pairsecrets.PrizarenaToken, router)
}
