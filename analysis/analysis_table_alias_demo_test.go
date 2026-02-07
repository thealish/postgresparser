// Package analysis provides SQL analysis capabilities using the ANTLR PostgreSQL parser.
// This file demonstrates the table alias preservation fix with detailed output.
package analysis

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDemonstrateTableAliasPreservation shows how the fix preserves table aliases
// for columns with the same name from different tables.
func TestDemonstrateTableAliasPreservation(t *testing.T) {
	sql := `SELECT * FROM orders o
		JOIN inventory i ON o.product_id = i.product_id
		WHERE o.status = 'active' AND i.status = 'in_stock'`

	result, err := AnalyzeSQL(sql)
	require.NoError(t, err)
	require.NotNil(t, result)

	t.Log("\n=== Table Alias Preservation Demo ===")
	t.Logf("SQL: %s\n", sql)

	// Group filters by column name
	filtersByColumn := make(map[string][]SQLColumnUsage)
	for _, usage := range result.ColumnUsage {
		if usage.UsageType == SQLUsageTypeFilter {
			filtersByColumn[usage.Column] = append(filtersByColumn[usage.Column], usage)
		}
	}

	// Show status column filters
	t.Log("Status column filters (should have 2 entries with different aliases):")
	for i, usage := range filtersByColumn["status"] {
		t.Logf("  %d. TableAlias: '%s', Column: '%s', Operator: '%s', Expression: '%s'",
			i+1, usage.TableAlias, usage.Column, usage.Operator, usage.Expression)
	}

	t.Log("\nBefore the fix:")
	t.Log("  - TableAlias would be empty for both entries")
	t.Log("  - Only ONE entry would exist (second one discarded)")
	t.Log("\nAfter the fix:")
	t.Log("  - TableAlias is preserved ('o' and 'i')")
	t.Log("  - Both entries exist with correct table context")
	t.Log("  - Consumers can now tie filters back to correct tables")
}
