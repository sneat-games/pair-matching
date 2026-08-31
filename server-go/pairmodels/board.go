package pairmodels

import (
	"bytes"
	"fmt"
	"github.com/prizarena/turnbased"
	"github.com/strongo/db"
	"github.com/strongo/db/gaedb"
	"github.com/strongo/emoji/go"
	"github.com/strongo/slices"
	"google.golang.org/appengine/datastore"
	"math/rand"
	"strings"
	"time"
)

type PairsBoardEntity struct {
	turnbased.BoardEntityBase
	PairsPlayerEntity
	Cells      string         `datastore:",noindex,omitempty"`
	Size       turnbased.Size `datastore:",noindex"`
	MaxPlayers int            `datastore:",noindex,omitempty"` // E.g. 1 - single player, 0 - no limits
}

const PairBoardKind = "B"

type PairsBoard struct {
	db.StringID
	*PairsBoardEntity
}

var _ db.EntityHolder = (*PairsPlayer)(nil)

func (PairsBoard) Kind() string {
	return PairBoardKind
}

func (eh PairsBoard) Entity() interface{} {
	return eh.PairsBoardEntity
}

func (PairsBoard) NewEntity() interface{} {
	return new(PairsBoardEntity)
}

func (eh *PairsBoard) SetEntity(entity interface{}) {
	eh.PairsBoardEntity = entity.(*PairsBoardEntity)
}

func (board PairsBoardEntity) Rows() (rows [][]string) {
	var x, y = 0, 0
	width, height := board.Size.WidthHeight()
	if width <= 0 || height <= 0 {
		panic(fmt.Sprintf("Both width & height should be > 0, got: width=%v, height=%v", width, height))
	}
	if width == 0 || height == 0 {
		return
	}
	rows = make([][]string, height)
	rows[0] = make([]string, width)
	for _, r := range strings.Split(board.Cells, ",") {
		rows[y][x] = r
		if x++; x == width {
			x = 0
			if y++; y < height {
				rows[y] = make([]string, width)
			}
		}
	}
	return
}

func (board PairsBoardEntity) GetCell(ca turnbased.CellAddress) string {
	if ca == "" {
		panic("Cell address is required to get cell value")
	}
	x, y := ca.XY()
	k := y*board.Size.Width() + x
	var runeIndex int
	for _, r := range strings.Split(board.Cells, ",") {
		if runeIndex == k {
			return r
		}
		runeIndex++
	}
	return ""
}

func (board PairsBoardEntity) DrawBoard(colSeparator, rowSeparator string) string {
	s := new(bytes.Buffer)

	s.WriteRune('\n')
	rows := board.Rows()
	for _, row := range rows {
		s.WriteString(strings.Join(row, colSeparator))
		s.WriteString(rowSeparator)
	}
	return s.String()
}

func (board PairsBoardEntity) IsCompleted(players []PairsPlayer) (isCompleted bool) {
	if len(players) == 0 {
		return false
	}
	cells := "," + board.Cells + ","
	for _, p := range players {
		matchedTiles := strings.Split(p.MatchedItems, ",")
		for _, matchedCell := range matchedTiles {
			for i := 0; i < 2; i++ {
				cells = strings.Replace(cells, ","+matchedCell+",", ",", 1)
			}
			if cells == ",," {
				return true
			}
		}
	}

	return cells == ","
}

func (board *PairsBoardEntity) Load(ps []datastore.Property) (err error) {
	return datastore.LoadStruct(board, ps)
}

func (board *PairsBoardEntity) Save() (properties []datastore.Property, err error) {
	if properties, err = datastore.SaveStruct(board); err != nil {
		return properties, err
	}
	if properties, err = gaedb.CleanProperties(properties, map[string]gaedb.IsOkToRemove{
		"pdc": gaedb.IsZeroTime,
		"pdl": gaedb.IsZeroTime,
	}); err != nil {
		return
	}
	return
}

var goodEmojies []string

func emojiSet() []string {
	if len(goodEmojies) == 0 {
		categories := [][]string{
			emojis.CategoryActivity,
			emojis.CategoryFlags,
			emojis.CategoryFoods,
			emojis.CategoryNature,
			emojis.CategoryObjects,
			emojis.CategoryPeople,
			emojis.CategoryPlaces,
		}
		var length int
		for _, category := range categories {
			length += len(category)
		}
		goodEmojies = make([]string, length)
		var i int
		for _, category := range categories {
			for _, emoji := range category {
				goodEmojies[i] = emoji
				i++
			}
		}
	}
	return goodEmojies[:]
}

var rnd = rand.New(rand.NewSource(time.Now().UnixNano()))

func NewCells(width, height int) string {
	available := emojiSet()
	pairsCount := width * height / 2

	items := make([]string, pairsCount*2)
	for i := 0; i < pairsCount; i++ {
	random:
		rndCount := 0
		randIndex := rnd.Intn(len(available))
		for k := 0; k < i; k++ {
			if items[k] == available[randIndex] {
				rndCount++
				if rndCount > 100 {
					panic("rndCount > 100")
				}
				goto random
			}
		}
		items[i] = available[randIndex]
		items[i+pairsCount] = available[randIndex]
	}
	slices.Shuffle(rnd, len(items), func(i, j int) {
		items[i], items[j] = items[j], items[i]
	})

	return strings.Join(items, ",")
}
