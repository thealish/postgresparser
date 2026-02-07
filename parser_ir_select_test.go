// parser_ir_select_test.go covers SELECT-focused IR behaviours.
package postgresparser

import (
	"strings"
	"testing"
)

// TestIR_MinimalSelect validates baseline SELECT parsing.
func TestIR_MinimalSelect(t *testing.T) {
	sql := `SELECT id FROM users`
	ir := parseAssertNoError(t, sql)

	if ir.Command != QueryCommandSelect {
		t.Fatalf("expected command SELECT, got %s", ir.Command)
	}
	if len(ir.Tables) != 1 || ir.Tables[0].Name != "users" {
		t.Fatalf("expected table 'users', got %#v", ir.Tables)
	}
	if len(ir.Columns) != 1 || ir.Columns[0].Expression != "id" {
		t.Fatalf("unexpected columns %#v", ir.Columns)
	}
}

// TestIR_TableAlias ensures table aliases are captured correctly.
func TestIR_TableAlias(t *testing.T) {
	sql := `SELECT u.id, u.email FROM users u`
	ir := parseAssertNoError(t, sql)

	if len(ir.Tables) != 1 || ir.Tables[0].Name != "users" || ir.Tables[0].Alias != "u" {
		t.Fatalf("unexpected table metadata %+v", ir.Tables)
	}
	if len(ir.Columns) != 2 || ir.Columns[0].Expression != "u.id" {
		t.Fatalf("unexpected columns %+v", ir.Columns)
	}
}

// TestIR_FunctionsOnColumns confirms scalar expressions and aliases persist.
func TestIR_FunctionsOnColumns(t *testing.T) {
	sql := `SELECT COALESCE(email, 'unknown') AS email, LOWER(name) FROM users`
	ir := parseAssertNoError(t, sql)

	if len(ir.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %+v", ir.Columns)
	}
	if ir.Columns[0].Expression != "COALESCE(email, 'unknown')" || ir.Columns[0].Alias != "email" {
		t.Fatalf("unexpected first column %+v", ir.Columns[0])
	}
	if ir.Columns[1].Expression != "LOWER(name)" {
		t.Fatalf("unexpected second column %+v", ir.Columns[1])
	}
}

// TestIR_AggregateFunctions verifies aggregate projections and WHERE clauses.
func TestIR_AggregateFunctions(t *testing.T) {
	sql := `SELECT SUM(price) AS total, COUNT(*) FROM orders WHERE status = 'shipped'`
	ir := parseAssertNoError(t, sql)

	if ir.Columns[0].Expression != "SUM(price)" || ir.Columns[0].Alias != "total" {
		t.Fatalf("unexpected first column %+v", ir.Columns[0])
	}
	if ir.Columns[1].Expression != "COUNT(*)" {
		t.Fatalf("unexpected second column %+v", ir.Columns[1])
	}
	if len(ir.Where) != 1 || normalise(ir.Where[0]) != "status='shipped'" {
		t.Fatalf("unexpected WHERE clause %+v", ir.Where)
	}
}

// TestIR_ArithmeticInProjection checks arithmetic expressions in projections.
func TestIR_ArithmeticInProjection(t *testing.T) {
	sql := `SELECT price * quantity AS total_cost FROM orders`
	ir := parseAssertNoError(t, sql)

	if len(ir.Columns) != 1 || ir.Columns[0].Expression != "price * quantity" || ir.Columns[0].Alias != "total_cost" {
		t.Fatalf("unexpected column %+v", ir.Columns[0])
	}
}

// TestIR_CaseExpressions ensures CASE expressions survive in projections.
func TestIR_CaseExpressions(t *testing.T) {
	sql := `
SELECT
  CASE
    WHEN status = 'shipped' THEN 'done'
    WHEN status = 'pending' THEN 'waiting'
    ELSE 'other'
  END AS state
FROM orders`
	ir := parseAssertNoError(t, sql)

	if len(ir.Columns) != 1 {
		t.Fatalf("expected 1 column, got %+v", ir.Columns)
	}
	expr := ir.Columns[0].Expression
	if !strings.Contains(expr, "CASE") || !strings.Contains(expr, "WHEN status = 'shipped'") {
		t.Fatalf("unexpected CASE expression %q", expr)
	}
	if ir.Columns[0].Alias != "state" {
		t.Fatalf("unexpected alias %q", ir.Columns[0].Alias)
	}
}

// TestIR_JSONOperators confirms JSON operators are preserved.
func TestIR_JSONOperators(t *testing.T) {
	sql := `
SELECT data->>'name' AS username, data->'meta'->>'id' AS user_id
FROM profiles
WHERE (data->>'active')::boolean = true`
	ir := parseAssertNoError(t, sql)

	if len(ir.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %+v", ir.Columns)
	}
	if ir.Columns[0].Expression != "data->>'name'" || ir.Columns[0].Alias != "username" {
		t.Fatalf("unexpected first column %+v", ir.Columns[0])
	}
	if ir.Columns[1].Expression != "data->'meta'->>'id'" || ir.Columns[1].Alias != "user_id" {
		t.Fatalf("unexpected second column %+v", ir.Columns[1])
	}
	if len(ir.Where) != 1 || !strings.Contains(ir.Where[0], "data->>'active'") {
		t.Fatalf("unexpected WHERE clause %+v", ir.Where)
	}
}

// TestIR_WindowFunctions checks window definitions are retained.
func TestIR_WindowFunctions(t *testing.T) {
	sql := `
SELECT id,
       ROW_NUMBER() OVER (PARTITION BY tenant ORDER BY created_at DESC) AS seq
FROM orders`
	ir := parseAssertNoError(t, sql)

	if len(ir.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %+v", ir.Columns)
	}
	expr := ir.Columns[1].Expression
	if !strings.Contains(expr, "ROW_NUMBER() OVER") || !strings.Contains(expr, "PARTITION BY tenant") || !strings.Contains(expr, "ORDER BY created_at DESC") {
		t.Fatalf("unexpected window expression %q", expr)
	}
	if ir.Columns[1].Alias != "seq" {
		t.Fatalf("unexpected alias %q", ir.Columns[1].Alias)
	}
}

// TestIR_GroupByHavingFunctions validates GROUP BY and HAVING extraction.
func TestIR_GroupByHavingFunctions(t *testing.T) {
	sql := `
SELECT region, SUM(sales) AS total
FROM metrics
GROUP BY region
HAVING SUM(sales) > 100`
	ir := parseAssertNoError(t, sql)

	if len(ir.GroupBy) != 1 || ir.GroupBy[0] != "region" {
		t.Fatalf("unexpected GROUP BY %+v", ir.GroupBy)
	}
	if len(ir.Having) != 1 || normalise(ir.Having[0]) != "sum(sales)>100" {
		t.Fatalf("unexpected HAVING %+v", ir.Having)
	}
}

// TestIR_OrderByWithFunctions verifies ORDER BY expressions and modifiers.
func TestIR_OrderByWithFunctions(t *testing.T) {
	sql := `
SELECT id, name
FROM users
ORDER BY COALESCE(last_login, created_at) DESC NULLS FIRST`
	ir := parseAssertNoError(t, sql)

	if len(ir.OrderBy) != 1 {
		t.Fatalf("expected 1 ORDER BY, got %+v", ir.OrderBy)
	}
	order := ir.OrderBy[0]
	if order.Expression != "COALESCE(last_login, created_at)" {
		t.Fatalf("unexpected ORDER BY expression %+v", order)
	}
	if order.Direction != "DESC" || order.Nulls != "NULLS FIRST" {
		t.Fatalf("unexpected ORDER BY modifiers %+v", order)
	}
}

// TestIR_CTEAndNestedCTE checks multiple CTEs are captured with raw SQL.
func TestIR_CTEAndNestedCTE(t *testing.T) {
	sql := `
WITH recent AS (
  SELECT id, created_at FROM orders WHERE created_at > NOW() - INTERVAL '30 days'
), active_users AS (
  SELECT user_id FROM logins WHERE last_login > NOW() - INTERVAL '7 days'
)
SELECT r.id
FROM recent r
JOIN active_users a ON r.id = a.user_id`
	ir := parseAssertNoError(t, sql)

	if len(ir.CTEs) != 2 {
		t.Fatalf("expected 2 CTEs, got %+v", ir.CTEs)
	}
	if strings.ToLower(ir.CTEs[0].Name) != "recent" || !strings.Contains(ir.CTEs[0].Query, "SELECT id, created_at") {
		t.Fatalf("unexpected first CTE %+v", ir.CTEs[0])
	}
	if strings.ToLower(ir.CTEs[1].Name) != "active_users" || !strings.Contains(ir.CTEs[1].Query, "SELECT user_id") {
		t.Fatalf("unexpected second CTE %+v", ir.CTEs[1])
	}
}

// TestIR_LimitOffset confirms LIMIT/OFFSET text is stored verbatim.
func TestIR_LimitOffset(t *testing.T) {
	sql := `SELECT * FROM logs LIMIT 10 OFFSET 5`
	ir := parseAssertNoError(t, sql)

	if ir.Limit == nil {
		t.Fatalf("expected limit clause, got nil")
	}
	if !strings.Contains(strings.ToUpper(ir.Limit.Limit), "LIMIT 10") {
		t.Fatalf("unexpected limit text %q", ir.Limit.Limit)
	}
	if !strings.Contains(strings.ToUpper(ir.Limit.Offset), "OFFSET 5") {
		t.Fatalf("unexpected offset text %q", ir.Limit.Offset)
	}
}

// TestIR_Parameters ensures positional and anonymous parameters are recorded.
func TestIR_Parameters(t *testing.T) {
	sql := `SELECT * FROM users WHERE age > ? AND id = $2`
	ir := parseAssertNoError(t, sql)

	if len(ir.Parameters) != 2 {
		t.Fatalf("expected 2 parameters, got %+v", ir.Parameters)
	}
	if ir.Parameters[0].Raw != "?" || ir.Parameters[0].Position != 1 {
		t.Fatalf("unexpected first parameter %+v", ir.Parameters[0])
	}
	if ir.Parameters[1].Raw != "$2" || ir.Parameters[1].Position != 2 {
		t.Fatalf("unexpected second parameter %+v", ir.Parameters[1])
	}
}

// TestIR_LateralCorrelationNoFalseMatch ensures short aliases don't false-match
// substrings in LATERAL function text (e.g., alias "a" should not match "data.").
func TestIR_LateralCorrelationNoFalseMatch(t *testing.T) {
	sql := `SELECT * FROM products a
		CROSS JOIN LATERAL unnest(a.tags) AS tag(name)`
	ir := parseAssertNoError(t, sql)

	// Alias "a" should match "a.tags" via word boundary.
	foundA := false
	for _, corr := range ir.Correlations {
		if corr.OuterAlias == "a" && corr.Type == "LATERAL" {
			foundA = true
			break
		}
	}
	if !foundA {
		t.Fatalf("expected LATERAL correlation for alias 'a', got %+v", ir.Correlations)
	}

	// Verify that a column named "data" does NOT false-match alias "a".
	sql2 := `SELECT * FROM metadata
		CROSS JOIN LATERAL unnest(data.items) AS item(val)`
	ir2 := parseAssertNoError(t, sql2)

	for _, corr := range ir2.Correlations {
		if corr.OuterAlias == "a" {
			t.Fatalf("alias 'a' should NOT match 'data.items', but got correlation %+v", corr)
		}
	}
}

// TestIR_IgnoresDoubleSlashComment verifies preprocessing strips // comments.
func TestIR_IgnoresDoubleSlashComment(t *testing.T) {
	sql := `SELECT id FROM users // extra metadata`
	ir := parseAssertNoError(t, sql)

	if len(ir.Tables) != 1 || ir.Tables[0].Name != "users" {
		t.Fatalf("unexpected tables %+v", ir.Tables)
	}
	if len(ir.Columns) != 1 || ir.Columns[0].Expression != "id" {
		t.Fatalf("unexpected columns %+v", ir.Columns)
	}
}

// TestIR_ComplexMonsterQuery stress-tests complex real-world SELECT shapes.
func TestIR_ComplexMonsterQuery(t *testing.T) {
	sql := `
WITH recent_orders AS (
  SELECT
    o.id,
    o.customer_id,
    o.created_at,
    SUM(oi.quantity * oi.price) AS total_amount
  FROM orders o
  JOIN order_items oi ON oi.order_id = o.id
  WHERE o.created_at >= NOW() - INTERVAL '30 days'
  GROUP BY o.id, o.customer_id, o.created_at
),
ranked_customers AS (
  SELECT
    c.id AS customer_id,
    c.name,
    COUNT(ro.id) AS order_count,
    SUM(ro.total_amount) AS revenue,
    RANK() OVER (ORDER BY SUM(ro.total_amount) DESC) AS rank
  FROM customers c
  JOIN recent_orders ro ON ro.customer_id = c.id
  GROUP BY c.id, c.name
),
top_customers AS (
  SELECT customer_id
  FROM ranked_customers
  WHERE rank <= 10
),
expanded_data AS (
  SELECT
    tc.customer_id,
    jsonb_build_object(
      'info', c.metadata ->> 'info',
      'orders', (
        SELECT jsonb_agg(to_jsonb(ro.*))
        FROM recent_orders ro
        WHERE ro.customer_id = tc.customer_id
      )
    ) AS full_data
  FROM top_customers tc
  JOIN customers c ON c.id = tc.customer_id
)
SELECT
  ed.customer_id,
  ed.full_data ->> 'info' AS customer_info,
  COUNT(DISTINCT (o.order_json ->> 'id')) AS order_count,
  SUM((o.order_json ->> 'total_amount')::numeric) AS total_revenue
FROM expanded_data ed
CROSS JOIN LATERAL jsonb_array_elements(ed.full_data -> 'orders') AS o(order_json)
WHERE (o.order_json ->> 'total_amount')::numeric > 100
GROUP BY ed.customer_id, ed.full_data
HAVING SUM((o.order_json ->> 'total_amount')::numeric) > 5000
ORDER BY total_revenue DESC
LIMIT 5;
`
	ir := parseAssertNoError(t, sql)

	if len(ir.CTEs) != 4 {
		t.Fatalf("expected 4 CTEs, got %+v", ir.CTEs)
	}
	if strings.ToLower(ir.CTEs[3].Name) != "expanded_data" {
		t.Fatalf("unexpected final CTE %+v", ir.CTEs[3])
	}
	if len(ir.Tables) < 2 {
		t.Fatalf("expected at least expanded_data and lateral function in tables, got %+v", ir.Tables)
	}
	// Find expanded_data CTE reference (not necessarily first due to base tables from CTEs)
	foundExpandedData := false
	for _, tbl := range ir.Tables {
		if tbl.Type == TableTypeCTE && strings.ToLower(tbl.Name) == "expanded_data" {
			foundExpandedData = true
			break
		}
	}
	if !foundExpandedData {
		t.Fatalf("expected to find expanded_data CTE reference in tables %+v", ir.Tables)
	}
	foundLateral := false
	for _, tbl := range ir.Tables {
		if tbl.Type == TableTypeFunction && strings.Contains(tbl.Raw, "jsonb_array_elements") {
			foundLateral = true
			break
		}
	}
	if !foundLateral {
		t.Fatalf("expected to find jsonb_array_elements lateral function in tables %+v", ir.Tables)
	}
	if len(ir.OrderBy) != 1 || strings.ToLower(ir.OrderBy[0].Expression) != "total_revenue" {
		t.Fatalf("unexpected ORDER BY %+v", ir.OrderBy)
	}
	if ir.Limit == nil || !strings.Contains(strings.ToUpper(ir.Limit.Limit), "LIMIT 5") {
		t.Fatalf("unexpected LIMIT %+v", ir.Limit)
	}
}

// TestIR_InnerJoinWithConditions checks join predicates capture multiple clauses.
func TestIR_InnerJoinWithConditions(t *testing.T) {
	sql := `
SELECT o.id, c.name
FROM orders o
INNER JOIN customers c ON o.customer_id = c.id AND c.status = 'active'`
	ir := parseAssertNoError(t, sql)

	if len(ir.Tables) != 2 || ir.Tables[0].Name != "orders" || ir.Tables[1].Name != "customers" {
		t.Fatalf("unexpected tables %+v", ir.Tables)
	}
	if len(ir.JoinConditions) != 1 {
		t.Fatalf("expected 1 join condition, got %+v", ir.JoinConditions)
	}
	joinExpr := normalise(ir.JoinConditions[0])
	if !strings.Contains(joinExpr, "o.customer_id=c.id") || !strings.Contains(joinExpr, "c.status='active'") {
		t.Fatalf("unexpected join expression %q", joinExpr)
	}
}

// TestIR_LeftJoinWithFunctions ensures LEFT JOIN filters and expressions persist.
func TestIR_LeftJoinWithFunctions(t *testing.T) {
	sql := `
SELECT u.id, COALESCE(p.name, 'N/A') AS product_name
FROM users u
LEFT JOIN purchases p ON u.id = p.user_id AND p.amount > 0`
	ir := parseAssertNoError(t, sql)

	if len(ir.Tables) != 2 || ir.Tables[0].Name != "users" || ir.Tables[1].Name != "purchases" {
		t.Fatalf("unexpected tables %+v", ir.Tables)
	}
	if ir.Columns[1].Expression != "COALESCE(p.name, 'N/A')" || ir.Columns[1].Alias != "product_name" {
		t.Fatalf("unexpected second column %+v", ir.Columns[1])
	}
	if len(ir.JoinConditions) != 1 || !strings.Contains(ir.JoinConditions[0], "p.amount > 0") {
		t.Fatalf("unexpected join condition %+v", ir.JoinConditions)
	}
}

// TestIR_FallbackToUnknown confirms unsupported statements return UNKNOWN.
func TestIR_FallbackToUnknown(t *testing.T) {
	sql := `CREATE TABLE demo(id INT PRIMARY KEY);`
	ir := parseAssertNoError(t, sql)

	if ir.Command != QueryCommandUnknown {
		t.Fatalf("expected UNKNOWN command for unsupported statement, got %s", ir.Command)
	}
}

// TestContainsWordDot verifies the word-boundary dot matching used by LATERAL correlation detection.
func TestContainsWordDot(t *testing.T) {
	tests := []struct {
		text string
		word string
		want bool
	}{
		{"t.id", "t", true},
		{"data.id", "a", false},        // "a" is a substring of "data", not at word boundary
		{"data.id", "data", true},      // "data" at start
		{"foo(data.id)", "data", true}, // preceded by non-ident char
		{"x_data.id", "data", false},   // preceded by underscore (ident char)
		{"t.id + s.value", "s", true},  // preceded by space
		{"t.id + s.value", "t", true},  // at start
		{"", "t", false},               // empty text
		{"t.id", "", false},            // empty word
	}
	for _, tt := range tests {
		got := containsWordDot(tt.text, tt.word)
		if got != tt.want {
			t.Errorf("containsWordDot(%q, %q) = %v, want %v", tt.text, tt.word, got, tt.want)
		}
	}
}
