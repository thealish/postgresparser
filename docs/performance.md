# Performance Guide

## Overview

`postgresparser` is a **pure Go** SQL parser — zero cgo, zero C toolchain, single-binary deploys, effortless cross-compilation. With SLL prediction mode enabled, it parses most queries in **70–350 µs** with minimal allocations.

## SLL Prediction Mode

The single biggest performance lever is ANTLR's **SLL prediction mode**, which delivers **4–8x faster parsing** and **80%+ fewer allocations** compared to the default LL mode.

### How it works

ANTLR resolves grammar ambiguities using an ATN (Augmented Transition Network) simulator with two modes:

- **LL (default):** Full parser-context analysis at every ambiguous decision point. Always correct, but does more work than necessary for most inputs.
- **SLL:** Decides based only on lookahead tokens. Much faster, and correct for all practical SQL. For the rare edge case where SLL can't resolve an ambiguity, you fall back to LL.

### The SLL-first pattern

Try SLL first; fall back to LL only if needed:

```go
import (
    "github.com/antlr4-go/antlr/v4"
    "github.com/valkdb/postgresparser/gen"
)

func parseFast(sql string) (gen.IRootContext, error) {
    // Attempt 1: SLL mode (fast path).
    input := antlr.NewInputStream(sql)
    lexer := gen.NewPostgreSQLLexer(input)
    stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
    parser := gen.NewPostgreSQLParser(stream)
    parser.BuildParseTrees = true
    parser.RemoveErrorListeners()
    parser.GetInterpreter().SetPredictionMode(antlr.PredictionModeSLL)

    errListener := &errorCounter{}
    parser.AddErrorListener(errListener)

    root := parser.Root()
    if errListener.count == 0 {
        return root, nil // SLL succeeded.
    }

    // Attempt 2: LL mode (SLL reported errors — might be false positives).
    input = antlr.NewInputStream(sql)
    lexer = gen.NewPostgreSQLLexer(input)
    stream = antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
    parser = gen.NewPostgreSQLParser(stream)
    parser.BuildParseTrees = true
    parser.RemoveErrorListeners()
    parser.GetInterpreter().SetPredictionMode(antlr.PredictionModeLL)

    llErrListener := &errorCounter{}
    parser.AddErrorListener(llErrListener)

    root = parser.Root()
    if llErrListener.count > 0 {
        return nil, fmt.Errorf("parse failed with %d errors", llErrListener.count)
    }
    return root, nil
}

type errorCounter struct {
    antlr.DefaultErrorListener
    count int
}

func (l *errorCounter) SyntaxError(_ antlr.Recognizer, _ interface{}, _, _ int, _ string, _ antlr.RecognitionException) {
    l.count++
}
```

See [`examples/sll_mode/main.go`](../examples/sll_mode/main.go) for a full working example.

### Benchmarks: SLL vs LL

| Query | LL (default) | SLL | Speedup | Alloc reduction |
|-------|-------------|-----|---------|-----------------|
| SimpleSelect | 403 µs / 2,209 allocs | 74 µs / 348 allocs | **5.4x** | 84% fewer |
| JoinThreeTables | 1,176 µs / 6,986 allocs | 211 µs / 971 allocs | **5.6x** | 86% fewer |
| InsertUpsert | 246 µs / 1,041 allocs | 206 µs / 909 allocs | **1.2x** | 13% fewer |
| CTE | 1,428 µs / 7,492 allocs | 347 µs / 1,382 allocs | **4.1x** | 82% fewer |
| Subquery | 1,199 µs / 6,940 allocs | 281 µs / 1,296 allocs | **4.3x** | 81% fewer |
| DDL CreateTable | 133 µs / 587 allocs | 107 µs / 453 allocs | **1.2x** | 23% fewer |
| ComplexUpdate | 1,765 µs / 10,088 allocs | 232 µs / 872 allocs | **7.6x** | 91% fewer |
| WindowFunction | 1,014 µs / 4,997 allocs | 291 µs / 1,207 allocs | **3.5x** | 76% fewer |

### SLL compatibility

Tested across 36 diverse SQL patterns (SELECTs, JOINs, CTEs, subqueries, window functions, JSONB operators, MERGE, LATERAL, DISTINCT ON, FOR UPDATE, DDL, set operations, casts, BETWEEN, LIKE, etc.): **all 36 pass SLL with zero errors**.

The PostgreSQL ANTLR grammar is SLL-compatible for all practical SQL. The LL fallback is a safety net, not the common path.

### Fallback cost

If SLL can't resolve a particular query (rare), it falls back to LL — costing roughly 2x the normal LL time for that one query. In practice, we haven't found a real-world query that triggers this.

## Profiling Breakdown

CPU profile of `ParseSQL` on a CTE query:

| Component | CPU (flat) | Notes |
|-----------|-----------|-------|
| Go runtime / GC | ~46% | Scanning short-lived ANTLR objects |
| ANTLR ATN simulator | ~36% | Grammar prediction, closure computation |
| IR extraction (postgresparser code) | ~3% | `populateSelect`, `extractCTEs`, etc. |

The IR extraction layer is lightweight — the cost is dominated by the ANTLR runtime, which SLL mode dramatically reduces.

### Top allocation sources (LL mode)

| Allocator | % of allocations |
|-----------|-----------------|
| `NewATNConfig` | 24% |
| `JMap.Put` | 17% |
| `JStore.Put` | 10% |
| `NewBaseSingletonPredictionContext` | 5.5% |
| `NewBitSet` | 5.4% |

All inside the ANTLR runtime. SLL mode eliminates most of these by avoiding full-context prediction.

## Running Benchmarks

The `benchmark/` directory contains a standalone module with comparative benchmarks:

```bash
cd benchmark
go test -bench=. -benchmem -count=3
```

This benchmarks `postgresparser.ParseSQL()` and raw ANTLR parse steps in LL, SLL, and SLL-with-fallback modes across 8 query types.
