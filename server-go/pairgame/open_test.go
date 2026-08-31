package pairgame

import (
	"github.com/prizarena/turnbased"
	"github.com/sneat-games/pair-matching/server-go/pairmodels"
	"github.com/strongo/db"
	"strconv"
	"testing"
	"time"
)

func TestOpenCell(t *testing.T) {
	var (
		players     []pairmodels.PairsPlayer
		playersByID map[string]pairmodels.PairsPlayer
	)

	newPlayers := func() {
		newPlayer := func(id string) pairmodels.PairsPlayer {
			p := pairmodels.PairsPlayer{
				StringID: db.NewStrID(id),
				PairsPlayerEntity: &pairmodels.PairsPlayerEntity{
					PlayerCreated: time.Now(),
				},
			}
			return p
		}
		p1 := newPlayer("p1")
		p2 := newPlayer("p2")
		players = []pairmodels.PairsPlayer{p1, p2}
		playersByID = map[string]pairmodels.PairsPlayer{p1.ID: p1, p2.ID: p2}
	}

	type expectedPlayer struct {
		Open1, Open2 turnbased.CellAddress
		MatchedCount int
		MatchedItems string
	}

	type expected struct {
		changed bool
		players map[string]expectedPlayer
	}

	type Step struct {
		player string
		ca     turnbased.CellAddress
		expected
	}

	type testCase struct {
		board pairmodels.PairsBoardEntity
		steps []Step
	}

	testCases := []testCase{
		{
			board: pairmodels.PairsBoardEntity{
				Size:  "D3",
				Cells: "1,2,3,4,5,6,6,5,4,3,2,1",
			},
			steps: []Step{
				{
					player: "p1", ca: "A2",
					expected: expected{
						changed: true,
						players: map[string]expectedPlayer{
							"p1": {Open1: "A2"},
							"p2": {},
						},
					},
				},
				{ // Same player, different cell
					player: "p1", ca: "A3",
					expected: expected{
						changed: true,
						players: map[string]expectedPlayer{
							"p1": {Open1: "A2", Open2: "A3"},
							"p2": {},
						},
					},
				},
				{ // Same player, next 1st cell
					player: "p1", ca: "B2",
					expected: expected{
						changed: true,
						players: map[string]expectedPlayer{
							"p1": {Open1: "B2", Open2: ""},
							"p2": {},
						},
					},
				},
				{ // Same player, 2nd and matching cell
					player: "p1", ca: "C2",
					expected: expected{
						changed: true,
						players: map[string]expectedPlayer{
							"p1": {Open1: "B2", Open2: "C2", MatchedItems: "6", MatchedCount: 1},
							"p2": {},
						},
					},
				},
				{ // Same player, another 1st cell
					player: "p1", ca: "A1",
					expected: expected{
						changed: true,
						players: map[string]expectedPlayer{
							"p1": {Open1: "A1", Open2: "", MatchedItems: "6", MatchedCount: 1},
							"p2": {},
						},
					},
				},
				{ // Another player, cell is matching to previously opened by 1st player
					player: "p2", ca: "D3",
					expected: expected{
						changed: true,
						players: map[string]expectedPlayer{
							"p1": {Open1: "A1", MatchedItems: "6", MatchedCount: 1},
							"p2": {Open1: "D3", MatchedItems: "1", MatchedCount: 1},
						},
					},
				},
			},
		},
		{
			board: pairmodels.PairsBoardEntity{
				Size:  "D3",
				Cells: "🚂,🚷,🚂,🚍,🚺,🚄,🚒,🚷,🚍,🚄,🚒,🚺",
			},
			steps: []Step{
				{
					player: "p1", ca: "A1",
					expected: expected{
						changed: true,
						players: map[string]expectedPlayer{
							"p1": {Open1: "A1", MatchedItems: "", MatchedCount: 0},
							"p2": {},
						},
					},
				},
			},
		},
	}

	var i int
	var step Step
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Unexpected panic at step %v: %v", i+1, r)
		}
	}()

	for i, testCase := range testCases {
		// if i == 0 {
		// 	continue
		// }
		theTestCase := testCase
		t.Run("case_"+strconv.Itoa(i+1), func(t *testing.T) {
			newPlayers()
			for i, step = range theTestCase.steps {

				stepPlayer := playersByID[step.player]

				if changed, _, _, err := OpenCell(&theTestCase.board, step.ca, stepPlayer, players); err != nil {
					t.Fatalf("Error at step #%v: %v", i, err)
				} else {
					if changed != step.expected.changed {
						if changed {
							t.Fatalf("Expected to be NOT changed at step #%v", i+1)
						} else {
							t.Fatalf("Expected to BE changed at step #%v", i+1)
						}
					}
					for pID, expectedP := range step.expected.players {
						actualP := playersByID[pID]
						var errorsCount int
						if actualP.Open1 != expectedP.Open1 {
							errorsCount++
							t.Errorf("Step #%v: actualP.Open1:%v != expectedP.Open1:%v", i+1, actualP.Open1, expectedP.Open1)
						}
						if actualP.Open2 != expectedP.Open2 {
							errorsCount++
							t.Errorf("Step #%v: actualP.Open2:%v != expectedP.Open2:%v", i+1, actualP.Open2, expectedP.Open2)
						}
						if actualP.MatchedItems != expectedP.MatchedItems {
							errorsCount++
							t.Errorf("Step #%v: actualP.MatchedItems:%v != expectedP.MatchedItems:%v", i+1, actualP.MatchedItems, expectedP.MatchedItems)
						}
						if actualP.MatchedCount != expectedP.MatchedCount {
							errorsCount++
							t.Errorf("Step #%v: actualP.MatchedCount:%v != expectedP.MatchedCount:%v", i+1, actualP.MatchedCount, expectedP.MatchedCount)
						}
						if errorsCount > 0 {
							t.Fatalf("Step #%v: errorsCount: %v", i+1, errorsCount)
						}
					}
				}
			}
		})
	}
}
