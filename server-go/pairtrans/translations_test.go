package pairtrans

import "testing"

func TestTRANS(t *testing.T) {
	if len(TRANS) == 0 {
		t.Fatal("TRANS is empty")
	}
}
