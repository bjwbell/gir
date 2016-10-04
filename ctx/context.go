// Copied from robpike.io/ivy/exec

// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ctx

import (
	"github.com/bjwbell/gir/config"
	"github.com/bjwbell/gir/value"
)

// Context holds execution context, specifically the binding of names to values and operators.
// It is the only implementation of ../value/Context, but since it references the value
// package, there would be a cycle if that package depended on this type definition.
type Context struct {
	// config is the configuration state used for evaluation, printing, etc.
	// Accessed through the value.Context Config method.
	config *config.Config
}

// NewContext returns a new execution context: the stack and variables,
// plus the execution configuration.
func NewContext(conf *config.Config) value.Context {
	c := &Context{
		config: conf,
	}
	return c
}

func (c *Context) Config() *config.Config {
	return c.config
}

// SetConstants re-assigns the fundamental constant values using the current
// setting of floating-point precision.
func (c *Context) SetConstants() {
}

// Lookup returns the value of a symbol.
func (c *Context) Lookup(name string) value.Value {
	return nil
}

// assignLocal binds a value to the name in the current function.
func (c *Context) assignLocal(name string, value value.Value) {
}

// Assign assigns the variable the value. The variable must
// be defined either in the current function or globally.
// Inside a function, new variables become locals.
func (c *Context) Assign(name string, val value.Value) {
}

// push pushes a new frame onto the context stack.
func (c *Context) push() {
}

// pop pops the top frame from the stack.
func (c *Context) pop() {
}

// Eval evaluates a list of expressions.
func (c *Context) Eval(exprs []value.Expr) []value.Value {
	return nil
}

// EvalUnary evaluates a unary operator.
func (c *Context) EvalUnary(op string, right value.Value) value.Value {
	return nil
}

func (c *Context) UserDefined(op string, isBinary bool) bool {
	return false
}

// EvalBinary evaluates a binary operator.
func (c *Context) EvalBinary(left value.Value, op string, right value.Value) value.Value {
	return nil
}

// noVar guarantees that there is no global variable with that name,
// preventing an op from being defined with the same name as a variable,
// which could cause problems. A variable with value zero is considered to
// be OK, so one can clear a variable before defining a symbol. A cleared
// variable is removed from the global symbol table.
// noVar also prevents defining builtin variables as ops.
func (c *Context) noVar(name string) {
}

// noOp is the dual of noVar. It also checks for assignment to builtins.
// It just errors out if there is a conflict.
func (c *Context) noOp(name string) {
}

// Declare makes the name a variable while parsing the next function.
func (c *Context) Declare(name string) {
}

// ForgetAll forgets the declared variables.
func (c *Context) ForgetAll() {
}

func (c *Context) isVariable(op string) bool {
	return false
}
