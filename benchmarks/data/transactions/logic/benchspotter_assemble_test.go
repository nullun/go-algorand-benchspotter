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

// This file is in package logic_test, like evalBench_test.go, because the
// TinyMan program lives in data/txntest, which imports logic.

package logic_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/algorand/go-algorand/data/transactions/logic"
	"github.com/algorand/go-algorand/data/txntest"
)

// BenchmarkAssemble times AssembleString on the TinyMan v4 logicsig, a
// fixed, medium-sized program with constant blocks, branches and labels.
func BenchmarkAssemble(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := logic.AssembleString(txntest.TmLsig)
		if err != nil {
			require.NoError(b, err)
		}
	}
}

// BenchmarkDisassemble times Disassemble on the assembled TinyMan program.
func BenchmarkDisassemble(b *testing.B) {
	ops, err := logic.AssembleString(txntest.TmLsig)
	require.NoError(b, err)
	program := ops.Program
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := logic.Disassemble(program)
		if err != nil {
			require.NoError(b, err)
		}
	}
}
