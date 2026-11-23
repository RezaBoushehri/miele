package optimize

import (
	"errors"
	"math"
	"sort"
)

// ErrNoFit is returned when a part cannot be placed on the board at all.
var ErrNoFit = errors.New("optimize: part does not fit on the board")

type orient struct {
	along, across float64
	rotated       bool
}

type otype struct {
	part *Part
	n    int
	oLen []orient // orientations when ripping along length
	cur  []orient
	area float64
}

func newType(p *Part, rotOK bool) *otype {
	L, W := p.Length, p.Width
	t := &otype{part: p, n: p.Qty, area: L * W}
	t.oLen = []orient{{L, W, false}}
	if rotOK && math.Abs(L-W) > eps {
		t.oLen = append(t.oLen, orient{W, L, true})
	}
	return t
}

func (t *otype) orients(ripAlongLen bool) []orient {
	if ripAlongLen {
		return t.oLen
	}
	out := make([]orient, len(t.oLen))
	for i, o := range t.oLen {
		out[i] = orient{o.across, o.along, o.rotated}
	}
	return out
}

func buildColumn(types []*otype, idx int, o orient, sw, capLen float64, st Settings) (Column, map[int]int, bool) {
	k := st.Kerf
	if o.across > sw+eps || o.along > capLen+eps {
		return Column{}, nil, false
	}
	used := map[int]int{idx: 1}
	col := Column{Length: o.along, Pieces: []Piece{{o.along, o.across, types[idx].part, o.rotated}}}
	if math.Abs(o.across-sw) < eps {
		return col, used, true
	}
	total := o.across
	roomOK := func(add float64) bool {
		return total+k+add <= sw-st.Trim3+eps || math.Abs(total+k+add-sw) < eps
	}
	for pass := 0; pass < 3; pass++ {
		for j, t := range types {
			if pass == 0 && j != idx {
				continue
			}
			if pass == 1 && j == idx {
				continue
			}
			for _, o2 := range t.cur {
				same := math.Abs(o2.along-o.along) < eps
				shorter := o2.along < o.along-eps && o.along-o2.along >= k+st.MinRest4
				if (pass < 2 && !same) || (pass == 2 && !shorter) {
					continue
				}
				for t.n-used[j] > 0 && roomOK(o2.across) {
					used[j]++
					col.Pieces = append(col.Pieces, Piece{o2.along, o2.across, t.part, o2.rotated})
					total += k + o2.across
				}
			}
		}
	}
	if total+st.Trim3 > sw+eps && math.Abs(total-sw) > eps {
		return Column{}, nil, false
	}
	return col, used, true
}

func buildStrip(types []*otype, sw, capLen float64, st Settings, rip bool) (Strip, map[int]int) {
	k := st.Kerf
	for _, t := range types {
		t.cur = t.orients(rip)
	}
	s := Strip{Width: sw}
	usedTot := map[int]int{}
	usedLen := 0.0
	for {
		var bestCol Column
		var bestUsed map[int]int
		bestScore := [2]float64{-1, -1}
		found := false
		for i, t := range types {
			if t.n-usedTot[i] <= 0 {
				continue
			}
			for _, o := range t.cur {
				need := o.along
				if len(s.Columns) > 0 {
					need += k
				}
				if usedLen+need > capLen+eps || o.across > sw+eps {
					continue
				}
				saved := make([]int, len(types))
				for j, tt := range types {
					saved[j] = tt.n
					tt.n -= usedTot[j]
				}
				col, used, ok := buildColumn(types, i, o, sw, capLen-usedLen, st)
				for j, tt := range types {
					tt.n = saved[j]
				}
				if !ok {
					continue
				}
				var area float64
				for _, p := range col.Pieces {
					area += p.Along * p.Across
				}
				fill := area / (col.Length * sw)
				score := [2]float64{fill, col.Length}
				if !found || score[0] > bestScore[0]+eps ||
					(math.Abs(score[0]-bestScore[0]) < eps && score[1] > bestScore[1]) {
					found = true
					bestScore = score
					bestCol = col
					bestUsed = used
				}
			}
		}
		if !found {
			break
		}
		for j, u := range bestUsed {
			usedTot[j] += u
		}
		s.Columns = append(s.Columns, bestCol)
		usedLen += bestCol.Length
		if len(s.Columns) > 1 {
			usedLen += k
		}
	}
	return s, usedTot
}

func fillBoard(types []*otype, L, W, T float64, st Settings, rip bool, mode string) *BoardLayout {
	k := st.Kerf
	bl := &BoardLayout{L: L, W: W, T: T, RipAlongLen: rip, Qty: 1}
	capLen := bl.stripLen() - 2*st.Trim2
	capStack := bl.stackLen() - 2*st.Trim1
	usedStack := 0.0
	for _, t := range types {
		t.cur = t.orients(rip)
	}
	for {
		widthSet := map[float64]bool{}
		for _, t := range types {
			if t.n <= 0 {
				continue
			}
			for _, o := range t.orients(rip) {
				if o.along <= capLen+eps {
					widthSet[o.across] = true
				}
			}
		}
		room := capStack - usedStack
		if len(bl.Strips) > 0 {
			room -= k
		}
		var widths []float64
		for w := range widthSet {
			if w <= room+eps {
				widths = append(widths, w)
			}
		}
		sort.Sort(sort.Reverse(sort.Float64Slice(widths)))
		if len(widths) == 0 {
			break
		}
		if mode == "widest" {
			widths = widths[:1]
		}
		var bestStrip Strip
		var bestUsed map[int]int
		bestScore := [2]float64{-1, -1}
		found := false
		for _, sw := range widths {
			s, used := buildStrip(types, sw, capLen, st, rip)
			if len(s.Columns) == 0 {
				continue
			}
			var area float64
			for _, c := range s.Columns {
				for _, p := range c.Pieces {
					area += p.Along * p.Across
				}
			}
			var score [2]float64
			if mode == "fill" {
				score = [2]float64{round(area/(sw*capLen), 3), sw}
			} else {
				score = [2]float64{sw, 0}
			}
			if !found || score[0] > bestScore[0]+eps ||
				(math.Abs(score[0]-bestScore[0]) < eps && score[1] > bestScore[1]) {
				found = true
				bestScore = score
				bestStrip = s
				bestUsed = used
			}
		}
		if !found {
			break
		}
		for j, u := range bestUsed {
			types[j].n -= u
		}
		bl.Strips = append(bl.Strips, bestStrip)
		usedStack += bestStrip.Width
		if len(bl.Strips) > 1 {
			usedStack += k
		}
	}
	return bl
}

var orders = []func(a, b *otype) bool{
	func(a, b *otype) bool { return a.area > b.area },
	func(a, b *otype) bool { return maxSide(a) > maxSide(b) },
	func(a, b *otype) bool { return a.oLen[0].across > b.oLen[0].across },
	func(a, b *otype) bool { return a.oLen[0].along > b.oLen[0].along },
}

func maxSide(t *otype) float64 { return math.Max(t.oLen[0].along, t.oLen[0].across) }

// Optimize arranges parts on L x W x T boards and returns merged board layouts.
func Optimize(parts []Part, L, W, T float64, st Settings) ([]*BoardLayout, error) {
	var live []Part
	for _, p := range parts {
		if p.Qty > 0 {
			live = append(live, p)
		}
	}
	for i := range live {
		p := &live[i]
		ok := false
		for _, o := range newType(p, st.Rotate && p.Rotate).oLen {
			if o.along <= L-2*st.Trim2+eps && o.across <= W-2*st.Trim1+eps {
				ok = true
			}
		}
		if !ok {
			return nil, ErrNoFit
		}
	}

	ripModes := []bool{true, false}
	var best []*BoardLayout
	bestScore := [2]float64{math.MaxFloat64, math.MaxFloat64}
	perBoardModes := []string{"best", "ripTrue", "ripFalse"}

	for _, less := range orders {
		for _, mode := range []string{"fill", "widest"} {
			for _, perBoard := range perBoardModes {
				types := make([]*otype, len(live))
				for i := range live {
					types[i] = newType(&live[i], st.Rotate && live[i].Rotate)
				}
				sort.SliceStable(types, func(i, j int) bool { return less(types[i], types[j]) })

				var boards []*BoardLayout
				progress := true
				for anyLeft(types) && progress {
					var rips []bool
					switch perBoard {
					case "best":
						rips = ripModes
					case "ripTrue":
						rips = []bool{true}
					default:
						rips = []bool{false}
					}
					var chosen *BoardLayout
					var chosenCounts []int
					bestArea := -1.0
					for _, rip := range rips {
						saved := snapshot(types)
						bl := fillBoard(types, L, W, T, st, rip, mode)
						a := bl.AreaUsed()
						if a > bestArea {
							bestArea = a
							chosen = bl
							chosenCounts = snapshot(types)
						}
						restore(types, saved)
					}
					if bestArea <= 0 {
						progress = false
						break
					}
					restore(types, chosenCounts)
					boards = append(boards, chosen)
				}
				if !progress {
					continue
				}
				score := [2]float64{float64(len(boards)), 0}
				if len(boards) > 0 {
					score[1] = boards[len(boards)-1].AreaUsed()
				}
				if score[0] < bestScore[0]-0.5 ||
					(math.Abs(score[0]-bestScore[0]) < 0.5 && score[1] < bestScore[1]) {
					bestScore = score
					best = boards
				}
			}
		}
	}
	if best == nil {
		return nil, ErrNoFit
	}
	return mergeIdentical(best), nil
}

func anyLeft(types []*otype) bool {
	for _, t := range types {
		if t.n > 0 {
			return true
		}
	}
	return false
}

func snapshot(types []*otype) []int {
	s := make([]int, len(types))
	for i, t := range types {
		s[i] = t.n
	}
	return s
}

func restore(types []*otype, s []int) {
	for i, t := range types {
		t.n = s[i]
	}
}

func mergeIdentical(boards []*BoardLayout) []*BoardLayout {
	var out []*BoardLayout
	for _, b := range boards {
		merged := false
		for _, o := range out {
			if o.Signature() == b.Signature() {
				o.Qty++
				merged = true
				break
			}
		}
		if !merged {
			out = append(out, b)
		}
	}
	return out
}

// Summary reports board count, pattern count and material utilization.
type Summary struct {
	Boards      int
	Patterns    int
	Utilization float64
}

// Summarize computes run totals.
func Summarize(layouts []*BoardLayout) Summary {
	n, used, total := 0, 0.0, 0.0
	for _, b := range layouts {
		n += b.Qty
		used += b.AreaUsed() * float64(b.Qty)
		total += b.L * b.W * float64(b.Qty)
	}
	u := 0.0
	if total > 0 {
		u = used / total
	}
	return Summary{Boards: n, Patterns: len(layouts), Utilization: u}
}
