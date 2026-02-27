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
			"test.met:1:44: reference escaping its allocation scope\n" +
				`    { mut a = 123 mut b = &a { mut c = 123 b = &c } }` + "\n" +
				"                                               ^^",
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
			"test.met:8:29: reference escaping its allocation scope\n" +
				strings.Trim(`
					    {
						    b = &c
						        ^^
					    }
					`, "\n"),
		}},
		{"deref assign", `
			{
				mut a = 123
				mut y = &a
				mut z = &mut y
				{
				  mut c = 456
				  *z = &c
				}
			}
			`, []string{
			"test.met:8:24: reference escaping its allocation scope\n" +
				strings.Trim(`
				      mut c = 456
				      *z = &c
				           ^^
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
					mut y = &mut z
					{
						mut c = 789
						*y = &c
					}
				}
			}
			`, []string{
			"test.met:10:30: reference escaping its allocation scope\n" +
				strings.Trim(`
					        mut c = 789
					        *y = &c
					             ^^
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
				mut y = &mut x
				{
					mut c = 456
					*y = &c
				}
			}
			`, []string{
			"test.met:8:26: reference escaping its allocation scope\n" +
				strings.Trim(`
					    mut c = 456
					    *y = &c
					         ^^
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
			"test.met:8:31: reference escaping its allocation scope\n" +
				strings.Trim(`
					    mut c = 456
					    foo.ptr = &c
					              ^^
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
			"test.met:7:32: reference escaping its allocation scope\n" +
				strings.Trim(`
						mut c = 456
					    mut cRef = &c
								   ^^
					    mut x = &cRef
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
			"test.met:6:37: reference escaping its allocation scope\n" +
				strings.Trim(`
						let x = 42
				        let w = Wrapper(&x)
									    ^^
				        let rw = &w
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
			"test.met:7:41: reference escaping its allocation scope\n" +
				strings.Trim(`
						let x = 42
				        let o = Outer(Inner(&x))
										    ^^
				        let ro = &o
				`, "\n"),
		}},
		// Struct reassignment: w is reassigned to carry a local ref, then w.ptr escapes.
		{"field read after struct reassign escape", `
			{
				struct Wrapper { ptr &Int }
				let a = 1
				mut w = Wrapper(&a)
				let r = {
					let x = 42
					w = Wrapper(&x)
					w.ptr
				}
				r
			}
			`, []string{
			"test.met:8:33: reference escaping its allocation scope\n" +
				strings.Trim(`
				        let x = 42
				        w = Wrapper(&x)
				                    ^^
				        w.ptr
				`, "\n"),
		}},
		// Arena-allocated struct escapes the block where the allocator lives.
		{"arena alloc escapes block", `
			{
				struct Planet { name Str }
				let p = {
					alloc @a = Arena()
					new @a Planet("Earth")
				}
				p
			}
			`, []string{
			"test.met:6:21: reference escaping its allocation scope\n" +
				strings.Trim(`
				        alloc @a = Arena()
				        new @a Planet("Earth")
				        ^^^^^^^^^^^^^^^^^^^^^^
				    }
				`, "\n"),
		}},
		// Arena-allocated array escapes the block where the allocator lives.
		{"arena array alloc escapes block", `
			{
				let p = {
					alloc @a = Arena()
					new @a [5]Int()
				}
				p
			}
			`, []string{
			"test.met:5:21: reference escaping its allocation scope\n" +
				strings.Trim(`
				        alloc @a = Arena()
				        new @a [5]Int()
				        ^^^^^^^^^^^^^^^
				    }
				`, "\n"),
		}},
		// Valid: arena-allocated struct used within the allocator's scope.
		{"valid arena alloc same scope", `
			{
				struct Planet { name Str }
				alloc @a = Arena()
				let p = new @a Planet("Earth")
				p
			}
			`, []string{}},
		// Valid: allocator passed as param, result used in caller's scope.
		{"valid arena alloc via param", `
			{
				struct Planet { name Str }
				fun make(@a Arena) &Planet { let p = new @a Planet("Earth") &p }
				alloc @a = Arena()
				let p = make(@a)
				p
			}
			`, []string{}},
		// Arena-allocated struct escapes through a nested block result.
		{"arena alloc escapes nested block", `
			{
				struct Planet { name Str }
				alloc @outer = Arena()
				let p = {
					alloc @a = Arena()
					new @a Planet("Earth")
				}
				p
			}
			`, []string{
			"test.met:7:21: reference escaping its allocation scope\n" +
				strings.Trim(`
				        alloc @a = Arena()
				        new @a Planet("Earth")
				        ^^^^^^^^^^^^^^^^^^^^^^
				    }
				`, "\n"),
		}},
		// Arena ref assigned to another variable and passed to a function still escapes.
		{"arena ref propagates through assignment and call", `
			{
				struct Planet { name Str }
				fun make(@a Arena) &Planet { let p = new @a Planet("Earth") &p }
				fun identity(p &Planet) &Planet { p }
				let r = {
					alloc @a = Arena()
					let p = make(@a)
					let q = p
					identity(q)
				}
				r
			}
			`, []string{
			"test.met:10:21: reference escaping its allocation scope\n" +
				strings.Trim(`
				        let q = p
				        identity(q)
				        ^^^^^^^^^^^
				    }
				`, "\n"),
		}},
		// Arena-allocated struct escapes through a function call with a local allocator.
		{"arena alloc escapes via function call", `
			{
				struct Planet { name Str }
				fun make(@a Arena) &Planet { let p = new @a Planet("Earth") &p }
				let p = {
					alloc @inner = Arena()
					make(@inner)
				}
				p
			}
			`, []string{
			"test.met:7:21: reference escaping its allocation scope\n" +
				strings.Trim(`
				        alloc @inner = Arena()
				        make(@inner)
				        ^^^^^^^^^^^^
				    }
				`, "\n"),
		}},
		{"allocator in struct escapes via function", `
			{
				struct Planet { name Str }
				struct Holder { @a Arena }
				fun make_planet(h Holder) &Planet {
					let p = new h.@a Planet("Earth")
					&p
				}
				let result = {
					alloc @a = Arena()
					let h = Holder(@a)
					make_planet(h)
				}
				result
			}
			`, []string{
			"test.met:12:21: reference escaping its allocation scope\n" +
				strings.Trim(`
				        let h = Holder(@a)
				        make_planet(h)
				        ^^^^^^^^^^^^^^
				    }
				`, "\n"),
		}},

		// Valid: function takes a Holder but doesn't use its allocator in return value.
		{"valid allocator in struct not used in return", `
			{
				struct Planet { name Str }
				struct Holder { @a Arena }
				fun ignore_holder(h Holder, p &Planet) &Planet { p }
				alloc @a = Arena()
				let h = Holder(@a)
				let p = new @a Planet("Earth")
				ignore_holder(h, &p)
			}
			`, []string{}},

		// Valid: struct with allocator used within same scope.
		{"valid struct with allocator same scope", `
			{
				struct Planet { name Str }
				struct Holder { @a Arena }
				alloc @a = Arena()
				let h = Holder(@a)
				let p = new h.@a Planet("Earth")
			}
			`, []string{}},

		// Nested struct: allocator buried two levels deep.
		{"allocator in nested struct escapes", `
			{
				struct Planet { name Str }
				struct Inner { @a Arena }
				struct Outer { inner Inner }
				fun make_planet(o Outer) &Planet {
					let p = new o.inner.@a Planet("Earth")
					&p
				}
				let result = {
					alloc @a = Arena()
					let o = Outer(Inner(@a))
					make_planet(o)
				}
				result
			}
			`, []string{
			"test.met:13:21: reference escaping its allocation scope\n" +
				strings.Trim(`
				        let o = Outer(Inner(@a))
				        make_planet(o)
				        ^^^^^^^^^^^^^^
				    }
				`, "\n"),
		}},

		// Valid: shadowed variable with ref should not trigger false positive.
		{"valid shadowed ref no escape", `
			{
				mut a = 123
				mut b = &a
				{
					mut c = 456
					mut b = &mut c
					*b = 789
				}
			}
			`, []string{}},
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
		// Two refs from different scopes passed to a function that could
		// return either — result carries both taints, should fail.
		{"two refs different scopes pick escapes", `
			{
				fun pick(a &Int, b &Int) &Int { if true { a } else { b } }
				let x = 42
				let r = {
					let y = 99
					pick(&x, &y)
				}
				r
			}
			`, []string{
			"test.met:7:30: reference escaping its allocation scope\n" +
				strings.Trim(`
				        let y = 99
				        pick(&x, &y)
				                 ^^
				    }
				`, "\n"),
		}},
		// Valid: both refs from same scope, pick result doesn't escape.
		{"valid two refs same scope pick", `
			{
				fun pick(a &Int, b &Int) &Int { if true { a } else { b } }
				let x = 42
				let y = 99
				let r = pick(&x, &y)
				r
			}
			`, []string{}},

		// Struct literal containing a ref to a local — ref escapes via intermediate binding.
		{"struct literal ref escape via binding", `
			{
				struct Wrapper { ptr &Int }
				let r = {
					let x = 42
					let w = Wrapper(&x)
					w
				}
				r
			}
			`, []string{
			"test.met:6:37: reference escaping its allocation scope\n" +
				strings.Trim(`
				        let x = 42
				        let w = Wrapper(&x)
				                        ^^
				        w
				`, "\n"),
		}},
		// Array literal containing a ref to a local — ref escapes via intermediate binding.
		{"array literal ref escape via binding", `
			{
				let r = {
					let x = 42
					let arr = [&x]
					arr
				}
				r
			}
			`, []string{
			"test.met:5:32: reference escaping its allocation scope\n" +
				strings.Trim(`
				        let x = 42
				        let arr = [&x]
				                   ^^
				        arr
				`, "\n"),
		}},
		// Valid: array of refs where the refs don't escape.
		{"valid array literal ref no escape", `
			{
				let x = 42
				let arr = [&x]
				arr
			}
			`, []string{}},

		// Field assign through index: arr[0].ptr = &c where c is local and arr escapes.
		{"field assign through index escapes", `
			{
				struct Wrapper { mut ptr &Int }
				mut a = 123
				mut arr = [Wrapper(&a)]
				{
					mut c = 456
					arr[0].ptr = &c
				}
			}
			`, []string{
			"test.met:8:34: reference escaping its allocation scope\n" +
				strings.Trim(`
					    mut c = 456
					    arr[0].ptr = &c
					                 ^^
					}
					`, "\n"),
		}},
		// Valid: field assign through index where ref doesn't escape.
		{"valid field assign through index no escape", `
			{
				struct Wrapper { mut ptr &Int }
				mut a = 123
				mut b = 456
				mut arr = [Wrapper(&a)]
				arr[0].ptr = &b
			}
			`, []string{}},

		// Index assign: foo.arr[0] = &c where c is local and foo escapes.
		{"index assign through field escapes", `
			{
				struct Container { mut values [1]&mut Int }
				mut a = 123
				mut foo = Container([&mut a])
				{
					mut c = 456
					foo.values[0] = &mut c
				}
			}
			`, []string{
			"test.met:8:37: reference escaping its allocation scope\n" +
				strings.Trim(`
					    mut c = 456
					    foo.values[0] = &mut c
					                    ^^^^^^
					}
					`, "\n"),
		}},
		// Valid: index assign through field where ref doesn't escape.
		{"valid index assign through field no escape", `
			{
				struct Container { mut values [1]&mut Int }
				mut a = 123
				mut b = 456
				mut foo = Container([&mut a])
				foo.values[0] = &mut b
			}
			`, []string{}},

		{"function reassign then return ref escapes", `
			{
				fun bad(src &Int) &Int {
					mut x = 1
					x = *src
					&x
				}
				let a = 42
				bad(&a)
			}
			`, []string{
			"test.met:6:21: reference escaping its allocation scope\n" +
				strings.Trim(`
				        x = *src
				        &x
				        ^^
				    }
				`, "\n"),
		}},
		{"allocator field function reassign then return ref escapes", `
			{
				struct Holder { @a Arena }
				fun bad(src &Holder) &Holder {
					alloc @tmp = Arena()
					mut h = Holder(@tmp)
					h = *src
					&h
				}
				alloc @a = Arena()
				let src = Holder(@a)
				bad(&src)
			}
			`, []string{
			"test.met:8:21: reference escaping its allocation scope\n" +
				strings.Trim(`
				        h = *src
				        &h
				        ^^
				    }
				`, "\n"),
		}},
		{"reassign then ref escapes", `
			{
				let r = {
					mut x = 1
					x = 2
					&x
				}
				r
			}
			`, []string{
			"test.met:6:21: reference escaping its allocation scope\n" +
				strings.Trim(`
				        x = 2
				        &x
				        ^^
				    }
				`, "\n"),
		}},
		{"leak through field mutation bypass", `
			{
				struct S { mut r &Int }
				fun store(s &mut S, p &Int) void {
					s.r = p
				}
				mut x = 42
				mut s = S(&mut x)
				{
					mut y = 99
					store(&mut s, &y)
				}
				print_int(*s.r)
			}
			`, []string{
			"test.met:11:35: reference escaping its allocation scope\n" +
				strings.Trim(`
						mut y = 99
						store(&mut s, &y)
								      ^^
					}
				`, "\n"),
		}},
		{"leak through transitive mutation bypass", `
			{
				struct S { mut r &Int }
				fun identity(s &mut S) &mut S { s }
				mut x = 42
				mut s = S(&mut x)
				{
					mut y = 99
					let s2 = identity(&mut s)
					s2.r = &y
				}
				print_int(*s.r)
			}
			`, []string{
			"test.met:10:28: reference escaping its allocation scope\n" +
				strings.Trim(`
						let s2 = identity(&mut s)
						s2.r = &y
							   ^^
					}
				`, "\n"),
		}},
		{"leak through returned ref bypass", `
			{
				struct Planet { mut diameter &Int }
				fun identity(p &mut Planet) &mut Planet { p }
				mut initial_diameter = 12742
				mut earth = Planet(&mut initial_diameter)
				{
					mut local_diameter = 99
					let earth_ref = identity(&mut earth)
					earth_ref.diameter = &local_diameter
				}
				print_int(*earth.diameter)
			}
			`, []string{
			"test.met:10:42: reference escaping its allocation scope\n" +
				strings.Trim(`
					    let earth_ref = identity(&mut earth)
						earth_ref.diameter = &local_diameter
											 ^^^^^^^^^^^^^^^
					}
				`, "\n"),
		}},
		{"leak stack-ref into arena-allocated struct", `
			{
				struct S { mut r &Int }
				fun store(s &mut S, p &Int) void { s.r = p }
				alloc @a = Arena()
				mut x = 1
				mut s = new @a S(&mut x)
				{
					mut y = 99
					store(&mut s, &y)
				}
				print_int(*s.r)
			}
			`, []string{
			"test.met:10:35: reference escaping its allocation scope\n" +
				strings.Trim(`
				        mut y = 99
				        store(&mut s, &y)
				                      ^^
				    }
				`, "\n"),
		}},
		{"leak through forward declare bypass", `
			{
				struct S { mut r &Int }
				fun f1(s &mut S, p &Int) void {
					f2(s, p)
				}
				fun f2(s &mut S, p &Int) void {
					s.r = p
				}
				mut x = 42
				mut s = S(&mut x)
				{
					mut y = 99
					f1(&mut s, &y)
				}
				print_int(*s.r)
			}
			`, []string{
			"test.met:14:32: reference escaping its allocation scope\n" +
				strings.Trim(`
				        mut y = 99
				        f1(&mut s, &y)
				                   ^^
				    }
				`, "\n"),
		}},
		// mutual recursion (perform_task -> delegate_work -> perform_task). The analyzer detects the cycle
		// during on-demand analysis and applies a pessimistic "worst-case" effect (assuming all parameters
		// could be mutated/returned). This is sound but overzealous, as it flags safe code like this as a leak.
		{"pessimistic mutual recursion (overzealous)", `
			{
				struct Worker {
					mut current_task &Int
				}

				fun perform_task(worker &mut Worker, task &Int) void {
					delegate_work(worker, task)
				}

				fun delegate_work(worker &mut Worker, task &Int) void {
					perform_task(worker, task)
				}

				mut default_task = 0
				mut primary_worker = Worker(&mut default_task)

				{
					mut local_task = 99
					perform_task(&mut primary_worker, &local_task)
				}

				print_int(*primary_worker.current_task)
			}
			`, []string{
			"test.met:20:55: reference escaping its allocation scope\n" +
				strings.Trim(`
						mut local_task = 99
						perform_task(&mut primary_worker, &local_task)
                                                          ^^^^^^^^^^^
					}
				`, "\n"),
		}},
		{"leak through side-effect on returned reference argument", `
			{
				struct Planet { mut diameter &Int }
				fun identity(p &mut Planet) &mut Planet { p }
				fun update_diameter(p &mut Planet, new_diameter &Int) void { p.diameter = new_diameter }

				mut initial_diameter = 12742
				mut earth = Planet(&mut initial_diameter)
				{
					mut local_diameter = 99
					update_diameter(identity(&mut earth), &local_diameter)
				}
				print_int(*earth.diameter)
			}
			`, []string{
			"test.met:11:59: reference escaping its allocation scope\n" +
				strings.Trim(`
				        mut local_diameter = 99
				        update_diameter(identity(&mut earth), &local_diameter)
				                                              ^^^^^^^^^^^^^^^
				    }
				`, "\n"),
		}},
		{"bypass: multi-level pointer mutation", `
			{
				struct Planet { mut diameter &Int }
				mut initial_diameter = 12742
				mut earth = Planet(&mut initial_diameter)
				mut earth_ptr = &mut earth
				mut earth_ptr_ptr = &mut earth_ptr
				{
					mut local_diameter = 99
					mut local_earth = Planet(&mut local_diameter)
					**earth_ptr_ptr = local_earth
				}
				print_int(*earth.diameter)
			}
			`, []string{
			"test.met:10:46: reference escaping its allocation scope\n" +
				strings.Trim(`
						mut local_diameter = 99
						mut local_earth = Planet(&mut local_diameter)
												 ^^^^^^^^^^^^^^^^^^^
						**earth_ptr_ptr = local_earth
				`, "\n"),
		}},
		{"bypass: mutation through field reference", `
			{
				struct Planet { mut diameter_ref &mut &Int }
				mut initial_diameter = 12742
				mut initial_diameter_ptr = &initial_diameter
				mut earth = Planet(&mut initial_diameter_ptr)
				{
					mut local_diameter = 99
					*earth.diameter_ref = &local_diameter
				}
				print_int(**earth.diameter_ref)
			}
			`, []string{
			"test.met:9:43: reference escaping its allocation scope\n" +
				strings.Trim(`
					    mut local_diameter = 99
					    *earth.diameter_ref = &local_diameter
					                          ^^^^^^^^^^^^^^^
					}
					`, "\n"),
		}},
		{"bypass: multiple field assignments overwrite leak", `
			{
				struct Planet { mut name Str  mut diameter &Int }
				mut initial_diameter = 12742
				mut earth = Planet("Earth", &initial_diameter)
				{
					mut local_diameter = 99
					earth.diameter = &local_diameter
					earth.name = "New Earth"
				}
				print_int(*earth.diameter)
			}
			`, []string{
			"test.met:8:38: reference escaping its allocation scope\n" +
				strings.Trim(`
					    mut local_diameter = 99
					    earth.diameter = &local_diameter
					                     ^^^^^^^^^^^^^^^
					    earth.name = "New Earth"
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
			// a.Debug = base.NewStdoutDebug("lifetime")
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
