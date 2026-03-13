# Module Tests

**setup external modules**

```metall
```

```module.lib
struct Point { x Int y Int }
fun get_lib() Int { 42 }
fun Point.sum(p Point) Int { p.x + p.y }
```

```module.generic
struct Pair<A, B> { first A second B }
fun identity<T>(x T) T { x }
```

```module.local
fun get_hello() Str { "hello" }
```

## OK

**import unused**

```metall
use lib
fun main() void {}
```

**local import unused**

```metall
use local::hello
fun main() void {}
```

**call imported function**

```metall
use lib
fun main() void { print_int(lib::get_lib()) }
```

**call imported function return type**

```metall
use lib
fun main() void { let x = lib::get_lib() print_int(x) }
```

```bindings
Module: scope01
  Import: scope02
  Fun: scope02
    SimpleType: scope03
    Block: scope03
      Var: scope04
        Call: scope04
          Path: scope04
      Call: scope04
        Ident: scope04
        Ident: scope04
---
scope01:
scope02:
  lib: lib
  main: fun01
scope03:
scope04:
  x: Int
fun01 = fun() void
```

**use imported struct**

```metall
use lib
fun main() void { let p = lib::Point(1, 2) print_int(p.x) }
```

```bindings
Module: scope01
  Import: scope02
  Fun: scope02
    SimpleType: scope03
    Block: scope03
      Var: scope04
        TypeConstruction: scope04
          Path: scope04
          Int: scope04
          Int: scope04
      Call: scope04
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
fun01    = fun() void
struct01 = Point { x Int, y Int }
```

**call method on imported struct**

```metall
use lib
fun main() void { let p = lib::Point(1, 2) print_int(p.sum()) }
```

**call method on imported struct via path**

```metall
use lib
fun main() void { let p = lib::Point(1, 2) print_int(lib::Point.sum(p)) }
```

**assign imported function to variable**

```metall
use lib
fun main() void { let f = lib::get_lib print_int(f()) }
```

**local import call**

```metall
use local::hello
fun main() void { let s = hello::get_hello() print_str(s) }
```

```bindings
Module: scope01
  Import: scope02
  Fun: scope02
    SimpleType: scope03
    Block: scope03
      Var: scope04
        Call: scope04
          Path: scope04
      Call: scope04
        Ident: scope04
        Ident: scope04
---
scope01:
scope02:
  hello: local::hello
  main: fun01
scope03:
scope04:
  s: Str
fun01 = fun() void
```

**aliased import**

```metall
use l = lib
fun main() void { print_int(l::get_lib()) }
```

**generic function from import**

```metall
use generic
fun main() void { let x = generic::identity<Int>(42) print_int(x) }
```

```bindings
Module: scope01
  Import: scope02
  Fun: scope02
    SimpleType: scope03
    Block: scope03
      Var: scope04
        Call: scope04
          Path: scope04
            SimpleType: scope04
          Int: scope04
      Call: scope04
        Ident: scope04
        Ident: scope04
---
scope01:
scope02:
  generic: generic
  main: fun01
scope03:
scope04:
  x: Int
fun01 = fun() void
```

**generic struct from import**

```metall
use generic
fun main() void {
    let p = generic::Pair<Int, Str>(1, "hi")
    print_int(p.first)
    print_str(p.second)
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
          Path: scope04
            SimpleType: scope04
            SimpleType: scope04
          Int: scope04
          String: scope04
      Call: scope04
        Ident: scope04
        FieldAccess: scope04
          Ident: scope04
      Call: scope04
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
fun01    = fun() void
struct01 = Pair<Int, Str> { first Int, second Str }
```

## Error

**unknown symbol in import**

```metall
use lib
fun main() void { lib::unknown() }
```

```error
test.met:2:19: symbol not defined in lib: unknown
    use lib
    fun main() void { lib::unknown() }
                      ^^^^^^^^^^^^
```

**wrong arg type to imported function**

```metall
use lib
fun main() void { lib::get_lib("oops") }
```

```error
test.met:2:19: argument count mismatch: expected 0, got 1
    use lib
    fun main() void { lib::get_lib("oops") }
                      ^^^^^^^^^^^^^^^^^^^^
```

**generic struct wrong type arg count**

```metall
use generic
fun main() void { generic::Pair<Int>(1) }
```

```error
test.met:2:19: type argument count mismatch: expected 2, got 1
    use generic
    fun main() void { generic::Pair<Int>(1) }
                      ^^^^^^^^^^^^^^^^^^
```

**generic function wrong type arg count**

```metall
use generic
fun main() void { generic::identity<Int, Str>(42) }
```

```error
test.met:2:19: type argument count mismatch: expected 1, got 2
    use generic
    fun main() void { generic::identity<Int, Str>(42) }
                      ^^^^^^^^^^^^^^^^^^^^^^^^^^^
```

**nested module access**

```metall
use lib
fun main() void { lib::sub::foo() }
```

```error
test.met:2:19: invalid module path
    use lib
    fun main() void { lib::sub::foo() }
                      ^^^^^^^^^^^^^
```

**dot syntax on module**

```metall
use lib
fun main() void { lib.get_lib() }
```

```error
test.met:2:19: cannot access field on non-struct type: lib
    use lib
    fun main() void { lib.get_lib() }
                      ^^^
```
