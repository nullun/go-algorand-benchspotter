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
)

// bspBoxReps is how many times a box operation is repeated in one program.
const bspBoxReps = 1000

// bspBenchmarkBoxOp builds a program repeating operation bspBoxReps times and
// runs it through EvalApp b.N / bspBoxReps times, so ns/op is the cost of one
// repetition. The ledger is not Reset between runs because the fixture box
// lives in the same mods layer that Reset clears; the operations are chosen
// to leave the boxes as they found them. Instructions in operation other
// than the one under test are reported as extra/op.
func bspBenchmarkBoxOp(b *testing.B, ep *EvalParams, operation string) {
	b.Helper()
	inst := strings.Count(operation, ";") + strings.Count(operation, "\n")
	program := testProg(b, strings.Repeat(operation+"\n", bspBoxReps)+"int 1", LogicVersion).Program
	runs := b.N / bspBoxReps
	final := program
	if b.N%bspBoxReps != 0 {
		runs++
		final = testProg(b, strings.Repeat(operation+"\n", b.N%bspBoxReps)+"int 1", LogicVersion).Program
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < runs; i++ {
		p := program
		if i == runs-1 {
			p = final
		}
		ep.reset()
		pass, err := EvalApp(p, 0, 888, ep)
		if err != nil || !pass {
			require.NoError(b, err)
			require.True(b, pass)
		}
	}
	b.ReportMetric(float64(inst), "extra/op")
}

// BenchmarkBoxOps times the box opcodes against a pre-populated 64 byte box
// named "self". The sample txn references boxes "self" and "other" for app
// 888, giving a 200 byte I/O budget in the test protocol, which every
// operation below stays within.
func BenchmarkBoxOps(b *testing.B) {
	ep, tx, ledger := makeSampleEnv()
	proto := *ep.Proto
	proto.MaxAppProgramCost = 1_000_000
	ep.Proto = &proto

	ledger.NewApp(tx.Sender, 888, basics.AppParams{})
	require.NoError(b, ledger.NewBox(888, "self", make([]byte, 64), appAddr(888)))

	b.Run("box_get", func(b *testing.B) {
		bspBenchmarkBoxOp(b, ep, `byte "self"; box_get; pop; pop`)
	})
	b.Run("box_len", func(b *testing.B) {
		bspBenchmarkBoxOp(b, ep, `byte "self"; box_len; pop; pop`)
	})
	b.Run("box_extract", func(b *testing.B) {
		bspBenchmarkBoxOp(b, ep, `byte "self"; int 8; int 16; box_extract; pop`)
	})
	b.Run("box_replace", func(b *testing.B) {
		bspBenchmarkBoxOp(b, ep, `byte "self"; int 8; byte 0x0102030405060708; box_replace`)
	})
	b.Run("box_splice", func(b *testing.B) {
		bspBenchmarkBoxOp(b, ep, `byte "self"; int 8; int 8; byte 0x0102030405060708; box_splice`)
	})
	b.Run("box_put", func(b *testing.B) {
		bspBenchmarkBoxOp(b, ep, `byte "self"; int 64; bzero; box_put`)
	})
	// Grow and shrink back so the box is 64 bytes again for the next run.
	b.Run("box_resize", func(b *testing.B) {
		bspBenchmarkBoxOp(b, ep, `byte "self"; int 72; box_resize; byte "self"; int 64; box_resize`)
	})
	// Create and delete "other" in the same step so the ledger is unchanged.
	b.Run("box_create_del", func(b *testing.B) {
		bspBenchmarkBoxOp(b, ep, `byte "other"; int 64; box_create; pop; byte "other"; box_del; pop`)
	})
}
