// parser_ir_merge_test.go verifies MERGE support and nested subquery metadata.
package postgresparser

import (
	"strings"
	"testing"
)

// TestIR_MergeStatement validates MERGE target/source and action capture.
func TestIR_MergeStatement(t *testing.T) {
	sql := `
MERGE INTO customers c
USING (
  SELECT customer_id, total_spent
  FROM summary
) s
ON c.id = s.customer_id
WHEN MATCHED AND s.total_spent > 1000 THEN UPDATE SET vip = true
WHEN NOT MATCHED THEN INSERT (id, total_spent) VALUES (s.customer_id, s.total_spent)
WHEN MATCHED THEN DELETE;`

	ir := parseAssertNoError(t, sql)

	if ir.Command != QueryCommandMerge {
		t.Fatalf("expected MERGE command, got %s", ir.Command)
	}
	if ir.Merge == nil {
		t.Fatalf("expected merge metadata, got nil")
	}
	if strings.ToLower(ir.Merge.Target.Name) != "customers" || ir.Merge.Target.Alias != "c" {
		t.Fatalf("unexpected merge target %+v", ir.Merge.Target)
	}
	if ir.Merge.Source.Table.Type != TableTypeSubquery || ir.Merge.Source.Table.Alias != "s" {
		t.Fatalf("expected subquery source, got %+v", ir.Merge.Source.Table)
	}
	if !containsTable(ir.Tables, "customers") || !containsTable(ir.Tables, "summary") {
		t.Fatalf("expected merge tables to include customers and summary %+v", ir.Tables)
	}
	if !strings.Contains(ir.Merge.Condition, "c.id = s.customer_id") {
		t.Fatalf("unexpected merge condition %q", ir.Merge.Condition)
	}
	if len(ir.Merge.Actions) != 3 {
		t.Fatalf("expected three merge actions, got %+v", ir.Merge.Actions)
	}
	if ir.Merge.Actions[0].Type != "UPDATE" || !strings.Contains(ir.Merge.Actions[0].Condition, "total_spent > 1000") {
		t.Fatalf("unexpected update action %+v", ir.Merge.Actions[0])
	}
	if len(ir.Merge.Actions[0].SetClauses) != 1 || !strings.Contains(ir.Merge.Actions[0].SetClauses[0], "vip = true") {
		t.Fatalf("unexpected update set clauses %+v", ir.Merge.Actions[0].SetClauses)
	}
	if ir.Merge.Actions[1].Type != "INSERT" || len(ir.Merge.Actions[1].InsertColumns) != 2 {
		t.Fatalf("unexpected insert action %+v", ir.Merge.Actions[1])
	}
	if !strings.Contains(ir.Merge.Actions[1].InsertValues, "VALUES (s.customer_id, s.total_spent)") {
		t.Fatalf("unexpected insert values %+v", ir.Merge.Actions[1].InsertValues)
	}
	if ir.Merge.Actions[2].Type != "DELETE" {
		t.Fatalf("expected delete action, got %+v", ir.Merge.Actions[2])
	}
	if len(ir.SetClauses) == 0 || !strings.Contains(ir.SetClauses[0], "vip = true") {
		t.Fatalf("expected merge set clauses to be recorded, got %+v", ir.SetClauses)
	}
	if len(ir.Subqueries) == 0 || strings.ToLower(ir.Subqueries[0].Alias) != "s" {
		t.Fatalf("expected merge source subquery metadata %+v", ir.Subqueries)
	}
	if ir.Subqueries[0].Query == nil || !containsTable(ir.Subqueries[0].Query.Tables, "summary") {
		t.Fatalf("expected subquery to capture summary table %+v", ir.Subqueries[0])
	}
}

// TestExtractDeleteConditionFromText verifies the text-based condition extraction for
// MERGE DELETE clauses. Once the grammar is regenerated with (AND a_expr)? support,
// this helper will be superseded by AST-based extraction via ctx.A_expr().
func TestExtractDeleteConditionFromText(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"WHEN MATCHED THEN DELETE", ""},
		{"WHEN MATCHED DELETE", ""},
		{"WHEN MATCHED AND s.total > 100 THEN DELETE", "s.total > 100"},
		{"WHEN MATCHED AND s.total > 100 DELETE", "s.total > 100"},
		{"WHEN MATCHED AND x = 1 AND y = 2 THEN DELETE", "x = 1 AND y = 2"},
	}
	for _, tt := range tests {
		got := extractDeleteConditionFromText(tt.input)
		if got != tt.want {
			t.Errorf("extractDeleteConditionFromText(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestIR_SubqueryMetadataFromClause confirms subquery IR is retained.
func TestIR_SubqueryMetadataFromClause(t *testing.T) {
	sql := `
SELECT t.id, sq.total
FROM tenants t
JOIN (
  SELECT tenant_id, SUM(amount) AS total
  FROM invoices
  GROUP BY tenant_id
) sq ON sq.tenant_id = t.id`

	ir := parseAssertNoError(t, sql)

	if !containsTable(ir.Tables, "tenants") || !containsTable(ir.Tables, "invoices") {
		t.Fatalf("expected tenants and invoices tables %+v", ir.Tables)
	}
	foundSubquery := false
	for _, tbl := range ir.Tables {
		if tbl.Type == TableTypeSubquery && strings.ToLower(tbl.Alias) == "sq" {
			foundSubquery = true
			break
		}
	}
	if !foundSubquery {
		t.Fatalf("expected subquery table reference in %+v", ir.Tables)
	}
	if len(ir.Subqueries) == 0 {
		t.Fatalf("expected subquery metadata, got none")
	}
	var sq *SubqueryRef
	for i := range ir.Subqueries {
		if strings.ToLower(ir.Subqueries[i].Alias) == "sq" {
			sq = &ir.Subqueries[i]
			break
		}
	}
	if sq == nil || sq.Query == nil {
		t.Fatalf("expected parsed subquery metadata %+v", ir.Subqueries)
	}
	if !containsTable(sq.Query.Tables, "invoices") {
		t.Fatalf("expected subquery to reference invoices, got %+v", sq.Query.Tables)
	}
	if len(sq.Query.GroupBy) == 0 || !strings.Contains(strings.ToLower(sq.Query.GroupBy[0]), "tenant_id") {
		t.Fatalf("expected subquery group by metadata, got %+v", sq.Query.GroupBy)
	}
}
