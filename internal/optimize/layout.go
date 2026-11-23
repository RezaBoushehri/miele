// Package optimize arranges rectangular parts on boards (staged guillotine cutting)
// and turns the result into CADmatic LC4 pattern trees. See docs/LC4_FORMAT.md.
package optimize

import (
	"fmt"
	"math"

	"miele/internal/lc4"
)

const eps = 1e-6

// Settings holds the saw geometry.
type Settings struct {
	Kerf     float64 // SaegeblattStaerke
	Trim1    float64 // edge trim before/after the strips (stage 1), each side
	Trim2    float64 // edge trim inside each strip (stage 2), each side
	Trim3    float64 // leading trim of a recut column (stage 3)
	MinRest4 float64 // smallest offcut allowed when trimming a part to length (stage 4)
	Rotate   bool    // global rotation switch
}

// DefaultSettings matches the sample runs (kerf 4.4, 15 mm edges, 10 mm recut).
func DefaultSettings() Settings {
	return Settings{Kerf: 4.4, Trim1: 15, Trim2: 15, Trim3: 10, MinRest4: 5, Rotate: true}
}

// Part is a required panel with a quantity.
type Part struct {
	Name          string
	Length, Width float64
	Qty           int
	Rotate        bool // false => keep grain (length stays along board length)
}

// Piece is one placed rectangle, in strip coordinates.
type Piece struct {
	Along, Across float64
	Part          *Part
	Rotated       bool
}

// Column stacks pieces across a strip.
type Column struct {
	Length float64
	Pieces []Piece
}

// Strip runs along the board; columns are placed along it.
type Strip struct {
	Width   float64
	Columns []Column
}

// BoardLayout is one board's complete plan.
type BoardLayout struct {
	L, W, T     float64
	RipAlongLen bool
	Strips      []Strip
	Qty         int
}

func (b *BoardLayout) stripLen() float64 {
	if b.RipAlongLen {
		return b.L
	}
	return b.W
}

func (b *BoardLayout) stackLen() float64 {
	if b.RipAlongLen {
		return b.W
	}
	return b.L
}

// AreaUsed returns the total part area on the board (mm^2).
func (b *BoardLayout) AreaUsed() float64 {
	var a float64
	for _, s := range b.Strips {
		for _, c := range s.Columns {
			for _, p := range c.Pieces {
				a += p.Along * p.Across
			}
		}
	}
	return a
}

// Signature identifies identical layouts (for merging into one pattern).
func (b *BoardLayout) Signature() string {
	s := fmt.Sprintf("%v|", b.RipAlongLen)
	for _, st := range b.Strips {
		s += fmt.Sprintf("[%.3f", st.Width)
		for _, c := range st.Columns {
			s += fmt.Sprintf("(%.3f", c.Length)
			for _, p := range c.Pieces {
				s += fmt.Sprintf("{%.3f,%.3f}", p.Along, p.Across)
			}
			s += ")"
		}
		s += "]"
	}
	return s
}

// Placement is a placed rectangle in board coordinates (x along L, y along W).
type Placement struct {
	X, Y, DX, DY float64
	Piece        Piece
}

func isDirect(c Column, s Strip) bool {
	return len(c.Pieces) == 1 && math.Abs(c.Pieces[0].Across-s.Width) < eps &&
		math.Abs(c.Pieces[0].Along-c.Length) < eps
}

func colTrim(c Column, s Strip, st Settings) float64 {
	used := st.Kerf * float64(len(c.Pieces)-1)
	for _, p := range c.Pieces {
		used += p.Across
	}
	if used+st.Trim3 > s.Width+eps {
		return 0
	}
	return st.Trim3
}

// Placements lists every part rectangle in board coordinates.
func (b *BoardLayout) Placements(st Settings) []Placement {
	var out []Placement
	a0 := st.Trim1
	for _, s := range b.Strips {
		bx := st.Trim2
		for _, c := range s.Columns {
			type r4 struct {
				u, v, du, dv float64
				p            Piece
			}
			var rects []r4
			if isDirect(c, s) {
				rects = append(rects, r4{bx, a0, c.Length, s.Width, c.Pieces[0]})
			} else {
				t3 := colTrim(c, s, st)
				aa := a0 + t3
				for _, p := range c.Pieces {
					rects = append(rects, r4{bx, aa, p.Along, p.Across, p})
					aa += p.Across + st.Kerf
				}
			}
			for _, rc := range rects {
				if b.RipAlongLen {
					out = append(out, Placement{rc.u, rc.v, rc.du, rc.dv, rc.p})
				} else {
					out = append(out, Placement{rc.v, rc.u, rc.dv, rc.du, rc.p})
				}
			}
			bx += c.Length + st.Kerf
		}
		a0 += s.Width + st.Kerf
	}
	return out
}

// ---- tree construction ---------------------------------------------------------------

func node(typ int32, size float64) *lc4.Node {
	return &lc4.Node{
		Type: typ, Size: size, Qty: 1,
		Name: lc4.Str(nodeName(typ)), Label: lc4.Str(""),
		E12:    defaultE12(typ),
		TailI1: -1, TailI2: -1, TailS1: lc4.Str(""), TailI3: -1, TailS2: lc4.Str("-1"),
	}
}

func nodeName(typ int32) string {
	switch typ {
	case lc4.TypeCut:
		return "Schnitt"
	case lc4.TypeStrip:
		return "Streifen"
	case lc4.TypeTrim:
		return "Anschnitt"
	case lc4.TypePart:
		return "Teil"
	case lc4.TypeRest:
		return "Rest"
	}
	return ""
}

func defaultE12(typ int32) int32 {
	if typ == lc4.TypeCut || typ == lc4.TypeTrim {
		return 4
	}
	return 0
}

// seq builds the child list: trim, cut, item, cut, item ..., cut, trailing trim.
func seq(items []*lc4.Node, extent, trim, kerf float64) ([]*lc4.Node, error) {
	used := kerf * float64(len(items)-1)
	for _, it := range items {
		used += it.Size
	}
	if len(items) == 1 && trim == 0 && math.Abs(used-extent) < eps {
		return items, nil
	}
	var out []*lc4.Node
	if trim > 0 {
		out = append(out, node(lc4.TypeTrim, trim), node(lc4.TypeCut, kerf))
	}
	for k, it := range items {
		if k > 0 {
			out = append(out, node(lc4.TypeCut, kerf))
		}
		out = append(out, it)
	}
	rest := round(extent-trim-used, 5)
	if rest < -eps {
		return nil, fmt.Errorf("items do not fit: extent %g need %g", extent, trim+used)
	}
	if rest > eps {
		out = append(out, node(lc4.TypeCut, kerf), node(lc4.TypeTrim, rest))
	}
	return out, nil
}

func nodeKey(n *lc4.Node) string {
	s := fmt.Sprintf("%d:%.5f(", n.Type, n.Size)
	for _, c := range n.Children {
		s += nodeKey(c)
	}
	return s + ")"
}

// compressCopies collapses runs of identical items into qty=n + (n-1) 'Copy' nodes.
func compressCopies(children []*lc4.Node) []*lc4.Node {
	i := 0
	for i < len(children) {
		n := children[i]
		if n.Type == lc4.TypePart || n.Type == lc4.TypeStrip {
			j := i
			run := 1
			for j+2 < len(children) && children[j+1].Type == lc4.TypeCut &&
				nodeKey(children[j+2]) == nodeKey(n) {
				run++
				j += 2
			}
			if run > 1 {
				n.Qty = int32(run)
				for k := 1; k < run; k++ {
					c := children[i+2*k]
					c.Qty = 0
					c.Label = lc4.Str("Copy")
					c.Flags = []bool{false, true, false}
				}
			}
			i = j + 1
		} else {
			i++
		}
	}
	return children
}

func strip(size float64, children []*lc4.Node) *lc4.Node {
	n := node(lc4.TypeStrip, size)
	n.Children = compressCopies(children)
	n.E12 = 0
	for _, c := range children {
		if c.Type == lc4.TypeStrip {
			n.E12 = 1
			break
		}
	}
	return n
}

func stripNodes(bl *BoardLayout, st Settings) ([]*lc4.Node, error) {
	var strips []*lc4.Node
	for _, s := range bl.Strips {
		var cols []*lc4.Node
		for _, c := range s.Columns {
			if isDirect(c, s) {
				cols = append(cols, node(lc4.TypePart, c.Length))
				continue
			}
			var parts []*lc4.Node
			for _, p := range c.Pieces {
				if math.Abs(p.Along-c.Length) < eps {
					parts = append(parts, node(lc4.TypePart, p.Across))
				} else {
					inner, err := seq([]*lc4.Node{node(lc4.TypePart, p.Along)}, c.Length, 0, st.Kerf)
					if err != nil {
						return nil, err
					}
					parts = append(parts, strip(p.Across, inner))
				}
			}
			ch, err := seq(parts, s.Width, colTrim(c, s, st), st.Kerf)
			if err != nil {
				return nil, err
			}
			cols = append(cols, strip(c.Length, ch))
		}
		ch, err := seq(cols, bl.stripLen(), st.Trim2, st.Kerf)
		if err != nil {
			return nil, err
		}
		strips = append(strips, strip(s.Width, ch))
	}
	return seq(strips, bl.stackLen(), st.Trim1, st.Kerf)
}

// BuildPattern turns one board layout into an LC4 pattern.
func BuildPattern(bl *BoardLayout, index int, st Settings) (*lc4.Pattern, error) {
	kids, err := stripNodes(bl, st)
	if err != nil {
		return nil, err
	}
	if bl.RipAlongLen {
		full := strip(bl.L, kids)
		full.E12 = 1
		kids = []*lc4.Node{full}
	}
	root := node(lc4.TypeRoot, bl.W)
	root.Name = lc4.Str(fmt.Sprintf("%05d", index+1))
	root.E12 = 1
	root.Children = compressCopies(kids)
	root.Root = lc4.RootInfo{A: 0, S: lc4.Str(""), B: 0, E14: 3, Idx: int32(index), Used: true}

	params := make([]lc4.Param, len(lc4.DefaultParams))
	copy(params, lc4.DefaultParams)
	p := &lc4.Pattern{Root: root, Length: bl.L, Qty: int32(bl.Qty), I2: 1,
		S1: lc4.Str(""), Params: params}
	p.Stats = computeStats(p, st)
	return p, nil
}

func computeStats(p *lc4.Pattern, st Settings) lc4.Stats {
	var nCuts, nParts int32
	var cutLen, area float64
	var walk func(n *lc4.Node, xext, yext float64, alongX bool)
	walk = func(n *lc4.Node, xext, yext float64, alongX bool) {
		for _, c := range n.Children {
			cx, cy := xext, c.Size
			if alongX {
				cx, cy = c.Size, yext
			}
			switch c.Type {
			case lc4.TypeCut:
				nCuts++
				if alongX {
					cutLen += yext
				} else {
					cutLen += xext
				}
			case lc4.TypePart:
				nParts++
				area += cx * cy
			}
			walk(c, cx, cy, !alongX)
		}
	}
	L, W := p.Length, p.Root.Size
	walk(p.Root, L, W, true)
	boardArea := L * W
	kerfArea := cutLen * st.Kerf
	waste := boardArea - area - kerfArea
	return lc4.Stats{
		I0: 1, WasteArea: round(waste, 4), BoardArea: boardArea, KerfArea: round(kerfArea, 4),
		NParts: nParts, PartsArea: round(area, 4),
		WastePct: (waste + kerfArea) / boardArea * 100,
		NCuts:    nCuts, CutLength: round(cutLen, 5),
		J2: 4,
	}
}

// BuildRun assembles the LC4 and PRV bytes for a set of layouts.
func BuildRun(name string, layouts []*BoardLayout, st Settings) (lc4Bytes, prvBytes []byte, err error) {
	var pats []*lc4.Pattern
	var boards []lc4.Board
	total := int32(0)
	for i, bl := range layouts {
		p, e := BuildPattern(bl, i, st)
		if e != nil {
			return nil, nil, e
		}
		pats = append(pats, p)
		boards = append(boards, lc4.Board{
			Length: bl.L, Width: bl.W, Thickness: bl.T, Qty: 1,
			S1: lc4.Str(""), S2: lc4.Str(""), S3: lc4.Str(""), S4: lc4.Str(""), S5: lc4.Str(""),
			RawA: []byte{0, 0, 0, 0}, RawB: []byte{0, 0, 0, 0},
		})
		total += int32(bl.Qty)
	}
	f := &lc4.File{Name: name, Generator: "manually generated", Boards: boards, Patterns: pats, Trailer: lc4.RunTrailer}
	prv := &lc4.PRV{Name: name, Generator: "manually generated", NPatterns: int32(len(pats)),
		NBoards: int32(len(boards)), TotalBoards: total, Status: 1}
	return f.Bytes(), prv.Bytes(), nil
}

func round(v float64, places int) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(v*p) / p
}
