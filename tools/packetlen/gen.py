#!/usr/bin/env python3
"""Generate the server->client packet length table from rAthena's source.

RO's wire protocol has no framing markers: a packet is an id followed by a
payload whose length you are simply expected to know. Guess one length wrong
and every packet after it on that connection is garbage. Maintaining that
table by hand is how it drifts, so we lift it from the rAthena tree we build
the server from, resolving the #if PACKETVER guards for our packetver.

Usage:
    python3 tools/packetlen/gen.py <rathena-src-dir> [packetver] > \
        internal/network/packets/lengths.go
"""

import re
import sys

PACKET_RE = re.compile(r"^\s*packet\(\s*(0[xX][0-9A-Fa-f]{4})\s*,\s*(-?\d+)")
COND_RE = re.compile(r"^\s*#\s*(if|ifdef|ifndef|elif|else|endif)\b(.*)$")


def build_env(packetver: int) -> dict:
    """Mirror src/config/packets.hpp for the given packetver.

    Undefined identifiers evaluate to 0 in #if, which is what C does.
    """
    is_re = (20151104 < packetver < 20180704) or (20200902 <= packetver <= 20211118)
    env = {
        "PACKETVER": packetver,
        "PACKETVER_RE_NUM": packetver if is_re else 0,
        "PACKETVER_MAIN_NUM": 0 if is_re else packetver,
        "PACKETVER_ZERO_NUM": 0,
    }
    env["_defined"] = {
        "PACKETVER": True,
        "PACKETVER_RE": is_re,
        "PACKETVER_ZERO": False,
        "PACKETVER_MAIN": not is_re,
        # Build feature flags that gate packet structs. Values are whatever a
        # stock rAthena build has; a server built with custom flags would need
        # these changed to match, or its packets will be missing from the table.
        "HOTKEY_SAVING": True,  # src/common/mmo.hpp, unconditional
        "ENABLE_CASHSHOP_PREVIEW_PATCH": False,  # src/config/core.hpp, commented out
        "ENABLE_OLD_CASHSHOP_PREVIEW_PATCH": False,  # src/config/core.hpp, commented out
    }
    return env


def evaluate(expr: str, env: dict) -> bool:
    """Evaluate a C preprocessor conditional expression."""
    expr = expr.split("//")[0].strip()

    def defined(match):
        name = match.group(1)
        return "1" if env["_defined"].get(name, name in env and env[name]) else "0"

    expr = re.sub(r"defined\s*\(\s*(\w+)\s*\)", defined, expr)
    expr = re.sub(r"defined\s+(\w+)", defined, expr)

    # Substitute known macros; anything still unknown becomes 0, as in C.
    def ident(match):
        name = match.group(0)
        if name in env:
            return str(env[name])
        return "0"

    expr = re.sub(r"[A-Za-z_]\w*", ident, expr)
    expr = expr.replace("&&", " and ").replace("||", " or ").replace("!", " not ")
    # Re-repair != which the ! replacement above breaks.
    expr = expr.replace("  not =", " !=")

    try:
        return bool(eval(expr, {"__builtins__": {}}, {}))  # noqa: S307 - fixed input
    except Exception as exc:  # pragma: no cover - surfaces malformed guards
        print(f"// WARNING: could not evaluate {expr!r}: {exc}", file=sys.stderr)
        return False


# Scalar widths for the packed packet structs in packets.hpp.
SCALAR_SIZES = {
    "int8": 1, "uint8": 1, "char": 1, "bool": 1,
    "int16": 2, "uint16": 2,
    "int32": 4, "uint32": 4, "int": 4,
    "int64": 8, "uint64": 8,
}

# The only named array bounds the packet structs use.
ARRAY_CONSTS = {
    "NAME_LENGTH": 24,            # (23 + 1)
    "MAP_NAME_LENGTH": 12,        # (11 + 1)
    "MAP_NAME_LENGTH_EXT": 16,    # MAP_NAME_LENGTH + 4
    "WEB_AUTH_TOKEN_LENGTH": 17,  # 16 + 1
}

# Any packed struct, not just PACKET_* — some packets embed helper structs
# (hotkey_data, for one) whose size is needed to size the packet.
STRUCT_START_RE = re.compile(r"^\s*struct\s+(\w+)\s*\{")
STRUCT_END_RE = re.compile(r"^\s*\}\s*__attribute__\(\(packed\)\)\s*;")
FIELD_RE = re.compile(
    r"^\s*(?:struct\s+)?(\w+)\s+(\w+)\s*(?:\[\s*([^\]]*)\s*\])?\s*;"
)
HEADER_RE = re.compile(
    r"^\s*DEFINE_PACKET_(?:HEADER|ID)\s*\(\s*(\w+)\s*,\s*(0[xX][0-9A-Fa-f]+)\s*\)"
)

# Array bounds are sometimes #defined in the same header, inside the version
# guards, so they have to be picked up as we walk rather than known up front.
DEFINE_RE = re.compile(r"^\s*#\s*define\s+([A-Z_][A-Z_0-9]*)\s+(.+?)\s*$")


def struct_size(fields, structs, consts):
    """Packed size of a struct, or None when it can't be resolved."""
    total = 0
    for type_name, field_name, bound in fields:
        if field_name == "packetLength":
            # Carries its own length on the wire.
            return VARIABLE
        if type_name in SCALAR_SIZES:
            width = SCALAR_SIZES[type_name]
        elif type_name in structs:
            nested = struct_size(structs[type_name], structs, consts)
            if nested is None or nested == VARIABLE:
                return None
            width = nested
        else:
            return None

        if bound is None:
            total += width
            continue
        bound = bound.strip()
        if bound == "":
            # Flexible array member: variable-length packet.
            return VARIABLE
        if bound.isdigit():
            count = int(bound)
        elif bound in consts:
            count = consts[bound]
        else:
            return None
        total += width * count
    return total


VARIABLE = -1


def parse_structs(path, env):
    """Collect struct definitions and their packet ids, honouring #if guards."""
    structs = {}
    ids = {}
    consts = dict(ARRAY_CONSTS)
    stack = []
    current = None
    fields = []

    with open(path, encoding="utf-8", errors="replace") as handle:
        for line in handle:
            cond = COND_RE.match(line)
            if cond:
                kind, rest = cond.group(1), cond.group(2)
                if kind in ("if", "ifdef", "ifndef"):
                    if kind == "ifdef":
                        value = evaluate(f"defined({rest.strip()})", env)
                    elif kind == "ifndef":
                        value = not evaluate(f"defined({rest.strip()})", env)
                    else:
                        value = evaluate(rest, env)
                    parent = all(frame[0] for frame in stack)
                    stack.append([value and parent, value])
                elif kind == "elif":
                    if stack:
                        taken = stack[-1][1]
                        value = evaluate(rest, env) and not taken
                        parent = all(frame[0] for frame in stack[:-1])
                        stack[-1] = [value and parent, taken or value]
                elif kind == "else":
                    if stack:
                        taken = stack[-1][1]
                        parent = all(frame[0] for frame in stack[:-1])
                        stack[-1] = [(not taken) and parent, True]
                elif kind == "endif":
                    if stack:
                        stack.pop()
                continue

            if stack and not all(frame[0] for frame in stack):
                continue

            if current is None:
                define = DEFINE_RE.match(line)
                if define:
                    value = define.group(2).split("//")[0].strip()
                    try:
                        consts[define.group(1)] = int(
                            eval(value, {"__builtins__": {}}, dict(consts))  # noqa: S307
                        )
                    except Exception:
                        pass  # not an integer constant; nothing we can use
                    continue

                start = STRUCT_START_RE.match(line)
                if start:
                    current = start.group(1)
                    fields = []
                    continue
                header = HEADER_RE.match(line)
                if header:
                    ids["PACKET_" + header.group(1)] = int(header.group(2), 16)
                continue

            if STRUCT_END_RE.match(line):
                structs[current] = fields
                current = None
                continue

            field = FIELD_RE.match(line)
            if field:
                fields.append((field.group(1), field.group(2), field.group(3)))

    lengths = {}
    for name, pid in ids.items():
        if name not in structs:
            continue
        size = struct_size(structs[name], structs, consts)
        if size is not None:
            lengths[pid] = size
    return lengths


def parse(path: str, env: dict) -> dict:
    """Walk the file honouring #if/#elif/#else/#endif nesting."""
    lengths = {}
    # Each stack frame: [currently_active, any_branch_taken_yet]
    stack = []

    with open(path, encoding="utf-8", errors="replace") as handle:
        for line in handle:
            cond = COND_RE.match(line)
            if cond:
                kind, rest = cond.group(1), cond.group(2)
                if kind in ("if", "ifdef", "ifndef"):
                    if kind == "ifdef":
                        value = evaluate(f"defined({rest.strip()})", env)
                    elif kind == "ifndef":
                        value = not evaluate(f"defined({rest.strip()})", env)
                    else:
                        value = evaluate(rest, env)
                    parent = all(frame[0] for frame in stack)
                    stack.append([value and parent, value])
                elif kind == "elif":
                    if stack:
                        taken = stack[-1][1]
                        value = evaluate(rest, env) and not taken
                        parent = all(frame[0] for frame in stack[:-1])
                        stack[-1] = [value and parent, taken or value]
                elif kind == "else":
                    if stack:
                        taken = stack[-1][1]
                        parent = all(frame[0] for frame in stack[:-1])
                        stack[-1] = [(not taken) and parent, True]
                elif kind == "endif":
                    if stack:
                        stack.pop()
                continue

            if stack and not all(frame[0] for frame in stack):
                continue

            match = PACKET_RE.match(line)
            if match:
                lengths[int(match.group(1), 16)] = int(match.group(2))

    return lengths


def main() -> int:
    if len(sys.argv) < 2:
        print(__doc__, file=sys.stderr)
        return 2

    src = sys.argv[1].rstrip("/")
    packetver = int(sys.argv[2]) if len(sys.argv) > 2 else 20211103
    env = build_env(packetver)

    # The packet db is authoritative where it has an entry. Modern rAthena
    # declares many packets only as packed structs, so those fill the gaps.
    lengths = parse(f"{src}/src/map/clif_packetdb.hpp", env)
    db_count = len(lengths)

    from_structs = {}
    for header in (
        "src/map/packets.hpp",
        "src/map/packets_struct.hpp",
        "src/common/packets.hpp",
    ):
        from_structs.update(parse_structs(f"{src}/{header}", env))

    conflicts = 0
    added = 0
    for pid, size in from_structs.items():
        if pid in lengths:
            if lengths[pid] != size:
                conflicts += 1
                print(
                    f"// NOTE: 0x{pid:04X} packet_db={lengths[pid]} struct={size}"
                    " (keeping packet_db)",
                    file=sys.stderr,
                )
            continue
        lengths[pid] = size
        added += 1

    print(
        f"// packet_db entries: {db_count}, added from structs: {added},"
        f" conflicts: {conflicts}",
        file=sys.stderr,
    )

    out = []
    out.append("// Code generated by tools/packetlen/gen.py. DO NOT EDIT.")
    out.append("//")
    out.append(f"// Source: rAthena src/map/clif_packetdb.hpp at PACKETVER {packetver}.")
    out.append("// Regenerate with:")
    out.append("//")
    out.append("//\tpython3 tools/packetlen/gen.py <rathena-src> %d > \\" % packetver)
    out.append("//\t\tinternal/network/packets/lengths.go")
    out.append("")
    out.append("package packets")
    out.append("")
    out.append("// VariableLength marks a packet whose size is carried in a uint16 at")
    out.append("// offset 2 rather than being fixed.")
    out.append("const VariableLength = -1")
    out.append("")
    out.append("// mapPacketLengths is the wire length of each map-server packet, keyed by")
    out.append("// packet id. RO has no framing markers, so a wrong entry here desyncs the")
    out.append("// whole connection rather than corrupting one packet.")
    out.append("var mapPacketLengths = map[uint16]int{")
    for pid in sorted(lengths):
        out.append(f"\t0x{pid:04X}: {lengths[pid]},")
    out.append("}")
    out.append("")

    print("\n".join(out))
    print(f"// generated {len(lengths)} entries", file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
