package pairgaedal

import (
	"github.com/sneat-games/pair-matching/server-go/pairdal"
	"testing"
)

func TestRegisterDal(t *testing.T) {
	RegisterDal()
	if pairdal.DB == nil {
		t.Fatal("pairdal.DB == nil")
	}
}
