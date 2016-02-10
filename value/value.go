// Copied from robpike.io/ivy/value/value.go
package value

type Value interface {
	// String is for internal debugging only. It uses default configuration
	// and puts parentheses around every value so it's clear when it is used.
	// All user output should call Sprint instead.
	String() string
	Sprint() string
	Eval(Context) Value

	// ProgString is like String, but suitable for program listing.
	// For instance, it ignores the user format for numbers and
	// puts quotes on chars, guaranteeing a correct representation.
	ProgString() string
}
