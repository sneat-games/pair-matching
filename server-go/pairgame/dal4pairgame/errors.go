package dal4pairgame

import "fmt"

func errMissingField(name string) error {
	return fmt.Errorf("dal4pairgame: missing required field %q", name)
}

func errIndexedField(collection string, i int, field string) error {
	return fmt.Errorf("dal4pairgame: %s[%d].%s is required", collection, i, field)
}

func errDuplicatePlayer(userID string) error {
	return fmt.Errorf("dal4pairgame: duplicate player userID=%s", userID)
}

func errDuplicatePlayerID(id uint8) error {
	return fmt.Errorf("dal4pairgame: duplicate player id=%d", id)
}

func errInvalidStatus(status Status) error {
	return fmt.Errorf("dal4pairgame: invalid status %q", status)
}

func errInvalidMode(mode Mode) error {
	return fmt.Errorf("dal4pairgame: invalid mode %q", mode)
}

func errWrongBotCount(mode Mode, bots int) error {
	return fmt.Errorf("dal4pairgame: mode %q requires a specific bot count, got %d bot(s)", mode, bots)
}
