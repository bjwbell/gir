// Copied from "robpike.io/ivy/run"

// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package run provides the execution control for ivy.
// It is factored out of main so it can be used for tests.
// This layout also helps out ivy/mobile.
package run // import "github.com/bjwbell/gir/run"

import (
	"fmt"
	"io"
	"strings"

	"github.com/bjwbell/gir/config"
	"github.com/bjwbell/gir/parse"
	"github.com/bjwbell/gir/scan"
	"github.com/bjwbell/gir/value"
)

func init() {
	//value.IvyEval = IvyEval
}

// IvyEval is the function called by value/unaryIvy to implement the ivy (eval) operation.
// It is exported but is not intended to be used outside of ivy.
func IvyEval(context value.Context, str string) value.Value {
	scanner := scan.New(context, "<ivy>", strings.NewReader(str))
	parser := parse.NewParser("<ivy>", scanner, context)
	return eval(parser, context)
}

// Run runs the parser/evaluator until EOF or error.
// The return value says whether we completed without error. If the return
// value is true, it means we ran out of data (EOF) and the run was successful.
// Typical execution is therefore to loop calling Run until it succeeds.
// Error details are reported to the configured error output stream.
func Run(p *parse.Parser, context value.Context, interactive bool) (success bool) {
	conf := context.Config()
	writer := conf.Output()
	defer func() {
		if conf.Debug("panic") {
			return
		}
		err := recover()
		if err == nil {
			return
		}
		p.FlushToNewline()
		if err, ok := err.(value.Error); ok {
			fmt.Fprintf(conf.ErrOutput(), "%s%s\n", p.Loc(), err)
			if interactive {
				fmt.Fprintln(writer)
			}
			success = false
			return
		}
		panic(err)
	}()
	for {
		if interactive {
			fmt.Fprint(writer, conf.Prompt())
		}
		exprs, ok := p.Line()
		var values []value.Value
		if exprs != nil {
			values = context.Eval(exprs)
		}
		if values != nil {
			printValues(conf, writer, values)
			context.Assign("_", values[len(values)-1])
		}
		if !ok {
			return true
		}
		if interactive {
			fmt.Fprintln(writer)
		}
	}
}

// eval runs until EOF or error. It prints every value but the last, and returns the last.
// By last we mean the last expression of the last evaluation.
// (Expressions are separated by ; in the input.)
// It is always called from (somewhere below) run, so if it errors out the recover in
// run will catch it.
func eval(p *parse.Parser, context value.Context) value.Value {
	conf := context.Config()
	writer := conf.Output()
	var prevValues []value.Value
	for {
		exprs, ok := p.Line()
		var values []value.Value
		if exprs != nil {
			values = context.Eval(exprs)
		}
		if !ok {
			if len(prevValues) == 0 {
				return nil
			}
			printValues(conf, writer, prevValues[:len(prevValues)-1])
			return prevValues[len(prevValues)-1]
		}
		printValues(conf, writer, prevValues)
		prevValues = values
	}
}

// printValues neatly prints the values returned from execution, followed by a newline.
// It also handles the ')debug types' output.
func printValues(conf *config.Config, writer io.Writer, values []value.Value) {
	if len(values) == 0 {
		return
	}
	if conf.Debug("types") {
		for i, v := range values {
			if i > 0 {
				fmt.Fprint(writer, ",")
			}
			fmt.Fprintf(writer, "%T", v)
		}
		fmt.Fprintln(writer)
	}
	for i, v := range values {
		s := v.Sprint(conf)
		if i > 0 && len(s) > 0 && s[len(s)-1] != '\n' {
			fmt.Fprint(writer, " ")
		}
		fmt.Fprint(writer, s)
	}
	fmt.Fprintln(writer)
}
