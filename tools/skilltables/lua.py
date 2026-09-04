"""Reconstruct the tables a compiled Lua chunk builds.

The sprite tables the client ships are flat — a name against a number — and
reading them needs nothing more than following LOADK and SETTABLE. The skill
tables are not: a skill's entry is a table of its own inside another table, and
its fields are sometimes arrays, which the compiler writes with an instruction
the flat reader ignores entirely.

So this runs the parts of the bytecode that build tables, and hands back what
they built. Only the instructions a data file uses are implemented: loading
constants, making tables, reading and writing their fields, and the array
writes. Anything else leaves a register unknown, which reads back as None
rather than as a wrong value.

Both bytecode versions the archive ships are run. They agree on the numbers of
every instruction here but one — the array write — and on that one they
disagree over its number, its block size and how a block is addressed.
"""

import lub

# Opcodes, numbered the same in both versions for everything below.
OP_MOVE = 0
OP_LOADK = 1
OP_LOADBOOL = 2
OP_LOADNIL = 3
OP_GETGLOBAL = 5
OP_GETTABLE = 6
OP_SETGLOBAL = 7
OP_SETTABLE = 9
OP_NEWTABLE = 10

# SETLIST is the one instruction read here that the two versions do not agree
# on, and they disagree three ways over: its number, how many elements a block
# carries, and how the block is addressed.
#
# 5.1 numbers it 34 and gives it B elements with C numbering the block; 5.0
# numbers it 31 and packs both into Bx, where the low five bits are one less
# than the count and the rest is the index the block ends at. Read with the
# other version's rules it writes nothing at all, which is a table of strings
# that comes back empty.
OP_SETLIST_50 = 31
OP_SETLIST_51 = 34

# How many array elements one block carries, from each version's lopcodes.h.
FIELDS_PER_FLUSH_50 = 32
FIELDS_PER_FLUSH_51 = 50


class Table(dict):
    """A Lua table under construction.

    A dict, because every table in these files is read by key afterwards —
    the array part is stored under its integer indices, which is what Lua
    does anyway.
    """

    def array(self):
        """The array part, in order, stopping at the first gap."""
        out = []
        i = 1
        while i in self:
            out.append(self[i])
            i += 1

        return out


def run(path, seed=None):
    """Every global the chunk assigns, as {name: value}.

    `seed` carries globals defined in other files. These tables are written
    across several chunks that reference each other's — a skill's effect entry
    is keyed by `SKID.SM_BASH` and valued with `EFID.EF_BASH`, and neither of
    those globals exists in the file that uses them. Without the seed every key
    reads as nothing and the table comes back empty, which is exactly what it
    did.
    """
    globals_ = dict(seed or {})

    for proto in lub.load(path):
        version = proto.version
        registers = {}

        def value(operand):
            """What an operand refers to: a constant, or a register's contents."""
            if lub.is_constant(operand, version):
                return proto.constants[lub.constant_index(operand, version)]

            return registers.get(operand)

        for pc, instruction in enumerate(proto.code):
            op, a, b, c = lub.decode(instruction, version)

            if op == OP_LOADK:
                registers[a] = proto.constants[lub.bx(instruction, version)]
            elif op == OP_LOADBOOL:
                registers[a] = bool(b)
            elif op == OP_LOADNIL:
                for register in range(a, b + 1):
                    registers[register] = None
            elif op == OP_MOVE:
                registers[a] = registers.get(b)
            elif op == OP_NEWTABLE:
                registers[a] = Table()
            elif op == OP_GETGLOBAL:
                registers[a] = globals_.get(proto.constants[lub.bx(instruction, version)])
            elif op == OP_SETGLOBAL:
                globals_[proto.constants[lub.bx(instruction, version)]] = registers.get(a)
            elif op == OP_GETTABLE:
                table = registers.get(b)
                registers[a] = table.get(value(c)) if isinstance(table, dict) else None
            elif op == OP_SETTABLE:
                table = registers.get(a)
                if isinstance(table, dict):
                    key = value(b)
                    if key is not None:
                        table[key] = value(c)
            elif op == OP_SETLIST_51 and version == lub.VERSION_51:
                table = registers.get(a)
                if not isinstance(table, dict):
                    continue

                # B is how many values follow the table register; zero means
                # "everything up to the stack top", which a data file's fixed
                # list never uses. C numbers the block of fifty.
                block = (c - 1) if c > 0 else 0
                for i in range(1, b + 1):
                    table[block * FIELDS_PER_FLUSH_51 + i] = registers.get(a + i)
            elif op == OP_SETLIST_50 and version == lub.VERSION_50:
                table = registers.get(a)
                if not isinstance(table, dict):
                    continue

                # Bx holds both: the low bits are the count less one, and
                # clearing them leaves the index the block starts from.
                index = lub.bx(instruction, version)
                count = (index & (FIELDS_PER_FLUSH_50 - 1)) + 1
                base = index & ~(FIELDS_PER_FLUSH_50 - 1)

                for i in range(1, count + 1):
                    table[base + i] = registers.get(a + i)

    return globals_
