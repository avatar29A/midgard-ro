#!/usr/bin/env python3
"""Generate the skill-to-effect table from nostalro-client's.

What a skill looks like when it goes off is not in the archive and not on the
wire. The client's own skilleffectinfolist.lub names an effect for 65 skills
and says nothing about the rest; the effects themselves are mostly not STR
files at all but particle systems the original client draws in code, which is
why there is nothing to read them out of.

nostalro-client has done that identification, in the open, under Apache 2.0:

    https://github.com/nmeylan/nostalro-client
    lib/effects/src/skill_effects.rs

Its tables say, per skill, what plays when the cast begins, on the caster, on
the target and on a placed cell. This reads them and writes ours.

Nothing is trusted on its name alone. A skill name has to resolve against
rAthena's own skill_db — theirs are the same names with the underscores taken
out — and an effect name has to have an implementation in their effects
directory, which is what says the effect is one somebody has actually worked
out rather than a placeholder. What fails either check is reported and left
out.

Usage:
    gen.py <nostalro checkout> <skill_db.yml> <names.go> > skilleffects.go
"""

import os
import re
import sys

# The job prefixes of the first and second classes, which is where this starts.
# Their skills have not changed since the version nostalro targets, so the
# mapping carries over whole; the later classes have and are left for when
# somebody checks them.
FIRST_AND_SECOND = (
    'NV', 'SM', 'MG', 'AC', 'AL', 'MC', 'TF',           # novice and first
    'KN', 'PR', 'WZ', 'BS', 'HT', 'AS', 'CR', 'MO',     # second
    'SA', 'RG', 'AM', 'BD', 'DC', 'CH', 'PA', 'MS',
)

# Which table each function is.
TABLES = {
    'begin_cast_effect': 'BeginCast',
    'caster_skill_effects': 'OnCaster',
    'target_skill_effects': 'OnTarget',
    'ground_placed_effect': 'OnGround',
}


def rust_arms(source, function):
    """Every `S::Name | S::Other => ...` arm of one function, as (skills, effects)."""
    start = source.index(f'pub fn {function}(')
    end = source.index('\npub fn ', start + 1) if '\npub fn ' in source[start + 1:] else len(source)
    body = source[start:end]

    arms = []
    for match in re.finditer(r'^\s{8}((?:S::\w+(?:\s*\|\s*)?)+)\s*=>(.*?)(?=\n\s{8}\S|\n\s{4}\})',
                             body, re.M | re.S):
        skills = re.findall(r'S::(\w+)', match.group(1))
        effects = re.findall(r'E::(\w+)', match.group(2))
        if skills:
            arms.append((skills, effects))

    return arms


def server_skills(path):
    """rAthena's skill names to ids, keyed the way nostalro spells them."""
    ids, flat = {}, {}
    current = None

    for line in open(path, encoding='utf-8'):
        found = re.match(r'^  - Id: (\d+)', line)
        if found:
            current = int(found.group(1))
            continue

        name = re.match(r'^    Name: (\S+)', line)
        if name and current is not None:
            ids[name.group(1)] = current
            flat.setdefault(name.group(1).replace('_', '').lower(), []).append(name.group(1))

    return ids, flat


def implemented_effects(root):
    """Effect names nostalro has an implementation for, from its effects directory."""
    directory = os.path.join(root, 'lib/effects/src/effects')
    found = {}

    for entry in sorted(os.listdir(directory)):
        if not entry.endswith('.rs'):
            continue

        # The whole module comment, not just its first line: a file often
        # covers several effects and names them across the header.
        header = []
        for line in open(os.path.join(directory, entry), encoding='utf-8'):
            if not line.startswith('//!'):
                break

            header.append(line)

        for name in re.findall(r'EF_[A-Z0-9_]+', ''.join(header)):
            found.setdefault(name, entry)

    return found


def known_skills(names_go):
    return {int(m.group(1)) for m in re.finditer(r'^\t(\d+):\s+"', open(names_go, encoding='utf-8').read(), re.M)}


def main():
    if len(sys.argv) != 4:
        print(__doc__, file=sys.stderr)
        return 2

    root, skill_db, names_go = sys.argv[1:4]
    source = open(os.path.join(root, 'lib/effects/src/skill_effects.rs'), encoding='utf-8').read()

    ids, flat = server_skills(skill_db)
    effects = implemented_effects(root)
    known = known_skills(names_go)

    # skill id -> {table: [effect names]}
    table = {}
    unresolved_skills, unresolved_effects = set(), set()

    for function, label in TABLES.items():
        for skills, names in rust_arms(source, function):
            # Every effect the table names is kept. Whether nostalro has an
            # implementation for it is a separate question, counted below —
            # the identification is what is being ported here, and it is worth
            # having for an effect nobody has drawn yet.
            wanted = ['EF_' + name.upper() for name in names]
            for effect in wanted:
                if effect not in effects:
                    unresolved_effects.add(effect)

            if not wanted:
                continue

            for skill in skills:
                flat_key = skill.lower()
                candidates = flat.get(flat_key, [])
                if len(candidates) != 1:
                    unresolved_skills.add(skill)
                    continue

                server_name = candidates[0]
                if not server_name.split('_')[0] in FIRST_AND_SECOND:
                    continue

                skill_id = ids[server_name]
                if skill_id not in known:
                    unresolved_skills.add(skill)
                    continue

                table.setdefault(skill_id, {}).setdefault(label, [])
                for effect in wanted:
                    if effect not in table[skill_id][label]:
                        table[skill_id][label].append(effect)

    out = [
        "// Code generated by tools/skilleffects/gen.py. DO NOT EDIT.",
        "//",
        "// Source: nostalro-client, lib/effects/src/skill_effects.rs, which is",
        "// licensed Apache 2.0 and lives at https://github.com/nmeylan/nostalro-client.",
        "// The identification of which effect belongs to which skill is theirs; this",
        "// is a translation of it.",
        "//",
        "// First and second class skills only. Those have not changed since the",
        "// version nostalro targets, so the mapping carries over whole; the later",
        "// classes have, and are left until somebody checks them.",
        "",
        "package skills",
        "",
        "// SkillEffects is what a skill plays, and where.",
        "//",
        "// Four moments, because the original draws four: the circle under the",
        "// caster while it casts, what the caster does when it goes off, what appears",
        "// on whoever it hit, and what is left on a cell for a placed skill.",
        "type SkillEffects struct {",
        "\tBeginCast []string",
        "\tOnCaster  []string",
        "\tOnTarget  []string",
        "\tOnGround  []string",
        "}",
        "",
        "// skillEffects maps a skill id to the effects it plays, by the names the",
        "// client's own EFID table uses.",
        "var skillEffects = map[uint16]SkillEffects{",
    ]

    def quoted(names):
        return "{" + ", ".join(f'"{n}"' for n in names) + "}"

    for skill_id in sorted(table):
        parts = []
        for label in ('BeginCast', 'OnCaster', 'OnTarget', 'OnGround'):
            if table[skill_id].get(label):
                parts.append(f"{label}: []string{quoted(table[skill_id][label])}")

        out.append(f"\t{skill_id}: {{{', '.join(parts)}}},")

    out += ["}", ""]
    print("\n".join(out))

    named = {e for entry in table.values() for names in entry.values() for e in names}
    drawn = {e for e in named if e in effects}

    print(f"// {len(table)} first and second class skills mapped, naming "
          f"{len(named)} effects, of which {len(drawn)} have an implementation "
          f"upstream to work from", file=sys.stderr)
    if unresolved_skills:
        print(f"// skills that did not resolve: {len(unresolved_skills)} "
              f"{sorted(unresolved_skills)[:8]}", file=sys.stderr)
    if unresolved_effects:
        print(f"// effects with no implementation upstream: {len(unresolved_effects)} "
              f"{sorted(unresolved_effects)[:8]}", file=sys.stderr)

    return 0


if __name__ == "__main__":
    sys.exit(main())
