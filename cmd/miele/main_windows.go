//go:build windows

// Command miele is the Windows desktop app: enter or load a parts table, preview the
// cutting patterns, and — once approved — write the CADmatic LC4/PRV run to the saw folder.
// It runs only while the licensed USB flash drive is present.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"miele/internal/app"
	"miele/internal/license"
	"miele/internal/optimize"
	"miele/internal/partsio"
)

type ui struct {
	mw       *walk.MainWindow
	model    *partsModel
	tv       *walk.TableView
	leName   *walk.LineEdit
	leL, leW *walk.LineEdit
	leT      *walk.LineEdit
	leKerf   *walk.LineEdit
	leTrim   *walk.LineEdit
	leRecut  *walk.LineEdit
	cbRotate *walk.CheckBox
	leOut    *walk.LineEdit
	eName    *walk.LineEdit
	eL, eW   *walk.LineEdit
	eQty     *walk.LineEdit
	eRotate  *walk.CheckBox
	img      *walk.ImageView
	summary  *walk.Label
	btnSave  *walk.PushButton
	status   *walk.StatusBarItem

	result *app.Result
	bitmap *walk.Bitmap
}

func main() {
	st := license.Authorize()
	if !st.OK {
		walk.MsgBox(nil, "مجوز نامعتبر",
			st.Reason+"\n\nبرنامه بدون فلش مجوز اجرا نمی‌شود.",
			walk.MsgBoxIconError)
		os.Exit(1)
	}

	u := &ui{model: newPartsModel()}
	licInfo := "بدون انقضا"
	if st.License != nil && !st.License.Expiry.IsZero() {
		licInfo = "انقضا: " + st.License.Expiry.Format("2006-01-02")
	}
	owner := ""
	if st.License != nil {
		owner = st.License.Customer
	}

	if err := (MainWindow{
		AssignTo: &u.mw,
		Title:    "میله — بهینه‌ساز برش و خروجی LC4",
		MinSize:  Size{Width: 1040, Height: 720},
		Layout:   VBox{},
		StatusBarItems: []StatusBarItem{
			{AssignTo: &u.status, Text: fmt.Sprintf("مجوز: %s  —  %s", owner, licInfo), Width: 400},
		},
		Children: []Widget{
			u.topBar(),
			HSplitter{
				Children: []Widget{
					u.leftPanel(),
					u.rightPanel(),
				},
			},
		},
	}).Create(); err != nil {
		walk.MsgBox(nil, "خطا", err.Error(), walk.MsgBoxIconError)
		return
	}
	u.setDefaults()
	u.mw.Run()
}

func (u *ui) topBar() Widget {
	return Composite{
		Layout: Grid{Columns: 8},
		Children: []Widget{
			Label{Text: "نام کار:"},
			LineEdit{AssignTo: &u.leName, ColumnSpan: 3},
			Label{Text: "پوشه‌ی خروجی:"},
			LineEdit{AssignTo: &u.leOut, ColumnSpan: 2},
			PushButton{Text: "...", MaxSize: Size{Width: 36}, OnClicked: u.browseOut},

			Label{Text: "طول ورق:"},
			LineEdit{AssignTo: &u.leL},
			Label{Text: "عرض ورق:"},
			LineEdit{AssignTo: &u.leW},
			Label{Text: "ضخامت:"},
			LineEdit{AssignTo: &u.leT},
			Label{Text: "ضخامت تیغ:"},
			LineEdit{AssignTo: &u.leKerf},

			Label{Text: "دورریز لبه:"},
			LineEdit{AssignTo: &u.leTrim},
			Label{Text: "دورریز برش مجدد:"},
			LineEdit{AssignTo: &u.leRecut},
			CheckBox{AssignTo: &u.cbRotate, Text: "چرخش قطعات مجاز", ColumnSpan: 2, Checked: true},
		},
	}
}

func (u *ui) leftPanel() Widget {
	return Composite{
		Layout:  VBox{},
		MaxSize: Size{Width: 470},
		Children: []Widget{
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					PushButton{Text: "بارگذاری جدول (CSV/Excel)", OnClicked: u.loadFile},
					PushButton{Text: "نمونه", OnClicked: u.loadSample},
					PushButton{Text: "پاک‌کردن همه", OnClicked: u.clearAll},
				},
			},
			TableView{
				AssignTo:         &u.tv,
				Model:            u.model,
				MinSize:          Size{Height: 220},
				ColumnsOrderable: false,
				Columns: []TableViewColumn{
					{Title: "#", Width: 36},
					{Title: "نام", Width: 130},
					{Title: "طول", Width: 70},
					{Title: "عرض", Width: 70},
					{Title: "تعداد", Width: 55},
					{Title: "چرخش", Width: 60},
				},
				OnCurrentIndexChanged: u.onSelect,
			},
			GroupBox{
				Title:  "افزودن / ویرایش قطعه",
				Layout: Grid{Columns: 4},
				Children: []Widget{
					Label{Text: "نام:"},
					LineEdit{AssignTo: &u.eName, ColumnSpan: 3},
					Label{Text: "طول:"},
					LineEdit{AssignTo: &u.eL},
					Label{Text: "عرض:"},
					LineEdit{AssignTo: &u.eW},
					Label{Text: "تعداد:"},
					LineEdit{AssignTo: &u.eQty, Text: "1"},
					CheckBox{AssignTo: &u.eRotate, Text: "چرخش مجاز", Checked: true, ColumnSpan: 2},
					Composite{
						Layout:     HBox{MarginsZero: true},
						ColumnSpan: 4,
						Children: []Widget{
							PushButton{Text: "افزودن", OnClicked: u.addPart},
							PushButton{Text: "به‌روزرسانی انتخاب", OnClicked: u.updatePart},
							PushButton{Text: "حذف انتخاب", OnClicked: u.removePart},
						},
					},
				},
			},
			PushButton{Text: "بهینه‌سازی و پیش‌نمایش", OnClicked: u.optimize},
			Label{AssignTo: &u.summary, Text: ""},
		},
	}
}

func (u *ui) rightPanel() Widget {
	return Composite{
		Layout: VBox{},
		Children: []Widget{
			ScrollView{
				Layout:   VBox{},
				Children: []Widget{ImageView{AssignTo: &u.img, Mode: ImageViewModeIdeal}},
			},
			PushButton{
				AssignTo:  &u.btnSave,
				Text:      "تأیید و ذخیره خروجی (LC4 + prv)",
				Enabled:   false,
				OnClicked: u.save,
			},
		},
	}
}

func (u *ui) setDefaults() {
	u.leL.SetText("2800")
	u.leW.SetText("2070")
	u.leT.SetText("18")
	u.leKerf.SetText("4.4")
	u.leTrim.SetText("15")
	u.leRecut.SetText("10")
	u.leOut.SetText(app.DefaultOutputDir)
}

// ---- actions ----

func (u *ui) browseOut() {
	dlg := new(walk.FileDialog)
	dlg.Title = "پوشه‌ی خروجی"
	if ok, _ := dlg.ShowBrowseFolder(u.mw); ok && dlg.FilePath != "" {
		u.leOut.SetText(dlg.FilePath)
	}
}

func (u *ui) loadFile() {
	dlg := new(walk.FileDialog)
	dlg.Title = "بارگذاری جدول قطعات"
	dlg.Filter = "جدول (*.csv;*.xlsx)|*.csv;*.xlsx|همه (*.*)|*.*"
	if ok, _ := dlg.ShowOpen(u.mw); !ok || dlg.FilePath == "" {
		return
	}
	parts, err := partsio.ReadFile(dlg.FilePath)
	if err != nil {
		u.errorBox("خطا در خواندن فایل", err)
		return
	}
	u.model.setItems(fromParts(parts))
	if strings.TrimSpace(u.leName.Text()) == "" {
		u.leName.SetText(baseName(dlg.FilePath))
	}
	u.setStatus(fmt.Sprintf("%d قطعه بارگذاری شد", len(parts)))
}

func (u *ui) loadSample() {
	u.model.setItems([]partRow{
		{"baghal", 720, 560, 4, false},
		{"kaf", 800, 560, 2, true},
		{"dar", 716, 396, 6, false},
		{"tabaghe", 764, 540, 5, true},
		{"pasht", 2000, 300, 3, true},
		{"kesho", 450, 150, 10, true},
	})
	if strings.TrimSpace(u.leName.Text()) == "" {
		u.leName.SetText("نمونه")
	}
}

func (u *ui) clearAll() {
	u.model.setItems(nil)
	u.clearPreview()
}

func (u *ui) onSelect() {
	i := u.tv.CurrentIndex()
	if i < 0 || i >= len(u.model.items) {
		return
	}
	p := u.model.items[i]
	u.eName.SetText(p.Name)
	u.eL.SetText(num(p.Length))
	u.eW.SetText(num(p.Width))
	u.eQty.SetText(strconv.Itoa(p.Qty))
	u.eRotate.SetChecked(p.Rotate)
}

func (u *ui) readEdit() (partRow, bool) {
	l, ok1 := parseNum(u.eL.Text())
	w, ok2 := parseNum(u.eW.Text())
	q, ok3 := parseNum(u.eQty.Text())
	if !ok1 || !ok2 || l <= 0 || w <= 0 {
		u.errorBox("ورودی نامعتبر", fmt.Errorf("طول و عرض باید عدد مثبت باشند"))
		return partRow{}, false
	}
	qty := 1
	if ok3 && q >= 1 {
		qty = int(q)
	}
	name := strings.TrimSpace(u.eName.Text())
	if name == "" {
		name = fmt.Sprintf("P%d", len(u.model.items)+1)
	}
	return partRow{Name: name, Length: l, Width: w, Qty: qty, Rotate: u.eRotate.Checked()}, true
}

func (u *ui) addPart() {
	if p, ok := u.readEdit(); ok {
		u.model.add(p)
	}
}

func (u *ui) updatePart() {
	i := u.tv.CurrentIndex()
	if i < 0 {
		u.errorBox("انتخاب", fmt.Errorf("ابتدا یک ردیف را انتخاب کنید"))
		return
	}
	if p, ok := u.readEdit(); ok {
		u.model.update(i, p)
	}
}

func (u *ui) removePart() {
	i := u.tv.CurrentIndex()
	if i < 0 {
		return
	}
	u.model.remove(i)
}

func (u *ui) readJob() (app.Job, error) {
	L, ok1 := parseNum(u.leL.Text())
	W, ok2 := parseNum(u.leW.Text())
	T, ok3 := parseNum(u.leT.Text())
	if !ok1 || !ok2 || !ok3 {
		return app.Job{}, fmt.Errorf("ابعاد ورق را درست وارد کنید")
	}
	kerf, _ := parseNum(u.leKerf.Text())
	trim, _ := parseNum(u.leTrim.Text())
	recut, _ := parseNum(u.leRecut.Text())
	if kerf <= 0 {
		kerf = 4.4
	}
	st := optimize.Settings{Kerf: kerf, Trim1: trim, Trim2: trim, Trim3: recut, MinRest4: 5, Rotate: u.cbRotate.Checked()}
	return app.Job{
		Name:     u.leName.Text(),
		Board:    app.Board{Length: L, Width: W, Thickness: T},
		Parts:    u.model.parts(),
		Settings: st,
	}, nil
}

func (u *ui) optimize() {
	job, err := u.readJob()
	if err != nil {
		u.errorBox("ورودی", err)
		return
	}
	u.mw.SetEnabled(false)
	defer u.mw.SetEnabled(true)
	res, err := job.Run()
	if err != nil {
		u.errorBox("بهینه‌سازی", err)
		return
	}
	u.result = res
	if err := u.showPreview(res); err != nil {
		u.errorBox("پیش‌نمایش", err)
		return
	}
	s := res.Summary
	u.summary.SetText(fmt.Sprintf("تعداد ورق: %d   —   تعداد الگو: %d   —   راندمان: %.1f%%",
		s.Boards, s.Patterns, s.Utilization*100))
	u.btnSave.SetEnabled(true)
	u.setStatus("پیش‌نمایش آماده است — در صورت تأیید، خروجی را ذخیره کنید")
}

func (u *ui) showPreview(res *app.Result) error {
	img := res.PreviewImage(0.3)
	bmp, err := walk.NewBitmapFromImageForDPI(img, 96)
	if err != nil {
		return err
	}
	u.img.SetImage(bmp)
	if u.bitmap != nil {
		u.bitmap.Dispose()
	}
	u.bitmap = bmp
	return nil
}

func (u *ui) clearPreview() {
	u.img.SetImage(nil)
	if u.bitmap != nil {
		u.bitmap.Dispose()
		u.bitmap = nil
	}
	u.summary.SetText("")
	u.btnSave.SetEnabled(false)
	u.result = nil
}

func (u *ui) save() {
	if u.result == nil {
		return
	}
	dir := strings.TrimSpace(u.leOut.Text())
	if dir == "" {
		u.errorBox("خروجی", fmt.Errorf("پوشه‌ی خروجی را مشخص کنید"))
		return
	}
	name := app.SanitizeName(u.result.Job.Name)
	msg := fmt.Sprintf("فایل‌های زیر در این پوشه نوشته شوند؟\n\n%s\n\n%s.LC4\n%s.prv", dir, name, name)
	if walk.MsgBox(u.mw, "تأیید ذخیره", msg, walk.MsgBoxOKCancel|walk.MsgBoxIconQuestion) != walk.DlgCmdOK {
		return
	}
	lc4Path, prvPath, err := u.result.Save(dir)
	if err != nil {
		u.errorBox("ذخیره", err)
		return
	}
	walk.MsgBox(u.mw, "انجام شد",
		fmt.Sprintf("خروجی ذخیره شد:\n%s\n%s", lc4Path, prvPath), walk.MsgBoxIconInformation)
	u.setStatus("خروجی ذخیره شد: " + lc4Path)
}

func (u *ui) errorBox(title string, err error) {
	walk.MsgBox(u.mw, title, err.Error(), walk.MsgBoxIconError)
}

func (u *ui) setStatus(s string) {
	if u.status != nil {
		u.status.SetText(s)
	}
}

// ---- helpers ----

func parseNum(s string) (float64, bool) {
	s = strings.TrimSpace(strings.ReplaceAll(faDigits(s), ",", "."))
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}

func num(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }

func baseName(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		p = p[i+1:]
	}
	if i := strings.LastIndex(p, "."); i >= 0 {
		p = p[:i]
	}
	return p
}

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
