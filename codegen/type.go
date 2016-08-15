package codegen

import (
	"go/types"

	"github.com/bjwbell/ssa"
)

type Type struct {
	types.Type
}

func StdSizes() types.StdSizes {
	var std types.StdSizes
	// TODO: make dependent on arch
	std.WordSize = 8
	std.MaxAlign = 8
	return std
}

var Typ = []*Type{
	types.Bool:          &Type{types.Typ[types.Bool]},
	types.Int:           &Type{types.Typ[types.Int]},
	types.Int8:          &Type{types.Typ[types.Int8]},
	types.Int16:         &Type{types.Typ[types.Int16]},
	types.Int32:         &Type{types.Typ[types.Int32]},
	types.Int64:         &Type{types.Typ[types.Int64]},
	types.Uint:          &Type{types.Typ[types.Uint]},
	types.Uint8:         &Type{types.Typ[types.Uint8]},
	types.Uint16:        &Type{types.Typ[types.Uint16]},
	types.Uint32:        &Type{types.Typ[types.Uint32]},
	types.Uint64:        &Type{types.Typ[types.Uint64]},
	types.Uintptr:       &Type{types.Typ[types.Uintptr]},
	types.Float32:       &Type{types.Typ[types.Float32]},
	types.Float64:       &Type{types.Typ[types.Float64]},
	types.Complex64:     &Type{types.Typ[types.Complex64]},
	types.Complex128:    &Type{types.Typ[types.Complex128]},
	types.String:        &Type{types.Typ[types.String]},
	types.UnsafePointer: &Type{types.Typ[types.UnsafePointer]},

	// types for untyped values
	// types.UntypedBool:    CTBOOL,
	// types.UntypedInt:     CTINT,
	// types.UntypedRune:    CTRUNE,
	// types.UntypedFloat:   CTFLT,
	// types.UntypedComplex: CTCPLX,
	// types.UntypedString:  CTSTR,
	// types.UntypedNil:     CTNIL
}

// Basic returns *types.Basic if t.Type is *types.Basic
// else nil is returned.
func (t *Type) Basic() *types.Basic {
	if basic, ok := t.Type.(*types.Basic); ok {
		return basic
	}
	return nil
}

// Struct returns *types.Struct if t.Type is *types.Struct
// else nil is returned.
func (t *Type) Struct() *types.Struct {
	if s, ok := t.Type.(*types.Struct); ok {
		return s
	}
	return nil
}

// Array returns *types.Array if t.Type is *types.Array
// else nil is returned.
func (t *Type) Array() *types.Array {
	if array, ok := t.Type.(*types.Array); ok {
		return array
	}
	return nil
}

// IsBasicInfoFlag returns true if t.Type is types.Basic and
// the BasicInfo for t.Type matches flags, otherwise false is returned
func (t *Type) IsBasicInfoFlag(flag types.BasicInfo) bool {
	if basic := t.Basic(); basic != nil {
		info := basic.Info()
		return info&flag == 1
	} else {
		return false
	}
}

func (t *Type) IsBasic() bool {
	return t.Basic() != nil
}

func (t *Type) IsBoolean() bool {
	return t.IsBasicInfoFlag(types.IsBoolean)
}

func (t *Type) IsInteger() bool {
	return t.IsBasicInfoFlag(types.IsInteger)
}

func (t *Type) IsSigned() bool {
	return (t.IsBasic() && !t.IsBasicInfoFlag(types.IsUnsigned))
}

func (t *Type) IsFloat() bool {
	return t.IsBasicInfoFlag(types.IsFloat)
}

func (t *Type) IsComplex() bool {
	return t.IsBasicInfoFlag(types.IsComplex)
}

func (t *Type) IsPtr() bool {
	if basic := t.Basic(); basic != nil {
		return basic.Kind() == types.UnsafePointer
	}
	switch t.Type.(type) {
	case *types.Pointer, *types.Map, *types.Signature, *types.Chan:
		return true
	}
	return false
}

func (t *Type) IsString() bool {
	return t.IsBasicInfoFlag(types.IsString)
}

func (t *Type) IsMap() bool {
	_, ok := t.Type.(*types.Map)
	return ok
}

func (t *Type) IsChan() bool {
	_, ok := t.Type.(*types.Map)
	return ok
}

func (t *Type) IsSlice() bool {
	_, ok := t.Type.(*types.Slice)
	return ok
}

func (t *Type) IsArray() bool {
	_, ok := t.Type.(*types.Array)
	return ok
}

func (t *Type) IsStruct() bool {
	return t.Struct() != nil
}

func (t *Type) IsInterface() bool {
	_, ok := t.Type.(*types.Interface)
	return ok
}

func (t *Type) Size() int64 {
	std := StdSizes()
	return std.Sizeof(t.Type)
}

func (t *Type) Alignment() int64 {
	std := StdSizes()
	return std.Alignof(t.Type)
}

func (t *Type) IsMemory() bool { return false } // special ssa-package-only types
func (t *Type) IsFlags() bool  { return false }
func (t *Type) IsVoid() bool   { return false }

// Elem, if t.Type is []T or *T or [n]T, return T, otherwise return nil
func (t *Type) Elem() ssa.Type {
	if t.IsSlice() || t.IsPtr() || t.IsArray() {
		return &Type{t.Underlying()}
	} else {
		return nil
	}
}

// PtrTo, given T, returns *T
func (t *Type) PtrTo() ssa.Type {
	return &Type{types.NewPointer(t.Type)}
}

// NumFields returns the # of fields of a struct, panics if t is not a types.Struct
func (t *Type) NumFields() int {
	if !t.IsStruct() {
		panic("NumFields can only be called with Struct's")
	}
	s := t.Type.(*types.Struct)
	return s.NumFields()
}

// FieldTypes returns the type of ith field of the struct and panics on error
func (t *Type) FieldType(i int) ssa.Type {
	if s := t.Struct(); s == nil {
		panic("FieldType can only be called with Struct's")
	} else {
		if s.NumFields() <= i {
			panic("Invalid field #")
		}
		field := Type{s.Field(int(i)).Type()}
		return &field
	}
}

// FieldOff returns the offset of ith field of the struct and panics on error
func (t *Type) FieldOff(i int) int64 {
	if s := t.Struct(); s == nil {
		panic("FieldOff can only be called with Struct's")
	} else {
		if s.NumFields() <= i {
			panic("Invalid field #")
		}
		std := StdSizes()
		field := s.Field(int(i))
		offsets := std.Offsetsof([]*types.Var{field})
		return offsets[0]
	}
}

// NumElem returns the # of elements of an array and panics on error
func (t *Type) NumElem() int64 {
	if array := t.Array(); array == nil {
		panic("NumElem can only be called with types.Array")
	} else {
		return array.Len()
	}
}

func (t *Type) String() string {
	return t.Type.String()
}

// SimpleString is a coarser generic description of T, e.g. T's underlying type
func (t *Type) SimpleString() string {
	return t.Type.Underlying().String()
}

func (t *Type) Equal(v ssa.Type) bool {
	if v2, ok := v.(*Type); ok {
		return types.Identical(t, v2)
	}
	return false
}

// Bound returns the num elements if t is an array, if t is a slice it returns -1,
// and if t is neither an array or slice it panics
func (t *Type) Bound() int64 {
	if t.Array() != nil {
		return t.NumElem()
	} else if t.IsSlice() {
		return -1
	} else {
		panic("Bound called with invalid type")
	}
}

func (t *Type) Width() int64 {
	return t.Size()
}

// compare types, returning one of CMPlt, CMPeq, CMPgt.
func (t *Type) Compare(t2 ssa.Type) ssa.Cmp {
	if t.Equal(t2) {
		return ssa.CMPeq
	}
	// TODO
	return ssa.CMPlt
}

// given []T or *T or [n]T, return T
func (t *Type) ElemType() ssa.Type {
	// TODO
	return nil
}

// name of ith field of the struct
func (t *Type) FieldName(i int) string {
	// TODO
	return "<ssair.Type.FieldName>"
}


func (t *Type) IsPtrShaped() bool {
	// TODO
	return false
}
