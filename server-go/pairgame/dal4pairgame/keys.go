package dal4pairgame

import "github.com/dal-go/record"

// newGameKey builds the ext/pairmatching/games/{gameID} key, namespacing
// this extension's records below a single "ext/{ExtensionID}" parent key.
func newGameKey(gameID string) *record.Key {
	extKey := record.NewKeyWithID("ext", ExtensionID)
	return record.NewKeyWithParentAndID(extKey, GamesCollection, gameID)
}
