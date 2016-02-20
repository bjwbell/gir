package scan

import (
	"fmt"
	"io"

	"github.com/bjwbell/gir/value"
)

// Copied from robpike.io/ivy/scan

type Type int

type Token struct {
	Type Type
	Line int
	Text string
}

const (
	EOF   Type = iota // zero value so closed channel delivers EOF
	Error             // error occurred; value is text of error
	Newline
	// types of tokens
	Assign     // '='
	Identifier // alphanumeric identifier
	Number     // simple number
	Op         // "op", operator keyword
	RightParen // ')'
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

// stateFn represents the state of the scanner as a function that returns the next state.
type stateFn func(*Scanner) stateFn

type Scanner struct {
	tokens  chan Token // channel of scanned items
	context value.Context
	r       io.ByteReader
	done    bool
	name    string // the name of the input; used only for error reports
	buf     []byte
	input   string  // the line of text being scanned.
	state   stateFn // the next lexing function to enter
	line    int     // line number in input
	pos     int     // current position in the input
	start   int     // start position of this item
	width   int     // width of last rune read from input
}

// New creates a new scanner for the input string.
func New(context value.Context, name string, r io.ByteReader) *Scanner {
	l := &Scanner{
		r:       r,
		name:    name,
		line:    1,
		tokens:  make(chan Token, 2), // We need a little room to save tokens.
		context: context,
		state:   lexAny,
	}
	return l
}

// lexAny scans non-space items.
func lexAny(l *Scanner) stateFn {
	fmt.Println("lexAny: TODO")
	return nil
}

// Next returns the next token.
func (l *Scanner) Next() Token {
	// The lexer is concurrent but we don't want it to run in parallel
	// with the rest of the interpreter, so we only run the state machine
	// when we need a token.
	for l.state != nil {
		select {
		case tok := <-l.tokens:
			return tok
		default:
			// Run the machine
			l.state = l.state(l)
		}
	}
	if l.tokens != nil {
		close(l.tokens)
		l.tokens = nil
	}
	return Token{EOF, l.pos, "EOF"}
}
