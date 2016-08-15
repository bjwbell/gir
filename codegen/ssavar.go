package codegen

import (
	"fmt"
	"go/types"

	"github.com/bjwbell/ssa"
)

type ssaVar interface {
	ssaVarType()
	Name() string
	Class() NodeClass
	String() string
	Typ() ssa.Type
	Xoffset() int64
}

type ssaParam struct {
	ssaVar
	v   *types.Var
	ctx Ctx
}

func (p *ssaParam) Name() string {
	return p.v.Name()
}

func (p ssaParam) String() string {
	return fmt.Sprintf("{ssaParam: %v}", p.Name())
}

func (p *ssaParam) Class() NodeClass {
	return PPARAM
}

func (p *ssaParam) Xoffset() int64 {
	// TODO
	return 0
}

func (p ssaParam) Typ() ssa.Type {
	return &Type{p.v.Type()}
}

type ssaRetVar struct {
	ssaVar
	v   *types.Var
	ctx Ctx
}

func (p *ssaRetVar) Name() string {
	name := p.v.Name()
	if name == "" {
		name = "ret0"
	}
	fmt.Println("SSARETVAR.NAME(): ", name)
	return name
}

func (p ssaRetVar) String() string {
	return fmt.Sprintf("{ssaRetVar: %v}", p.Name())
}

func (p *ssaRetVar) Class() NodeClass {
	// return PPARAMOUT
	return PPARAM
}

func (p *ssaRetVar) Xoffset() int64 {
	// TODO
	return 8
}

func (p ssaRetVar) Typ() ssa.Type {
	return &Type{p.v.Type()}
}

type ssaLocal struct {
	ssaVar
	obj types.Object
	ctx Ctx
}

func (local *ssaLocal) Name() string {
	return local.obj.Name()
}

func (local ssaLocal) String() string {
	return fmt.Sprintf("{ssaLocal: %v}", local.Name())
}

func (local *ssaLocal) Class() NodeClass {
	return PAUTO
}

func (local *ssaLocal) Xoffset() int64 {
	// TODO
	return 0
}

func (local ssaLocal) Typ() ssa.Type {
	return &Type{local.obj.Type()}
}
