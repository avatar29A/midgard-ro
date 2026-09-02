#!/usr/bin/env python3
"""Report where the client's skill data and the server's disagree.

Both describe the same skills and neither is a copy of the other: the archive
is kRO's, the database is rAthena's, and they are years apart in places. Which
one to believe depends on the field, and that is the point of running this —
the answer is not "the client is wrong", it is "here is what you cannot take
from the client".

What it prints:

  - skills one side has and the other does not
  - max levels that disagree, which decide what a skill point may buy
  - SP costs that disagree, which decide what the window says a cast costs

Usage:
    crosscheck.py <lua dir> <jobidentity.lub> <skill_db.yml>
"""

import re
import sys

import gen


def server_skills(path):
    """Id, name, max level and SP per level, out of rAthena's skill_db."""
    skills, cur, in_sp, level = {}, None, False, None

    for line in open(path, encoding='utf-8'):
        found = re.match(r'^  - Id: (\d+)', line)
        if found:
            cur = {'Id': int(found.group(1)), 'sp': {}}
            skills[cur['Id']] = cur
            in_sp = False
            continue

        if cur is None:
            continue

        if re.match(r'^      SpCost:', line):
            in_sp = True
            continue

        if in_sp:
            at = re.match(r'^        - Level: (\d+)', line)
            if at:
                level = int(at.group(1))
                continue

            amount = re.match(r'^          Amount: (\d+)', line)
            if amount and level is not None:
                cur['sp'][level] = int(amount.group(1))
                continue

            if re.match(r'^ {4,6}\w', line):
                in_sp = False

        field = re.match(r'^    (Name|MaxLevel): (.+?)\s*$', line)
        if field:
            value = field.group(2)
            cur[field.group(1)] = int(value) if value.isdigit() else value

    return skills


def main():
    if len(sys.argv) != 4:
        print(__doc__, file=sys.stderr)
        return 2

    seed, info, _, _ = gen.read(sys.argv[1], sys.argv[2])
    names = {int(v): k for k, v in seed['SKID'].items()}
    server = server_skills(sys.argv[3])

    client_ids = {int(k) for k in info}
    both = sorted(client_ids & set(server))

    print(f"client {len(client_ids)} skills, server {len(server)}, in both {len(both)}")
    print(f"  client only: {len(client_ids - set(server))}")
    print(f"  server only: {len(set(server) - client_ids)}")

    levels, costs = [], []
    for skill in both:
        entry = info[float(skill)]

        theirs = server[skill].get('MaxLevel')
        ours = entry.get('MaxLv')
        if theirs is not None and ours is not None and int(ours) != theirs:
            levels.append((skill, int(ours), theirs))

        sp = entry.get('SpAmount')
        if isinstance(sp, dict):
            for level, amount in server[skill]['sp'].items():
                ours = sp.get(level)
                if ours is not None and int(ours) not in (0, amount):
                    costs.append((skill, level, int(ours), amount))
                    break

    print(f"\nmax level disagrees on {len(levels)} skills")
    for skill, ours, theirs in levels[:15]:
        print(f"  {skill:<6}{names.get(skill, '?'):<24}client {ours:<4}server {theirs}")

    print(f"\nSP cost disagrees on {len(costs)} skills")
    for skill, level, ours, theirs in costs[:15]:
        print(f"  {skill:<6}{names.get(skill, '?'):<24}lv{level:<3}client {ours:<5}server {theirs}")

    print("\nThe server is the one to believe for both: it is what refuses the "
          "cast and what charges the SP.")

    return 0


if __name__ == "__main__":
    sys.exit(main())
