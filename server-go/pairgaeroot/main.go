package pairgaeroot

import (
	"github.com/sneat-games/pair-matching/server-go/pairapp"
	"github.com/strongo/bots-framework/hosts/appengine"
	"github.com/strongo/log"
)

func init() {
	log.AddLogger(gaehost.GaeLogger)
	pairapp.InitApp(gaehost.GaeBotHost{})
}
