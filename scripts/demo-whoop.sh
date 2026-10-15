#!/bin/sh
# Stages a WHOOP export as plain notes, ready to index, for recording a demo
# against real data instead of a fixture.
#
#   scripts/demo-whoop.sh [export.zip] [notesdir]
#
# The export is WHOOP's "My Data" download: journal_entries, physiological_
# cycles, sleeps, and workouts. Nothing is committed and nothing is indexed —
# this only writes the notes, so that the "tennis add" you type on camera is
# a real one against your real database.
#
# The raw CSVs are not indexed. Each row repeats every other row on the same
# day, so a search comes back as a wall of duplicated CSV and the actual note
# — the one readable sentence in the file — is buried. Grouping by day into
# one note per day puts the sentence first, where a search finds it.
set -eu

zip="${1:-$HOME/Documents/Health/my_whoop_data_2023_09_23.zip}"
notes="${2:-$HOME/Documents/Health/whoop-notes}"

[ -f "$zip" ] || { echo "demo: no export at $zip" >&2; exit 1; }
command -v tennis >/dev/null 2>&1 || { echo "demo: tennis not on PATH" >&2; exit 1; }

rm -rf "$notes"
mkdir -p "$notes/.data"
unzip -qo "$zip" -d "$notes/.data"

python3 - "$notes" <<'PY'
import csv, collections, glob, os, sys

# Not "notes": the loop below binds that name to the day's note lines.
outdir = sys.argv[1]
rows = list(csv.DictReader(open(os.path.join(outdir, ".data/journal_entries.csv"))))

# One WHOOP cycle carries a row per question, and every row on that day
# repeats the same free-text note. Grouping by cycle keeps the note once and
# drops the duplication.
cycles = collections.defaultdict(list)
for r in rows:
    if r["Question text"].strip() or r["Notes"].strip():
        cycles[r["Cycle start time"]].append(r)

written = 0
for start, rs in sorted(cycles.items()):
    # Only the days with something written in them. Every other day is 22
    # identical boolean answers with an empty note, and indexing those buries
    # the handful of real sentences under a wall of "false".
    notes = sorted({r["Notes"].strip() for r in rs if r["Notes"].strip()})
    if not notes:
        continue
    # Just the note. The 22 yes/no questions on the same row are data, but
    # they are not prose, and a whole file of "false" puts every real
    # sentence below the fold in a recording.
    lines = [f"# WHOOP journal — {start[:10]}", ""]
    lines += [f"- {n}" for n in notes]
    written += 1
    with open(os.path.join(outdir, f"{start[:10].replace('-', '')}.md"), "w") as fh:
        fh.write("\n".join(lines))

print(f"demo: {len(cycles)} cycles, {written} with a written note")
PY

# Staging only. The indexing is left to you to type, because tennis add is the
# thing worth showing — this just gets the export into a shape worth searching.
rm -rf "$notes/.data"

printf '\n%d notes staged in %s. Now index and search them:\n' \
  "$(find "$notes" -name '*.md' | wc -l | tr -d ' ')" "$notes"
cat <<EOF
  tennis add ~/Documents/Health/whoop-notes --ns whoop
  tennis search "ski patrol" --ns whoop
EOF