// Copied from robpike.io/ivy/value/value.go
package value

import (
	"fmt"

	"github.com/bjwbell/gir/config"
)

type Value interface {
	// String is for internal debugging only. It uses default configuration
	// and puts parentheses around every value so it's clear when it is used.
	// All user output should call Sprint instead.
	String() string
	Sprint(*config.Config) string
	Eval(Context) Value

	// ProgString is like String, but suitable for program listing.
	// For instance, it ignores the user format for numbers and
	// puts quotes on chars, guaranteeing a correct representation.
	ProgString() string
}

// Error is the type we recognize as a recoverable run-time error.
type Error string

// Errorf panics with the formatted string, with type Error.
func Errorf(format string, args ...interface{}) {
	panic(Error(fmt.Sprintf(format, args...)))
}
