// Code generated from the upstream ANTLR PostgreSQL grammar support files.
// Adapted for the Valk SQL parser Go target.

package gen

import (
	"errors"
	"unicode"

	"github.com/antlr4-go/antlr/v4"
)

var errStackEmpty = errors.New("stack empty")

// StringStack mirrors the helper used by the upstream grammar actions.
type StringStack struct {
	items []string
}

func (s *StringStack) Push(val string) {
	s.items = append(s.items, val)
}

func (s *StringStack) Pop() (string, error) {
	if s.IsEmpty() {
		return "", errStackEmpty
	}
	val := s.items[0]
	s.items = s.items[1:]
	return val, nil
}

func (s *StringStack) PopOrEmpty() string {
	val, err := s.Pop()
	if err != nil {
		return ""
	}
	return val
}

func (s *StringStack) Peek() (string, error) {
	if s.IsEmpty() {
		return "", errStackEmpty
	}
	return s.items[0], nil
}

func (s *StringStack) PeekOrEmpty() string {
	val, err := s.Peek()
	if err != nil {
		return ""
	}
	return val
}

func (s *StringStack) IsEmpty() bool {
	return len(s.items) == 0
}

// PostgreSQLLexerBase embeds the ANTLR base lexer and exposes helper hooks
// referenced by the grammar actions.
type PostgreSQLLexerBase struct {
	*antlr.BaseLexer
	stack StringStack
}

func NewPostgreSQLLexerBase(input antlr.CharStream) *PostgreSQLLexerBase {
	return &PostgreSQLLexerBase{
		BaseLexer: antlr.NewBaseLexer(input),
	}
}

func (l *PostgreSQLLexerBase) PushTag() {
	l.stack.Push(l.GetText())
}

func (l *PostgreSQLLexerBase) IsTag() bool {
	if l.stack.IsEmpty() {
		return false
	}
	return l.GetText() == l.stack.PeekOrEmpty()
}

func (l *PostgreSQLLexerBase) PopTag() {
	l.stack.Pop()
}

func (l *PostgreSQLLexerBase) CheckLaMinus() bool {
	return l.GetInputStream().LA(1) != '-'
}

func (l *PostgreSQLLexerBase) CheckLaStar() bool {
	return l.GetInputStream().LA(1) != '*'
}

func (l *PostgreSQLLexerBase) CharIsLetter() bool {
	ch := l.GetInputStream().LA(-1)
	return unicode.IsLetter(rune(ch))
}

func (l *PostgreSQLLexerBase) HandleNumericFail() {
	index := l.GetInputStream().Index() - 2
	l.GetInputStream().Seek(index)
	l.SetType(PostgreSQLLexerIntegral)
}

func (l *PostgreSQLLexerBase) HandleLessLessGreaterGreater() {
	switch l.GetText() {
	case "<<":
		l.SetType(PostgreSQLLexerLESS_LESS)
	case ">>":
		l.SetType(PostgreSQLLexerGREATER_GREATER)
	}
}

func (l *PostgreSQLLexerBase) UnterminatedBlockCommentDebugAssert() {
	// Placeholder for upstream debug hook; no-op in Go target.
}

func (l *PostgreSQLLexerBase) CheckIfUtf32Letter() bool {
	stream := l.GetInputStream()
	codePoint := stream.LA(-2)<<8 + stream.LA(-1)
	var chars []rune
	if codePoint < 0x10000 {
		chars = []rune{rune(codePoint)}
	} else {
		codePoint -= 0x10000
		chars = []rune{
			rune(codePoint/0x400 + 0xd800),
			rune(codePoint%0x400 + 0xdc00),
		}
	}
	return unicode.IsLetter(chars[0])
}

func (l *PostgreSQLLexerBase) IsSemiColon() bool {
	return l.GetInputStream().LA(1) == ';'
}
