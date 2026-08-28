#!/usr/bin/env python3
"""Generate the item table: name, category and icon resource, by id.

Same reason as the skills table: this archive's item names are Korean, and
the inventory packet carries ids only. rAthena's item_db carries an English
Name per id, so these come from the server tree the client talks to.

Unlike skills there are tens of thousands of these, so they are written as a
data file the package embeds rather than as Go source — a map literal that
size is a megabyte of code for a reviewer to scroll past.

Three fields per item, from two sources:

  name      rAthena's item_db Name — English, which the archive is not.
  category  its Type, folded to the three tabs the inventory window has.
  resource  the icon's file name, from the archive's own
            idnum2itemresnametable.txt. Those are EUC-KR and the icon files
            are named with the same bytes, so they are decoded here and
            re-encoded when the texture is loaded.

    python3 tools/itemnames/gen.py <resnametable.txt> <rathena>/db/re/item_db_*.yml \
        > internal/game/items/names.txt
"""

import re
import sys


# Categories, folded from rAthena's Type to the tabs the window has. Anything
# that is not consumable and not worn is "etc", which is what the tab is for.
USABLE = {"Healing", "Usable", "DelayConsume", "Cash"}
EQUIP = {"Weapon", "Armor", "Shadowgear"}


def category(item_type: str) -> str:
    if item_type in USABLE:
        return "item"
    if item_type in EQUIP:
        return "equip"

    return "etc"


def read_resources(path: str) -> dict:
    """Read the archive's id to icon-name table.

    Lines are `<id>#<resource>#`, EUC-KR, with CRLF endings.
    """
    resources = {}

    with open(path, "rb") as handle:
        for raw in handle:
            line = raw.decode("euc-kr", errors="replace").strip()
            parts = line.split("#")
            if len(parts) < 2 or not parts[0].isdigit():
                continue

            resources[int(parts[0])] = parts[1]

    return resources


def main() -> int:
    if len(sys.argv) < 3:
        print(__doc__, file=sys.stderr)
        return 2

    resources = read_resources(sys.argv[1])

    entries = {}
    types = {}

    for path in sys.argv[2:]:
        item_id = None

        with open(path, encoding="utf-8") as handle:
            for line in handle:
                if match := re.match(r"\s*-\s+Id:\s*(\d+)\s*$", line):
                    item_id = int(match.group(1))
                    continue

                if item_id is None:
                    continue

                if match := re.match(r"\s*Type:\s*(\S+)\s*$", line):
                    if item_id in entries:
                        types[item_id] = match.group(1).strip('"').strip("'")
                    continue

                if match := re.match(r"\s*Name:\s*(.+?)\s*$", line):
                    name = match.group(1).strip('"').strip("'")
                    # Tabs are the field separator, so a name may not hold one.
                    entries[item_id] = name.replace("\t", " ")

    if not entries:
        print("no items found — are those item_db yml files?", file=sys.stderr)
        return 1

    for item_id in sorted(entries):
        print("%d\t%s\t%s\t%s" % (
            item_id,
            entries[item_id],
            category(types.get(item_id, "Etc")),
            resources.get(item_id, "").replace("\t", " "),
        ))

    print("generated %d items, %d with an icon" % (
        len(entries), sum(1 for i in entries if resources.get(i))), file=sys.stderr)

    return 0


if __name__ == "__main__":
    sys.exit(main())
