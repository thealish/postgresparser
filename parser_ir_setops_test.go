// parser_ir_setops_test.go focuses on UNION/INTERSECT/EXCEPT metadata.
package postgresparser

import (
	"strings"
	"testing"
)

// TestIR_SetOperationsMetadata checks multi-branch set-ops are captured in order.
func TestIR_SetOperationsMetadata(t *testing.T) {
	sql := `
(
  SELECT user_id
  FROM logins
  INTERSECT
  SELECT user_id
  FROM payments
)
EXCEPT ALL
SELECT user_id
FROM banned_users;`

	ir := parseAssertNoError(t, sql)

	if len(ir.SetOperations) != 2 {
		t.Fatalf("expected two set operations, got %+v", ir.SetOperations)
	}

	first := ir.SetOperations[0]
	if first.Type != "INTERSECT" {
		t.Fatalf("expected first operation INTERSECT, got %q", first.Type)
	}
	if len(first.Columns) != 1 || first.Columns[0] != "user_id" {
		t.Fatalf("unexpected INTERSECT columns %+v", first.Columns)
	}
	if !strings.Contains(strings.ToLower(first.Query), "from payments") {
		t.Fatalf("expected INTERSECT query to contain payments, got %q", first.Query)
	}
	if len(first.Tables) != 1 || strings.ToLower(first.Tables[0].Name) != "payments" {
		t.Fatalf("expected payments table captured for INTERSECT, got %+v", first.Tables)
	}

	second := ir.SetOperations[1]
	if second.Type != "EXCEPT ALL" {
		t.Fatalf("expected second operation EXCEPT ALL, got %q", second.Type)
	}
	if len(second.Columns) != 1 || second.Columns[0] != "user_id" {
		t.Fatalf("unexpected EXCEPT columns %+v", second.Columns)
	}
	if !strings.Contains(strings.ToLower(second.Query), "from banned_users") {
		t.Fatalf("expected EXCEPT query to contain banned_users, got %q", second.Query)
	}
	if len(second.Tables) != 1 || strings.ToLower(second.Tables[0].Name) != "banned_users" {
		t.Fatalf("expected banned_users table captured for EXCEPT, got %+v", second.Tables)
	}
	if !containsTable(ir.Tables, "logins") || !containsTable(ir.Tables, "payments") || !containsTable(ir.Tables, "banned_users") {
		t.Fatalf("expected all set-operation tables surfaced at top level, got %+v", ir.Tables)
	}
}

// TestIR_SetOperationTableDriven sanity-checks table capture across common set ops.
func TestIR_SetOperationTableDriven(t *testing.T) {
	tests := []struct {
		name             string
		sql              string
		expectedTables   []string
		expectedOpTables []string
	}{
		{
			name: "Union distinct across staging tables",
			sql: `
SELECT account_id FROM current_accounts
UNION
SELECT account_id FROM archived_accounts`,
			expectedTables:   []string{"current_accounts", "archived_accounts"},
			expectedOpTables: []string{"archived_accounts"},
		},
		{
			name: "Intersect dedupe check",
			sql: `
SELECT user_id FROM service_a_events
INTERSECT ALL
SELECT user_id FROM service_b_events`,
			expectedTables:   []string{"service_a_events", "service_b_events"},
			expectedOpTables: []string{"service_b_events"},
		},
		{
			name: "Except eliminates revoked permissions",
			sql: `
SELECT user_id FROM granted_permissions
EXCEPT
SELECT user_id FROM revoked_permissions`,
			expectedTables:   []string{"granted_permissions", "revoked_permissions"},
			expectedOpTables: []string{"revoked_permissions"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ir := parseAssertNoError(t, tc.sql)

			if len(ir.SetOperations) == 0 {
				t.Fatalf("expected set operations metadata, got none")
			}

			for _, tbl := range tc.expectedTables {
				if !containsTable(ir.Tables, tbl) {
					t.Fatalf("expected table %q in top-level tables %+v", tbl, ir.Tables)
				}
			}

			for _, tbl := range tc.expectedOpTables {
				found := false
				for _, op := range ir.SetOperations {
					if containsTable(op.Tables, tbl) {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("expected table %q in set operations %+v", tbl, ir.SetOperations)
				}
			}
		})
	}
}
