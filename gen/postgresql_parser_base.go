// Code generated from the upstream ANTLR PostgreSQL grammar support files.
// Adapted for the Valk SQL parser Go target.

package gen

import (
	"strings"

	"github.com/antlr4-go/antlr/v4"
)

// PostgreSQLParserBase provides the helper hooks referenced by grammar actions.
type PostgreSQLParserBase struct {
	*antlr.BaseParser
}

func NewPostgreSQLParserBase(input antlr.TokenStream) *PostgreSQLParserBase {
	return &PostgreSQLParserBase{
		BaseParser: antlr.NewBaseParser(input),
	}
}

func (p *PostgreSQLParserBase) ParseRoutineBody() {
	// Placeholder for routine body parsing hook from upstream grammar.
	// The detailed implementation relies on PL/pgSQL parsing which we do not
	// currently require for slow query analysis.
}

func TrimQuotes(s string) string {
	if s == "" {
		return s
	}
	// Mirrors upstream helper: drop leading quote and the trailing quote/escape.
	if len(s) < 3 {
		return ""
	}
	return s[1 : len(s)-2]
}

func unquote(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		ch := s[i]
		b.WriteByte(ch)
		if ch == '\'' && i < len(s)-1 && s[i+1] == '\'' {
			i++
		}
	}
	return b.String()
}

func GetRoutineBodyString(rule *SconstContext) string {
	if rule == nil || rule.Anysconst() == nil {
		return ""
	}
	any := rule.Anysconst().(*AnysconstContext)

	if s := any.StringConstant(); s != nil {
		return unquote(TrimQuotes(s.GetText()))
	}
	if s := any.UnicodeEscapeStringConstant(); s != nil {
		return TrimQuotes(s.GetText())
	}
	if s := any.EscapeStringConstant(); s != nil {
		return TrimQuotes(s.GetText())
	}

	var b strings.Builder
	for _, node := range any.AllDollarText() {
		b.WriteString(node.GetText())
	}
	return b.String()
}

func (p *PostgreSQLParserBase) OnlyAcceptableOps() bool {
	token := p.GetTokenStream().LT(1)
	if token == nil {
		return false
	}
	switch token.GetText() {
	case "!", "!!", "!=-":
		return true
	default:
		return false
	}
}
