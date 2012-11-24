// based on https://github.com/nsf/gocode/blob/master/decl.go

// Copyright (C) 2010 nsf <no.smile.face@gmail.com>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package main

// decl.class
type decl_class int16

const (
	decl_invalid = decl_class(-1 + iota)

	// these are in a sorted order
	decl_const
	decl_func
	decl_package
	decl_type
	decl_var

	// this one serves as a temporary type for those methods that were
	// declared before their actual owner
	decl_methods_stub
)

func (this decl_class) String() string {
	switch this {
	case decl_invalid:
		return "PANIC"
	case decl_const:
		return "const"
	case decl_func:
		return "func"
	case decl_package:
		return "package"
	case decl_type:
		return "type"
	case decl_var:
		return "var"
	case decl_methods_stub:
		return "IF YOU SEE THIS, REPORT A BUG" // :D
	}
	panic("unreachable")
}
