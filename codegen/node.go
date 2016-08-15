package codegen

import (
	"fmt"
	"go/ast"

	"github.com/bjwbell/ssa"
)

type Node struct {
	node  ast.Node
	ctx   Ctx
	Var   ssaVar
	class NodeClass
}

func (n *Node) ssaVarType() {
}

func (n *Node) String() string {
	return fmt.Sprintf("%#v", n)
}

func (n *Node) Typ() ssa.Type {
	switch node := n.node.(type) {
	case *ast.Ident:
		t := &Type{n.ctx.fn.ObjectOf(node).Type()}
		return t
	case ast.Expr:
		t := &Type{n.ctx.fn.TypeOf(node)}
		return t
	default:
		panic("can't get type of node")
	}
}

func (n *Node) Name() string {
	switch node := n.node.(type) {
	case *ast.Ident:
		return node.Name
	}
	panic("unandled case for node.Name")
}

func (n *Node) Class() NodeClass {
	if n.Var != nil {
		return n.Var.Class()
	}
	return n.class
}

func (n *Node) Xoffset() int64 {
	if n.Var != nil {
		return n.Var.Xoffset()
	} else {
		return 8
	}
}

type NodeClass uint8

// declaration context
const (
	Pxxx      NodeClass = iota
	PEXTERN             // global variable
	PAUTO               // local variables
	PPARAM              // input arguments
	PPARAMOUT           // output results
	PPARAMREF           // closure variable reference
	PFUNC               // global function

	PDISCARD // discard during parse of duplicate import

	PHEAP NodeClass = 1 << 7 // an extra bit to identify an escaped variable
)

// Node ops.
type NodeOp uint8

const (
	OXXX NodeOp = iota

	// names
	ONAME    // var, const or func name
	ONONAME  // unnamed arg or return value: f(int, string) (int, error) { etc }
	OTYPE    // type name
	OPACK    // import
	OLITERAL // literal

	// expressions
	OADD             // Left + Right
	OSUB             // Left - Right
	OOR              // Left | Right
	OXOR             // Left ^ Right
	OADDSTR          // Left + Right (string addition)
	OADDR            // &Left
	OANDAND          // Left && Right
	OAPPEND          // append(List)
	OARRAYBYTESTR    // Type(Left) (Type is string, Left is a []byte)
	OARRAYBYTESTRTMP // Type(Left) (Type is string, Left is a []byte, ephemeral)
	OARRAYRUNESTR    // Type(Left) (Type is string, Left is a []rune)
	OSTRARRAYBYTE    // Type(Left) (Type is []byte, Left is a string)
	OSTRARRAYBYTETMP // Type(Left) (Type is []byte, Left is a string, ephemeral)
	OSTRARRAYRUNE    // Type(Left) (Type is []rune, Left is a string)
	OAS              // Left = Right or (if Colas=true) Left := Right
	OAS2             // List = Rlist (x, y, z = a, b, c)
	OAS2FUNC         // List = Rlist (x, y = f())
	OAS2RECV         // List = Rlist (x, ok = <-c)
	OAS2MAPR         // List = Rlist (x, ok = m["foo"])
	OAS2DOTTYPE      // List = Rlist (x, ok = I.(int))
	OASOP            // Left Etype= Right (x += y)
	OASWB            // Left = Right (with write barrier)
	OCALL            // Left(List) (function call, method call or type conversion)
	OCALLFUNC        // Left(List) (function call f(args))
	OCALLMETH        // Left(List) (direct method call x.Method(args))
	OCALLINTER       // Left(List) (interface method call x.Method(args))
	OCALLPART        // Left.Right (method expression x.Method, not called)
	OCAP             // cap(Left)
	OCLOSE           // close(Left)
	OCLOSURE         // func Type { Body } (func literal)
	OCMPIFACE        // Left Etype Right (interface comparison, x == y or x != y)
	OCMPSTR          // Left Etype Right (string comparison, x == y, x < y, etc)
	OCOMPLIT         // Right{List} (composite literal, not yet lowered to specific form)
	OMAPLIT          // Type{List} (composite literal, Type is map)
	OSTRUCTLIT       // Type{List} (composite literal, Type is struct)
	OARRAYLIT        // Type{List} (composite literal, Type is array or slice)
	OPTRLIT          // &Left (left is composite literal)
	OCONV            // Type(Left) (type conversion)
	OCONVIFACE       // Type(Left) (type conversion, to interface)
	OCONVNOP         // Type(Left) (type conversion, no effect)
	OCOPY            // copy(Left, Right)
	ODCL             // var Left (declares Left of type Left.Type)

	// Used during parsing but don't last.
	ODCLFUNC  // func f() or func (r) f()
	ODCLFIELD // struct field, interface field, or func/method argument/return value.
	ODCLCONST // const pi = 3.14
	ODCLTYPE  // type Int int

	ODELETE    // delete(Left, Right)
	ODOT       // Left.Right (Left is of struct type)
	ODOTPTR    // Left.Right (Left is of pointer to struct type)
	ODOTMETH   // Left.Right (Left is non-interface, Right is method name)
	ODOTINTER  // Left.Right (Left is interface, Right is method name)
	OXDOT      // Left.Right (before rewrite to one of the preceding)
	ODOTTYPE   // Left.Right or Left.Type (.Right during parsing, .Type once resolved)
	ODOTTYPE2  // Left.Right or Left.Type (.Right during parsing, .Type once resolved; on rhs of OAS2DOTTYPE)
	OEQ        // Left == Right
	ONE        // Left != Right
	OLT        // Left < Right
	OLE        // Left <= Right
	OGE        // Left >= Right
	OGT        // Left > Right
	OIND       // *Left
	OINDEX     // Left[Right] (index of array or slice)
	OINDEXMAP  // Left[Right] (index of map)
	OKEY       // Left:Right (key:value in struct/array/map literal, or slice index pair)
	OPARAM     // variant of ONAME for on-stack copy of a parameter or return value that escapes.
	OLEN       // len(Left)
	OMAKE      // make(List) (before type checking converts to one of the following)
	OMAKECHAN  // make(Type, Left) (type is chan)
	OMAKEMAP   // make(Type, Left) (type is map)
	OMAKESLICE // make(Type, Left, Right) (type is slice)
	OMUL       // Left * Right
	ODIV       // Left / Right
	OMOD       // Left % Right
	OLSH       // Left << Right
	ORSH       // Left >> Right
	OAND       // Left & Right
	OANDNOT    // Left &^ Right
	ONEW       // new(Left)
	ONOT       // !Left
	OCOM       // ^Left
	OPLUS      // +Left
	OMINUS     // -Left
	OOROR      // Left || Right
	OPANIC     // panic(Left)
	OPRINT     // print(List)
	OPRINTN    // println(List)
	OPAREN     // (Left)
	OSEND      // Left <- Right
	OSLICE     // Left[Right.Left : Right.Right] (Left is untypechecked or slice; Right.Op==OKEY)
	OSLICEARR  // Left[Right.Left : Right.Right] (Left is array)
	OSLICESTR  // Left[Right.Left : Right.Right] (Left is string)
	OSLICE3    // Left[R.Left : R.R.Left : R.R.R] (R=Right; Left is untypedchecked or slice; R.Op and R.R.Op==OKEY)
	OSLICE3ARR // Left[R.Left : R.R.Left : R.R.R] (R=Right; Left is array; R.Op and R.R.Op==OKEY)
	ORECOVER   // recover()
	ORECV      // <-Left
	ORUNESTR   // Type(Left) (Type is string, Left is rune)
	OSELRECV   // Left = <-Right.Left: (appears as .Left of OCASE; Right.Op == ORECV)
	OSELRECV2  // List = <-Right.Left: (apperas as .Left of OCASE; count(List) == 2, Right.Op == ORECV)
	OIOTA      // iota
	OREAL      // real(Left)
	OIMAG      // imag(Left)
	OCOMPLEX   // complex(Left, Right)

	// statements
	OBLOCK    // { List } (block of code)
	OBREAK    // break
	OCASE     // case List: Nbody (select case after processing; List==nil means default)
	OXCASE    // case List: Nbody (select case before processing; List==nil means default)
	OCONTINUE // continue
	ODEFER    // defer Left (Left must be call)
	OEMPTY    // no-op (empty statement)
	OFALL     // fallthrough (after processing)
	OXFALL    // fallthrough (before processing)
	OFOR      // for Ninit; Left; Right { Nbody }
	OGOTO     // goto Left
	OIF       // if Ninit; Left { Nbody } else { Rlist }
	OLABEL    // Left:
	OPROC     // go Left (Left must be call)
	ORANGE    // for List = range Right { Nbody }
	ORETURN   // return List
	OSELECT   // select { List } (List is list of OXCASE or OCASE)
	OSWITCH   // switch Ninit; Left { List } (List is a list of OXCASE or OCASE)
	OTYPESW   // List = Left.(type) (appears as .Left of OSWITCH)

	// types
	OTCHAN   // chan int
	OTMAP    // map[string]int
	OTSTRUCT // struct{}
	OTINTER  // interface{}
	OTFUNC   // func()
	OTARRAY  // []int, [8]int, [N]int or [...]int

	// misc
	ODDD        // func f(args ...int) or f(l...) or var a = [...]int{0, 1, 2}.
	ODDDARG     // func f(args ...int), introduced by escape analysis.
	OINLCALL    // intermediary representation of an inlined call.
	OEFACE      // itable and data words of an empty-interface value.
	OITAB       // itable word of an interface value.
	OSPTR       // base pointer of a slice or string.
	OCLOSUREVAR // variable reference at beginning of closure function
	OCFUNC      // reference to c function pointer (not go func value)
	OCHECKNIL   // emit code to ensure pointer/interface not nil
	OVARKILL    // variable is dead

	// thearch-specific registers
	OREGISTER // a register, such as AX.
	OINDREG   // offset plus indirect of a register, such as 8(SP).

	// arch-specific opcodes
	OCMP    // compare: ACMP.
	ODEC    // decrement: ADEC.
	OINC    // increment: AINC.
	OEXTEND // extend: ACWD/ACDQ/ACQO.
	OHMUL   // high mul: AMUL/AIMUL for unsigned/signed (OMUL uses AIMUL for both).
	OLROT   // left rotate: AROL.
	ORROTC  // right rotate-carry: ARCR.
	ORETJMP // return to other function
	OPS     // compare parity set (for x86 NaN check)
	OPC     // compare parity clear (for x86 NaN check)
	OSQRT   // sqrt(float64), on systems that have hw support
	OGETG   // runtime.getg() (read g pointer)

	OEND
)
