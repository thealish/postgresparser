// preprocess.go sanitizes raw SQL input before parsing. It normalizes
// whitespace, strips trailing semicolons, and handles edge cases like
// dollar-quoted strings and string-literal escaping.
package postgresparser

import (
	"strings"
	"unicode"
)

// quoteState tracks string literal state while preprocessing SQL.
type quoteState struct {
	inSingle  bool
	inDouble  bool
	inDollar  bool
	dollarTag string
}

// preprocessSQLInput trims the statement and removes trailing // comments
// that our grammar does not understand (custom metadata appended to the query),
// while preserving PostgreSQL string literal semantics (including dollar-quoted strings).
func preprocessSQLInput(sql string) string {
	if sql == "" {
		return ""
	}

	state := &quoteState{}
	lines := strings.Split(sql, "\n")
	for i, line := range lines {
		stripped := stripDoubleSlashComment(line, state)
		stripped = strings.TrimRightFunc(stripped, unicode.IsSpace)
		lines[i] = stripped
	}

	cleaned := strings.Join(lines, "\n")
	return strings.TrimSpace(cleaned)
}

// stripDoubleSlashComment removes a trailing // comment from a line while respecting string literals
// (single, double, and dollar-quoted strings that may span lines).
func stripDoubleSlashComment(line string, state *quoteState) string {
	var b strings.Builder
	runes := []rune(line)

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		// Handle dollar-quoted strings first.
		if state.inDollar {
			b.WriteRune(r)
			if r == '$' && hasDollarTerminator(runes, i, state.dollarTag) {
				tagRuneLen := len([]rune(state.dollarTag))
				for j := 1; j < tagRuneLen+2 && i+j < len(runes); j++ {
					b.WriteRune(runes[i+j])
				}
				i += tagRuneLen + 1
				state.inDollar = false
				state.dollarTag = ""
			}
			continue
		}

		switch r {
		case '\'':
			if !state.inDouble {
				b.WriteRune(r)
				if state.inSingle && i+1 < len(runes) && runes[i+1] == '\'' {
					i++
					b.WriteRune(runes[i])
					continue
				}
				state.inSingle = !state.inSingle
				continue
			}
		case '"':
			if !state.inSingle {
				b.WriteRune(r)
				state.inDouble = !state.inDouble
				continue
			}
		case '$':
			if !state.inSingle && !state.inDouble {
				if tag, ok := parseDollarTag(runes, i); ok {
					state.inDollar = true
					state.dollarTag = tag
					terminatorLen := len([]rune(tag)) + 2
					for j := 0; j < terminatorLen && i+j < len(runes); j++ {
						b.WriteRune(runes[i+j])
					}
					i += terminatorLen - 1
					continue
				}
			}
		case '/':
			if !state.inSingle && !state.inDouble && state.dollarTag == "" && i+1 < len(runes) && runes[i+1] == '/' {
				return b.String()
			}
		}

		b.WriteRune(r)
	}

	return b.String()
}

// parseDollarTag inspects runes starting at start and returns the tag used in a
// dollar-quoted literal ($tag$). It returns false if the sequence does not form a valid tag.
func parseDollarTag(runes []rune, start int) (string, bool) {
	if start >= len(runes) || runes[start] != '$' {
		return "", false
	}
	tag := strings.Builder{}
	for i := start + 1; i < len(runes); i++ {
		switch runes[i] {
		case '$':
			return tag.String(), true
		default:
			if !isDollarTagChar(runes[i]) {
				return "", false
			}
			tag.WriteRune(runes[i])
		}
	}
	return "", false
}

// hasDollarTerminator checks whether the runes slice at index start contains $tag$.
func hasDollarTerminator(runes []rune, start int, tag string) bool {
	tagRunes := []rune(tag)
	needed := len(tagRunes) + 2
	if start+needed > len(runes) {
		return false
	}
	for i := 0; i < needed; i++ {
		switch i {
		case 0, needed - 1:
			if runes[start+i] != '$' {
				return false
			}
		default:
			if tagRunes[i-1] != runes[start+i] {
				return false
			}
		}
	}
	return true
}

// isDollarTagChar reports whether a rune is valid within a dollar-quote tag name.
func isDollarTagChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}
