#!/usr/bin/env python3
"""Generate the weapon sprite tables from the client's own Lua tables.

The server never says which sprite a weapon is drawn as. What arrives in
ZC_SPRITE_CHANGE is whatever rAthena's map_session_data::update_look put
there, which is the item's view id if its database row has one and the item's
own id if it does not — and in the renewal item database exactly one weapon of
2806 has a view id, so in practice it is always the item id. The character list
at login carries something else again: the weapon *class*, straight out of the
char table. Two encodings of the same fact, and neither can be turned into a
file name without a table.

The client ships both halves of that table, and this reads them:

    weapontable.lub   Weapon_IDs      = { WEAPONTYPE_SHORTSWORD = 1, ... }
                      WeaponNameTable = { [Weapon_IDs.WEAPONTYPE_X] = "_단검" }
    iteminfo.lub      tbl = { [1201] = { ..., ClassNum = 1 } }

WeaponNameTable is keyed by class number and holds the suffix the sprite is
filed under: the base classes 1..23, the six dual-wield combinations 25..30,
and from 31 up the weapons with art of their own, whose suffix is their item
id. ClassNum is what turns an item id into one of those numbers.

ClassNum is not weapons-only: it is the sprite number for whatever slot an item
is worn in, so a shield and a dagger both have ClassNum 1 in namespaces of
their own. The table is emitted whole and read only against the slot the look
arrived for, which is the only reading that is meaningful.

Usage:
    genweapons.py weapons <weapontable.lub> > weaponnames.go
    genweapons.py items   <iteminfo.lub>    > itemclass.go
"""

import sys

import lub

# Opcodes beyond the ones lub names, all of them numbered the same in 5.0 and
# 5.1: a table constructor's parts, and the globals it is finally stored in.
OP_MOVE = 0
OP_LOADNIL = 3
OP_GETGLOBAL = 5
OP_SETGLOBAL = 7


def read_globals(path):
    """Every global table the chunk assigns, as {name: {key: value}}.

    The tables are built in a register and stored into a global at the end, so
    a SETTABLE says nothing about which table it is filling until the SETGLOBAL
    that closes it. Following that is what tells the sprite names apart from
    the hit sounds, which are the same shape over the same keys in the same
    file.

    A key written as `Weapon_IDs.WEAPONTYPE_X` compiles to a GETTABLE against
    an earlier global, so the tables already read are kept to resolve it.
    """
    tables = {}

    for proto in lub.load(path):
        version = proto.version
        registers, building = {}, {}

        for instruction in proto.code:
            op, a, b, c = lub.decode(instruction, version)

            if op == lub.OP_LOADK:
                registers[a] = proto.constants[lub.bx(instruction, version)]
            elif op == OP_GETGLOBAL:
                registers[a] = Global(proto.constants[lub.bx(instruction, version)])
            elif op == lub.OP_NEWTABLE:
                registers[a] = None
                building[a] = {}
            elif op == lub.OP_GETTABLE:
                base = registers.get(b)
                key = operand(proto, registers, c)
                registers[a] = (
                    tables[base.name].get(key)
                    if isinstance(base, Global) and base.name in tables
                    else None
                )
            elif op == lub.OP_SETTABLE:
                key = operand(proto, registers, b)
                if a in building and key is not None:
                    building[a][key] = operand(proto, registers, c)
            elif op == OP_SETGLOBAL:
                name = proto.constants[lub.bx(instruction, version)]
                tables[name] = building.pop(a, {})

    return tables


class Global:
    """A register holding a global, so a GETTABLE against it can be followed."""

    def __init__(self, name):
        self.name = name


def operand(proto, registers, value):
    """The value an operand refers to: a constant, or whatever a register holds."""
    if lub.is_constant(value, proto.version):
        return proto.constants[lub.constant_index(value, proto.version)]

    return registers.get(value)


def read_records(path):
    """Every `[id] = { field = value }` record in the chunk, as {id: {field: value}}.

    Each record is built in a register of its own and then stored into the
    outer table under its id, so a record is finished by the SETTABLE whose key
    is a number. Registers are reused between records, which is why every write
    to one drops what it held: a register that was a table and is then loaded
    with a string is no longer that table, and treating it as one attaches the
    next record's fields to the last record's id.
    """
    records = {}

    for proto in lub.load(path):
        version = proto.version
        registers, building = {}, {}

        def clear(register):
            registers.pop(register, None)
            building.pop(register, None)

        for instruction in proto.code:
            op, a, b, c = lub.decode(instruction, version)

            if op == lub.OP_LOADK:
                clear(a)
                registers[a] = proto.constants[lub.bx(instruction, version)]
            elif op == lub.OP_NEWTABLE:
                clear(a)
                building[a] = {}
            elif op == OP_MOVE:
                clear(a)
                if b in building:
                    building[a] = building[b]
                elif b in registers:
                    registers[a] = registers[b]
            elif op == OP_LOADNIL:
                for register in range(a, b + 1):
                    clear(register)
            elif op == lub.OP_SETTABLE:
                key = operand(proto, registers, b)
                value = (
                    building[c]
                    if not lub.is_constant(c, version) and c in building
                    else operand(proto, registers, c)
                )

                if isinstance(key, str) and a in building:
                    building[a][key] = value
                elif isinstance(key, float) and isinstance(value, dict):
                    records[int(key)] = value

    return records


def go_string(value):
    escaped = value.replace("\\", "\\\\").replace('"', '\\"')

    return f'"{escaped}"'


def emit(header, declaration, rows):
    print("\n".join(header))
    print(declaration)
    for row in rows:
        print(f"\t{row}")
    print("}")


def weapons(path):
    """class number -> the suffix its sprite is filed under."""
    tables = read_globals(path)

    names = {
        int(key): value
        for key, value in tables.get("WeaponNameTable", {}).items()
        if isinstance(key, float) and isinstance(value, str) and value
    }
    if not names:
        print(f"{path}: no WeaponNameTable", file=sys.stderr)
        return 1

    emit(
        [
            "// Code generated by tools/spritenames/genweapons.py. DO NOT EDIT.",
            "//",
            "// Source: the client's own weapontable.lub, extracted from the GRF.",
            "// Regenerate after a client data update.",
            "",
            "package charsprite",
            "",
            "// weaponSpriteNames maps a weapon class number to the suffix the sprite",
            "// is filed under, leading underscore and all.",
            "//",
            "// 1 to 23 are the classes rAthena's weapon_type enum numbers the same way,",
            "// 25 to 30 the six dual-wield combinations, and 31 up the weapons that have",
            "// art of their own, whose suffix is their item id. 24 is the enum's own end",
            "// marker and names nothing.",
        ],
        "var weaponSpriteNames = map[int]string{",
        [f"{key}: {go_string(names[key])}," for key in sorted(names)],
    )

    print(f"// {len(names)} weapon classes", file=sys.stderr)

    return 0


def items(path):
    """item id -> the sprite class number the client draws it as."""
    records = read_records(path)

    classes = {
        item: int(record["ClassNum"])
        for item, record in records.items()
        if isinstance(record.get("ClassNum"), float) and record["ClassNum"] != 0
    }
    if not classes:
        print(f"{path}: no item carried a ClassNum", file=sys.stderr)
        return 1

    emit(
        [
            "// Code generated by tools/spritenames/genweapons.py. DO NOT EDIT.",
            "//",
            "// Source: the client's own iteminfo.lub, extracted from the GRF.",
            "// Regenerate after a client data update.",
            "",
            "package charsprite",
            "",
            "// itemSpriteClass maps an item id to the sprite class number the client",
            "// draws it as — the item's ClassNum.",
            "//",
            "// The number means something different depending on where the item is worn:",
            "// a shield and a dagger are both class 1, in namespaces of their own. Look",
            "// one up only against the slot the look arrived for.",
            "//",
            "// Items the client draws no differently for are left out, which is every",
            "// item whose ClassNum is zero.",
        ],
        "var itemSpriteClass = map[int]int{",
        [f"{item}: {classes[item]}," for item in sorted(classes)],
    )

    print(
        f"// {len(classes)} items with a sprite class, of {len(records)} read",
        file=sys.stderr,
    )

    return 0


def main():
    if len(sys.argv) != 3 or sys.argv[1] not in ("weapons", "items"):
        print(__doc__, file=sys.stderr)
        return 2

    return weapons(sys.argv[2]) if sys.argv[1] == "weapons" else items(sys.argv[2])


if __name__ == "__main__":
    sys.exit(main())
