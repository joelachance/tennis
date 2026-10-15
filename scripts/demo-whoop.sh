#!/bin/sh
# Stages a WHOOP export into a searchable Tennis namespace, for recording a
# demo against real data instead of a fixture.
#
#   scripts/demo-whoop.sh [export.zip] [workdir]
#
# The export is WHOOP's "My Data" download: journal_entries, physiological_
# cycles, sleeps, and workouts. Nothing here is committed — the notes are
# written to a work directory and the database lives beside them.
#
# The raw CSVs are not indexed. Each row repeats every other row on the same
# day, so a search comes back as a wall of duplicated CSV and the actual note
# — the one readable sentence in the file — is buried. Grouping by day into
# one note per day puts the sentence first, where a search finds it.
set -eu

zip="${1:-$HOME/Documents/Health/my_whoop_data_2023_09_23.zip}"
work="${2:-/tmp/tennis-demo}"

[ -f "$zip" ] || { echo "demo: no export at $zip" >&2; exit 1; }
command -v tennis >/dev/null 2>&1 || { echo "demo: tennis not on PATH" >&2; exit 1; }

rm -rf "$work"
mkdir -p "$work/data" "$work/notes"
unzip -qo "$zip" -d "$work/data"

python3 - "$work" <<'PY'
import csv, collections, glob, os, sys

work = sys.argv[1]
rows = list(csv.DictReader(open(os.path.join(work, "data/journal_entries.csv"))))

# One WHOOP cycle carries a row per question, and every row on that day
# repeats the same free-text note. Grouping by cycle keeps the note once and
# drops the duplication.
cycles = collections.defaultdict(list)
for r in rows:
    if r["Question text"].strip() or r["Notes"].strip():
        cycles[r["Cycle start time"]].append(r)

for start, rs in sorted(cycles.items()):
    lines = [f"# WHOOP journal — {start[:10]}", ""]
    notes = sorted({r["Notes"].strip() for r in rs if r["Notes"].strip()})
    if notes:
        lines += ["## Note", ""] + [f"- {n}" for n in notes] + [""]
    lines += ["## Answers", ""]
    seen = set()
    for r in sorted(rs, key=lambda r: r["Question text"]):
        q = r["Question text"].strip()
        if not q or q in seen:
            continue
        seen.add(q)
        lines.append(f"- {q}: **{r['Answered yes'] or 'n/a'}**")
    with open(os.path.join(work, "notes", f"{start[:10].replace('-', '')}.md"), "w") as fh:
        fh.write("\n".join(lines))

print(f"demo: {len(cycles)} days of notes written")
PY

db="$work/tennis.sqlite"
tennis add "$work/notes" --ns whoop --db "$db" --no-cards

printf '\n%d notes indexed in /tmp/tennis-demo. Try:\n' "$(find "$work/notes" -name '*.md' | wc -l | tr -d ' ')"
cat <<EOF
  tennis search "ski patrol"                     --ns whoop --db $db
  tennis search "journal entries mentioning a dog" --ns whoop --db $db
  tennis ls                                       --ns whoop --db $db
EOF