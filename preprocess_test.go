package postgresparser

import "testing"

func TestPreprocessRemovesDoubleSlashComments(t *testing.T) {
	sql := "SELECT * FROM users // custom meta"
	cleaned := preprocessSQLInput(sql)

	if cleaned != "SELECT * FROM users" {
		t.Fatalf("unexpected cleaned SQL %q", cleaned)
	}
}

func TestPreprocessPreservesUrlLiterals(t *testing.T) {
	sql := "SELECT 'http://example.com' AS url // comment"
	cleaned := preprocessSQLInput(sql)

	expected := "SELECT 'http://example.com' AS url"
	if cleaned != expected {
		t.Fatalf("expected %q, got %q", expected, cleaned)
	}
}

func TestPreprocessPreservesDollarQuotedString(t *testing.T) {
	sql := "SELECT $$http://example.com//keep$$ AS url // comment"
	cleaned := preprocessSQLInput(sql)

	expected := "SELECT $$http://example.com//keep$$ AS url"
	if cleaned != expected {
		t.Fatalf("expected %q, got %q", expected, cleaned)
	}
}

func TestPreprocessHandlesMultilineDollarQuotedString(t *testing.T) {
	sql := "SELECT $$line1\n//not comment\nline3$$ AS body // trailing"
	cleaned := preprocessSQLInput(sql)

	expected := "SELECT $$line1\n//not comment\nline3$$ AS body"
	if cleaned != expected {
		t.Fatalf("expected %q, got %q", expected, cleaned)
	}
}

func TestParseDollarTagValid(t *testing.T) {
	runes := []rune("$foo$rest")
	tag, ok := parseDollarTag(runes, 0)
	if !ok || tag != "foo" {
		t.Fatalf("expected tag foo, got %q ok=%v", tag, ok)
	}
}

func TestParseDollarTagInvalidChar(t *testing.T) {
	runes := []rune("$foo-bar$")
	if tag, ok := parseDollarTag(runes, 0); ok || tag != "" {
		t.Fatalf("expected failure, got tag=%q ok=%v", tag, ok)
	}
}

func TestHasDollarTerminator(t *testing.T) {
	runes := []rune("text$foo$rest")
	if !hasDollarTerminator(runes, 4, "foo") {
		t.Fatalf("expected terminator to be detected")
	}
	if hasDollarTerminator(runes, 4, "bar") {
		t.Fatalf("expected terminator mismatch")
	}
}

func TestPreprocessTrimsWhitespace(t *testing.T) {
	sql := "  SELECT id FROM users   \n"
	cleaned := preprocessSQLInput(sql)

	if cleaned != "SELECT id FROM users" {
		t.Fatalf("expected trimmed SQL, got %q", cleaned)
	}
}
