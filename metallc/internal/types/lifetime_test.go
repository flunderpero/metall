package types

import (
	"strings"
	"testing"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/token"
)

func TestLifetimeAnalyzer(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"stack alloc escapes block", `let foo = { let bar = 123 &bar }`, []string{
			"test.met:1:27: reference escaping its allocation scope\n" +
				`    let foo = { let bar = 123 &bar }` + "\n" +
				"                              ^^^^",
		}},
		{"assign ref to outer", `{ mut a = 123 mut b = &a { mut c = 123 b = &c } }`, []string{
			"test.met:1:40: reference escaping its allocation scope\n" +
				`    { mut a = 123 mut b = &a { mut c = 123 b = &c } }` + "\n" +
				"                                           ^^^^^^",
		}},
		{"nested", `
			{
				mut a = 123
				mut b = &a
				{
					mut c = 123
					{
						b = &c
					}
			    }
			}
			`, []string{
			"test.met:8:25: reference escaping its allocation scope\n" +
				strings.Trim(`
					    {
						    b = &c
						    ^^^^^^
					    }
					`, "\n"),
		}},
		{"deref assign", `
			{
				mut a = 123
				mut y = &a
				mut z = &y
				{
				  mut c = 456
				  *z = &c
				}
			}
			`, []string{
			"test.met:8:19: reference escaping its allocation scope\n" +
				strings.Trim(`
				      mut c = 456
				      *z = &c
				      ^^^^^^^
				    }
				`, "\n"),
		}},
		// Deref assign through multiple nested scopes - z is in outermost,
		// y is in middle, c is in innermost. *y = &c should error.
		{"deref assign multi-level", `
			{
				mut a = 123
				mut z = &a
				{
					mut b = 456
					mut y = &z
					{
						mut c = 789
						*y = &c
					}
				}
			}
			`, []string{
			"test.met:10:25: reference escaping its allocation scope\n" +
				strings.Trim(`
					        mut c = 789
					        *y = &c
					        ^^^^^^^
					    }
					`, "\n"),
		}},
		// Valid: ref doesn't escape - assigning ref from same or outer scope
		{"valid same scope ref", `
			{
				mut a = 123
				mut b = &a
				mut c = 456
				b = &c
			}
			`, []string{}},
		// Valid: ref from outer scope assigned to inner variable
		{"valid outer ref to inner var", `
			{
				mut a = 123
				{
					mut b = &a
					b
				}
			}
			`, []string{}},
		// Chain of refs: x -> y (ref to x) -> z (ref to y)
		// Assigning through *z should affect x, and if we assign &c where c is local, it should error
		{"ref chain deref", `
			{
				mut a = 123
				mut x = &a
				mut y = &x
				{
					mut c = 456
					*y = &c
				}
			}
			`, []string{
			"test.met:8:21: reference escaping its allocation scope\n" +
				strings.Trim(`
					    mut c = 456
					    *y = &c
					    ^^^^^^^
					}
					`, "\n"),
		}},
		// Field write: assigning a local ref to a struct field that escapes the block.
		{"field write escapes", `
			{
				struct Foo { mut ptr &Int }
				mut a = 123
				mut foo = Foo(&a)
				{
					mut c = 456
					foo.ptr = &c
				}
			}
			`, []string{
			"test.met:8:21: reference escaping its allocation scope\n" +
				strings.Trim(`
					    mut c = 456
					    foo.ptr = &c
					    ^^^^^^^^^^^^
					}
					`, "\n"),
		}},
		// Field write: ref stays in same scope, no escape.
		{"valid field write same scope", `
			{
				struct Foo { mut ptr &Int }
				mut a = 123
				mut b = 456
				mut foo = Foo(&a)
				foo.ptr = &b
			}
			`, []string{}},
		// Function returning a ref to a local variable.
		{"return ref to local from function", `
			{
				fun bad() &Int {
					mut x = 42
					&x
				}
			}
			`, []string{
			"test.met:5:21: reference escaping its allocation scope\n" +
				strings.Trim(`
				        mut x = 42
				        &x
				        ^^
				    }
				`, "\n"),
		}},
		// Deref on RHS: b = *x where x points to a ref to local c
		// The ref that *x evaluates to should not escape
		{"deref rhs escapes", `
			{
				mut b = 0
				mut bRef = &b
				{
					mut c = 456
					mut cRef = &c
					mut x = &cRef
					bRef = *x
				}
			}
			`, []string{
			"test.met:9:21: reference escaping its allocation scope\n" +
				strings.Trim(`
					    mut x = &cRef
					    bRef = *x
					    ^^^^^^^^^
					}
					`, "\n"),
		}},
		// Ref escapes through a function call - x's ref is returned by
		// identity and escapes the block.
		{"call return ref escape", `
			{
				fun identity(a &Int) &Int { a }
				let r = {
					let x = 42
					identity(&x)
				}
				r
			}
			`, []string{
			"test.met:6:30: reference escaping its allocation scope\n" +
				strings.Trim(`
				        let x = 42
				        identity(&x)
				                 ^^
				    }
				`, "\n"),
		}},
		// Transitive: function calls another function, propagating ref through.
		{"call transitive ref escape", `
			{
				fun identity(a &Int) &Int { a }
				fun wrapper(a &Int) &Int { identity(a) }
				let r = {
					let x = 42
					wrapper(&x)
				}
				r
			}
			`, []string{
			"test.met:7:29: reference escaping its allocation scope\n" +
				strings.Trim(`
				        let x = 42
				        wrapper(&x)
				                ^^
				    }
				`, "\n"),
		}},
		// Valid: transitive call where the ref does not escape.
		{"valid call transitive no escape", `
			{
				fun identity(a &Int) &Int { a }
				fun wrapper(a &Int) &Int { identity(a) }
				let x = 42
				let r = wrapper(&x)
				r
			}
			`, []string{}},
		// Function returns a struct by value with a ref field set to a param.
		{"call return struct with ref field", `
			{
				struct Wrapper { ptr &Int }
				fun wrap(a &Int) Wrapper { Wrapper(a) }
				let w = {
					let x = 42
					wrap(&x)
				}
				w
			}
			`, []string{
			"test.met:7:26: reference escaping its allocation scope\n" +
				strings.Trim(`
				        let x = 42
				        wrap(&x)
				             ^^
				    }
				`, "\n"),
		}},
		// Nested structs: Outer contains Inner which has a ref field.
		{"nested struct ref field escape", `
			{
				struct Inner { ptr &Int }
				struct Outer { inner Inner }
				let o = {
					let x = 42
					Outer(Inner(&x))
				}
				o
			}
			`, []string{
			"test.met:7:33: reference escaping its allocation scope\n" +
				strings.Trim(`
				        let x = 42
				        Outer(Inner(&x))
				                    ^^
				    }
				`, "\n"),
		}},
		// Field read: reading a ref field from a struct should propagate taints.
		{"field read ref escape", `
			{
				struct Wrapper { ptr &Int }
				let r = {
					let x = 42
					let w = Wrapper(&x)
					w.ptr
				}
				r
			}
			`, []string{
			"test.met:6:37: reference escaping its allocation scope\n" +
				strings.Trim(`
				        let x = 42
				        let w = Wrapper(&x)
				                        ^^
				        w.ptr
				`, "\n"),
		}},
		// Field read through a ref to a struct with a ref field.
		{"field read through ref escape", `
			{
				struct Wrapper { ptr &Int }
				let r = {
					let x = 42
					let w = Wrapper(&x)
					let rw = &w
					rw.ptr
				}
				r
			}
			`, []string{
			"test.met:7:30: reference escaping its allocation scope\n" +
				strings.Trim(`
				        let w = Wrapper(&x)
				        let rw = &w
				                 ^^
				        rw.ptr
				`, "\n"),
		}},
		// Nested field access through a ref: r.inner.ptr where r is &Outer.
		{"nested field read through ref escape", `
			{
				struct Inner { ptr &Int }
				struct Outer { inner Inner }
				let r = {
					let x = 42
					let o = Outer(Inner(&x))
					let ro = &o
					ro.inner.ptr
				}
				r
			}
			`, []string{
			"test.met:8:30: reference escaping its allocation scope\n" +
				strings.Trim(`
				        let o = Outer(Inner(&x))
				        let ro = &o
				                 ^^
				        ro.inner.ptr
				`, "\n"),
		}},
		// If/else: ref to a local escapes through one branch.
		{"if ref escape", `
			{
				let a = 1
				let r = {
					let x = 42
					if true { &x } else { &a }
				}
				r
			}
			`, []string{
			"test.met:6:31: reference escaping its allocation scope\n" +
				strings.Trim(`
				        let x = 42
				        if true { &x } else { &a }
				                  ^^
				    }
				`, "\n"),
		}},
	}

	assert := base.NewAssert(t)
	hasOnly := false
	for _, tt := range tests {
		if strings.HasPrefix(tt.name, "!"+"only") {
			hasOnly = true
			break
		}
	}
	for _, tt := range tests {
		if hasOnly && !strings.HasPrefix(tt.name, "!"+"only") {
			continue
		}
		t.Run(tt.name, func(t *testing.T) {
			source := base.NewSource("test.met", []rune(strings.ReplaceAll(tt.src, "\t", "    ")))
			tokens := token.Lex(source)
			parser := ast.NewParser(tokens)
			exprID, parseOK := parser.ParseExpr()
			assert.Equal(0, len(parser.Diagnostics), "parsing failed:\n%s", parser.Diagnostics)
			assert.Equal(true, parseOK)
			e := NewEngine(parser.AST)
			e.Query(exprID)
			assert.Equal(0, len(e.Diagnostics), "type check failed: %s", e.Diagnostics)
			a := NewLifetimeAnalyzer(e)
			a.Debug = base.NewStdoutDebug("lifetime")
			a.Check(exprID)
			for i, want := range tt.want {
				if i >= len(a.Diagnostics) {
					t.Fatalf("no more diagnostics, but wanted: %s", want)
				}
				want = strings.Trim(strings.ReplaceAll(want, "\t", "    "), "\n")
				want = strings.TrimRight(want, " ")
				assert.Equal(want, a.Diagnostics[i].Display())
			}
			if len(e.Diagnostics) > len(tt.want) {
				t.Fatalf("there are more diagnostics than expected: %s", a.Diagnostics[len(tt.want):])
			}
		})
	}
}
