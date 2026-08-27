#!/usr/bin/env python3
"""Generate the item id to name table from rAthena's item database.

Same reason as the skills table: this archive's item names are Korean, and
the inventory packet carries ids only. rAthena's item_db carries an English
Name per id, so these come from the server tree the client talks to.

Unlike skills there are tens of thousands of these, so they are written as a
data file the package embeds rather than as Go source — a map literal that
size is a megabyte of code for a reviewer to scroll past.

    python3 tools/itemnames/gen.py <rathena>/db/re/item_db_*.yml \
        > internal/game/items/names.txt
"""

import re
import sys


def main() -> int:
    if len(sys.argv) < 2:
        print(__doc__, file=sys.stderr)
        return 2

    entries = {}

    for path in sys.argv[1:]:
        item_id = None

        with open(path, encoding="utf-8") as handle:
            for line in handle:
                if match := re.match(r"\s*-\s+Id:\s*(\d+)\s*$", line):
                    item_id = int(match.group(1))
                    continue

                if item_id is None:
                    continue

                if match := re.match(r"\s*Name:\s*(.+?)\s*$", line):
                    name = match.group(1).strip('"').strip("'")
                    # Tabs are the field separator, so a name may not hold one.
                    entries[item_id] = name.replace("\t", " ")
                    item_id = None

    if not entries:
        print("no items found — are those item_db yml files?", file=sys.stderr)
        return 1

    for item_id in sorted(entries):
        print("%d\t%s" % (item_id, entries[item_id]))

    print("generated %d items" % len(entries), file=sys.stderr)

    return 0


if __name__ == "__main__":
    sys.exit(main())
