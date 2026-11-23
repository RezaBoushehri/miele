// Package app is the front-end-independent core: it ties the optimizer, the LC4 writer
// and rendering together so the GUI and the CLI share exactly one tested code path.
package app

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"

	"miele/internal/draw"
	"miele/internal/lc4"
	"miele/internal/optimize"
)

// DefaultOutputDir is where the SAWTEQ S-200 reads runs from (see instruction.txt).
const DefaultOutputDir = `D:\machine1\prod_data\Cadv51\data\Runs`

type imageRGBA = image.RGBA

var previewBG = color.RGBA{0xfa, 0xfa, 0xfa, 0xff}

// Board describes the source panel.
type Board struct {
	Length, Width, Thickness float64
}

// Job is one optimization request.
type Job struct {
	Name     string
	Board    Board
	Parts    []optimize.Part
	Settings optimize.Settings
}

// Result holds the computed layouts ready for preview and saving.
type Result struct {
	Job      Job
	Layouts  []*optimize.BoardLayout
	Summary  optimize.Summary
	colorIdx map[string]int
}

// Run validates the job and runs the optimizer.
func (j Job) Run() (*Result, error) {
	if strings.TrimSpace(j.Name) == "" {
		return nil, fmt.Errorf("نام کار را وارد کنید")
	}
	b := j.Board
	if b.Length <= 0 || b.Width <= 0 || b.Thickness <= 0 {
		return nil, fmt.Errorf("ابعاد ورق باید بزرگ‌تر از صفر باشد")
	}
	if b.Width > b.Length {
		b.Length, b.Width = b.Width, b.Length
	}
	if len(j.Parts) == 0 {
		return nil, fmt.Errorf("حداقل یک قطعه لازم است")
	}
	st := j.Settings
	if st.Kerf <= 0 {
		st = optimize.DefaultSettings()
	}
	layouts, err := optimize.Optimize(j.Parts, b.Length, b.Width, b.Thickness, st)
	if err != nil {
		if err == optimize.ErrNoFit {
			return nil, fmt.Errorf("یک یا چند قطعه در ورق %gx%g جا نمی‌شود", b.Length, b.Width)
		}
		return nil, err
	}
	colorIdx := map[string]int{}
	for i, p := range j.Parts {
		if _, ok := colorIdx[p.Name]; !ok {
			colorIdx[p.Name] = i
		}
	}
	j.Board = b
	j.Settings = st
	return &Result{Job: j, Layouts: layouts, Summary: optimize.Summarize(layouts), colorIdx: colorIdx}, nil
}

// PatternImage renders pattern i (0-based) to an image at the given scale.
func (r *Result) PatternImage(i int, scale float64) image.Image {
	return draw.Board(r.Layouts[i], r.Job.Settings, r.colorIdx, scale)
}

// ColorIndex exposes the part->colour mapping (for legends).
func (r *Result) ColorIndex() map[string]int { return r.colorIdx }

// Bytes returns the LC4 and PRV file contents.
func (r *Result) Bytes() (lc4Bytes, prvBytes []byte, err error) {
	return optimize.BuildRun(r.Job.Name, r.Layouts, r.Job.Settings)
}

// Save writes <name>.LC4 and <name>.prv into dir and returns the two paths.
func (r *Result) Save(dir string) (lc4Path, prvPath string, err error) {
	lc4Bytes, prvBytes, err := r.Bytes()
	if err != nil {
		return "", "", err
	}
	if err := verify(lc4Bytes); err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", "", err
	}
	name := SanitizeName(r.Job.Name)
	lc4Path = filepath.Join(dir, name+".LC4")
	prvPath = filepath.Join(dir, name+".prv")
	if err := os.WriteFile(lc4Path, lc4Bytes, 0644); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(prvPath, prvBytes, 0644); err != nil {
		return "", "", err
	}
	return lc4Path, prvPath, nil
}

// verify re-parses the generated file as a last safety check before it reaches the saw.
func verify(lc4Bytes []byte) error {
	m, err := lc4.Parse(lc4Bytes)
	if err != nil {
		return fmt.Errorf("خطای داخلی: فایل تولیدشده قابل خواندن نیست: %w", err)
	}
	if string(m.Bytes()) != string(lc4Bytes) {
		return fmt.Errorf("خطای داخلی: فایل تولیدشده پایدار نیست")
	}
	return nil
}

var badChars = strings.NewReplacer(
	`\`, "_", "/", "_", ":", "_", "*", "_", "?", "_", `"`, "_", "<", "_", ">", "_", "|", "_")

// SanitizeName makes a run name safe as a Windows file name.
func SanitizeName(s string) string {
	s = strings.TrimSpace(badChars.Replace(s))
	if s == "" {
		s = "run"
	}
	return s
}

// PreviewImage renders all patterns stacked into one image for on-screen preview.
func (r *Result) PreviewImage(scale float64) image.Image {
	imgs := make([]*imageRGBA, 0, len(r.Layouts))
	caps := make([]string, 0, len(r.Layouts))
	for i := range r.Layouts {
		imgs = append(imgs, draw.Board(r.Layouts[i], r.Job.Settings, r.colorIdx, scale))
		u := r.Layouts[i].AreaUsed() / (r.Layouts[i].L * r.Layouts[i].W) * 100
		caps = append(caps, fmt.Sprintf("Pattern %05d  -  boards: %d  -  used %.1f%%",
			i+1, r.Layouts[i].Qty, u))
	}
	return draw.Stack(imgs, caps, previewBG)
}
