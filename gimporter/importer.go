package gimporter

import (
	"github.com/bjwbell/gir/gst"
	"go/types"
	"go/token"
)

func ParseFuncDecl(fnDecl *gst.FuncDecl) (*types.Func, bool) {
	var fn *types.Func
	var pkg *types.Package
	pkg = nil
	name := fnDecl.Name
	var sig *types.Signature
	sig = types.NewSignature(nil, nil, nil, false)
	var pos token.Pos
	fn = types.NewFunc(pos, pkg, name, sig)
	return fn, true
}
