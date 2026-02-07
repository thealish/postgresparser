// parser_ir_ddl_test.go exercises DDL statement parsing at the IR level.
package postgresparser

import (
	"testing"
)

func TestIR_DDL_DropTable(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		wantActions int
		wantType    DDLActionType
		wantObject  string
		wantFlags   []string
		wantTables  int
	}{
		{
			name:        "simple",
			sql:         "DROP TABLE users",
			wantActions: 1,
			wantType:    DDLDropTable,
			wantObject:  "users",
			wantTables:  1,
		},
		{
			name:        "IF EXISTS",
			sql:         "DROP TABLE IF EXISTS users",
			wantActions: 1,
			wantType:    DDLDropTable,
			wantObject:  "users",
			wantFlags:   []string{"IF_EXISTS"},
			wantTables:  1,
		},
		{
			name:        "CASCADE",
			sql:         "DROP TABLE users CASCADE",
			wantActions: 1,
			wantType:    DDLDropTable,
			wantObject:  "users",
			wantFlags:   []string{"CASCADE"},
			wantTables:  1,
		},
		{
			name:        "schema-qualified",
			sql:         "DROP TABLE myschema.users",
			wantActions: 1,
			wantType:    DDLDropTable,
			wantObject:  "myschema.users",
			wantTables:  1,
		},
		{
			name:        "multiple tables",
			sql:         "DROP TABLE users, orders, products",
			wantActions: 3,
			wantType:    DDLDropTable,
			wantTables:  3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ir := parseAssertNoError(t, tc.sql)
			if ir.Command != QueryCommandDDL {
				t.Fatalf("expected DDL command, got %s", ir.Command)
			}
			if len(ir.DDLActions) != tc.wantActions {
				t.Fatalf("expected %d actions, got %d: %+v", tc.wantActions, len(ir.DDLActions), ir.DDLActions)
			}
			act := ir.DDLActions[0]
			if act.Type != tc.wantType {
				t.Fatalf("expected type %s, got %s", tc.wantType, act.Type)
			}
			if tc.wantObject != "" && act.ObjectName != tc.wantObject {
				t.Fatalf("expected object %q, got %q", tc.wantObject, act.ObjectName)
			}
			assertFlags(t, act.Flags, tc.wantFlags)
			if len(ir.Tables) != tc.wantTables {
				t.Fatalf("expected %d tables, got %d: %+v", tc.wantTables, len(ir.Tables), ir.Tables)
			}
		})
	}
}

func TestIR_DDL_DropIndex(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		wantActions int
		wantFlags   []string
	}{
		{
			name:        "simple",
			sql:         "DROP INDEX idx_users_email",
			wantActions: 1,
		},
		{
			name:        "CONCURRENTLY",
			sql:         "DROP INDEX CONCURRENTLY idx_users_email",
			wantActions: 1,
			wantFlags:   []string{"CONCURRENTLY"},
		},
		{
			name:        "IF EXISTS",
			sql:         "DROP INDEX IF EXISTS idx_users_email",
			wantActions: 1,
			wantFlags:   []string{"IF_EXISTS"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ir := parseAssertNoError(t, tc.sql)
			if ir.Command != QueryCommandDDL {
				t.Fatalf("expected DDL command, got %s", ir.Command)
			}
			if len(ir.DDLActions) != tc.wantActions {
				t.Fatalf("expected %d actions, got %d", tc.wantActions, len(ir.DDLActions))
			}
			act := ir.DDLActions[0]
			if act.Type != DDLDropIndex {
				t.Fatalf("expected DROP_INDEX, got %s", act.Type)
			}
			if act.ObjectName != "idx_users_email" {
				t.Fatalf("expected object idx_users_email, got %q", act.ObjectName)
			}
			assertFlags(t, act.Flags, tc.wantFlags)
		})
	}
}

func TestIR_DDL_CreateIndex(t *testing.T) {
	tests := []struct {
		name       string
		sql        string
		wantObject string
		wantCols   int
		wantFlags  []string
		wantIdx    string
		wantTables int
	}{
		{
			name:       "simple",
			sql:        "CREATE INDEX idx_email ON users (email)",
			wantObject: "idx_email",
			wantCols:   1,
			wantTables: 1,
		},
		{
			name:       "CONCURRENTLY",
			sql:        "CREATE INDEX CONCURRENTLY idx_email ON users (email)",
			wantObject: "idx_email",
			wantCols:   1,
			wantFlags:  []string{"CONCURRENTLY"},
			wantTables: 1,
		},
		{
			name:       "UNIQUE",
			sql:        "CREATE UNIQUE INDEX idx_email ON users (email)",
			wantObject: "idx_email",
			wantCols:   1,
			wantFlags:  []string{"UNIQUE"},
			wantTables: 1,
		},
		{
			name:       "USING gin",
			sql:        "CREATE INDEX idx_tags ON posts USING gin (tags)",
			wantObject: "idx_tags",
			wantCols:   1,
			wantIdx:    "gin",
			wantTables: 1,
		},
		{
			name:       "USING btree",
			sql:        "CREATE INDEX idx_name ON users USING btree (name)",
			wantObject: "idx_name",
			wantCols:   1,
			wantIdx:    "btree",
			wantTables: 1,
		},
		{
			name:       "multi-column",
			sql:        "CREATE INDEX idx_compound ON users (last_name, first_name)",
			wantObject: "idx_compound",
			wantCols:   2,
			wantTables: 1,
		},
		{
			name:       "IF NOT EXISTS",
			sql:        "CREATE INDEX IF NOT EXISTS idx_email ON users (email)",
			wantObject: "idx_email",
			wantCols:   1,
			wantFlags:  []string{"IF_NOT_EXISTS"},
			wantTables: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ir := parseAssertNoError(t, tc.sql)
			if ir.Command != QueryCommandDDL {
				t.Fatalf("expected DDL command, got %s", ir.Command)
			}
			if len(ir.DDLActions) != 1 {
				t.Fatalf("expected 1 action, got %d", len(ir.DDLActions))
			}
			act := ir.DDLActions[0]
			if act.Type != DDLCreateIndex {
				t.Fatalf("expected CREATE_INDEX, got %s", act.Type)
			}
			if act.ObjectName != tc.wantObject {
				t.Fatalf("expected object %q, got %q", tc.wantObject, act.ObjectName)
			}
			if len(act.Columns) != tc.wantCols {
				t.Fatalf("expected %d columns, got %d: %v", tc.wantCols, len(act.Columns), act.Columns)
			}
			assertFlags(t, act.Flags, tc.wantFlags)
			if tc.wantIdx != "" && act.IndexType != tc.wantIdx {
				t.Fatalf("expected index type %q, got %q", tc.wantIdx, act.IndexType)
			}
			if len(ir.Tables) != tc.wantTables {
				t.Fatalf("expected %d tables, got %d", tc.wantTables, len(ir.Tables))
			}
		})
	}
}

func TestIR_DDL_AlterTableDropColumn(t *testing.T) {
	tests := []struct {
		name      string
		sql       string
		wantCol   string
		wantFlags []string
	}{
		{
			name:    "simple",
			sql:     "ALTER TABLE users DROP COLUMN email",
			wantCol: "email",
		},
		{
			name:      "IF EXISTS",
			sql:       "ALTER TABLE users DROP COLUMN IF EXISTS email",
			wantCol:   "email",
			wantFlags: []string{"IF_EXISTS"},
		},
		{
			name:      "CASCADE",
			sql:       "ALTER TABLE users DROP COLUMN email CASCADE",
			wantCol:   "email",
			wantFlags: []string{"CASCADE"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ir := parseAssertNoError(t, tc.sql)
			if ir.Command != QueryCommandDDL {
				t.Fatalf("expected DDL command, got %s", ir.Command)
			}
			if len(ir.DDLActions) != 1 {
				t.Fatalf("expected 1 action, got %d: %+v", len(ir.DDLActions), ir.DDLActions)
			}
			act := ir.DDLActions[0]
			if act.Type != DDLDropColumn {
				t.Fatalf("expected DROP_COLUMN, got %s", act.Type)
			}
			if len(act.Columns) != 1 || act.Columns[0] != tc.wantCol {
				t.Fatalf("expected column %q, got %v", tc.wantCol, act.Columns)
			}
			assertFlags(t, act.Flags, tc.wantFlags)
			if !containsTable(ir.Tables, "users") {
				t.Fatalf("expected table 'users' in Tables, got %+v", ir.Tables)
			}
		})
	}
}

func TestIR_DDL_AlterTableAddColumn(t *testing.T) {
	ir := parseAssertNoError(t, "ALTER TABLE users ADD COLUMN status text")
	if ir.Command != QueryCommandDDL {
		t.Fatalf("expected DDL command, got %s", ir.Command)
	}
	if len(ir.DDLActions) != 1 {
		t.Fatalf("expected 1 action, got %d: %+v", len(ir.DDLActions), ir.DDLActions)
	}
	act := ir.DDLActions[0]
	if act.Type != DDLAlterTable {
		t.Fatalf("expected ALTER_TABLE, got %s", act.Type)
	}
	if len(act.Columns) != 1 || act.Columns[0] != "status" {
		t.Fatalf("expected column [status], got %v", act.Columns)
	}
	assertFlagPresent(t, act.Flags, "ADD_COLUMN")
}

func TestIR_DDL_AlterTableMultiAction(t *testing.T) {
	ir := parseAssertNoError(t, "ALTER TABLE users ADD COLUMN status text, DROP COLUMN legacy")
	if ir.Command != QueryCommandDDL {
		t.Fatalf("expected DDL command, got %s", ir.Command)
	}
	if len(ir.DDLActions) != 2 {
		t.Fatalf("expected 2 actions, got %d: %+v", len(ir.DDLActions), ir.DDLActions)
	}
	// First action: ADD COLUMN
	if ir.DDLActions[0].Type != DDLAlterTable {
		t.Fatalf("expected ALTER_TABLE for first action, got %s", ir.DDLActions[0].Type)
	}
	assertFlagPresent(t, ir.DDLActions[0].Flags, "ADD_COLUMN")
	// Second action: DROP COLUMN
	if ir.DDLActions[1].Type != DDLDropColumn {
		t.Fatalf("expected DROP_COLUMN for second action, got %s", ir.DDLActions[1].Type)
	}
	if len(ir.DDLActions[1].Columns) != 1 || ir.DDLActions[1].Columns[0] != "legacy" {
		t.Fatalf("expected column [legacy], got %v", ir.DDLActions[1].Columns)
	}
}

func TestIR_DDL_Truncate(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		wantActions int
		wantFlags   []string
		wantTables  int
	}{
		{
			name:        "simple",
			sql:         "TRUNCATE users",
			wantActions: 1,
			wantTables:  1,
		},
		{
			name:        "TABLE keyword",
			sql:         "TRUNCATE TABLE users",
			wantActions: 1,
			wantTables:  1,
		},
		{
			name:        "CASCADE",
			sql:         "TRUNCATE TABLE users CASCADE",
			wantActions: 1,
			wantFlags:   []string{"CASCADE"},
			wantTables:  1,
		},
		{
			name:        "multiple tables",
			sql:         "TRUNCATE users, orders",
			wantActions: 2,
			wantTables:  2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ir := parseAssertNoError(t, tc.sql)
			if ir.Command != QueryCommandDDL {
				t.Fatalf("expected DDL command, got %s", ir.Command)
			}
			if len(ir.DDLActions) != tc.wantActions {
				t.Fatalf("expected %d actions, got %d", tc.wantActions, len(ir.DDLActions))
			}
			for _, act := range ir.DDLActions {
				if act.Type != DDLTruncate {
					t.Fatalf("expected TRUNCATE type, got %s", act.Type)
				}
			}
			assertFlags(t, ir.DDLActions[0].Flags, tc.wantFlags)
			if len(ir.Tables) != tc.wantTables {
				t.Fatalf("expected %d tables, got %d", tc.wantTables, len(ir.Tables))
			}
		})
	}
}

// assertFlags checks that all expected flags are present in the actual flags slice.
func assertFlags(t *testing.T, actual, expected []string) {
	t.Helper()
	for _, want := range expected {
		found := false
		for _, got := range actual {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected flag %q in %v", want, actual)
		}
	}
}

// assertFlagPresent checks that a specific flag is present.
func assertFlagPresent(t *testing.T, flags []string, flag string) {
	t.Helper()
	for _, f := range flags {
		if f == flag {
			return
		}
	}
	t.Fatalf("expected flag %q in %v", flag, flags)
}
