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
fun internal_helper() Int { 99 }
pub let public_const = 42
let private_const = 99
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
  hello: local::hello
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
            SimpleType: scope04
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
            SimpleType: scope04
            SimpleType: scope04
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
