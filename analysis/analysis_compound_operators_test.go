// Package analysis provides SQL query analysis using the PostgreSQL parser.
// This file tests compound operator extraction from WHERE clauses with multiple conditions.
package analysis

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAnalyzeSQL_CompoundOperators validates that the analyzer extracts all operators
// from a compound WHERE clause, not just the first one.
func TestAnalyzeSQL_CompoundOperators(t *testing.T) {
	sql := `SELECT id, name
            FROM products
            WHERE price > 100
              AND stock <= 50
              OR discount != 0;`

	result, err := AnalyzeSQL(sql)
	require.NoError(t, err, "AnalyzeSQL should parse compound conditions without error")
	require.NotNil(t, result)

	// Collect all operator tokens extracted from WHERE predicates
	var ops []string
	for _, usage := range result.ColumnUsage {
		if usage.UsageType == SQLUsageTypeFilter {
			if usage.Operator != "" {
				ops = append(ops, usage.Operator)
			}
		}
	}

	// Verify that all comparison operators were detected
	expected := []string{">", "<=", "!="}
	assert.ElementsMatch(t, expected, ops,
		"All operators in compound condition should be extracted individually")

	// Verify that columns associated with each operator are correct
	var cols []string
	for _, usage := range result.ColumnUsage {
		if usage.UsageType == SQLUsageTypeFilter {
			cols = append(cols, usage.Column)
		}
	}
	assert.ElementsMatch(t, []string{"price", "stock", "discount"}, cols,
		"Each comparison column should be identified correctly")

	// Optional: ensure no duplicate or merged expression remains
	for _, u := range result.ColumnUsage {
		assert.NotContains(t, u.Operator, "AND", "Operators should not contain boolean connectors")
		assert.NotContains(t, u.Operator, "OR", "Operators should not contain boolean connectors")
	}
}
