#!/usr/bin/env bash
#
# agreement/cryptoVerifier_test.go: BenchmarkCryptoVerifierVoteVertification
# reads results from verifier.Verified(protocol.AgreementVoteTag), but vote
# results moved to their own channel, VerifiedVotes(), and Verified panics on
# the vote tag ("Verified called on bad type"). The panic takes the whole
# agreement test binary down. Behind that, the vote is built for round 300
# against a fixture ledger at round 1, which fails the seed lookup; the
# working bundle benchmark uses the ledger's round, so this does too.
#
# The same change is a commit on the bench/fix-broken-benchmarks branch of the
# go-algorand clone, for an upstream PR. This edits the throwaway checkout only.
set -eu

: "${CHECKOUT:?}"
FILE="$CHECKOUT/agreement/cryptoVerifier_test.go"

if [ ! -f "$FILE" ]; then
  echo "skip: $FILE does not exist at this ref"
  exit 0
fi
if ! grep -q 'func (c \*poolCryptoVerifier) VerifiedVotes()' "$CHECKOUT/agreement/cryptoVerifier.go"; then
  echo "skip: VerifiedVotes does not exist at this ref, Verified still handles votes"
  exit 0
fi
if ! grep -qF 'c := verifier.Verified(protocol.AgreementVoteTag)' "$FILE"; then
  echo "skip: benchmark already reads VerifiedVotes or differs"
  exit 0
fi

perl -0pi -e 's{c := verifier\.Verified\(protocol\.AgreementVoteTag\)\n\n\tsenderIdx := findSender\(ledger, basics\.Round\(300\), 0, 0, addresses, selections\)\n\trequest := cryptoVoteRequest\{message: makeMessage\(0, protocol\.AgreementVoteTag, addresses\[senderIdx\], ledger, selections\[senderIdx\], votings\[senderIdx\], 300, 0, 0\), Round: ledger\.NextRound\(\)\}}{c := verifier.VerifiedVotes() // patched by benchspotter\n\n\tround := ledger.NextRound()\n\tsenderIdx := findSender(ledger, round, 0, 0, addresses, selections)\n\trequest := cryptoVoteRequest{message: makeMessage(0, protocol.AgreementVoteTag, addresses[senderIdx], ledger, selections[senderIdx], votings[senderIdx], round, 0, 0), Round: round}}' "$FILE"
grep -q 'c := verifier.VerifiedVotes' "$FILE" || { echo "patch did not apply: benchmark shape differs" >&2; exit 1; }
grep -n 'c := verifier.VerifiedVotes\|round := ledger.NextRound' "$FILE"
