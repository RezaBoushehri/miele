"""The small .prv file CADmatic keeps next to every .LC4 run (run summary)."""
from dataclasses import dataclass
from typing import Optional
from .codec import Reader, Writer


@dataclass
class PRVFile:
    name: str
    n_patterns: int
    n_boards: int
    total_boards: int          # sum of the pattern quantities
    status: int = 1            # 0..7 in the samples, 1 in most of them
    generator: str = 'manually generated'
    i1: int = 0
    i2: int = 0
    e25: int = 0
    tail: tuple = (0, 0, 0)
    s1: Optional[str] = None
    s2: Optional[str] = None
    s3: Optional[str] = None

    @classmethod
    def parse(cls, data: bytes):
        r = Reader(data)
        assert r.bool()
        r.struct(6)
        name = r.string(); gen = r.string()
        npat = r.int(); nb = r.int(); i1 = r.int(); i2 = r.int()
        total = r.int(); status = r.int()
        e25 = r.enum(0x25)
        s1 = None if not r.bool() else ValueError('unexpected')
        s2 = None if not r.bool() else ValueError('unexpected')
        s3 = None if not r.bool() else ValueError('unexpected')
        tail = (r.int(), r.int(), r.int())
        assert r.eof()
        return cls(name, npat, nb, total, status, gen, i1, i2, e25, tail)

    def to_bytes(self):
        w = Writer()
        w.bool(True); w.struct(6)
        w.string(self.name); w.string(self.generator)
        w.int(self.n_patterns); w.int(self.n_boards); w.int(self.i1); w.int(self.i2)
        w.int(self.total_boards); w.int(self.status)
        w.enum(0x25, self.e25)
        w.bool(False); w.bool(False); w.bool(False)
        for v in self.tail:
            w.int(v)
        return w.data()
