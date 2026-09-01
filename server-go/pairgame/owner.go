package pairgame

// Owner identifies which side either owns a matched pair or is on turn.
// It fits in 2 bits on the wire (value 3 is reserved/unused), which doubles
// as the "is this pair matched" flag: Unmatched means exactly that.
type Owner uint8

const (
	Unmatched Owner = 0 // pair not yet matched by anyone
	Human     Owner = 1
	Robot     Owner = 2
)

func (o Owner) String() string {
	switch o {
	case Human:
		return "human"
	case Robot:
		return "robot"
	default:
		return "unmatched"
	}
}

// Other returns the opposing player for a Human/Robot turn owner. It panics
// if called on Unmatched, which is never a valid turn value.
func (o Owner) Other() Owner {
	switch o {
	case Human:
		return Robot
	case Robot:
		return Human
	default:
		panic("pairgame: Other() called on a non-player Owner")
	}
}
