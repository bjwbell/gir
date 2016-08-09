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

	"github.com/bjwbell/gir/config"
	"github.com/bjwbell/gir/value"
)

func init() {
	//value.IvyEval = IvyEval
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
