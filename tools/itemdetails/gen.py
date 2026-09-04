#!/usr/bin/env python3
"""Generate what an item's information window shows, from rAthena's item database.

The inventory packet carries an id, a count and whether a thing is worn. That
is enough to draw a grid of icons and nothing else: a player looking at a
Guard cannot see that it is armour, that it weighs thirty, that it goes in the
left hand or that it can be refined.

The rest is in the server's own database. Not in the archive: idnum2itemdesctable
is Korean, the same way the skill descriptions are, so the prose the original
draws is no use under an English name. What rAthena has instead is every
figure, in English, for the same server tree the client is talking to — which
is where names.txt already comes from, so the two stay in step.

Written as its own data file rather than as more columns on names.txt: the
names are read every time the inventory is drawn and this is read when
somebody asks about one item, so they are parsed separately and this one only
when it is wanted.

    python3 tools/itemdetails/gen.py <rathena>/db/re/item_db_*.yml \
        > internal/game/items/details.txt
"""

import re
import sys

# The fields worth carrying, in the order they are written.
#
# Everything here is a figure or a word the window puts on a line. What is
# left out is the Script — it is source code rather than prose, and a player
# reading "itemheal rand(45,65),0" has learned nothing they could not see.
SCALARS = {
    "Type": "type",
    "SubType": "subtype",
    "Weight": "weight",
    "Attack": "attack",
    "Defense": "defense",
    "Range": "range",
    "Slots": "slots",
    "WeaponLevel": "level",
    "ArmorLevel": "level",
    "EquipLevelMin": "minlevel",
    "Buy": "buy",
    "Refineable": "refineable",
}

# The two blocks that are sets of names rather than one value.
SETS = {"Jobs": "jobs", "Locations": "locations"}

COLUMNS = [
    "type", "subtype", "weight", "attack", "defense", "range", "slots",
    "level", "minlevel", "refineable", "locations", "jobs", "buy",
]


def indent_of(line):
    return len(line) - len(line.lstrip(" "))


def read_items(paths):
    """Every item in the database, as {id: {field: value}}.

    A reader for the shape these files are in rather than for YAML: two-space
    indents, no anchors, no flow style. What it has to keep track of is the
    two blocks whose contents are a set of names — who may wear a thing and
    where it goes — since those are nested a level deeper than the rest.
    """
    items = {}

    current = None
    block = None

    for path in paths:
        with open(path, encoding="utf-8") as handle:
            for line in handle:
                if not line.strip() or line.lstrip().startswith("#"):
                    continue

                indent = indent_of(line)
                text = line.strip()

                if match := re.match(r"-\s+Id:\s*(\d+)\s*$", text):
                    current = {}
                    block = None
                    items[int(match.group(1))] = current
                    continue

                if current is None:
                    continue

                # Out of a set the moment the indent comes back to a field.
                if indent <= 4:
                    block = None

                if block is not None:
                    if match := re.match(r"(\w+):\s*true\s*$", text):
                        current.setdefault(block, []).append(match.group(1))

                    continue

                if indent != 4:
                    continue

                if match := re.match(r"(\w+):\s*$", text):
                    if field := SETS.get(match.group(1)):
                        block = field

                    continue

                if match := re.match(r"(\w+):\s*(.+?)\s*$", text):
                    if field := SCALARS.get(match.group(1)):
                        current[field] = match.group(2).strip("\"'")

    return items


def cell(value):
    """One field, with the empties written as nothing rather than as zero.

    A file where every consumable carries seven noughts is a megabyte of
    noughts; a file where they are blank is the same table and half the size.
    """
    if value in (None, "", "0", 0, "false"):
        return ""

    if value == "true":
        return "1"

    if isinstance(value, list):
        return ",".join(value)

    return str(value).replace("\t", " ")


def main() -> int:
    if len(sys.argv) < 2:
        print(__doc__, file=sys.stderr)
        return 2

    items = read_items(sys.argv[1:])
    if not items:
        print("no items found — are those item_db yml files?", file=sys.stderr)
        return 1

    written = 0

    for item_id in sorted(items):
        entry = items[item_id]

        # Jobs listed one by one only when the item is fussy about them. An
        # absent Jobs block means anybody may wear it, and writing out every
        # job for every sword is most of the file.
        fields = [cell(entry.get(name)) for name in COLUMNS]

        if not any(fields):
            continue

        print("%d\t%s" % (item_id, "\t".join(fields)))
        written += 1

    print("generated %d items of %d" % (written, len(items)), file=sys.stderr)

    return 0


if __name__ == "__main__":
    sys.exit(main())
