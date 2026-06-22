# Module Tests

**setup external modules**

```metall
```

```module.lib
pub struct Point { pub x Int pub y Int }
pub struct Secret { hidden Int }
pub struct RefHolder { pub r &mut Int }
pub fun make_mixed() Mixed { Mixed(1, 2) }
pub struct Mixed { pub visible Int internal Int }
pub fun get_lib() Int { 42 }
pub fun Point.sum(p Point) Int { p.x + p.y }
pub fun Point.origin() Point { Point(0, 0) }
fun Point.secret(p Point) Int { p.x }
fun internal_helper() Int { 99 }
pub let public_const = 42
let private_const = 99
pub enum IOErr U8 = not_found | denied
pub union Either = Int | Str
```

`reexport` re-exports `lib` symbols: a `pub use` crosses the module boundary, a
bare `use` stays module-private. It also re-exports a generic.

```module.reexport
pub use lib.Point
pub use lib.get_lib
use lib.Secret
pub use generic.identity
```

`priv_test` is the `_test` companion of `priv`; a companion may *use* its
subject's private symbols but must not `pub use` (re-export) them.

```module.priv
fun secret() Int { 7 }
pub fun ok() Int { 1 }
```

```module.priv_test
pub use priv.secret
```

```module.lib_test
use lib
pub fun test_internal() Int { lib.internal_helper() }
```

```module.generic
pub struct Pair<A, B> { pub first A pub second B }
pub fun identity<T>(x T) T { x }
```

```module.local
pub fun get_hello() Str { "hello" }
```

```module.shapes
pub shape Showable {
    pub fun Showable.show(self Showable) Int
}
pub fun show<T Showable>(x T) Int { x.show() }
pub fun show_twice<T Showable>(x T) Int { show<T>(x) + show<T>(x) }
```

## OK

**import unused**

```metall
use lib
fun main() void {}
```

**local import unused**

```metall
use local.hello
fun main() void {}
```

**call imported function**

```metall
use lib
fun main() void { _ = lib.get_lib() }
```

**call imported function return type**

```metall
use lib
fun main() void { let x = lib.get_lib() }
```

```bindings
Module: scope01
  Import: scope02
  Fun: scope02
    SimpleType: scope03
    Block: scope03
      Var: scope04
        Call: scope04
          FieldAccess: scope04
            Ident: scope04
---
scope01:
scope02:
  lib: lib
  main: fun01
scope03:
scope04:
  x: Int
fun01 = sync fun() void
```

**use imported struct**

```metall
use lib
fun main() void { let p = lib.Point(1, 2) _ = p.x _ = p.y }
```

```bindings
Module: scope01
  Import: scope02
  Fun: scope02
    SimpleType: scope03
    Block: scope03
      Var: scope04
        TypeConstruction: scope04
          FieldAccess: scope04
            Ident: scope04
          Int: scope04
          Int: scope04
      Assign: scope04
        Ident: scope04
        FieldAccess: scope04
          Ident: scope04
      Assign: scope04
        Ident: scope04
        FieldAccess: scope04
          Ident: scope04
---
scope01:
scope02:
  lib: lib
  main: fun01
scope03:
scope04:
  p: struct01
fun01    = sync fun() void
struct01 = Point { x Int, y Int }
```

**call method on imported struct**

```metall
use lib
fun main() void { let p = lib.Point(1, 2) _ = p.sum() }
```

**call method on imported struct via path**

```metall
use lib
fun main() void { let p = lib.Point(1, 2) _ = lib.Point.sum(p) }
```

**assign imported function to variable**

```metall
use lib
fun main() void { let f = lib.get_lib _ = f() }
```

**local import call**

```metall
use local.hello
fun main() void { let s = hello.get_hello() }
```

```bindings
Module: scope01
  Import: scope02
  Fun: scope02
    SimpleType: scope03
    Block: scope03
      Var: scope04
        Call: scope04
          FieldAccess: scope04
            Ident: scope04
---
scope01:
scope02:
  hello: hello
  main: fun01
scope03:
scope04:
  s: Str
fun01 = sync fun() void
```

**aliased import**

```metall
use l = lib
fun main() void { _ = l.get_lib() }
```

**generic function from import**

```metall
use generic
fun main() void { let x = generic.identity<Int>(42) }
```

```bindings
Module: scope01
  Import: scope02
  Fun: scope02
    SimpleType: scope03
    Block: scope03
      Var: scope04
        Call: scope04
          FieldAccess: scope04
            Ident: scope04
          Int: scope04
---
scope01:
scope02:
  generic: generic
  main: fun01
scope03:
scope04:
  x: Int
fun01 = sync fun() void
```

**infer type args for generic function from import**

```metall
use generic
fun main() void { let x = generic.identity(42) }
```

```bindings
Module: scope01
  Import: scope02
  Fun: scope02
    SimpleType: scope03
    Block: scope03
      Var: scope04
        Call: scope04
          FieldAccess: scope04
            Ident: scope04
          Int: scope04
---
scope01:
scope02:
  generic: generic
  main: fun01
scope03:
scope04:
  x: Int
fun01 = sync fun() void
```

**generic struct from import**

```metall
use generic
fun main() void {
    let p = generic.Pair<Int, Str>(1, "hi")
    _ = p.first
    _ = p.second
}
```

```bindings
Module: scope01
  Import: scope02
  Fun: scope02
    SimpleType: scope03
    Block: scope03
      Var: scope04
        TypeConstruction: scope04
          FieldAccess: scope04
            Ident: scope04
          Int: scope04
          String: scope04
      Assign: scope04
        Ident: scope04
        FieldAccess: scope04
          Ident: scope04
      Assign: scope04
        Ident: scope04
        FieldAccess: scope04
          Ident: scope04
---
scope01:
scope02:
  generic: generic
  main: fun01
scope03:
scope04:
  p: struct01
fun01    = sync fun() void
struct01 = Pair<Int, Str> { first Int, second Str }
```

**cross-module shape satisfaction**

A generic function in module A calls another generic function in module A
with a shape constraint. The concrete type and its method are defined in
the calling module B. The shape check must find the method in B's scope.

```metall
use shapes

pub struct Widget { pub value Int }
pub fun Widget.show(w Widget) Int { w.value }

fun main() void {
    let w = Widget(42)
    let r = shapes.show_twice(w)
}
```

```bindings
Module: scope01
  Import: scope02
  Struct: scope02
    StructField: scope03
      SimpleType: scope03
  Fun: scope02
    FunParam: scope04
      SimpleType: scope04
    SimpleType: scope04
    Block: scope04
      FieldAccess: scope05
        Ident: scope05
  Fun: scope02
    SimpleType: scope06
    Block: scope06
      Var: scope07
        TypeConstruction: scope07
          Ident: scope07
          Int: scope07
      Var: scope07
        Call: scope07
          FieldAccess: scope07
            Ident: scope07
          Ident: scope07
---
scope01:
scope02:
  Widget: struct01
  main: fun01
  main.Widget.show: fun02
  shapes: shapes
scope03:
scope04:
  w: struct01
scope05:
scope06:
scope07:
  r: Int
  w: struct01
struct01 = Widget { value Int }
fun01    = sync fun() void
fun02    = sync fun(struct01) Int
```

**cross-module shape method dispatch**

A generic function in the calling module dispatches a shape method through a
`<T Shape>` constraint where the shape is declared in an imported module. The
dispatch must resolve the shape method in the shape's module.

```metall
use shapes

pub struct Widget { pub value Int }
pub fun Widget.show(w Widget) Int { w.value }

fun show_here<T shapes.Showable>(x T) Int { x.show() }

fun main() void {
    let w = Widget(42)
    let r = show_here<Widget>(w)
}
```

```error
```

## Error

**unknown symbol in import**

```metall
use lib
fun main() void { lib.unknown() }
```

```error
test.met:2:19: symbol not defined in lib: unknown
    use lib
    fun main() void { lib.unknown() }
                      ^^^^^^^^^^^
```

**wrong arg type to imported function**

```metall
use lib
fun main() void { lib.get_lib("oops") }
```

```error
test.met:2:19: argument count mismatch: expected 0, got 1
    use lib
    fun main() void { lib.get_lib("oops") }
                      ^^^^^^^^^^^^^^^^^^^
```

**generic struct wrong type arg count**

```metall
use generic
fun main() void { generic.Pair<Int>(1) }
```

```error
test.met:2:19: type argument count mismatch: expected 2, got 1
    use generic
    fun main() void { generic.Pair<Int>(1) }
                      ^^^^^^^^^^^^^^^^^
```

**generic function wrong type arg count**

```metall
use generic
fun main() void { generic.identity<Int, Str>(42) }
```

```error
test.met:2:19: type argument count mismatch: expected 1, got 2
    use generic
    fun main() void { generic.identity<Int, Str>(42) }
                      ^^^^^^^^^^^^^^^^^^^^^^^^^^
```

**non-existent member on module**

```metall
use lib
fun main() void { lib.nope() }
```

```error
test.met:2:19: symbol not defined in lib: nope
    use lib
    fun main() void { lib.nope() }
                      ^^^^^^^^
```

**dot syntax on module**

```metall
use lib
fun main() void { lib.get_lib() }
```

```error
test.met:2:19: return type mismatch: expected void, got Int
    use lib
    fun main() void { lib.get_lib() }
                      ^^^^^^^^^^^^^
```

## Visibility

**test module can access non-pub symbols in its subject module**

```metall
use lib_test
fun main() void { _ = lib_test.test_internal() }
```

```error
```

**non-pub function not accessible from outside module**

```metall
use lib
fun main() void { _ = lib.internal_helper() }
```

```error
test.met:2:23: lib::internal_helper is not public
    use lib
    fun main() void { _ = lib.internal_helper() }
                          ^^^^^^^^^^^^^^^^^^^
```

**non-pub field not accessible from outside module**

```metall
use lib
fun main() void {
    let s = lib.Secret(1)
}
```

```error
test.met:3:13: cannot construct lib.Secret from outside its module: field hidden is not public
    fun main() void {
        let s = lib.Secret(1)
                ^^^^^^^^^^^^^
    }
```

**pub let accessible from outside module**

```metall
use lib
fun main() void { _ = lib.public_const }
```

```error
```

**non-pub let not accessible from outside module**

```metall
use lib
fun main() void { _ = lib.private_const }
```

```error
test.met:2:23: lib::private_const is not public
    use lib
    fun main() void { _ = lib.private_const }
                          ^^^^^^^^^^^^^^^^^
```

**can write through pub &mut field from outside module**

```metall
use lib

fun main() void {
    mut x = 42
    let h = lib.RefHolder(&mut x)
    h.r.* = 99
}
```

```error
```

**can reassign pub field from outside module with mutable container**

```metall
use lib

fun main() void {
    mut x = 42
    mut y = 99
    mut h = lib.RefHolder(&mut x)
    h.r = &mut y
}
```

```error
```

**cannot access non-pub field from outside module**

```metall
use lib

fun main() void {
    mut m = lib.make_mixed()
    m.internal = 99
}
```

```error
test.met:5:7: field lib.Mixed.internal is not public
        mut m = lib.make_mixed()
        m.internal = 99
          ^^^^^^^^
    }
```

## Symbol Imports

`use a.b.Name` brings a single symbol into scope; `pub use a.b.Name` re-exports
it. Modules cannot be re-exported.

**import a struct**

```metall
use lib.Point
fun main() void {
    let p = Point(1, 2)
    _ = p.x
}
```

**import and rename**

Construction and method resolution both work through the chosen name.

```metall
use Pt = lib.Point
fun main() void {
    let p Pt = Pt(1, 2)
    _ = p.sum()
    _ = Pt.sum(p)
}
```

**import a function**

```metall
use lib.get_lib
fun main() void {
    _ = get_lib()
}
```

**import a const**

```metall
use lib.public_const
fun main() void {
    let x = public_const
}
```

**import an enum**

```metall
use lib.IOErr
fun main() void {
    let e IOErr = IOErr.not_found
    _ = IOErr.denied.tag
}
```

**import a union**

```metall
use lib.Either
fun main() void {
    let u Either = 5
}
```

**static and factory calls through an imported type**

```metall
use lib.Point
fun main() void {
    let p = Point(1, 2)
    _ = p.sum()
    _ = Point.sum(p)
    let o Point = Point.origin()
}
```

**re-exported pub symbols are usable**

```metall
use reexport
fun main() void {
    let p = reexport.Point(1, 2)
    _ = reexport.get_lib()
}
```

**import a re-exported symbol**

```metall
use reexport.Point
fun main() void {
    let p = Point(1, 2)
}
```

## Symbol Import Errors

**cannot re-export a module**

```metall
pub use lib
fun main() void {}
```

```error
test.met:1:5: cannot re-export a module; only symbols can be re-exported
    pub use lib
        ^^^^^^^
    fun main() void {}
```

**symbol not found**

```metall
use lib.Nope
fun main() void {}
```

```error
test.met:1:1: symbol not defined in lib: Nope
    use lib.Nope
    ^^^^^^^^^^^^
    fun main() void {}
```

**private symbol cannot be imported**

```metall
use lib.internal_helper
fun main() void {}
```

```error
test.met:1:1: lib::internal_helper is not public
    use lib.internal_helper
    ^^^^^^^^^^^^^^^^^^^^^^^
    fun main() void {}
```

**a bare-use re-export is not accessible**

```metall
use reexport
fun main() void {
    _ = reexport.Secret(1)
}
```

```error
test.met:3:9: reexport::Secret is not public
    fun main() void {
        _ = reexport.Secret(1)
            ^^^^^^^^^^^^^^^
    }
```

**cannot declare a method on an imported type**

```metall
use lib.Point
fun Point.twice(p Point) Int { p.x * 2 }
fun main() void {}
```

```error
test.met:2:5: cannot declare a method on imported symbol `Point`
    use lib.Point
    fun Point.twice(p Point) Int { p.x * 2 }
        ^^^^^^^^^^^
    fun main() void {}
```

**static call through an import respects method visibility**

```metall
use lib.Point
fun main() void {
    let p = Point(1, 2)
    _ = Point.secret(p)
}
```

```error
test.met:4:9: method lib.Point.secret is not public
        let p = Point(1, 2)
        _ = Point.secret(p)
            ^^^^^^^^^^^^
    }
```

**a type imported under a lowercase name**

```metall
use point = lib.Point
fun main() void {}
```

```error
test.met:1:1: a type imported as `point` must be capitalized
    use point = lib.Point
    ^^^^^^^^^^^^^^^^^^^^^
    fun main() void {}
```

**a value imported under a capitalized name**

```metall
use Get = lib.get_lib
fun main() void {}
```

```error
test.met:1:1: a value imported as `Get` must not be capitalized
    use Get = lib.get_lib
    ^^^^^^^^^^^^^^^^^^^^^
    fun main() void {}
```

**import name collides with a local declaration**

```metall
use lib.Point
struct Point { x Int }
fun main() void {}
```

```error
test.met:2:8: symbol already defined: Point
    use lib.Point
    struct Point { x Int }
           ^^^^^
    fun main() void {}
```

**a re-exported generic resolves**

```metall
use reexport
fun main() void {
    let x = reexport.identity(5)
}
```

**a capitalized whole-module rename is rejected**

```metall
use M = lib
fun main() void {}
```

```error
test.met:1:1: a module imported as `M` must not be capitalized
    use M = lib
    ^^^^^^^^^^^
    fun main() void {}
```

**a _test companion cannot re-export a private symbol**

```metall
use priv_test
fun main() void {}
```

```error
lib/priv_test.met:1:5: priv::secret is not public
    pub use priv.secret
        ^^^^^^^^^^^^^^^
```

**import a generic type**

Type arguments apply to the bare imported name.

```metall
use generic.Pair
fun main() void {
    let p = Pair<Int, Str>(1, "hi")
    _ = p.first
}
```

**import a shape as a generic bound**

```metall
use shapes.Showable
pub struct Widget { pub v Int }
pub fun Widget.show(w Widget) Int { w.v }
fun render<T Showable>(x T) Int { x.show() }
fun main() void {
    _ = render(Widget(1))
}
```

**imported type in field, param, and return positions**

```metall
use lib.Point
struct Holder { pub p Point }
fun mid(a Point) Point { a }
fun main() void {
    let h = Holder(mid(Point(1, 2)))
    _ = h.p.x
}
```

**match on an imported enum**

```metall
use lib.IOErr
fun classify(e IOErr) Int {
    match e {
        case IOErr.not_found: 1
        case IOErr.denied: 2
    }
}
fun main() void {
    _ = classify(IOErr.not_found)
}
```

**match on an imported union**

```metall
use lib.Either
fun first(u Either) Int {
    match u {
        case Int n: n
        case Str s: 0
    }
}
fun main() void {
    _ = first(5)
}
```

**static method via a re-export module path**

```metall
use reexport
fun main() void {
    let p = reexport.Point(1, 2)
    _ = reexport.Point.sum(p)
}
```
