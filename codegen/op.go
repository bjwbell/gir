package codegen

import "github.com/bjwbell/ssa"

type OpValue interface {
}

// SSE2 types
type M128i [16]byte
type M128 [4]float32
type M128d [2]float64

func Op2(op ssa.Op, src OpValue, dst OpValue) { panic("unreachable") }
