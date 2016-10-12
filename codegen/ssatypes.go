package codegen

import (
	"fmt"
	"go/types"

	"github.com/bjwbell/ssa"
)

type ssaLabel struct {
	target  *ssa.Block // block identified by this label
	defNode *Node      // label definition Node
	name    string
	// Label use Node (OGOTO, OBREAK, OCONTINUE).
	// Used only for error detection and reporting.
	// There might be multiple uses, but we only need to track one.
	useNode  *Node
	reported bool // reported indicates whether an error has already been reported for this label
}

//func (l *ssaLabel) name() string { return l.defNode != nil }

// defined reports whether the label has a definition (OLABEL node).
func (l *ssaLabel) defined() bool { return l.defNode != nil }

// used reports whether the label has a use (OGOTO, OBREAK, or OCONTINUE node).
func (l *ssaLabel) used() bool { return l.useNode != nil }

// ssaExport exports a bunch of compiler services for the ssa backend.
type ssaExport struct {
	log bool
}

func (s *ssaExport) TypeBool() ssa.Type    { return Typ[types.Bool] }
func (s *ssaExport) TypeInt8() ssa.Type    { return Typ[types.Int8] }
func (s *ssaExport) TypeInt16() ssa.Type   { return Typ[types.Int16] }
func (s *ssaExport) TypeInt32() ssa.Type   { return Typ[types.Int32] }
func (s *ssaExport) TypeInt64() ssa.Type   { return Typ[types.Int64] }
func (s *ssaExport) TypeUInt8() ssa.Type   { return Typ[types.Uint8] }
func (s *ssaExport) TypeUInt16() ssa.Type  { return Typ[types.Uint16] }
func (s *ssaExport) TypeUInt32() ssa.Type  { return Typ[types.Uint32] }
func (s *ssaExport) TypeUInt64() ssa.Type  { return Typ[types.Uint64] }
func (s *ssaExport) TypeFloat32() ssa.Type { return Typ[types.Float32] }
func (s *ssaExport) TypeFloat64() ssa.Type { return Typ[types.Float64] }
func (s *ssaExport) TypeInt() ssa.Type     { return Typ[types.Int] }
func (s *ssaExport) TypeUintptr() ssa.Type { return Typ[types.Uintptr] }
func (s *ssaExport) TypeString() ssa.Type  { return Typ[types.String] }
func (s *ssaExport) TypeBytePtr() ssa.Type { return Typ[types.Uint8].PtrTo() }

// StringData returns a symbol (a *Sym wrapped in an interface) which
// is the data component of a global string constant containing s.
func (*ssaExport) StringData(s string) interface{} {
	// TODO
	return nil
}

func (e *ssaExport) Auto(t ssa.Type) ssa.GCNode {
	/*n := temp(t.(*Type))   // Note: adds new auto to Curfn.Func.Dcl list
	e.mustImplement = true // This modifies the input to SSA, so we want to make sure we succeed from here!*/
	//return n
	return nil
}

func (e *ssaExport) CanSSA(t ssa.Type) bool {
	return true //canSSAType(t.(*Type))
}

// Log logs a message from the compiler.
func (e *ssaExport) Logf(msg string, args ...interface{}) {
	// If e was marked as unimplemented, anything could happen. Ignore.
	if e.log {
		fmt.Printf(msg, args...)
	}
}

func Fatalf(format string, args ...interface{}) {
	msg := "internal compiler error: " + format
	fmt.Printf(msg, args)
	fmt.Printf("\n")
	panic("")
}

// Fatal reports a compiler error and exits.
func (e *ssaExport) Fatalf(line int32, msg string, args ...interface{}) {
	Fatalf(msg, args...)
}

// Unimplemented reports that the function cannot be compiled.
// It will be removed once SSA work is complete.
func (e *ssaExport) Unimplementedf(line int32, msg string, args ...interface{}) {
	Fatalf(msg, args...)
}

// Warnl reports a "warning", which is usually flag-triggered
// logging output for the benefit of tests.
func (e *ssaExport) Warnl(line int32, fmt_ string, args ...interface{}) {
	panic("Warnl")
	//Warnl(line, fmt_, args...)
}

func (e *ssaExport) Debug_checknil() bool {
	return false
}

func (e *ssaExport) Line(l int32) string {
	return "<ssaExport.Line>"
}

// Log returns true if logging is not a no-op
// some logging calls account for more than a few heap allocations.
func (e *ssaExport) Log() bool {
	return true
}

// A LocalSlot is a location in the stack frame.
// It is (possibly a subpiece of) a PPARAM, PPARAMOUT, or PAUTO ONAME node.

func (e *ssaExport) SplitString(localSlot ssa.LocalSlot) (ssa.LocalSlot, ssa.LocalSlot) {
	// TODO
	return ssa.LocalSlot{}, ssa.LocalSlot{}
}

func (e *ssaExport) SplitInterface(localSlot ssa.LocalSlot) (ssa.LocalSlot, ssa.LocalSlot) {
	// TODO
	return ssa.LocalSlot{}, ssa.LocalSlot{}
}

func (e *ssaExport) SplitSlice(localSlot ssa.LocalSlot) (ssa.LocalSlot, ssa.LocalSlot, ssa.LocalSlot) {
	// TODO
	return ssa.LocalSlot{}, ssa.LocalSlot{}, ssa.LocalSlot{}
}

func (e *ssaExport) SplitComplex(localSlot ssa.LocalSlot) (ssa.LocalSlot, ssa.LocalSlot) {
	// TODO
	return ssa.LocalSlot{}, ssa.LocalSlot{}
}

func (e *ssaExport) SplitStruct(localSlot ssa.LocalSlot, i int) ssa.LocalSlot {
	// TODO
	return ssa.LocalSlot{}
}

// returns (hi, lo)
func (e *ssaExport) SplitInt64(localslot ssa.LocalSlot) (ssa.LocalSlot, ssa.LocalSlot) {
	// TODO
	return ssa.LocalSlot{}, ssa.LocalSlot{}
}

// AllocFrame assigns frame offsets to all live auto variables.
func (e *ssaExport) AllocFrame(f *ssa.Func) {
	// TODO
}
