# miele — برش بهینه‌ی ورق و خروجی LC4 برای اره‌ی HOMAG SAWTEQ S-200

لیست قطعه‌ها (طول، عرض، تعداد) و ابعاد ورق را می‌گیرد، قطعه‌ها را روی ورق می‌چیند و فایل
`LC4` + `prv` تحویل می‌دهد که مستقیم در پوشه‌ی Runs دستگاه
(`D:\machine1\prod_data\Cadv51\data\Runs`) کپی می‌شود.

## وضعیت فعلی (نمونه‌ی اولیه)

- `lc4/codec.py`, `lc4/model.py`, `lc4/prv.py` — خواندن/نوشتن فرمت LC4 و PRV. هر ۱۵۶ نمونه‌ی
  پوشه‌ی `test/` بایت‌به‌بایت یکسان بازنویسی می‌شوند. شرح فرمت: [docs/LC4_FORMAT.md](docs/LC4_FORMAT.md)
- `lc4/optimize.py` — چیدمان گیوتینی ۳ تا ۴ مرحله‌ای (نوار → ستون → برش مجدد → کوتاه‌کردن)،
  با رعایت ضخامت تیغ، دورریز لبه‌ها و جهت الیاف (قطعه‌هایی که نباید بچرخند).
- `lc4/layout.py` — تبدیل چیدمان به درخت برش CADmatic و محاسبه‌ی آمار هر الگو.
- `lc4/report.py` — گزارش HTML با نقشه‌ی هر الگو.

## اجرا

```bash
python3 -m lc4 examples/kitchen.csv --board 2800x2070x18 --name "TEST KITCHEN 18 MILL" -o out/
python3 -m unittest tests.test_lc4
```

فرمت CSV: `name,length,width,qty,rotate` — ستون rotate: `1` یعنی چرخش مجاز، `0` یعنی جهت الیاف حفظ شود.
تنظیمات: `--kerf 4.4` (ضخامت تیغ)، `--trim 15` (دورریز لبه‌ی ورق)، `--recut-trim 10`، `--no-rotate`.
