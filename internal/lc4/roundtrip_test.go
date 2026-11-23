package lc4

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func samples(t *testing.T, glob string) []string {
	root, _ := filepath.Abs(filepath.Join("..", "..", "test"))
	m, err := filepath.Glob(filepath.Join(root, glob))
	if err != nil || len(m) == 0 {
		t.Fatalf("no samples for %s", glob)
	}
	return m
}

func TestLC4Roundtrip(t *testing.T) {
	for _, f := range samples(t, "*.LC4") {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		m, err := Parse(data)
		if err != nil {
			t.Fatalf("%s: %v", filepath.Base(f), err)
		}
		if !bytes.Equal(m.Bytes(), data) {
			t.Fatalf("%s: roundtrip mismatch", filepath.Base(f))
		}
	}
}

func TestPRVRoundtrip(t *testing.T) {
	for _, f := range samples(t, "*.prv") {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		p, err := ParsePRV(data)
		if err != nil {
			t.Fatalf("%s: %v", filepath.Base(f), err)
		}
		if !bytes.Equal(p.Bytes(), data) {
			t.Fatalf("%s: roundtrip mismatch", filepath.Base(f))
		}
	}
}
