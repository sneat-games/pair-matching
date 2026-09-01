package pairgame

import "testing"

func TestBitWriterReaderRoundTrip(t *testing.T) {
	fields := []struct {
		v uint64
		n int
	}{
		{0, 1}, {1, 1}, {2, 2}, {5, 3}, {127, 7}, {0, 7}, {1<<31 - 1, 31}, {4294967295, 32},
	}

	w := &bitWriter{}
	for _, f := range fields {
		w.writeBits(f.v, f.n)
	}

	wantBits := 0
	for _, f := range fields {
		wantBits += f.n
	}
	if got := w.bitLen(); got != wantBits {
		t.Fatalf("bitLen() = %d, want %d", got, wantBits)
	}

	r := &bitReader{buf: w.bytes()}
	for i, f := range fields {
		got, err := r.readBits(f.n)
		if err != nil {
			t.Fatalf("field %d: readBits: %v", i, err)
		}
		if got != f.v {
			t.Errorf("field %d: readBits(%d) = %d, want %d", i, f.n, got, f.v)
		}
	}
}

func TestBitReaderShortBuffer(t *testing.T) {
	r := &bitReader{buf: []byte{0xff}}
	if _, err := r.readBits(8); err != nil {
		t.Fatalf("readBits(8) on 1 byte: unexpected error %v", err)
	}
	if _, err := r.readBits(1); err == nil {
		t.Fatal("readBits(1) past end of buffer: expected an error, got nil")
	}
}

func TestBitWriterPacksMSBFirst(t *testing.T) {
	w := &bitWriter{}
	w.writeBits(0b101, 3)
	w.writeBits(0b1, 1)
	w.writeBits(0b0000, 4)
	got := w.bytes()
	want := byte(0b10110000)
	if len(got) != 1 || got[0] != want {
		t.Fatalf("bytes() = %08b, want %08b", got, []byte{want})
	}
}
