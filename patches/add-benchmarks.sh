#!/usr/bin/env bash
#
# Copy the benchmarks this repo carries (benchmarks/<package path>/*_test.go)
# into the checkout. They are in-package tests named benchspotter_*_test.go,
# each carrying go-algorand's licence header, so upstreaming one is a copy and
# a PR. They never overwrite an upstream file.
#
# A benchmark written against master uses internal APIs that an old tag may
# not have. patches/zz-compile-gate.sh, which runs after every other patch,
# compiles each touched package and removes the added files where they do not
# build, so that tag simply does not get those series.
set -u

: "${CHECKOUT:?}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC="$REPO_ROOT/benchmarks"

if [ ! -d "$SRC" ]; then
  echo "skip: no benchmarks/ directory"
  exit 0
fi

pkgs=""
copied=0
while IFS= read -r f; do
  rel="${f#"$SRC"/}"
  dir="$(dirname "$rel")"
  if [ ! -d "$CHECKOUT/$dir" ]; then
    echo "skip $rel: package $dir does not exist at this ref"
    continue
  fi
  if [ -e "$CHECKOUT/$rel" ] && git -C "$CHECKOUT" ls-files --error-unmatch "$rel" >/dev/null 2>&1; then
    echo "skip $rel: upstream has a file of that name"
    continue
  fi
  cp "$f" "$CHECKOUT/$rel"
  copied=$((copied + 1))
  case " $pkgs " in *" $dir "*) ;; *) pkgs="$pkgs $dir" ;; esac
done < <(find "$SRC" -name '*_test.go' | sort)
echo "copied $copied benchmark files into $(echo $pkgs | wc -w | tr -d ' ') packages"
