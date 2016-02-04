package scan

import (
	"fmt"
	"io"
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

type Scanner struct {
	tokens chan Token // channel of scanned items
	r      io.ByteReader
	done   bool
	name   string // the name of the input; used only for error reports
	buf    []byte
	input  string // the line of text being scanned.
	line   int    // line number in input
	pos    int    // current position in the input
	start  int    // start position of this item
	width  int    // width of last rune read from input
}
