// COPIED FROM robpike.io/ivy/parse

package parse

import (
	"fmt"

	"github.com/bjwbell/gir/ast"
	"github.com/bjwbell/gir/scan"
	"github.com/bjwbell/gir/token"
	"github.com/bjwbell/gir/value"
)

// tree formats an expression in an unambiguous form for debugging.
func Tree(e interface{}) string {
	switch e := e.(type) {
	case *ast.File:
		return fmt.Sprintf("(package %s %s)", e.Name, Tree(e.Decls))
	case []ast.FuncDecl:
		s := ""
		for _, fn := range e {
			s +=fmt.Sprintf("func %s\n %s\n", fn.Name, Tree(fn.Epxrs))
		}
		return s
	case value.Int:
		return fmt.Sprintf("<int %s>", e)
	case variableExpr:
		return fmt.Sprintf("<var %s>", e.name)
	case *unary:
		return fmt.Sprintf("(%s %s)", e.op, Tree(e.right))
	case *binary:
		// Special case for [].
		if e.op == "[]" {
			return fmt.Sprintf("(%s[%s])", Tree(e.left), Tree(e.right))
		}
		return fmt.Sprintf("(%s %s %s)", Tree(e.left), e.op, Tree(e.right))
	case sliceExpr:
		s := "<TODO>"
		return s
	case []value.Expr:
		if len(e) == 1 {
			return Tree(e[0])
		}
		s := "<"
		for i, expr := range e {
			if i > 0 {
				s += "; "
			}
			s += Tree(expr)
		}
		s += ">"
		return s
	default:
		return fmt.Sprintf("%T", e)
	}
}


// sliceExpr holds a syntactic vector to be verified and evaluated.
type sliceExpr []value.Expr


func (s sliceExpr) ProgString() string {
	// TODO
	return "<sliceExpr>"
}

// variableExpr identifies a variable to be looked up and evaluated.
type variableExpr struct {
	name string
}

func (e variableExpr) Eval(context value.Context) value.Value {
	// TODO
	return nil
}

func (e variableExpr) ProgString() string {
	return e.name
}



type unary struct {
	op    string
	right interface{}
}


type binary struct {
	op    string
	left  interface{}
	right interface{}
}

func (b *binary) ProgString() string {
	// TODO
	return "<binary>"
}

func (b *binary) Eval(context value.Context) value.Value {
	// TODO
	return nil
}

// Parser stores the state of the parser.
type Parser struct {
	scanner    *scan.Scanner
	fileName   string
	lineNum    int
	errorCount int // Number of errors.
	peekTok    token.Token
	curTok     token.Token // most recent token from scanner
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


// Println prints the args and writes them to the configured output writer.
func (p *Parser) Println(args ...interface{}) {
	fmt.Fprintln(p.context.Config().Output(), args...)
}

// FlushToNewline any remaining characters on the current input line.
func (p *Parser) FlushToNewline() {
	for p.curTok.Type != token.Newline && p.curTok.Type != token.EOF {
		p.nextErrorOut(false)
	}
}

func (p *Parser) next() token.Token {
	return p.nextErrorOut(true)
}

// nextErrorOut accepts a flag whether to trigger a panic on error.
// The flag is set to false when we are draining input tokens in FlushToNewline.
func (p *Parser) nextErrorOut(errorOut bool) token.Token {
	tok := p.peekTok
	if tok.Type != token.EOF {
		p.peekTok = token.Token{Type: token.EOF}
	} else {
		tok = p.scanner.Next()
	}
	if tok.Type == token.Error && errorOut {
		p.errorf("%q", tok)
	}
	p.curTok = tok
	if tok.Type != token.Newline {
		// Show the line number before we hit the newline.
		p.lineNum = tok.Line
	}
	return tok
}

func (p *Parser) errorf(format string, args ...interface{}) {
	p.peekTok = token.Token{Type: token.EOF}
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

func (p *Parser) peek() token.Token {
	tok := p.peekTok
	if tok.Type != token.EOF {
		return tok
	}
	p.peekTok = p.scanner.Next()
	return p.peekTok
}



func (p *Parser) error(msg string) {
	// TODO: Improve
	panic(msg)
}

func (p *Parser) expectTok(t token.Type) (bool, token.Token) {
	tok := p.next() // make progress
	if tok.Type == t {
		return true, tok
	} else {
		return false, tok
	}
}


func (p *Parser) expect(tok token.Token) (bool, token.Token) {
	t := p.next() // make progress
	
	if tok.Type == t.Type && tok.Text == t.Text {
		return true, t
	} else {
		return false, t
	}
}

func (p *Parser) absorbWhitespace() {
	for ;  p.peek().Type == token.Newline; p.next() {
	}
}

func (p *Parser) ParseFile() (*ast.File) {
	pos_valid, _ := p.expect(token.Token{token.PACKAGE, 0, "package"})
	if !pos_valid {
		p.error("expected package keyword")
		return nil
	}
	ident := p.parseIdent()
	if ident.Text == "_" {
		p.error("invalid package name _")
	}
	var decls []ast.FuncDecl
	for p.peek().Type != token.EOF {
		decls = append(decls, *p.parseFuncDecl())
	}

	return &ast.File{
		Name:       ident.Text,
		Decls:      decls,
	}
}


func (p *Parser) parseIdent() *token.Token {
	name := "_"
	if p.peek().Type == token.Identifier {
		name = p.peek().Text
		p.next()	} else {
		p.expectTok(token.Identifier) // use expect() error handling
	}
	return &token.Token{token.Identifier, 0, name}
}


func (p *Parser) parseFuncDecl() *ast.FuncDecl {
	p.absorbWhitespace()
	if func_valid, _ := p.expect(token.Token{token.FUNC, 0, "func"}); !func_valid {
		p.error(fmt.Sprintf("expected func keyword, got %v", p.peek()))
	}
	var decl ast.FuncDecl
	exprs, ok := p.Line()
	decl.Epxrs = exprs
	decl.Name = "_"
	if !ok {
		p.error("Error in p.Line()")
	}
	return &decl
}

// Line reads a line of input and returns the values it evaluates.
// A nil returned slice means there were no values.
// The boolean reports whether the line is valid.
func (p *Parser) Line() ([]value.Expr, bool) {
	tok := p.peek()
	switch tok.Type {
	case token.RightParen:
		// TODO
		return nil, true
	case token.Op:
		// TODO
		return nil, true
	}
	// TODO

	exprs, ok := p.expressionList()
	if !ok {
		return nil, false
	}
	return exprs, true
}

// expressionList:
//'\n'
//statementList '\n'
func (p *Parser) expressionList() ([]value.Expr, bool) {
	tok := p.next()
	switch tok.Type {
	case token.EOF:
		return nil, false
	case token.Newline:
		return nil, true
	}
	exprs, ok := p.statementList(tok)
	if !ok {
		return nil, false
	}
	tok = p.next()
	switch tok.Type {
	case token.EOF, token.Newline:
	default:
		p.errorf("unexpected %s", tok)
	}
	if len(exprs) > 0 && p.context.Config().Debug("parse") {
		p.Println(Tree(exprs))
	}
	return exprs, ok
}

// statementList:
//expr
//expr ';' expr
func (p *Parser) statementList(tok token.Token) ([]value.Expr, bool) {
	expr := p.expr(tok)
	var exprs []value.Expr
	if expr != nil {
		exprs = []value.Expr{expr}
	}
	if p.peek().Type == token.Semicolon {
		p.next()
		more, ok := p.statementList(p.next())
		if ok {
			exprs = append(exprs, more...)
		}
	}
	return exprs, true
}

// expr
//operand
//operand binop expr
func (p *Parser) expr(tok token.Token) value.Expr {
	if p.peek().Type == token.Assign && tok.Type != token.Identifier {
		p.errorf("cannot assign to %s", tok)
	}
	expr := p.operand(tok, true)
	tok = p.peek()
	switch tok.Type {
	case token.Newline, token.EOF, token.RightParen, token.RightBrack, token.Semicolon:
		return expr
	case token.Identifier:
		// TODO
		return nil
	case token.Operator, token.Assign:
		p.next()
		return &binary{
			left:  expr,
			op:    tok.Text,
			right: p.expr(p.next()),
		}
	}
	p.errorf("after expression: unexpected %s", p.peek())
	return nil
}

// operand
//number
//char constant
//string constant
//vector
//operand [ Expr ]...
//unop Expr
func (p *Parser) operand(tok token.Token, indexOK bool) value.Expr {
	var expr value.Expr
	switch tok.Type {
	case token.Identifier:
		// TODO
		fallthrough
	case token.Number, token.Rational, token.String, token.LeftParen:
		expr = p.numberOrVector(tok)
	default:
		p.errorf("unexpected %s", tok)
	}
	if indexOK {
		expr = p.index(expr)
	}
	return expr
}

// index
//expr
//expr [ expr ]
//expr [ expr ] [ expr ] ....
func (p *Parser) index(expr value.Expr) value.Expr {
	for p.peek().Type == token.LeftBrack {
		p.next()
		index := p.expr(p.next())
		tok := p.next()
		if tok.Type != token.RightBrack {
			p.errorf("expected right bracket, found %s", tok)
		}
		expr = &binary{
			op:    "[]",
			left:  expr,
			right: index,
		}
	}
	return expr
}

// number
//integer
//rational
//string
//variable
//'(' Expr ')'
// If the value is a string, value.Expr is nil.
func (p *Parser) number(tok token.Token) (expr value.Expr, str string) {
	var err error
	text := tok.Text
	switch tok.Type {
	case token.Identifier:
		expr = p.variable(text)
	case token.String:
		// TODO
		str = "<token.String>"
	case token.Number, token.Rational:
		expr, err = value.Parse(p.context.Config(), text)
	case token.LeftParen:
		expr = p.expr(p.next())
		tok := p.next()
		if tok.Type != token.RightParen {
			p.errorf("expected right paren, found %s", tok)
		}
	}
	if err != nil {
		p.errorf("%s: %s", text, err)
	}
	return expr, str
}

// numberOrVector turns the token and what follows into a numeric Value, possibly a vector.
// numberOrVector
//number
//string
//numberOrVector...
func (p *Parser) numberOrVector(tok token.Token) value.Expr {
	expr, str := p.number(tok)
	switch p.peek().Type {
	case token.Number, token.Rational, token.String, token.Identifier, token.LeftParen:
		// TODO:
		// Further vector elements follow.
		return nil
	}
	var slice sliceExpr
	if expr == nil {
		// Must be a string.
		slice = append(slice, evalString(str)...)
	} else {
		slice = sliceExpr{expr}
	}
	if len(slice) == 1 {
		return slice[0] // Just a singleton.
	}
	return slice
}

func (p *Parser) variable(name string) variableExpr {
	return variableExpr{
		name: name,
	}
}

// evalString turns a parsed string constant into a slice of
// value.Exprs each of which is a value.Char.
func evalString(str string) []value.Expr {
	r := ([]rune)(str)
	v := make([]value.Expr, len(r))
	for i, c := range r {
		v[i] = value.Char(c)
	}
	return v
}
