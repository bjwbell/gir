package codegen

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"github.com/bjwbell/cmd/src"
	"github.com/bjwbell/gir/gst"
	"github.com/bjwbell/ssa"
)

type Block struct {
	b     *ssa.Block
	label *ast.LabeledStmt
	stmts []ast.Stmt
}

func (b *Block) Name() string {
	if b.label == nil {
		return "_"
	}
	if b.label.Label == nil {
		panic("block label ident is nil")
	}
	name := b.label.Label.Name
	return name
}

type state struct {
	// configuration (arch) information
	config *ssa.Config
	// context includes *token.File and *types.File
	ctx Ctx

	// function we're building
	f      *ssa.Func
	fnInfo *types.Info
	fnType *types.Func
	fnDecl *ast.FuncDecl
	// labels and labeled control flow nodes in f
	labels       map[string]*ssaLabel
	labeledNodes map[ast.Node]*ssaLabel

	// gotos that jump forward; required for deferred checkGoto calls
	fwdGotos []*Node
	// Code that must precede any return
	// (e.g., copying value of heap-escaped paramout back to true paramout)
	//exitCode *NodeList

	// unlabeled break and continue statement tracking
	breakTo    *ssa.Block // current target for plain break statement
	continueTo *ssa.Block // current target for plain continue statement

	// current location where we're interpreting the AST
	curBlock *ssa.Block

	// variable assignments in the current block (map from variable symbol to ssa value)
	// *Node is the unique identifier (an ONAME Node) for the variable.
	vars map[ssaVar]*ssa.Value

	// all defined variables at the end of each block.  Indexed by block ID.
	defvars []map[ssaVar]*ssa.Value

	// addresses of PPARAM and PPARAMOUT variables.
	decladdrs map[ssaVar]*ssa.Value

	// symbols for PEXTERN, PAUTO and PPARAMOUT variables so they can be reused.
	varsyms map[ssaVar]interface{}

	// starting values.  Memory, stack pointer, and globals pointer
	startmem *ssa.Value
	sp       *ssa.Value
	sb       *ssa.Value

	// line number stack.  The current line number is top of stack
	line []int32

	blocks []*Block
	//unlabeledBlocks []*ssa.Block
	//labledBlocks    map[string]*ssa.Block
}

func (s *state) retVar() *ssaRetVar {
	retVars := getReturnVar(s.ctx, s.fnType)
	if len(retVars) > 1 {
		panic("more than one return value is unsupported")
	}
	if len(retVars) == 0 {
		return nil
	}
	ret := retVars[0]
	if ret == nil {
		panic("nil ret var")
	}
	return ret
}

func (s *state) retVarAddr() *ssa.Value {
	ret := s.retVar()
	retSym := &ssa.ArgSymbol{Typ: ret.Typ(), Node: ret}
	aux := retSym
	retVarAddr := s.entryNewValue1A(ssa.OpAddr, ret.Typ().PtrTo(), aux, s.sp)
	return retVarAddr
}

func (s *state) label(ident *ast.Ident) *ssaLabel {
	lab := s.labels[ident.Name]
	if lab == nil {
		lab = new(ssaLabel)
		lab.name = ident.Name
		s.labels[ident.Name] = lab
	}
	return lab
}

func (s *state) Logf(msg string, args ...interface{}) { s.config.Logf(msg, args...) }

func (s *state) Fatalf(msg string, args ...interface{}) { s.config.Fatalf(src.XPos{}, msg, args...) }
func (s *state) Unimplementedf(msg string, args ...interface{}) {
	// TODO: comment/remove when no longer needed for debugging
	fmt.Printf("s.UNIMPLEMENTED msg: %v\n", fmt.Sprintf(msg, args))

	s.config.Fatalf(src.XPos{}, msg, args...)
}

// func (s *state) Warnl(line int32, msg string, args ...interface{}) { s.config.Warnl(line, msg, args...) }
func (s *state) Debug_checknil() bool { return s.config.Debug_checknil() }

var (
	// dummy node for the memory variable
	memVar = ssaParam{}

// dummy nodes for temporary variables
/*ptrVar   = Node{Op: ONAME, Class: Pxxx, Sym: &Sym{Name: "ptr"}}
capVar   = Node{Op: ONAME, Class: Pxxx, Sym: &Sym{Name: "cap"}}
typVar   = Node{Op: ONAME, Class: Pxxx, Sym: &Sym{Name: "typ"}}
idataVar = Node{Op: ONAME, Class: Pxxx, Sym: &Sym{Name: "idata"}}
okVar    = Node{Op: ONAME, Class: Pxxx, Sym: &Sym{Name: "ok"}}*/
)

// startBlock sets the current block we're generating code in to b.
func (s *state) startBlock(b *ssa.Block) {
	if s.curBlock != nil {
		s.Fatalf("starting block %v when block %v has not ended", b, s.curBlock)
	}
	s.curBlock = b
	//s.vars = map[*Node]*ssa.Value{}
}

// endBlock marks the end of generating code for the current block.
// Returns the (former) current block.  Returns nil if there is no current
// block, i.e. if no code flows to the current execution point.
func (s *state) endBlock() *ssa.Block {
	b := s.curBlock
	if b == nil {
		return nil
	}
	for len(s.defvars) <= int(b.ID) {
		s.defvars = append(s.defvars, nil)
	}
	s.defvars[b.ID] = s.vars
	s.curBlock = nil
	s.vars = nil
	b.Pos = s.peekLine()
	return b
}

// pushLine pushes a line number on the line number stack.
func (s *state) pushLine(line int32) {
	s.line = append(s.line, line)
}

// peekLine peek the top of the line number stack.
func (s *state) peekLine() src.XPos {
	return src.XPos{}
}

func (s *state) Errorf(msg string, args ...interface{}) {
	panic(msg)
}

// newValue0 adds a new value with no arguments to the current block.
func (s *state) newValue0(op ssa.Op, t ssa.Type) *ssa.Value {
	return s.curBlock.NewValue0(s.peekLine(), op, t)
}

// newValue0A adds a new value with no arguments and an aux value to the current block.
func (s *state) newValue0A(op ssa.Op, t ssa.Type, aux interface{}) *ssa.Value {
	return s.curBlock.NewValue0A(s.peekLine(), op, t, aux)
}

// newValue0I adds a new value with no arguments and an auxint value to the current block.
func (s *state) newValue0I(op ssa.Op, t ssa.Type, auxint int64) *ssa.Value {
	return s.curBlock.NewValue0I(s.peekLine(), op, t, auxint)
}

// newValue1 adds a new value with one argument to the current block.
func (s *state) newValue1(op ssa.Op, t ssa.Type, arg *ssa.Value) *ssa.Value {
	return s.curBlock.NewValue1(s.peekLine(), op, t, arg)
}

// newValue1A adds a new value with one argument and an aux value to the current block.
func (s *state) newValue1A(op ssa.Op, t ssa.Type, aux interface{}, arg *ssa.Value) *ssa.Value {
	return s.curBlock.NewValue1A(s.peekLine(), op, t, aux, arg)
}

// newValue1I adds a new value with one argument and an auxint value to the current block.
func (s *state) newValue1I(op ssa.Op, t ssa.Type, aux int64, arg *ssa.Value) *ssa.Value {
	return s.curBlock.NewValue1I(s.peekLine(), op, t, aux, arg)
}

// newValue2 adds a new value with two arguments to the current block.
func (s *state) newValue2(op ssa.Op, t ssa.Type, arg0, arg1 *ssa.Value) *ssa.Value {
	return s.curBlock.NewValue2(s.peekLine(), op, t, arg0, arg1)
}

// newValue2I adds a new value with two arguments and an auxint value to the current block.
func (s *state) newValue2I(op ssa.Op, t ssa.Type, aux int64, arg0, arg1 *ssa.Value) *ssa.Value {
	return s.curBlock.NewValue2I(s.peekLine(), op, t, aux, arg0, arg1)
}

// newValue3 adds a new value with three arguments to the current block.
func (s *state) newValue3(op ssa.Op, t ssa.Type, arg0, arg1, arg2 *ssa.Value) *ssa.Value {
	return s.curBlock.NewValue3(s.peekLine(), op, t, arg0, arg1, arg2)
}

// newValue3I adds a new value with three arguments and an auxint value to the current block.
func (s *state) newValue3I(op ssa.Op, t ssa.Type, aux int64, arg0, arg1, arg2 *ssa.Value) *ssa.Value {
	return s.curBlock.NewValue3I(s.peekLine(), op, t, aux, arg0, arg1, arg2)
}

// entryNewValue0 adds a new value with no arguments to the entry block.
func (s *state) entryNewValue0(op ssa.Op, t ssa.Type) *ssa.Value {
	return s.f.Entry.NewValue0(s.peekLine(), op, t)
}

// entryNewValue0A adds a new value with no arguments and an aux value to the entry block.
func (s *state) entryNewValue0A(op ssa.Op, t ssa.Type, aux interface{}) *ssa.Value {
	return s.f.Entry.NewValue0A(s.peekLine(), op, t, aux)
}

// entryNewValue0I adds a new value with no arguments and an auxint value to the entry block.
func (s *state) entryNewValue0I(op ssa.Op, t ssa.Type, auxint int64) *ssa.Value {
	return s.f.Entry.NewValue0I(s.peekLine(), op, t, auxint)
}

// entryNewValue1 adds a new value with one argument to the entry block.
func (s *state) entryNewValue1(op ssa.Op, t ssa.Type, arg *ssa.Value) *ssa.Value {
	return s.f.Entry.NewValue1(s.peekLine(), op, t, arg)
}

// entryNewValue1 adds a new value with one argument and an auxint value to the entry block.
func (s *state) entryNewValue1I(op ssa.Op, t ssa.Type, auxint int64, arg *ssa.Value) *ssa.Value {
	return s.f.Entry.NewValue1I(s.peekLine(), op, t, auxint, arg)
}

// entryNewValue1A adds a new value with one argument and an aux value to the entry block.
func (s *state) entryNewValue1A(op ssa.Op, t ssa.Type, aux interface{}, arg *ssa.Value) *ssa.Value {
	return s.f.Entry.NewValue1A(s.peekLine(), op, t, aux, arg)
}

// entryNewValue2 adds a new value with two arguments to the entry block.
func (s *state) entryNewValue2(op ssa.Op, t ssa.Type, arg0, arg1 *ssa.Value) *ssa.Value {
	return s.f.Entry.NewValue2(s.peekLine(), op, t, arg0, arg1)
}

// PXOR adds a new PXOR value to the entry block.
func (s *state) PXOR(b *ssa.Block) *ssa.Value {
	t := Typ[types.Float32]
	var cnst43 float32
	cnst43 = 42.0

	val0 := s.f.ConstFloat32(s.peekLine(), t, float64(cnst43))
	val1 := s.f.ConstFloat32(s.peekLine(), t, float64(cnst43))
	return b.NewValue2(s.peekLine(), ssa.OpAMD64PXOR, t, val0, val1)
}

// const* routines add a new const value to the entry block.
func (s *state) constBool(c bool) *ssa.Value {
	return s.f.ConstBool(s.peekLine(), Typ[types.Bool], c)
}
func (s *state) constInt8(t ssa.Type, c int8) *ssa.Value {
	return s.f.ConstInt8(s.peekLine(), t, c)
}
func (s *state) constInt16(t ssa.Type, c int16) *ssa.Value {
	return s.f.ConstInt16(s.peekLine(), t, c)
}
func (s *state) constInt32(t ssa.Type, c int32) *ssa.Value {
	return s.f.ConstInt32(s.peekLine(), t, c)
}
func (s *state) constInt64(t ssa.Type, c int64) *ssa.Value {
	return s.f.ConstInt64(s.peekLine(), t, c)
}
func (s *state) constFloat32(t ssa.Type, c float64) *ssa.Value {
	return s.f.ConstFloat32(s.peekLine(), t, c)
}
func (s *state) constFloat64(t ssa.Type, c float64) *ssa.Value {
	return s.f.ConstFloat64(s.peekLine(), t, c)
}
func (s *state) constInt(t ssa.Type, c int64) *ssa.Value {
	if s.config.IntSize == 8 {
		return s.constInt64(t, c)
	}
	if int64(int32(c)) != c {
		s.Fatalf("integer constant too big %d", c)
	}
	return s.constInt32(t, int32(c))
}

func (s *state) labeledEntryBlock(block *ast.BlockStmt) bool {
	return s.blockLabel(block) != nil
}

func (s *state) blockLabel(block *ast.BlockStmt) *ast.LabeledStmt {
	// the first stmt may be a label for the entry block
	if len(block.List) >= 1 {
		if labeledStmt, ok := block.List[0].(*ast.LabeledStmt); ok {
			return labeledStmt
		}

	}
	return nil
}

func (s *state) scanBlocksGst(fnBody gst.Stmt) {
	var block *Block
	b := s.f.NewBlock(ssa.BlockPlain)
	var labelStmt *ast.LabeledStmt
	labelStmt = &ast.LabeledStmt{}
	var labelIdent ast.Ident
	labelStmt.Label = &labelIdent
	labelStmt.Stmt = &ast.ReturnStmt{}
	block = &Block{b: b, label: labelStmt}
	block.stmts = []ast.Stmt{labelStmt}
	s.blocks = append(s.blocks, block)

	s.checkBlocks()
}

func (s *state) scanBlocks(fnBody *ast.BlockStmt) {
	stmtList := fnBody.List
	entryBlock := true
	var block *Block
	for _, stmt := range stmtList {
		var labelStmt *ast.LabeledStmt
		var isLabel bool
		labelStmt, isLabel = stmt.(*ast.LabeledStmt)
		if isLabel || block == nil {
			if block == nil {
				if !entryBlock {
					panic("internal error")
				}
				labelStmt = nil
			}
			b := s.f.NewBlock(ssa.BlockPlain)
			block = &Block{b: b, label: labelStmt}
			s.blocks = append(s.blocks, block)
		}
		block.stmts = append(block.stmts, stmt)
	}
	s.checkBlocks()
}

func (s *state) checkBlocks() {
	for _, block := range s.blocks {
		s.checkBlock(block)
	}
}

func (s *state) checkBlock(block *Block) {
	entryBlock := s.isEntryBlock(block)
	if len(block.stmts) < 1 {
		s.Errorf("ERROR: block must have at least one statement")
	}
	if lbl, ok := block.stmts[0].(*ast.LabeledStmt); !ok {
		if !entryBlock {
			s.Errorf("ERROR: block first statment must be label")
		}
	} else {
		if lbl.Label.Name != block.Name() {
			panic("label name doesn't match block name")
		}
	}
	lastStmt := block.stmts[len(block.stmts)-1]
	s.checkLastStmt(block, lastStmt)
}

func (s *state) checkLastStmt(block *Block, stmt ast.Stmt) {
	if branch, ok := stmt.(*ast.BranchStmt); ok {
		if branch.Tok != token.GOTO {
			s.Errorf("ERROR: only goto allowed in branch stmt not break, continue, or fallthrough")
		}
	} else if lbledStmt, ok := stmt.(*ast.LabeledStmt); ok {
		if len(block.stmts) > 1 {
			s.Errorf("Block can't have multiple labels")
		}
		s.checkLastStmt(block, lbledStmt.Stmt)
	} else if ifStmt, ok := stmt.(*ast.IfStmt); ok {
		_, _, _, err := s.matchIfStmt(ifStmt)
		if err != nil {
			s.Errorf(fmt.Sprintf("%v", err))
		}
	} else if _, ok := stmt.(*ast.ReturnStmt); ok {
		//
	} else {
		// the entry block doesn't have to explicitly transfer control
		if !s.isEntryBlock(block) {
			s.Errorf("Last stmt must a transfer control")
		}
	}
}

func (s *state) isEntryBlock(block *Block) bool {
	return block == s.blocks[0]
}

func (s *state) processBlocks() {
	for _, block := range s.blocks {
		s.processBlock(block)
	}
}

func (s *state) processBlock(block *Block) {
	for _, stmt := range block.stmts {
		s.stmt(block, stmt)
	}
}

// body converts the body of fn to SSA and adds it to s.
func (s *state) body(block *ast.BlockStmt) {
	if !s.labeledEntryBlock(block) {
		panic("entry block must be labeled (even if with \"_\")")
	}
	s.stmtList(block.List, true)
}

// ssaStmtList converts the statement n to SSA and adds it to s.
func (s *state) stmtList(stmtList []ast.Stmt, firstBlock bool) {
	// firstStmt := firstBlock
	// for _, stmt := range stmtList {
	// 	//s.stmt(stmt, firstStmt)
	// 	//firstStmt = false
	// }
}

func NewNode(n ast.Node, ctx Ctx) *Node {
	return &Node{node: n, ctx: ctx}
}

func ExprNode(n ast.Expr, ctx Ctx) *Node {
	return &Node{node: n, ctx: ctx}
}

func isBlankIdent(ident *ast.Ident) bool {
	return ident != nil && ident.Name == "_"
}

func blockIdx(blocks []*Block, block *Block) int {
	for i, b := range blocks {
		if b == block {
			return i
		}
	}
	return -1
}

func (s *state) nextBlock(block *Block) *Block {
	i := blockIdx(s.blocks, block)
	if i == -1 || len(s.blocks) <= i+1 {
		return nil
	}
	return s.blocks[i+1]
}

func (s *state) GetBlock(ssaBlock *ssa.Block) *Block {
	for _, block := range s.blocks {
		if block.b == ssaBlock {
			return block
		}
	}
	return nil
}

func (s *state) getBlockFromName(name string) *Block {
	for _, block := range s.blocks {
		if block.Name() == name {
			return block
		}
	}
	return nil
}

func (s *state) matchIfStmt(stmt *ast.IfStmt) (condIdent *ast.Ident, yesLabel string, noLabel string, err error) {
	var errored bool
	var ok bool
	if stmt.Init != nil {
		s.Errorf("Error: if statement cannot have init expr")
	}
	errMsg := "Error: if statement must be of the form \"if t1 { goto lbl1 } else { goto lbl2 }\""
	if len(stmt.Body.List) != 1 {
		return nil, "", "", fmt.Errorf(errMsg)
	}

	bodyStmt, ok := stmt.Body.List[0].(*ast.BranchStmt)
	errored = errored || !ok

	if stmt.Else == nil {
		errored = true
	}

	elseBody, ok := stmt.Else.(*ast.BlockStmt)
	errored = errored || !ok

	elseStmt, ok := elseBody.List[0].(*ast.BranchStmt)
	errored = errored || !ok

	condIdent, ok = stmt.Cond.(*ast.Ident)
	errored = errored || !ok

	if errored {
		return nil, "", "", fmt.Errorf(errMsg)
	}

	yesLabel = bodyStmt.Label.Name
	noLabel = elseStmt.Label.Name
	fmt.Println("if condIdent:", condIdent)
	fmt.Println("if bdyStmt:", bodyStmt)
	fmt.Println("if elseStmt:", elseStmt)
	return condIdent, yesLabel, noLabel, nil
}

// stmt converts the statement stmt to SSA and adds it to s.
func (s *state) stmt(block *Block, stmt ast.Stmt) {
	// node := stmt.(ast.Node)
	// n := &Node{node: node, ctx: s.ctx}
	// s.pushLine(n.Lineno())
	// defer s.popLine()

	// TODO

	switch stmt := stmt.(type) {
	case *ast.LabeledStmt:
		lblIdent := stmt.Label
		//if isBlankIdent(lblIdent) {
		//  return
		//}

		lab := s.label(lblIdent)

		if !lab.defined() {
			lab.defNode = NewNode(stmt, s.ctx)
		} else {
			s.Errorf("label %v already defined at %v", lblIdent.Name, "<line#>")
			lab.reported = true
		}

		if lblIdent.Name != block.Name() {
			panic("block label name doesn't match block name")
		}

		// The label might already have a target block via a goto.
		if lab.target == nil {
			lab.target = block.b
		}

		s.stmt(block, stmt.Stmt)

		// go to that label (we pretend "label:" is preceded by "goto label")
		// b := s.endBlock()
		// b.AddEdgeTo(lab.target)
		// s.startBlock(lab.target)
	case *ast.AssignStmt:
		s.assignStmt(stmt)
	case *ast.BadStmt:
		panic("error BadStmt")
	case *ast.BlockStmt:
		// TODO: handle correctly
		s.stmtList(stmt.List, false)
	case *ast.BranchStmt:
		n := NewNode(stmt, s.ctx)
		switch stmt.Tok {
		case token.GOTO:
		default:
			s.Errorf("Error: only goto branch statements supported (not break, continue, or fallthrough ")
		}

		lab := s.label(stmt.Label)
		if lab.target == nil {
			lab.target = s.getBlockFromName(lab.name).b
			if lab.target == nil {
				panic("nil label target block")
			}
		}
		if !lab.used() {
			lab.useNode = n
		}

		if lab.defined() {
			s.checkGoto(n, lab.defNode)
		} else {
			s.fwdGotos = append(s.fwdGotos, n)
		}

		block.b.AddEdgeTo(lab.target)
	case *ast.DeclStmt:
		decl, ok := stmt.Decl.(*ast.GenDecl)
		if !ok {
			panic("expected *ast.GenDecl")

		}
		switch decl.Tok {
		case token.IMPORT:
			panic("internal error")
		case token.TYPE:
			panic("internal error")
		case token.CONST:
			// panic("unimplementedf")
		case token.VAR:
			// panic("unimplementedf")
		default:
			panic("internal error")
		}
		//panic(fmt.Sprintf("todo ast.DeclStmt: %#v", stmt))
	case *ast.EmptyStmt: // No op
	case *ast.ExprStmt:
		expr := stmt.X
		switch expr := expr.(type) {
		case *ast.CallExpr:
			callexpr := expr
			fn, ok := callexpr.Fun.(*ast.SelectorExpr)
			if ok {
				fnPkg := fmt.Sprintf("%v", fn.X)
				fnName := fmt.Sprintf("%v", fn.Sel)
				if fnPkg == "ssair" && fnName == "Op2" {
					s.PXOR(block.b)
				} else {
					panic("call expr not implemented: fnPkg - " + fnPkg + ", fnName - " + fnName + ", " + fmt.Sprintf("%#v", expr))
				}
			} else {
				panic("call expr not implemented")
			}
		default:
			panic("default expr not implemented")
		}
	case *ast.IfStmt:
		condIdent, yes, no, err := s.matchIfStmt(stmt)
		if err != nil {
			break
		}
		c := s.expr(&Node{node: condIdent, ctx: s.ctx})
		block.b.Kind = ssa.BlockIf
		block.b.Control = c
		block.b.Likely = ssa.BranchUnknown
		yesBlock := s.getBlockFromName(yes)
		noBlock := s.getBlockFromName(no)
		block.b.AddEdgeTo(yesBlock.b)
		block.b.AddEdgeTo(noBlock.b)
	case *ast.IncDecStmt:
		panic("todo ast.IncDecStmt")
	case *ast.ReturnStmt:
		if len(stmt.Results) > 1 {
			panic("unsupported: multiple return values")
		}
		if len(stmt.Results) == 1 {
			res := stmt.Results[0]
			node := NewNode(res, s.ctx)
			t := node.Typ()
			v := s.expr(node)
			addr := s.retVarAddr()
			s.vars[&memVar] = s.newValue3I(ssa.OpStore, ssa.TypeMem, t.Size(), addr, v, s.mem())
		}
		m := s.mem()
		block.b.Kind = ssa.BlockRet
		block.b.Control = m

	case *ast.ForStmt:
		panic("unsupported: ForStmt")
	case *ast.GoStmt:
		panic("unsupported: GoStmt")
	case *ast.RangeStmt:
		panic("unsupported: RangeStmt")
	case *ast.DeferStmt:
		panic("unsupported: DeferStmt")
	case *ast.SelectStmt:
		panic("unsupported: SelectStmt")
	case *ast.SendStmt:
		panic("unsupported: SendStmt")
	case *ast.SwitchStmt:
		panic("unsupported: SwitchStmt")
	case *ast.TypeSwitchStmt:
		panic("unsupported: TypeSwitchStmt")
	default:
		fmt.Println("stmt: ", stmt)
		panic("unknown ast.Stmt")
	}
}

// variable returns the value of a variable at the current location.
func (s *state) variable(name ssaVar, t ssa.Type) *ssa.Value {
	v := s.vars[name]
	if v == nil {
		// TODO: get type?  Take Sym as arg?
		v = s.newValue0A(ssa.OpFwdRef, t, name)
		s.vars[name] = v
	}
	return v
}

func (s *state) mem() *ssa.Value {
	return s.variable(&memVar, ssa.TypeMem)
}

func (s *state) linkForwardReferences() {
	// Build ssa graph.  Each variable on its first use in a basic block
	// leaves a FwdRef in that block representing the incoming value
	// of that variable.  This function links that ref up with possible definitions,
	// inserting Phi values as needed.  This is essentially the algorithm
	// described by Brau, Buchwald, Hack, Leißa, Mallon, and Zwinkau:
	// http://pp.info.uni-karlsruhe.de/uploads/publikationen/braun13cc.pdf
	for _, b := range s.f.Blocks {
		for _, v := range b.Values {
			if v.Op != ssa.OpFwdRef {
				continue
			}
			name := v.Aux.(ssaVar)
			v.Op = ssa.OpCopy
			v.Aux = nil
			v.SetArgs1(s.lookupVarIncoming(b, v.Type, name))
		}
	}
}

// lookupVarIncoming finds the variable's value at the start of block b.
func (s *state) lookupVarIncoming(b *ssa.Block, t ssa.Type, name ssaVar) *ssa.Value {
	// TODO(khr): have lookupVarIncoming overwrite the fwdRef or copy it
	// will be used in, instead of having the result used in a copy value.
	if b == s.f.Entry {
		if name == &memVar {
			return s.startmem
		}
		//return nil

		if canSSA(name) {
			v := s.entryNewValue0A(ssa.OpArg, t, name)
			// v starts with AuxInt == 0.
			//s.addNamedValue(name, v)
			return v
		}
		// // variable is live at the entry block.  Load it.
		// addr := s.decladdrs[name]
		// if addr == nil {
		// 	// TODO: closure args reach here.
		// 	s.Unimplementedf("unhandled closure arg %s at entry to function %s", name, b.Func.Name)
		// }
		// if _, ok := addr.Aux.(*ssa.ArgSymbol); !ok {
		// 	s.Fatalf("variable live at start of function %s is not an argument %s", b.Func.Name, name)
		// }
		// return s.entryNewValue2(ssa.OpLoad, t, addr, s.startmem)
	}
	var vals []*ssa.Value
	for _, p := range b.Preds {
		vals = append(vals, s.lookupVarOutgoing(p.Block(), t, name))
	}
	if len(vals) == 0 {
		// This block is dead; we have no predecessors and we're not the entry block.
		// It doesn't matter what we use here as long as it is well-formed,
		// so use the default/zero value.
		return nil
		// if name == &memVar {
		// 	return s.startmem
		// }
		// return s.zeroVal(name.Type)
	}
	v0 := vals[0]
	for i := 1; i < len(vals); i++ {
		if vals[i] != v0 {
			// need a phi value
			v := b.NewValue0(s.peekLine(), ssa.OpPhi, t)
			v.AddArgs(vals...)
			//s.addNamedValue(name, v)
			return v
		}
	}
	return v0
}

// lookupVarOutgoing finds the variable's value at the end of block b.
func (s *state) lookupVarOutgoing(b *ssa.Block, t ssa.Type, name ssaVar) *ssa.Value {
	return nil
	// m := s.defvars[b.ID]
	// if v, ok := m[name]; ok {
	// 	return v
	// }
	// // The variable is not defined by b and we haven't
	// // looked it up yet.  Generate v, a copy value which
	// // will be the outgoing value of the variable.  Then
	// // look up w, the incoming value of the variable.
	// // Make v = copy(w).  We need the extra copy to
	// // prevent infinite recursion when looking up the
	// // incoming value of the variable.
	// v := b.NewValue0(s.peekLine(), ssa.OpCopy, t)
	// m[name] = v
	// v.AddArg(s.lookupVarIncoming(b, t, name))
	// return v
}

// TODO: the above mutually recursive functions can lead to very deep stacks.  Fix that.

func (s *state) addNamedValue(n *Node, v *ssa.Value) {
	if n.class == Pxxx {
		// Don't track our dummy nodes (&memVar etc.).
		return
	}
	if v == nil {
		panic("nil *ssa.Value")
	}
	if v.Type == nil {
		panic("nil v.Type (*ssa.Value)")
	}
	if n.class == PAUTO && (v.Type.IsString() || v.Type.IsSlice() || v.Type.IsInterface()) {
		// TODO: can't handle auto compound objects with pointers yet.
		return
	}
	// if n.Class == PAUTO && n.Xoffset != 0 {
	// 	s.Fatalf("AUTO var with offset %s %d", n, n.Xoffset)
	// }

	loc := ssa.LocalSlot{N: n, Type: n.Typ(), Off: 0}
	values, ok := s.f.NamedValues[loc]
	if !ok {
		s.f.Names = append(s.f.Names, loc)
	}
	s.f.NamedValues[loc] = append(values, v)
}

type opAndType struct {
	op  NodeOp
	typ types.BasicKind
}

var opToSSA = map[opAndType]ssa.Op{
	opAndType{OADD, types.Int8}:   ssa.OpAdd8,
	opAndType{OADD, types.Uint8}:  ssa.OpAdd8,
	opAndType{OADD, types.Int16}:  ssa.OpAdd16,
	opAndType{OADD, types.Uint16}: ssa.OpAdd16,
	opAndType{OADD, types.Int32}:  ssa.OpAdd32,
	opAndType{OADD, types.Uint32}: ssa.OpAdd32,
	//opAndType{OADD, types.Ptr32}:   ssa.OpAdd32,
	opAndType{OADD, types.Int64}:  ssa.OpAdd64,
	opAndType{OADD, types.Uint64}: ssa.OpAdd64,
	//opAndType{OADD, types.Ptr64}:   ssa.OpAdd64,
	opAndType{OADD, types.Float32}: ssa.OpAdd32F,
	opAndType{OADD, types.Float64}: ssa.OpAdd64F,

	opAndType{OSUB, types.Int8}:    ssa.OpSub8,
	opAndType{OSUB, types.Uint8}:   ssa.OpSub8,
	opAndType{OSUB, types.Int16}:   ssa.OpSub16,
	opAndType{OSUB, types.Uint16}:  ssa.OpSub16,
	opAndType{OSUB, types.Int32}:   ssa.OpSub32,
	opAndType{OSUB, types.Uint32}:  ssa.OpSub32,
	opAndType{OSUB, types.Int64}:   ssa.OpSub64,
	opAndType{OSUB, types.Uint64}:  ssa.OpSub64,
	opAndType{OSUB, types.Float32}: ssa.OpSub32F,
	opAndType{OSUB, types.Float64}: ssa.OpSub64F,

	opAndType{ONOT, types.Bool}: ssa.OpNot,

	opAndType{OMINUS, types.Int8}:    ssa.OpNeg8,
	opAndType{OMINUS, types.Uint8}:   ssa.OpNeg8,
	opAndType{OMINUS, types.Int16}:   ssa.OpNeg16,
	opAndType{OMINUS, types.Uint16}:  ssa.OpNeg16,
	opAndType{OMINUS, types.Int32}:   ssa.OpNeg32,
	opAndType{OMINUS, types.Uint32}:  ssa.OpNeg32,
	opAndType{OMINUS, types.Int64}:   ssa.OpNeg64,
	opAndType{OMINUS, types.Uint64}:  ssa.OpNeg64,
	opAndType{OMINUS, types.Float32}: ssa.OpNeg32F,
	opAndType{OMINUS, types.Float64}: ssa.OpNeg64F,

	opAndType{OCOM, types.Int8}:   ssa.OpCom8,
	opAndType{OCOM, types.Uint8}:  ssa.OpCom8,
	opAndType{OCOM, types.Int16}:  ssa.OpCom16,
	opAndType{OCOM, types.Uint16}: ssa.OpCom16,
	opAndType{OCOM, types.Int32}:  ssa.OpCom32,
	opAndType{OCOM, types.Uint32}: ssa.OpCom32,
	opAndType{OCOM, types.Int64}:  ssa.OpCom64,
	opAndType{OCOM, types.Uint64}: ssa.OpCom64,

	opAndType{OIMAG, types.Complex64}:  ssa.OpComplexImag,
	opAndType{OIMAG, types.Complex128}: ssa.OpComplexImag,
	opAndType{OREAL, types.Complex64}:  ssa.OpComplexReal,
	opAndType{OREAL, types.Complex128}: ssa.OpComplexReal,

	opAndType{OMUL, types.Int8}:    ssa.OpMul8,
	opAndType{OMUL, types.Uint8}:   ssa.OpMul8,
	opAndType{OMUL, types.Int16}:   ssa.OpMul16,
	opAndType{OMUL, types.Uint16}:  ssa.OpMul16,
	opAndType{OMUL, types.Int32}:   ssa.OpMul32,
	opAndType{OMUL, types.Uint32}:  ssa.OpMul32,
	opAndType{OMUL, types.Int64}:   ssa.OpMul64,
	opAndType{OMUL, types.Uint64}:  ssa.OpMul64,
	opAndType{OMUL, types.Float32}: ssa.OpMul32F,
	opAndType{OMUL, types.Float64}: ssa.OpMul64F,

	opAndType{ODIV, types.Float32}: ssa.OpDiv32F,
	opAndType{ODIV, types.Float64}: ssa.OpDiv64F,

	opAndType{OHMUL, types.Int8}:   ssa.OpHmul8,
	opAndType{OHMUL, types.Uint8}:  ssa.OpHmul8u,
	opAndType{OHMUL, types.Int16}:  ssa.OpHmul16,
	opAndType{OHMUL, types.Uint16}: ssa.OpHmul16u,
	opAndType{OHMUL, types.Int32}:  ssa.OpHmul32,
	opAndType{OHMUL, types.Uint32}: ssa.OpHmul32u,

	opAndType{ODIV, types.Int8}:   ssa.OpDiv8,
	opAndType{ODIV, types.Uint8}:  ssa.OpDiv8u,
	opAndType{ODIV, types.Int16}:  ssa.OpDiv16,
	opAndType{ODIV, types.Uint16}: ssa.OpDiv16u,
	opAndType{ODIV, types.Int32}:  ssa.OpDiv32,
	opAndType{ODIV, types.Uint32}: ssa.OpDiv32u,
	opAndType{ODIV, types.Int64}:  ssa.OpDiv64,
	opAndType{ODIV, types.Uint64}: ssa.OpDiv64u,

	opAndType{OMOD, types.Int8}:   ssa.OpMod8,
	opAndType{OMOD, types.Uint8}:  ssa.OpMod8u,
	opAndType{OMOD, types.Int16}:  ssa.OpMod16,
	opAndType{OMOD, types.Uint16}: ssa.OpMod16u,
	opAndType{OMOD, types.Int32}:  ssa.OpMod32,
	opAndType{OMOD, types.Uint32}: ssa.OpMod32u,
	opAndType{OMOD, types.Int64}:  ssa.OpMod64,
	opAndType{OMOD, types.Uint64}: ssa.OpMod64u,

	opAndType{OAND, types.Int8}:   ssa.OpAnd8,
	opAndType{OAND, types.Uint8}:  ssa.OpAnd8,
	opAndType{OAND, types.Int16}:  ssa.OpAnd16,
	opAndType{OAND, types.Uint16}: ssa.OpAnd16,
	opAndType{OAND, types.Int32}:  ssa.OpAnd32,
	opAndType{OAND, types.Uint32}: ssa.OpAnd32,
	opAndType{OAND, types.Int64}:  ssa.OpAnd64,
	opAndType{OAND, types.Uint64}: ssa.OpAnd64,

	opAndType{OOR, types.Int8}:   ssa.OpOr8,
	opAndType{OOR, types.Uint8}:  ssa.OpOr8,
	opAndType{OOR, types.Int16}:  ssa.OpOr16,
	opAndType{OOR, types.Uint16}: ssa.OpOr16,
	opAndType{OOR, types.Int32}:  ssa.OpOr32,
	opAndType{OOR, types.Uint32}: ssa.OpOr32,
	opAndType{OOR, types.Int64}:  ssa.OpOr64,
	opAndType{OOR, types.Uint64}: ssa.OpOr64,

	opAndType{OXOR, types.Int8}:   ssa.OpXor8,
	opAndType{OXOR, types.Uint8}:  ssa.OpXor8,
	opAndType{OXOR, types.Int16}:  ssa.OpXor16,
	opAndType{OXOR, types.Uint16}: ssa.OpXor16,
	opAndType{OXOR, types.Int32}:  ssa.OpXor32,
	opAndType{OXOR, types.Uint32}: ssa.OpXor32,
	opAndType{OXOR, types.Int64}:  ssa.OpXor64,
	opAndType{OXOR, types.Uint64}: ssa.OpXor64,

	opAndType{OEQ, types.Bool}:   ssa.OpEq8,
	opAndType{OEQ, types.Int8}:   ssa.OpEq8,
	opAndType{OEQ, types.Uint8}:  ssa.OpEq8,
	opAndType{OEQ, types.Int16}:  ssa.OpEq16,
	opAndType{OEQ, types.Uint16}: ssa.OpEq16,
	opAndType{OEQ, types.Int32}:  ssa.OpEq32,
	opAndType{OEQ, types.Uint32}: ssa.OpEq32,
	opAndType{OEQ, types.Int64}:  ssa.OpEq64,
	opAndType{OEQ, types.Uint64}: ssa.OpEq64,
	// opAndType{OEQ, types.Inter}:     ssa.OpEqInter,
	// opAndType{OEQ, types.Array}:     ssa.OpEqSlice,
	// opAndType{OEQ, types.Func}:      ssa.OpEqPtr,
	// opAndType{OEQ, types.Map}:       ssa.OpEqPtr,
	// opAndType{OEQ, types.Chan}:      ssa.OpEqPtr,
	// opAndType{OEQ, types.Ptr64}:     ssa.OpEqPtr,
	opAndType{OEQ, types.Uintptr}: ssa.OpEqPtr,
	// opAndType{OEQ, types.Unsafeptr}: ssa.OpEqPtr,
	opAndType{OEQ, types.Float64}: ssa.OpEq64F,
	opAndType{OEQ, types.Float32}: ssa.OpEq32F,

	opAndType{ONE, types.Bool}:   ssa.OpNeq8,
	opAndType{ONE, types.Int8}:   ssa.OpNeq8,
	opAndType{ONE, types.Uint8}:  ssa.OpNeq8,
	opAndType{ONE, types.Int16}:  ssa.OpNeq16,
	opAndType{ONE, types.Uint16}: ssa.OpNeq16,
	opAndType{ONE, types.Int32}:  ssa.OpNeq32,
	opAndType{ONE, types.Uint32}: ssa.OpNeq32,
	opAndType{ONE, types.Int64}:  ssa.OpNeq64,
	opAndType{ONE, types.Uint64}: ssa.OpNeq64,
	// opAndType{ONE, types.Inter}:     ssa.OpNeqInter,
	// opAndType{ONE, types.Array}:     ssa.OpNeqSlice,
	// opAndType{ONE, types.Func}:      ssa.OpNeqPtr,
	// opAndType{ONE, types.Map}:       ssa.OpNeqPtr,
	// opAndType{ONE, types.Chan}:      ssa.OpNeqPtr,
	// opAndType{ONE, types.Ptr64}:     ssa.OpNeqPtr,
	opAndType{ONE, types.Uintptr}: ssa.OpNeqPtr,
	// opAndType{ONE, types.Unsafeptr}: ssa.OpNeqPtr,
	opAndType{ONE, types.Float64}: ssa.OpNeq64F,
	opAndType{ONE, types.Float32}: ssa.OpNeq32F,

	opAndType{OLT, types.Int8}:    ssa.OpLess8,
	opAndType{OLT, types.Uint8}:   ssa.OpLess8U,
	opAndType{OLT, types.Int16}:   ssa.OpLess16,
	opAndType{OLT, types.Uint16}:  ssa.OpLess16U,
	opAndType{OLT, types.Int32}:   ssa.OpLess32,
	opAndType{OLT, types.Uint32}:  ssa.OpLess32U,
	opAndType{OLT, types.Int64}:   ssa.OpLess64,
	opAndType{OLT, types.Uint64}:  ssa.OpLess64U,
	opAndType{OLT, types.Float64}: ssa.OpLess64F,
	opAndType{OLT, types.Float32}: ssa.OpLess32F,

	opAndType{OGT, types.Int8}:    ssa.OpGreater8,
	opAndType{OGT, types.Uint8}:   ssa.OpGreater8U,
	opAndType{OGT, types.Int16}:   ssa.OpGreater16,
	opAndType{OGT, types.Uint16}:  ssa.OpGreater16U,
	opAndType{OGT, types.Int32}:   ssa.OpGreater32,
	opAndType{OGT, types.Uint32}:  ssa.OpGreater32U,
	opAndType{OGT, types.Int64}:   ssa.OpGreater64,
	opAndType{OGT, types.Uint64}:  ssa.OpGreater64U,
	opAndType{OGT, types.Float64}: ssa.OpGreater64F,
	opAndType{OGT, types.Float32}: ssa.OpGreater32F,

	opAndType{OLE, types.Int8}:    ssa.OpLeq8,
	opAndType{OLE, types.Uint8}:   ssa.OpLeq8U,
	opAndType{OLE, types.Int16}:   ssa.OpLeq16,
	opAndType{OLE, types.Uint16}:  ssa.OpLeq16U,
	opAndType{OLE, types.Int32}:   ssa.OpLeq32,
	opAndType{OLE, types.Uint32}:  ssa.OpLeq32U,
	opAndType{OLE, types.Int64}:   ssa.OpLeq64,
	opAndType{OLE, types.Uint64}:  ssa.OpLeq64U,
	opAndType{OLE, types.Float64}: ssa.OpLeq64F,
	opAndType{OLE, types.Float32}: ssa.OpLeq32F,

	opAndType{OGE, types.Int8}:    ssa.OpGeq8,
	opAndType{OGE, types.Uint8}:   ssa.OpGeq8U,
	opAndType{OGE, types.Int16}:   ssa.OpGeq16,
	opAndType{OGE, types.Uint16}:  ssa.OpGeq16U,
	opAndType{OGE, types.Int32}:   ssa.OpGeq32,
	opAndType{OGE, types.Uint32}:  ssa.OpGeq32U,
	opAndType{OGE, types.Int64}:   ssa.OpGeq64,
	opAndType{OGE, types.Uint64}:  ssa.OpGeq64U,
	opAndType{OGE, types.Float64}: ssa.OpGeq64F,
	opAndType{OGE, types.Float32}: ssa.OpGeq32F,

	// opAndType{OLROT, types.Uint8}:  ssa.OpLrot8,
	// opAndType{OLROT, types.Uint16}: ssa.OpLrot16,
	// opAndType{OLROT, types.Uint32}: ssa.OpLrot32,
	// opAndType{OLROT, types.Uint64}: ssa.OpLrot64,

	opAndType{OSQRT, types.Float64}: ssa.OpSqrt,
}

func (s *state) ssaOp(op NodeOp, t *Type) ssa.Op {
	/*etype := s.concreteEtype(t)
	x, ok := opToSSA[opAndType{op, etype}]
	if !ok {
		//s.Unimplementedf("unhandled binary op %s %s", opnames[op], Econv(int(etype), 0))
	}
	return x*/
	return opToSSA[opAndType{}]
}

func floatForComplex(t *Type) *Type {
	if t.Size() == 8 {
		return Typ[types.Float32]
	} else {
		return Typ[types.Float64]
	}
}

type opAndTwoTypes struct {
	op     NodeOp
	etype1 types.BasicKind
	etype2 types.BasicKind
}

type twoTypes struct {
	etype1 types.BasicKind
	etype2 types.BasicKind
}

type twoOpsAndType struct {
	op1              ssa.Op
	op2              ssa.Op
	intermediateType types.BasicKind
}

var fpConvOpToSSA = map[twoTypes]twoOpsAndType{

	twoTypes{types.Int8, types.Float32}:  twoOpsAndType{ssa.OpSignExt8to32, ssa.OpCvt32to32F, types.Int32},
	twoTypes{types.Int16, types.Float32}: twoOpsAndType{ssa.OpSignExt16to32, ssa.OpCvt32to32F, types.Int32},
	twoTypes{types.Int32, types.Float32}: twoOpsAndType{ssa.OpCopy, ssa.OpCvt32to32F, types.Int32},
	twoTypes{types.Int64, types.Float32}: twoOpsAndType{ssa.OpCopy, ssa.OpCvt64to32F, types.Int64},

	twoTypes{types.Int8, types.Float64}:  twoOpsAndType{ssa.OpSignExt8to32, ssa.OpCvt32to64F, types.Int32},
	twoTypes{types.Int16, types.Float64}: twoOpsAndType{ssa.OpSignExt16to32, ssa.OpCvt32to64F, types.Int32},
	twoTypes{types.Int32, types.Float64}: twoOpsAndType{ssa.OpCopy, ssa.OpCvt32to64F, types.Int32},
	twoTypes{types.Int64, types.Float64}: twoOpsAndType{ssa.OpCopy, ssa.OpCvt64to64F, types.Int64},

	twoTypes{types.Float32, types.Int8}:  twoOpsAndType{ssa.OpCvt32Fto32, ssa.OpTrunc32to8, types.Int32},
	twoTypes{types.Float32, types.Int16}: twoOpsAndType{ssa.OpCvt32Fto32, ssa.OpTrunc32to16, types.Int32},
	twoTypes{types.Float32, types.Int32}: twoOpsAndType{ssa.OpCvt32Fto32, ssa.OpCopy, types.Int32},
	twoTypes{types.Float32, types.Int64}: twoOpsAndType{ssa.OpCvt32Fto64, ssa.OpCopy, types.Int64},

	twoTypes{types.Float64, types.Int8}:  twoOpsAndType{ssa.OpCvt64Fto32, ssa.OpTrunc32to8, types.Int32},
	twoTypes{types.Float64, types.Int16}: twoOpsAndType{ssa.OpCvt64Fto32, ssa.OpTrunc32to16, types.Int32},
	twoTypes{types.Float64, types.Int32}: twoOpsAndType{ssa.OpCvt64Fto32, ssa.OpCopy, types.Int32},
	twoTypes{types.Float64, types.Int64}: twoOpsAndType{ssa.OpCvt64Fto64, ssa.OpCopy, types.Int64},
	// unsigned
	twoTypes{types.Uint8, types.Float32}:  twoOpsAndType{ssa.OpZeroExt8to32, ssa.OpCvt32to32F, types.Int32},
	twoTypes{types.Uint16, types.Float32}: twoOpsAndType{ssa.OpZeroExt16to32, ssa.OpCvt32to32F, types.Int32},
	twoTypes{types.Uint32, types.Float32}: twoOpsAndType{ssa.OpZeroExt32to64, ssa.OpCvt64to32F, types.Int64}, // go wide to dodge unsigned
	twoTypes{types.Uint64, types.Float32}: twoOpsAndType{ssa.OpCopy, ssa.OpInvalid, types.Uint64},            // Cvt64Uto32F, branchy code expansion instead

	twoTypes{types.Uint8, types.Float64}:  twoOpsAndType{ssa.OpZeroExt8to32, ssa.OpCvt32to64F, types.Int32},
	twoTypes{types.Uint16, types.Float64}: twoOpsAndType{ssa.OpZeroExt16to32, ssa.OpCvt32to64F, types.Int32},
	twoTypes{types.Uint32, types.Float64}: twoOpsAndType{ssa.OpZeroExt32to64, ssa.OpCvt64to64F, types.Int64}, // go wide to dodge unsigned
	twoTypes{types.Uint64, types.Float64}: twoOpsAndType{ssa.OpCopy, ssa.OpInvalid, types.Uint64},            // Cvt64Uto64F, branchy code expansion instead

	twoTypes{types.Float32, types.Uint8}:  twoOpsAndType{ssa.OpCvt32Fto32, ssa.OpTrunc32to8, types.Int32},
	twoTypes{types.Float32, types.Uint16}: twoOpsAndType{ssa.OpCvt32Fto32, ssa.OpTrunc32to16, types.Int32},
	twoTypes{types.Float32, types.Uint32}: twoOpsAndType{ssa.OpCvt32Fto64, ssa.OpTrunc64to32, types.Int64}, // go wide to dodge unsigned
	twoTypes{types.Float32, types.Uint64}: twoOpsAndType{ssa.OpInvalid, ssa.OpCopy, types.Uint64},          // Cvt32Fto64U, branchy code expansion instead

	twoTypes{types.Float64, types.Uint8}:  twoOpsAndType{ssa.OpCvt64Fto32, ssa.OpTrunc32to8, types.Int32},
	twoTypes{types.Float64, types.Uint16}: twoOpsAndType{ssa.OpCvt64Fto32, ssa.OpTrunc32to16, types.Int32},
	twoTypes{types.Float64, types.Uint32}: twoOpsAndType{ssa.OpCvt64Fto64, ssa.OpTrunc64to32, types.Int64}, // go wide to dodge unsigned
	twoTypes{types.Float64, types.Uint64}: twoOpsAndType{ssa.OpInvalid, ssa.OpCopy, types.Uint64},          // Cvt64Fto64U, branchy code expansion instead

	// float
	twoTypes{types.Float64, types.Float32}: twoOpsAndType{ssa.OpCvt64Fto32F, ssa.OpCopy, types.Float32},
	twoTypes{types.Float64, types.Float64}: twoOpsAndType{ssa.OpCopy, ssa.OpCopy, types.Float64},
	twoTypes{types.Float32, types.Float32}: twoOpsAndType{ssa.OpCopy, ssa.OpCopy, types.Float32},
	twoTypes{types.Float32, types.Float64}: twoOpsAndType{ssa.OpCvt32Fto64F, ssa.OpCopy, types.Float64},
}

var shiftOpToSSA = map[opAndTwoTypes]ssa.Op{
	opAndTwoTypes{OLSH, types.Int8, types.Uint8}:   ssa.OpLsh8x8,
	opAndTwoTypes{OLSH, types.Uint8, types.Uint8}:  ssa.OpLsh8x8,
	opAndTwoTypes{OLSH, types.Int8, types.Uint16}:  ssa.OpLsh8x16,
	opAndTwoTypes{OLSH, types.Uint8, types.Uint16}: ssa.OpLsh8x16,
	opAndTwoTypes{OLSH, types.Int8, types.Uint32}:  ssa.OpLsh8x32,
	opAndTwoTypes{OLSH, types.Uint8, types.Uint32}: ssa.OpLsh8x32,
	opAndTwoTypes{OLSH, types.Int8, types.Uint64}:  ssa.OpLsh8x64,
	opAndTwoTypes{OLSH, types.Uint8, types.Uint64}: ssa.OpLsh8x64,

	opAndTwoTypes{OLSH, types.Int16, types.Uint8}:   ssa.OpLsh16x8,
	opAndTwoTypes{OLSH, types.Uint16, types.Uint8}:  ssa.OpLsh16x8,
	opAndTwoTypes{OLSH, types.Int16, types.Uint16}:  ssa.OpLsh16x16,
	opAndTwoTypes{OLSH, types.Uint16, types.Uint16}: ssa.OpLsh16x16,
	opAndTwoTypes{OLSH, types.Int16, types.Uint32}:  ssa.OpLsh16x32,
	opAndTwoTypes{OLSH, types.Uint16, types.Uint32}: ssa.OpLsh16x32,
	opAndTwoTypes{OLSH, types.Int16, types.Uint64}:  ssa.OpLsh16x64,
	opAndTwoTypes{OLSH, types.Uint16, types.Uint64}: ssa.OpLsh16x64,

	opAndTwoTypes{OLSH, types.Int32, types.Uint8}:   ssa.OpLsh32x8,
	opAndTwoTypes{OLSH, types.Uint32, types.Uint8}:  ssa.OpLsh32x8,
	opAndTwoTypes{OLSH, types.Int32, types.Uint16}:  ssa.OpLsh32x16,
	opAndTwoTypes{OLSH, types.Uint32, types.Uint16}: ssa.OpLsh32x16,
	opAndTwoTypes{OLSH, types.Int32, types.Uint32}:  ssa.OpLsh32x32,
	opAndTwoTypes{OLSH, types.Uint32, types.Uint32}: ssa.OpLsh32x32,
	opAndTwoTypes{OLSH, types.Int32, types.Uint64}:  ssa.OpLsh32x64,
	opAndTwoTypes{OLSH, types.Uint32, types.Uint64}: ssa.OpLsh32x64,

	opAndTwoTypes{OLSH, types.Int64, types.Uint8}:   ssa.OpLsh64x8,
	opAndTwoTypes{OLSH, types.Uint64, types.Uint8}:  ssa.OpLsh64x8,
	opAndTwoTypes{OLSH, types.Int64, types.Uint16}:  ssa.OpLsh64x16,
	opAndTwoTypes{OLSH, types.Uint64, types.Uint16}: ssa.OpLsh64x16,
	opAndTwoTypes{OLSH, types.Int64, types.Uint32}:  ssa.OpLsh64x32,
	opAndTwoTypes{OLSH, types.Uint64, types.Uint32}: ssa.OpLsh64x32,
	opAndTwoTypes{OLSH, types.Int64, types.Uint64}:  ssa.OpLsh64x64,
	opAndTwoTypes{OLSH, types.Uint64, types.Uint64}: ssa.OpLsh64x64,

	opAndTwoTypes{ORSH, types.Int8, types.Uint8}:   ssa.OpRsh8x8,
	opAndTwoTypes{ORSH, types.Uint8, types.Uint8}:  ssa.OpRsh8Ux8,
	opAndTwoTypes{ORSH, types.Int8, types.Uint16}:  ssa.OpRsh8x16,
	opAndTwoTypes{ORSH, types.Uint8, types.Uint16}: ssa.OpRsh8Ux16,
	opAndTwoTypes{ORSH, types.Int8, types.Uint32}:  ssa.OpRsh8x32,
	opAndTwoTypes{ORSH, types.Uint8, types.Uint32}: ssa.OpRsh8Ux32,
	opAndTwoTypes{ORSH, types.Int8, types.Uint64}:  ssa.OpRsh8x64,
	opAndTwoTypes{ORSH, types.Uint8, types.Uint64}: ssa.OpRsh8Ux64,

	opAndTwoTypes{ORSH, types.Int16, types.Uint8}:   ssa.OpRsh16x8,
	opAndTwoTypes{ORSH, types.Uint16, types.Uint8}:  ssa.OpRsh16Ux8,
	opAndTwoTypes{ORSH, types.Int16, types.Uint16}:  ssa.OpRsh16x16,
	opAndTwoTypes{ORSH, types.Uint16, types.Uint16}: ssa.OpRsh16Ux16,
	opAndTwoTypes{ORSH, types.Int16, types.Uint32}:  ssa.OpRsh16x32,
	opAndTwoTypes{ORSH, types.Uint16, types.Uint32}: ssa.OpRsh16Ux32,
	opAndTwoTypes{ORSH, types.Int16, types.Uint64}:  ssa.OpRsh16x64,
	opAndTwoTypes{ORSH, types.Uint16, types.Uint64}: ssa.OpRsh16Ux64,

	opAndTwoTypes{ORSH, types.Int32, types.Uint8}:   ssa.OpRsh32x8,
	opAndTwoTypes{ORSH, types.Uint32, types.Uint8}:  ssa.OpRsh32Ux8,
	opAndTwoTypes{ORSH, types.Int32, types.Uint16}:  ssa.OpRsh32x16,
	opAndTwoTypes{ORSH, types.Uint32, types.Uint16}: ssa.OpRsh32Ux16,
	opAndTwoTypes{ORSH, types.Int32, types.Uint32}:  ssa.OpRsh32x32,
	opAndTwoTypes{ORSH, types.Uint32, types.Uint32}: ssa.OpRsh32Ux32,
	opAndTwoTypes{ORSH, types.Int32, types.Uint64}:  ssa.OpRsh32x64,
	opAndTwoTypes{ORSH, types.Uint32, types.Uint64}: ssa.OpRsh32Ux64,

	opAndTwoTypes{ORSH, types.Int64, types.Uint8}:   ssa.OpRsh64x8,
	opAndTwoTypes{ORSH, types.Uint64, types.Uint8}:  ssa.OpRsh64Ux8,
	opAndTwoTypes{ORSH, types.Int64, types.Uint16}:  ssa.OpRsh64x16,
	opAndTwoTypes{ORSH, types.Uint64, types.Uint16}: ssa.OpRsh64Ux16,
	opAndTwoTypes{ORSH, types.Int64, types.Uint32}:  ssa.OpRsh64x32,
	opAndTwoTypes{ORSH, types.Uint64, types.Uint32}: ssa.OpRsh64Ux32,
	opAndTwoTypes{ORSH, types.Int64, types.Uint64}:  ssa.OpRsh64x64,
	opAndTwoTypes{ORSH, types.Uint64, types.Uint64}: ssa.OpRsh64Ux64,
}

func (s *state) ssaShiftOp(op NodeOp, t *Type, u *Type) ssa.Op {
	return opToSSA[opAndType{}]
	/*etype1 := s.concreteEtype(t)
	etype2 := s.concreteEtype(u)
	x, ok := shiftOpToSSA[opAndTwoTypes{op, etype1, etype2}]
	if !ok {
		//s.Unimplementedf("unhandled shift op %s etype=%s/%s", opnames[op], Econv(int(etype1), 0), Econv(int(etype2), 0))
	}
	return x*/
}

func (s *state) ssaRotateOp(op NodeOp, t *Type) ssa.Op {
	return opToSSA[opAndType{}]
	/*etype1 := s.concreteEtype(t)
	x, ok := opToSSA[opAndType{op, etype1}]
	if !ok {
		//s.Unimplementedf("unhandled rotate op %s etype=%s", opnames[op], Econv(int(etype1), 0))
	}
	return x*/
}

func (s *state) ssaVar(n *Node) ssaVar {
	// fn := s.ctx.fn
	// scope := fn.Scopes[n.node]
	// typeObject := scope.Lookup(n.Name())
	// if typeObject == nil {
	// 	panic("couldn't lookup node in scope")
	// }
	// typeObject.
	// fn.Defs

	vars := getVars(s.ctx, s.fnDecl, s.fnType)
	for _, v := range vars {
		if v.Name() == n.Name() {
			return v
		}
	}
	fmt.Printf("n: %#v", n)
	panic("couldn't find var for node n")
}

// expr converts the expression n to ssa, adds it to s and returns the ssa result.
func (s *state) expr(n *Node) *ssa.Value {
	//return nil
	// TODO
	//s.stmtList(n.Ninit)
	ctx := s.ctx

	switch expr := n.node.(type) {
	case *ast.Ident:
		if canSSA(n) {
			ssaVar := s.ssaVar(n)
			return s.variable(ssaVar, n.Typ())
		}
		panic(fmt.Sprintf("unimplementedf for expr: %#v", expr))
		// addr := s.addr(n, false)
		// return s.newValue2(ssa.OpLoad, n.Type, addr, s.mem())
	case *ast.BasicLit:
		typeAndValue := ctx.fn.Types[expr]
		// t := typeAndValue.Type
		v := typeAndValue.Value

		switch v.Kind() {
		case constant.Int:
			i, ok := constant.Int64Val(v)
			if !ok {
				panic("internal error")
			}
			switch n.Typ().Size() {
			case 1:
				return s.constInt8(n.Typ(), int8(i))
			case 2:
				return s.constInt16(n.Typ(), int16(i))
			case 4:
				return s.constInt32(n.Typ(), int32(i))
			case 8:
				return s.constInt64(n.Typ(), i)
			default:
				s.Fatalf("bad integer size %d", n.Typ().Size())
				return nil
			}
		case constant.String:
			return s.entryNewValue0A(ssa.OpConstString, n.Typ(), constant.StringVal(v))
		case constant.Bool:
			return s.constBool(constant.BoolVal(v))
		case constant.Unknown:
			panic("unknown basic literal")

		case constant.Float:
			f, ok := constant.Float64Val(v)
			if !ok {
				panic("internal error")
			}

			switch n.Typ().Size() {
			case 4:
				// -0.0 literals need to be treated as if they were 0.0, adding 0.0 here
				// accomplishes this while not affecting other values.
				return s.constFloat32(n.Typ(), float64(float32(f)+0.0))
			case 8:
				return s.constFloat64(n.Typ(), f+0.0)
			default:
				s.Fatalf("bad float size %d", n.Typ().Size())
				return nil
			}
		case constant.Complex:
			panic("complex numbers not supported")
		default:
			s.Unimplementedf("unhandled literal %#v", expr)
			return nil
		}
	case *ast.BinaryExpr:
		// TODO
		switch expr.Op {
		case token.ADD:
			return s.entryNewValue2(ssa.OpAdd64, n.Typ(), s.expr(ExprNode(expr.X, s.ctx)), s.expr(ExprNode(expr.Y, s.ctx)))
		case token.SUB:
			//
		case token.MUL:
			//
		case token.QUO:
			//
		case token.REM:
			//
		case token.AND:
			//
		case token.OR:
			//
		case token.XOR:
			//
		case token.SHL:
			//
		case token.SHR:
			//
		case token.AND_NOT:
			//
		case token.LAND:
			//
		case token.LEQ:
			//
		case token.LOR:
			//
		case token.LSS:
			//
		case token.GTR:
			//

		}
		panic("unimplementedf *ast.BinaryExpr")
	default:
		panic(fmt.Sprintf("unimplemented expr: %#v", expr))
	}
}

// condBranch evaluates the boolean expression cond and branches to yes
// if cond is true and no if cond is false.
// This function is intended to handle && and || better than just calling
// s.expr(cond) and branching on the result.
func (s *state) condBranch(cond ast.Expr, yes, no *ssa.Block) {
	switch e := cond.(type) {
	case *ast.ParenExpr:
		s.condBranch(e.X, yes, no)
		return

	case *ast.BinaryExpr:
		switch e.Op {
		case token.LAND:
			ltrue := s.f.NewBlock(ssa.BlockPlain) // "cond.true"
			s.condBranch(e.X, ltrue, no)
			s.curBlock = ltrue
			s.condBranch(e.Y, yes, no)
			return

		case token.LOR:
			lfalse := s.f.NewBlock(ssa.BlockPlain) // "cond.false"
			s.condBranch(e.X, yes, lfalse)
			s.curBlock = lfalse
			s.condBranch(e.Y, yes, no)
			return
		}

	case *ast.UnaryExpr:
		if e.Op == token.NOT {
			s.condBranch(e.X, no, yes)
			return
		}
	}
	c := s.expr(NewNode(cond, s.ctx))
	b := s.endBlock()
	b.Kind = ssa.BlockIf
	b.Control = c
	b.Likely = 0
	b.AddEdgeTo(yes)
	b.AddEdgeTo(no)
}

//assign(left *Node, right *ssa.Value, wb bool) {
func (s *state) assignStmt(stmt *ast.AssignStmt) {
	if len(stmt.Lhs) > 1 || len(stmt.Rhs) > 1 {
		s.Errorf("Multivalue assignments not allowed")
		return
	}
	if len(stmt.Lhs) == 0 || len(stmt.Rhs) == 0 {
		panic("internal error")
	}
	leftExpr := stmt.Lhs[0]
	rightExpr := stmt.Rhs[0]
	leftIdent, ok := leftExpr.(*ast.Ident)
	if !ok {
		s.Errorf("expected ident")
		return
	}
	rightValue := s.expr(&Node{node: rightExpr, ctx: s.ctx, class: PAUTO})
	if stmt.Tok == token.DEFINE {
		leftNode := &Node{node: leftIdent, ctx: s.ctx, class: PAUTO}
		leftObj := s.fnInfo.ObjectOf(leftIdent)
		leftVar := &ssaLocal{obj: leftObj, ctx: s.ctx}
		s.addNamedValue(leftNode, rightValue)
		if canSSA(leftNode) {
			// Update variable assignment.
			s.vars[leftVar] = rightValue
			s.addNamedValue(leftNode, rightValue)
			return
		} else {
			panic("can't ssa node")
		}
	} else if stmt.Tok == token.ASSIGN {
		leftNode := &Node{node: leftIdent, ctx: s.ctx, class: PAUTO}
		leftObj := s.fnInfo.ObjectOf(leftIdent)
		leftVar := &ssaLocal{obj: leftObj, ctx: s.ctx}
		s.addNamedValue(leftNode, rightValue)
		if canSSA(leftNode) {
			// Update variable assignment.
			s.vars[leftVar] = rightValue
			s.addNamedValue(leftNode, rightValue)
			return
		} else {
			panic("can't ssa node")
		}
	} else {
		panic("internal error")
	}

}

func canSSA(n ssaVar) bool {
	switch n.Class() {
	case PEXTERN, PPARAMOUT, PPARAMREF:
		return false
	}
	return true
}

// zeroVal returns the zero value for type t.
func (s *state) zeroVal(t *Type) *ssa.Value {
	switch {
	case t.IsInteger():
		switch t.Size() {
		case 1:
			return s.constInt8(t, 0)
		case 2:
			return s.constInt16(t, 0)
		case 4:
			return s.constInt32(t, 0)
		case 8:
			return s.constInt64(t, 0)
		default:
			s.Fatalf("bad sized integer type %s", t)
		}
	case t.IsFloat():
		switch t.Size() {
		case 4:
			return s.constFloat32(t, 0)
		case 8:
			return s.constFloat64(t, 0)
		default:
			s.Fatalf("bad sized float type %s", t)
		}
	case t.IsComplex():
		switch t.Size() {
		case 8:
			z := s.constFloat32(Typ[types.Float32], 0)
			return s.entryNewValue2(ssa.OpComplexMake, t, z, z)
		case 16:
			z := s.constFloat64(Typ[types.Float64], 0)
			return s.entryNewValue2(ssa.OpComplexMake, t, z, z)
		default:
			s.Fatalf("bad sized complex type %s", t)
		}

	case t.IsString():
		return s.entryNewValue0A(ssa.OpConstString, t, "")
	case t.IsPtr():
		return s.entryNewValue0(ssa.OpConstNil, t)
	case t.IsBoolean():
		return s.constBool(false)
	case t.IsInterface():
		return s.entryNewValue0(ssa.OpConstInterface, t)
	case t.IsSlice():
		return s.entryNewValue0(ssa.OpConstSlice, t)
	}
	s.Unimplementedf("zero for type %v not implemented", t)
	return nil
}

// lookupSymbol is used to retrieve the symbol (Extern, Arg or Auto) used for a particular node.
// This improves the effectiveness of cse by using the same Aux values for the
// same symbols.
func (s *state) lookupSymbol(n *Node, sym interface{}) interface{} {
	switch sym.(type) {
	default:
		s.Fatalf("sym %v is of uknown type %T", sym, sym)
	case *ssa.ExternSymbol, *ssa.ArgSymbol, *ssa.AutoSymbol:
		// these are the only valid types
	}

	// if lsym, ok := s.varsyms[n]; ok {
	// 	return lsym
	// } else {
	// 	s.varsyms[n] = sym
	// 	return sym
	// }
	return nil
}

// addr converts the address of the expression n to SSA, adds it to s and returns the SSA result.
// The value that the returned Value represents is guaranteed to be non-nil.
// If bounded is true then this address does not require a nil check for its operand
// even if that would otherwise be implied.
func (s *state) addr(n *Node, bounded bool) *ssa.Value {
	return nil
	// t := Ptrto(n.Type())
	// switch n.Op() {
	// case ONAME:
	// 	switch n.Class() {
	// 	case PEXTERN:
	// 		panic("External variables are unsupported")
	// 	case PPARAM:
	// 		// parameter slot
	// 		v := s.decladdrs[n]
	// 		if v == nil {
	// 			if flag_race != 0 && n.String() == ".fp" {
	// 				s.Unimplementedf("race detector mishandles nodfp")
	// 			}
	// 			s.Fatalf("addr of undeclared ONAME %v. declared: %v", n, s.decladdrs)
	// 		}
	// 		return v
	// 	case PAUTO:
	// 		// We need to regenerate the address of autos
	// 		// at every use.  This prevents LEA instructions
	// 		// from occurring before the corresponding VarDef
	// 		// op and confusing the liveness analysis into thinking
	// 		// the variable is live at function entry.
	// 		// TODO: I'm not sure if this really works or we're just
	// 		// getting lucky.  We might need a real dependency edge
	// 		// between vardef and addr ops.
	// 		aux := &ssa.AutoSymbol{Typ: n.Type(), Node: n}
	// 		return s.newValue1A(ssa.OpAddr, t, aux, s.sp)
	// 	case PPARAMOUT: // Same as PAUTO -- cannot generate LEA early.
	// 		// ensure that we reuse symbols for out parameters so
	// 		// that cse works on their addresses
	// 		aux := s.lookupSymbol(n, &ssa.ArgSymbol{Typ: n.Type(), Node: n})
	// 		return s.newValue1A(ssa.OpAddr, t, aux, s.sp)
	// 	case PAUTO | PHEAP, PPARAM | PHEAP, PPARAMOUT | PHEAP, PPARAMREF:
	// 		return s.expr(n.Name().Heapaddr())
	// 	default:
	// 		s.Unimplementedf("variable address class %v not implemented", n.Class)
	// 		return nil
	// 	}
	// case OINDREG:
	// 	// indirect off a register
	// 	// used for storing/loading arguments/returns to/from callees
	// 	if int(n.Reg()) != Thearch.REGSP {
	// 		s.Unimplementedf("OINDREG of non-SP register %s in addr: %v", "n.Reg", n) //obj.Rconv(int(n.Reg)), n)
	// 		return nil
	// 	}
	// 	return s.entryNewValue1I(ssa.OpOffPtr, t, n.Xoffset(), s.sp)
	// case OINDEX:
	// 	if n.Left().Type().IsSlice() {
	// 		a := s.expr(n.Left())
	// 		i := s.expr(n.Right())
	// 		i = s.extendIndex(i)
	// 		len := s.newValue1(ssa.OpSliceLen, Types[TINT], a)
	// 		if !n.Bounded() {
	// 			s.boundsCheck(i, len)
	// 		}
	// 		p := s.newValue1(ssa.OpSlicePtr, t, a)
	// 		return s.newValue2(ssa.OpPtrIndex, t, p, i)
	// 	} else { // array
	// 		a := s.addr(n.Left(), bounded)
	// 		i := s.expr(n.Right())
	// 		i = s.extendIndex(i)
	// 		len := s.constInt(Types[TINT], n.Left().Type().Bound())
	// 		if !n.Bounded() {
	// 			s.boundsCheck(i, len)
	// 		}
	// 		et := n.Left().Type().Elem()
	// 		elemType := et.(*Type)
	// 		return s.newValue2(ssa.OpPtrIndex, Ptrto(elemType), a, i)
	// 	}
	// case OIND:
	// 	p := s.expr(n.Left())
	// 	if !bounded {
	// 		s.nilCheck(p)
	// 	}
	// 	return p
	// case ODOT:
	// 	p := s.addr(n.Left(), bounded)
	// 	return s.newValue2(ssa.OpAddPtr, t, p, s.constInt(Types[TINT], n.Xoffset()))
	// case ODOTPTR:
	// 	p := s.expr(n.Left())
	// 	if !bounded {
	// 		s.nilCheck(p)
	// 	}
	// 	return s.newValue2(ssa.OpAddPtr, t, p, s.constInt(Types[TINT], n.Xoffset()))
	// case OCLOSUREVAR:
	// 	return s.newValue2(ssa.OpAddPtr, t,
	// 		s.entryNewValue0(ssa.OpGetClosurePtr, Ptrto(Types[TUINT8])),
	// 		s.constInt(Types[TINT], n.Xoffset()))
	// case OPARAM:
	// 	p := n.Left()
	// 	if p.Op() != ONAME || !(p.Class() == PPARAM|PHEAP || p.Class() == PPARAMOUT|PHEAP) {
	// 		panic("OPARAM not of ONAME,{PPARAM,PPARAMOUT}|PHEAP")
	// 	}

	// 	// Recover original offset to address passed-in param value.
	// 	original_p := *p
	// 	//original_p.Xoffset() = n.Xoffset()
	// 	aux := &ssa.ArgSymbol{Typ: n.Type(), Node: &original_p}
	// 	return s.entryNewValue1A(ssa.OpAddr, t, aux, s.sp)
	// case OCONVNOP:
	// 	addr := s.addr(n.Left(), bounded)
	// 	return s.newValue1(ssa.OpCopy, t, addr) // ensure that addr has the right type

	// default:
	// 	s.Unimplementedf("unhandled addr %v", Oconv(int(n.Op()), 0))
	// 	return nil
	// }
}

// checkGoto checks that a goto from from to to does not
// jump into a block
func (s *state) checkGoto(from *Node, to *Node) {
	// TODO: determine if goto jumps into a block
	var block *ssa.Block
	if block != nil {
		s.Errorf("goto %v jumps into block starting at %v", "<checkGoto.lblName>", "<checkGoto.line#")
	}

}
