package draw

import (
	"image/png"
	"os"
	"testing"

	"miele/internal/optimize"
)

func TestRenderBoards(t *testing.T) {
	parts := []optimize.Part{
		{"baghal", 720, 560, 4, false}, {"kaf", 800, 560, 2, true}, {"dar", 716, 396, 6, false},
		{"tabaghe", 764, 540, 5, true}, {"pasht", 2000, 300, 3, true}, {"kesho", 450, 150, 10, true},
	}
	st := optimize.DefaultSettings()
	layouts, err := optimize.Optimize(parts, 2800, 2070, 18, st)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]int{}
	for i, p := range parts {
		names[p.Name] = i
	}
	out := os.Getenv("RENDER_OUT")
	if out == "" {
		return
	}
	for i, bl := range layouts {
		img := Board(bl, st, names, 0.3)
		f, _ := os.Create(out + "/board" + string(rune('0'+i)) + ".png")
		png.Encode(f, img)
		f.Close()
	}
}
