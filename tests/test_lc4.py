import glob
import os
import unittest

from lc4.model import LC4File, CUT, TRIM, PART
from lc4.prv import PRVFile
from lc4.layout import Settings, build_run
from lc4.optimize import Part, optimize

HERE = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SAMPLES = sorted(glob.glob(os.path.join(HERE, 'test', '*.LC4')))
PRVS = sorted(glob.glob(os.path.join(HERE, 'test', '*.prv')))


def geometry_errors(m):
    """extent == trim + items + kerf*(cuts - absorbed) for every node (see layout.py)."""
    errs = []

    def walk(n, xext, yext, along_x):
        if n.children:
            ext = xext if along_x else yext
            cuts = [c for c in n.children if c.type == CUT]
            k = cuts[0].size if cuts else 0
            non = sum(c.size for c in n.children if c.type != CUT)
            adj = (n.children[0].type == TRIM) + (len(n.children) > 1 and n.children[-1].type == TRIM)
            if abs(non + k * (len(cuts) - adj) - ext) > 1e-3:
                errs.append((n.name, ext))
            for c in n.children:
                if c.size <= 0:
                    errs.append((c.name, c.size))
                walk(c, *((c.size, yext) if along_x else (xext, c.size)), not along_x)
    for p in m.patterns:
        walk(p.root, p.length, p.root.size, True)
    return errs


class RoundTrip(unittest.TestCase):
    def test_lc4_samples_roundtrip(self):
        self.assertTrue(SAMPLES)
        for f in SAMPLES:
            data = open(f, 'rb').read()
            self.assertEqual(LC4File.parse(data).to_bytes(), data, f)

    def test_prv_samples_roundtrip(self):
        for f in PRVS:
            data = open(f, 'rb').read()
            self.assertEqual(PRVFile.parse(data).to_bytes(), data, f)


class Generate(unittest.TestCase):
    def test_generated_run_is_consistent(self):
        parts = [Part('a', 720, 560, 4, False), Part('b', 800, 560, 2), Part('c', 716, 396, 6),
                 Part('d', 764, 540, 5), Part('e', 2000, 300, 3), Part('f', 450, 150, 10)]
        st = Settings()
        layouts = optimize(parts, 2800, 2070, 18, st)
        lc4, prv = build_run('TEST', layouts, st)
        m = LC4File.parse(lc4)
        self.assertEqual(m.to_bytes(), lc4)
        self.assertEqual(geometry_errors(m), [])
        n_parts = sum(p.stats.n_parts * p.qty for p in m.patterns)
        self.assertEqual(n_parts, sum(p.qty for p in parts))
        # every placed piece lies inside the board and pieces do not overlap
        for b in layouts:
            rects = list(b.placements(st))
            for (x, y, dx, dy, _) in rects:
                self.assertTrue(x >= 0 and y >= 0 and x + dx <= b.L and y + dy <= b.W)
            for i, r in enumerate(rects):
                for s in rects[i + 1:]:
                    overlap = (r[0] < s[0] + s[2] - 1e-6 and s[0] < r[0] + r[2] - 1e-6 and
                               r[1] < s[1] + s[3] - 1e-6 and s[1] < r[1] + r[3] - 1e-6)
                    self.assertFalse(overlap)
        # grain: parts with rotate=False keep their length along the board length
        for b in layouts:
            for (x, y, dx, dy, p) in b.placements(st):
                if not p.part.rotate:
                    self.assertEqual((dx, dy), (p.part.length, p.part.width))
        self.assertEqual(PRVFile.parse(prv).total_boards, sum(p.qty for p in m.patterns))


if __name__ == '__main__':
    unittest.main()
