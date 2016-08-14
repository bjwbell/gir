package ast

import (
	"github.com/bjwbell/gir/value"
)

type Stmt interface {
	stmt()
}

type ExprStmt struct {
	Exprs []value.Expr 
}

func (s *ExprStmt) stmt() {
}

type RetStmt struct {
}

func (ret *RetStmt) stmt() {	
}
