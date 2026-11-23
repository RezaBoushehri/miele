package lc4

// PRV is the small run-summary file saved next to every LC4 run.
type PRV struct {
	Name        string
	Generator   string
	NPatterns   int32
	NBoards     int32
	I1, I2      int32
	TotalBoards int32 // sum of pattern quantities
	Status      int32
	E25         int32
	Tail        [3]int32
}

// ParsePRV decodes a .prv file.
func ParsePRV(data []byte) (*PRV, error) {
	r := NewReader(data)
	p := &PRV{}
	if _, err := r.Bool(); err != nil {
		return nil, err
	}
	if err := r.StructV(6); err != nil {
		return nil, err
	}
	name, _, err := r.String()
	if err != nil {
		return nil, err
	}
	p.Name = name
	gen, _, err := r.String()
	if err != nil {
		return nil, err
	}
	p.Generator = gen
	ints := []*int32{&p.NPatterns, &p.NBoards, &p.I1, &p.I2, &p.TotalBoards, &p.Status}
	for _, d := range ints {
		if *d, err = r.Int(); err != nil {
			return nil, err
		}
	}
	if p.E25, err = r.Enum(0x25); err != nil {
		return nil, err
	}
	for i := 0; i < 3; i++ {
		if _, err = r.Bool(); err != nil {
			return nil, err
		}
	}
	for i := 0; i < 3; i++ {
		if p.Tail[i], err = r.Int(); err != nil {
			return nil, err
		}
	}
	return p, nil
}

// Bytes encodes the summary back to .prv.
func (p *PRV) Bytes() []byte {
	w := &Writer{}
	gen := p.Generator
	if gen == "" {
		gen = "manually generated"
	}
	w.Bool(true)
	w.Struct(6)
	w.String(p.Name, true)
	w.String(gen, true)
	for _, v := range []int32{p.NPatterns, p.NBoards, p.I1, p.I2, p.TotalBoards, p.Status} {
		w.Int(v)
	}
	w.Enum(0x25, p.E25)
	w.Bool(false)
	w.Bool(false)
	w.Bool(false)
	for _, v := range p.Tail {
		w.Int(v)
	}
	return w.buf
}
