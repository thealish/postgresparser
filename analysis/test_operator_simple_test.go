// Package analysis provides SQL query analysis using the PostgreSQL parser.
// This file contains a simple test case to verify operator extraction from compound conditions.
package analysis

import (
	"testing"
)

// TestSimpleOperatorCase tests that we correctly extract individual operators
// from compound WHERE conditions (e.g., "price > 100 AND stock <= 50").
func TestSimpleOperatorCase(t *testing.T) {
	// Simple case with AND
	sql := `SELECT * FROM products WHERE price > 100 AND stock <= 50`

	result, err := AnalyzeSQL(sql)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("WHERE clause:", result.Where)
	t.Log("\nColumn Usage:")
	for _, usage := range result.ColumnUsage {
		t.Logf("  Column: %s, Operator: '%s', UsageType: %s, Context: %s",
			usage.Column, usage.Operator, usage.UsageType, usage.Context)
	}

	// Verify we have operators for both conditions
	operatorCount := make(map[string]int)
	for _, usage := range result.ColumnUsage {
		if usage.UsageType == "filter" && usage.Operator != "" {
			operatorCount[usage.Operator]++
		}
	}

	if operatorCount[">"] != 1 {
		t.Errorf("Expected operator '>' to appear once, got %d", operatorCount[">"])
	}
	if operatorCount["<="] != 1 {
		t.Errorf("Expected operator '<=' to appear once, got %d", operatorCount["<="])
	}

	// Print summary for debugging
	t.Log("\nOperator Count:")
	for op, count := range operatorCount {
		t.Logf("  '%s': %d", op, count)
	}
}
