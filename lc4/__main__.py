"""Command line: python -m lc4 parts.csv --board 2800x2070x18 --name "RUN NAME" -o out/

parts.csv columns: name,length,width,qty[,rotate]   (rotate: 1 = may be turned, 0 = keep grain)
Lines starting with # and an optional header line are ignored.
Writes <name>.LC4, <name>.prv (copy both to D:\\machine1\\prod_data\\Cadv51\\data\\Runs) and
<name>.html (drawing of every pattern).
"""
import argparse
import csv
import os
import sys

from .layout import Settings, build_run
from .optimize import Part, optimize, summary, NoFit
from .report import html_report


def read_parts(path):
    parts = []
    with open(path, newline='', encoding='utf-8-sig') as f:
        for row in csv.reader(f):
            if not row or row[0].strip().startswith('#'):
                continue
            try:
                length, width = float(row[1]), float(row[2])
            except (ValueError, IndexError):
                continue          # header or malformed line
            qty = int(float(row[3])) if len(row) > 3 and row[3].strip() else 1
            rot = True
            if len(row) > 4 and row[4].strip():
                rot = row[4].strip().lower() not in ('0', 'no', 'false', 'n', 'grain')
            parts.append(Part(row[0].strip(), length, width, qty, rot))
    return parts


def main(argv=None):
    ap = argparse.ArgumentParser(prog='lc4', description='Optimize parts on boards and write a CADmatic LC4 run.')
    ap.add_argument('parts', help='CSV file: name,length,width,qty[,rotate]')
    ap.add_argument('--board', required=True, help='LxWxT, e.g. 2800x2070x18')
    ap.add_argument('--name', help='run name (default: CSV file name)')
    ap.add_argument('-o', '--out', default='.', help='output directory')
    ap.add_argument('--kerf', type=float, default=4.4)
    ap.add_argument('--trim', type=float, default=15.0, help='edge trim of the board (mm)')
    ap.add_argument('--recut-trim', type=float, default=10.0)
    ap.add_argument('--no-rotate', action='store_true', help='keep grain for all parts')
    a = ap.parse_args(argv)

    L, W, T = (float(x) for x in a.board.lower().replace('*', 'x').split('x'))
    if W > L:
        L, W = W, L
    name = a.name or os.path.splitext(os.path.basename(a.parts))[0]
    st = Settings(kerf=a.kerf, trim1=a.trim, trim2=a.trim, trim3=a.recut_trim,
                  allow_rotate=not a.no_rotate)
    parts = read_parts(a.parts)
    try:
        layouts = optimize(parts, L, W, T, st)
    except NoFit as e:
        print('error:', e, file=sys.stderr)
        return 2
    lc4, prv = build_run(name, layouts, st)
    os.makedirs(a.out, exist_ok=True)
    base = os.path.join(a.out, name)
    with open(base + '.LC4', 'wb') as f:
        f.write(lc4)
    with open(base + '.prv', 'wb') as f:
        f.write(prv)
    with open(base + '.html', 'w', encoding='utf-8') as f:
        f.write(html_report(name, layouts, st, parts))
    s = summary(layouts)
    print(f'{s["boards"]} boards, {s["patterns"]} patterns, utilization {s["utilization"] * 100:.1f}%')
    print('written:', base + '.LC4', base + '.prv', base + '.html')
    return 0


if __name__ == '__main__':
    sys.exit(main())
