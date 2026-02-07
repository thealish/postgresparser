package postgresparser

import (
	"strings"
	"testing"
)

func TestParseSQLSimpleSelect(t *testing.T) {
	sql := "SELECT id, name FROM users WHERE status = ?;"
	pq, err := ParseSQL(sql)
	if err != nil {
		t.Fatalf("ParseSQL returned error: %v", err)
	}
	if pq.Command != QueryCommandSelect {
		t.Fatalf("expected command SELECT, got %s", pq.Command)
	}
	if len(pq.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(pq.Columns))
	}
	if pq.Columns[0].Expression != "id" || pq.Columns[1].Expression != "name" {
		t.Fatalf("unexpected columns %+v", pq.Columns)
	}
	if len(pq.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(pq.Tables))
	}
	if pq.Tables[0].Name != "users" || pq.Tables[0].Type != TableTypeBase {
		t.Fatalf("unexpected table %+v", pq.Tables[0])
	}
	if len(pq.Where) != 1 || normalise(pq.Where[0]) != "status=?" {
		t.Fatalf("unexpected WHERE clause %v", pq.Where)
	}
	if len(pq.Parameters) != 1 || pq.Parameters[0].Raw != "?" || pq.Parameters[0].Position != 1 {
		t.Fatalf("unexpected parameters %+v", pq.Parameters)
	}
}

func TestParseSQLJoinWithAlias(t *testing.T) {
	sql := `
SELECT o.id, c.name
FROM orders o
JOIN customers c ON o.customer_id = c.id
WHERE o.created_at > $1 AND c.active = true;`

	pq, err := ParseSQL(sql)
	if err != nil {
		t.Fatalf("ParseSQL returned error: %v", err)
	}
	if len(pq.Tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(pq.Tables))
	}
	if pq.Tables[0].Name != "orders" || pq.Tables[0].Alias != "o" {
		t.Fatalf("unexpected first table %+v", pq.Tables[0])
	}
	if pq.Tables[1].Name != "customers" || pq.Tables[1].Alias != "c" {
		t.Fatalf("unexpected second table %+v", pq.Tables[1])
	}
	if len(pq.JoinConditions) == 0 || !strings.Contains(normalise(pq.JoinConditions[0]), "o.customer_id=c.id") {
		t.Fatalf("expected join condition, got %v", pq.JoinConditions)
	}
	if len(pq.Where) != 1 || !strings.Contains(normalise(pq.Where[0]), "o.created_at>$1") {
		t.Fatalf("unexpected WHERE clause %v", pq.Where)
	}
	if len(pq.Parameters) != 1 || pq.Parameters[0].Raw != "$1" || pq.Parameters[0].Position != 1 {
		t.Fatalf("unexpected parameters %+v", pq.Parameters)
	}
}

func TestParseSQLWithCTE(t *testing.T) {
	sql := `
WITH ranked AS (
    SELECT id, ROW_NUMBER() OVER (PARTITION BY tenant ORDER BY created_at DESC) AS seq
    FROM orders
    WHERE tenant = ?
)
SELECT r.id, r.seq
FROM ranked r
WHERE r.seq <= 5;`

	pq, err := ParseSQL(sql)
	if err != nil {
		t.Fatalf("ParseSQL returned error: %v", err)
	}
	if len(pq.CTEs) != 1 {
		t.Fatalf("expected 1 CTE, got %d", len(pq.CTEs))
	}
	if strings.ToLower(pq.CTEs[0].Name) != "ranked" {
		t.Fatalf("unexpected CTE %+v", pq.CTEs[0])
	}
	// Should now have both the base table "orders" from the CTE and the CTE reference "ranked"
	if len(pq.Tables) != 2 {
		t.Fatalf("expected 2 tables (orders from CTE and ranked ref), got %d: %+v", len(pq.Tables), pq.Tables)
	}
	foundOrders := false
	foundRanked := false
	for _, tbl := range pq.Tables {
		if tbl.Name == "orders" && tbl.Type == TableTypeBase {
			foundOrders = true
		}
		if strings.ToLower(tbl.Name) == "ranked" && tbl.Type == TableTypeCTE {
			foundRanked = true
		}
	}
	if !foundOrders {
		t.Fatalf("expected orders base table from CTE, tables: %+v", pq.Tables)
	}
	if !foundRanked {
		t.Fatalf("expected ranked CTE reference, tables: %+v", pq.Tables)
	}
	if len(pq.Where) != 1 || normalise(pq.Where[0]) != "r.seq<=5" {
		t.Fatalf("unexpected WHERE clause %v", pq.Where)
	}
	if len(pq.Parameters) != 1 || pq.Parameters[0].Raw != "?" {
		t.Fatalf("unexpected parameters %+v", pq.Parameters)
	}
}

func TestParseSQLGroupOrderLimit(t *testing.T) {
	sql := `
SELECT tenant, COUNT(*) AS cnt
FROM events
WHERE occurred_at >= $1
GROUP BY tenant
HAVING COUNT(*) > 5
ORDER BY cnt DESC NULLS LAST
LIMIT 10 OFFSET 2;`

	pq, err := ParseSQL(sql)
	if err != nil {
		t.Fatalf("ParseSQL returned error: %v", err)
	}
	if len(pq.GroupBy) != 1 || pq.GroupBy[0] != "tenant" {
		t.Fatalf("unexpected GROUP BY %v", pq.GroupBy)
	}
	if len(pq.Having) != 1 || !strings.Contains(normalise(pq.Having[0]), "count(*)>5") {
		t.Fatalf("unexpected HAVING %v", pq.Having)
	}
	if len(pq.OrderBy) != 1 || strings.ToUpper(pq.OrderBy[0].Direction) != "DESC" || strings.ToUpper(pq.OrderBy[0].Nulls) != "NULLS LAST" {
		t.Fatalf("unexpected ORDER BY %+v", pq.OrderBy)
	}
	if pq.Limit == nil || !strings.Contains(strings.ToUpper(pq.Limit.Limit), "LIMIT") {
		t.Fatalf("expected LIMIT clause, got %+v", pq.Limit)
	}
	if pq.Limit.Offset == "" || !strings.Contains(strings.ToUpper(pq.Limit.Offset), "OFFSET") {
		t.Fatalf("expected OFFSET clause, got %+v", pq.Limit)
	}
}

func TestParseSQLFunctionAndSubqueryTables(t *testing.T) {
	sql := `
SELECT sub.id, ids.id
FROM (SELECT id FROM accounts) sub
CROSS JOIN LATERAL unnest($1) AS ids(id);`

	pq, err := ParseSQL(sql)
	if err != nil {
		t.Fatalf("ParseSQL returned error: %v", err)
	}
	var foundSubquery, foundFunction, foundBase bool
	for _, tbl := range pq.Tables {
		switch tbl.Type {
		case TableTypeSubquery:
			if strings.ToLower(tbl.Alias) == "sub" {
				foundSubquery = true
			}
		case TableTypeFunction:
			if strings.ToLower(tbl.Alias) == "ids" {
				foundFunction = true
			}
		case TableTypeBase:
			if strings.ToLower(tbl.Name) == "accounts" {
				foundBase = true
			}
		}
	}
	if !foundSubquery {
		t.Fatalf("expected subquery table reference in %+v", pq.Tables)
	}
	if !foundFunction {
		t.Fatalf("expected function table reference in %+v", pq.Tables)
	}
	if !foundBase {
		t.Fatalf("expected base accounts table to be surfaced in %+v", pq.Tables)
	}
	if len(pq.Parameters) != 1 || pq.Parameters[0].Raw != "$1" {
		t.Fatalf("unexpected parameters %+v", pq.Parameters)
	}
}

// normalise strips whitespace and lowercases a string for comparison.
