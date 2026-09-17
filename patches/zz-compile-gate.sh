#!/usr/bin/env bash
#
# Runs last (the zz- prefix): make sure every package the other patches
# touched still compiles at this ref, and undo what does not.
#
# A patch written against master can meet an older tag where the code it
# relies on differs. benchspotter aborts the whole session when one package's
# go test fails, so one such package would cost the run. Every package with a
# modified or added Go file gets its test binary compiled (nothing runs). If
# that fails, first the added files are removed and the package compiled
# again, since the added benchmarks are the usual culprit and the edits to
# upstream files usually still hold; if it still fails, the package's upstream
# files are restored too. Either way the reason is logged and the tag simply
# does not get those series, the same as a benchmark name that does not exist
# there.
#
# This needs the toolchain chosen and libsodium built, which is why
# bs_prepare (lib/common.sh) applies patches after both.
set -u

: "${CHECKOUT:?}"
cd "$CHECKOUT"

dirs="$(git status --porcelain --untracked-files=all \
  | awk '{print $2}' | grep '\.go$' | xargs -n1 dirname 2>/dev/null | sort -u)"
if [ -z "$dirs" ]; then
  echo "nothing to check"
  exit 0
fi

compiles() { go test -c -o /dev/null "./$1" >"$2" 2>&1; }

kept=0; trimmed=0; reverted=0
for d in $dirs; do
  log="$(mktemp)"
  if compiles "$d" "$log"; then
    kept=$((kept + 1))
  else
    echo "$d does not compile at this ref:"
    grep -v '^#' "$log" | head -4 | sed 's/^/    /'
    git clean -fq -- "$d" >/dev/null
    if compiles "$d" "$log"; then
      echo "  dropped the added files in $d; the edits to upstream files stand"
      trimmed=$((trimmed + 1))
    else
      git checkout --quiet -- "$d"
      echo "  restored $d to upstream entirely"
      reverted=$((reverted + 1))
    fi
  fi
  rm -f "$log"
done
echo "compile gate: $kept packages kept, $trimmed trimmed to upstream edits, $reverted restored"
