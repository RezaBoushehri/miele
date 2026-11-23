"""Panel cutting optimizer: staged guillotine layouts for a panel saw.

Each board is filled strip by strip. For every new strip all candidate strip widths are tried
and the strip that wastes the least is kept; inside a strip the column that fills the strip
width best is added next. This is run with several orderings and both rip directions, and the
plan with the fewest boards (then the emptiest last board) wins. Identical board layouts are
merged into one pattern with a quantity.
"""
from dataclasses import dataclass
from typing import List
from .layout import Settings, Piece, Column, Strip, BoardLayout, EPS


@dataclass
class Part:
    name: str
    length: float
    width: float
    qty: int = 1
    rotate: bool = True      # False when the grain direction must be kept (length along board L)


class NoFit(Exception):
    pass


class _Type:
    """A distinct part size with a remaining count."""
    def __init__(self, part, rot_ok):
        self.part = part
        self.n = int(part.qty)
        L, W = float(part.length), float(part.width)
        self.o_len = [(L, W, False)]                 # (along L, along W, rotated)
        if rot_ok and abs(L - W) > EPS:
            self.o_len.append((W, L, True))
        self.area = L * W

    def orients(self, rip_along_length):
        """(along strip, across strip, rotated) for the given rip direction."""
        if rip_along_length:
            return self.o_len
        return [(b, a, r) for (a, b, r) in self.o_len]


def _build_column(types, idx, o, sw, cap_len, st):
    """Column started by types[idx] in orientation o. Returns (Column, used dict) or None."""
    k = st.kerf
    along, across, rot = o
    if across > sw + EPS or along > cap_len + EPS:
        return None
    used = {idx: 1}
    pieces = [Piece(along, across, types[idx].part, rot)]
    if abs(across - sw) < EPS:
        return Column(along, pieces), used
    total = across

    def room_ok(t_add):
        # fits exactly without recut trim, or with the trim
        return total + k + t_add <= sw - st.trim3 + EPS or abs(total + k + t_add - sw) < EPS

    # stack the same type first, then others of the same length, then shorter ones (stage 4)
    for pass_ in (0, 1, 2):
        for j, t in enumerate(types):
            if pass_ == 0 and j != idx:
                continue
            if pass_ > 0 and j == idx and pass_ == 1:
                continue
            for (a2, c2, r2) in t._cur:
                same = abs(a2 - along) < EPS
                shorter = a2 < along - EPS and along - a2 >= k + st.min_rest4
                if (pass_ < 2 and not same) or (pass_ == 2 and not shorter):
                    continue
                while t.n - used.get(j, 0) > 0 and room_ok(c2):
                    used[j] = used.get(j, 0) + 1
                    pieces.append(Piece(a2, c2, t.part, r2))
                    total += k + c2
    if total + st.trim3 > sw + EPS and abs(total - sw) > EPS:
        return None
    return Column(along, pieces), used


def _build_strip(types, sw, cap_len, st, rip):
    """Greedy fill of one strip of width sw. Returns (Strip, used dict)."""
    k = st.kerf
    for t in types:
        t._cur = t.orients(rip)
    strip = Strip(sw)
    used_tot = {}
    used_len = 0.0
    while True:
        best = None
        for i, t in enumerate(types):
            if t.n - used_tot.get(i, 0) <= 0:
                continue
            for o in t._cur:
                need = o[0] + (k if strip.columns else 0)
                if used_len + need > cap_len + EPS or o[1] > sw + EPS:
                    continue
                # temporarily reduce counts by what this strip already used
                saved = [tt.n for tt in types]
                for j, u in used_tot.items():
                    types[j].n -= u
                res = _build_column(types, i, o, sw, cap_len - used_len, st)
                for tt, n in zip(types, saved):
                    tt.n = n
                if not res:
                    continue
                col, used = res
                area = sum(p.along * p.across for p in col.pieces)
                fill = area / (col.length * sw)
                score = (fill, col.length)
                if best is None or score > best[0]:
                    best = (score, col, used)
        if not best:
            break
        _, col, used = best
        for j, u in used.items():
            used_tot[j] = used_tot.get(j, 0) + u
        strip.columns.append(col)
        used_len += col.length + (k if len(strip.columns) > 1 else 0)
    return strip, used_tot


def _fill_board(types, L, W, T, st, rip, mode):
    k = st.kerf
    bl = BoardLayout(L, W, T, rip)
    cap_len = bl.strip_len - 2 * st.trim2
    cap_stack = bl.stack_len - 2 * st.trim1
    used_stack = 0.0
    for t in types:
        t._cur = t.orients(rip)
    while True:
        widths = set()
        for t in types:
            if t.n > 0:
                for (a, c, r) in t.orients(rip):
                    if a <= cap_len + EPS:
                        widths.add(c)
        room = cap_stack - used_stack - (k if bl.strips else 0)
        widths = sorted((w for w in widths if w <= room + EPS), reverse=True)
        if not widths:
            break
        if mode == 'widest':
            widths = widths[:1]
        best = None
        for sw in widths:
            strip, used = _build_strip(types, sw, cap_len, st, rip)
            if not strip.columns:
                continue
            area = sum(p.along * p.across for c in strip.columns for p in c.pieces)
            fill = area / (sw * cap_len)
            score = (round(fill, 3), sw) if mode == 'fill' else (sw,)
            if best is None or score > best[0]:
                best = (score, strip, used)
        if not best:
            break
        _, strip, used = best
        for j, u in used.items():
            types[j].n -= u
        bl.strips.append(strip)
        used_stack += strip.width + (k if len(bl.strips) > 1 else 0)
    return bl


ORDERS = {
    'area': lambda t: -t.area,
    'maxside': lambda t: -max(t.o_len[0][:2]),
    'width': lambda t: -t.o_len[0][1],
    'length': lambda t: -t.o_len[0][0],
}


def optimize(parts: List[Part], L: float, W: float, T: float, st: Settings = None,
             rip_modes=(True, False)):
    st = st or Settings()
    parts = [p for p in parts if int(p.qty) > 0]
    for p in parts:
        ok = False
        for (a, b, r) in _Type(p, st.allow_rotate and p.rotate).o_len:
            if a <= L - 2 * st.trim2 + EPS and b <= W - 2 * st.trim1 + EPS:
                ok = True
        if not ok:
            raise NoFit(f'part "{p.name}" {p.length}x{p.width} does not fit on board {L}x{W}')

    best = None
    for order in ORDERS.values():
        for mode in ('fill', 'widest'):
            for per_board in ('best',) + tuple(rip_modes):
                types = sorted((_Type(p, st.allow_rotate and p.rotate) for p in parts), key=order)
                boards = []
                while any(t.n > 0 for t in types):
                    cands = []
                    for rip in (rip_modes if per_board == 'best' else (per_board,)):
                        saved = [t.n for t in types]
                        bl = _fill_board(types, L, W, T, st, rip, mode)
                        cands.append((bl.area_used(), bl, [t.n for t in types]))
                        for t, n in zip(types, saved):
                            t.n = n
                    area, bl, counts = max(cands, key=lambda c: c[0])
                    if area <= 0:
                        raise NoFit('optimizer made no progress')
                    for t, n in zip(types, counts):
                        t.n = n
                    boards.append(bl)
                score = (len(boards), boards[-1].area_used())
                if best is None or score < best[0]:
                    best = (score, boards)
    return merge_identical(best[1])


def merge_identical(boards):
    out = []
    for b in boards:
        for o in out:
            if o.signature() == b.signature():
                o.qty += 1
                break
        else:
            out.append(b)
    return out


def summary(layouts, st: Settings = None):
    n = sum(b.qty for b in layouts)
    used = sum(b.area_used() * b.qty for b in layouts)
    total = sum(b.L * b.W * b.qty for b in layouts)
    return {'boards': n, 'patterns': len(layouts), 'utilization': used / total if total else 0}
