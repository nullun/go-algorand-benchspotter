# shellcheck shell=bash
#
# Shared helpers for the benchmark scripts. Everything here operates on a
# go-algorand checkout that this repo owns and is free to destroy: `git clean
# -xfdq` and `git checkout --detach` run in that directory, never in this one.
# That is the whole reason the results live in a separate repo.

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export BENCHSPOTTER_PATH="${BENCHSPOTTER_PATH:-$REPO_ROOT/.benchspotter}"
CHECKOUT="${CHECKOUT:-$REPO_ROOT/checkout/go-algorand}"
REMOTE="${REMOTE:-https://github.com/algorand/go-algorand.git}"
TOOLCHAIN="${TOOLCHAIN:-pinned}"
# PATH before any per-tag Go was put in front of it; bs_prepare restores it
# each time so one tag's Go 1.20 does not leak into the next tag.
BS_ORIG_PATH="$PATH"
# -benchtime for every benchmark. go test reads GOFLAGS, so benchspotter needs
# no flag for it. The Go default of 1s costs 2-4 s of ramp per -count repeat of
# every series and buys nothing for a ns-to-ms benchmark; 300ms gave the same
# ns/op within noise on all but the handful whose work scales with b.N, and
# those are patched or gone (see lib/benches.txt). Recorded as a session tag,
# benchtime:<value>, because a change here is a series break for the few that
# remain sensitive to it: check-steps.py compares within one value only and the
# site draws a rule where it changes.
BENCHTIME="${BENCHTIME:-300ms}"
export GOFLAGS="${GOFLAGS:+$GOFLAGS }-benchtime=$BENCHTIME"

bs_preflight() {
  command -v benchspotter >/dev/null || { echo "benchspotter not on PATH" >&2; return 1; }
  command -v jq >/dev/null || { echo "jq not on PATH" >&2; return 1; }
  mkdir -p "$BENCHSPOTTER_PATH"
  BENCHSPOTTER_PATH="$(cd "$BENCHSPOTTER_PATH" && pwd)"
  export BENCHSPOTTER_PATH
}

# bs_clone - make sure $CHECKOUT is a go-algorand clone with current refs.
# Kept between runs; the build cache and the clone are what make a master
# run take minutes instead of an hour.
bs_clone() {
  if [ ! -d "$CHECKOUT/.git" ]; then
    echo "cloning $REMOTE into $CHECKOUT"
    mkdir -p "$(dirname "$CHECKOUT")"
    git clone --quiet "$REMOTE" "$CHECKOUT" || return 1
  fi
  git -C "$CHECKOUT" fetch --quiet --tags --force origin || return 1
}

# bs_prepare <ref> <logdir> <logprefix> - check out ref, clean, pick the
# toolchain, build libsodium, patch. Echoes nothing; sets BS_GOVERSION, BS_COMMIT,
# BS_COMMIT_DATE, BS_DIRECTIVE.
bs_prepare() {
  local ref="$1" logdir="$2" prefix="$3"

  # --force discards the working tree edits patches/ made last time; without it
  # the previous run's patches survive into this one.
  git -C "$CHECKOUT" checkout --force --detach --quiet "$ref" || return 1
  # crypto/libs is the libsodium build, 40 s here and minutes on a 4-core
  # runner, and its only input is the libsodium-fork submodule commit. Keep it
  # across runs and drop it when that commit changes.
  local sodium stamp="$CHECKOUT/crypto/libs/.benchspotter-libsodium"
  sodium="$(git -C "$CHECKOUT" rev-parse "HEAD:crypto/libsodium-fork" 2>/dev/null)"
  if [ -z "$sodium" ] || [ "$(cat "$stamp" 2>/dev/null)" != "$sodium" ]; then
    rm -rf "$CHECKOUT/crypto/libs"
  fi
  git -C "$CHECKOUT" clean -xfdq -e crypto/libs

  BS_COMMIT="$(git -C "$CHECKOUT" rev-parse --short HEAD)"
  BS_COMMIT_DATE="$(git -C "$CHECKOUT" log -1 --format=%cs HEAD)"
  BS_COMMIT_TIME="$(bs_commit_time HEAD)"
  BS_CONSENSUS="$(bs_consensus_version HEAD)"

  # Which Go builds this ref. From v3.24.0 go.mod names it in a toolchain
  # directive. Before that the version upstream released with is BUILD= in
  # scripts/get_golang_version.sh (1.20.5 at v3.17.0, 1.20.14 at v3.23.1).
  # GOTOOLCHAIN can switch to any go1.21 or later by downloading it, but the
  # toolchain module has nothing older, so a Go 1.20 is installed through
  # golang.org/dl into ~/sdk and put in front of PATH for this ref only.
  PATH="$BS_ORIG_PATH"
  unset GOTOOLCHAIN
  BS_DIRECTIVE="$(awk '/^toolchain /{print $2}' "$CHECKOUT/go.mod")"
  local want=""
  if [ "$TOOLCHAIN" = pinned ]; then
    want="${BS_DIRECTIVE:-$(bs_build_go_version)}"
  fi
  if [ -z "$want" ]; then
    # NOTE: GOTOOLCHAIN=auto only ever steps UP. With a local Go newer than the
    # go.mod directive every ref builds with the local Go, which is fine for the
    # master series and wrong for a historical sweep.
    :
  elif bs_version_ge "$want" go1.21.0; then
    export GOTOOLCHAIN="$want"
  else
    bs_install_go "$want" "$logdir/$prefix.goinstall.log" || return 1
    PATH="$HOME/sdk/$want/bin:$PATH"
  fi

  BS_GOVERSION="$(cd "$CHECKOUT" && go version 2>"$logdir/$prefix.goversion.log" | awk '{print $3}')"
  [ -n "$BS_GOVERSION" ] || return 1

  # The crypto package needs libsodium; the make target is a no-op when the
  # archive kept above is still there.
  local os arch
  os="$(cd "$CHECKOUT" && ./scripts/ostype.sh)"
  arch="$(cd "$CHECKOUT" && ./scripts/archtype.sh)"
  make -C "$CHECKOUT" "crypto/libs/$os/$arch/lib/libsodium.a" \
    >"$logdir/$prefix.libsodium.log" 2>&1 || return 1
  [ -n "$sodium" ] && echo "$sodium" > "$stamp"

  # After the toolchain and libsodium: patches/zz-compile-gate.sh compiles
  # every package the patches touched, and that needs both.
  bs_apply_patches "$logdir/$prefix.patches.log" || return 1
}

# bs_build_go_version - the Go upstream built this ref with, as "go1.20.14",
# from scripts/get_golang_version.sh; empty when the script or BUILD= is absent.
bs_build_go_version() {
  local v
  v="$(sed -n 's/^[[:space:]]*BUILD=\([0-9][0-9.]*\).*/\1/p' "$CHECKOUT/scripts/get_golang_version.sh" 2>/dev/null | head -1)"
  [ -n "$v" ] && echo "go$v"
}

# bs_version_ge <a> <b> - true when Go version a is b or newer (go1.20.14 style).
bs_version_ge() {
  [ "$(printf '%s\n%s\n' "$1" "$2" | sort -V | head -1)" = "$2" ]
}

# bs_install_go <version> <log> - make sure ~/sdk/<version>/bin/go exists, using
# the golang.org/dl wrappers (go install golang.org/dl/go1.20.14@latest, then
# go1.20.14 download). A no-op once it is there, so a runner that keeps ~/sdk
# pays for it once. GOFLAGS is cleared: the -benchtime in it is for go test.
bs_install_go() {
  local v="$1" log="$2" sdk="$HOME/sdk/$1" gobin
  [ -x "$sdk/bin/go" ] && return 0
  echo "installing $v into $sdk via golang.org/dl"
  gobin="$(GOFLAGS='' go env GOPATH)/bin"
  GOFLAGS='' GOTOOLCHAIN=auto go install "golang.org/dl/$v@latest" >"$log" 2>&1 || { tail -3 "$log" >&2; return 1; }
  GOFLAGS='' "$gobin/$v" download >>"$log" 2>&1 || { tail -3 "$log" >&2; return 1; }
  [ -x "$sdk/bin/go" ]
}

# bs_apply_patches <log> - run every patches/*.sh against the checkout. They
# edit the working tree of a throwaway clone, so go-algorand itself needs no
# changes for any of this to work.
bs_apply_patches() {
  local log="$1" p
  : > "$log"
  for p in "$REPO_ROOT"/patches/*.sh; do
    [ -e "$p" ] || continue
    echo "--- $(basename "$p")" >> "$log"
    CHECKOUT="$CHECKOUT" bash "$p" >> "$log" 2>&1 || return 1
  done
}

# bs_commit_time <ref> - the committer time of ref in the checkout, as a UTC
# timestamp that sorts as a string: commit:2026-09-15T13:22:01Z is the tag.
bs_commit_time() {
  TZ=UTC git -C "$CHECKOUT" log -1 --date=format-local:%Y-%m-%dT%H:%M:%SZ --format=%cd "$1"
}

# bs_consensus_version <ref> - ConsensusCurrentVersion at ref, as "v42". A bump
# moves the ledger benchmarks at once (new rules, payouts, heartbeats), so the
# site draws a rule there, like it does for a toolchain change.
bs_consensus_version() {
  git -C "$CHECKOUT" show "$1:protocol/consensus.go" 2>/dev/null \
    | sed -n 's/^const ConsensusCurrentVersion = ConsensusV\([0-9][0-9]*\)$/v\1/p'
}

# bs_session_meta <id> <name> <note> [tag...] - the fields benchspotter cannot
# know: its own GoVersion field is runtime.Version() of the benchspotter binary,
# identical for every session, so the toolchain that built the code is a tag.
# The commit time is a tag too (commit:<utc timestamp>): a session records only
# the hash, and the site orders points by when the commit was made, not by when
# it was measured, so a tag measured late still lands where it belongs.
bs_session_meta() {
  local id="$1" name="$2" note="$3"; shift 3
  local t
  for t in "$@"; do benchspotter session tag "$id" "$t"; done
  [ -n "${BS_COMMIT_TIME:-}" ] && benchspotter session tag "$id" "commit:$BS_COMMIT_TIME"
  [ -n "${BS_CONSENSUS:-}" ] && benchspotter session tag "$id" "consensus:$BS_CONSENSUS"
  benchspotter session tag "$id" "benchtime:$BENCHTIME"
  benchspotter session note "$id" "$note"
}

# bs_last_session_id <name> - benchspotter keeps a partial session when one
# package fails but prints no id; find it by name.
bs_last_session_id() {
  benchspotter session ls -f json 2>/dev/null \
    | jq -r --arg n "$1" '[.[] | select(.name==$n)] | sort_by(.time) | last | .id // empty'
}
