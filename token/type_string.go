// Code generated by "stringer -type=Type"; DO NOT EDIT

package token

import "fmt"

const _Type_name = "EOFErrorNewlineAssignCharIdentifierNumberOperatorOpRationalLeftParenRightParenLeftBraceRightBraceSemicolonLeftBrackRightBrackStringFUNCPACKAGERET"

var _Type_index = [...]uint8{0, 3, 8, 15, 21, 25, 35, 41, 49, 51, 59, 68, 78, 87, 97, 106, 115, 125, 131, 135, 142, 145}

func (i Type) String() string {
	if i < 0 || i >= Type(len(_Type_index)-1) {
		return fmt.Sprintf("Type(%d)", i)
	}
	return _Type_name[_Type_index[i]:_Type_index[i+1]]
}
