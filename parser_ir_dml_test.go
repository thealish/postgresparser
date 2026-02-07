// parser_ir_dml_test.go exercises INSERT/UPDATE/DELETE specific behaviour.
package postgresparser

import (
	"strings"
	"testing"
)

// TestIR_AdvancedExpressionsAndSetOps exercises UNION chains plus key DML shapes.
func TestIR_AdvancedExpressionsAndSetOps(t *testing.T) {
	tests := []struct {
		name      string
		sql       string
		expectCmd QueryCommand
	}{
		{
			name: "UNION ALL with nested CASE expression",
			sql: `
			(
			  SELECT id,
			         CASE
			           WHEN amount > 100 THEN 'high'
			           ELSE 'low'
			         END AS spending_level
			  FROM orders
			  WHERE status = 'paid'
			)
			UNION ALL
			(
			  SELECT id, 'archived' AS spending_level
			  FROM archived_orders
			)
			ORDER BY id;`,
			expectCmd: QueryCommandSelect,
		},
		{
			name: "INSERT with VALUES and RETURNING",
			sql: `
			INSERT INTO customers (name, email, signup_date)
			VALUES ('John', 'john@example.com', NOW())
			RETURNING id, created_at;`,
			expectCmd: QueryCommandInsert,
		},
		{
			name: "UPDATE with FROM and complex SET expression",
			sql: `
			UPDATE products p
			SET price = price * (1 - d.discount_rate),
			    updated_at = NOW()
			FROM discounts d
			WHERE p.category_id = d.category_id
			  AND d.expires_at > NOW()
			RETURNING p.id, p.price;`,
			expectCmd: QueryCommandUpdate,
		},
		{
			name: "DELETE with USING and RETURNING clause",
			sql: `
			DELETE FROM sessions s
			USING users u
			WHERE s.user_id = u.id AND u.active = false
			RETURNING s.id;`,
			expectCmd: QueryCommandDelete,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ir := parseAssertNoError(t, tc.sql)

			if ir.Command != tc.expectCmd {
				t.Fatalf("expected command %s, got %s", tc.expectCmd, ir.Command)
			}

			if len(ir.Columns) == 0 && ir.Command == QueryCommandSelect {
				t.Fatalf("[%s] expected columns to be captured", tc.name)
			}

			if ir.Command == QueryCommandUpdate {
				foundSet := false
				for _, expr := range ir.SetClauses {
					if strings.Contains(expr, "price * (1 - d.discount_rate)") {
						foundSet = true
					}
				}
				if !foundSet {
					t.Fatalf("[%s] expected complex SET expression, got %+v", tc.name, ir.SetClauses)
				}
			}

			if ir.Command == QueryCommandDelete && len(ir.Returning) == 0 {
				t.Fatalf("[%s] expected RETURNING clause for DELETE", tc.name)
			}

			switch tc.name {
			case "UNION ALL with nested CASE expression":
				if len(ir.SetOperations) != 1 {
					t.Fatalf("expected one set operation, got %+v", ir.SetOperations)
				}
				op := ir.SetOperations[0]
				if op.Type != "UNION ALL" {
					t.Fatalf("expected UNION ALL type, got %q", op.Type)
				}
				if len(op.Columns) != 2 || op.Columns[0] != "id" {
					t.Fatalf("unexpected union columns %+v", op.Columns)
				}
				if !strings.Contains(strings.ToLower(op.Query), "from archived_orders") {
					t.Fatalf("expected RHS query to reference archived_orders, got %q", op.Query)
				}
				if len(op.Tables) != 1 || strings.ToLower(op.Tables[0].Name) != "archived_orders" {
					t.Fatalf("expected archived_orders table in set operation, got %+v", op.Tables)
				}
			default:
				if len(ir.SetOperations) != 0 {
					t.Fatalf("expected no set operations, got %+v", ir.SetOperations)
				}
			}
		})
	}
}

// TestIR_ExampleLibraryScenarios enforces behaviour from the design examples.
func TestIR_ExampleLibraryScenarios(t *testing.T) {
	cases := []struct {
		name string
		sql  string
	}{
		{
			name: "Example A - Select with parameter",
			sql:  `SELECT * FROM accounts WHERE status = ?`,
		},
		{
			name: "Example B - Join with explicit predicates",
			sql: `
SELECT orders.id, customers.name
FROM orders
JOIN customers ON customers.id = orders.customer_id`,
		},
		{
			name: "Example C - CTE with window and predicate",
			sql: `
WITH ranked AS (
  SELECT id, RANK() OVER (ORDER BY created_at) AS r
  FROM items
)
SELECT *
FROM ranked
WHERE r <= 10`,
		},
		{
			name: "Example D - Update with returning",
			sql:  `UPDATE products SET price = price * 1.1 WHERE category = ? RETURNING id`,
		},
		{
			name: "Example E - Delete using join and returning",
			sql: `
DELETE FROM sessions
USING users
WHERE sessions.user_id = users.id AND users.active = false
RETURNING sessions.id`,
		},
		{
			name: "Example F - Insert with returning",
			sql:  `INSERT INTO metrics (id, payload) VALUES (?, ?) RETURNING id`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ir := parseAssertNoError(t, tc.sql)

			switch tc.name {
			case "Example A - Select with parameter":
				if ir.Command != QueryCommandSelect {
					t.Fatalf("expected SELECT command, got %s", ir.Command)
				}
				if len(ir.Tables) != 1 || ir.Tables[0].Name != "accounts" {
					t.Fatalf("expected accounts table, got %+v", ir.Tables)
				}
				if len(ir.Columns) != 1 || ir.Columns[0].Expression != "*" {
					t.Fatalf("expected wildcard column, got %+v", ir.Columns)
				}
				if len(ir.Where) != 1 || !strings.Contains(normalise(ir.Where[0]), "status=?") {
					t.Fatalf("expected status predicate in WHERE, got %+v", ir.Where)
				}
				if len(ir.Parameters) != 1 || ir.Parameters[0].Raw != "?" || ir.Parameters[0].Position != 1 {
					t.Fatalf("expected single anonymous parameter, got %+v", ir.Parameters)
				}
			case "Example B - Join with explicit predicates":
				if ir.Command != QueryCommandSelect {
					t.Fatalf("expected SELECT command, got %s", ir.Command)
				}
				if len(ir.Tables) != 2 {
					t.Fatalf("expected two tables, got %+v", ir.Tables)
				}
				if ir.Tables[0].Name != "orders" || ir.Tables[1].Name != "customers" {
					t.Fatalf("unexpected tables %+v", ir.Tables)
				}
				if len(ir.JoinConditions) != 1 || !strings.Contains(normalise(ir.JoinConditions[0]), "customers.id=orders.customer_id") {
					t.Fatalf("expected join predicate on customer id, got %+v", ir.JoinConditions)
				}
			case "Example C - CTE with window and predicate":
				if len(ir.CTEs) != 1 || strings.ToLower(ir.CTEs[0].Name) != "ranked" {
					t.Fatalf("expected ranked CTE, got %+v", ir.CTEs)
				}
				// Should now have both the base table "items" and the CTE reference "ranked"
				if len(ir.Tables) != 2 {
					t.Fatalf("expected 2 tables (items from CTE and ranked ref), got %d: %+v", len(ir.Tables), ir.Tables)
				}
				foundItems := false
				foundRanked := false
				for _, tbl := range ir.Tables {
					if tbl.Name == "items" && tbl.Type == TableTypeBase {
						foundItems = true
					}
					if strings.ToLower(tbl.Name) == "ranked" && tbl.Type == TableTypeCTE {
						foundRanked = true
					}
				}
				if !foundItems {
					t.Fatalf("expected items base table from CTE, tables: %+v", ir.Tables)
				}
				if !foundRanked {
					t.Fatalf("expected ranked CTE reference, tables: %+v", ir.Tables)
				}
				if len(ir.Where) != 1 || !strings.Contains(normalise(ir.Where[0]), "r<=10") {
					t.Fatalf("expected r <= 10 predicate, got %+v", ir.Where)
				}
			case "Example D - Update with returning":
				if ir.Command != QueryCommandUpdate {
					t.Fatalf("expected UPDATE command, got %s", ir.Command)
				}
				if len(ir.Tables) == 0 || ir.Tables[0].Name != "products" {
					t.Fatalf("expected products table, got %+v", ir.Tables)
				}
				foundSet := false
				for _, clause := range ir.SetClauses {
					if strings.Contains(normalise(clause), "price=price*1.1") {
						foundSet = true
						break
					}
				}
				if !foundSet {
					t.Fatalf("expected price set clause, got %+v", ir.SetClauses)
				}
				if len(ir.Returning) != 1 || !strings.Contains(strings.ToUpper(ir.Returning[0]), "RETURNING") {
					t.Fatalf("expected RETURNING clause, got %+v", ir.Returning)
				}
			case "Example E - Delete using join and returning":
				if ir.Command != QueryCommandDelete {
					t.Fatalf("expected DELETE command, got %s", ir.Command)
				}
				hasUsers := false
				for _, tbl := range ir.Tables {
					if tbl.Name == "users" {
						hasUsers = true
						break
					}
				}
				if !hasUsers {
					t.Fatalf("expected users in USING clause, got %+v", ir.Tables)
				}
				if len(ir.Returning) != 1 || !strings.Contains(ir.Returning[0], "sessions.id") {
					t.Fatalf("expected RETURNING sessions.id, got %+v", ir.Returning)
				}
			case "Example F - Insert with returning":
				if ir.Command != QueryCommandInsert {
					t.Fatalf("expected INSERT command, got %s", ir.Command)
				}
				if len(ir.InsertColumns) != 2 || ir.InsertColumns[0] != "id" || ir.InsertColumns[1] != "payload" {
					t.Fatalf("unexpected insert columns %+v", ir.InsertColumns)
				}
				if len(ir.Parameters) != 2 || ir.Parameters[0].Raw != "?" || ir.Parameters[1].Raw != "?" {
					t.Fatalf("expected two anonymous parameters, got %+v", ir.Parameters)
				}
				if len(ir.Returning) != 1 || !strings.Contains(ir.Returning[0], "RETURNING id") {
					t.Fatalf("expected RETURNING id clause, got %+v", ir.Returning)
				}
			default:
				t.Fatalf("unhandled case %q", tc.name)
			}
		})
	}
}

// TestIR_InsertOnConflictCapturesSetClauses validates DO UPDATE metadata.
func TestIR_InsertOnConflictCapturesSetClauses(t *testing.T) {
	sql := `
INSERT INTO account_balances (id, balance)
VALUES ($1, $2)
ON CONFLICT (id) DO UPDATE
SET balance = EXCLUDED.balance + $3,
    updated_at = NOW()
RETURNING id, balance`

	ir := parseAssertNoError(t, sql)

	if ir.Command != QueryCommandInsert {
		t.Fatalf("expected INSERT command, got %s", ir.Command)
	}
	if len(ir.InsertColumns) != 2 || ir.InsertColumns[0] != "id" || ir.InsertColumns[1] != "balance" {
		t.Fatalf("unexpected insert columns %+v", ir.InsertColumns)
	}
	if ir.Upsert == nil || ir.Upsert.Action != "DO UPDATE" {
		t.Fatalf("expected DO UPDATE upsert metadata, got %+v", ir.Upsert)
	}
	if len(ir.Upsert.TargetColumns) != 1 || ir.Upsert.TargetColumns[0] != "id" {
		t.Fatalf("unexpected upsert target columns %+v", ir.Upsert.TargetColumns)
	}
	if len(ir.Upsert.SetClauses) < 2 {
		t.Fatalf("expected set clauses for ON CONFLICT, got %+v", ir.Upsert.SetClauses)
	}
	if len(ir.Parameters) != 3 || ir.Parameters[0].Position != 1 || ir.Parameters[2].Position != 3 {
		t.Fatalf("unexpected parameters %+v", ir.Parameters)
	}
	if len(ir.Returning) != 1 || !strings.Contains(ir.Returning[0], "RETURNING id, balance") {
		t.Fatalf("expected RETURNING id, balance clause, got %+v", ir.Returning)
	}
}

// TestIR_InsertOnConflictDoNothingMetadata verifies DO NOTHING targets/filters.
func TestIR_InsertOnConflictDoNothingMetadata(t *testing.T) {
	sql := `
INSERT INTO accounts (id, email)
VALUES ($1, $2)
ON CONFLICT (id) WHERE is_active = TRUE DO NOTHING`

	ir := parseAssertNoError(t, sql)

	if ir.Command != QueryCommandInsert {
		t.Fatalf("expected INSERT command, got %s", ir.Command)
	}
	if ir.Upsert == nil {
		t.Fatalf("expected upsert metadata for DO NOTHING")
	}
	if ir.Upsert.Action != "DO NOTHING" {
		t.Fatalf("expected DO NOTHING action, got %+v", ir.Upsert)
	}
	if len(ir.Upsert.TargetColumns) != 1 || ir.Upsert.TargetColumns[0] != "id" {
		t.Fatalf("unexpected target columns %+v", ir.Upsert.TargetColumns)
	}
	if ir.Upsert.TargetWhere == "" || !strings.Contains(ir.Upsert.TargetWhere, "is_active") {
		t.Fatalf("expected target WHERE clause to capture predicate, got %+v", ir.Upsert.TargetWhere)
	}
	if len(ir.Upsert.SetClauses) != 0 {
		t.Fatalf("expected no set clauses for DO NOTHING, got %+v", ir.Upsert.SetClauses)
	}
}
