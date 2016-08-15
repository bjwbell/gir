package codegen

import (
	"go/types"
	"strings"
)

func GoProto(fn *types.Func) (string, string, string) {
	pkgname := "package " + fn.Pkg().Name() + "\n"
	imports := ""
	signature := fn.Type().(*types.Signature)
	sig := strings.TrimPrefix(signature.String(), "func(")
	fnproto := "func " + fn.Name() + "(" + sig + "\n"
	return pkgname, imports, fnproto
}
