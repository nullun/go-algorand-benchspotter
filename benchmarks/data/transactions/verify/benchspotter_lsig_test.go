// Copyright (C) 2019-2026 Algorand Foundation Ltd.
// This file is part of go-algorand
//
// go-algorand is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// go-algorand is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with go-algorand.  If not, see <https://www.gnu.org/licenses/>.

// Added by go-algorand-benchspotter (benchmarks/data/transactions/verify); a candidate for upstream.

package verify

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/algorand/go-algorand/data/bookkeeping"
	"github.com/algorand/go-algorand/data/transactions/logic"
	"github.com/algorand/go-algorand/data/txntest"
	"github.com/algorand/go-algorand/protocol"
)

// BenchmarkLogicSigSanityCheck times LogicSigSanityCheck on the
// contract-account LogicSig in the TinyMan swap group: version and size
// checks, logic.CheckSignature over the program, and the comparison of
// the sender to the program hash. It does not evaluate the program.
func BenchmarkLogicSigSanityCheck(b *testing.B) {
	txns := txntest.CreateTinyManTxGroup(b, false)
	stxns, _ := txntest.CreateTinyManSignedTxGroup(b, txns)
	require.NotEmpty(b, stxns[1].Lsig.Logic)

	hdr := bookkeeping.BlockHeader{
		UpgradeState: bookkeeping.UpgradeState{
			CurrentProtocol: protocol.ConsensusCurrentVersion,
		},
	}
	groupCtx, err := PrepareGroupContext(stxns, &hdr, &logic.NoHeaderLedger{}, nil)
	require.NoError(b, err)
	require.NoError(b, LogicSigSanityCheck(1, groupCtx))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := LogicSigSanityCheck(1, groupCtx); err != nil {
			b.Fatal(err)
		}
	}
}
