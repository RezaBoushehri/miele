//go:build windows

package main

import (
	"github.com/lxn/walk"

	"miele/internal/optimize"
)

// partRow is one editable line in the parts table.
type partRow struct {
	Name          string
	Length, Width float64
	Qty           int
	Rotate        bool
}

func (p partRow) rotateText() string {
	if p.Rotate {
		return "بله"
	}
	return "خیر"
}

// partsModel is the walk TableView model backing the parts list.
type partsModel struct {
	walk.TableModelBase
	items []partRow
}

func newPartsModel() *partsModel { return &partsModel{} }

func (m *partsModel) RowCount() int { return len(m.items) }

func (m *partsModel) Value(row, col int) interface{} {
	p := m.items[row]
	switch col {
	case 0:
		return row + 1
	case 1:
		return p.Name
	case 2:
		return p.Length
	case 3:
		return p.Width
	case 4:
		return p.Qty
	case 5:
		return p.rotateText()
	}
	return nil
}

func (m *partsModel) setItems(items []partRow) {
	m.items = items
	m.PublishRowsReset()
}

func (m *partsModel) add(p partRow) {
	m.items = append(m.items, p)
	m.PublishRowsReset()
}

func (m *partsModel) update(i int, p partRow) {
	if i >= 0 && i < len(m.items) {
		m.items[i] = p
		m.PublishRowsReset()
	}
}

func (m *partsModel) remove(i int) {
	if i >= 0 && i < len(m.items) {
		m.items = append(m.items[:i], m.items[i+1:]...)
		m.PublishRowsReset()
	}
}

func (m *partsModel) parts() []optimize.Part {
	out := make([]optimize.Part, 0, len(m.items))
	for _, p := range m.items {
		out = append(out, optimize.Part{
			Name: p.Name, Length: p.Length, Width: p.Width, Qty: p.Qty, Rotate: p.Rotate,
		})
	}
	return out
}

func fromParts(parts []optimize.Part) []partRow {
	out := make([]partRow, 0, len(parts))
	for _, p := range parts {
		out = append(out, partRow{Name: p.Name, Length: p.Length, Width: p.Width, Qty: p.Qty, Rotate: p.Rotate})
	}
	return out
}
