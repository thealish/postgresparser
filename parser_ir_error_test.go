// parser_ir_error_test.go verifies structured error reporting from ParseSQL.
package postgresparser

import (
	"strings"
	"testing"
)

// TestIR_ErrorHandling checks a single syntax error includes line/column info.
func TestIR_ErrorHandling(t *testing.T) {
	badSQL := `SELECT FROM WHERE broken = true`
	_, err := ParseSQL(badSQL)
	if err == nil {
		t.Fatalf("expected parse error, got nil")
	}
	perr, ok := err.(*ParseErrors)
	if !ok {
		t.Fatalf("expected ParseErrors type, got %T", err)
	}
	if len(perr.Errors) == 0 {
		t.Fatalf("expected at least one syntax error entry, got %+v", perr)
	}
	first := perr.Errors[0]
	if first.Line <= 0 || first.Column < 0 || first.Message == "" {
		t.Fatalf("expected detailed syntax error, got %+v", first)
	}
}

// TestIR_ErrorHandlingMultiple verifies aggregated errors still expose line info.
func TestIR_ErrorHandlingMultiple(t *testing.T) {
	badSQL := `
SELECT *
FROM
UNION
SELECT`
	_, err := ParseSQL(badSQL)
	if err == nil {
		t.Fatalf("expected parse error, got nil")
	}
	perr, ok := err.(*ParseErrors)
	if !ok {
		t.Fatalf("expected ParseErrors type, got %T", err)
	}
	if len(perr.Errors) == 0 {
		t.Fatalf("expected at least one syntax error entry, got %+v", perr)
	}
	if !strings.Contains(perr.Error(), "line") {
		t.Fatalf("expected error message to include line information, got %q", perr.Error())
	}
}
