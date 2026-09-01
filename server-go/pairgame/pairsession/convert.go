package pairsession

import (
	"github.com/sneat-games/pair-matching/server-go/pairgame"
	"github.com/sneat-games/pair-matching/server-go/pairgame/dal4pairgame"
)

// toGameState projects a persisted GameDbo into the pure rules engine's
// GameState. It is a read-only snapshot: mutate the returned value freely
// (pairgame.Flip does exactly that), then write the parts that changed
// back onto the SAME *GameDbo via writeBackPlayers/writeBackBoard — those
// two together, not a full re-projection, because GameDbo.Players carries
// identity fields (UserID, Name, ChatID) that pairgame.Player has no
// business knowing about and that a naive round trip would silently drop.
func toGameState(d *dal4pairgame.GameDbo) pairgame.GameState {
	players := make([]pairgame.Player, len(d.Players))
	for i, p := range d.Players {
		players[i] = pairgame.Player{
			ID:      pairgame.PlayerID(p.ID),
			IsBot:   p.IsBot,
			Pending: p.Pending,
			Score:   p.Score,
			Memory:  p.Memory,
		}
	}
	pairOwner := make([]pairgame.PlayerID, len(d.PairOwner))
	for i, o := range d.PairOwner {
		pairOwner[i] = pairgame.PlayerID(o)
	}
	log := make([]pairgame.Reveal, len(d.Log))
	for i, e := range d.Log {
		log[i] = pairgame.Reveal{By: pairgame.PlayerID(e.By), Cell: e.Cell, PairID: e.PairID, Matched: e.Matched}
	}
	return pairgame.GameState{
		SizeIndex: d.SizeIndex,
		Mode:      pairgame.LayoutInline, // see the package doc: a stored game is always dealt inline
		Faces:     append([]uint8(nil), d.Faces...),
		PairOwner: pairOwner,
		Players:   players,
		Log:       log,
	}
}

// writeBackPlayers copies the per-seat fields Flip/RobotMove can change
// (Pending, Score) from g back onto d.Players, by index — g.Players and
// d.Players are always the same length and in the same seating order
// (players are only ever appended, never reordered or removed), so this
// never needs to match seats up by ID. IsBot and Memory are seat-setup
// constants Flip never changes, so they are not written back; the
// identity fields (UserID, Name, ChatID) that only d.Players carries are
// left untouched entirely.
func writeBackPlayers(g pairgame.GameState, d *dal4pairgame.GameDbo) {
	for i := range d.Players {
		d.Players[i].Pending = g.Players[i].Pending
		d.Players[i].Score = g.Players[i].Score
	}
}

// writeBackBoard copies PairOwner and Log — both fully derived, append-
// only, and owned entirely by the rules engine — from g back onto d.
func writeBackBoard(g pairgame.GameState, d *dal4pairgame.GameDbo) {
	d.PairOwner = make([]uint8, len(g.PairOwner))
	for i, o := range g.PairOwner {
		d.PairOwner[i] = uint8(o)
	}
	d.Log = make([]dal4pairgame.RevealDbo, len(g.Log))
	for i, e := range g.Log {
		d.Log[i] = dal4pairgame.RevealDbo{By: uint8(e.By), Cell: e.Cell, PairID: e.PairID, Matched: e.Matched}
	}
}
