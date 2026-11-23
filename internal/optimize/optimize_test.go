package optimize

import (
	"testing"

	"miele/internal/lc4"
)

func geomErrors(m *lc4.File) []string {
	var errs []string
	var walk func(n *lc4.Node, xext, yext float64, alongX bool)
	walk = func(n *lc4.Node, xext, yext float64, alongX bool) {
		if len(n.Children) > 0 {
			ext := yext
			if alongX {
				ext = xext
			}
			var k, non float64
			cuts := 0
			for _, c := range n.Children {
				if c.Type == lc4.TypeCut {
					cuts++
					k = c.Size
				} else {
					non += c.Size
				}
			}
			adj := 0
			if n.Children[0].Type == lc4.TypeTrim {
				adj++
			}
			if len(n.Children) > 1 && n.Children[len(n.Children)-1].Type == lc4.TypeTrim {
				adj++
			}
			if d := non + k*float64(cuts-adj) - ext; d > 1e-3 || d < -1e-3 {
				errs = append(errs, n.Name.S)
			}
			for _, c := range n.Children {
				if c.Size <= 0 {
					errs = append(errs, "nonpositive:"+c.Name.S)
				}
				cx, cy := xext, c.Size
				if alongX {
					cx, cy = c.Size, yext
				}
				walk(c, cx, cy, !alongX)
			}
		}
	}
	for _, p := range m.Patterns {
		walk(p.Root, p.Length, p.Root.Size, true)
	}
	return errs
}

func TestOptimizeConsistent(t *testing.T) {
	parts := []Part{
		{"a", 720, 560, 4, false}, {"b", 800, 560, 2, true}, {"c", 716, 396, 6, false},
		{"d", 764, 540, 5, true}, {"e", 2000, 300, 3, true}, {"f", 450, 150, 10, true},
	}
	st := DefaultSettings()
	layouts, err := Optimize(parts, 2800, 2070, 18, st)
	if err != nil {
		t.Fatal(err)
	}
	lc4b, prvb, err := BuildRun("TEST", layouts, st)
	if err != nil {
		t.Fatal(err)
	}
	m, err := lc4.Parse(lc4b)
	if err != nil {
		t.Fatal(err)
	}
	if !bytesEqual(m.Bytes(), lc4b) {
		t.Fatal("generated LC4 does not round-trip")
	}
	if e := geomErrors(m); len(e) > 0 {
		t.Fatalf("geometry errors: %v", e)
	}
	// part count conserved
	var want int32
	for _, p := range parts {
		want += int32(p.Qty)
	}
	var got int32
	for _, p := range m.Patterns {
		got += p.Stats.NParts * p.Qty
	}
	if got != want {
		t.Fatalf("part count: got %d want %d", got, want)
	}
	// overlap-free, inside board, grain respected
	for _, b := range layouts {
		pl := b.Placements(st)
		for _, r := range pl {
			if r.X < -1e-6 || r.Y < -1e-6 || r.X+r.DX > b.L+1e-6 || r.Y+r.DY > b.W+1e-6 {
				t.Fatalf("piece outside board: %+v", r)
			}
			if !r.Piece.Part.Rotate && (r.DX != r.Piece.Part.Length || r.DY != r.Piece.Part.Width) {
				t.Fatalf("grain violated for %s", r.Piece.Part.Name)
			}
		}
		for i := range pl {
			for j := i + 1; j < len(pl); j++ {
				a, c := pl[i], pl[j]
				if a.X < c.X+c.DX-1e-6 && c.X < a.X+a.DX-1e-6 &&
					a.Y < c.Y+c.DY-1e-6 && c.Y < a.Y+a.DY-1e-6 {
					t.Fatalf("overlap between %s and %s", a.Piece.Part.Name, c.Piece.Part.Name)
				}
			}
		}
	}
	p, err := lc4.ParsePRV(prvb)
	if err != nil {
		t.Fatal(err)
	}
	if int(p.TotalBoards) != Summarize(layouts).Boards {
		t.Fatalf("prv total boards mismatch")
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
