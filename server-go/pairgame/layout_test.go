package pairgame

import "testing"

func assertValidLayout(t *testing.T, faces []uint8, pairs int) {
	t.Helper()
	counts := make(map[uint8]int, pairs)
	for _, f := range faces {
		if int(f) >= pairs {
			t.Fatalf("face id %d out of range for %d pairs", f, pairs)
		}
		counts[f]++
	}
	if len(counts) != pairs {
		t.Fatalf("got %d distinct face ids, want %d", len(counts), pairs)
	}
	for f, c := range counts {
		if c != 2 {
			t.Errorf("face %d appears %d times, want exactly 2", f, c)
		}
	}
}

func TestShuffleFacesIsAValidPairing(t *testing.T) {
	for _, size := range Sizes {
		faces := ShuffleFaces(12345, size.Pairs())
		if len(faces) != size.Cells() {
			t.Fatalf("%+v: ShuffleFaces returned %d faces, want %d", size, len(faces), size.Cells())
		}
		assertValidLayout(t, faces, size.Pairs())
	}
}

func TestShuffleFacesIsDeterministic(t *testing.T) {
	a := ShuffleFaces(42, 8)
	b := ShuffleFaces(42, 8)
	if string(a) != string(b) {
		t.Fatalf("ShuffleFaces(42, 8) not deterministic: %v vs %v", a, b)
	}
}

func TestShuffleFacesDiffersByOtherSeed(t *testing.T) {
	same := 0
	const trials = 20
	for seed := uint32(0); seed < trials; seed++ {
		a := ShuffleFaces(seed, 8)
		b := ShuffleFaces(seed+1000, 8)
		if string(a) == string(b) {
			same++
		}
	}
	if same == trials {
		t.Fatal("ShuffleFaces produced the same layout for every seed pair tried — seed is not affecting the shuffle")
	}
}

func TestDeriveFacesIsAValidPairing(t *testing.T) {
	secret := []byte("test-bot-secret")
	for i, size := range Sizes {
		faces := DeriveFaces(secret, snapshotVersion, uint8(i), 777)
		if len(faces) != size.Cells() {
			t.Fatalf("%+v: DeriveFaces returned %d faces, want %d", size, len(faces), size.Cells())
		}
		assertValidLayout(t, faces, size.Pairs())
	}
}

func TestDeriveFacesIsDeterministic(t *testing.T) {
	secret := []byte("s3cr3t")
	a := DeriveFaces(secret, snapshotVersion, 3, 99)
	b := DeriveFaces(secret, snapshotVersion, 3, 99)
	if string(a) != string(b) {
		t.Fatalf("DeriveFaces not deterministic for the same (secret, version, sizeIndex, seed)")
	}
}

// TestDeriveFacesRequiresTheSecret is the crux of REQ:state-in-callback-data's
// secrecy property for a memory game: two different secrets, given the exact
// same public seed, must (with overwhelming probability) produce different
// layouts — otherwise the "seed" would be exactly as public/spoilable as
// LayoutInline's explicit array, just smaller.
func TestDeriveFacesRequiresTheSecret(t *testing.T) {
	faceA := DeriveFaces([]byte("secret-a"), snapshotVersion, 3, 4242)
	faceB := DeriveFaces([]byte("secret-b"), snapshotVersion, 3, 4242)
	if string(faceA) == string(faceB) {
		t.Fatal("DeriveFaces produced the same layout for the same seed under two different secrets")
	}
}

func TestDeriveFacesVariesBySizeIndex(t *testing.T) {
	secret := []byte("secret")
	a := DeriveFaces(secret, snapshotVersion, 2, 555) // 3x4, 6 pairs
	b := DeriveFaces(secret, snapshotVersion, 4, 555) // 4x5, 10 pairs — different length already
	if len(a) == len(b) {
		t.Fatalf("expected different-length layouts for different size indexes, got equal length %d", len(a))
	}
}

func TestBitsForCount(t *testing.T) {
	cases := map[int]int{0: 0, 1: 0, 2: 1, 3: 2, 4: 2, 5: 3, 8: 3, 9: 4, 16: 4, 18: 5, 32: 5}
	for n, want := range cases {
		if got := bitsForCount(n); got != want {
			t.Errorf("bitsForCount(%d) = %d, want %d", n, got, want)
		}
	}
}
