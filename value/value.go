// COPIED FROM robpike.io/ivy/value/value.go
package value

import (
	"fmt"
	"strings"

	"github.com/bjwbell/gir/config"
)

var debugConf = &config.Config{} // For debugging, e.g. to call a String method

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


func (err Error) Error() string {
	return string(err)
}

// Errorf panics with the formatted string, with type Error.
func Errorf(format string, args ...interface{}) {
	panic(Error(fmt.Sprintf(format, args...)))
}

func Parse(conf *config.Config, s string) (Value, error) {
	// Is it a rational? If so, it's TODO
	if strings.ContainsRune(s, '/') {
		panic("rationals unsupported")
	}
	// Not a rational, but might be something like 1.3e-2 and therefore
	// become a rational.
	i, err := setIntString(conf, s)
	if err == nil {
		return i, nil
	}
	// TODO
	return nil, err
}
