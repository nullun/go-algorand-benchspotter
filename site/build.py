# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///
"""Build the static trend site from the benchspotter store.

Reads .benchspotter/sessions/<uuid>/{meta.json,results.bench} and writes
site/dist/{index.html,data.json}. Nothing here needs the benchspotter binary,
so GitHub Pages can build from a plain checkout.

Usage: uv run site/build.py [--store DIR] [--out DIR]
"""

import argparse
import json
import re
import shutil
import statistics
import sys
from datetime import datetime, timezone
from pathlib import Path

# Go prints the benchmark name with the GOMAXPROCS it ran under: "BenchmarkTxn-12".
PROCS = re.compile(r"-\d+$")
# "BenchmarkTxn-12 46232 25878 ns/op 3361 B/op 6 allocs/op", values possibly 1.29e+06.
NUMBER = re.compile(r"^[0-9.eE+-]+$")


def session_time(session_id):
    """benchspotter ids are UUIDv7: the first 48 bits are the creation time in ms."""
    ms = int(session_id.replace("-", "")[:12], 16)
    return datetime.fromtimestamp(ms / 1000, tz=timezone.utc)


MODULE = "github.com/algorand/go-algorand/"


def parse_bench(path):
    """Return {"pkg: BenchmarkName": {metric: [values]}} from a Go benchmark output file.

    go test prints "pkg: <import path>" before each package's lines, and a bare
    name is ambiguous (BenchmarkSign exists in crypto and crypto/secp256k1), so
    the package is part of the key.
    """
    out = {}
    pkg = "?"
    for line in path.read_text(errors="replace").splitlines():
        fields = line.split()
        if len(fields) == 2 and fields[0] == "pkg:":
            pkg = fields[1].removeprefix(MODULE)
            continue
        if len(fields) < 4 or not fields[0].startswith("Benchmark"):
            continue
        name = f"{pkg}: {PROCS.sub('', fields[0])}"
        # fields[1] is the iteration count, then (value, unit) pairs.
        rest = fields[2:]
        metrics = out.setdefault(name, {})
        for value, unit in zip(rest[::2], rest[1::2]):
            if not NUMBER.match(value):
                continue
            try:
                metrics.setdefault(unit, []).append(float(value))
            except ValueError:
                pass
    return out


def load_sessions(store):
    sessions = []
    for d in sorted((store / "sessions").iterdir()):
        meta_path, bench_path = d / "meta.json", d / "results.bench"
        if not meta_path.exists() or not bench_path.exists():
            continue
        meta = json.loads(meta_path.read_text())
        machine = meta.get("machine") or {}
        tags = meta.get("tags") or []
        # commit:<utc timestamp>, set by the scripts (lib/common.sh). A session
        # without one (none should remain) sorts by the time it was recorded.
        commit_time = next((t[len("commit:"):] for t in tags if t.startswith("commit:")), None)
        sessions.append(
            {
                "id": d.name,
                "time": session_time(d.name).isoformat(),
                "commit_time": commit_time,
                "name": meta.get("name", d.name),
                "commit": (meta.get("git_commit") or "")[:10],
                "tags": tags,
                "notes": meta.get("notes", ""),
                "cpu": machine.get("cpu", "?"),
                "platform": f"{machine.get('goos', '?')}/{machine.get('goarch', '?')}",
                "results": parse_bench(bench_path),
            }
        )
    # Commit order, with record time breaking ties (the RUNS repeats of one
    # commit). The page can re-sort by record time; the series arrays are
    # indexed by position in this list either way.
    sessions.sort(key=lambda s: (s["commit_time"] or s["time"], s["time"]))
    return sessions


def load_notes(path):
    """{BenchmarkName: note} from the trailing comments in lib/benches.txt."""
    notes = {}
    if not path.exists():
        return notes
    for line in path.read_text().splitlines():
        code, _, note = line.partition("#")
        if code.split() and note.strip():
            notes[code.split()[0]] = note.strip()
    return notes


def build(store, out):
    sessions = load_sessions(store)
    notes = load_notes(Path(__file__).resolve().parent.parent / "lib" / "benches.txt")
    if not sessions:
        sys.exit(f"no sessions found under {store}")

    names = sorted({name for s in sessions for name in s["results"]})
    units = {}
    for name in names:
        for s in sessions:
            for unit in s["results"].get(name, {}):
                units.setdefault(name, set()).add(unit)

    # One value per session per metric: the median of the -count repeats, which
    # is what benchspotter's own trend view reports. The range of those repeats
    # goes alongside, because on the hosted runner the spread inside one process
    # is two to four times the spread between the RUNS processes: a band drawn
    # from run medians alone would understate the noise.
    series, ranges = {}, {}
    for name in names:
        series[name], ranges[name] = {}, {}
        for unit in sorted(units.get(name, ())):
            meds, rng = [], []
            for s in sessions:
                vals = s["results"].get(name, {}).get(unit)
                meds.append(round(statistics.median(vals), 6) if vals else None)
                rng.append([round(min(vals), 6), round(max(vals), 6), len(vals)] if vals else None)
            series[name][unit] = meds
            ranges[name][unit] = rng

    # A series that no longer runs (MerkleCommit/sha256, say) still has years of
    # history worth reading, so keep it and let the page label it.
    retired = sorted(n for n in names if series[n] and
                     all(vals[-1] is None for vals in series[n].values()))

    data = {
        "retired": retired,
        # What each benchmark measures, from lib/benches.txt, keyed like series.
        # A sub-benchmark takes its parent's note.
        "notes": {n: notes[n.split(": ", 1)[1].split("/")[0]]
                  for n in names if n.split(": ", 1)[1].split("/")[0] in notes},
        "generated": datetime.now(tz=timezone.utc).isoformat(timespec="seconds"),
        "sessions": [{k: v for k, v in s.items() if k != "results"} for s in sessions],
        "series": series,
        # [min, max, samples] per session, same indexing as series.
        "ranges": ranges,
    }

    out.mkdir(parents=True, exist_ok=True)
    (out / "data.json").write_text(json.dumps(data, separators=(",", ":")))
    shutil.copyfile(Path(__file__).parent / "index.html", out / "index.html")
    print(f"{len(sessions)} sessions, {len(names)} benchmarks -> {out}")


if __name__ == "__main__":
    here = Path(__file__).resolve().parent
    ap = argparse.ArgumentParser()
    ap.add_argument("--store", type=Path, default=here.parent / ".benchspotter")
    ap.add_argument("--out", type=Path, default=here / "dist")
    args = ap.parse_args()
    build(args.store, args.out)
