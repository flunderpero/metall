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
		{"stack ref escapes", `let x = { let y = 123 &y }`, []string{
			"test.met:1:23: reference escaping its allocation scope (via block result)\n" +
				`    let x = { let y = 123 &y }` + "\n" +
				"                          ^^",
		}},
		{"assign ref to outer", `{ mut x = 123 mut y = &x { mut z = 123 y = &z } }`, []string{
			"test.met:1:44: reference escaping its allocation scope (via mutation of outer variable)\n" +
				`    { mut x = 123 mut y = &x { mut z = 123 y = &z } }` + "\n" +
				"                                               ^^",
		}},
		{"nested block escape", `
			{
				mut x = 123
				mut y = &x
				{
					mut z = 123
					{
						y = &z
					}
			    }
			}
			`, []string{
			"test.met:8:29: reference escaping its allocation scope (via mutation of outer variable)\n" +
				strings.Trim(`
					    {
						    y = &z
						        ^^
					    }
					`, "\n"),
		}},
		{"deref assign escapes", `
			{
				mut x = 123
				mut y = &x
				mut z = &mut y
				{
				  mut w = 456
				  z.* = &w
				}
			}
			`, []string{
			"test.met:8:25: reference escaping its allocation scope (via deref assignment)\n" +
				strings.Trim(`
				      mut w = 456
				      z.* = &w
				            ^^
				    }
				`, "\n"),
		}},
		// Deref assign through multiple nested scopes.
		{"deref assign multi-level escape", `
			{
				mut x = 123
				mut y = &x
				{
					mut z = 456
					mut w = &mut y
					{
						mut v = 789
						w.* = &v
					}
				}
			}
			`, []string{
			"test.met:10:31: reference escaping its allocation scope (via deref assignment)\n" +
				strings.Trim(`
					        mut v = 789
					        w.* = &v
					              ^^
					    }
					`, "\n"),
		}},
		{"valid same scope ref", `
			{
				mut x = 123
				mut y = &x
				mut z = 456
				y = &z
			}
			`, []string{}},
		{"valid outer ref to inner", `
			{
				mut x = 123
				let r = {
					mut y = &x
					y
				}
			}
			`, []string{}},
		// Chain of refs: x -> y -> z. Deref assign through z escapes.
		{"deref assign through ref chain escapes", `
			{
				mut x = 123
				mut y = &x
				mut z = &mut y
				{
					mut w = 456
					z.* = &w
				}
			}
			`, []string{
			"test.met:8:27: reference escaping its allocation scope (via deref assignment)\n" +
				strings.Trim(`
					    mut w = 456
					    z.* = &w
					          ^^
					}
					`, "\n"),
		}},
		{"field write escapes", `
			{
				struct Foo { mut one &Int }
				mut x = 123
				mut y = Foo(&x)
				{
					mut z = 456
					y.one = &z
				}
			}
			`, []string{
			"test.met:8:29: reference escaping its allocation scope (via mutation of outer variable)\n" +
				strings.Trim(`
					    mut z = 456
					    y.one = &z
					            ^^
					}
					`, "\n"),
		}},
		{"valid field write", `
			{
				struct Foo { mut one &Int }
				mut x = 123
				mut y = 456
				mut z = Foo(&x)
				z.one = &y
			}
			`, []string{}},
		{"return ref to local", `
			{
				fun foo() &Int {
					mut x = 42
					&x
				}
			}
			`, []string{
			"test.met:5:21: reference escaping its allocation scope (via block result)\n" +
				strings.Trim(`
				        mut x = 42
				        &x
				        ^^
				    }
				`, "\n"),
		}},
		{"return expr ref to local", `
			{
				fun foo() &Int {
					mut x = 42
					return &x
				}
			}
			`, []string{
			"test.met:5:28: reference escaping its allocation scope (via return)\n" +
				strings.Trim(`
				        mut x = 42
				        return &x
				               ^^
				    }
				`, "\n"),
		}},
		{"escape via return in if branch", `
			{
				fun foo(a &Int) &Int {
					mut x = 42
					if true { return &x }
					a
				}
			}
			`, []string{
			"test.met:5:38: reference escaping its allocation scope (via return)\n" +
				strings.Trim(`
				        mut x = 42
				        if true { return &x }
				                         ^^
				        a
				`, "\n"),
		}},
		// {"local ref escapes via mut param", `
		// 	{
		// 		struct Foo { mut one &Int }
		// 		fun foo(a &mut Foo) void {
		// 			let x = 42
		// 			a.one = &x
		// 		}
		// 	}
		// 	`, []string{
		// 	"should fail",
		// }},
		{"deref on rhs escapes", `
			{
				mut x = 0
				mut y = &x
				{
					mut z = 456
					mut w = &z
					mut v = &w
					y = v.*
				}
			}
			`, []string{
			"test.met:7:29: reference escaping its allocation scope (via mutation of outer variable)\n" +
				strings.Trim(`
						mut z = 456
					    mut w = &z
								^^
					    mut v = &w
					`, "\n"),
		}},
		{"call returns ref to local", `
			{
				fun identity(a &Int) &Int { a }
				let x = {
					let y = 42
					identity(&y)
				}
				x
			}
			`, []string{
			"test.met:6:30: reference escaping its allocation scope (via block result)\n" +
				strings.Trim(`
				        let y = 42
				        identity(&y)
				                 ^^
				    }
				`, "\n"),
		}},
		{"transitive call returns ref to local", `
			{
				fun identity(a &Int) &Int { a }
				fun foo(a &Int) &Int { identity(a) }
				let x = {
					let y = 42
					foo(&y)
				}
				x
			}
			`, []string{
			"test.met:7:25: reference escaping its allocation scope (via block result)\n" +
				strings.Trim(`
				        let y = 42
				        foo(&y)
				            ^^
				    }
				`, "\n"),
		}},
		{"call returns struct with ref to local", `
			{
				struct Wrapper { one &Int }
				fun foo(a &Int) Wrapper { Wrapper(a) }
				let x = {
					let y = 42
					foo(&y)
				}
				x
			}
			`, []string{
			"test.met:7:25: reference escaping its allocation scope (via block result)\n" +
				strings.Trim(`
				        let y = 42
				        foo(&y)
				            ^^
				    }
				`, "\n"),
		}},
		{"nested struct literal ref escapes", `
			{
				struct Foo { one &Int }
				struct Bar { one Foo }
				let x = {
					let y = 42
					Bar(Foo(&y))
				}
				x
			}
			`, []string{
			"test.met:7:29: reference escaping its allocation scope (via block result)\n" +
				strings.Trim(`
				        let y = 42
				        Bar(Foo(&y))
				                ^^
				    }
				`, "\n"),
		}},
		{"field read propagates ref escape", `
			{
				struct Wrapper { one &Int }
				let x = {
					let y = 42
					let z = Wrapper(&y)
					z.one
				}
				x
			}
			`, []string{
			"test.met:6:37: reference escaping its allocation scope (via block result)\n" +
				strings.Trim(`
				        let y = 42
				        let z = Wrapper(&y)
				                        ^^
				        z.one
				`, "\n"),
		}},
		{"field read through ref propagates escape", `
			{
				struct Wrapper { one &Int }
				let x = {
					let y = 42
					let z = Wrapper(&y)
					let w = &z
					w.one
				}
				x
			}
			`, []string{
			"test.met:6:37: reference escaping its allocation scope (via block result)\n" +
				strings.Trim(`
						let y = 42
				        let z = Wrapper(&y)
									    ^^
				        let w = &z
				`, "\n"),
		}},
		{"nested field read propagates escape", `
			{
				struct Foo { one &Int }
				struct Bar { one Foo }
				let x = {
					let y = 42
					let z = Bar(Foo(&y))
					let w = &z
					w.one.one
				}
				x
			}
			`, []string{
			"test.met:7:37: reference escaping its allocation scope (via block result)\n" +
				strings.Trim(`
						let y = 42
				        let z = Bar(Foo(&y))
										^^
				        let w = &z
				`, "\n"),
		}},
		{"field read after reassign escapes", `
			{
				struct Wrapper { one &Int }
				let x = 1
				mut y = Wrapper(&x)
				let z = {
					let w = 42
					y = Wrapper(&w)
					y.one
				}
			}
			`, []string{
			"test.met:8:33: reference escaping its allocation scope (via mutation of outer variable)\n" +
				strings.Trim(`
				        let w = 42
				        y = Wrapper(&w)
				                    ^^
				        y.one
				`, "\n"),
			"test.met:8:33: reference escaping its allocation scope (via block result)\n" +
				strings.Trim(`
				        let w = 42
				        y = Wrapper(&w)
				                    ^^
				        y.one
				`, "\n"),
		}},
		// Heap-allocated struct escapes the block where the allocator lives.
		{"heap alloc escapes", `
			{
				struct Foo { one Str }
				let x = {
					let @myalloc = Arena()
					new(@myalloc, Foo("hello"))
				}
				x
			}
			`, []string{
			"test.met:6:21: reference escaping its allocation scope (via block result)\n" +
				strings.Trim(`
				        let @myalloc = Arena()
				        new(@myalloc, Foo("hello"))
				        ^^^^^^^^^^^^^^^^^^^^^^^^^^^
				    }
				`, "\n"),
		}},
		// Heap-allocated array escapes the block where the allocator lives.
		{"heap alloc array escapes", `
			{
				let x = {
					let @myalloc = Arena()
					new(@myalloc, [5]Int())
				}
				x
			}
			`, []string{
			"test.met:5:21: reference escaping its allocation scope (via block result)\n" +
				strings.Trim(`
				        let @myalloc = Arena()
				        new(@myalloc, [5]Int())
				        ^^^^^^^^^^^^^^^^^^^^^^^
				    }
				`, "\n"),
		}},
		// Valid: allocator passed as param, result used in caller's scope.
		{"valid heap alloc through param", `
			{
				struct Foo { one Str }
				fun foo(@myalloc Arena) &Foo { new(@myalloc, Foo("hello")) }
				let @myalloc = Arena()
				let x = foo(@myalloc)
			}
			`, []string{}},
		// Heap-allocated struct escapes through a nested block result.
		{"heap alloc nested escape", `
			{
				struct Foo { one Str }
				let @youralloc = Arena()
				let x = {
					let @myalloc = Arena()
					new(@myalloc, Foo("hello"))
				}
				x
			}
			`, []string{
			"test.met:7:21: reference escaping its allocation scope (via block result)\n" +
				strings.Trim(`
				        let @myalloc = Arena()
				        new(@myalloc, Foo("hello"))
				        ^^^^^^^^^^^^^^^^^^^^^^^^^^^
				    }
				`, "\n"),
		}},
		// Heap alloc ref assigned to another variable and passed to a function still escapes.
		{"heap alloc ref assignment escapes", `
			{
				struct Foo { one Str }
				fun foo(@myalloc Arena) &Foo { new(@myalloc, Foo("hello")) }
				fun identity(a &Foo) &Foo { a }
				let x = {
					let @myalloc = Arena()
					let y = foo(@myalloc)
					let z = y
					identity(z)
				}
				x
			}
			`, []string{
			"test.met:10:21: reference escaping its allocation scope (via block result)\n" +
				strings.Trim(`
				        let z = y
				        identity(z)
				        ^^^^^^^^^^^
				    }
				`, "\n"),
		}},
		// Heap-allocated struct escapes through a function call with a local allocator.
		{"heap alloc call escape", `
			{
				struct Foo { one Str }
				fun foo(@myalloc Arena) &Foo { new(@myalloc, Foo("hello")) }
				let x = {
					let @youralloc = Arena()
					foo(@youralloc)
				}
				x
			}
			`, []string{
			"test.met:7:21: reference escaping its allocation scope (via block result)\n" +
				strings.Trim(`
				        let @youralloc = Arena()
				        foo(@youralloc)
				        ^^^^^^^^^^^^^^^
				    }
				`, "\n"),
		}},
		{"allocator field escape", `
			{
				struct Foo { one Str }
				struct Bar { @myalloc Arena }
				fun foo(a Bar) &Foo {
					new(a.@myalloc, Foo("hello"))
				}
				let x = {
					let @myalloc = Arena()
					let y = Bar(@myalloc)
					foo(y)
				}
				x
			}
			`, []string{
			"test.met:11:21: reference escaping its allocation scope (via block result)\n" +
				strings.Trim(`
				        let y = Bar(@myalloc)
				        foo(y)
				        ^^^^^^
				    }
				`, "\n"),
		}},

		// Valid: struct with allocator used within same scope.
		{"valid struct allocator", `
			{
				struct Foo { one Str }
				struct Bar { @myalloc Arena }
				let @myalloc = Arena()
				let x = Bar(@myalloc)
				let y = new(x.@myalloc, Foo("hello"))
			}
			`, []string{}},

		// Nested struct: allocator buried two levels deep.
		{"nested allocator escape", `
			{
				struct Foo { one Str }
				struct Bar { @myalloc Arena }
				struct Baz { one Bar }
				fun foo(a Baz) &Foo {
					new(a.one.@myalloc, Foo("hello"))
				}
				let x = {
					let @myalloc = Arena()
					let y = Baz(Bar(@myalloc))
					foo(y)
				}
				x
			}
			`, []string{
			"test.met:12:21: reference escaping its allocation scope (via block result)\n" +
				strings.Trim(`
				        let y = Baz(Bar(@myalloc))
				        foo(y)
				        ^^^^^^
				    }
				`, "\n"),
		}},

		// Default value in new-array carries a ref that escapes the inner scope.
		{"new array default ref escapes", `
			{
				struct Wrapper { one &Int }
				let @a = Arena()
				let x = {
					let local = 123
					new_mut(@a, [3]Wrapper(Wrapper(&local)))
				}
			}
			`, []string{
			"test.met:7:52: reference escaping its allocation scope (via block result)\n" +
				strings.Trim(`
				        let local = 123
				        new_mut(@a, [3]Wrapper(Wrapper(&local)))
				                                       ^^^^^^
				    }
				`, "\n"),
		}},
		// Default value in make-slice carries a ref that escapes the inner scope.
		{"make slice default ref escapes", `
			{
				struct Wrapper { one &Int }
				let @a = Arena()
				let x = {
					let local = 123
					make(@a, []Wrapper(3, Wrapper(&local)))
				}
			}
			`, []string{
			"test.met:7:51: reference escaping its allocation scope (via block result)\n" +
				strings.Trim(`
				        let local = 123
				        make(@a, []Wrapper(3, Wrapper(&local)))
				                                      ^^^^^^
				    }
				`, "\n"),
		}},

		// Ref array: default value is a reference to a stack local that escapes.
		{"new ref array default escapes", `
			{
				let @a = Arena()
				let x = {
					let local = 123
					new_mut(@a, [3]&Int(&local))
				}
			}
			`, []string{
			"test.met:6:41: reference escaping its allocation scope (via block result)\n" +
				strings.Trim(`
				        let local = 123
				        new_mut(@a, [3]&Int(&local))
				                            ^^^^^^
				    }
				`, "\n"),
		}},
		// Ref slice: default value is a reference to a stack local that escapes.
		{"make ref slice default escapes", `
			{
				let @a = Arena()
				let x = {
					let local = 123
					make(@a, []&Int(3, &local))
				}
			}
			`, []string{
			"test.met:6:40: reference escaping its allocation scope (via block result)\n" +
				strings.Trim(`
				        let local = 123
				        make(@a, []&Int(3, &local))
				                           ^^^^^^
				    }
				`, "\n"),
		}},

		// Valid: shadowed variable with ref should not trigger false positive.
		{"valid shadowed ref", `
			{
				mut x = 123
				mut y = &x
				{
					mut z = 456
					mut y = &mut z
					y.* = 789
				}
			}
			`, []string{}},
		// If/else: ref to a local escapes through one branch.
		{"if branch ref escapes", `
			{
				let x = 1
				let y = {
					let z = 42
					if true { &z } else { &x }
				}
			}
			`, []string{
			"test.met:6:31: reference escaping its allocation scope (via block result)\n" +
				strings.Trim(`
				        let z = 42
				        if true { &z } else { &x }
				                  ^^
				    }
				`, "\n"),
		}},
		// Two refs from different scopes passed to a function that could
		// return either - result carries both taints, should fail.
		{"call with mixed-scope refs escapes", `
			{
				fun foo(a &Int, b &Int) &Int { if true { a } else { b } }
				let x = 42
				let y = {
					let z = 99
					foo(&x, &z)
				}
			}
			`, []string{
			"test.met:7:29: reference escaping its allocation scope (via block result)\n" +
				strings.Trim(`
				        let z = 99
				        foo(&x, &z)
				                ^^
				    }
				`, "\n"),
		}},
		// Valid: both refs from same scope, result doesn't escape.
		{"valid call with same-scope refs", `
			{
				fun foo(a &Int, b &Int) &Int { if true { a } else { b } }
				let x = 42
				let y = 99
				let z = foo(&x, &y)
			}
			`, []string{}},

		// Struct literal containing a ref to a local - ref escapes via intermediate binding.
		{"struct literal ref escapes", `
			{
				struct Wrapper { one &Int }
				let x = {
					let y = 42
					let z = Wrapper(&y)
					z
				}
				x
			}
			`, []string{
			"test.met:6:37: reference escaping its allocation scope (via block result)\n" +
				strings.Trim(`
				        let y = 42
				        let z = Wrapper(&y)
				                        ^^
				        z
				`, "\n"),
		}},
		// Array literal containing a ref to a local - ref escapes via intermediate binding.
		{"array literal ref escapes", `
			{
				let x = {
					let y = 42
					let z = [&y]
					z
				}
				x
			}
			`, []string{
			"test.met:5:30: reference escaping its allocation scope (via block result)\n" +
				strings.Trim(`
				        let y = 42
				        let z = [&y]
				                 ^^
				        z
				`, "\n"),
		}},
		// Valid: array of refs where the refs don't escape.
		{"valid array literal ref", `
			{
				let x = 42
				let y = [&x]
			}
			`, []string{}},

		// Field assign through index: y[0].one = &z where z is local and y escapes.
		{"index field write escapes", `
			{
				struct Wrapper { mut one &Int }
				mut x = 123
				mut y = [Wrapper(&x)]
				{
					mut z = 456
					y[0].one = &z
				}
			}
			`, []string{
			"test.met:8:32: reference escaping its allocation scope (via mutation of outer variable)\n" +
				strings.Trim(`
					    mut z = 456
					    y[0].one = &z
					               ^^
					}
					`, "\n"),
		}},
		// Valid: field assign through index where ref doesn't escape.
		{"valid index field write", `
			{
				struct Wrapper { mut one &Int }
				mut x = 123
				mut y = 456
				mut z = [Wrapper(&x)]
				z[0].one = &y
			}
			`, []string{}},

		// Index assign: y.one[0] = &mut z where z is local and y escapes.
		{"field index write escape", `
			{
				struct Foo { mut one [1]&mut Int }
				mut x = 123
				mut y = Foo([&mut x])
				{
					mut z = 456
					y.one[0] = &mut z
				}
			}
			`, []string{
			"test.met:8:32: reference escaping its allocation scope (via mutation of outer variable)\n" +
				strings.Trim(`
					    mut z = 456
					    y.one[0] = &mut z
					               ^^^^^^
					}
					`, "\n"),
		}},
		// Valid: index assign through field where ref doesn't escape.
		{"valid field index write", `
			{
				struct Foo { mut one [1]&mut Int }
				mut x = 123
				mut y = 456
				mut z = Foo([&mut x])
				z.one[0] = &mut y
			}
			`, []string{}},

		// Function returns ref to a local that was reassigned from a deref.
		{"return ref to reassigned local", `
			{
				fun foo(a &Int) &Int {
					mut x = 1
					x = a.*
					&x
				}
				let y = 42
				let r = foo(&y)
			}
			`, []string{
			"test.met:6:21: reference escaping its allocation scope (via block result)\n" +
				strings.Trim(`
				        x = a.*
				        &x
				        ^^
				    }
				`, "\n"),
		}},
		// Same as above but the local is a heap-allocated struct.
		{"return ref to reassigned heap alloc local", `
			{
				struct Foo { @myalloc Arena }
				fun foo(a &Foo) &Foo {
					let @youralloc = Arena()
					mut x = Foo(@youralloc)
					x = a.*
					&x
				}
				let @myalloc = Arena()
				let x = Foo(@myalloc)
				let r = foo(&x)
			}
			`, []string{
			"test.met:8:21: reference escaping its allocation scope (via block result)\n" +
				strings.Trim(`
				        x = a.*
				        &x
				        ^^
				    }
				`, "\n"),
		}},
		// Ref to local still escapes even after the local is reassigned.
		{"ref to local after reassign escapes", `
			{
				let x = {
					mut y = 1
					y = 2
					&y
				}
				x
			}
			`, []string{
			"test.met:6:21: reference escaping its allocation scope (via block result)\n" +
				strings.Trim(`
				        y = 2
				        &y
				        ^^
				    }
				`, "\n"),
		}},
		// No false positive: function takes &mut but doesn't flow the ref.
		{
			"no escape through unused mut ref param",
			`{ fun foo(a &mut Int) void { a.* = 321 } mut x = 123 foo(&mut x) }`,
			[]string{},
		},
		{"field mutation bypass", `
			{
				struct Foo { mut one &Int }
				fun foo(a &mut Foo, b &Int) void {
					a.one = b
				}
				mut x = 42
				mut y = Foo(&mut x)
				{
					mut z = 99
					foo(&mut y, &z)
				}
				print_int(y.one.*)
			}
			`, []string{
			"test.met:11:33: reference escaping its allocation scope (via mutation of outer variable)\n" +
				strings.Trim(`
						mut z = 99
						foo(&mut y, &z)
								    ^^
					}
				`, "\n"),
		}},
		{"transitive mutation bypass", `
			{
				struct Foo { mut one &Int }
				fun identity(a &mut Foo) &mut Foo { a }
				mut x = 42
				mut y = Foo(&mut x)
				{
					mut z = 99
					let w = identity(&mut y)
					w.one = &z
				}
				print_int(y.one.*)
			}
			`, []string{
			"test.met:10:29: reference escaping its allocation scope (via mutation of outer variable)\n" +
				strings.Trim(`
						let w = identity(&mut y)
						w.one = &z
							    ^^
					}
				`, "\n"),
		}},
		{"returned ref bypass", `
			{
				struct Foo { mut one &Int }
				fun identity(a &mut Foo) &mut Foo { a }
				mut x = 12742
				mut y = Foo(&mut x)
				{
					mut z = 99
					let w = identity(&mut y)
					w.one = &z
				}
				print_int(y.one.*)
			}
			`, []string{
			"test.met:10:29: reference escaping its allocation scope (via mutation of outer variable)\n" +
				strings.Trim(`
					    let w = identity(&mut y)
						w.one = &z
								^^
					}
				`, "\n"),
		}},
		{"heap alloc stack-ref bypass", `
			{
				struct Foo { mut one &Int }
				fun foo(a &mut Foo, b &Int) void { a.one = b }
				let @myalloc = Arena()
				mut x = 1
				let y = new_mut(@myalloc, Foo(&mut x))
				{
					mut z = 99
					foo(y, &z)
				}
				print_int(y.one.*)
			}
			`, []string{
			"test.met:10:28: reference escaping its allocation scope (via mutation of outer variable)\n" +
				strings.Trim(`
				        mut z = 99
				        foo(y, &z)
				               ^^
				    }
				`, "\n"),
		}},
		{"forward declare bypass", `
			{
				struct Foo { mut one &Int }
				fun foo(a &mut Foo, b &Int) void {
					bar(a, b)
				}
				fun bar(a &mut Foo, b &Int) void {
					a.one = b
				}
				mut x = 42
				mut y = Foo(&mut x)
				{
					mut z = 99
					foo(&mut y, &z)
				}
				print_int(y.one.*)
			}
			`, []string{
			"test.met:14:33: reference escaping its allocation scope (via mutation of outer variable)\n" +
				strings.Trim(`
				        mut z = 99
				        foo(&mut y, &z)
				                    ^^
				    }
				`, "\n"),
		}},
		// Pessimistic effects should not taint the return flow when the return
		// type cannot carry a reference.
		{"pessimistic no return taint for void", `
			{
				fun foo(a &Int) void { foo(a) }
				let x = 42
				foo(&x)
			}
			`, []string{}},
		{"pessimistic no return taint for int", `
			{
				fun foo(a &Int) Int { foo(a) }
				let x = 42
				foo(&x)
			}
			`, []string{}},
		{"pessimistic return taint for ref", `
			{
				fun foo(a &Int) &Int { foo(a) }
				let x = 42
				foo(&x)
			}
			`, []string{
			"test.met:5:21: reference escaping its allocation scope (via block result)\n" +
				strings.Trim(`
					let x = 42
					foo(&x)
					    ^^
				}
				`, "\n"),
		}},
		// mutual recursion (foo -> bar -> foo). The analyzer detects the cycle
		// during on-demand analysis and applies a pessimistic "worst-case" effect (assuming all parameters
		// could be mutated/returned). This is sound but overzealous, as it flags safe code like this as a leak.
		{"mutual recursion bypass", `
			{
				struct Foo {
					mut one &Int
				}

				fun foo(a &mut Foo, b &Int) Int {
					bar(a, b)
					42
				}

				fun bar(a &mut Foo, b &Int) Int {
					foo(a, b)
					1337
				}

				mut x = 0
				mut y = Foo(&mut x)

				{
					mut z = 99
					foo(&mut y, &z)
				}

				print_int(y.one.*)
			}
			`, []string{
			"test.met:22:33: reference escaping its allocation scope (via mutation of outer variable)\n" +
				strings.Trim(`
						mut z = 99
						foo(&mut y, &z)
                                    ^^
					}
				`, "\n"),
		}},
		{"side-effect bypass", `
			{
				struct Foo { mut one &Int }
				fun identity(a &mut Foo) &mut Foo { a }
				fun foo(a &mut Foo, b &Int) void { a.one = b }

				mut x = 12742
				mut y = Foo(&mut x)
				{
					mut z = 99
					foo(identity(&mut y), &z)
				}
				print_int(y.one.*)
			}
			`, []string{
			"test.met:11:43: reference escaping its allocation scope (via mutation of outer variable)\n" +
				strings.Trim(`
				        mut z = 99
				        foo(identity(&mut y), &z)
				                              ^^
				    }
				`, "\n"),
		}},
		{"multi-level deref mutation escapes", `
			{
				struct Foo { mut one &Int }
				mut x = 12742
				mut y = Foo(&mut x)
				mut z = &mut y
				mut w = &mut z
				{
					mut a = 99
					mut b = Foo(&mut a)
					w.*.* = b
				}
				print_int(y.one.*)
			}
			`, []string{
			"test.met:10:33: reference escaping its allocation scope (via deref assignment)\n" +
				strings.Trim(`
						mut a = 99
						mut b = Foo(&mut a)
									^^^^^^
						w.*.* = b
				`, "\n"),
		}},
		{"deref field ref mutation escapes", `
			{
				struct Foo { mut one &mut &Int }
				mut x = 12742
				mut y = &x
				mut z = Foo(&mut y)
				{
					mut w = 99
					z.one.* = &w
				}
				print_int(z.one.*.*)
			}
			`, []string{
			"test.met:9:31: reference escaping its allocation scope (via deref assignment)\n" +
				strings.Trim(`
					    mut w = 99
					    z.one.* = &w
					              ^^
					}
					`, "\n"),
		}},
		// Writing to one field must not clear escape taint on another field.
		{"field overwrite doesn't mask escape", `
			{
				struct Foo { mut one Str  mut two &Int }
				mut x = 12742
				mut y = Foo("hello", &x)
				{
					mut z = 99
					y.two = &z
					y.one = "bye"
				}
				print_int(y.two.*)
			}
			`, []string{
			"test.met:8:29: reference escaping its allocation scope (via mutation of outer variable)\n" +
				strings.Trim(`
					    mut z = 99
					    y.two = &z
					            ^^
					    y.one = "bye"
					`, "\n"),
		}},
		// For-loop: reference to a local declared inside the loop body escapes
		// to an outer-scope variable.
		{"for loop ref escapes", `
			{
				mut x = 0
				mut y = &x
				for {
					mut z = 99
					y = &z
					break
				}
			}
			`, []string{
			"test.met:7:25: reference escaping its allocation scope (via mutation of outer variable)\n" +
				strings.Trim(`
				        mut z = 99
				        y = &z
				            ^^
				        break
				`, "\n"),
		}},
		// TODO: The lifetime checker does not track refs flowing through function
		// call return values when the ref originates from an argument.
		{"method ref escapes via return", `
			{
				struct Foo { }
				fun Foo.escape(f Foo, a &Int) &Int { a }
				fun bar(t Foo) &Int { let a = 42 t.escape(&a) }
				let r = {
					let f = Foo()
					bar(f)
				}
				r
			}
			`, []string{
			"test.met:5:59: reference escaping its allocation scope (via block result)\n" +
				strings.Trim(`
					fun Foo.escape(f Foo, a &Int) &Int { a }
					fun bar(t Foo) &Int { let a = 42 t.escape(&a) }
															  ^^
					let r = {
				`, "\n"),
		}},
		// Test that type parameter constraints are checked at all.
		// And pessimistic function effects are applied.
		{"shape method ref escapes", `
			{
				shape Shape {
					fun Shape.escape(s Shape, a &Int) &Int
				}
				struct Foo { }
				fun Foo.escape(f Foo, a &Int) &Int { a }
				fun bar<T Shape>(t T) &Int { let a = 42 t.escape(&a) }
				let r = {
					let f = Foo()
					bar<Foo>(f)
				}
				r
			}
			`, []string{
			"test.met:8:66: reference escaping its allocation scope (via block result)\n" +
				strings.Trim(`
					fun Foo.escape(f Foo, a &Int) &Int { a }
					fun bar<T Shape>(t T) &Int { let a = 42 t.escape(&a) }
																	 ^^
					let r = {
				`, "\n"),
		}},
		// Test that shape fields are checked.
		{"shape field ref escapes", `
			{
				shape Shape { 
					mut one &Int 
				}
				
				fun foo<T Shape>(s &mut T) void {
					let x = 42
					s.one = &x
				}
			}
			`, []string{
			"test.met:9:29: reference escaping its allocation scope (via mutation of outer variable)\n" +
				strings.Trim(`
						let x = 42
						s.one = &x
						        ^^
					}
				`, "\n"),
		}},
		{"shape mut param ref escapes via side effect", `
			{
				shape HasRef { mut one &Int }
				struct Foo { mut one &Int }
				struct Bar { mut one &Int }
				fun baz<T HasRef>(t &mut T, b &Int) void { t.one = b }
				mut x = 42
				mut foo = Foo(&mut x)
				{
					let z = 99
					baz<Foo>(&mut foo, &z)
				}
			}
			`, []string{
			"test.met:11:40: reference escaping its allocation scope (via mutation of outer variable)\n" +
				strings.Trim(`
				        let z = 99
				        baz<Foo>(&mut foo, &z)
				                           ^^
				    }
				`, "\n"),
		}},
		{"shape value param no escape when not written to mut ref", `
			{
				shape HasRef { one &Int }
				struct Foo { one &Int }
				struct Bar { mut one &Int }
				fun baz<T HasRef>(t T, b &mut Bar) void { b.one = b.one }
				let x = 42
				mut y = Bar(&x)
				{
					let z = 99
					baz<Foo>(Foo(&z), &mut y)
				}
				print_int(y.one.*)
			}
			`, nil},
		{"shape value param ref escapes via side effect", `
			{
				shape HasRef { one &Int }
				struct Foo { one &Int }
				struct Bar { mut one &Int }
				fun baz<T HasRef>(t T, b &mut Bar) void { b.one = t.one }
				let x = 42
				mut y = Bar(&x)
				{
					let z = 99
					baz<Foo>(Foo(&z), &mut y)
				}
				print_int(y.one.*)
			}
			`, []string{
			"test.met:11:34: reference escaping its allocation scope (via mutation of outer variable)\n" +
				strings.Trim(`
				        let z = 99
				        baz<Foo>(Foo(&z), &mut y)
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
			source := base.NewSource("test.met", "test", true, []rune(strings.ReplaceAll(tt.src, "\t", "    ")))
			tokens := token.Lex(source)
			parser := ast.NewParser(tokens, 1)
			exprID, parseOK := parser.ParseExpr(0)
			assert.Equal(0, len(parser.Diagnostics), "parsing failed:\n%s", parser.Diagnostics)
			assert.Equal(true, parseOK)
			preludeAST, _ := ast.PreludeAST()
			parser.Roots = append(parser.Roots, exprID)
			e := NewEngine(parser.AST, preludeAST)
			e.Query(exprID)
			assert.Equal(0, len(e.c.diagnostics), "type check failed: %s", e.c.diagnostics)
			a := NewLifetimeAnalyzer(e.c.ast, e.c.scopeGraph, e.Env())
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
			if len(a.Diagnostics) > len(tt.want) {
				t.Fatalf(
					"there are more diagnostics than expected. \n>>> want:\n%s\n>>> got:\n%s",
					tt.want,
					a.Diagnostics,
				)
			}
		})
	}
}
