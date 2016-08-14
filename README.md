# gir
Go Intermediate Representation

[![Build Status](http://travis-ci.org/bjwbell/gir.svg?branch=master)](https://travis-ci.org/bjwbell/gir)

# format
The GIR format borrows from the LLVM language reference, http://llvm.org/docs/LangRef.html.

Example, multiple an integer by 8.
```
%result = mul i32 %X, 8
```
Definition of a function:
```
define i32 main() {
  ret i32 0
}
```

## keywords
1. define
2. ret
