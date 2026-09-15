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


def parse_bench(path):
    """Return {benchmark name: {metric: [values]}} from a Go benchmark output file."""
    out = {}
    for line in path.read_text(errors="replace").splitlines():
        fields = line.split()
        if len(fields) < 4 or not fields[0].startswith("Benchmark"):
            continue
        name = PROCS.sub("", fields[0])
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
        sessions.append(
            {
                "id": d.name,
                "time": session_time(d.name).isoformat(),
                "name": meta.get("name", d.name),
                "commit": (meta.get("git_commit") or "")[:10],
                "tags": meta.get("tags") or [],
                "notes": meta.get("notes", ""),
                "cpu": machine.get("cpu", "?"),
                "platform": f"{machine.get('goos', '?')}/{machine.get('goarch', '?')}",
                "results": parse_bench(bench_path),
            }
        )
    sessions.sort(key=lambda s: s["time"])
    return sessions


def build(store, out):
    sessions = load_sessions(store)
    if not sessions:
        sys.exit(f"no sessions found under {store}")

    names = sorted({name for s in sessions for name in s["results"]})
    units = {}
    for name in names:
        for s in sessions:
            for unit in s["results"].get(name, {}):
                units.setdefault(name, set()).add(unit)

    # One point per session per metric: the median of the -count repeats, which
    # is what benchspotter's own trend view reports.
    series = {}
    for name in names:
        series[name] = {
            unit: [
                round(statistics.median(s["results"][name][unit]), 6)
                if name in s["results"] and unit in s["results"][name]
                else None
                for s in sessions
            ]
            for unit in sorted(units.get(name, ()))
        }

    # A series that no longer runs (MerkleCommit/sha256, say) still has years of
    # history worth reading, so keep it and let the page label it.
    retired = sorted(n for n in names if series[n] and
                     all(vals[-1] is None for vals in series[n].values()))

    data = {
        "retired": retired,
        "generated": datetime.now(tz=timezone.utc).isoformat(timespec="seconds"),
        "sessions": [{k: v for k, v in s.items() if k != "results"} for s in sessions],
        "series": series,
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
