package parse

import (
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
