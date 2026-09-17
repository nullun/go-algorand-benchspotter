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

// Added by go-algorand-benchspotter (benchmarks/data/transactions/logic); a candidate for upstream.

package logic

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/algorand/go-algorand/data/basics"
)

// bspBenchmarkInnerTxn runs program (which submits one inner transaction)
// through EvalApp b.N times, resetting the EvalParams and the test Ledger's
// mods between runs so every run starts from the same state.
func bspBenchmarkInnerTxn(b *testing.B, ep *EvalParams, ledger *Ledger, source string) {
	b.Helper()
	program := testProg(b, source, LogicVersion).Program
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ep.reset()
		ledger.Reset()
		pass, err := EvalApp(program, 0, 888, ep)
		if err != nil || !pass {
			require.NoError(b, err)
			require.True(b, pass)
		}
	}
}

// BenchmarkInnerTxn times EvalApp end to end for an app that submits one
// inner payment, one inner asset transfer, or one inner app call, using the
// fixtures from evalAppTxn_test.go: app 888 funded with algos and asset 77,
// and app 56 (in the sample txn's ForeignApps) with a trivial approval
// program.
func BenchmarkInnerTxn(b *testing.B) {
	ep, tx, ledger := makeSampleEnv()

	const plenty = 1 << 50
	ledger.NewApp(tx.Receiver, 888, basics.AppParams{})
	ledger.NewAccount(appAddr(888), plenty)
	// tx.Receiver (txn Accounts 1) is the asset creator, so it holds asset 77.
	ledger.NewAsset(tx.Receiver, 77, basics.AssetParams{Total: plenty})
	ledger.NewHolding(appAddr(888), 77, plenty, false)
	ledger.NewApp(basics.Address{0x01}, 56, basics.AppParams{
		ApprovalProgram:   testProg(b, "int 1", 5).Program,
		ClearStateProgram: testProg(b, "int 1", 5).Program,
	})

	b.Run("pay", func(b *testing.B) {
		bspBenchmarkInnerTxn(b, ep, ledger, `
  itxn_begin
   int pay;                          itxn_field TypeEnum
   global CurrentApplicationAddress; itxn_field Sender
   txn Accounts 1;                   itxn_field Receiver
   int 100;                          itxn_field Amount
  itxn_submit
  int 1
`)
	})
	b.Run("axfer", func(b *testing.B) {
		bspBenchmarkInnerTxn(b, ep, ledger, `
  itxn_begin
   int axfer;                        itxn_field TypeEnum
   global CurrentApplicationAddress; itxn_field Sender
   int 77;                           itxn_field XferAsset
   txn Accounts 1;                   itxn_field AssetReceiver
   int 100;                          itxn_field AssetAmount
  itxn_submit
  int 1
`)
	})
	b.Run("appl", func(b *testing.B) {
		bspBenchmarkInnerTxn(b, ep, ledger, `
  itxn_begin
   int appl;                         itxn_field TypeEnum
   int 56;                           itxn_field ApplicationID
  itxn_submit
  int 1
`)
	})
}
