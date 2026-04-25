#!/usr/bin/env bash
# Clone rAthena at the pinned SHA into ./build/rathena and stage our seed SQL.
# Idempotent — re-running fast-forwards to the pin and re-copies the seed.
# See docs/research/rathena-setup.md and RFC #49.

set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
PIN="$(tr -d '[:space:]' < "$HERE/pin.txt")"
BUILD="$HERE/build/rathena"
REPO_URL="https://github.com/rathena/rathena.git"

if [[ -z "$PIN" ]]; then
    echo "error: pin.txt is empty" >&2
    exit 1
fi

if [[ ! -d "$BUILD/.git" ]]; then
    echo "Cloning rAthena into $BUILD ..."
    git clone --quiet "$REPO_URL" "$BUILD"
fi

cd "$BUILD"
git fetch --quiet --tags origin
git checkout --quiet --detach "$PIN"

# Stage MVP seed SQL alongside upstream rAthena schema. Files run in
# alphabetical order on first MariaDB init; the `zzz_` prefix forces ours
# to run after upstream's `main.sql` / `logs.sql`.
cp "$HERE/seed/"*.sql "$BUILD/sql-files/"

echo
echo "rAthena pinned at $PIN"
echo "Next: cd $HERE && docker compose up"
