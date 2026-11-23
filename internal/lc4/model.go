package lc4

import "fmt"

// Node type values (enum tag 0x11).
const (
	TypeRoot  int32 = 1
	TypeCut   int32 = 2  // Schnitt, size = kerf
	TypeStrip int32 = 3  // Streifen
	TypeTrim  int32 = 4  // Anschnitt (edge trim / waste)
	TypeRest  int32 = 10 // Rest (usable offcut)
	TypePart  int32 = 11 // Teil
)

// NStr is a nullable string (OK=false means a null string object).
type NStr struct {
	S  string
	OK bool
}

// Str makes a present (non-null) string.
func Str(s string) NStr { return NStr{s, true} }

// Null is the null string.
var Null = NStr{}

func readListHeader(r *Reader) (n int32, null bool, err error) {
	present, err := r.Bool()
	if err != nil {
		return 0, false, err
	}
	if !present {
		return 0, true, nil
	}
	if err = r.StructV(1); err != nil {
		return 0, false, err
	}
	v, err := r.Int()
	return v, false, err
}

func writeListHeader(w *Writer, n int32, null bool) {
	if null {
		w.Bool(false)
		return
	}
	w.Bool(true)
	w.Struct(1)
	w.Int(n)
}

func (w *Writer) nstr(s NStr) { w.String(s.S, s.OK) }

func (r *Reader) nstr() (NStr, error) {
	s, ok, err := r.String()
	return NStr{s, ok}, err
}

// Board is one source panel; there is one per pattern, in pattern order.
type Board struct {
	Length, Width, Thickness float64
	Qty                      int32
	S1, S2, S3, S4, S5       NStr
	I1, E13, Inner, E30      int32
	RawA, RawB               []byte
	Flags                    []bool
}

func readBoard(r *Reader) (Board, error) {
	var b Board
	fl := []bool{}
	if _, err := r.Bool(); err != nil {
		return b, err
	}
	if err := r.StructV(1); err != nil {
		return b, err
	}
	if err := r.StructV(4); err != nil {
		return b, err
	}
	var err error
	if b.S1, err = r.nstr(); err != nil {
		return b, err
	}
	if b.S2, err = r.nstr(); err != nil {
		return b, err
	}
	if b.S3, err = r.nstr(); err != nil {
		return b, err
	}
	readDecFlag := func(dst *float64) error {
		v, err := r.Decimal()
		if err != nil {
			return err
		}
		*dst = v
		f, err := r.Bool()
		fl = append(fl, f)
		return err
	}
	if err = readDecFlag(&b.Length); err != nil {
		return b, err
	}
	if err = readDecFlag(&b.Width); err != nil {
		return b, err
	}
	if err = readDecFlag(&b.Thickness); err != nil {
		return b, err
	}
	if b.Qty, err = r.Int(); err != nil {
		return b, err
	}
	if b.I1, err = r.Int(); err != nil {
		return b, err
	}
	if b.E13, err = r.Enum(0x13); err != nil {
		return b, err
	}
	f, err := r.Bool()
	if err != nil {
		return b, err
	}
	fl = append(fl, f)
	if _, err = r.Bool(); err != nil {
		return b, err
	}
	if err = r.StructV(1); err != nil {
		return b, err
	}
	if b.Inner, err = r.Int(); err != nil {
		return b, err
	}
	if f, err = r.Bool(); err != nil {
		return b, err
	}
	fl = append(fl, f)
	if b.RawA, err = r.Raw4(); err != nil {
		return b, err
	}
	if b.S4, err = r.nstr(); err != nil {
		return b, err
	}
	if b.S5, err = r.nstr(); err != nil {
		return b, err
	}
	if b.RawB, err = r.Raw4(); err != nil {
		return b, err
	}
	if b.E30, err = r.Enum(0x30); err != nil {
		return b, err
	}
	b.Flags = fl
	return b, nil
}

func (b Board) write(w *Writer) {
	fl := flagsOr(b.Flags, 5)
	w.Bool(true)
	w.Struct(1)
	w.Struct(4)
	w.nstr(b.S1)
	w.nstr(b.S2)
	w.nstr(b.S3)
	w.Decimal(b.Length)
	w.Bool(fl[0])
	w.Decimal(b.Width)
	w.Bool(fl[1])
	w.Decimal(b.Thickness)
	w.Bool(fl[2])
	w.Int(b.Qty)
	w.Int(b.I1)
	w.Enum(0x13, b.E13)
	w.Bool(fl[3])
	w.Bool(true)
	w.Struct(1)
	w.Int(b.Inner)
	w.Bool(fl[4])
	w.Raw4(b.RawA)
	w.nstr(b.S4)
	w.nstr(b.S5)
	w.Raw4(b.RawB)
	w.Enum(0x30, b.E30)
}

// RootInfo is the extra block carried only by a pattern's root node.
type RootInfo struct {
	A    int32
	S    NStr
	B    int32
	E14  int32
	Idx  int32 // pattern index
	Used bool  // whether RootInfo is present
}

// Node is a cell in the cutting tree (root, cut, strip, trim, rest or part).
type Node struct {
	Type        int32
	Size        float64
	Qty         int32
	Name, Label NStr
	E12         int32
	Children    []*Node
	Root        RootInfo
	E26, I26    int32
	TailI1      int32
	TailI2      int32
	TailS1      NStr
	TailI3      int32
	TailS2      NStr
	Flags       []bool
}

func readNode(r *Reader, isRoot bool) (*Node, error) {
	n := &Node{}
	fl := []bool{}
	if _, err := r.Bool(); err != nil {
		return nil, err
	}
	if err := r.StructV(9); err != nil {
		return nil, err
	}
	if isRoot {
		if err := r.StructV(9); err != nil {
			return nil, err
		}
	}
	var err error
	if n.Name, err = r.nstr(); err != nil {
		return nil, err
	}
	if n.Label, err = r.nstr(); err != nil {
		return nil, err
	}
	if n.Size, err = r.Decimal(); err != nil {
		return nil, err
	}
	f, err := r.Bool()
	if err != nil {
		return nil, err
	}
	fl = append(fl, f)
	if n.Qty, err = r.Int(); err != nil {
		return nil, err
	}
	if n.Type, err = r.Enum(0x11); err != nil {
		return nil, err
	}
	if n.E12, err = r.Enum(0x12); err != nil {
		return nil, err
	}
	if err = r.StructV(1); err != nil {
		return nil, err
	}
	n1, err := r.Int()
	if err != nil {
		return nil, err
	}
	if n1 != 0 {
		if n1 != 1 {
			return nil, fmt.Errorf("lc4: unexpected root info count %d", n1)
		}
		n.Root.Used = true
		if _, err = r.Bool(); err != nil {
			return nil, err
		}
		if err = r.StructV(3); err != nil {
			return nil, err
		}
		if n.Root.A, err = r.Int(); err != nil {
			return nil, err
		}
		if f, err = r.Bool(); err != nil {
			return nil, err
		}
		fl = append(fl, f)
		if n.Root.S, err = r.nstr(); err != nil {
			return nil, err
		}
		if f, err = r.Bool(); err != nil {
			return nil, err
		}
		fl = append(fl, f)
		if n.Root.B, err = r.Int(); err != nil {
			return nil, err
		}
		if _, err = r.Bool(); err != nil {
			return nil, err
		}
		if n.Root.E14, err = r.Enum(0x14); err != nil {
			return nil, err
		}
		if n.Root.Idx, err = r.Int(); err != nil {
			return nil, err
		}
	}
	if _, _, err = readListHeader(r); err != nil { // always-empty list
		return nil, err
	}
	nc, _, err := readListHeader(r)
	if err != nil {
		return nil, err
	}
	for i := int32(0); i < nc; i++ {
		c, err := readNode(r, false)
		if err != nil {
			return nil, err
		}
		n.Children = append(n.Children, c)
	}
	if f, err = r.Bool(); err != nil {
		return nil, err
	}
	fl = append(fl, f)
	if n.E26, err = r.Enum(0x26); err != nil {
		return nil, err
	}
	if n.I26, err = r.Int(); err != nil {
		return nil, err
	}
	if f, err = r.Bool(); err != nil {
		return nil, err
	}
	fl = append(fl, f)
	if n.TailI1, err = r.Int(); err != nil {
		return nil, err
	}
	if n.TailI2, err = r.Int(); err != nil {
		return nil, err
	}
	if _, _, err = readListHeader(r); err != nil {
		return nil, err
	}
	if _, _, err = readListHeader(r); err != nil {
		return nil, err
	}
	if n.TailS1, err = r.nstr(); err != nil {
		return nil, err
	}
	if n.TailI3, err = r.Int(); err != nil {
		return nil, err
	}
	if n.TailS2, err = r.nstr(); err != nil {
		return nil, err
	}
	n.Flags = fl
	return n, nil
}

func (n *Node) write(w *Writer, isRoot bool) {
	size := 3
	if n.Root.Used {
		size = 5
	}
	fl := flagsOr(n.Flags, size)
	fi := 0
	pop := func() bool { v := fl[fi]; fi++; return v }
	w.Bool(true)
	w.Struct(9)
	if isRoot {
		w.Struct(9)
	}
	w.nstr(n.Name)
	w.nstr(n.Label)
	w.Decimal(n.Size)
	w.Bool(pop())
	w.Int(n.Qty)
	w.Enum(0x11, n.Type)
	w.Enum(0x12, n.E12)
	w.Struct(1)
	if n.Root.Used {
		w.Int(1)
		w.Bool(true)
		w.Struct(3)
		w.Int(n.Root.A)
		w.Bool(pop())
		w.nstr(n.Root.S)
		w.Bool(pop())
		w.Int(n.Root.B)
		w.Bool(true)
		w.Enum(0x14, n.Root.E14)
		w.Int(n.Root.Idx)
	} else {
		w.Int(0)
	}
	writeListHeader(w, 0, false)
	writeListHeader(w, int32(len(n.Children)), false)
	for _, c := range n.Children {
		c.write(w, false)
	}
	w.Bool(pop())
	w.Enum(0x26, n.E26)
	w.Int(n.I26)
	w.Bool(pop())
	w.Int(n.TailI1)
	w.Int(n.TailI2)
	writeListHeader(w, 0, false)
	writeListHeader(w, 0, false)
	w.nstr(n.TailS1)
	w.Int(n.TailI3)
	w.nstr(n.TailS2)
}

// Stats is the per-pattern area summary (areas in mm^2).
type Stats struct {
	I0                   int32
	WasteArea, BoardArea float64
	NRest                int32
	RestArea, KerfArea   float64
	NParts               int32
	PartsArea, WastePct  float64
	E27                  int32
	Ints                 [4]int32
	NCuts                int32
	CutLength            float64
	J1, J2               int32
	S1                   NStr
	J3, J4               int32
	S2                   NStr
}

// Pattern is one cutting diagram plus its parameters and statistics.
type Pattern struct {
	Root   *Node
	Length float64
	Qty    int32 // number of boards cut with this pattern
	I2     int32
	S1     NStr
	E10    int32
	Params []Param
	Stats  Stats
	Flags  []bool
}

func readPattern(r *Reader) (*Pattern, error) {
	p := &Pattern{}
	fl := []bool{}
	var err error
	if p.Root, err = readNode(r, true); err != nil {
		return nil, err
	}
	if p.Length, err = r.Decimal(); err != nil {
		return nil, err
	}
	f, err := r.Bool()
	if err != nil {
		return nil, err
	}
	fl = append(fl, f)
	if p.Qty, err = r.Int(); err != nil {
		return nil, err
	}
	if p.I2, err = r.Int(); err != nil {
		return nil, err
	}
	if p.S1, err = r.nstr(); err != nil {
		return nil, err
	}
	if err = r.StructV(1); err != nil {
		return nil, err
	}
	if p.E10, err = r.Enum(0x10); err != nil {
		return nil, err
	}
	if f, err = r.Bool(); err != nil {
		return nil, err
	}
	fl = append(fl, f)
	np, _, err := readListHeader(r)
	if err != nil {
		return nil, err
	}
	for i := int32(0); i < np; i++ {
		if _, err = r.Bool(); err != nil {
			return nil, err
		}
		if err = r.StructV(2); err != nil {
			return nil, err
		}
		name, _, err := r.String()
		if err != nil {
			return nil, err
		}
		val, _, err := r.String()
		if err != nil {
			return nil, err
		}
		flag, err := r.Bool()
		if err != nil {
			return nil, err
		}
		p.Params = append(p.Params, Param{name, val, flag})
	}
	st := Stats{}
	if st.I0, err = r.Int(); err != nil {
		return nil, err
	}
	if f, err = r.Bool(); err != nil {
		return nil, err
	}
	fl = append(fl, f)
	if st.WasteArea, err = r.Double(); err != nil {
		return nil, err
	}
	if st.BoardArea, err = r.Double(); err != nil {
		return nil, err
	}
	if st.NRest, err = r.Int(); err != nil {
		return nil, err
	}
	if st.RestArea, err = r.Double(); err != nil {
		return nil, err
	}
	if st.KerfArea, err = r.Double(); err != nil {
		return nil, err
	}
	if st.NParts, err = r.Int(); err != nil {
		return nil, err
	}
	if st.PartsArea, err = r.Double(); err != nil {
		return nil, err
	}
	if st.WastePct, err = r.Double(); err != nil {
		return nil, err
	}
	if st.E27, err = r.Enum(0x27); err != nil {
		return nil, err
	}
	for i := 0; i < 4; i++ {
		if st.Ints[i], err = r.Int(); err != nil {
			return nil, err
		}
	}
	if st.NCuts, err = r.Int(); err != nil {
		return nil, err
	}
	if st.CutLength, err = r.Decimal(); err != nil {
		return nil, err
	}
	if f, err = r.Bool(); err != nil {
		return nil, err
	}
	fl = append(fl, f)
	if st.J1, err = r.Int(); err != nil {
		return nil, err
	}
	if st.J2, err = r.Int(); err != nil {
		return nil, err
	}
	if st.S1, err = r.nstr(); err != nil {
		return nil, err
	}
	if st.J3, err = r.Int(); err != nil {
		return nil, err
	}
	if f, err = r.Bool(); err != nil {
		return nil, err
	}
	fl = append(fl, f)
	if st.J4, err = r.Int(); err != nil {
		return nil, err
	}
	if st.S2, err = r.nstr(); err != nil {
		return nil, err
	}
	p.Stats = st
	p.Flags = fl
	return p, nil
}

func (p *Pattern) write(w *Writer) {
	fl := flagsOr(p.Flags, 5)
	fi := 0
	pop := func() bool { v := fl[fi]; fi++; return v }
	p.Root.write(w, true)
	w.Decimal(p.Length)
	w.Bool(pop())
	w.Int(p.Qty)
	w.Int(p.I2)
	w.nstr(p.S1)
	w.Struct(1)
	w.Enum(0x10, p.E10)
	w.Bool(pop())
	writeListHeader(w, int32(len(p.Params)), false)
	for _, pr := range p.Params {
		w.Bool(true)
		w.Struct(2)
		w.String(pr.Name, true)
		w.String(pr.Value, true)
		w.Bool(pr.Flag)
	}
	st := p.Stats
	w.Int(st.I0)
	w.Bool(pop())
	w.Double(st.WasteArea)
	w.Double(st.BoardArea)
	w.Int(st.NRest)
	w.Double(st.RestArea)
	w.Double(st.KerfArea)
	w.Int(st.NParts)
	w.Double(st.PartsArea)
	w.Double(st.WastePct)
	w.Enum(0x27, st.E27)
	for _, v := range st.Ints {
		w.Int(v)
	}
	w.Int(st.NCuts)
	w.Decimal(st.CutLength)
	w.Bool(pop())
	w.Int(st.J1)
	w.Int(st.J2)
	w.nstr(st.S1)
	w.Int(st.J3)
	w.Bool(pop())
	w.Int(st.J4)
	w.nstr(st.S2)
}

// File is a parsed LC4 run (boards and cutting patterns).
type File struct {
	Name      string
	Generator string
	Boards    []Board
	Patterns  []*Pattern
	Trailer   []byte
}

// Parse decodes an LC4 run.
func Parse(data []byte) (*File, error) {
	r := NewReader(data)
	f := &File{}
	if _, err := r.Bool(); err != nil {
		return nil, err
	}
	if err := r.StructV(12); err != nil {
		return nil, err
	}
	name, _, err := r.String()
	if err != nil {
		return nil, err
	}
	f.Name = name
	gen, _, err := r.String()
	if err != nil {
		return nil, err
	}
	f.Generator = gen
	if _, _, err = readListHeader(r); err != nil {
		return nil, err
	}
	nb, _, err := readListHeader(r)
	if err != nil {
		return nil, err
	}
	for i := int32(0); i < nb; i++ {
		b, err := readBoard(r)
		if err != nil {
			return nil, err
		}
		f.Boards = append(f.Boards, b)
	}
	if _, _, err = readListHeader(r); err != nil {
		return nil, err
	}
	np, _, err := readListHeader(r)
	if err != nil {
		return nil, err
	}
	for i := int32(0); i < np; i++ {
		p, err := readPattern(r)
		if err != nil {
			return nil, err
		}
		f.Patterns = append(f.Patterns, p)
	}
	f.Trailer = append([]byte(nil), data[r.Pos():]...)
	return f, nil
}

// Bytes encodes the run back to LC4.
func (f *File) Bytes() []byte {
	w := &Writer{}
	w.Bool(true)
	w.Struct(12)
	gen := f.Generator
	if gen == "" {
		gen = "manually generated"
	}
	w.String(f.Name, true)
	w.String(gen, true)
	writeListHeader(w, 0, false)
	writeListHeader(w, int32(len(f.Boards)), false)
	for _, b := range f.Boards {
		b.write(w)
	}
	writeListHeader(w, 0, false)
	writeListHeader(w, int32(len(f.Patterns)), false)
	for _, p := range f.Patterns {
		p.write(w)
	}
	trailer := f.Trailer
	if trailer == nil {
		trailer = RunTrailer
	}
	w.buf = append(w.buf, trailer...)
	return w.buf
}

func flagsOr(fl []bool, n int) []bool {
	if len(fl) >= n {
		return fl
	}
	out := make([]bool, n)
	copy(out, fl)
	return out
}
