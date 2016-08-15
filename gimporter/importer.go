package gimporter

import (
	"github.com/bjwbell/gir/gst"
	"github.com/bjwbell/gir/gtypes"
)

func ParseFuncDecl(fnDecl *gst.FuncDecl) (*gtypes.Func, bool) {
	var fn gtypes.Func
	fn.Pkg = nil
	fn.Name = fnDecl.Name
	fn.Typ = &gtypes.Signature{}
	return &fn, true
}
