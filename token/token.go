package token

import (
	"fmt"
)

type Type int

type Token struct {
	Type Type
	Line int
	Text string
}

//go:generate stringer -type=Type

const (
	EOF   Type = iota // zero value so closed channel delivers EOF
	Error             // error occurred; value is text of error
	Newline
	// types of tokens
	Func       // 'func'
	Assign     // '='
	Char       // printable ASCII character; grab bag for comma etc.
	Identifier // alphanumeric identifier
	Number     // simple number
	Operator   // known operator
	Op         // "op", operator keyword
	Rational   // rational number like 2/3
	LeftParen  // '('
	RightParen // ')'
	LeftBrace  // '{'
	RightBrace // '}'
	Semicolon  // ';'
	LeftBrack  // '['
	RightBrack // ']'
	String     // quoted string (includes quotes)
)

func (i Token) String() string {
	switch {
	case i.Type == EOF:
		return "EOF"
	case i.Type == Error:
		return "error: " + i.Text
	case len(i.Text) > 10:
		return fmt.Sprintf("%v: %.10q...", i.Type, i.Text)
	}
	return fmt.Sprintf("%v: %q", i.Type, i.Text)
}
