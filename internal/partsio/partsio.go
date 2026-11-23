// Package partsio loads and saves the parts list from CSV or XLSX.
package partsio

import (
	"archive/zip"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"miele/internal/optimize"
)

// header synonyms (lower-cased, spaces/underscores stripped) -> field.
var (
	nameKeys = set("name", "part", "title", "نام", "قطعه", "اسم")
	lenKeys  = set("length", "len", "l", "طول", "درازا")
	widKeys  = set("width", "w", "عرض", "پهنا")
	qtyKeys  = set("qty", "quantity", "count", "تعداد", "عدد", "number")
	rotKeys  = set("rotate", "rot", "grain", "چرخش", "گردش", "رگه")
)

func set(keys ...string) map[string]bool {
	m := map[string]bool{}
	for _, k := range keys {
		m[k] = true
	}
	return m
}

func norm(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.NewReplacer(" ", "", "_", "", "-", "", "(mm)", "", "mm", "").Replace(s)
	return s
}

// ReadFile loads parts from a .csv or .xlsx file (by extension).
func ReadFile(path string) ([]optimize.Part, error) {
	if strings.EqualFold(filepathExt(path), ".xlsx") {
		return ReadXLSX(path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ReadCSV(f)
}

func filepathExt(p string) string {
	if i := strings.LastIndex(p, "."); i >= 0 {
		return p[i:]
	}
	return ""
}

// ReadCSV parses a parts table from CSV (comma, semicolon or tab separated).
func ReadCSV(r io.Reader) ([]optimize.Part, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	text := strings.TrimPrefix(string(data), "\ufeff") // BOM
	comma := ','
	if strings.Count(text, ";") > strings.Count(text, ",") {
		comma = ';'
	} else if strings.Count(text, "\t") > strings.Count(text, ",") {
		comma = '\t'
	}
	cr := csv.NewReader(strings.NewReader(text))
	cr.Comma = comma
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true
	rows, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}
	return rowsToParts(rows)
}

// ReadXLSX reads the first worksheet of an .xlsx workbook (minimal, dependency-free).
func ReadXLSX(path string) ([]optimize.Part, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	var shared []string
	var sheet *zip.File
	sheetName := "xl/worksheets/sheet1.xml"
	for _, f := range zr.File {
		switch {
		case f.Name == "xl/sharedStrings.xml":
			shared, _ = readSharedStrings(f)
		case f.Name == sheetName:
			sheet = f
		}
	}
	if sheet == nil {
		for _, f := range zr.File {
			if strings.HasPrefix(f.Name, "xl/worksheets/") && strings.HasSuffix(f.Name, ".xml") {
				sheet = f
				break
			}
		}
	}
	if sheet == nil {
		return nil, fmt.Errorf("partsio: no worksheet in %s", path)
	}
	rows, err := readSheet(sheet, shared)
	if err != nil {
		return nil, err
	}
	return rowsToParts(rows)
}

func readSharedStrings(f *zip.File) ([]string, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	type si struct {
		T string   `xml:"t"`
		R []string `xml:"r>t"`
	}
	var doc struct {
		SI []si `xml:"si"`
	}
	if err := xml.NewDecoder(rc).Decode(&doc); err != nil {
		return nil, err
	}
	out := make([]string, len(doc.SI))
	for i, s := range doc.SI {
		if s.T != "" {
			out[i] = s.T
		} else {
			out[i] = strings.Join(s.R, "")
		}
	}
	return out, nil
}

func readSheet(f *zip.File, shared []string) ([][]string, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	type cell struct {
		R  string `xml:"r,attr"`
		T  string `xml:"t,attr"`
		V  string `xml:"v"`
		IS struct {
			T string `xml:"t"`
		} `xml:"is"`
	}
	var doc struct {
		Rows []struct {
			Cells []cell `xml:"c"`
		} `xml:"sheetData>row"`
	}
	if err := xml.NewDecoder(rc).Decode(&doc); err != nil {
		return nil, err
	}
	var out [][]string
	for _, row := range doc.Rows {
		var cells []string
		col := 0
		for _, c := range row.Cells {
			want := colIndex(c.R)
			for col < want {
				cells = append(cells, "")
				col++
			}
			val := c.V
			switch c.T {
			case "s":
				if i, err := strconv.Atoi(c.V); err == nil && i >= 0 && i < len(shared) {
					val = shared[i]
				}
			case "inlineStr":
				val = c.IS.T
			}
			cells = append(cells, val)
			col++
		}
		out = append(out, cells)
	}
	return out, nil
}

func colIndex(ref string) int {
	n := 0
	for _, r := range ref {
		if r >= 'A' && r <= 'Z' {
			n = n*26 + int(r-'A'+1)
		} else {
			break
		}
	}
	if n == 0 {
		return 0
	}
	return n - 1
}

func rowsToParts(rows [][]string) ([]optimize.Part, error) {
	// find columns: use a header row if the first non-empty row has recognizable headers.
	ci := map[string]int{"name": 0, "length": 1, "width": 2, "qty": 3, "rotate": 4}
	start := 0
	for ri, row := range rows {
		if allEmpty(row) {
			continue
		}
		if m, ok := matchHeader(row); ok {
			ci = m
			start = ri + 1
		} else {
			start = ri
		}
		break
	}
	var parts []optimize.Part
	for _, row := range rows[start:] {
		if allEmpty(row) {
			continue
		}
		name := strings.TrimSpace(get(row, ci["name"]))
		l, okL := parseNum(get(row, ci["length"]))
		w, okW := parseNum(get(row, ci["width"]))
		if !okL || !okW || l <= 0 || w <= 0 {
			continue // header leftovers or blank lines
		}
		qty := 1
		if q, ok := parseNum(get(row, ci["qty"])); ok && q > 0 {
			qty = int(q)
		}
		rotate := true
		if rv := strings.TrimSpace(get(row, ci["rotate"])); rv != "" {
			rotate = truthy(rv)
		}
		if name == "" {
			name = fmt.Sprintf("P%d", len(parts)+1)
		}
		parts = append(parts, optimize.Part{Name: name, Length: l, Width: w, Qty: qty, Rotate: rotate})
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("partsio: no valid rows found (expected columns: name, length, width, qty)")
	}
	return parts, nil
}

func matchHeader(row []string) (map[string]int, bool) {
	m := map[string]int{"name": -1, "length": -1, "width": -1, "qty": -1, "rotate": -1}
	hits := 0
	for i, cell := range row {
		n := norm(cell)
		switch {
		case nameKeys[n] && m["name"] < 0:
			m["name"] = i
			hits++
		case lenKeys[n] && m["length"] < 0:
			m["length"] = i
			hits++
		case widKeys[n] && m["width"] < 0:
			m["width"] = i
			hits++
		case qtyKeys[n] && m["qty"] < 0:
			m["qty"] = i
			hits++
		case rotKeys[n] && m["rotate"] < 0:
			m["rotate"] = i
			hits++
		}
	}
	if m["length"] < 0 || m["width"] < 0 || hits < 2 {
		return nil, false
	}
	return m, true
}

func allEmpty(row []string) bool {
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

func get(row []string, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}
	return row[i]
}

func parseNum(s string) (float64, bool) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	s = faDigits(s)
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}

func truthy(s string) bool {
	switch strings.ToLower(faDigits(strings.TrimSpace(s))) {
	case "0", "no", "false", "n", "grain", "خير", "خیر", "ندارد", "nه":
		return false
	}
	return true
}

// faDigits converts Persian/Arabic-Indic digits to ASCII.
func faDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= '۰' && r <= '۹':
			b.WriteRune('0' + (r - '۰'))
		case r >= '٠' && r <= '٩':
			b.WriteRune('0' + (r - '٠'))
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// WriteTemplateCSV writes an example parts file.
func WriteTemplateCSV(path string) error {
	return os.WriteFile(path, []byte(
		"name,length,width,qty,rotate\n"+
			"baghal,720,560,4,0\n"+
			"kaf,800,560,2,1\n"+
			"dar,716,396,6,0\n"), 0644)
}
