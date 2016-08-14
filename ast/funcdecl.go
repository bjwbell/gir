package ast

import (
	"github.com/bjwbell/gir/value"
)

type FuncDecl struct {
	Name string

	Epxrs []value.Expr 
}
