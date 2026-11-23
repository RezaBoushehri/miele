"""HTML report with one SVG drawing per cutting pattern."""
import html
from .layout import Settings

COLORS = ['#8ecae6', '#ffb703', '#90be6d', '#f28482', '#b8b8ff', '#f6bd60', '#84a59d',
          '#cdb4db', '#ffafcc', '#a2d2ff', '#e9c46a', '#95d5b2']


def _svg(bl, st, color_of, scale=0.28):
    L, W = bl.L, bl.W
    w, h = L * scale, W * scale
    out = [f'<svg viewBox="-2 -2 {w + 4:.1f} {h + 4:.1f}" width="{w + 4:.0f}" height="{h + 4:.0f}">',
           f'<rect x="0" y="0" width="{w:.1f}" height="{h:.1f}" fill="#eee" stroke="#555"/>']
    for x, y, dx, dy, p in bl.placements(st):
        name = p.part.name if p.part else ''
        out.append(f'<rect x="{x * scale:.1f}" y="{y * scale:.1f}" width="{dx * scale:.1f}" '
                   f'height="{dy * scale:.1f}" fill="{color_of(name)}" stroke="#333" stroke-width="0.6">'
                   f'<title>{html.escape(name)} {dx:g} x {dy:g}</title></rect>')
        fs = min(11, dx * scale / 6, dy * scale / 2.2)
        if fs >= 5:
            cx, cy = (x + dx / 2) * scale, (y + dy / 2) * scale
            out.append(f'<text x="{cx:.1f}" y="{cy:.1f}" font-size="{fs:.1f}" text-anchor="middle">'
                       f'<tspan x="{cx:.1f}" dy="-0.2em">{html.escape(name)}</tspan>'
                       f'<tspan x="{cx:.1f}" dy="1.1em">{dx:g}×{dy:g}</tspan></text>')
    out.append('</svg>')
    return '\n'.join(out)


def html_report(name, layouts, st: Settings, parts):
    names = [p.name for p in parts]
    color_of = lambda n: COLORS[names.index(n) % len(COLORS)] if n in names else '#ccc'
    total_boards = sum(b.qty for b in layouts)
    used = sum(b.area_used() * b.qty for b in layouts)
    area = sum(b.L * b.W * b.qty for b in layouts)
    rows = ''.join(f'<tr><td>{html.escape(p.name)}</td><td>{p.length:g}</td><td>{p.width:g}</td>'
                   f'<td>{p.qty}</td><td>{"✓" if p.rotate else "—"}</td></tr>' for p in parts)
    pats = []
    for i, b in enumerate(layouts):
        u = b.area_used() / (b.L * b.W) * 100
        pats.append(f'<section><h2>الگو {i + 1:05d} — تعداد ورق: {b.qty} — '
                    f'استفاده {u:.1f}٪ — جهت برش اول: {"طولی" if b.rip_along_length else "عرضی"}</h2>'
                    f'{_svg(b, st, color_of)}</section>')
    return f'''<!doctype html><html lang="fa" dir="rtl"><head><meta charset="utf-8">
<title>{html.escape(name)}</title>
<style>body{{font-family:Tahoma,sans-serif;margin:16px;background:#fafafa}}
table{{border-collapse:collapse}}td,th{{border:1px solid #bbb;padding:3px 8px}}
section{{margin:18px 0}}svg{{direction:ltr;background:#fff;max-width:100%;height:auto}}
h2{{font-size:15px}}</style></head><body>
<h1>{html.escape(name)}</h1>
<p>ورق: {layouts[0].L:g} × {layouts[0].W:g} × {layouts[0].T:g} — تعداد کل ورق: {total_boards}
— تعداد الگو: {len(layouts)} — راندمان کل: {used / area * 100:.1f}٪ — ضخامت تیغ: {st.kerf:g}</p>
<table><tr><th>قطعه</th><th>طول</th><th>عرض</th><th>تعداد</th><th>چرخش مجاز</th></tr>{rows}</table>
{''.join(pats)}
</body></html>'''
