// Example: using SLL prediction mode for faster parsing.
//
// ANTLR's default LL prediction mode is correct for all grammars but slow.
// SLL mode skips expensive full-context analysis and is 4-8x faster for
// typical SQL queries. The tradeoff: SLL may report false syntax errors
// on grammars with context-dependent ambiguities.
//
// The standard pattern is SLL-first with LL fallback:
//  1. Parse with SLL mode.
//  2. If syntax error → re-parse with LL (the error might be SLL's limitation).
//  3. If no error → done, use the SLL result.
//
// In practice, the PostgreSQL ANTLR grammar is SLL-compatible for all common
// SQL patterns. The LL fallback is a safety net, not the common path.
package main

import (
	"fmt"
	"log"

	"github.com/antlr4-go/antlr/v4"
	"github.com/valkdb/postgresparser/gen"
)

func main() {
	sql := `WITH active_users AS (
    SELECT id, name FROM users WHERE active = true
)
SELECT au.name, COUNT(o.id) AS order_count
FROM active_users au
LEFT JOIN orders o ON o.user_id = au.id
GROUP BY au.name
ORDER BY order_count DESC`

	root := parseSLLWithFallback(sql)
	if root == nil {
		log.Fatal("parse returned nil")
	}
	fmt.Printf("Parsed successfully: %s...\n", sql[:min(60, len(sql))])

	// Walk the tree to extract statement type.
	stmtBlock := root.Stmtblock()
	if stmtBlock != nil && stmtBlock.Stmtmulti() != nil {
		stmts := stmtBlock.Stmtmulti().AllStmt()
		for i, stmt := range stmts {
			text := stmt.GetText()
			if len(text) > 80 {
				text = text[:80] + "..."
			}
			fmt.Printf("Statement %d: %s\n", i+1, text)
		}
	}
}

// parseSLLWithFallback tries SLL prediction mode first (4-8x faster),
// falling back to LL mode if SLL reports a syntax error.
func parseSLLWithFallback(sql string) gen.IRootContext {
	// --- Attempt 1: SLL mode ---
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
		return root // SLL succeeded — fast path.
	}

	// --- Attempt 2: LL mode (SLL reported errors) ---
	fmt.Printf("SLL mode reported %d error(s), falling back to LL mode\n", errListener.count)
	input = antlr.NewInputStream(sql)
	lexer = gen.NewPostgreSQLLexer(input)
	stream = antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	parser = gen.NewPostgreSQLParser(stream)
	parser.BuildParseTrees = true
	parser.RemoveErrorListeners()
	parser.GetInterpreter().SetPredictionMode(antlr.PredictionModeLL)

	return parser.Root()
}

// errorCounter counts syntax errors without storing details.
type errorCounter struct {
	antlr.DefaultErrorListener
	count int
}

func (l *errorCounter) SyntaxError(_ antlr.Recognizer, _ interface{}, _, _ int, _ string, _ antlr.RecognitionException) {
	l.count++
}
