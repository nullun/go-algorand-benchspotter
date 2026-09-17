# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///
"""Flag steps between the newest benchmark point and the points before it.

A point is every session of one commit on one runner (the RUNS repeats), as on
the site. Points are in commit order. The point checked is the one measured
most recently, and it is compared with the --history points before it in
history of the same kind (nightly against nightlies, release against releases)
on the same runner, so an old tag measured late is judged against its
neighbours, not against the newest nightlies. A step counts when it is
larger than --min-pct and larger than --spread-factor times the widest spread
(over every repeat of every run) seen in either the new point or the baseline
points, so a
noisy series has to move further than a quiet one before anything is said.

The two NoiseSentinel* series do not depend on go-algorand (see
patches/add-noise-sentinel.sh). If one of them stepped too, every other step is
marked as probably the machine.

Prints a Markdown report. Exit status is 0 whether or not anything stepped;
--fail-on-step makes it 3 when a step was found (1 and 2 stay what Python and
argparse mean by them), for a workflow that wants to open an issue on it.

Usage: uv run lib/check-steps.py [--store DIR] [--runner TAG] [--history N]
"""

import argparse
import statistics
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent / "site"))
import build  # noqa: E402  site/build.py: session loading and bench parsing

SENTINEL = "NoiseSentinel"


def runner_of(session):
    return next((t for t in session["tags"] if t.startswith("runner:")), "runner:?")


KINDS = ("nightly", "release", "range")


def kind_of(session):
    return next((t for t in session["tags"] if t in KINDS), None)


def commit_key(s):
    return (s["commit_time"] or s["time"], s["time"])


def points(sessions, runner, kind):
    """[{label, commit, sessions}] for one runner and kind, in commit order."""
    out, seen = [], {}
    for s in sorted(sessions, key=commit_key):
        if runner_of(s) != runner or kind_of(s) != kind or "partial" in s["tags"]:
            continue
        if s["commit"] not in seen:
            seen[s["commit"]] = {"label": s["name"], "commit": s["commit"], "sessions": []}
            out.append(seen[s["commit"]])
        seen[s["commit"]]["sessions"].append(s)
    return out


def point_value(point, name, unit):
    """(median of the run medians, spread fraction over every sample) or None.

    The spread takes every -count repeat of every run, not just the run
    medians: on the hosted runner the spread inside one process is two to four
    times the spread between processes, so run medians alone understate the
    noise a step has to beat.
    """
    runs = [
        s["results"][name][unit]
        for s in point["sessions"]
        if name in s["results"] and unit in s["results"][name]
    ]
    if not runs:
        return None
    med = statistics.median(statistics.median(r) for r in runs)
    allv = [v for r in runs for v in r]
    spread = (max(allv) - min(allv)) / med if med else 0.0
    return med, spread


def check(store, runner, kind, history, min_pct, spread_factor, unit):
    sessions = build.load_sessions(store)
    if runner is None:
        runner = runner_of(sessions[-1])
    if kind is None:
        kind = kind_of(sessions[-1])
    pts = points(sessions, runner, kind)
    if len(pts) < 2:
        return runner, None, []
    # The point measured most recently, wherever its commit sits in history.
    i = max(range(len(pts)), key=lambda k: max(s["time"] for s in pts[k]["sessions"]))
    if i == 0:
        return runner, None, []
    new, base = pts[i], pts[max(0, i - history):i]

    rows = []
    for name in sorted({n for s in new["sessions"] for n in s["results"]}):
        cur = point_value(new, name, unit)
        prev = [v for v in (point_value(p, name, unit) for p in base) if v]
        if not cur or not prev:
            continue
        baseline = statistics.median(v for v, _ in prev)
        if not baseline:
            continue
        change = (cur[0] - baseline) / baseline * 100
        noise = max([cur[1]] + [s for _, s in prev]) * 100
        stepped = abs(change) > max(min_pct, spread_factor * noise)
        rows.append({
            "name": name.replace(": Benchmark", " "),
            "value": cur[0], "baseline": baseline, "change": change,
            "noise": noise, "stepped": stepped, "points": len(prev),
        })
    return runner, new, rows


def fmt(v, unit):
    if unit == "ns/op":
        for div, u in ((1e9, "s"), (1e6, "ms"), (1e3, "us")):
            if v >= div:
                return f"{v / div:.2f} {u}"
        return f"{v:.1f} ns"
    return f"{v:.4g} {unit}"


def report(runner, new, rows, unit):
    if new is None:
        return f"Not enough points on {runner} to compare.\n", False
    steps = [r for r in rows if r["stepped"]]
    machine = any(SENTINEL in r["name"] for r in steps)
    lines = [f"## {new['label']} ({new['commit']}) on {runner}", ""]
    if not steps:
        lines.append(f"No step in {len(rows)} series.")
    else:
        lines.append(f"{len(steps)} of {len(rows)} series stepped.")
        if machine:
            lines.append("")
            lines.append("**A noise sentinel stepped too, so the machine moved, not go-algorand.**")
        lines += ["", f"| benchmark | now | before | change | spread |", "|---|---:|---:|---:|---:|"]
        for r in sorted(steps, key=lambda r: -abs(r["change"])):
            lines.append(
                f"| {r['name']} | {fmt(r['value'], unit)} | {fmt(r['baseline'], unit)} "
                f"| {r['change']:+.1f}% | {r['noise']:.1f}% |"
            )
    lines.append("")
    return "\n".join(lines), bool(steps)


def main():
    here = Path(__file__).resolve().parent
    ap = argparse.ArgumentParser()
    ap.add_argument("--store", type=Path, default=here.parent / ".benchspotter")
    ap.add_argument("--runner", help="runner:<label> tag; default: the newest session's")
    ap.add_argument("--kind", choices=KINDS,
                    help="compare within nightly, release or range points; default: the newest session's")
    ap.add_argument("--history", type=int, default=7, help="baseline points (default 7)")
    ap.add_argument("--min-pct", type=float, default=5.0, help="smallest step reported")
    ap.add_argument("--spread-factor", type=float, default=2.0,
                    help="step must exceed this many times the run-to-run spread")
    ap.add_argument("--unit", default="ns/op")
    ap.add_argument("--fail-on-step", action="store_true")
    a = ap.parse_args()

    runner, new, rows = check(a.store, a.runner, a.kind, a.history, a.min_pct, a.spread_factor, a.unit)
    text, stepped = report(runner, new, rows, a.unit)
    print(text)
    sys.exit(3 if stepped and a.fail_on_step else 0)


if __name__ == "__main__":
    main()
