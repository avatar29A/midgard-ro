"""Read the constant table out of a Lua 5.0 binary chunk.

The client keeps its sprite name tables as compiled Lua in the GRF, and they
are the only place the mapping from a unit's job id to its sprite file exists.
Running the bytecode is not needed to recover them: both tables are written as
a flat sequence of constants, so reading the constant pool in order gives back
the pairs the source assigned.

Format is Lua 5.0's lundump.c. Only what those two files use is handled —
nil, boolean, number and string constants, and nested prototypes.
"""

import struct

SIGNATURE = b"\033Lua"
VERSION_50 = 0x50

LUA_TNIL = 0
LUA_TBOOLEAN = 1
LUA_TNUMBER = 3
LUA_TSTRING = 4


class Reader:
    """Sequential reader over a binary chunk, sized by the chunk's own header."""

    def __init__(self, data):
        self.data = data
        self.pos = 0
        # Filled in by read_header; the header declares its own widths so a
        # chunk compiled on another machine still reads correctly.
        self.int_size = 4
        self.size_t_size = 4
        self.instruction_size = 4
        self.number_size = 8
        self.little_endian = True

    def take(self, n):
        if self.pos + n > len(self.data):
            raise ValueError(f"chunk ends mid-value at offset {self.pos}")
        chunk = self.data[self.pos:self.pos + n]
        self.pos += n
        return chunk

    def byte(self):
        return self.take(1)[0]

    def _uint(self, width):
        return int.from_bytes(self.take(width), "little" if self.little_endian else "big")

    def integer(self):
        return self._uint(self.int_size)

    def size(self):
        return self._uint(self.size_t_size)

    def number(self):
        fmt = "<d" if self.little_endian else ">d"
        if self.number_size != 8:
            raise ValueError(f"unsupported lua_Number width {self.number_size}")
        return struct.unpack(fmt, self.take(8))[0]

    def string(self):
        """A dumped string is a size followed by its bytes, including the
        trailing NUL. Size zero means the string was absent, not empty."""
        n = self.size()
        if n == 0:
            return None
        raw = self.take(n)
        return raw[:-1] if raw.endswith(b"\0") else raw


def decode_string(raw):
    """Decode a string constant.

    The client's data is Korean, encoded EUC-KR (as cp949, its superset).
    Sprite names are usually ASCII, which cp949 passes through unchanged, but
    a few are Korean and some carry a subdirectory, so decoding as UTF-8 turns
    them into replacement characters and loses the name entirely.
    """
    try:
        return raw.decode("cp949")
    except UnicodeDecodeError:
        return raw.decode("latin-1")


def read_header(r):
    if r.take(4) != SIGNATURE:
        raise ValueError("not a Lua binary chunk")

    version = r.byte()
    if version != VERSION_50:
        raise ValueError(f"unsupported Lua bytecode version 0x{version:02x}")

    r.little_endian = r.byte() == 1
    r.int_size = r.byte()
    r.size_t_size = r.byte()
    r.instruction_size = r.byte()
    r.byte()  # SIZE_OP
    r.byte()  # SIZE_A
    r.byte()  # SIZE_B
    r.byte()  # SIZE_C
    r.number_size = r.byte()
    r.take(r.number_size)  # TEST_NUMBER, written so a reader can check its format


class Proto:
    """One compiled function: its constant pool and its instructions."""

    def __init__(self):
        self.constants = []
        self.code = []


def read_function(r, protos):
    """Walk one prototype and everything nested inside it."""
    proto = Proto()
    protos.append(proto)
    out = proto.constants

    r.string()  # source name
    r.integer()  # line defined
    r.byte()  # nups
    r.byte()  # numparams
    r.byte()  # is_vararg
    r.byte()  # maxstacksize

    # Line info
    r.take(r.integer() * r.int_size)

    # Locals
    for _ in range(r.integer()):
        r.string()
        r.integer()  # start pc
        r.integer()  # end pc

    # Upvalues
    for _ in range(r.integer()):
        r.string()

    # Constants
    for _ in range(r.integer()):
        tag = r.byte()
        if tag == LUA_TNUMBER:
            out.append(r.number())
        elif tag == LUA_TSTRING:
            value = r.string()
            out.append(decode_string(value) if value is not None else None)
        elif tag == LUA_TBOOLEAN:
            out.append(bool(r.byte()))
        elif tag == LUA_TNIL:
            out.append(None)
        else:
            raise ValueError(f"unknown constant tag {tag} at offset {r.pos}")

    # Nested prototypes, then this function's own code.
    for _ in range(r.integer()):
        read_function(r, protos)
    for _ in range(r.integer()):
        proto.code.append(r._uint(r.instruction_size))


def load(path):
    """Every prototype in the chunk, outermost first."""
    with open(path, "rb") as handle:
        r = Reader(handle.read())
    read_header(r)
    protos = []
    read_function(r, protos)
    return protos


def constants(path):
    """Every constant in the chunk, in the order the source used them."""
    out = []
    for proto in load(path):
        out.extend(proto.constants)
    return out


# Instruction layout, from Lua 5.0's lopcodes.h. Note the field order: 5.0
# packs C, then B, then A, and 5.1 later swapped A to the front. Decoding 5.0
# bytecode with the 5.1 layout yields register numbers in the hundreds, which
# is the symptom to look for.
#
#   POS_OP = 0, POS_C = 6, POS_B = 15, POS_A = 24, POS_Bx = POS_C
#
# 5.0 also marks an operand as a constant by size rather than by a flag bit:
# anything at or above MAXSTACK is a constant index offset by MAXSTACK. 5.1
# replaced that with a bit flag, so this too is version specific.
MAXSTACK = 250

OP_LOADK = 1
OP_GETGLOBAL = 5
OP_GETTABLE = 6
OP_SETTABLE = 9


def decode(instruction):
    """Split an instruction into (opcode, A, B, C)."""
    op = instruction & 0x3F
    c = (instruction >> 6) & 0x1FF
    b = (instruction >> 15) & 0x1FF
    a = (instruction >> 24) & 0xFF
    return op, a, b, c


def bx(instruction):
    """The 18-bit operand used by instructions that index the constant pool."""
    return (instruction >> 6) & 0x3FFFF


def is_constant(operand):
    return operand >= MAXSTACK


def constant_index(operand):
    return operand - MAXSTACK
