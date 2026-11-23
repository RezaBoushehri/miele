"""Structured model of an LC4 cutting-pattern file (HOMAG CADmatic 4.5).

Reverse engineered from files saved by a SAWTEQ S-200 (pattern entry, "manually generated").
Every field the format contains is kept so that parse -> write is byte identical.
"""
from dataclasses import dataclass, field
from typing import List, Optional
from .codec import Reader, Writer

# Node types (enum tag 0x11)
ROOT, CUT, STRIP, TRIM, PART = 1, 2, 3, 4, 11
NODE_NAMES = {ROOT: None, CUT: 'Schnitt', STRIP: 'Streifen', TRIM: 'Anschnitt', PART: 'Teil'}


def _list_header(r: Reader):
    if not r.bool():
        return None
    r.struct(1)
    return r.int()


def _w_list_header(w: Writer, n):
    if n is None:
        w.bool(False)
        return
    w.bool(True)
    w.struct(1)
    w.int(n)


@dataclass
class Board:
    length: float
    width: float
    thickness: float
    qty: int = 1
    s1: Optional[str] = ''
    s2: Optional[str] = ''
    s3: Optional[str] = ''
    i1: int = 0
    e13: int = 0
    raw_a: bytes = b'\0\0\0\0'
    raw_b: bytes = b'\0\0\0\0'
    s4: Optional[str] = ''
    s5: Optional[str] = ''
    e30: int = 0
    inner: int = 0
    flags: Optional[list] = None

    @classmethod
    def read(cls, r: Reader):
        fl = []
        assert r.bool()
        r.struct(1)
        r.struct(4)
        s1, s2, s3 = r.string(), r.string(), r.string()
        L = r.decimal(); fl.append(r.bool())
        W = r.decimal(); fl.append(r.bool())
        T = r.decimal(); fl.append(r.bool())
        qty = r.int(); i1 = r.int(); e13 = r.enum(0x13)
        fl.append(r.bool())
        assert r.bool(); r.struct(1); inner = r.int()
        fl.append(r.bool())
        a = r.raw4(); s4 = r.string(); s5 = r.string(); b = r.raw4()
        e30 = r.enum(0x30)
        return cls(L, W, T, qty, s1, s2, s3, i1, e13, a, b, s4, s5, e30, inner, fl)

    def write(self, w: Writer):
        fl = list(self.flags) if self.flags else [False] * 5
        w.bool(True); w.struct(1); w.struct(4)
        w.string(self.s1); w.string(self.s2); w.string(self.s3)
        w.decimal(self.length); w.bool(fl.pop(0))
        w.decimal(self.width); w.bool(fl.pop(0))
        w.decimal(self.thickness); w.bool(fl.pop(0))
        w.int(self.qty); w.int(self.i1); w.enum(0x13, self.e13)
        w.bool(fl.pop(0))
        w.bool(True); w.struct(1); w.int(self.inner)
        w.bool(fl.pop(0))
        w.raw4(self.raw_a); w.string(self.s4); w.string(self.s5); w.raw4(self.raw_b)
        w.enum(0x30, self.e30)


@dataclass
class Node:
    type: int
    size: float
    qty: int = 1
    name: Optional[str] = None
    label: Optional[str] = ''
    e12: Optional[int] = None
    children: List['Node'] = field(default_factory=list)
    # pattern-root only data (S3 item)
    root_info: Optional[list] = None
    e26: int = 0
    i26: int = 0
    tail_i1: int = -1
    tail_i2: int = -1
    tail_s1: Optional[str] = ''
    tail_i3: int = -1
    tail_s2: Optional[str] = '-1'
    flags: Optional[list] = None

    def __post_init__(self):
        if self.name is None:
            self.name = NODE_NAMES.get(self.type) or ''
        if self.e12 is None:
            self.e12 = 4 if self.type in (CUT, TRIM) else 0

    @classmethod
    def read(cls, r: Reader, is_root=False):
        fl = []
        assert r.bool()
        r.struct(9)
        if is_root:
            r.struct(9)
        name = r.string(); label = r.string()
        size = r.decimal(); fl.append(r.bool())
        qty = r.int(); typ = r.enum(0x11); e12 = r.enum(0x12)
        r.struct(1); n1 = r.int()
        root_info = None
        if n1:
            assert n1 == 1, n1
            assert r.bool(); r.struct(3)
            a = r.int(); fl.append(r.bool()); s = r.string(); fl.append(r.bool())
            b = r.int(); assert r.bool(); e14 = r.enum(0x14); c = r.int()
            root_info = [a, s, b, e14, c]
        n2 = _list_header(r); assert n2 == 0, n2
        nc = _list_header(r)
        children = [Node.read(r) for _ in range(nc)]
        fl.append(r.bool())
        e26 = r.enum(0x26); i26 = r.int()
        fl.append(r.bool())
        t1 = r.int(); t2 = r.int()
        assert _list_header(r) == 0
        assert _list_header(r) == 0
        ts1 = r.string(); ti3 = r.int(); ts2 = r.string()
        return cls(typ, size, qty, name, label, e12, children, root_info,
                   e26, i26, t1, t2, ts1, ti3, ts2, fl)

    def write(self, w: Writer, is_root=False):
        fl = list(self.flags) if self.flags else [False] * (5 if self.root_info else 3)
        w.bool(True); w.struct(9)
        if is_root:
            w.struct(9)
        w.string(self.name); w.string(self.label)
        w.decimal(self.size); w.bool(fl.pop(0))
        w.int(self.qty); w.enum(0x11, self.type); w.enum(0x12, self.e12)
        w.struct(1)
        if self.root_info:
            a, s, b, e14, c = self.root_info
            w.int(1)
            w.bool(True); w.struct(3)
            w.int(a); w.bool(fl.pop(0)); w.string(s); w.bool(fl.pop(0))
            w.int(b); w.bool(True); w.enum(0x14, e14); w.int(c)
        else:
            w.int(0)
        _w_list_header(w, 0)
        _w_list_header(w, len(self.children))
        for c in self.children:
            c.write(w)
        w.bool(fl.pop(0))
        w.enum(0x26, self.e26); w.int(self.i26)
        w.bool(fl.pop(0))
        w.int(self.tail_i1); w.int(self.tail_i2)
        _w_list_header(w, 0); _w_list_header(w, 0)
        w.string(self.tail_s1); w.int(self.tail_i3); w.string(self.tail_s2)


@dataclass
class Param:
    name: str
    value: str
    flag: bool = False


@dataclass
class Stats:
    i0: int = 1
    waste_area: float = 0.0
    board_area: float = 0.0
    n_rest: int = 0
    rest_area: float = 0.0
    kerf_area: float = 0.0
    n_parts: int = 0
    parts_area: float = 0.0
    waste_pct: float = 0.0
    e27: int = 0
    ints: tuple = (0, 0, 0, 0)
    n_cuts: int = 0
    cut_length: float = 0.0
    j1: int = 0
    j2: int = 4
    s1: Optional[str] = None
    j3: int = 0
    j4: int = 0
    s2: Optional[str] = None


@dataclass
class Pattern:
    root: Node
    length: float
    qty: int = 1
    i2: int = 1
    s1: Optional[str] = ''
    e10: int = 0
    params: List[Param] = field(default_factory=list)
    stats: Stats = field(default_factory=Stats)
    flags: Optional[list] = None

    @classmethod
    def read(cls, r: Reader):
        fl = []
        root = Node.read(r, is_root=True)
        length = r.decimal(); fl.append(r.bool())
        qty = r.int(); i2 = r.int(); s1 = r.string()
        r.struct(1); e10 = r.enum(0x10); fl.append(r.bool())
        n = _list_header(r)
        params = []
        for _ in range(n):
            assert r.bool(); r.struct(2)
            name = r.string(); value = r.string(); flag = r.bool()
            params.append(Param(name, value, flag))
        st = Stats()
        st.i0 = r.int(); fl.append(r.bool())
        st.waste_area = r.double(); st.board_area = r.double()
        st.n_rest = r.int(); st.rest_area = r.double(); st.kerf_area = r.double()
        st.n_parts = r.int(); st.parts_area = r.double(); st.waste_pct = r.double()
        st.e27 = r.enum(0x27)
        st.ints = tuple(r.int() for _ in range(4))
        st.n_cuts = r.int(); st.cut_length = r.decimal(); fl.append(r.bool())
        st.j1 = r.int(); st.j2 = r.int(); st.s1 = r.string(); st.j3 = r.int()
        fl.append(r.bool())
        st.j4 = r.int(); st.s2 = r.string()
        return cls(root, length, qty, i2, s1, e10, params, st, fl)

    def write(self, w: Writer):
        fl = list(self.flags) if self.flags else [False] * 5
        self.root.write(w, is_root=True)
        w.decimal(self.length); w.bool(fl.pop(0))
        w.int(self.qty); w.int(self.i2); w.string(self.s1)
        w.struct(1); w.enum(0x10, self.e10); w.bool(fl.pop(0))
        _w_list_header(w, len(self.params))
        for p in self.params:
            w.bool(True); w.struct(2)
            w.string(p.name); w.string(p.value); w.bool(p.flag)
        st = self.stats
        w.int(st.i0); w.bool(fl.pop(0))
        w.double(st.waste_area); w.double(st.board_area)
        w.int(st.n_rest); w.double(st.rest_area); w.double(st.kerf_area)
        w.int(st.n_parts); w.double(st.parts_area); w.double(st.waste_pct)
        w.enum(0x27, st.e27)
        for v in st.ints:
            w.int(v)
        w.int(st.n_cuts); w.decimal(st.cut_length); w.bool(fl.pop(0))
        w.int(st.j1); w.int(st.j2); w.string(st.s1); w.int(st.j3)
        w.bool(fl.pop(0))
        w.int(st.j4); w.string(st.s2)


@dataclass
class LC4File:
    name: str
    boards: List[Board] = field(default_factory=list)
    patterns: List[Pattern] = field(default_factory=list)
    generator: str = 'manually generated'
    trailer: bytes = b''

    @classmethod
    def parse(cls, data: bytes):
        r = Reader(data)
        assert r.bool(); r.struct(12)
        name = r.string(); gen = r.string()
        assert _list_header(r) == 0
        nb = _list_header(r)
        boards = [Board.read(r) for _ in range(nb)]
        assert _list_header(r) == 0
        np_ = _list_header(r)
        patterns = [Pattern.read(r) for _ in range(np_)]
        trailer = data[r.i:]
        return cls(name, boards, patterns, gen, trailer)

    def to_bytes(self, trailer=None):
        w = Writer()
        w.bool(True); w.struct(12)
        w.string(self.name); w.string(self.generator)
        _w_list_header(w, 0)
        _w_list_header(w, len(self.boards))
        for b in self.boards:
            b.write(w)
        _w_list_header(w, 0)
        _w_list_header(w, len(self.patterns))
        for p in self.patterns:
            p.write(w)
        w.parts.append(self.trailer if trailer is None else trailer)
        return w.data()
