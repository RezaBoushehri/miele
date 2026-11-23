package optimize

import (
	"math/rand"
	"testing"
)

func TestDeterministicAndFast(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	lens := []float64{300, 400, 450, 560, 600, 720, 764, 800, 1200, 2000}
	wids := []float64{100, 150, 300, 396, 450, 540, 560}
	var parts []Part
	for i := 0; i < 40; i++ {
		parts = append(parts, Part{
			Name: "p", Length: lens[r.Intn(len(lens))], Width: wids[r.Intn(len(wids))],
			Qty: 1 + r.Intn(8), Rotate: r.Float64() < 0.7,
		})
	}
	st := DefaultSettings()
	a, err := Optimize(parts, 2800, 2070, 18, st)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Optimize(parts, 2800, 2070, 18, st)
	if Summarize(a) != Summarize(b) {
		t.Fatal("non-deterministic result")
	}
	t.Logf("40 types -> %+v", Summarize(a))
}
