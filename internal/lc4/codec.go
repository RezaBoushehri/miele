// Package lc4 reads and writes the HOMAG CADmatic LC4 / PRV binary run format.
//
// The stream is a sequence of tagged values (little endian). See docs/LC4_FORMAT.md.
// The reader/writer here reproduce every sample run in test/ byte-for-byte.
package lc4

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"

	"golang.org/x/text/encoding/charmap"
)

// Decimals are stored as int64 scaled by 1e5 (all lengths are millimetres).
const decScale = 100000.0

func mustHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

var win1252 = charmap.Windows1252

// Reader is a cursor over an LC4/PRV byte stream.
type Reader struct {
	b   []byte
	pos int
}

// NewReader returns a Reader over b.
func NewReader(b []byte) *Reader { return &Reader{b: b} }

// EOF reports whether the cursor reached the end.
func (r *Reader) EOF() bool { return r.pos >= len(r.b) }

// Pos returns the current offset (useful in error messages).
func (r *Reader) Pos() int { return r.pos }

func (r *Reader) u16() (uint16, error) {
	if r.pos+2 > len(r.b) {
		return 0, fmt.Errorf("lc4: unexpected end of data at %#x", r.pos)
	}
	v := binary.LittleEndian.Uint16(r.b[r.pos:])
	r.pos += 2
	return v, nil
}

func (r *Reader) tag(want uint16) error {
	t, err := r.u16()
	if err != nil {
		return err
	}
	if t != want {
		return fmt.Errorf("lc4: offset %#x: expected tag %#x, got %#x", r.pos-2, want, t)
	}
	return nil
}

// Peek returns the next tag without consuming it.
func (r *Reader) Peek() (uint16, error) {
	p := r.pos
	t, err := r.u16()
	r.pos = p
	return t, err
}

// Bool reads a 01-tagged boolean (also used as a present/null flag).
func (r *Reader) Bool() (bool, error) {
	if err := r.tag(1); err != nil {
		return false, err
	}
	if r.pos >= len(r.b) {
		return false, fmt.Errorf("lc4: truncated bool at %#x", r.pos)
	}
	v := r.b[r.pos]
	r.pos++
	return v != 0, nil
}

// Struct reads an object header (05) and returns its class version.
func (r *Reader) Struct() (uint16, error) {
	if err := r.tag(5); err != nil {
		return 0, err
	}
	return r.u16()
}

// StructV reads an object header and checks the version.
func (r *Reader) StructV(want uint16) error {
	v, err := r.Struct()
	if err != nil {
		return err
	}
	if v != want {
		return fmt.Errorf("lc4: offset %#x: expected struct v%d, got v%d", r.pos-2, want, v)
	}
	return nil
}

// Int reads a 06-tagged int32.
func (r *Reader) Int() (int32, error) {
	if err := r.tag(6); err != nil {
		return 0, err
	}
	if r.pos+4 > len(r.b) {
		return 0, fmt.Errorf("lc4: truncated int at %#x", r.pos)
	}
	v := int32(binary.LittleEndian.Uint32(r.b[r.pos:]))
	r.pos += 4
	return v, nil
}

// Int64 reads an 08-tagged int64.
func (r *Reader) Int64() (int64, error) {
	if err := r.tag(8); err != nil {
		return 0, err
	}
	if r.pos+8 > len(r.b) {
		return 0, fmt.Errorf("lc4: truncated int64 at %#x", r.pos)
	}
	v := int64(binary.LittleEndian.Uint64(r.b[r.pos:]))
	r.pos += 8
	return v, nil
}

// Raw4 reads an 0a-tagged 4-byte blob.
func (r *Reader) Raw4() ([]byte, error) {
	if err := r.tag(0x0a); err != nil {
		return nil, err
	}
	if r.pos+4 > len(r.b) {
		return nil, fmt.Errorf("lc4: truncated raw4 at %#x", r.pos)
	}
	v := append([]byte(nil), r.b[r.pos:r.pos+4]...)
	r.pos += 4
	return v, nil
}

// Double reads a 0b-tagged float64.
func (r *Reader) Double() (float64, error) {
	if err := r.tag(0x0b); err != nil {
		return 0, err
	}
	if r.pos+8 > len(r.b) {
		return 0, fmt.Errorf("lc4: truncated double at %#x", r.pos)
	}
	v := math.Float64frombits(binary.LittleEndian.Uint64(r.b[r.pos:]))
	r.pos += 8
	return v, nil
}

// String reads a 0d string object; ok is false for a null string.
func (r *Reader) String() (s string, ok bool, err error) {
	if err = r.tag(0x0d); err != nil {
		return
	}
	present, err := r.Bool()
	if err != nil || !present {
		return
	}
	if err = r.tag(0x0e); err != nil {
		return
	}
	n, err := r.Int()
	if err != nil {
		return
	}
	if r.pos+int(n) > len(r.b) {
		return "", false, fmt.Errorf("lc4: truncated string at %#x", r.pos)
	}
	raw := r.b[r.pos : r.pos+int(n)]
	r.pos += int(n)
	dec, err := win1252.NewDecoder().Bytes(raw)
	if err != nil {
		return "", false, err
	}
	return string(dec), true, nil
}

// Decimal reads a 0f decimal (int64/1e5).
func (r *Reader) Decimal() (float64, error) {
	if err := r.tag(0x0f); err != nil {
		return 0, err
	}
	if err := r.StructV(1); err != nil {
		return 0, err
	}
	v, err := r.Int64()
	if err != nil {
		return 0, err
	}
	return float64(v) / decScale, nil
}

// Enum reads a tag-identified enum value (int32 payload).
func (r *Reader) Enum(tag uint16) (int32, error) {
	if err := r.tag(tag); err != nil {
		return 0, err
	}
	return r.Int()
}

// Writer builds an LC4/PRV byte stream.
type Writer struct {
	buf []byte
}

// Bytes returns the written data.
func (w *Writer) Bytes() []byte { return w.buf }

func (w *Writer) tag(t uint16) {
	w.buf = binary.LittleEndian.AppendUint16(w.buf, t)
}

// Bool writes a 01 boolean.
func (w *Writer) Bool(v bool) {
	w.tag(1)
	if v {
		w.buf = append(w.buf, 1)
	} else {
		w.buf = append(w.buf, 0)
	}
}

// Struct writes an object header with the given version.
func (w *Writer) Struct(version uint16) {
	w.tag(5)
	w.buf = binary.LittleEndian.AppendUint16(w.buf, version)
}

// Int writes a 06 int32.
func (w *Writer) Int(v int32) {
	w.tag(6)
	w.buf = binary.LittleEndian.AppendUint32(w.buf, uint32(v))
}

// Int64 writes an 08 int64.
func (w *Writer) Int64(v int64) {
	w.tag(8)
	w.buf = binary.LittleEndian.AppendUint64(w.buf, uint64(v))
}

// Raw4 writes an 0a 4-byte blob.
func (w *Writer) Raw4(v []byte) {
	w.tag(0x0a)
	b := make([]byte, 4)
	copy(b, v)
	w.buf = append(w.buf, b...)
}

// Double writes a 0b float64.
func (w *Writer) Double(v float64) {
	w.tag(0x0b)
	w.buf = binary.LittleEndian.AppendUint64(w.buf, math.Float64bits(v))
}

// String writes a 0d string object; call with ok=false for a null string.
func (w *Writer) String(s string, ok bool) {
	w.tag(0x0d)
	if !ok {
		w.Bool(false)
		return
	}
	w.Bool(true)
	w.tag(0x0e)
	enc, err := win1252.NewEncoder().Bytes([]byte(s))
	if err != nil {
		// mirror Python errors='replace'
		enc, _ = win1252.NewEncoder().Bytes([]byte(sanitize1252(s)))
	}
	w.Int(int32(len(enc)))
	w.buf = append(w.buf, enc...)
}

// Decimal writes an 0f decimal (value*1e5 rounded to int64).
func (w *Writer) Decimal(v float64) {
	w.tag(0x0f)
	w.Struct(1)
	w.Int64(int64(math.Round(v * decScale)))
}

// Enum writes a tag-identified enum value.
func (w *Writer) Enum(tag uint16, v int32) {
	w.tag(tag)
	w.Int(v)
}

func sanitize1252(s string) string {
	out := make([]rune, 0, len(s))
	enc := win1252.NewEncoder()
	for _, r := range s {
		if _, err := enc.Bytes([]byte(string(r))); err != nil {
			out = append(out, '?')
		} else {
			out = append(out, r)
		}
	}
	return string(out)
}
