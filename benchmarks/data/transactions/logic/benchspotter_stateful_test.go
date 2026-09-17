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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/algorand/go-algorand/data/basics"
	"github.com/algorand/go-algorand/data/transactions"
)

// bspStateReps is how many times a stateful opcode is repeated in one
// program, so that one EvalApp amortises the evaluation setup.
const bspStateReps = 1000

// bspStateEnv builds the sample app environment used throughout
// evalStateful_test.go and raises the opcode budget so that a program
// repeating a stateful opcode bspStateReps times can run to completion.
func bspStateEnv() (*EvalParams, *transactions.Transaction, *Ledger) {
	ep, tx, ledger := makeSampleEnv()
	proto := *ep.Proto
	proto.MaxAppProgramCost = 1_000_000
	ep.Proto = &proto
	return ep, tx, ledger
}

// bspBenchmarkStateOp follows benchmarkOperation in eval_test.go: it builds
// a program that repeats operation bspStateReps times and runs it b.N /
// bspStateReps times through EvalApp, so ns/op is the cost of one repetition.
// The number of instructions in operation besides the one under test is
// reported as extra/op.
func bspBenchmarkStateOp(b *testing.B, ep *EvalParams, ledger *Ledger, operation string) {
	b.Helper()
	inst := strings.Count(operation, ";") + strings.Count(operation, "\n")
	program := testProg(b, strings.Repeat(operation+"\n", bspStateReps)+"int 1", LogicVersion).Program
	runs := b.N / bspStateReps
	final := program
	if b.N%bspStateReps != 0 {
		runs++
		final = testProg(b, strings.Repeat(operation+"\n", b.N%bspStateReps)+"int 1", LogicVersion).Program
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < runs; i++ {
		p := program
		if i == runs-1 {
			p = final
		}
		ep.reset()
		ledger.Reset()
		pass, err := EvalApp(p, 0, 888, ep)
		if err != nil || !pass {
			require.NoError(b, err)
			require.True(b, pass)
		}
	}
	b.ReportMetric(float64(inst), "extra/op")
}

// BenchmarkStateOps times the stateful opcodes that read and write app
// global and local state, and the *_params_get / asset_holding_get lookups,
// against the in-memory test Ledger.
func BenchmarkStateOps(b *testing.B) {
	ep, tx, ledger := bspStateEnv()

	// Order matters: NewAccount replaces the balance record, so it comes
	// before the holding and the local state are attached to the Sender.
	ledger.NewAccount(tx.Sender, 1_000_000)
	ledger.NewApp(tx.Sender, 888, basics.AppParams{
		StateSchemas: basics.StateSchemas{
			GlobalStateSchema: basics.StateSchema{NumUint: 4, NumByteSlice: 4},
			LocalStateSchema:  basics.StateSchema{NumUint: 4, NumByteSlice: 4},
		},
	})
	ledger.NewGlobal(888, "gk", 1)
	ledger.NewLocals(tx.Sender, 888)
	ledger.NewLocal(tx.Sender, 888, "lk", 1)

	// Asset 55 is in the sample txn's ForeignAssets; app 56 is in ForeignApps.
	ledger.NewAsset(tx.Receiver, 55, basics.AssetParams{Total: 1_000_000, UnitName: "tok"})
	ledger.NewHolding(tx.Sender, 55, 10, false)
	ledger.NewApp(tx.Receiver, 56, basics.AppParams{
		ApprovalProgram:   testProg(b, "int 1", LogicVersion).Program,
		ClearStateProgram: testProg(b, "int 1", LogicVersion).Program,
		StateSchemas: basics.StateSchemas{
			GlobalStateSchema: basics.StateSchema{NumUint: 2, NumByteSlice: 1},
		},
	})

	// `global OpcodeBudget` yields a different value on every repetition, so
	// the puts are real writes rather than same-value no-ops.
	b.Run("app_global_get", func(b *testing.B) {
		bspBenchmarkStateOp(b, ep, ledger, `byte "gk"; app_global_get; pop`)
	})
	b.Run("app_global_put", func(b *testing.B) {
		bspBenchmarkStateOp(b, ep, ledger, `byte "gk"; global OpcodeBudget; app_global_put`)
	})
	b.Run("app_global_del", func(b *testing.B) {
		bspBenchmarkStateOp(b, ep, ledger, `byte "gk"; app_global_del`)
	})
	b.Run("app_local_get", func(b *testing.B) {
		bspBenchmarkStateOp(b, ep, ledger, `int 0; byte "lk"; app_local_get; pop`)
	})
	b.Run("app_local_put", func(b *testing.B) {
		bspBenchmarkStateOp(b, ep, ledger, `int 0; byte "lk"; global OpcodeBudget; app_local_put`)
	})
	b.Run("app_local_del", func(b *testing.B) {
		bspBenchmarkStateOp(b, ep, ledger, `int 0; byte "lk"; app_local_del`)
	})
	b.Run("asset_holding_get", func(b *testing.B) {
		bspBenchmarkStateOp(b, ep, ledger, `int 0; int 55; asset_holding_get AssetBalance; pop; pop`)
	})
	b.Run("asset_params_get", func(b *testing.B) {
		bspBenchmarkStateOp(b, ep, ledger, `int 55; asset_params_get AssetTotal; pop; pop`)
	})
	b.Run("app_params_get", func(b *testing.B) {
		bspBenchmarkStateOp(b, ep, ledger, `int 56; app_params_get AppGlobalNumUint; pop; pop`)
	})
	b.Run("acct_params_get", func(b *testing.B) {
		bspBenchmarkStateOp(b, ep, ledger, `int 0; acct_params_get AcctBalance; pop; pop`)
	})
}
