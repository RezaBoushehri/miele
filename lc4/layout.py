"""Cutting layout (what the optimizer produces) and its conversion to LC4 pattern trees.

Geometry (same as CADmatic): the board is L (length, long side) x W (width).
A layout is a 3/4-stage guillotine plan:
  stage 1: rip cuts divide the board into strips (Streifen);
  stage 2: cross cuts divide each strip into columns;
  stage 3: recuts divide a column across the strip into the final parts;
  stage 4: (optional) a part shorter than its column is trimmed to length.

`rip_along_length=True` (the usual CADmatic way, 619 of 716 sample patterns): strips run along
the full board length L and are stacked across W. In the LC4 tree this is written as
root -> one full-length Streifen -> strips. With False the strips run across W and are
stacked along L (root -> strips directly).

In strip coordinates: `along` = the strip's long direction, `across` = its width direction.

LC4 size convention (verified on all 156 sample files): a leading trim ("Anschnitt") includes the
kerf of the cut that follows it, and so does a trailing waste trim. Hence for every node
    extent = trim + sum(item sizes) + kerf * (number of items - 1) + trailing_trim
"""
from dataclasses import dataclass, field
from typing import List
from . import model as M
from .model import Node, Pattern, Board, Param, Stats, LC4File
from .prv import PRVFile
from .template import DEFAULT_PARAMS, RUN_TRAILER

EPS = 1e-6


@dataclass
class Settings:
    kerf: float = 4.4          # SaegeblattStaerke
    trim1: float = 15.0        # edge trim before/after the strips (stage 1), both sides
    trim2: float = 15.0        # edge trim inside each strip (stage 2), both sides
    trim3: float = 10.0        # leading trim of a recut column (stage 3)
    min_rest4: float = 5.0     # smallest offcut allowed when trimming a part to length (stage 4)
    allow_rotate: bool = True  # global switch; parts can also forbid rotation (grain)


@dataclass
class Piece:
    along: float               # size along the strip
    across: float              # size across the strip
    part: object = None        # source part (name, ...)
    rotated: bool = False      # True when the part was turned 90 degrees on the board


@dataclass
class Column:
    length: float              # size along the strip
    pieces: List[Piece] = field(default_factory=list)   # stacked across the strip


@dataclass
class Strip:
    width: float               # size across the strip
    columns: List[Column] = field(default_factory=list)


@dataclass
class BoardLayout:
    L: float
    W: float
    T: float
    rip_along_length: bool = True
    strips: List[Strip] = field(default_factory=list)
    qty: int = 1

    @property
    def strip_len(self):       # extent along the strips
        return self.L if self.rip_along_length else self.W

    @property
    def stack_len(self):       # extent across the strips
        return self.W if self.rip_along_length else self.L

    def signature(self):
        return (self.rip_along_length,) + tuple(
            (s.width, tuple((c.length, tuple((p.along, p.across) for p in c.pieces))
                            for c in s.columns)) for s in self.strips)

    def pieces(self):
        for s in self.strips:
            for c in s.columns:
                yield from c.pieces

    def area_used(self):
        return sum(p.along * p.across for p in self.pieces())

    def placements(self, st: Settings):
        """Yield (x, y, dx, dy, piece): x along L, y along W, origin at the board corner."""
        a0 = st.trim1
        for s in self.strips:
            b = st.trim2
            for c in s.columns:
                if _is_direct(c, s):
                    rects = [(b, a0, c.length, s.width, c.pieces[0])]
                else:
                    t3 = _col_trim(c, s, st)
                    aa = a0 + t3
                    rects = []
                    for p in c.pieces:
                        rects.append((b, aa, p.along, p.across, p))
                        aa += p.across + st.kerf
                for (u, v, du, dv, p) in rects:
                    if self.rip_along_length:
                        yield u, v, du, dv, p
                    else:
                        yield v, u, dv, du, p
                b += c.length + st.kerf
            a0 += s.width + st.kerf


def _is_direct(c: Column, s: Strip):
    return (len(c.pieces) == 1 and abs(c.pieces[0].across - s.width) < EPS
            and abs(c.pieces[0].along - c.length) < EPS)


def _col_trim(c: Column, s: Strip, st: Settings):
    """Leading recut trim; dropped when the pieces fill the strip width exactly without it."""
    used = sum(p.across for p in c.pieces) + st.kerf * (len(c.pieces) - 1)
    return 0.0 if used + st.trim3 > s.width + EPS else st.trim3


# ---- tree construction ---------------------------------------------------------------
def _seq(items, extent, trim, kerf):
    """Children list: trim, cut, item, cut, item ..., cut, trailing trim."""
    used = sum(i.size for i in items) + kerf * (len(items) - 1)
    if len(items) == 1 and trim == 0 and abs(used - extent) < EPS:
        return list(items)                       # one full-size child, no cut at all
    out = []
    if trim > 0:
        out += [Node(M.TRIM, trim), Node(M.CUT, kerf)]
    for k, it in enumerate(items):
        if k:
            out.append(Node(M.CUT, kerf))
        out.append(it)
    rest = round(extent - trim - used, 5)
    if rest < -EPS:
        raise ValueError(f'items do not fit: extent {extent}, need {trim + used}')
    if rest > EPS:
        out += [Node(M.CUT, kerf), Node(M.TRIM, rest)]
    return out


def _key(n):
    return (n.type, round(n.size, 5), tuple(_key(c) for c in n.children))


def _compress_copies(children):
    """CADmatic stores runs of identical items as: first item qty=n, then n-1 'Copy' items qty=0."""
    i = 0
    while i < len(children):
        n = children[i]
        if n.type in (M.PART, M.STRIP):
            j = i
            run = [n]
            while (j + 2 < len(children) and children[j + 1].type == M.CUT
                   and _key(children[j + 2]) == _key(n)):
                run.append(children[j + 2])
                j += 2
            if len(run) > 1:
                n.qty = len(run)
                for c in run[1:]:
                    c.qty = 0
                    c.label = 'Copy'
                    c.flags = [False, True, False]
            i = j + 1
        else:
            i += 1
    return children


def _strip(size, children):
    n = Node(M.STRIP, size, children=_compress_copies(list(children)))
    n.e12 = 1 if any(c.type == M.STRIP for c in children) else 0
    return n


def _strip_nodes(bl: BoardLayout, st: Settings):
    strips = []
    for s in bl.strips:
        cols = []
        for c in s.columns:
            if _is_direct(c, s):
                cols.append(Node(M.PART, c.length))
                continue
            parts = []
            for p in c.pieces:
                if abs(p.along - c.length) < EPS:
                    parts.append(Node(M.PART, p.across))
                else:   # stage 4: trim the part to its length
                    parts.append(_strip(p.across, _seq([Node(M.PART, p.along)], c.length, 0, st.kerf)))
            cols.append(_strip(c.length, _seq(parts, s.width, _col_trim(c, s, st), st.kerf)))
        strips.append(_strip(s.width, _seq(cols, bl.strip_len, st.trim2, st.kerf)))
    return _seq(strips, bl.stack_len, st.trim1, st.kerf)


def build_pattern(bl: BoardLayout, index: int, st: Settings) -> Pattern:
    kids = _strip_nodes(bl, st)
    if bl.rip_along_length:
        full = _strip(bl.L, kids)
        full.e12 = 1
        kids = [full]
    root = Node(M.ROOT, bl.W, name=f'{index + 1:05d}', e12=1,
                children=_compress_copies(kids), root_info=[0, '', 0, 3, index])
    pat = Pattern(root, bl.L, qty=bl.qty, params=[Param(*p) for p in DEFAULT_PARAMS])
    pat.stats = compute_stats(pat, st)
    return pat


def compute_stats(pat: Pattern, st: Settings) -> Stats:
    """Area statistics as CADmatic stores them (areas in mm^2)."""
    acc = {'cuts': 0, 'cut_len': 0.0, 'area': 0.0, 'parts': 0}

    def walk(n, xext, yext, along_x):
        for c in n.children:
            cx, cy = (c.size, yext) if along_x else (xext, c.size)
            if c.type == M.CUT:
                acc['cuts'] += 1
                acc['cut_len'] += yext if along_x else xext
            elif c.type == M.PART:
                acc['parts'] += 1
                acc['area'] += cx * cy
            walk(c, cx, cy, not along_x)

    L, W = pat.length, pat.root.size
    walk(pat.root, L, W, True)
    board_area = L * W
    kerf_area = acc['cut_len'] * st.kerf
    waste = board_area - acc['area'] - kerf_area
    return Stats(waste_area=round(waste, 4), board_area=board_area, kerf_area=round(kerf_area, 4),
                 n_parts=acc['parts'], parts_area=round(acc['area'], 4),
                 waste_pct=(waste + kerf_area) / board_area * 100.0,
                 n_cuts=acc['cuts'], cut_length=round(acc['cut_len'], 5))


def build_run(name: str, layouts: List[BoardLayout], st: Settings):
    """Returns (lc4_bytes, prv_bytes)."""
    pats = [build_pattern(bl, i, st) for i, bl in enumerate(layouts)]
    boards = [Board(bl.L, bl.W, bl.T) for bl in layouts]
    f = LC4File(name, boards, pats, trailer=RUN_TRAILER)
    prv = PRVFile(name, len(pats), len(boards), sum(p.qty for p in pats))
    return f.to_bytes(), prv.to_bytes()
