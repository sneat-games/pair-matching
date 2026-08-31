package pairapp

import (
	"github.com/julienschmidt/httprouter"
	"github.com/sneat-games/pair-matching/server-go/pairbot"
	"github.com/sneat-games/pair-matching/server-go/pairdal/pairgaedal"
	"github.com/strongo/bots-framework/core"
	"net/http"
)

func InitApp(botHost bots.BotHost) {
	pairgaedal.RegisterDal()

	httpRouter := httprouter.New()
	http.Handle("/", httpRouter)

	pairbot.InitBot(httpRouter, botHost, pairAppContext{})
}
