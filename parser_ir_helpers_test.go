// parser_ir_helpers_test.go provides shared helpers for parser IR tests.
package postgresparser

import (
	"strings"
	"testing"
)

// parseAssertNoError parses SQL and fails the test if an error occurs.
func parseAssertNoError(t *testing.T, sql string) *ParsedQuery {
	t.Helper()
	q, err := ParseSQL(sql)
	if err != nil {
		t.Fatalf("ParseSQL(%q) returned error: %v", sql, err)
	}
	if q == nil {
		t.Fatalf("ParseSQL(%q) returned nil query", sql)
	}
	return q
}

// containsTable reports whether a table list includes the supplied name (case-insensitive).
func containsTable(tables []TableRef, name string) bool {
	target := strings.ToLower(name)
	for _, tbl := range tables {
		if strings.ToLower(tbl.Name) == target {
			return true
		}
	}
	return false
}

// TestSplitQualifiedName verifies schema/name splitting including quoted identifiers.
func TestSplitQualifiedName(t *testing.T) {
	tests := []struct {
		input      string
		wantSchema string
		wantName   string
	}{
		{"", "", ""},
		{"users", "", "users"},
		{"public.users", "public", "users"},
		{"mydb.public.users", "mydb.public", "users"},
		{`"my.schema"."my.table"`, `"my.schema"`, `"my.table"`},
		{`"dotted.schema".simple_table`, `"dotted.schema"`, "simple_table"},
		{`simple_schema."dotted.table"`, "simple_schema", `"dotted.table"`},
	}
	for _, tt := range tests {
		schema, name := splitQualifiedName(tt.input)
		if schema != tt.wantSchema || name != tt.wantName {
			t.Errorf("splitQualifiedName(%q) = (%q, %q), want (%q, %q)",
				tt.input, schema, name, tt.wantSchema, tt.wantName)
		}
	}
}

// normalise collapses whitespace and lowercases strings for comparison convenience.
func normalise(s string) string {
	compact := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(s), " ", ""), "\n", ""), "\t", "")
	return strings.ToLower(compact)
}
