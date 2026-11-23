package partsio

import (
	"os"
	"strings"
	"testing"
)

func TestReadCSV(t *testing.T) {
	csv := "name,length,width,qty,rotate\nbaghal,720,560,4,0\nkaf,800,560,2,1\n# comment\nخشاب,۴۵۰,۱۵۰,۱۰,بله\n"
	parts, err := ReadCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 3 {
		t.Fatalf("got %d parts", len(parts))
	}
	if parts[0].Name != "baghal" || parts[0].Length != 720 || parts[0].Rotate {
		t.Fatalf("row 0 wrong: %+v", parts[0])
	}
	if parts[2].Length != 450 || parts[2].Qty != 10 || !parts[2].Rotate {
		t.Fatalf("persian-digit row wrong: %+v", parts[2])
	}
}

func TestReadCSVNoHeader(t *testing.T) {
	parts, err := ReadCSV(strings.NewReader("a;720;560;2\nb;800;400;1\n"))
	if err != nil || len(parts) != 2 || parts[1].Width != 400 {
		t.Fatalf("semicolon/no-header parse failed: %v %+v", err, parts)
	}
}

func TestReadXLSX(t *testing.T) {
	p := os.Getenv("XLSX")
	if p == "" {
		t.Skip("no XLSX")
	}
	parts, err := ReadXLSX(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 3 || parts[0].Name != "baghal" || parts[2].Width != 396 {
		t.Fatalf("xlsx parse: %+v", parts)
	}
}
