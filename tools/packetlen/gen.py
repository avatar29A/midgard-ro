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

# packet(id, len) — the server -> client side. Like parseable_packet, either
# argument may be a literal, an enum constant or sizeof(PACKET_*), so both are
# captured as tokens and resolved rather than matched as numbers.
PACKET_RE = re.compile(r"^\s*packet\(\s*([^,]+?)\s*,\s*(.+?)\s*\)\s*;")

# parseable_packet(id, len, handler, ...) — the client -> server side. Both the
# id and the length may be a literal, a HEADER_* macro or sizeof(PACKET_*).
PARSEABLE_RE = re.compile(
    r"^\s*parseable_packet2?\(\s*([^,]+?)\s*,\s*([^,]+?)\s*,\s*clif_parse_(\w+)"
)
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
    # rAthena annotates several version guards inline, e.g.
    #   #if PACKETVER >= 20130000 /* not sure date */
    # Left in place these fail to parse, and a guard that cannot be evaluated
    # is treated as false, silently dropping the struct behind it.
    expr = re.sub(r"/\*.*?\*/", " ", expr)
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
    "float": 4, "double": 8,
}

# Fallback array bounds, for callers that resolve a single struct without
# scanning CONST_HEADERS first — layout.py does exactly that. The generator
# itself no longer needs these: every one is read from mmo.hpp with the same
# value, and the table is complete without them. They are kept because a bound
# that goes missing should degrade to a stale number in one tool rather than an
# unresolvable struct in both.
ARRAY_CONSTS = {
    "NAME_LENGTH": 24,            # (23 + 1)
    "MAP_NAME_LENGTH": 12,        # (11 + 1)
    "MAP_NAME_LENGTH_EXT": 16,    # MAP_NAME_LENGTH + 4
    "WEB_AUTH_TOKEN_LENGTH": 17,  # 16 + 1
}

# Headers scanned for their #defines alone, before the packet headers, because
# packet structs use bounds declared there: MAX_ITEM_OPTIONS is defined in
# packets.hpp as MAX_ITEM_RDM_OPT, which is in mmo.hpp, and TALKBOX_MESSAGE_SIZE
# is in map.hpp behind a PACKETVER guard — 21 from 20190904, 80 before it.
# Reading them beats writing the numbers down: a guarded constant transcribed
# at one packetver is wrong at another, and wrong silently.
CONST_HEADERS = (
    "src/common/mmo.hpp",
    "src/map/map.hpp",
)

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

# Enumerators such as `useItemAckType = 0x1c8`. rAthena names several packet
# ids this way and then writes packet(useItemAckType, ...) rather than the
# number, so an id-to-length table built from literals alone loses them. This
# is how 0x0B09 went missing and needed lengths_extra.go to carry it by hand.
ENUM_RE = re.compile(r"^\s*(\w+)\s*=\s*(0[xX][0-9A-Fa-f]+|\d+)\s*,?\s*(?://.*)?$")


def value_of(token: str, ids: dict, structs: dict, consts: dict):
    """Resolve one packet() / parseable_packet() argument to a number.

    Either argument can be a bare number, an enum constant, a HEADER_* macro or
    sizeof(PACKET_*). Returns None when it cannot be resolved, which the
    callers report rather than skip.
    """
    token = token.strip()
    if re.fullmatch(r"-?\d+", token):
        return int(token)
    if re.fullmatch(r"0[xX][0-9A-Fa-f]+", token):
        return int(token, 16)
    if token.startswith("HEADER_"):
        return ids.get("PACKET_" + token[len("HEADER_"):])

    # rAthena writes both sizeof(PACKET_X) and sizeof( struct PACKET_X ).
    sizeof = re.fullmatch(r"sizeof\(\s*(?:struct\s+)?(\w+)\s*\)", token)
    if sizeof and sizeof.group(1) in structs:
        return struct_size(structs[sizeof.group(1)], structs, consts)
    if token in consts:
        return consts[token]

    return None


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


# The headers that between them declare the packet structs. Which header a
# struct lands in says nothing about where its id is declared: ZC_ITEM_ENTRY is
# a struct in packets_struct.hpp whose DEFINE_PACKET_HEADER is in packets.hpp,
# under a comment reading "Other packets without struct defined in this file".
# So these are collected together and paired once at the end. Pairing file by
# file loses every packet split across two of them, and it loses them silently:
# the id simply never reaches the table, the reader resynchronises past the
# packet on the wire, and no handler ever runs.
HEADERS = (
    "src/map/packets.hpp",
    "src/map/packets_struct.hpp",
    "src/common/packets.hpp",
)


def collect_all(src: str, env: dict):
    """Merge the struct definitions, ids and constants from every header."""
    consts = dict(ARRAY_CONSTS)
    for header in CONST_HEADERS:
        _, _, found_consts = collect_structs(f"{src}/{header}", env, consts)
        consts.update(found_consts)

    structs, ids = {}, {}
    for header in HEADERS:
        found, found_ids, found_consts = collect_structs(f"{src}/{header}", env, consts)
        structs.update(found)
        ids.update(found_ids)
        consts.update(found_consts)

    return structs, ids, consts


def sizes_by_id(structs: dict, ids: dict, consts: dict) -> dict:
    """Packet id -> wire length, for every id whose struct resolves.

    A struct that will not resolve — an unknown field type, an array bound we
    cannot find — is reported rather than quietly skipped. Skipping is how
    0x0B09 stayed missing: the packet arrived, the reader had no length for it,
    resynchronised past the whole inventory, and the only sign was one warning
    line that read like a server hiccup.
    """
    lengths = {}
    skipped = []
    for name, pid in ids.items():
        if name not in structs:
            continue
        size = struct_size(structs[name], structs, consts)
        if size is None:
            skipped.append((pid, name))
            continue
        lengths[pid] = size

    for pid, name in sorted(skipped):
        print(f"// WARNING: 0x{pid:04X} {name} has a struct but no resolvable"
              " size — it will be missing from the table", file=sys.stderr)

    return lengths


def collect_structs(path, env, seed=None):
    """Collect struct definitions, their packet ids and any array-bound
    constants, honouring #if guards.

    seed carries constants already collected from earlier headers, so a bound
    defined in terms of another header's constant still resolves.

    Kept separate from sizes_by_id so tools that need the field layout — not
    just the total size — can walk the same resolved definitions.
    """
    structs = {}
    ids = {}
    consts = dict(ARRAY_CONSTS if seed is None else seed)
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

                enum = ENUM_RE.match(line)
                if enum and enum.group(1) not in consts:
                    consts[enum.group(1)] = int(enum.group(2), 0)
                continue

            if STRUCT_END_RE.match(line):
                structs[current] = fields
                current = None
                continue

            field = FIELD_RE.match(line)
            if field:
                fields.append((field.group(1), field.group(2), field.group(3)))

    return structs, ids, consts


def active_lines(path: str, env: dict):
    """Yield the lines of path whose #if/#elif/#else/#endif guards hold.

    Both packet tables are walked this way. Getting the nesting subtly
    different between them would show up as one packet id being wrong at one
    packetver, which is the hardest kind of bug to see.
    """
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

            yield line


def parse(path: str, env: dict, ids: dict, structs: dict, consts: dict) -> dict:
    """Server -> client lengths, from the packet(...) entries."""
    lengths = {}
    for line in active_lines(path, env):
        match = PACKET_RE.match(line)
        if not match:
            continue

        pid = value_of(match.group(1), ids, structs, consts)
        size = value_of(match.group(2), ids, structs, consts)
        if pid is None or size is None:
            print(f"// WARNING: could not resolve packet({match.group(1)},"
                  f" {match.group(2)})", file=sys.stderr)
            continue

        lengths[pid] = size

    return lengths


def parse_parseable(path: str, env: dict, ids: dict, structs: dict, consts: dict):
    """Client -> server lengths, from the parseable_packet(...) entries.

    Returns (lengths, unresolved), where lengths maps id -> (size, handler).

    A later registration wins: rAthena re-registers an id inside a version
    guard when the layout changed, and the last one whose guard holds is what a
    client at this packetver speaks. That is exactly the trap /item falls into
    — 0x013F at 26 bytes in the base block, 0x09CE at 102 from 20131223.

    Either argument can be a bare number, a HEADER_* macro, or
    sizeof(PACKET_*), so the struct table already collected for the server side
    is reused to resolve them.
    """
    lengths = {}
    unresolved = []

    for line in active_lines(path, env):
        match = PARSEABLE_RE.match(line)
        if not match:
            continue

        pid = value_of(match.group(1), ids, structs, consts)
        size = value_of(match.group(2), ids, structs, consts)
        handler = match.group(3)
        if pid is None or size is None:
            unresolved.append((match.group(1).strip(), match.group(2).strip(), handler))
            continue

        lengths[pid] = (size, handler)

    return lengths, unresolved


def emit_client(src: str, packetver: int, env: dict) -> int:
    """Write the client -> server table.

    This one does not frame anything — we only ever send packets we build, so
    nothing depends on it at runtime. It exists so an encoder's length can be
    checked against the server that will parse it, rather than against a number
    somebody read off a wiki for a different packetver.
    """
    structs, ids, consts = collect_all(src, env)

    lengths, unresolved = parse_parseable(
        f"{src}/src/map/clif_packetdb.hpp", env, ids, structs, consts
    )

    for pid, size, handler in unresolved:
        print(f"// NOTE: could not resolve parseable_packet({pid}, {size}, "
              f"clif_parse_{handler})", file=sys.stderr)

    out = []
    out.append("// Code generated by tools/packetlen/gen.py --client. DO NOT EDIT.")
    out.append("//")
    out.append(f"// Source: rAthena src/map/clif_packetdb.hpp at PACKETVER {packetver}.")
    out.append("// Regenerate with:")
    out.append("//")
    out.append("//\tpython3 tools/packetlen/gen.py <rathena-src> %d --client > \\" % packetver)
    out.append("//\t\tinternal/network/packets/clientlengths.go")
    out.append("//\tgofmt -w internal/network/packets/clientlengths.go")
    out.append("")
    out.append("package packets")
    out.append("")
    out.append("// clientPacketLengths is the size the server expects of each packet we")
    out.append("// send, keyed by packet id, with the handler that will parse it.")
    out.append("//")
    out.append("// Nothing frames outgoing packets — we build them, so we know how long")
    out.append("// they are. This table is here to check that what we build matches what")
    out.append("// the server will read, at the packetver we actually run. Several ids are")
    out.append("// registered more than once behind version guards and the wrong one is")
    out.append("// easy to copy from a wiki: /item is 0x013F at 26 bytes in the base block")
    out.append("// and 0x09CE at 102 from PACKETVER 20131223.")
    out.append("//")
    out.append("// A length of VariableLength means the size travels in the packet.")
    out.append("var clientPacketLengths = map[uint16]int{")
    for pid in sorted(lengths):
        size, handler = lengths[pid]
        out.append(f"\t0x{pid:04X}: {size}, // clif_parse_{handler}")
    out.append("}")
    out.append("")
    out.append("// ClientPacketLength is the length the server expects for a packet we")
    out.append("// send. Reports false for an id the server does not parse at this")
    out.append("// packetver — which, for a packet we are about to send, means we are")
    out.append("// about to be ignored.")
    out.append("func ClientPacketLength(id uint16) (int, bool) {")
    out.append("\tsize, ok := clientPacketLengths[id]")
    out.append("")
    out.append("\treturn size, ok")
    out.append("}")
    out.append("")

    print("\n".join(out))
    print(f"// generated {len(lengths)} client->server entries,"
          f" {len(unresolved)} unresolved", file=sys.stderr)

    return 0


def main() -> int:
    if len(sys.argv) < 2:
        print(__doc__, file=sys.stderr)
        return 2

    args = [a for a in sys.argv[1:] if not a.startswith("--")]
    client_side = "--client" in sys.argv

    src = args[0].rstrip("/")
    packetver = int(args[1]) if len(args) > 1 else 20211103
    env = build_env(packetver)

    if client_side:
        return emit_client(src, packetver, env)

    # The packet db is authoritative where it has an entry. Modern rAthena
    # declares many packets only as packed structs, so those fill the gaps.
    structs, ids, consts = collect_all(src, env)
    lengths = parse(f"{src}/src/map/clif_packetdb.hpp", env, ids, structs, consts)
    db_count = len(lengths)

    from_structs = sizes_by_id(structs, ids, consts)

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
