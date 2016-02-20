package parse

import (
	"fmt"

	"github.com/bjwbell/gir/scan"
	"github.com/bjwbell/gir/value"
)

// Copied from robpike.io/ivy/parse

// Parser stores the state for the ssair parser.
type Parser struct {
	scanner    *scan.Scanner
	fileName   string
	lineNum    int
	errorCount int // Number of errors.
	peekTok    scan.Token
	curTok     scan.Token // most recent token from scanner
	context    value.Context
}

// NewParser returns a new parser that will read from the scanner.
// The context must have have been created by this package's NewContext function.
func NewParser(fileName string, scanner *scan.Scanner, context value.Context) *Parser {
	return &Parser{
		scanner:  scanner,
		fileName: fileName,
		context:  context,
	}
}

// FlushToNewline any remaining characters on the current input line.
func (p *Parser) FlushToNewline() {
	for p.curTok.Type != scan.Newline && p.curTok.Type != scan.EOF {
		p.nextErrorOut(false)
	}
}

// nextErrorOut accepts a flag whether to trigger a panic on error.
// The flag is set to false when we are draining input tokens in FlushToNewline.
func (p *Parser) nextErrorOut(errorOut bool) scan.Token {
	tok := p.peekTok
	if tok.Type != scan.EOF {
		p.peekTok = scan.Token{Type: scan.EOF}
	} else {
		tok = p.scanner.Next()
	}
	if tok.Type == scan.Error && errorOut {
		p.errorf("%q", tok)
	}
	p.curTok = tok
	if tok.Type != scan.Newline {
		// Show the line number before we hit the newline.
		p.lineNum = tok.Line
	}
	return tok
}

func (p *Parser) errorf(format string, args ...interface{}) {
	p.peekTok = scan.Token{Type: scan.EOF}
	value.Errorf(format, args...)
}

// Loc returns the current input location in the form "name:line: ".
// If the name is <stdin>, it returns the empty string.
func (p *Parser) Loc() string {
	if p.fileName == "<stdin>" {
		return ""
	}
	return fmt.Sprintf("%s:%d: ", p.fileName, p.lineNum)
}

func (p *Parser) peek() scan.Token {
	tok := p.peekTok
	if tok.Type != scan.EOF {
		return tok
	}
	p.peekTok = p.scanner.Next()
	return p.peekTok
}

// Line reads a line of input and returns the values it evaluates.
// A nil returned slice means there were no values.
// The boolean reports whether the line is valid.
//
// Line
//	) special command '\n'
//	def function defintion
//	expressionList '\n'
func (p *Parser) Line() ([]value.Expr, bool) {
	tok := p.peek()
	switch tok.Type {
	case scan.RightParen:
		// TODO
		return nil, true
	case scan.Op:
		// TODO
		return nil, true
	}
	// TODO
	exprs, ok := []value.Expr(nil), true
	if !ok {
		return nil, false
	}
	return exprs, true
}
