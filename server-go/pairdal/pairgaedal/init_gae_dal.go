package pairgaedal

import (
	"github.com/sneat-games/pair-matching/server-go/pairdal"
	"github.com/strongo/db/gaedb"
)

func RegisterDal() {
	pairdal.DB = gaedb.NewDatabase()
}
