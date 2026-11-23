"""Low-level reader/writer for the HOMAG CADmatic LC4 / PRV binary stream.

The stream is a sequence of tagged values (little endian):
  01 00 <u8>        bool / "present" flag
  05 00 <u16>       struct header (class version)
  06 00 <i32>       int32
  08 00 <i64>       int64
  0a 00 <4 bytes>   4-byte value
  0b 00 <f64>       double
  0d 00             String  -> followed by bool + 0e 00 06 00 <i32 len> <bytes>
  0f 00             Decimal -> followed by 05 00 01 00 08 00 <i64 mantissa> (scale 1e-5)
  other 0x10..0x30  enum tag, followed by an int32
"""
import struct

SCALE = 100000  # decimals are stored as int64 * 1e-5


class Reader:
    def __init__(self, data: bytes):
        self.b = data
        self.i = 0

    def eof(self):
        return self.i >= len(self.b)

    def _tag(self, expect=None):
        t = struct.unpack_from('<H', self.b, self.i)[0]
        if expect is not None and t != expect:
            raise ValueError(f'offset {self.i:#x}: expected tag {expect:#x}, got {t:#x}')
        self.i += 2
        return t

    def peek(self):
        return struct.unpack_from('<H', self.b, self.i)[0]

    def bool(self):
        self._tag(1)
        v = self.b[self.i]
        self.i += 1
        return bool(v)

    def struct(self, version=None):
        self._tag(5)
        v = struct.unpack_from('<H', self.b, self.i)[0]
        self.i += 2
        if version is not None and v != version:
            raise ValueError(f'offset {self.i-4:#x}: expected struct v{version}, got v{v}')
        return v

    def int(self):
        self._tag(6)
        v = struct.unpack_from('<i', self.b, self.i)[0]
        self.i += 4
        return v

    def int64(self):
        self._tag(8)
        v = struct.unpack_from('<q', self.b, self.i)[0]
        self.i += 8
        return v

    def raw4(self):
        self._tag(0x0a)
        v = self.b[self.i:self.i + 4]
        self.i += 4
        return v

    def double(self):
        self._tag(0x0b)
        v = struct.unpack_from('<d', self.b, self.i)[0]
        self.i += 8
        return v

    def string(self):
        """String object; returns None for a null string."""
        self._tag(0x0d)
        if not self.bool():
            return None
        self._tag(0x0e)
        n = self.int()
        s = self.b[self.i:self.i + n]
        self.i += n
        return s.decode('cp1252')

    def decimal(self):
        self._tag(0x0f)
        self.struct(1)
        return self.int64() / SCALE

    def enum(self, tag):
        self._tag(tag)
        return self.int()


class Writer:
    def __init__(self):
        self.parts = []

    def data(self):
        return b''.join(self.parts)

    def _tag(self, t):
        self.parts.append(struct.pack('<H', t))

    def bool(self, v):
        self._tag(1)
        self.parts.append(b'\x01' if v else b'\x00')

    def struct(self, version):
        self._tag(5)
        self.parts.append(struct.pack('<H', version))

    def int(self, v):
        self._tag(6)
        self.parts.append(struct.pack('<i', int(v)))

    def int64(self, v):
        self._tag(8)
        self.parts.append(struct.pack('<q', int(v)))

    def raw4(self, v: bytes):
        self._tag(0x0a)
        self.parts.append(bytes(v))

    def double(self, v):
        self._tag(0x0b)
        self.parts.append(struct.pack('<d', float(v)))

    def string(self, s):
        self._tag(0x0d)
        if s is None:
            self.bool(False)
            return
        self.bool(True)
        self._tag(0x0e)
        raw = s.encode('cp1252', errors='replace')
        self.int(len(raw))
        self.parts.append(raw)

    def decimal(self, v):
        self._tag(0x0f)
        self.struct(1)
        self.int64(round(v * SCALE))

    def enum(self, tag, v):
        self._tag(tag)
        self.int(v)
