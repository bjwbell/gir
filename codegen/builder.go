package codegen

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"

	"github.com/bjwbell/cmd/obj"
	"github.com/bjwbell/ssa"
)

func TypeCheckFn(file, pkgName, fn string, log bool) (fileTok *token.File, fileAst *ast.File, fnDecl *ast.FuncDecl, function *types.Func, info *types.Info, er error) {
	var conf types.Config
	conf.Importer = importer.Default()
	fset := token.NewFileSet()
	fileAst, err := parser.ParseFile(fset, file, nil, parser.AllErrors)
	var terrors string
	if err != nil {
		fmt.Printf("Error parsing %v, error message: %v\n", file, err)
		terrors += fmt.Sprintf("err: %v\n", err)
		er = err
		return
	}
	fileTok = fset.File(fileAst.Pos())
	files := []*ast.File{fileAst}
	info = &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
	}
	pkg, err := conf.Check(pkgName, fset, files, info)
	if err != nil {
		if terrors != fmt.Sprintf("err: %v\n", err) {
			fmt.Printf("Type error (%v) message: %v\n", file, err)
			er = err
			return
		}
	}
	fmt.Println("pkg: ", pkg)
	fmt.Println("pkg.Complete:", pkg.Complete())
	scope := pkg.Scope()
	obj := scope.Lookup(fn)
	if obj == nil {
		fmt.Println("Couldnt lookup function: ", fn)
		er = fmt.Errorf("Couldnt lookup function: %v", fn)
		return
	}
	function, ok := obj.(*types.Func)
	if !ok {
		fmt.Printf("%v is a %v, not a function\n", fn, obj.Type().String())
		er = fmt.Errorf("%v is a %v, not a function\n", fn, obj.Type().String())
		return
	}
	for _, decl := range fileAst.Decls {
		if fdecl, ok := decl.(*ast.FuncDecl); ok {
			if fdecl.Name.Name == fn {
				fnDecl = fdecl
				break
			}
		}
	}
	if fnDecl == nil {
		fmt.Println("couldn't find function: ", fn)
		er = fmt.Errorf("couldn't find function: %v", fn)
		return
	}
	return
}

// BuildSSA parses the function, fn, which must be in ssa form and returns
// the corresponding ssa.Func
func BuildSSA(file, pkgName, fn string, log bool) (ssafn *ssa.Func, usessa bool) {
	fileTok, fileAst, fnDecl, function, info, err := TypeCheckFn(file, pkgName, fn, log)
	if err != nil {
		return nil, false
	}
	ssafn, ok := buildSSA(fileTok, fileAst, fnDecl, function, info, log)
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

func buildSSA(ftok *token.File, f *ast.File, fn *ast.FuncDecl, fnType *types.Func, fnInfo *types.Info, log bool) (ssafn *ssa.Func, ok bool) {

	// HACK, hardcoded
	arch := "amd64"

	signature, ok := fnType.Type().(*types.Signature)
	if !ok {
		panic("function type is not types.Signature")
	}
	if signature.Recv() != nil {
		fmt.Println("Methods not supported")
		return nil, false
	}
	if signature.Results().Len() > 1 {
		fmt.Println("Multiple return values not supported")
	}

	var e ssaExport
	var s state
	e.log = log
	link := obj.Link{}
	s.ctx = Ctx{ftok, fnInfo}
	s.fnDecl = fn
	s.fnType = fnType
	s.fnInfo = fnInfo
	s.config = ssa.NewConfig(arch, &e, &link, false)
	s.f = s.config.NewFunc()
	s.f.Name = fnType.Name()
	//s.f.Entry = s.f.NewBlock(ssa.BlockPlain)

	s.scanBlocks(fn.Body)
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
	vars := getVars(s.ctx, fn, fnType)
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

	fpVar := types.NewVar(0, fnType.Pkg(), ".fp", Typ[types.Int32].Type)
	nodfp := &ssaParam{v: fpVar, ctx: s.ctx}

	// nodfp is a special argument which is the function's FP.
	aux := &ssa.ArgSymbol{Typ: Typ[types.Uintptr], Node: nodfp}
	s.decladdrs[nodfp] = s.entryNewValue1A(ssa.OpAddr, Typ[types.Uintptr], aux, s.sp)

	s.processBlocks()

	// Link up variable uses to variable definitions
	s.linkForwardReferences()

	fmt.Println("f:", f)

	ssa.Compile(s.f)

	return s.f, true
}
