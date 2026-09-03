"""Read compiled Lua out of the client's .lub files.

The client keeps its own tables as compiled Lua in the GRF, and for several of
them that is the only place the mapping exists: a unit's job id to its sprite,
a head gear view id to its sprite, an item id to the weapon class it is drawn
as. None of it is in the protocol.

Both bytecode versions the archive ships are read. The older `lua files`
folder is Lua 5.0; `luafiles514` is Lua 5.1, and the two differ in more than a
version byte — the header carries different fields, a function's parts are
dumped in a different order, and the instruction word packs its operands
differently. A chunk read with the wrong layout does not fail, it yields
nonsense, so the version is carried on every prototype and every decode is
told which one it is looking at.

Only what these files use is handled: nil, boolean, number and string
constants, and nested prototypes.
"""

import struct

SIGNATURE = b"\033Lua"
VERSION_50 = 0x50
VERSION_51 = 0x51

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
        # 5.1 chunks say whether lua_Number is an integer type. The client's
        # are not, but a chunk that says so is read as it asks.
        self.integral = False

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
        if self.integral:
            return float(self._uint(self.number_size))
        if self.number_size != 8:
            raise ValueError(f"unsupported lua_Number width {self.number_size}")
        fmt = "<d" if self.little_endian else ">d"
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
    """Read the chunk header and return which version it declares.

    The two versions declare themselves differently past the version byte. 5.0
    states the width of each instruction field and then writes a sample number
    so a reader can check it decodes; 5.1 dropped both, and added a format byte
    and a flag saying whether its numbers are integers.
    """
    if r.take(4) != SIGNATURE:
        raise ValueError("not a Lua binary chunk")

    version = r.byte()
    if version not in (VERSION_50, VERSION_51):
        raise ValueError(f"unsupported Lua bytecode version 0x{version:02x}")

    if version == VERSION_51:
        format_id = r.byte()
        if format_id != 0:
            raise ValueError(f"unsupported chunk format {format_id}")

    r.little_endian = r.byte() == 1
    r.int_size = r.byte()
    r.size_t_size = r.byte()
    r.instruction_size = r.byte()

    if version == VERSION_50:
        r.byte()  # SIZE_OP
        r.byte()  # SIZE_A
        r.byte()  # SIZE_B
        r.byte()  # SIZE_C
        r.number_size = r.byte()
        # TEST_NUMBER, written so a reader can check its format.
        r.take(r.number_size)
    else:
        r.number_size = r.byte()
        r.integral = r.byte() == 1

    return version


class Proto:
    """One compiled function: its constant pool and its instructions.

    The version is carried here because decoding an instruction needs it, and
    by the time one is being decoded the header is long out of sight.
    """

    def __init__(self, version):
        self.version = version
        self.constants = []
        self.code = []


def read_constants(r, proto):
    """The constant pool of one prototype. Both versions write it the same."""
    for _ in range(r.integer()):
        tag = r.byte()
        if tag == LUA_TNUMBER:
            proto.constants.append(r.number())
        elif tag == LUA_TSTRING:
            value = r.string()
            proto.constants.append(decode_string(value) if value is not None else None)
        elif tag == LUA_TBOOLEAN:
            proto.constants.append(bool(r.byte()))
        elif tag == LUA_TNIL:
            proto.constants.append(None)
        else:
            raise ValueError(f"unknown constant tag {tag} at offset {r.pos}")


def read_code(r, proto):
    """The instructions of one prototype."""
    for _ in range(r.integer()):
        proto.code.append(r._uint(r.instruction_size))


def read_function(r, protos, version):
    """Walk one prototype and everything nested inside it.

    The two versions dump the same parts in a different order. 5.0 writes the
    debug tables first and the code last; 5.1 writes the code first and the
    debug tables last. Reading one order with the other's reader does not fail
    at the first wrong field — it reads a length out of the middle of something
    else and then walks off the end of the chunk, so the order is the whole of
    what this branch is for.
    """
    proto = Proto(version)
    protos.append(proto)

    r.string()  # source name
    r.integer()  # line defined
    if version == VERSION_51:
        r.integer()  # last line defined
    r.byte()  # nups
    r.byte()  # numparams
    r.byte()  # is_vararg
    r.byte()  # maxstacksize

    if version == VERSION_50:
        r.take(r.integer() * r.int_size)  # line info

        for _ in range(r.integer()):  # locals
            r.string()
            r.integer()  # start pc
            r.integer()  # end pc

        for _ in range(r.integer()):  # upvalues
            r.string()

        read_constants(r, proto)

        for _ in range(r.integer()):  # nested prototypes
            read_function(r, protos, version)

        read_code(r, proto)

        return

    read_code(r, proto)
    read_constants(r, proto)

    for _ in range(r.integer()):  # nested prototypes
        read_function(r, protos, version)

    r.take(r.integer() * r.int_size)  # line info

    for _ in range(r.integer()):  # locals
        r.string()
        r.integer()  # start pc
        r.integer()  # end pc

    for _ in range(r.integer()):  # upvalues
        r.string()


def load(path):
    """Every prototype in the chunk, outermost first."""
    with open(path, "rb") as handle:
        r = Reader(handle.read())
    version = read_header(r)
    protos = []
    read_function(r, protos, version)
    return protos


def constants(path):
    """Every constant in the chunk, in the order the source used them."""
    out = []
    for proto in load(path):
        out.extend(proto.constants)
    return out


# Instruction layout, from each version's lopcodes.h. The field order differs:
# 5.0 packs C, then B, then A; 5.1 swapped A to the front and narrowed nothing
# else. Decoding 5.0 bytecode with the 5.1 layout yields register numbers in
# the hundreds, which is the symptom to look for.
#
#   5.0   POS_OP = 0, POS_C = 6,  POS_B = 15, POS_A = 24, POS_Bx = POS_C
#   5.1   POS_OP = 0, POS_A = 6,  POS_C = 14, POS_B = 23, POS_Bx = POS_C
#
# The two also mark an operand as a constant differently. 5.0 does it by size:
# anything at or above MAXSTACK is a constant index offset by MAXSTACK. 5.1
# replaced that with a bit flag, BITRK, in the operand's top bit.
MAXSTACK = 250
BITRK = 1 << 8

# The opcodes read here happen to have the same numbers in both versions; the
# lists only diverge further down than anything below.
OP_LOADK = 1
OP_GETGLOBAL = 5
OP_GETTABLE = 6
OP_SETTABLE = 9
OP_NEWTABLE = 10


def decode(instruction, version=VERSION_50):
    """Split an instruction into (opcode, A, B, C)."""
    op = instruction & 0x3F

    if version == VERSION_51:
        a = (instruction >> 6) & 0xFF
        c = (instruction >> 14) & 0x1FF
        b = (instruction >> 23) & 0x1FF
    else:
        c = (instruction >> 6) & 0x1FF
        b = (instruction >> 15) & 0x1FF
        a = (instruction >> 24) & 0xFF

    return op, a, b, c


def bx(instruction, version=VERSION_50):
    """The 18-bit operand used by instructions that index the constant pool."""
    shift = 14 if version == VERSION_51 else 6

    return (instruction >> shift) & 0x3FFFF


def is_constant(operand, version=VERSION_50):
    if version == VERSION_51:
        return bool(operand & BITRK)

    return operand >= MAXSTACK


def constant_index(operand, version=VERSION_50):
    if version == VERSION_51:
        return operand & ~BITRK

    return operand - MAXSTACK
