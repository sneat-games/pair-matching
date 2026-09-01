package pairgame

import "errors"

// bitWriter packs unsigned integers into a byte slice MSB-first, bit by bit.
// Pair-Matching's snapshot has several sub-byte fields (a 1-bit turn flag, a
// 2-bit layout mode, 2-bit-per-pair ownership...), so — unlike Reversi's
// bitboards, which are already byte-aligned uint64s — it needs real bit
// packing rather than plain byte concatenation. The bit-at-a-time
// implementation is O(bits), which is fine: a full snapshot is at most a few
// hundred bits.
type bitWriter struct {
	buf []byte
	pos int // total bits written so far
}

// writeBits appends the low n bits of v, most-significant bit first.
func (w *bitWriter) writeBits(v uint64, n int) {
	for i := n - 1; i >= 0; i-- {
		bit := (v >> uint(i)) & 1
		byteIdx := w.pos / 8
		if byteIdx == len(w.buf) {
			w.buf = append(w.buf, 0)
		}
		if bit == 1 {
			bitIdx := uint(7 - w.pos%8)
			w.buf[byteIdx] |= 1 << bitIdx
		}
		w.pos++
	}
}

// bytes returns the packed buffer. The final byte is zero-padded on the low
// bits if the total bit count is not a multiple of 8.
func (w *bitWriter) bytes() []byte { return w.buf }

// bitLen returns the number of bits written so far (before byte padding).
func (w *bitWriter) bitLen() int { return w.pos }

// errShortSnapshot is returned when a snapshot's payload ends before a field
// that Decode expected to find has been fully read.
var errShortSnapshot = errors.New("pairgame: snapshot payload too short")

// bitReader is the inverse of bitWriter.
type bitReader struct {
	buf []byte
	pos int
}

func (r *bitReader) readBits(n int) (uint64, error) {
	var v uint64
	for i := 0; i < n; i++ {
		byteIdx := r.pos / 8
		if byteIdx >= len(r.buf) {
			return 0, errShortSnapshot
		}
		bitIdx := uint(7 - r.pos%8)
		bit := (r.buf[byteIdx] >> bitIdx) & 1
		v = (v << 1) | uint64(bit)
		r.pos++
	}
	return v, nil
}
