// Command mielecli is the headless engine: load parts, optimize, write the LC4 run.
// The GUI (cmd/miele) shares the same core (internal/app). This exists for testing,
// automation and as a fallback where the GUI cannot run.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"miele/internal/app"
	"miele/internal/optimize"
	"miele/internal/partsio"
)

func main() {
	board := flag.String("board", "2800x2070x18", "board size LxWxT in mm")
	name := flag.String("name", "", "run name (default: input file name)")
	out := flag.String("out", ".", "output directory")
	kerf := flag.Float64("kerf", 4.4, "saw kerf (mm)")
	trim := flag.Float64("trim", 15, "board edge trim (mm)")
	recut := flag.Float64("recut-trim", 10, "recut leading trim (mm)")
	noRotate := flag.Bool("no-rotate", false, "keep grain for all parts")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: mielecli [flags] parts.csv|parts.xlsx")
		flag.PrintDefaults()
		os.Exit(2)
	}
	in := flag.Arg(0)
	parts, err := partsio.ReadFile(in)
	if err != nil {
		die(err)
	}
	L, W, T, err := parseBoard(*board)
	if err != nil {
		die(err)
	}
	runName := *name
	if runName == "" {
		base := in
		if i := strings.LastIndexAny(base, `/\`); i >= 0 {
			base = base[i+1:]
		}
		if i := strings.LastIndex(base, "."); i >= 0 {
			base = base[:i]
		}
		runName = base
	}
	st := optimize.Settings{Kerf: *kerf, Trim1: *trim, Trim2: *trim, Trim3: *recut, MinRest4: 5, Rotate: !*noRotate}
	job := app.Job{Name: runName, Board: app.Board{Length: L, Width: W, Thickness: T}, Parts: parts, Settings: st}
	res, err := job.Run()
	if err != nil {
		die(err)
	}
	lc4Path, prvPath, err := res.Save(*out)
	if err != nil {
		die(err)
	}
	s := res.Summary
	fmt.Printf("%d قطعه در %d ورق (%d الگو)، راندمان %.1f%%\n",
		countParts(parts), s.Boards, s.Patterns, s.Utilization*100)
	fmt.Println("نوشته شد:", lc4Path)
	fmt.Println("نوشته شد:", prvPath)
}

func countParts(parts []optimize.Part) int {
	n := 0
	for _, p := range parts {
		n += p.Qty
	}
	return n
}

func parseBoard(s string) (L, W, T float64, err error) {
	s = strings.ToLower(strings.NewReplacer("*", "x", "×", "x").Replace(s))
	var a, b, c float64
	if _, err = fmt.Sscanf(s, "%gx%gx%g", &a, &b, &c); err != nil {
		return 0, 0, 0, fmt.Errorf("board must look like 2800x2070x18")
	}
	return a, b, c, nil
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
