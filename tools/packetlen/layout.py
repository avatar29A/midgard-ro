#!/usr/bin/env python3
"""Print the wire layout of an rAthena packet struct at a given PACKETVER.

Packet field offsets are the one thing that cannot be guessed. A wrong offset
does not fail loudly — it reads a neighbouring field, so a character faces the
wrong way or walks at the wrong speed, and the bug looks like it lives in the
movement code. This resolves the #if guards the same way the server's compiler
does and reports where each field actually lands.

Usage:
    layout.py <rathena-src> <struct-name> [packetver]

Example:
    layout.py ~/src/rathena packet_idle_unit 20211103
"""

import sys

from gen import SCALAR_SIZES, build_env, collect_structs, struct_size

HEADERS = (
    "src/map/packets_struct.hpp",
    "src/map/packets.hpp",
    "src/common/packets.hpp",
)


def field_width(type_name, bound, structs, consts):
    """Byte width of one field, or None when it cannot be resolved."""
    if type_name in SCALAR_SIZES:
        width = SCALAR_SIZES[type_name]
    elif type_name in structs:
        width = struct_size(structs[type_name], structs, consts)
        if width is None or width < 0:
            return None
    else:
        return None

    if bound is None:
        return width
    bound = bound.strip()
    if bound == "":
        return None  # flexible array member: everything after it is variable
    if bound.isdigit():
        return width * int(bound)
    if bound in consts:
        return width * consts[bound]
    return None


def main():
    if len(sys.argv) < 3:
        print(__doc__, file=sys.stderr)
        return 2

    src = sys.argv[1].rstrip("/")
    wanted = sys.argv[2]
    packetver = int(sys.argv[3]) if len(sys.argv) > 3 else 20211103
    env = build_env(packetver)

    structs, ids, consts = {}, {}, dict()
    for header in HEADERS:
        try:
            found, found_ids, found_consts = collect_structs(f"{src}/{header}", env)
        except FileNotFoundError:
            continue
        structs.update(found)
        ids.update(found_ids)
        consts.update(found_consts)

    if wanted not in structs:
        print(f"no struct {wanted!r} at PACKETVER {packetver}", file=sys.stderr)
        close = [name for name in structs if wanted.lower() in name.lower()]
        if close:
            print("did you mean: " + ", ".join(sorted(close)[:10]), file=sys.stderr)
        return 1

    print(f"// {wanted} at PACKETVER {packetver}")
    variant = "RE" if env["PACKETVER_RE_NUM"] else "MAIN"
    print(f"// build variant: {variant}")

    offset = 0
    for type_name, name, bound in structs[wanted]:
        width = field_width(type_name, bound, structs, consts)
        shown = f"{type_name} {name}" + (f"[{bound}]" if bound is not None else "")
        if width is None:
            print(f"{offset:>4}  {shown:<34} <- variable from here")
            break
        print(f"{offset:>4}  {shown:<34} {width} byte(s)")
        offset += width
    else:
        print(f"{offset:>4}  (total)")

    return 0


if __name__ == "__main__":
    sys.exit(main())
