// COPIED FROM robpike.io/ivy/value

// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import (
	"fmt"

	"github.com/bjwbell/gir/config"
)

type Char rune

const (
	sQuote = '\''
	dQuote = "\""
)

func (c Char) String() string {
	return "(" + string(c) + ")"
}

func (c Char) Sprint(conf *config.Config) string {
	// We ignore the format - chars are always textual.
	// TODO: What about escapes?
	return string(c)
}

func (c Char) ProgString() string {
	return fmt.Sprintf("%q", rune(c))
}

func (c Char) Eval(Context) Value {
	return c
}
