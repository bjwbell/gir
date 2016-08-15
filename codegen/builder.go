package codegen

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"github.com/bjwbell/cmd/obj"
	"github.com/bjwbell/ssa"
	"github.com/bjwbell/gir/gst"
	"github.com/bjwbell/gir/gimporter"
)

func TypeCheckFn(fnDecl *gst.FuncDecl, log bool) (function *types.Func, er error) {
	function, ok := gimporter.ParseFuncDecl(fnDecl)
	if !ok {
		fmt.Printf("Error importing %v\n", fnDecl.Name)
		er = fmt.Errorf("Error importing %v\n", fnDecl.Name)
		return
	}
	return
}

// BuildSSA parses the function, fn, which must be in ssa form and returns
// the corresponding ssa.Func
func BuildSSA(fnDecl *gst.FuncDecl, pkgName string, log bool) (ssafn *ssa.Func, usessa bool) {
	function, err := TypeCheckFn(fnDecl, log)
	if err != nil {
	 	return nil, false
	}
	ssafn, ok := buildSSA(fnDecl, function, log)
	return ssafn, ok
}

func getParameters(ctx Ctx, fn *types.Func) []*ssaParam {
	signature := fn.Type().(*types.Signature)
	if signature.Recv() != nil {
		panic("methods unsupported (only functions are supported)")
	}
	var params []*ssaParam
	for i := 0; i < signature.Params().Len(); i++ {
		param := signature.Params().At(i)
		n := ssaParam{v: param, ctx: ctx}
		params = append(params, &n)
	}
	return params
}

func getReturnVar(ctx Ctx, fn *types.Func) []*ssaRetVar {
	signature := fn.Type().(*types.Signature)
	if signature.Recv() != nil {
		panic("methods unsupported (only functions are supported)")
	}
	var results []*ssaRetVar
	for i := 0; i < signature.Results().Len(); i++ {
		ret := signature.Results().At(i)
		n := ssaRetVar{v: ret, ctx: ctx}
		results = append(results, &n)
	}
	return results
}

func linenum(f *token.File, p token.Pos) int32 {
	return int32(f.Line(p))
}

func isParam(ctx Ctx, fn *types.Func, obj types.Object) bool {
	params := getParameters(ctx, fn)
	for _, p := range params {
		if p.v.Id() == obj.Id() {
			return true
		}
	}
	return false
}

func getLocalDecls(ctx Ctx, fnDecl *ast.FuncDecl, fn *types.Func) []*ssaLocal {
	scope := fn.Scope()
	names := scope.Names()
	var locals []*ssaLocal
	for i := 0; i < len(names); i++ {
		name := names[i]
		obj := scope.Lookup(name)
		if isParam(ctx, fn, obj) {
			continue
		}
		node := ssaLocal{obj: obj, ctx: ctx}
		locals = append(locals, &node)
	}
	return locals
}

func getVars(ctx Ctx, fnDecl *ast.FuncDecl, fnType *types.Func) []ssaVar {
	var vars []ssaVar
	params := getParameters(ctx, fnType)
	locals := getLocalDecls(ctx, fnDecl, fnType)
	results := getReturnVar(ctx, fnType)
	for _, p := range params {
		for _, local := range locals {
			for _, ret := range results {
				if p.Name() == local.Name() {
					fmt.Printf("p.Name(): %v, local.Name(): %v\n", p.Name(), local.Name())
					panic("param and local with same name")
				}

				if p.Name() == ret.Name() {
					fmt.Printf("p.Name(): %v, ret.Name(): %v\n", p.Name(), ret.Name())
					panic("param and result value with same name")
				}

				if local.Name() == ret.Name() {
					fmt.Printf("local.Name(): %v, ret.Name(): %v\n", local.Name(), ret.Name())
					panic("local and result value with same name")
				}
			}

		}
	}
	for _, p := range params {
		vars = append(vars, p)
	}

	for _, r := range results {
		vars = append(vars, r)
	}

	for _, local := range locals {
		vars = append(vars, local)
	}
	return vars
}

func buildSSA(fn *gst.FuncDecl, fnType *types.Func, log bool) (ssafn *ssa.Func, ok bool) {

	// HACK, hardcoded
	arch := "amd64"

	signature, ok := fnType.Type().(*types.Signature)
	if signature == nil || !ok {
		return nil, false
	}
	
	if signature.Results().Len() > 1 {
		fmt.Println("Multiple return values not supported")
	}

	var e ssaExport
	var s state
	e.log = log
	link := obj.Link{}
	s.ctx = Ctx{nil, nil} //Ctx{ftok, fnInfo}
	s.fnDecl = nil
	s.fnType = nil
	s.fnInfo = nil
	s.config = ssa.NewConfig(arch, &e, &link, false)
	s.f = s.config.NewFunc()
	s.f.Name = fnType.Name()
	//s.f.Entry = s.f.NewBlock(ssa.BlockPlain)

	//s.scanBlocks(fn.Body)
	if len(s.blocks) < 1 {
		panic("no blocks found, need at least one block per function")
	}

	s.f.Entry = s.blocks[0].b

	s.startBlock(s.f.Entry)

	// Allocate starting values
	s.labels = map[string]*ssaLabel{}
	s.labeledNodes = map[ast.Node]*ssaLabel{}
	s.startmem = s.entryNewValue0(ssa.OpInitMem, ssa.TypeMem)
	s.sp = s.entryNewValue0(ssa.OpSP, Typ[types.Uintptr]) // TODO: use generic pointer type (unsafe.Pointer?) instead
	s.sb = s.entryNewValue0(ssa.OpSB, Typ[types.Uintptr])

	s.vars = map[ssaVar]*ssa.Value{}
	s.vars[&memVar] = s.startmem

	//s.varsyms = map[*Node]interface{}{}

	// Generate addresses of local declarations
	s.decladdrs = map[ssaVar]*ssa.Value{}
	vars := []ssaVar{} //getVars(s.ctx, fn, fnType)
	for _, v := range vars {
		switch v.Class() {
		case PPARAM:
			// aux := s.lookupSymbol(n, &ssa.ArgSymbol{Typ: n.Type, Node: n})
			// s.decladdrs[n] = s.entryNewValue1A(ssa.OpAddr, Ptrto(n.Type), aux, s.sp)
		case PAUTO | PHEAP:
			// TODO this looks wrong for PAUTO|PHEAP, no vardef, but also no definition
			// aux := s.lookupSymbol(n, &ssa.AutoSymbol{Typ: n.Type, Node: n})
			// s.decladdrs[n] = s.entryNewValue1A(ssa.OpAddr, Ptrto(n.Type), aux, s.sp)
		case PPARAM | PHEAP, PPARAMOUT | PHEAP:
		// This ends up wrong, have to do it at the PARAM node instead.
		case PAUTO, PPARAMOUT:
			// processed at each use, to prevent Addr coming
			// before the decl.
		case PFUNC:
			// local function - already handled by frontend
		default:
			str := ""
			if v.Class()&PHEAP != 0 {
				str = ",heap"
			}
			s.Unimplementedf("local variable with class %s%s unimplemented", v.Class(), str)
		}
	}

	//fpVar := types.NewVar(0, fnType.Pkg(), ".fp", Typ[types.Int32].Type)
	fpVar := types.NewVar(0, nil, ".fp", Typ[types.Int32].Type)
	nodfp := &ssaParam{v: fpVar, ctx: s.ctx}

	// nodfp is a special argument which is the function's FP.
	aux := &ssa.ArgSymbol{Typ: Typ[types.Uintptr], Node: nodfp}
	s.decladdrs[nodfp] = s.entryNewValue1A(ssa.OpAddr, Typ[types.Uintptr], aux, s.sp)

	s.processBlocks()

	// Link up variable uses to variable definitions
	s.linkForwardReferences()

	//fmt.Println("f:", f)

	ssa.Compile(s.f)

	return s.f, true
}
