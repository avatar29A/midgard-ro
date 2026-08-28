#!/usr/bin/env python3
"""Generate the @-command table for docs/research/chat-commands.md.

Reads the rAthena tree that `make server-up` clones at the pinned SHA, so the
document cannot drift from the server we actually run. Writes markdown to
stdout; warnings go to stderr.

    python3 tools/chatcmds/gen.py > /tmp/at-table.md

Sources:
    conf/atcommands.yml     command names, aliases, help text
    conf/groups.yml         which group may use which command
    src/map/atcommand.cpp   the registration table — the real list

PyYAML is not a dependency of this repo and both YAML files have a flat, fixed
shape, so they are parsed here directly rather than pulling one in.
"""
import re
import sys

R = "docker/rathena/build/rathena"


# --- reading the server tree -------------------------------------------------


def parse_atcommands(path):
    """[{name, aliases[], help[]}] from conf/atcommands.yml."""
    cmds, cur, mode = [], None, None
    for raw in open(path, encoding="utf-8"):
        line = raw.rstrip("\n")
        m = re.match(r"^  - Command: (\S+)\s*$", line)
        if m:
            cur = {"name": m.group(1), "aliases": [], "help": []}
            cmds.append(cur)
            mode = None
            continue
        if cur is None:
            continue
        if re.match(r"^    Aliases:\s*$", line):
            mode = "alias"
            continue
        if re.match(r"^    Help: \|\s*$", line):
            mode = "help"
            continue
        if mode == "alias":
            m = re.match(r"^      - (\S+)\s*$", line)
            if m:
                cur["aliases"].append(m.group(1))
                continue
            mode = None
        if mode == "help":
            m = re.match(r"^      (.*)$", line)
            if m:
                cur["help"].append(m.group(1).rstrip())
                continue
            mode = None
    return cmds


def parse_groups(path):
    """{name: {id, level, commands, charcommands, perms, inherit}} from groups.yml."""
    groups, cur, section = {}, None, None
    for raw in open(path, encoding="utf-8"):
        line = raw.rstrip("\n")
        m = re.match(r"^  - Id: (\d+)\s*$", line)
        if m:
            cur = {"id": int(m.group(1)), "name": None, "level": 0,
                   "commands": set(), "charcommands": set(),
                   "perms": set(), "inherit": []}
            section = None
            continue
        if cur is None:
            continue
        m = re.match(r"^    Name: (.+?)\s*$", line)
        if m:
            cur["name"] = m.group(1)
            groups[cur["name"]] = cur
            continue
        m = re.match(r"^    Level: (\d+)\s*$", line)
        if m:
            cur["level"] = int(m.group(1))
            continue
        header = False
        for key, field in (("Commands", "commands"),
                           ("CharCommands", "charcommands"),
                           ("Permissions", "perms"),
                           ("Inherit", "inherit")):
            if re.match(r"^    %s:\s*$" % key, line):
                section, header = field, True
                break
        if header:
            continue
        m = re.match(r"^      ([A-Za-z0-9_ ]+): (true|false)\s*$", line)
        if m and section and m.group(2) == "true":
            if section == "inherit":
                cur["inherit"].append(m.group(1))
            else:
                cur[section].add(m.group(1))
    return groups


def resolve(groups, name, field, seen=None):
    """A group's effective set for `field`, following Inherit."""
    seen = seen if seen is not None else set()
    if name in seen or name not in groups:
        return set()
    seen.add(name)
    out = set(groups[name][field])
    for parent in groups[name]["inherit"]:
        out |= resolve(groups, parent, field, seen)
    return out


def registered(path):
    """Command names as atcommand.cpp actually registers them.

    Four macro forms: ACMD_DEF(name), ACMD_DEF2("name", fn),
    ACMD_DEFR(name, restriction), ACMD_DEF2R("name", fn, restriction).
    """
    src = open(path, encoding="utf-8", errors="replace").read()
    names = set()
    for m in re.finditer(r"ACMD_DEF2R?\(\s*\"([^\"]+)\"", src):
        names.add(m.group(1))
    for m in re.finditer(r"ACMD_DEFR?\(\s*([a-z0-9_]+)\s*[,)]", src):
        if m.group(1) not in ("x", "x2"):  # the #define lines themselves
            names.add(m.group(1))
    return names


# --- rendering ---------------------------------------------------------------


def cell(text):
    """Collapse to one line and protect the column separator."""
    return re.sub(r"\s+", " ", text.replace("|", "\\|").strip())


def prose(text):
    """cell(), plus the escaping that only bare text needs.

    The help text is full of <char name> placeholders. In a prose cell a
    renderer eats those as HTML tags and the parameter silently vanishes; in a
    code span it must NOT be escaped, or the entity shows up literally. Hence
    two functions.
    """
    return cell(text).replace("<", "&lt;").replace(">", "&gt;")


def split_help(help_lines):
    """Separate the 'Params: ...' syntax lines from the description."""
    params, desc = [], []
    for line in help_lines:
        s = line.strip()
        if not s:
            continue
        m = re.match(r"^Params?:\s*(.*)$", s, re.I)
        if not m:
            desc.append(s)
            continue
        rest = m.group(1).strip()
        # Several entries run "Params: <x>. Sentence about it." on one line.
        cut = re.match(r"^(<.*?>|\S+)\.\s+(.+)$", rest)
        if cut and cut.group(2)[:1].isupper():
            params.append(cut.group(1))
            desc.append(cut.group(2))
        else:
            params.append(rest.rstrip("."))
    return params, desc


def main():
    cmds = parse_atcommands(f"{R}/conf/atcommands.yml")
    groups = parse_groups(f"{R}/conf/groups.yml")
    reg = registered(f"{R}/src/map/atcommand.cpp")

    names = set(c["name"] for c in cmds)
    if names ^ reg:
        print("WARNING: atcommands.yml and atcommand.cpp disagree — "
              f"yml-only={sorted(names - reg)} code-only={sorted(reg - names)}",
              file=sys.stderr)

    # Lowest group granting each command, walking groups by level then id. A
    # command no group lists is reachable only through `all_commands`.
    eff = {g: resolve(groups, g, "commands") for g in groups}
    grant = {}
    for g in sorted(groups.values(), key=lambda x: (x["level"], x["id"])):
        for c in eff[g["name"]]:
            grant.setdefault(c, g)

    seen, rows = set(), []
    for c in cmds:
        if c["name"] in seen:  # atcommands.yml lists refineui twice
            continue
        seen.add(c["name"])
        params, desc = split_help(c["help"])
        syntax = " / ".join(f"`@{c['name']} {p}`" for p in params) if params \
            else f"`@{c['name']}`"
        effect = prose(" ".join(desc)) or "—"
        if len(effect) > 190:
            effect = effect[:187].rsplit(" ", 1)[0] + "…"
        aliases = ", ".join(f"`{a}`" for a in c["aliases"]) or "—"
        g = grant.get(c["name"])
        group = f"{g['name']} ({g['id']})" if g else "Admin (99)"
        rows.append((c["name"], aliases, cell(syntax), effect, group))

    rows.sort(key=lambda r: r[0])

    print("| Command | Aliases | Syntax | Effect | Lowest group |")
    print("|---------|---------|--------|--------|--------------|")
    for name, aliases, syntax, effect, group in rows:
        print(f"| `@{name}` | {aliases} | {syntax} | {effect} | {group} |")

    print(f"{len(rows)} commands, {len(groups)} groups", file=sys.stderr)


if __name__ == "__main__":
    main()
