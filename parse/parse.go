package parse

import "github.com/bjwbell/gir/scan"

// Copied from robpike.io/ivy/parse

// Parser stores the state for the ssair parser.
type Parser struct {
	scanner    *scan.Scanner
	fileName   string
	lineNum    int
	errorCount int // Number of errors.
	peekTok    scan.Token
	curTok     scan.Token // most recent token from scanner
}
