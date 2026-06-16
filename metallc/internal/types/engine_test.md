# Type Checker Tests

## Basics

**Int literal**

```metall
123
```

```types
Int: Int
```

**Str literal**

```metall
"hello"
```

```types
String: Str
```

**Bytes literal**

```metall
b"abc"
```

```types
String: []U8
```

**Mut Str binding can be rebound**

```metall
{ mut s = "hello"  s = "world" }
```

```error
```

**Rune literal**

```metall
'a'
```

```types
RuneLiteral: Rune
```

**Rune literal unicode**

```metall
'é'
```

```types
RuneLiteral: Rune
```

**Block**

```metall
{ _ = 123 "hello" }
```

```types
Block: Str
  Assign: void
    Ident: ?
    Int: Int
  String: Str
```

**Empty block is void**

```metall
{ }
```

```types
Block: void
```

**Let binding**

```metall
let x = 123
```

```types
Var: void
  Int: Int
```

```bindings
Var: scope01
  Int: scope01
---
scope01:
  x: Int
```

**Mut binding**

```metall
mut x = 123
```

```types
Var: void
  Int: Int
```

```bindings
Var: scope01
  Int: scope01
---
scope01:
  x: Int (mut)
```

**Type annotation coerces &mut to &ref**

The annotated type is used for the binding, not the inferred expression type.

```metall
{ mut x = 1 let y &Int = &mut x }
```

```types
Block: void
  Var: void
    Int: Int
  Var: void
    RefType: &Int
      SimpleType: Int
    Ref: &mut Int
      Ident: Int
```

```bindings
Block: scope01
  Var: scope02
    Int: scope02
  Var: scope02
    RefType: scope02
      SimpleType: scope02
    Ref: scope02
      Ident: scope02
---
scope01:
scope02:
  x: Int (mut)
  y: &Int
```

**Assign is void**

```metall
{ mut x = 321 x = 123 }
```

```types
Block: void
  Var: void
    Int: Int
  Assign: void
    Ident: Int
    Int: Int
```

**Discard void**

```metall
_ = void
```

```types
Assign: void
  Ident: ?
  Ident: void
```

**Assign void to variable**

```metall
let x = void
```

```types
Var: void
  Ident: void
```

```bindings
Var: scope01
  Ident: scope01
---
scope01:
  x: void
```

**Pass void as function argument**

```metall
{ fun foo(a void) Int { 42 } foo(void) }
```

```types
Block: Int
  Fun: fun01
    FunParam: void
      SimpleType: void
    SimpleType: Int
    Block: Int
      Int: Int
  Call: Int
    Ident: fun01
    Ident: void
---
fun01 = sync fun(void) Int
```

**Return void from function**

```metall
{ fun foo() void { void } foo() }
```

```types
Block: void
  Fun: fun01
    SimpleType: void
    Block: void
      Ident: void
  Call: void
    Ident: fun01
---
fun01 = sync fun() void
```

**Void in if-else expression**

```metall
{ let x = if true { void } else { void } }
```

```types
Block: void
  Var: void
    If: void
      Bool: Bool
      Block: void
        Ident: void
      Block: void
        Ident: void
```

## Functions

**Generic function with same signature as non-generic function**

```metall
{
    fun bar(x Int) void {}
    fun foo<T>(x Int) void {}
    foo<Str>(1)
}
```

```types
Block: void
  Fun: fun01
    FunParam: Int
      SimpleType: Int
    SimpleType: void
    Block: void
  Fun: fun01
    TypeParam: T
    FunParam: Int
      SimpleType: Int
    SimpleType: void
    Block: void
  Call: void
    Ident: fun02
    Int: Int
---
fun01 = sync fun(Int) void
fun02 = fun(Int) void
```

**Fun declaration**

```metall
fun foo(a Int, b Str) Int { 123 }
```

```types
Fun: fun01
  FunParam: Int
    SimpleType: Int
  FunParam: Str
    SimpleType: Str
  SimpleType: Int
  Block: Int
    Int: Int
---
fun01 = sync fun(Int, Str) Int
```

**Fun params**

```metall
fun foo(a Int) Int { a }
```

```types
Fun: fun01
  FunParam: Int
    SimpleType: Int
  SimpleType: Int
  Block: Int
    Ident: Int
---
fun01 = sync fun(Int) Int
```

**Fun params are scoped to the fun**

```metall
{ fun foo(a Int) void {} fun bar(a Int) void {} }
```

```types
Block: fun01
  Fun: fun01
    FunParam: Int
      SimpleType: Int
    SimpleType: void
    Block: void
  Fun: fun01
    FunParam: Int
      SimpleType: Int
    SimpleType: void
    Block: void
---
fun01 = sync fun(Int) void
```

**Fun call**

```metall
{ fun foo(a Int) Int { 123 } foo(321) }
```

```types
Block: Int
  Fun: fun01
    FunParam: Int
      SimpleType: Int
    SimpleType: Int
    Block: Int
      Int: Int
  Call: Int
    Ident: fun01
    Int: Int
---
fun01 = sync fun(Int) Int
```

**Call void fun**

```metall
{ fun foo() void { } foo() }
```

```types
Block: void
  Fun: fun01
    SimpleType: void
    Block: void
  Call: void
    Ident: fun01
---
fun01 = sync fun() void
```

**Builtin print_str**

```metall
DebugIntern.print_str("hello")
```

```types
Call: void
  Ident: fun01
  String: Str
  Int: Int
  Bool: Bool
---
fun01 = sync fun(Str, Int, Bool) void
```

**Builtin print_int**

```metall
DebugIntern.print_int(123)
```

```types
Call: void
  Ident: fun01
  Int: Int
  Int: Int
  Bool: Bool
---
fun01 = sync fun(Int, Int, Bool) void
```

**Builtin print_bool**

```metall
DebugIntern.print_bool(true)
```

```types
Call: void
  Ident: fun01
  Bool: Bool
  Int: Int
  Bool: Bool
---
fun01 = sync fun(Bool, Int, Bool) void
```

**Shadowing**

```metall
{ let x = { let x = "hello" 123 } }
```

```bindings
Block: scope01
  Var: scope02
    Block: scope02
      Var: scope03
        String: scope03
      Int: scope03
---
scope01:
scope02:
  x: Int
scope03:
  x: Str
```

**Return**

```metall
fun foo() Int { return 1 }
```

```types
Fun: fun01
  SimpleType: Int
  Block: never
    Return: never
      Int: Int
---
fun01 = sync fun() Int
```

**Return void**

```metall
fun foo() void { return void }
```

```types
Fun: fun01
  SimpleType: void
  Block: never
    Return: never
      Ident: void
---
fun01 = sync fun() void
```

**Return expr type is void**

```metall
fun foo() Int { return 123 }
```

```types
Fun: fun01
  SimpleType: Int
  Block: never
    Return: never
      Int: Int
---
fun01 = sync fun() Int
```

**Fun type**

```metall
fun foo(bar fun(Int) Str) void {}
```

```types
Fun: fun01
  FunParam: fun02
    FunType: fun02
      SimpleType: Int
      SimpleType: Str
  SimpleType: void
  Block: void
---
fun02 = fun(Int) Str
fun01 = fun(fun02) void
```

**Fun type identity**

```metall
{
    fun foo(a Int) Str { "x" }
    fun bar(a Int) Str { "y" }
    fun apply(f fun(Int) Str, g fun(Int) Str, h fun(fun(Int) Str) Bool) void {}
}
```

```types
Block: fun01
  Fun: fun04
    FunParam: Int
      SimpleType: Int
    SimpleType: Str
    Block: Str
      String: Str
  Fun: fun04
    FunParam: Int
      SimpleType: Int
    SimpleType: Str
    Block: Str
      String: Str
  Fun: fun01
    FunParam: fun02
      FunType: fun02
        SimpleType: Int
        SimpleType: Str
    FunParam: fun02
      FunType: fun02
        SimpleType: Int
        SimpleType: Str
    FunParam: fun03
      FunType: fun03
        FunType: fun02
          SimpleType: Int
          SimpleType: Str
        SimpleType: Bool
    SimpleType: void
    Block: void
---
fun02 = fun(Int) Str
fun03 = fun(fun02) Bool
fun01 = fun(fun02, fun02, fun03) void
fun04 = sync fun(Int) Str
```

**Named fun assignable to fun-type**

```metall
{
    fun foo(a Int) Int { a }
    fun bar(f fun(Int) Int) Int { f(0) }
    bar(foo)
}
```

```types
Block: Int
  Fun: fun01
    FunParam: Int
      SimpleType: Int
    SimpleType: Int
    Block: Int
      Ident: Int
  Fun: fun02
    FunParam: fun03
      FunType: fun03
        SimpleType: Int
        SimpleType: Int
    SimpleType: Int
    Block: Int
      Call: Int
        Ident: fun03
        Int: Int
  Call: Int
    Ident: fun02
    Ident: fun01
---
fun01 = sync fun(Int) Int
fun03 = fun(Int) Int
fun02 = fun(fun03) Int
```

**If branches with different named funs**

```metall
{
    fun double(a Int) Int { a + a }
    fun triple(a Int) Int { a + a + a }
    if true { double } else { triple }
}
```

```types
Block: fun01
  Fun: fun01
    FunParam: Int
      SimpleType: Int
    SimpleType: Int
    Block: Int
      Binary: Int
        Ident: Int
        Ident: Int
  Fun: fun01
    FunParam: Int
      SimpleType: Int
    SimpleType: Int
    Block: Int
      Binary: Int
        Binary: Int
          Ident: Int
          Ident: Int
        Ident: Int
  If: fun01
    Bool: Bool
    Block: fun01
      Ident: fun01
    Block: fun01
      Ident: fun01
---
fun01 = sync fun(Int) Int
```

**Nested function cannot reference outer variable**

```metall
{
    let x = 1
    _ = fun(a Int) Int { x + a }
    fun bar(a Int) Int { x + a }
}
```

```error
test.met:4:26: symbol not defined: x
        _ = fun(a Int) Int { x + a }
        fun bar(a Int) Int { x + a }
                             ^
    }

test.met:3:26: cannot reference "x" from outer scope
        let x = 1
        _ = fun(a Int) Int { x + a }
                             ^
        fun bar(a Int) Int { x + a }
```

**Nested function cannot reference outer function parameter**

```metall
fun foo(a Int) fun(Int) Int {
    _ = fun(b Int) Int { a + b }
    fun bar(b Int) Int { a + b }
}
```

```error
test.met:3:26: cannot reference "a" from outer scope
        _ = fun(b Int) Int { a + b }
        fun bar(b Int) Int { a + b }
                             ^
    }

test.met:2:26: cannot reference "a" from outer scope
    fun foo(a Int) fun(Int) Int {
        _ = fun(b Int) Int { a + b }
                             ^
        fun bar(b Int) Int { a + b }
```

**Nested function cannot reference outer allocator**

```metall
{
    let @a = Arena()
    _ = fun() void { @a }
    fun bar() void { @a }
}
```

```error
test.met:4:22: symbol not defined: @a
        _ = fun() void { @a }
        fun bar() void { @a }
                         ^^
    }

test.met:3:22: cannot reference "@a" from outer scope
        let @a = Arena()
        _ = fun() void { @a }
                         ^^
        fun bar() void { @a }
```

**Nested function can reference outer function and struct**

```metall
{
    fun double(x Int) Int { x + x }
    struct Foo { x Int }
    _ = fun(a Int) Int { double(a) }
    fun bar(a Int) Int { Foo(double(a)).x }
}
```

```types
Block: fun01
  Fun: fun01
    FunParam: Int
      SimpleType: Int
    SimpleType: Int
    Block: Int
      Binary: Int
        Ident: Int
        Ident: Int
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Assign: void
    Ident: ?
    Block: fun01
      Fun: fun01
        FunParam: Int
          SimpleType: Int
        SimpleType: Int
        Block: Int
          Call: Int
            Ident: fun01
            Ident: Int
      Ident: fun01
  Fun: fun01
    FunParam: Int
      SimpleType: Int
    SimpleType: Int
    Block: Int
      FieldAccess: Int
        TypeConstruction: struct01
          Ident: struct01
          Call: Int
            Ident: fun01
            Ident: Int
---
fun01    = sync fun(Int) Int
struct01 = Foo { x Int }
```

## Closures

**Closure captures variable by value**

```metall
{ let x = 1 _ = fun[x]() Int { x } }
```

```error
```

**Closure captures variable by reference**

```metall
{ let x = 1 _ = fun[&x]() &Int { x } }
```

```error
```

**Closure captures variable by mutable reference**

```metall
{ mut x = 1 _ = fun[&mut x]() &mut Int { x } }
```

```error
```

**Closure without captures is still allowed**

```metall
{ _ = fun[](x Int) Int { x } }
```

```error
```

**Closure capture used in arithmetic**

```metall
{ let n = 1 _ = fun[n](x Int) Int { n + x } }
```

```error
```

**Closure cannot capture mutable ref to immutable binding**

```metall
{ let x = 1 _ = fun[&mut x]() void { } }
```

```error
test.met:1:26: cannot take mutable reference to immutable value
    { let x = 1 _ = fun[&mut x]() void { } }
                             ^
```

**Closure capture of undefined variable**

```metall
{ _ = fun[y]() Int { y } }
```

```error
test.met:1:11: capture: symbol not defined: y
    { _ = fun[y]() Int { y } }
              ^
test.met:1:22: symbol not defined: y
    { _ = fun[y]() Int { y } }
                         ^
```

**Closure captures allocator by value**

```metall
fun foo() void {
    let @a = Arena()
    _ = fun[@a]() []Int { @a.slice<Int>(1, 0) }
}
```

```error
```

**Closure cannot capture allocator by reference**

```metall
fun foo() void {
    let @a = Arena()
    _ = fun[&@a]() void { }
}
```

```error
test.met:3:14: allocator captures must be by value, not by reference
        let @a = Arena()
        _ = fun[&@a]() void { }
                 ^^
    }
```

**Closure cannot capture allocator by mutable reference**

```metall
fun foo() void {
    let @a = Arena()
    _ = fun[&mut @a]() void { }
}
```

```error
test.met:3:18: allocator captures must be by value, not by reference
        let @a = Arena()
        _ = fun[&mut @a]() void { }
                     ^^
    }
```

## Inferred function literal types

**Infer param type from call site**

```metall
{
    fun apply(f fun(Int) Int) Int { f(1) }
    _ = apply(fun(a) { a })
}
```

```error
```

**Infer return type from call site**

```metall
{
    fun apply(f fun(Int) Int) Int { f(1) }
    _ = apply(fun(a Int) { a })
}
```

```error
```

**Infer both param and return type from call site**

```metall
{
    fun apply(f fun(Int) Int) Int { f(1) }
    _ = apply(fun(a) { a })
}
```

```error
```

**Infer types from variable declaration**

```metall
{
    let f fun(Int) Int = fun(a) { a }
    _ = f
}
```

```error
```

**Infer param types with multiple params**

```metall
{
    fun apply(f fun(Int, Str) Bool) Bool { f(1, "hi") }
    _ = apply(fun(a, b) { true })
}
```

```error
```

**Infer param types with mixed explicit and inferred**

```metall
{
    fun apply(f fun(Int, Str) Bool) Bool { f(1, "hi") }
    _ = apply(fun(a, b Str) { true })
}
```

```error
```

**Infer only return type, params explicit**

```metall
{
    fun apply(f fun(Int) Int) Int { f(1) }
    _ = apply(fun(a Int) { a + 1 })
}
```

```error
```

**Cannot infer param type without type hint**

```metall
{
    _ = fun(a) { a }
}
```

```error
test.met:2:13: cannot infer type of parameter 'a'
    {
        _ = fun(a) { a }
                ^
    }
```

**Return type requires annotation without type hint**

```metall
{
    _ = fun(a Int) { a }
}
```

```error
test.met:2:9: cannot infer return type of function literal
    {
        _ = fun(a Int) { a }
            ^^^^^^^^^^^^^^^^
    }
```

**Infer with closure captures**

```metall
{
    fun apply(f fun(Int) Int) Int { f(1) }
    let n = 10
    _ = apply(fun[n](a) { n + a })
}
```

```error
```

**Infer param types in generic function call (filter pattern)**

```metall
{
    fun filter<T>(items []T, predicate fun(T) Bool) Bool { predicate(items[0]) }
    _ = filter(['a', 'b'][..], fun(a) { a == 'a' })
}
```

```error
```

**Infer param and return types in generic function call (map pattern)**

```metall
{
    fun transform<A, B>(x A, f fun(A) B) B { f(x) }
    _ = transform(42, fun(a) { a + 1 })
}
```

```error
```

**Infer with multiple generic type params from receiver and fun lit**

```metall
{
    fun fold<T, R>(items []T, init R, f fun(R, T) R) R { f(init, items[0]) }
    _ = fold(['a', 'b'][..], U32(0), fun(acc, elem) { acc + elem.to_u32() })
}
```

```error
```

**Infer in struct construction**

```metall
struct Wrapper {
    f fun(Int) Int
}
fun foo() void {
    let w = Wrapper { f: fun(a) { a } }
    _ = w
}
```

```error
```

## Nocopy

**Nocopy struct construction is allowed**

```metall module
nocopy struct Handle { id Int }
fun foo() void {
    let h = Handle(1)
    _ = h
}
```

```error
```

**Cannot copy nocopy value to another variable**

```metall module
nocopy struct Handle { id Int }
fun foo() void {
    let h = Handle(1)
    let h2 = h
    _ = h2
}
```

```error
test.met:4:14: cannot copy value of nocopy type test.Handle
        let h = Handle(1)
        let h2 = h
                 ^
        _ = h2

test.met:5:9: symbol not defined: h2
        let h2 = h
        _ = h2
            ^^
    }
```

**Cannot pass nocopy by value to function**

```metall module
nocopy struct Handle { id Int }
fun take(h Handle) void { _ = h }
fun foo() void {
    let h = Handle(1)
    take(h)
}
```

```error
test.met:5:10: cannot copy value of nocopy type test.Handle
        let h = Handle(1)
        take(h)
             ^
    }
```

**Can pass nocopy by reference**

```metall module
nocopy struct Handle { id Int }
fun use_ref(h &Handle) Int { h.id }
fun foo() void {
    let h = Handle(1)
    _ = use_ref(&h)
}
```

```error
```

**Can receive nocopy from function call**

```metall module
nocopy struct Handle { id Int }
fun make_handle() Handle { Handle(1) }
fun foo() void {
    let h = make_handle()
    _ = h
}
```

```error
```

**Reassign nocopy from construction is allowed**

```metall module
nocopy struct Handle { id Int }
fun foo() void {
    mut h = Handle(1)
    h = Handle(2)
    _ = h
}
```

```error
```

**Cannot reassign nocopy from existing binding**

```metall module
nocopy struct Handle { id Int }
fun foo() void {
    let a = Handle(1)
    mut h = Handle(2)
    h = a
    _ = h
}
```

```error
test.met:5:9: cannot copy value of nocopy type test.Handle
        mut h = Handle(2)
        h = a
            ^
        _ = h
```

**Struct containing nocopy field is also nocopy**

```metall module
nocopy struct Handle { id Int }
struct Wrapper { h Handle }
fun foo() void {
    let w = Wrapper(Handle(1))
    let w2 = w
    _ = w2
}
```

```error
test.met:5:14: cannot copy value of nocopy type test.Wrapper
        let w = Wrapper(Handle(1))
        let w2 = w
                 ^
        _ = w2

test.met:6:9: symbol not defined: w2
        let w2 = w
        _ = w2
            ^^
    }
```

**Nocopy union variant makes whole union nocopy**

```metall module
nocopy struct Handle { id Int }
union Resource = Handle | Int
fun foo() void {
    let r = Resource(1)
    let r2 = r
    _ = r2
}
```

```error
test.met:5:14: cannot copy value of nocopy type test.Resource
        let r = Resource(1)
        let r2 = r
                 ^
        _ = r2

test.met:6:9: symbol not defined: r2
        let r2 = r
        _ = r2
            ^^
    }
```

**Can construct a struct from a fresh nocopy value**

Moving a freshly constructed nocopy value into a field is not a copy.

```metall module
nocopy struct Handle { id Int }
struct Wrapper { h Handle }
fun foo() void {
    let w = Wrapper(Handle(1))
    _ = w
}
```

```error
```

**Cannot construct a struct from a nocopy binding**

Passing an existing nocopy binding as a construction argument copies it, the same
as passing it to a function.

```metall module
nocopy struct Handle { id Int }
struct Wrapper { h Handle }
fun foo() void {
    let h = Handle(1)
    let w = Wrapper(h)
    _ = w
}
```

```error
test.met:5:21: cannot copy value of nocopy type test.Handle
        let h = Handle(1)
        let w = Wrapper(h)
                        ^
        _ = w

test.met:6:9: symbol not defined: w
        let w = Wrapper(h)
        _ = w
            ^
    }
```

**Cannot construct a union from a nocopy binding**

```metall module
nocopy struct Handle { id Int }
union Resource = Handle | Int
fun foo() void {
    let h = Handle(1)
    let r = Resource(h)
    _ = r
}
```

```error
test.met:5:22: cannot copy value of nocopy type test.Handle
        let h = Handle(1)
        let r = Resource(h)
                         ^
        _ = r

test.met:6:9: symbol not defined: r
        let r = Resource(h)
        _ = r
            ^
    }
```

**Cannot copy a nocopy binding into an array literal**

```metall module
nocopy struct Handle { id Int }
fun foo() void {
    let h = Handle(1)
    let s = [h, h][..]
}
```

```error
test.met:4:14: cannot copy value of nocopy type test.Handle
        let h = Handle(1)
        let s = [h, h][..]
                 ^
    }
```

**Can build an array literal from fresh nocopy values**

Freshly constructed nocopy elements are moved into the array, not copied.

```metall module
nocopy struct Handle { id Int }
fun foo() void {
    let s = [Handle(1), Handle(2)][..]
}
```

```error
```

**Cannot capture a nocopy value by value**

A by-value closure capture copies the captured binding while the original stays
live, so it is rejected for a nocopy value.

```metall module
nocopy struct Handle { id Int }
fun foo() void {
    let h = Handle(1)
    let f = fun[h]() Int { h.id }
}
```

```error
test.met:4:17: cannot copy value of nocopy type test.Handle
        let h = Handle(1)
        let f = fun[h]() Int { h.id }
                    ^
    }
```

**Can capture a nocopy value by reference**

```metall module
nocopy struct Handle { id Int }
fun foo() void {
    let h = Handle(1)
    let f = fun[&h]() Int { h.id }
}
```

```error
```

**Cannot bind a nocopy element by value in for-in**

`for v in xs` copies each element out of the iterand; iterate by reference instead.

```metall module
nocopy struct Handle { id Int }
fun foo(xs []Handle) void {
    for v in xs {
    }
}
```

```error
test.met:3:9: cannot copy value of nocopy type test.Handle; iterate by reference with `for &`
    fun foo(xs []Handle) void {
        for v in xs {
            ^
        }
```

**Can bind a nocopy element by reference in for-in**

```metall module
nocopy struct Handle { id Int }
fun foo(xs []Handle) void {
    for &v in xs {
    }
}
```

```error
```

## Defer

**Defer block is void**

```metall
{ defer { } }
```

```types
Block: void
  Defer: void
    Block: void
```

**Defer block with non-void result**

```metall
{ defer { 1 } }
```

```error
test.met:1:3: defer block must not yield a value, got Int
    { defer { 1 } }
      ^^^^^^^^^^^
```

**Defer with return is rejected**

```metall module
fun main() void {
    defer { return void }
}
```

```error
test.met:2:5: defer block cannot transfer control: no return, break, continue, or try
    fun main() void {
        defer { return void }
        ^^^^^^^^^^^^^^^^^^^^^
    }
```

**Defer with try is rejected**

```metall module
fun fallible() !Int {
    5
}

fun main() void {
    defer { _ = try fallible() }
}
```

```error
test.met:6:5: defer block cannot transfer control: no return, break, continue, or try
    fun main() void {
        defer { _ = try fallible() }
        ^^^^^^^^^^^^^^^^^^^^^^^^^^^^
    }
```

**Defer with a break escaping the defer is rejected**

```metall module
fun main() void {
    for {
        defer { break }
    }
}
```

```error
test.met:3:9: defer block cannot transfer control: no return, break, continue, or try
        for {
            defer { break }
            ^^^^^^^^^^^^^^^
        }
```

**Defer with a loop containing break is allowed**

```metall module
fun main() void {
    defer { for { break } }
}
```

```error
```

**Defer with a return nested in an if is rejected**

```metall module
fun pick(ok Bool) void {
    defer {
        if ok { return void }
        DebugIntern.print_int(1)
    }
}
```

```error
test.met:2:5: defer block cannot transfer control: no return, break, continue, or try
    fun pick(ok Bool) void {
        defer {
        ^
            if ok { return void }
            DebugIntern.print_int(1)
        }
        ^
    }
```

**Defer with a return in a nested closure is allowed**

```metall module
fun main() void {
    defer {
        let g = fun() void { return void }
        g()
    }
}
```

```error
```

## Structs

**Struct declaration**

```metall
struct Foo { one Str two Int }
```

```types
Struct: struct01
  StructField: ?
    SimpleType: ?
  StructField: ?
    SimpleType: ?
---
struct01 = Foo { one Str, two Int }
```

**Forward declare struct type**

```metall
{ fun foo(a Foo) void {} struct Foo { one Str } }
```

```types
Block: struct01
  Fun: fun01
    FunParam: struct01
      SimpleType: struct01
    SimpleType: void
    Block: void
  Struct: struct01
    StructField: ?
      SimpleType: ?
---
struct01 = Foo { one Str }
fun01    = sync fun(struct01) void
```

**Field referencing a struct that fails to complete**

A field naming a struct completed after this one shares that struct's cached
type. It must not be flipped to ok before the named struct's own completion
fails on the unknown field type.

```metall
{
    struct A {
        b B
    }
    struct B {
        g DoesNotExist
    }
}
```

```error
test.met:6:11: symbol not defined: DoesNotExist
        struct B {
            g DoesNotExist
              ^^^^^^^^^^^^
        }
```

**Struct construction**

```metall
{ struct Foo { one Str two Int } let x = Foo("hello", 123) x }
```

```types
Block: struct01
  Struct: struct01
    StructField: ?
      SimpleType: ?
    StructField: ?
      SimpleType: ?
  Var: void
    TypeConstruction: struct01
      Ident: struct01
      String: Str
      Int: Int
  Ident: struct01
---
struct01 = Foo { one Str, two Int }
```

**Struct ref**

```metall
{
    struct Foo { one Str }
    let x = Foo("hello")
    &x
}
```

```types
Block: &struct01
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Var: void
    TypeConstruction: struct01
      Ident: struct01
      String: Str
  Ref: &struct01
    Ident: struct01
---
struct01 = Foo { one Str }
```

**Field read access**

```metall
{ struct Foo { one Str } let x = Foo("hello") x.one }
```

```types
Block: Str
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Var: void
    TypeConstruction: struct01
      Ident: struct01
      String: Str
  FieldAccess: Str
    Ident: struct01
---
struct01 = Foo { one Str }
```

**Field write access**

```metall
{ struct Foo { one Str } mut x = Foo("hello") x.one = "bye" }
```

```types
Block: void
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Var: void
    TypeConstruction: struct01
      Ident: struct01
      String: Str
  Assign: void
    FieldAccess: Str
      Ident: struct01
    String: Str
---
struct01 = Foo { one Str }
```

**Field write through mut ref param**

```metall
{ struct Foo { one Str } fun foo(a &mut Foo) void { a.one = "X" } mut x = Foo("hello") foo(&mut x) }
```

```types
Block: void
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Fun: fun01
    FunParam: &mut struct01
      RefType: &mut struct01
        SimpleType: struct01
    SimpleType: void
    Block: void
      Assign: void
        FieldAccess: Str
          Ident: &mut struct01
        String: Str
  Var: void
    TypeConstruction: struct01
      Ident: struct01
      String: Str
  Call: void
    Ident: fun01
    Ref: &mut struct01
      Ident: struct01
---
struct01 = Foo { one Str }
fun01    = fun(&mut struct01) void
```

**Nested field write on mut struct**

```metall
{ struct Foo { one Int } struct Bar { one Foo } mut x = Bar(Foo(1)) x.one.one = 2 }
```

```types
Block: void
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Struct: struct02
    StructField: ?
      SimpleType: ?
  Var: void
    TypeConstruction: struct02
      Ident: struct02
      TypeConstruction: struct01
        Ident: struct01
        Int: Int
  Assign: void
    FieldAccess: Int
      FieldAccess: struct01
        Ident: struct02
    Int: Int
---
struct01 = Foo { one Int }
struct02 = Bar { one struct01 }
```

**Field write through let binding of mut ref**

```metall
{ struct Foo { one Str } mut x = Foo("hello") let y = &mut x y.one = "X" }
```

```types
Block: void
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Var: void
    TypeConstruction: struct01
      Ident: struct01
      String: Str
  Var: void
    Ref: &mut struct01
      Ident: struct01
  Assign: void
    FieldAccess: Str
      Ident: &mut struct01
    String: Str
---
struct01 = Foo { one Str }
```

## Unions

**Union declaration**

```metall
union Foo = Str | Int
```

```types
Union: union01
  SimpleType: ?
  SimpleType: ?
---
union01 = Foo = Str | Int
```

**Union three variants**

```metall
union Foo = Str | Int | Bool
```

```types
Union: union01
  SimpleType: ?
  SimpleType: ?
  SimpleType: ?
---
union01 = Foo = Str | Int | Bool
```

**Forward declare union type**

```metall
{ fun foo(a Foo) void {} union Foo = Str | Int }
```

```types
Block: union01
  Fun: fun01
    FunParam: union01
      SimpleType: union01
    SimpleType: void
    Block: void
  Union: union01
    SimpleType: ?
    SimpleType: ?
---
union01 = Foo = Str | Int
fun01   = sync fun(union01) void
```

**Union with struct variant**

```metall
{
    struct Bar { one Int }
    union Foo = Str | Bar
    fun foo(a Foo) void {}
}
```

```types
Block: fun01
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Union: union01
    SimpleType: ?
    SimpleType: ?
  Fun: fun01
    FunParam: union01
      SimpleType: union01
    SimpleType: void
    Block: void
---
struct01 = Bar { one Int }
union01  = Foo = Str | struct01
fun01    = sync fun(union01) void
```

**Union with ref variant**

```metall
{
    union Foo = &Int | Str
    fun foo(a Foo) void {}
}
```

```types
Block: fun01
  Union: union01
    RefType: ?
      SimpleType: ?
    SimpleType: ?
  Fun: fun01
    FunParam: union01
      SimpleType: union01
    SimpleType: void
    Block: void
---
union01 = Foo = &Int | Str
fun01   = fun(union01) void
```

**Generic union**

```metall
{
    union Maybe<T> = T | Bool
    fun foo(a Maybe<Int>) void {}
}
```

```types
Block: fun01
  Union: union02
    TypeParam: ?
    SimpleType: ?
    SimpleType: ?
  Fun: fun01
    FunParam: union01
      SimpleType: union01
    SimpleType: void
    Block: void
---
union01 = Maybe<Int> = Int | Bool
fun01   = sync fun(union01) void
union02 = Maybe = T | Bool
```

**Generic union identity and distinctness**

```metall
{
    union Maybe<T> = T | Bool
    fun foo(a Maybe<Int>) void {}
    fun bar(a Maybe<Int>) void {}
    fun baz(a Maybe<Str>) void {}
}
```

```types
Block: fun01
  Union: union02
    TypeParam: ?
    SimpleType: ?
    SimpleType: ?
  Fun: fun02
    FunParam: union03
      SimpleType: union03
    SimpleType: void
    Block: void
  Fun: fun02
    FunParam: union03
      SimpleType: union03
    SimpleType: void
    Block: void
  Fun: fun01
    FunParam: union01
      SimpleType: union01
    SimpleType: void
    Block: void
---
union01 = Maybe<Str> = Str | Bool
fun01   = sync fun(union01) void
union02 = Maybe = T | Bool
union03 = Maybe<Int> = Int | Bool
fun02   = sync fun(union03) void
```

**Method on union**

```metall
{
    union Foo = Str | Int
    fun Foo.is_str(f &Foo) Bool { true }
    fun test(f &Foo) Bool { f.is_str() }
}
```

```types
Block: fun01
  Union: union01
    SimpleType: ?
    SimpleType: ?
  Fun: fun01
    FunParam: &union01
      RefType: &union01
        SimpleType: union01
    SimpleType: Bool
    Block: Bool
      Bool: Bool
  Fun: fun01
    FunParam: &union01
      RefType: &union01
        SimpleType: union01
    SimpleType: Bool
    Block: Bool
      Call: Bool
        FieldAccess: fun01
          Ident: &union01
---
union01 = Foo = Str | Int
fun01   = fun(&union01) Bool
```

**Forward declare union with struct after**

```metall
{
    union Foo = Str | Bar
    struct Bar { one Int }
    fun test(f Foo, b Bar) Int { b.one }
}
```

```types
Block: fun01
  Union: union01
    SimpleType: ?
    SimpleType: ?
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Fun: fun01
    FunParam: union01
      SimpleType: union01
    FunParam: struct01
      SimpleType: struct01
    SimpleType: Int
    Block: Int
      FieldAccess: Int
        Ident: struct01
---
struct01 = Bar { one Int }
union01  = Foo = Str | struct01
fun01    = sync fun(union01, struct01) Int
```

**Generic union with generic struct variant**

```metall
{
    struct Box<T> { value T }
    union Maybe<T> = T | Box<T>
    fun test(m Maybe<Int>) void {}
}
```

```types
Block: fun01
  Struct: struct02
    TypeParam: ?
    StructField: ?
      SimpleType: ?
  Union: union02
    TypeParam: ?
    SimpleType: ?
    SimpleType: ?
      SimpleType: ?
  Fun: fun01
    FunParam: union01
      SimpleType: union01
    SimpleType: void
    Block: void
---
struct01 = Box<Int> { value Int }
union01  = Maybe<Int> = Int | struct01
fun01    = sync fun(union01) void
struct02 = Box { value T }
struct03 = Box<T> { value T }
union02  = Maybe = T | struct03
```

**Union construction with first variant**

```metall
{
    union Either = Str | Int
    Either("hello")
}
```

```types
Block: union01
  Union: union01
    SimpleType: ?
    SimpleType: ?
  TypeConstruction: union01
    Ident: union01
    String: Str
---
union01 = Either = Str | Int
```

**Union construction with second variant**

```metall
{
    union Foo = Str | Int
    Foo(42)
}
```

```types
Block: union01
  Union: union01
    SimpleType: ?
    SimpleType: ?
  TypeConstruction: union01
    Ident: union01
    Int: Int
---
union01 = Foo = Str | Int
```

**Union construction with struct variant**

```metall
{
    struct Bar { one Int }
    union Foo = Str | Bar
    Foo(Bar(42))
}
```

```types
Block: union01
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Union: union01
    SimpleType: ?
    SimpleType: ?
  TypeConstruction: union01
    Ident: union01
    TypeConstruction: struct01
      Ident: struct01
      Int: Int
---
struct01 = Bar { one Int }
union01  = Foo = Str | struct01
```

**Generic union construction**

```metall
{
    union Maybe<T> = T | Bool
    Maybe<Int>(42)
}
```

```types
Block: union01
  Union: union02
    TypeParam: ?
    SimpleType: ?
    SimpleType: ?
  TypeConstruction: union01
    Ident: union01
    Int: Int
---
union01 = Maybe<Int> = Int | Bool
union02 = Maybe = T | Bool
```

**Union with unknown variant**

```metall
{
    union Foo = Str | DoesNotExist
    fun test(f Foo) Str {
        match f {
            case Str s: s
        }
    }
}
```

```error
test.met:2:23: symbol not defined: DoesNotExist
    {
        union Foo = Str | DoesNotExist
                          ^^^^^^^^^^^^
        fun test(f Foo) Str {
```

**Union with unknown variant referenced by a struct field**

A struct field is completed before the union it names, so the union's shared
cached type must not be flipped to ok before its own completion fails on the
unknown variant.

```metall
{
    union Foo = Str | DoesNotExist
    struct Holder {
        f Foo
    }
}
```

```error
test.met:2:23: symbol not defined: DoesNotExist
    {
        union Foo = Str | DoesNotExist
                          ^^^^^^^^^^^^
        struct Holder {
```

## Union Auto-Wrap

**Auto-wrap in let binding**

The expression `42` has type `Int`, but the binding `x` gets type `Foo` (the union).

```metall
{
    union Foo = Str | Int
    let x Foo = 42
    let y Foo = "hello"
}
```

```bindings
Block: scope01
  Union: scope02
    SimpleType: scope02
    SimpleType: scope02
  Var: scope02
    SimpleType: scope02
    Int: scope02
  Var: scope02
    SimpleType: scope02
    String: scope02
---
scope01:
scope02:
  Foo: union01
  x: union01
  y: union01
union01 = Foo = Str | Int
```

**Auto-wrap in function call**

Arguments `42` and `"hello"` are auto-wrapped to `Foo` at the call site.

```metall
{
    union Foo = Str | Int
    fun check(f Foo) void {}
    check(42)
    check("hello")
}
```

```bindings
Block: scope01
  Union: scope02
    SimpleType: scope02
    SimpleType: scope02
  Fun: scope02
    FunParam: scope03
      SimpleType: scope03
    SimpleType: scope03
    Block: scope03
  Call: scope02
    Ident: scope02
    Int: scope02
  Call: scope02
    Ident: scope02
    String: scope02
---
scope01:
scope02:
  Foo: union01
  check: fun01
scope03:
  f: union01
union01 = Foo = Str | Int
fun01   = sync fun(union01) void
```

**Auto-wrap in return and implicit return**

Both explicit `return` and implicit block result auto-wrap to the union return type.

```metall
{
    union Foo = Str | Int
    fun explicit() Foo { return 42 }
    fun implicit() Foo { "hello" }
}
```

```types
Block: fun01
  Union: union01
    SimpleType: ?
    SimpleType: ?
  Fun: fun01
    SimpleType: union01
    Block: never
      Return: never
        Int: Int
  Fun: fun01
    SimpleType: union01
    Block: union01
      String: Str
---
union01 = Foo = Str | Int
fun01   = sync fun() union01
```

**Auto-wrap in if-else branches**

Each branch independently wraps its variant to the union type.

```metall
{
    union Foo = Str | Int
    fun pick(b Bool) Foo {
        if b { 42 } else { "hello" }
    }
}
```

```types
Block: fun01
  Union: union01
    SimpleType: ?
    SimpleType: ?
  Fun: fun01
    FunParam: Bool
      SimpleType: Bool
    SimpleType: union01
    Block: union01
      If: union01
        Ident: Bool
        Block: union01
          Int: Int
        Block: union01
          String: Str
---
union01 = Foo = Str | Int
fun01   = sync fun(Bool) union01
```

**Auto-wrap in assignment**

Both the initializer and the reassignment auto-wrap to the union.

```metall
{
    union Foo = Str | Int
    fun test() void {
        mut x Foo = 42
        x = "hello"
    }
}
```

```bindings
Block: scope01
  Union: scope02
    SimpleType: scope02
    SimpleType: scope02
  Fun: scope02
    SimpleType: scope03
    Block: scope03
      Var: scope04
        SimpleType: scope04
        Int: scope04
      Assign: scope04
        Ident: scope04
        String: scope04
---
scope01:
scope02:
  Foo: union01
  test: fun01
scope03:
scope04:
  x: union01 (mut)
union01 = Foo = Str | Int
fun01   = sync fun() void
```

**Auto-wrap with generic union**

Wraps through monomorphized generic union.

```metall
{
    union Maybe<T> = T | Bool
    let x Maybe<Int> = 42
    let y Maybe<Str> = "hello"
}
```

```bindings
Block: scope01
  Union: scope02
    TypeParam: scope02
    SimpleType: scope02
    SimpleType: scope02
  Var: scope02
    SimpleType: scope02
    Int: scope02
  Var: scope02
    SimpleType: scope02
    String: scope02
---
scope01:
scope02:
  Maybe: union01
  T: ?
  x: union02
  y: union03
union01 = Maybe = T | Bool
union02 = Maybe<Int> = Int | Bool
union03 = Maybe<Str> = Str | Bool
```

**Auto-wrap does not wrap matching type**

Explicit `Foo(42)` already produces the union; no redundant wrapping.

```metall
{
    union Foo = Str | Int
    let x Foo = Foo(42)
}
```

```bindings
Block: scope01
  Union: scope02
    SimpleType: scope02
    SimpleType: scope02
  Var: scope02
    SimpleType: scope02
    TypeConstruction: scope02
      Ident: scope02
      Int: scope02
---
scope01:
scope02:
  Foo: union01
  x: union01
union01 = Foo = Str | Int
```

**Auto-wrap wrong variant type is rejected**

```metall
{
    union Foo = Str | Int
    let x Foo = true
}
```

```error
test.met:3:17: type mismatch: expected Foo, got Bool
        union Foo = Str | Int
        let x Foo = true
                    ^^^^
    }
```

**Auto-wrap non-union type is not affected**

```metall
{
    let x Int = 42
}
```

```types
Block: void
  Var: void
    SimpleType: Int
    Int: Int
```

**Auto-wrap void as union variant**

```metall
{
    struct MyErr { msg Str }
    union MyResult<T> = T | MyErr
    fun might_fail(ok Bool) MyResult<void> {
        if ok { void } else { MyErr("fail") }
    }
}
```

```bindings
Block: scope01
  Struct: scope02
    StructField: scope03
      SimpleType: scope03
  Union: scope02
    TypeParam: scope02
    SimpleType: scope02
    SimpleType: scope02
  Fun: scope02
    FunParam: scope04
      SimpleType: scope04
    SimpleType: scope04
    Block: scope04
      If: scope05
        Ident: scope05
        Block: scope05
          Ident: scope06
        Block: scope05
          TypeConstruction: scope07
            Ident: scope07
            String: scope07
---
scope01:
scope02:
  MyErr: struct01
  MyResult: union01
  T: ?
  might_fail: fun01
scope03:
scope04:
  ok: Bool
scope05:
scope06:
scope07:
struct01 = MyErr { msg Str }
union01  = MyResult = T | struct01
union02  = MyResult<void> = void | struct01
fun01    = sync fun(Bool) union02
```

## Booleans and If

**Bool true**

```metall
{ true }
```

```types
Block: Bool
  Bool: Bool
```

**Bool false**

```metall
{ false }
```

```types
Block: Bool
  Bool: Bool
```

**If then else**

```metall
{ let x = true if x { 42 } else { 123 }}
```

```types
Block: Int
  Var: void
    Bool: Bool
  If: Int
    Ident: Bool
    Block: Int
      Int: Int
    Block: Int
      Int: Int
```

**If without else**

```metall
{ let x = true if x { 42 } }
```

```types
Block: void
  Var: void
    Bool: Bool
  If: void
    Ident: Bool
    Block: Int
      Int: Int
```

**If with one branch return**

```metall
fun foo() Int { _ = if true { return 123 } else { "hello" } 321 }
```

```types
Fun: fun01
  SimpleType: Int
  Block: Int
    Assign: void
      Ident: ?
      If: Str
        Bool: Bool
        Block: never
          Return: never
            Int: Int
        Block: Str
          String: Str
    Int: Int
---
fun01 = sync fun() Int
```

**If with one branch break**

```metall
fun foo() void { for { _ = if true { break } else { "hello" } } }
```

```types
Fun: fun01
  SimpleType: void
  Block: void
    For: void
      Block: void
        Assign: void
          Ident: ?
          If: Str
            Bool: Bool
            Block: never
              Break: never
            Block: Str
              String: Str
---
fun01 = sync fun() void
```

**If with one branch continue**

```metall
fun foo() void { for { _ = if true { continue } else { "hello" } } }
```

```types
Fun: fun01
  SimpleType: void
  Block: void
    For: void
      Block: void
        Assign: void
          Ident: ?
          If: Str
            Bool: Bool
            Block: never
              Continue: never
            Block: Str
              String: Str
---
fun01 = sync fun() void
```

**If with both branches return**

```metall
fun foo() Int { if true { return 1 } else { return 2 } }
```

```types
Fun: fun01
  SimpleType: Int
  Block: never
    If: never
      Bool: Bool
      Block: never
        Return: never
          Int: Int
      Block: never
        Return: never
          Int: Int
---
fun01 = sync fun() Int
```

**Nested if with all branches return**

```metall
fun foo() Int { if true { if false { return 1 } else { return 2 } } else { return 3 } }
```

```types
Fun: fun01
  SimpleType: Int
  Block: never
    If: never
      Bool: Bool
      Block: never
        If: never
          Bool: Bool
          Block: never
            Return: never
              Int: Int
          Block: never
            Return: never
              Int: Int
      Block: never
        Return: never
          Int: Int
---
fun01 = sync fun() Int
```

**Nested return breaks outer if control flow**

```metall
fun foo(a Int) Int { _ = if true { if a == 0 { return 1 } else { return 2 } } else { "hello" } 321 }
```

```types
Fun: fun01
  FunParam: Int
    SimpleType: Int
  SimpleType: Int
  Block: Int
    Assign: void
      Ident: ?
      If: Str
        Bool: Bool
        Block: never
          If: never
            Binary: Bool
              Ident: Int
              Int: Int
            Block: never
              Return: never
                Int: Int
            Block: never
              Return: never
                Int: Int
        Block: Str
          String: Str
    Int: Int
---
fun01 = sync fun(Int) Int
```

**If value with a diverging branch takes the live branch's type**

```metall
fun pick(ok Bool) Int {
    let x = if ok { 42 } else { return 0 }
    x
}
```

```types
Fun: fun01
  FunParam: Bool
    SimpleType: Bool
  SimpleType: Int
  Block: Int
    Var: void
      If: Int
        Ident: Bool
        Block: Int
          Int: Int
        Block: never
          Return: never
            Int: Int
    Ident: Int
---
fun01 = sync fun(Bool) Int
```

## When

**When expression**

```metall
{
    let x = false
    let y = true
    let z = when {
        case x: 1
        case y: 2
        else: 3
    }
    z
}
```

```types
Block: Int
  Var: void
    Bool: Bool
  Var: void
    Bool: Bool
  Var: void
    When: Int
      Ident: Bool
      Block: Int
        Int: Int
      Ident: Bool
      Block: Int
        Int: Int
      Block: Int
        Int: Int
  Ident: Int
```

**When without else**

```metall
{
    let x = true
    when {
        case x: void
    }
}
```

```types
Block: void
  Var: void
    Bool: Bool
  When: void
    Ident: Bool
    Block: void
      Ident: void
```

**When without else with non-void branch**

```metall
{
    let x = true
    when {
        case x: 42
    }
}
```

```error
test.met:4:17: when branch type mismatch: expected void, got Int
        when {
            case x: 42
                    ^^
        }
```

**When without else assigned to variable**

```metall
{
    let x = true
    let y = when {
        case x: 42
    }
    y
}
```

```error
test.met:4:17: when branch type mismatch: expected void, got Int
        let y = when {
            case x: 42
                    ^^
        }

test.met:6:5: symbol not defined: y
        }
        y
        ^
    }
```

**When without else as return value**

```metall
fun foo() Int {
    when {
        case true: 42
    }
}
```

```error
test.met:3:20: when branch type mismatch: expected void, got Int
        when {
            case true: 42
                       ^^
        }
```

**When without else as function argument**

```metall
{
    fun foo(x Int) Int { 42 }
    foo(when { case true: 1 })
}
```

```error
test.met:3:27: when branch type mismatch: expected void, got Int
        fun foo(x Int) Int { 42 }
        foo(when { case true: 1 })
                              ^
    }
```

**When branches type mismatch**

```metall
{
    let x = false
    let y = true
    when {
        case x: 1
        case y: "hello"
        else: 3
    }
}
```

```error
test.met:6:17: when branch type mismatch: expected Int, got Str
            case x: 1
            case y: "hello"
                    ^^^^^^^
            else: 3
```

**When unused with non-void converging type**

```metall
{
    let x = false
    when {
        case x: 1
        else: 2
    }
    42
}
```

```error
test.met:3:5: expression result of type Int is unused, assign to _ to discard
        let x = false
        when {
        ^
            case x: 1
            else: 2
        }
        ^
        42
```

## References

**Ref type**

```metall
{ let x = 5 let y = &x y }
```

```types
Block: &Int
  Var: void
    Int: Int
  Var: void
    Ref: &Int
      Ident: Int
  Ident: &Int
```

**Mut ref type**

```metall
{ mut x = 5 let y = &mut x y }
```

```types
Block: &mut Int
  Var: void
    Int: Int
  Var: void
    Ref: &mut Int
      Ident: Int
  Ident: &mut Int
```

**Mut binding of immutable ref**

```metall
{ let x = 5 mut y = &x y }
```

```types
Block: &Int
  Var: void
    Int: Int
  Var: void
    Ref: &Int
      Ident: Int
  Ident: &Int
```

**Immutable ref to mut**

```metall
{ mut x = 5 mut y = &x y }
```

```types
Block: &Int
  Var: void
    Int: Int
  Var: void
    Ref: &Int
      Ident: Int
  Ident: &Int
```

**Ref of field**

```metall
{ struct Foo { one Int } let x = Foo(42) let y = &x.one y }
```

```types
Block: &Int
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Var: void
    TypeConstruction: struct01
      Ident: struct01
      Int: Int
  Var: void
    Ref: &Int
      FieldAccess: Int
        Ident: struct01
  Ident: &Int
---
struct01 = Foo { one Int }
```

**Ref of nested field**

```metall
{ struct Bar { one Int } struct Foo { bar Bar } let x = Foo(Bar(1)) let y = &x.bar.one y }
```

```types
Block: &Int
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Struct: struct02
    StructField: ?
      SimpleType: ?
  Var: void
    TypeConstruction: struct02
      Ident: struct02
      TypeConstruction: struct01
        Ident: struct01
        Int: Int
  Var: void
    Ref: &Int
      FieldAccess: Int
        FieldAccess: struct01
          Ident: struct02
  Ident: &Int
---
struct01 = Bar { one Int }
struct02 = Foo { bar struct01 }
```

**Ref of deref**

```metall
{ mut x = 5 let y = &mut x let z = &y.* z }
```

```types
Block: &Int
  Var: void
    Int: Int
  Var: void
    Ref: &mut Int
      Ident: Int
  Var: void
    Ref: &Int
      Deref: Int
        Ident: &mut Int
  Ident: &Int
```

**Mutable reference to struct field**

```metall
{ struct Foo { one Int } mut x = Foo(42) let y = &mut x.one y }
```

```types
Block: &mut Int
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Var: void
    TypeConstruction: struct01
      Ident: struct01
      Int: Int
  Var: void
    Ref: &mut Int
      FieldAccess: Int
        Ident: struct01
  Ident: &mut Int
---
struct01 = Foo { one Int }
```

**Deref**

```metall
{ let x = 5 let y = &x y.* }
```

```types
Block: Int
  Var: void
    Int: Int
  Var: void
    Ref: &Int
      Ident: Int
  Deref: Int
    Ident: &Int
```

**Deref field access**

```metall
{ struct Foo{ one Str } let x = Foo("hello") let y = &x x.one }
```

```types
Block: Str
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Var: void
    TypeConstruction: struct01
      Ident: struct01
      String: Str
  Var: void
    Ref: &struct01
      Ident: struct01
  FieldAccess: Str
    Ident: struct01
---
struct01 = Foo { one Str }
```

**Deref assign**

```metall
{ mut x = 1 mut y = &mut x y.* = 321 }
```

```types
Block: void
  Var: void
    Int: Int
  Var: void
    Ref: &mut Int
      Ident: Int
  Assign: void
    Deref: Int
      Ident: &mut Int
    Int: Int
```

**Nested deref assign**

```metall
{ mut x = 1 mut y = &mut x mut z = &mut y y.* = 123 z.*.* = 321 }
```

```types
Block: void
  Var: void
    Int: Int
  Var: void
    Ref: &mut Int
      Ident: Int
  Var: void
    Ref: &mut &mut Int
      Ident: &mut Int
  Assign: void
    Deref: Int
      Ident: &mut Int
    Int: Int
  Assign: void
    Deref: Int
      Deref: &mut Int
        Ident: &mut &mut Int
    Int: Int
```

**Mut ref parameter**

```metall
{ fun foo(a &mut Int) void { a.* = 321 } mut x = 123 foo(&mut x) }
```

```types
Block: void
  Fun: fun01
    FunParam: &mut Int
      RefType: &mut Int
        SimpleType: Int
    SimpleType: void
    Block: void
      Assign: void
        Deref: Int
          Ident: &mut Int
        Int: Int
  Var: void
    Int: Int
  Call: void
    Ident: fun01
    Ref: &mut Int
      Ident: Int
---
fun01 = fun(&mut Int) void
```

**&mut coerces to &ref in call**

```metall
{ fun foo(a &Int) void {} mut x = 123 foo(&x) }
```

```types
Block: void
  Fun: fun01
    FunParam: &Int
      RefType: &Int
        SimpleType: Int
    SimpleType: void
    Block: void
  Var: void
    Int: Int
  Call: void
    Ident: fun01
    Ref: &Int
      Ident: Int
---
fun01 = fun(&Int) void
```

**&mut coerces to &ref in struct construction**

```metall
{ struct Foo { one &Int } mut x = 1 let y = Foo(&x) }
```

```types
Block: void
  Struct: struct01
    StructField: ?
      RefType: ?
        SimpleType: ?
  Var: void
    Int: Int
  Var: void
    TypeConstruction: struct01
      Ident: struct01
      Ref: &Int
        Ident: Int
---
struct01 = Foo { one &Int }
```

**Nested slice mutability coercion**

```metall
{
    let @a = Arena()
    let buf = unsafe @a.slice_uninit<Int>(8)
    fun foo(x [][]Int) Int { 1 }
    foo([buf][..])
}
```

```error
```

**Nested ref mutability coercion**

```metall
{
    fun foo(a []&Int) Int { 1 }
    mut x = 123
    foo([&mut x][..])
}
```

```error
```

**Mutable slice is invariant in its element**

Assigning `[]mut &mut Int` to `[]mut &Int` is rejected: a mutable slice is
invariant in its element. If it were allowed, a `&Int` stored through the
narrowed view could be read back as `&mut Int` and used to write an immutable
place.

```metall
{
    mut x = 1
    mut arr = [&mut x]
    let s = arr[..]
    let s2 []mut &Int = s
}
```

```error
test.met:5:25: type mismatch: expected []mut &Int, got []mut &mut Int
        let s = arr[..]
        let s2 []mut &Int = s
                            ^
    }
```

**Mutable ref is invariant in its pointee**

Assigning `&mut &mut Int` to `&mut &Int` is rejected: a mutable ref is invariant
in its pointee. If it were allowed, a store through the alias could plant an
immutable ref where a mutable one is expected.

```metall
{
    mut x = 1
    mut r = &mut x
    let rr &mut &Int = &mut r
}
```

```error
test.met:4:24: type mismatch: expected &mut &Int, got &mut &mut Int
        mut r = &mut x
        let rr &mut &Int = &mut r
                           ^^^^^^
    }
```

**Fun returns ref**

```metall
{ fun foo(a &Int) &Int { a } let x = 123 foo(&x) }
```

```types
Block: &Int
  Fun: fun01
    FunParam: &Int
      RefType: &Int
        SimpleType: Int
    RefType: &Int
      SimpleType: Int
    Block: &Int
      Ident: &Int
  Var: void
    Int: Int
  Call: &Int
    Ident: fun01
    Ref: &Int
      Ident: Int
---
fun01 = fun(&Int) &Int
```

**Deref assign through &mut struct field**

```metall
{ struct Foo { one &mut Int } mut x = 1 let y = Foo(&mut x) y.one.* = 42 }
```

```types
Block: void
  Struct: struct01
    StructField: ?
      RefType: ?
        SimpleType: ?
  Var: void
    Int: Int
  Var: void
    TypeConstruction: struct01
      Ident: struct01
      Ref: &mut Int
        Ident: Int
  Assign: void
    Deref: Int
      FieldAccess: &mut Int
        Ident: struct01
    Int: Int
---
struct01 = Foo { one &mut Int }
```

**Reassign field of mutable ref type**

```metall
{ struct Foo { one &mut Int } mut x = 1 mut y = 2 mut z = Foo(&mut x) z.one = &mut y z.one.* = 99 }
```

```types
Block: void
  Struct: struct01
    StructField: ?
      RefType: ?
        SimpleType: ?
  Var: void
    Int: Int
  Var: void
    Int: Int
  Var: void
    TypeConstruction: struct01
      Ident: struct01
      Ref: &mut Int
        Ident: Int
  Assign: void
    FieldAccess: &mut Int
      Ident: struct01
    Ref: &mut Int
      Ident: Int
  Assign: void
    Deref: Int
      FieldAccess: &mut Int
        Ident: struct01
    Int: Int
---
struct01 = Foo { one &mut Int }
```

## Forward declaration and recursion

**Forward declaration call**

```metall
{ foo() fun foo() void { } }
```

```types
Block: fun01
  Call: void
    Ident: fun01
  Fun: fun01
    SimpleType: void
    Block: void
---
fun01 = sync fun() void
```

**Self recursion**

```metall
{ fun foo(a Int) Int { foo(a) } foo(1) }
```

```types
Block: Int
  Fun: fun01
    FunParam: Int
      SimpleType: Int
    SimpleType: Int
    Block: Int
      Call: Int
        Ident: fun01
        Ident: Int
  Call: Int
    Ident: fun01
    Int: Int
---
fun01 = sync fun(Int) Int
```

**Mutual recursion**

```metall
{ fun foo(a Int) Int { bar(a) } fun bar(a Int) Int { foo(a) } foo(10) }
```

```types
Block: Int
  Fun: fun01
    FunParam: Int
      SimpleType: Int
    SimpleType: Int
    Block: Int
      Call: Int
        Ident: fun01
        Ident: Int
  Fun: fun01
    FunParam: Int
      SimpleType: Int
    SimpleType: Int
    Block: Int
      Call: Int
        Ident: fun01
        Ident: Int
  Call: Int
    Ident: fun01
    Int: Int
---
fun01 = sync fun(Int) Int
```

## Allocators

**Allocator var**

```metall
let @myalloc = Arena()
```

```types
AllocatorVar: void
  TypeConstruction: Arena
    Ident: Arena
```

```bindings
AllocatorVar: scope01
  TypeConstruction: scope01
    Ident: scope01
---
scope01:
  @myalloc: Arena
```

**Heap alloc struct**

```metall
{ let @myalloc = Arena() struct Foo{one Str} let x = @myalloc.new<Foo>(Foo("hello")) x }
```

```types
Block: &mut struct01
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Var: void
    Call: &mut struct01
      FieldAccess: fun01
        Ident: Arena
        SimpleType: struct01
      TypeConstruction: struct01
        Ident: struct01
        String: Str
  Ident: &mut struct01
---
struct01 = Foo { one Str }
fun01    = fun(Arena, struct01) &mut struct01
```

**Pass alloc to fun**

```metall
{ fun foo(@a Arena) void {} let @a = Arena() foo(@a) }
```

```types
Block: void
  Fun: fun01
    FunParam: Arena
      SimpleType: Arena
    SimpleType: void
    Block: void
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Call: void
    Ident: fun01
    Ident: Arena
---
fun01 = fun(Arena) void
```

**Heap alloc mut struct**

```metall
{ let @a = Arena() struct Bar{one Str} @a.new<Bar>(Bar("hello")) }
```

```types
Block: &mut struct01
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Call: &mut struct01
    FieldAccess: fun01
      Ident: Arena
      SimpleType: struct01
    TypeConstruction: struct01
      Ident: struct01
      String: Str
---
struct01 = Bar { one Str }
fun01    = fun(Arena, struct01) &mut struct01
```

**Make uninit slice**

```metall
{ let @a = Arena() unsafe @a.slice_uninit<Int>(5) }
```

```types
Block: []mut Int
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Call: []mut Int
    FieldAccess: fun01
      Ident: Arena
      SimpleType: Int
    Int: Int
---
fun01 = unsafe fun(Arena, Int) []mut Int
```

**Struct with allocator field**

```metall
{ struct Foo { @myalloc Arena } let @myalloc = Arena() let x = Foo(@myalloc) }
```

```types
Block: void
  Struct: struct01
    StructField: ?
      SimpleType: ?
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Var: void
    TypeConstruction: struct01
      Ident: struct01
      Ident: Arena
---
struct01 = Foo { @myalloc Arena }
```

**Heap alloc from struct field**

```metall
{ struct Foo{one Str} struct Bar { @myalloc Arena } let @myalloc = Arena() let x = Bar(@myalloc) let y = x.@myalloc.new<Foo>(Foo("hello")) }
```

```types
Block: void
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Struct: struct02
    StructField: ?
      SimpleType: ?
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Var: void
    TypeConstruction: struct02
      Ident: struct02
      Ident: Arena
  Var: void
    Call: &mut struct01
      FieldAccess: fun01
        FieldAccess: Arena
          Ident: struct02
        SimpleType: struct01
      TypeConstruction: struct01
        Ident: struct01
        String: Str
---
struct01 = Foo { one Str }
struct02 = Bar { @myalloc Arena }
fun01    = fun(Arena, struct01) &mut struct01
```

**Make uninit slice**

```metall
{ let @myalloc = Arena() unsafe @myalloc.slice_uninit<Int>(5) }
```

```types
Block: []mut Int
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Call: []mut Int
    FieldAccess: fun01
      Ident: Arena
      SimpleType: Int
    Int: Int
---
fun01 = unsafe fun(Arena, Int) []mut Int
```

**Make slice with default**

```metall
{ let @a = Arena() @a.slice<Int>(5, 42) }
```

```types
Block: []mut Int
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Call: []mut Int
    FieldAccess: fun01
      Ident: Arena
      SimpleType: Int
    Int: Int
    Int: Int
---
fun01 = fun(Arena, Int, Int) []mut Int
```

**Make slice**

```metall
{ let @myalloc = Arena() unsafe @myalloc.slice_uninit<Int>(5) }
```

```types
Block: []mut Int
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Call: []mut Int
    FieldAccess: fun01
      Ident: Arena
      SimpleType: Int
    Int: Int
---
fun01 = unsafe fun(Arena, Int) []mut Int
```

**Make slice default**

```metall
{ let @myalloc = Arena() @myalloc.slice<Int>(5, 42) }
```

```types
Block: []mut Int
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Call: []mut Int
    FieldAccess: fun01
      Ident: Arena
      SimpleType: Int
    Int: Int
    Int: Int
---
fun01 = fun(Arena, Int, Int) []mut Int
```

**Make uninit Int slice**

```metall
{ let @a = Arena() let x = unsafe @a.slice_uninit<Int>(5) }
```

```types
Block: void
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Var: void
    Call: []mut Int
      FieldAccess: fun01
        Ident: Arena
        SimpleType: Int
      Int: Int
---
fun01 = unsafe fun(Arena, Int) []mut Int
```

**Make uninit safe struct slice**

```metall
{ struct Foo{one Int two Int} let @a = Arena() let x = unsafe @a.slice_uninit<Foo>(3) }
```

```types
Block: void
  Struct: struct01
    StructField: ?
      SimpleType: ?
    StructField: ?
      SimpleType: ?
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Var: void
    Call: []mut struct01
      FieldAccess: fun01
        Ident: Arena
        SimpleType: struct01
      Int: Int
---
struct01 = Foo { one Int, two Int }
fun01    = unsafe fun(Arena, Int) []mut struct01
```

**Make slice Bool with default**

```metall
{ let @a = Arena() let x = @a.slice<Bool>(5, false) }
```

```types
Block: void
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Var: void
    Call: []mut Bool
      FieldAccess: fun01
        Ident: Arena
        SimpleType: Bool
      Int: Int
      Bool: Bool
---
fun01 = fun(Arena, Int, Bool) []mut Bool
```

**Allocator alias from another allocator**

```metall
{ let @a = Arena() let @b = @a }
```

```types
Block: void
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  AllocatorVar: void
    Ident: Arena
```

```bindings
Block: scope01
  AllocatorVar: scope02
    TypeConstruction: scope02
      Ident: scope02
  AllocatorVar: scope02
    Ident: scope02
---
scope01:
scope02:
  @a: Arena
  @b: Arena
```

**Allocator alias from struct field**

```metall
{ struct Foo { @a Arena } fun get(f &Foo) void { let @b = f.@a } }
```

```types
Block: fun01
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Fun: fun01
    FunParam: &struct01
      RefType: &struct01
        SimpleType: struct01
    SimpleType: void
    Block: void
      AllocatorVar: void
        FieldAccess: Arena
          Ident: &struct01
---
struct01 = Foo { @a Arena }
fun01    = fun(&struct01) void
```

**Allocator var must be an allocator**

```metall
let @x = 42
```

```error
test.met:1:10: allocator binding '@x' must be initialized with an allocator, got Int
    let @x = 42
             ^^
```

**Allocator must be bound to an @-identifier**

```metall
{ let @a = Arena() let x = @a }
```

```error
test.met:1:20: allocators must be bound to an @-identifier (e.g. `let @x = ...`)
    { let @a = Arena() let x = @a }
                       ^^^^^^^^^^
```

**Allocator param must be an @-identifier**

```metall
{ fun foo(a Arena) void {} }
```

```error
test.met:1:11: allocator 'a' must be bound to an @-identifier
    { fun foo(a Arena) void {} }
              ^
```

**@-identifier param must be an allocator**

```metall
{ fun foo(@x Int) void {} }
```

```error
test.met:1:11: @-identifier '@x' must hold an allocator, got Int
    { fun foo(@x Int) void {} }
              ^^
```

**@-identifier struct field must be an allocator**

```metall
{ struct Foo { @x Int } }
```

```error
test.met:1:16: @-identifier '@x' must hold an allocator, got Int
    { struct Foo { @x Int } }
                   ^^
```

**Optional allocator must be bound to an @-identifier**

An allocator stays an allocator capability through `?` / `!`, so it must keep its
@-identifier.

```metall
{ fun foo(maybe ?Arena) void {} }
```

```error
test.met:1:11: allocator 'maybe' must be bound to an @-identifier
    { fun foo(maybe ?Arena) void {} }
              ^^^^^
```

**@-identifier may hold an optional or result allocator**

```metall
{ fun foo(@maybe ?Arena, @res !Arena) void {} }
```

```error
```

## Arrays and Slices

**Array type**

```metall
fun foo(a [5]Int) void {}
```

```types
Fun: fun01
  FunParam: [5]Int
    ArrayType: [5]Int
      SimpleType: Int
  SimpleType: void
  Block: void
---
fun01 = sync fun([5]Int) void
```

**Array type ids are stable**

```metall
fun foo(a [5]Int, b [5]Int, c [6]Int) void { _ = [1, 2, 3, 4, 5]}
```

```types
Fun: fun01
  FunParam: [5]Int
    ArrayType: [5]Int
      SimpleType: Int
  FunParam: [5]Int
    ArrayType: [5]Int
      SimpleType: Int
  FunParam: [6]Int
    ArrayType: [6]Int
      SimpleType: Int
  SimpleType: void
  Block: void
    Assign: void
      Ident: ?
      ArrayLiteral: [5]Int
        Int: Int
        Int: Int
        Int: Int
        Int: Int
        Int: Int
---
fun01 = sync fun([5]Int, [5]Int, [6]Int) void
```

**Array literal**

```metall
[1, 2, 3]
```

```types
ArrayLiteral: [3]Int
  Int: Int
  Int: Int
  Int: Int
```

**Index read**

```metall
{ let x = [1, 2, 3] x[1] }
```

```types
Block: Int
  Var: void
    ArrayLiteral: [3]Int
      Int: Int
      Int: Int
      Int: Int
  Index: Int
    Ident: [3]Int
    Int: Int
```

**Index write**

```metall
{ mut x = [1, 2, 3] x[1] = 5 }
```

```types
Block: void
  Var: void
    ArrayLiteral: [3]Int
      Int: Int
      Int: Int
      Int: Int
  Assign: void
    Index: Int
      Ident: [3]Int
      Int: Int
    Int: Int
```

**Subslice array lo..hi**

```metall
{ let x = [1, 2, 3] x[0..2] }
```

```types
Block: []Int
  Var: void
    ArrayLiteral: [3]Int
      Int: Int
      Int: Int
      Int: Int
  SubSlice: []Int
    Ident: [3]Int
    Range: struct01
      Int: Int
      Int: Int
---
struct01 = Range { start Int, end Int }
```

**Subslice array lo..=hi**

```metall
{ let x = [1, 2, 3] x[0..=2] }
```

```types
Block: []Int
  Var: void
    ArrayLiteral: [3]Int
      Int: Int
      Int: Int
      Int: Int
  SubSlice: []Int
    Ident: [3]Int
    Range: struct01
      Int: Int
      Int: Int
---
struct01 = Range { start Int, end Int }
```

**Subslice array ..hi**

```metall
{ let x = [1, 2, 3] x[..2] }
```

```types
Block: []Int
  Var: void
    ArrayLiteral: [3]Int
      Int: Int
      Int: Int
      Int: Int
  SubSlice: []Int
    Ident: [3]Int
    Range: struct01
      Int: Int
---
struct01 = Range { start Int, end Int }
```

**Subslice array lo..**

```metall
{ let x = [1, 2, 3] x[1..] }
```

```types
Block: []Int
  Var: void
    ArrayLiteral: [3]Int
      Int: Int
      Int: Int
      Int: Int
  SubSlice: []Int
    Ident: [3]Int
    Range: struct01
      Int: Int
---
struct01 = Range { start Int, end Int }
```

**Array len**

```metall
{ let x = [1, 2, 3] x.len }
```

```types
Block: Int
  Var: void
    ArrayLiteral: [3]Int
      Int: Int
      Int: Int
      Int: Int
  FieldAccess: Int
    Ident: [3]Int
```

**Slice index read**

```metall
{ let @myalloc = Arena() let x = unsafe @myalloc.slice_uninit<Int>(3) x[1] }
```

```types
Block: Int
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Var: void
    Call: []mut Int
      FieldAccess: fun01
        Ident: Arena
        SimpleType: Int
      Int: Int
  Index: Int
    Ident: []mut Int
    Int: Int
---
fun01 = unsafe fun(Arena, Int) []mut Int
```

**Slice index write**

```metall
{ let @myalloc = Arena() let x = unsafe @myalloc.slice_uninit<Int>(3) x[1] = 5 }
```

```types
Block: void
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Var: void
    Call: []mut Int
      FieldAccess: fun01
        Ident: Arena
        SimpleType: Int
      Int: Int
  Assign: void
    Index: Int
      Ident: []mut Int
      Int: Int
    Int: Int
---
fun01 = unsafe fun(Arena, Int) []mut Int
```

**Slice len**

```metall
{ let @myalloc = Arena() let x = unsafe @myalloc.slice_uninit<Int>(3) x.len }
```

```types
Block: Int
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Var: void
    Call: []mut Int
      FieldAccess: fun01
        Ident: Arena
        SimpleType: Int
      Int: Int
  FieldAccess: Int
    Ident: []mut Int
---
fun01 = unsafe fun(Arena, Int) []mut Int
```

**Slice as fun param**

```metall
{ let @a = Arena() fun foo(s []Int) Int { s[0] } let x = unsafe @a.slice_uninit<Int>(3) foo(x) }
```

```types
Block: Int
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Fun: fun01
    FunParam: []Int
      SliceType: []Int
        SimpleType: Int
    SimpleType: Int
    Block: Int
      Index: Int
        Ident: []Int
        Int: Int
  Var: void
    Call: []mut Int
      FieldAccess: fun02
        Ident: Arena
        SimpleType: Int
      Int: Int
  Call: Int
    Ident: fun01
    Ident: []mut Int
---
fun01 = fun([]Int) Int
fun02 = unsafe fun(Arena, Int) []mut Int
```

**Slice as fun param and return**

```metall
{ let @a = Arena() fun foo(s []Int) []Int { s } let x = unsafe @a.slice_uninit<Int>(3) foo(x) }
```

```types
Block: []Int
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Fun: fun01
    FunParam: []Int
      SliceType: []Int
        SimpleType: Int
    SliceType: []Int
      SimpleType: Int
    Block: []Int
      Ident: []Int
  Var: void
    Call: []mut Int
      FieldAccess: fun02
        Ident: Arena
        SimpleType: Int
      Int: Int
  Call: []Int
    Ident: fun01
    Ident: []mut Int
---
fun01 = fun([]Int) []Int
fun02 = unsafe fun(Arena, Int) []mut Int
```

**Struct with slice field**

```metall
{ let @a = Arena() struct Foo { one []Int } let s = unsafe @a.slice_uninit<Int>(3) let x = Foo(s) x.one[0] }
```

```types
Block: Int
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Struct: struct01
    StructField: ?
      SliceType: ?
        SimpleType: ?
  Var: void
    Call: []mut Int
      FieldAccess: fun01
        Ident: Arena
        SimpleType: Int
      Int: Int
  Var: void
    TypeConstruction: struct01
      Ident: struct01
      Ident: []mut Int
  Index: Int
    FieldAccess: []Int
      Ident: struct01
    Int: Int
---
struct01 = Foo { one []Int }
fun01    = unsafe fun(Arena, Int) []mut Int
```

**Ref to slice**

```metall
{
    let @a = Arena()
    let x = unsafe @a.slice_uninit<Int>(3)
    &x
}
```

```types
Block: &[]mut Int
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Var: void
    Call: []mut Int
      FieldAccess: fun01
        Ident: Arena
        SimpleType: Int
      Int: Int
  Ref: &[]mut Int
    Ident: []mut Int
---
fun01 = unsafe fun(Arena, Int) []mut Int
```

**Slice index through ref**

```metall
{ let @a = Arena() let x = unsafe @a.slice_uninit<Int>(3) let y = &x y[0] }
```

```types
Block: Int
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Var: void
    Call: []mut Int
      FieldAccess: fun01
        Ident: Arena
        SimpleType: Int
      Int: Int
  Var: void
    Ref: &[]mut Int
      Ident: []mut Int
  Index: Int
    Ident: &[]mut Int
    Int: Int
---
fun01 = unsafe fun(Arena, Int) []mut Int
```

**Slice len through ref**

```metall
{ let @a = Arena() let x = unsafe @a.slice_uninit<Int>(3) let y = &x y.len }
```

```types
Block: Int
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Var: void
    Call: []mut Int
      FieldAccess: fun01
        Ident: Arena
        SimpleType: Int
      Int: Int
  Var: void
    Ref: &[]mut Int
      Ident: []mut Int
  FieldAccess: Int
    Ident: &[]mut Int
---
fun01 = unsafe fun(Arena, Int) []mut Int
```

**Mut ref slice index write**

```metall
{ let @a = Arena() mut x = unsafe @a.slice_uninit<Int>(3) let y = &mut x y[0] = 42 }
```

```types
Block: void
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Var: void
    Call: []mut Int
      FieldAccess: fun01
        Ident: Arena
        SimpleType: Int
      Int: Int
  Var: void
    Ref: &mut []mut Int
      Ident: []mut Int
  Assign: void
    Index: Int
      Ident: &mut []mut Int
      Int: Int
    Int: Int
---
fun01 = unsafe fun(Arena, Int) []mut Int
```

**Make mut slice**

```metall
{ let @a = Arena() unsafe @a.slice_uninit<Int>(5) }
```

```types
Block: []mut Int
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Call: []mut Int
    FieldAccess: fun01
      Ident: Arena
      SimpleType: Int
    Int: Int
---
fun01 = unsafe fun(Arena, Int) []mut Int
```

**Mut slice assignable to immutable**

```metall
{ let @a = Arena() fun foo(s []Int) Int { s[0] } let x = unsafe @a.slice_uninit<Int>(3) foo(x) }
```

```types
Block: Int
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Fun: fun01
    FunParam: []Int
      SliceType: []Int
        SimpleType: Int
    SimpleType: Int
    Block: Int
      Index: Int
        Ident: []Int
        Int: Int
  Var: void
    Call: []mut Int
      FieldAccess: fun02
        Ident: Arena
        SimpleType: Int
      Int: Int
  Call: Int
    Ident: fun01
    Ident: []mut Int
---
fun01 = fun([]Int) Int
fun02 = unsafe fun(Arena, Int) []mut Int
```

**Mut slice index write no mut binding**

```metall
{ let @a = Arena() let x = unsafe @a.slice_uninit<Int>(3) x[0] = 5 }
```

```types
Block: void
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Var: void
    Call: []mut Int
      FieldAccess: fun01
        Ident: Arena
        SimpleType: Int
      Int: Int
  Assign: void
    Index: Int
      Ident: []mut Int
      Int: Int
    Int: Int
---
fun01 = unsafe fun(Arena, Int) []mut Int
```

**Subslice mut array**

```metall
{ mut x = [1, 2, 3] x[0..2] }
```

```types
Block: []mut Int
  Var: void
    ArrayLiteral: [3]Int
      Int: Int
      Int: Int
      Int: Int
  SubSlice: []mut Int
    Ident: [3]Int
    Range: struct01
      Int: Int
      Int: Int
---
struct01 = Range { start Int, end Int }
```

**Subslice mut slice**

```metall
{ let @a = Arena() let x = unsafe @a.slice_uninit<Int>(5) x[1..3] }
```

```types
Block: []mut Int
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Var: void
    Call: []mut Int
      FieldAccess: fun01
        Ident: Arena
        SimpleType: Int
      Int: Int
  SubSlice: []mut Int
    Ident: []mut Int
    Range: struct01
      Int: Int
      Int: Int
---
fun01    = unsafe fun(Arena, Int) []mut Int
struct01 = Range { start Int, end Int }
```

**Subslice mut slice through mut ref**

```metall
{ let @a = Arena() mut x = unsafe @a.slice_uninit<Int>(5) let y = &mut x y[1..3] }
```

```types
Block: []mut Int
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Var: void
    Call: []mut Int
      FieldAccess: fun01
        Ident: Arena
        SimpleType: Int
      Int: Int
  Var: void
    Ref: &mut []mut Int
      Ident: []mut Int
  SubSlice: []mut Int
    Ident: &mut []mut Int
    Range: struct01
      Int: Int
      Int: Int
---
fun01    = unsafe fun(Arena, Int) []mut Int
struct01 = Range { start Int, end Int }
```

**Subslice mut slice through immutable ref**

```metall
{ let @a = Arena() let x = unsafe @a.slice_uninit<Int>(5) let y = &x y[1..3] }
```

```types
Block: []Int
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Var: void
    Call: []mut Int
      FieldAccess: fun01
        Ident: Arena
        SimpleType: Int
      Int: Int
  Var: void
    Ref: &[]mut Int
      Ident: []mut Int
  SubSlice: []Int
    Ident: &[]mut Int
    Range: struct01
      Int: Int
      Int: Int
---
fun01    = unsafe fun(Arena, Int) []mut Int
struct01 = Range { start Int, end Int }
```

**Subslice through ref**

```metall
{ let x = [1, 2, 3] let y = &x y[0..2] }
```

```types
Block: []Int
  Var: void
    ArrayLiteral: [3]Int
      Int: Int
      Int: Int
      Int: Int
  Var: void
    Ref: &[3]Int
      Ident: [3]Int
  SubSlice: []Int
    Ident: &[3]Int
    Range: struct01
      Int: Int
      Int: Int
---
struct01 = Range { start Int, end Int }
```

**Empty slice in make**

```metall
{ let @a = Arena() let x = @a.slice<[]Int>(2, []) }
```

```types
Block: void
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Var: void
    Call: []mut []Int
      FieldAccess: fun01
        Ident: Arena
        SliceType: []Int
          SimpleType: Int
      Int: Int
      EmptySlice: []Int
---
fun01 = fun(Arena, Int, []Int) []mut []Int
```

**Empty slice in assignment**

```metall
{ let @a = Arena() mut x = unsafe @a.slice_uninit<Int>(3) x = [] }
```

```types
Block: void
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Var: void
    Call: []mut Int
      FieldAccess: fun01
        Ident: Arena
        SimpleType: Int
      Int: Int
  Assign: void
    Ident: []mut Int
    EmptySlice: []mut Int
---
fun01 = unsafe fun(Arena, Int) []mut Int
```

**Empty slice as fun arg**

```metall
{ fun foo(s []Int) void {} foo([]) }
```

```types
Block: void
  Fun: fun01
    FunParam: []Int
      SliceType: []Int
        SimpleType: Int
    SimpleType: void
    Block: void
  Call: void
    Ident: fun01
    EmptySlice: []Int
---
fun01 = fun([]Int) void
```

**Empty slice in struct construction**

```metall
{ struct Foo { items []Int } Foo([]) }
```

```types
Block: struct01
  Struct: struct01
    StructField: ?
      SliceType: ?
        SimpleType: ?
  TypeConstruction: struct01
    Ident: struct01
    EmptySlice: []Int
---
struct01 = Foo { items []Int }
```

**Empty slice in make default**

```metall
{ let @a = Arena() let x = @a.slice<[]Int>(3, []) }
```

```types
Block: void
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Var: void
    Call: []mut []Int
      FieldAccess: fun01
        Ident: Arena
        SliceType: []Int
          SimpleType: Int
      Int: Int
      EmptySlice: []Int
---
fun01 = fun(Arena, Int, []Int) []mut []Int
```

**Multidimensional array type**

```metall
fun foo(a [3][4]Int) void {}
```

```types
Fun: fun01
  FunParam: [3][4]Int
    ArrayType: [3][4]Int
      ArrayType: [4]Int
        SimpleType: Int
  SimpleType: void
  Block: void
---
fun01 = sync fun([3][4]Int) void
```

**Multidimensional slice type**

```metall
fun foo(a [][]Int) void {}
```

```types
Fun: fun01
  FunParam: [][]Int
    SliceType: [][]Int
      SliceType: []Int
        SimpleType: Int
  SimpleType: void
  Block: void
---
fun01 = fun([][]Int) void
```

**Mixed array slice type**

```metall
fun foo(a [3][]Int) void {}
```

```types
Fun: fun01
  FunParam: [3][]Int
    ArrayType: [3][]Int
      SliceType: []Int
        SimpleType: Int
  SimpleType: void
  Block: void
---
fun01 = fun([3][]Int) void
```

**Array fill construction infers the element type from the value**

```metall
[4 of U8(42)]
```

```types
ArrayConstruction: [4]U8
  TypeConstruction: U8
    Ident: U8
    Int: U8
```

**A typed binding rejects a length-mismatched construction**

```metall
{ let x [5]Int = [6 of 3] _ = x }
```

```error
test.met:1:18: type mismatch: expected [Int 5], got [Int 6]
    { let x [5]Int = [6 of 3] _ = x }
                     ^^^^^^^^
test.met:1:31: symbol not defined: x
    { let x [5]Int = [6 of 3] _ = x }
                                  ^
```

**Annotation flows the element type into an array literal**

```metall
{ let a [4]U8 = [1, 2, 3, 4] _ = a }
```

```types
Block: void
  Var: void
    ArrayType: [4]U8
      SimpleType: U8
    ArrayLiteral: [4]U8
      Int: U8
      Int: U8
      Int: U8
      Int: U8
  Assign: void
    Ident: ?
    Ident: [4]U8
```

**Annotation flows the element type into a fill construction**

```metall
{ let a [4]U8 = [4 of 1] _ = a }
```

```types
Block: void
  Var: void
    ArrayType: [4]U8
      SimpleType: U8
    ArrayConstruction: [4]U8
      Int: U8
  Assign: void
    Ident: ?
    Ident: [4]U8
```

**Field type flows into a construction argument**

```metall module
struct A { a [4]U8 }
fun f() void {
    let x = A([4 of 1])
    _ = x
}
```

```types
Module: test
  Struct: struct01
    StructField: ?
      ArrayType: ?
        SimpleType: ?
  Fun: fun01
    SimpleType: void
    Block: void
      Var: void
        TypeConstruction: struct01
          Ident: struct01
          ArrayConstruction: [4]U8
            Int: U8
      Assign: void
        Ident: ?
        Ident: struct01
---
struct01 = A { a [4]U8 }
fun01    = sync fun() void
```

**Array uninitialized construction**

```metall
unsafe [3 uninit Int]
```

```types
ArrayConstruction: [3]Int
  SimpleType: Int
```

**Uninitialized array requires unsafe**

```metall
[3 uninit Int]
```

```error
test.met:1:1: uninitialized array requires unsafe: write [N of v] to fill it
    [3 uninit Int]
    ^^^^^^^^^^^^^^
```

**Unsafe applies only to an uninitialized array**

```metall
unsafe [3 of Int(5)]
```

```error
test.met:1:8: unsafe applies only to an uninitialized array [N uninit T]
    unsafe [3 of Int(5)]
           ^^^^^^^^^^^^^
```

**Array fill of a nocopy value is rejected**

```metall module
nocopy struct Handle { id Int }
fun foo() void {
    let a = [3 of Handle(1)]
}
```

```error
test.met:3:13: cannot fill an array with nocopy type test.Handle; use unsafe [N uninit T] and set each element
    fun foo() void {
        let a = [3 of Handle(1)]
                ^^^^^^^^^^^^^^^^
    }
```

## Arithmetic and comparison

**Int +**

```metall
1 + 2
```

```types
Binary: Int
  Int: Int
  Int: Int
```

**Int -**

```metall
1 - 2
```

```types
Binary: Int
  Int: Int
  Int: Int
```

**Int star**

```metall
1 * 2
```

```types
Binary: Int
  Int: Int
  Int: Int
```

**Int /**

```metall
1 / 2
```

```types
Binary: Int
  Int: Int
  Int: Int
```

**Int %**

```metall
1 % 2
```

```types
Binary: Int
  Int: Int
  Int: Int
```

**< on int**

```metall
1 < 2
```

```types
Binary: Bool
  Int: Int
  Int: Int
```

**<= on int**

```metall
1 <= 2
```

```types
Binary: Bool
  Int: Int
  Int: Int
```

**> on int**

```metall
1 > 2
```

```types
Binary: Bool
  Int: Int
  Int: Int
```

**>= on int**

```metall
1 >= 2
```

```types
Binary: Bool
  Int: Int
  Int: Int
```

**== on int**

```metall
1 == 2
```

```types
Binary: Bool
  Int: Int
  Int: Int
```

**!= on int**

```metall
1 != 2
```

```types
Binary: Bool
  Int: Int
  Int: Int
```

**== on bool**

```metall
true == true
```

```types
Binary: Bool
  Bool: Bool
  Bool: Bool
```

**!= on bool**

```metall
true != true
```

```types
Binary: Bool
  Bool: Bool
  Bool: Bool
```

**And, not, or**

```metall
true and false or not true
```

```types
Binary: Bool
  Binary: Bool
    Bool: Bool
    Bool: Bool
  Unary: Bool
    Bool: Bool
```

**Float literal defaults to Float**

```metall
3.14
```

```types
Float: Float
```

**Float +**

```metall
1.5 + 2.5
```

```types
Binary: Float
  Float: Float
  Float: Float
```

**Float /**

```metall
3.0 / 2.0
```

```types
Binary: Float
  Float: Float
  Float: Float
```

**Float < yields Bool**

```metall
1.5 < 2.5
```

```types
Binary: Bool
  Float: Float
  Float: Float
```

**Unary minus on a float**

```metall
-1.5
```

```types
Unary: Float
  Float: Float
```

**Float == yields Bool**

```metall
1.5 == 1.5
```

```types
Binary: Bool
  Float: Float
  Float: Float
```

**F32 via constructor narrows the literal**

```metall
F32(1.5)
```

```types
TypeConstruction: F32
  Ident: F32
  Float: F32
```

**F32 materialization binary**

```metall
F32(1.5) + 2.5
```

```types
Binary: F32
  TypeConstruction: F32
    Ident: F32
    Float: F32
  Float: F32
```

## Type constructors and materialization

**Type constructor**

```metall
U8(42)
```

```types
TypeConstruction: U8
  Ident: U8
  Int: U8
```

**Int materialization binary**

```metall
U8(1) + 2
```

```types
Binary: U8
  TypeConstruction: U8
    Ident: U8
    Int: U8
  Int: U8
```

**Rune literal materialization binary**

```metall
U8(32) == ' '
```

```types
Binary: Bool
  TypeConstruction: U8
    Ident: U8
    Int: U8
  RuneLiteral: U8
```

**Int literal on LHS of binary**

```metall
{ let byte = U8(32) 10 == byte }
```

```types
Block: Bool
  Var: void
    TypeConstruction: U8
      Ident: U8
      Int: U8
  Binary: Bool
    Int: U8
    Ident: U8
```

**Rune literal on LHS of binary**

```metall
{ let byte = U8(32) ' ' == byte }
```

```types
Block: Bool
  Var: void
    TypeConstruction: U8
      Ident: U8
      Int: U8
  Binary: Bool
    RuneLiteral: U8
    Ident: U8
```

**Rune literal materialization call arg**

```metall
{ fun foo(a U8) U8 { a } foo('a') }
```

```types
Block: U8
  Fun: fun01
    FunParam: U8
      SimpleType: U8
    SimpleType: U8
    Block: U8
      Ident: U8
  Call: U8
    Ident: fun01
    RuneLiteral: U8
---
fun01 = sync fun(U8) U8
```

**Int materialization call arg**

```metall
{ fun foo(a U8) U8 { a } foo(42) }
```

```types
Block: U8
  Fun: fun01
    FunParam: U8
      SimpleType: U8
    SimpleType: U8
    Block: U8
      Ident: U8
  Call: U8
    Ident: fun01
    Int: U8
---
fun01 = sync fun(U8) U8
```

**Int materialization array literal**

```metall
[U8(1), 2, 3]
```

```types
ArrayLiteral: [3]U8
  TypeConstruction: U8
    Ident: U8
    Int: U8
  Int: U8
  Int: U8
```

**Int materialization struct construction**

```metall
{ struct Foo { one U8 two U8 } let x = Foo(1, 2) }
```

```types
Block: void
  Struct: struct01
    StructField: ?
      SimpleType: ?
    StructField: ?
      SimpleType: ?
  Var: void
    TypeConstruction: struct01
      Ident: struct01
      Int: U8
      Int: U8
---
struct01 = Foo { one U8, two U8 }
```

## Loops

**Conditional for loop**

```metall
for true { }
```

```types
For: void
  Bool: Bool
  Block: void
```

**Unconditional for loop**

```metall
for { }
```

```types
For: void
  Block: void
```

**For body must be scoped**

```metall
{ let a = 1 for { let a = "hello" }}
```

```types
Block: void
  Var: void
    Int: Int
  For: void
    Block: void
      Var: void
        String: Str
```

**For in range**

```metall
{ for x in 0..10 { } }
```

```types
Block: void
  For: void
    Range: struct01
      Int: Int
      Int: Int
    Block: void
---
struct01 = Range { start Int, end Int }
```

**Range as a value expression**

```metall
{ let r = 0..4 r }
```

```types
Block: struct01
  Var: void
    Range: struct01
      Int: Int
      Int: Int
  Ident: struct01
---
struct01 = Range { start Int, end Int }
```

**For in range inclusive**

```metall
{ for x in 0..=9 { } }
```

```types
Block: void
  For: void
    Range: struct01
      Int: Int
      Int: Int
    Block: void
---
struct01 = Range { start Int, end Int }
```

**For in range with break**

```metall
{ for x in 0..10 { if x == 5 { break } } }
```

```types
Block: void
  For: void
    Range: struct01
      Int: Int
      Int: Int
    Block: void
      If: void
        Binary: Bool
          Ident: Int
          Int: Int
        Block: never
          Break: never
---
struct01 = Range { start Int, end Int }
```

**For in range binding is Int**

```metall
{ for x in 0..10 { let y = x + 1 } }
```

```types
Block: void
  For: void
    Range: struct01
      Int: Int
      Int: Int
    Block: void
      Var: void
        Binary: Int
          Ident: Int
          Int: Int
---
struct01 = Range { start Int, end Int }
```

**For in range binding shadows outer**

```metall
{ let i = 0 for i in 0..1 { } }
```

```types
Block: void
  Var: void
    Int: Int
  For: void
    Range: struct01
      Int: Int
      Int: Int
    Block: void
---
struct01 = Range { start Int, end Int }
```

**For in slice binding is the element type**

```metall
{ for x in ["a", "b"] { let y = x } }
```

```types
Block: void
  For: void
    ArrayLiteral: [2]Str
      String: Str
      String: Str
    Block: void
      Var: void
        Ident: Str
```

**For in slice with index binds element and Int**

```metall
{ for x, i in [10, 20] { let y = x + i } }
```

```types
Block: void
  For: void
    ArrayLiteral: [2]Int
      Int: Int
      Int: Int
    Block: void
      Var: void
        Binary: Int
          Ident: Int
          Ident: Int
```

**For in over a slice value**

```metall
{ let s = [1, 2, 3][..] for x in s { let y = x + 1 } }
```

```types
Block: void
  Var: void
    SubSlice: []Int
      ArrayLiteral: [3]Int
        Int: Int
        Int: Int
        Int: Int
      Range: struct01
  For: void
    Ident: []Int
    Block: void
      Var: void
        Binary: Int
          Ident: Int
          Int: Int
---
struct01 = Range { start Int, end Int }
```

**For in over a non-iterable is rejected**

```metall
{ for x in 0 { } }
```

```error
test.met:1:12: unknown field: Int.next
    { for x in 0 { } }
               ^
```

**For in over an iterator binds the element type**

```metall
{
    struct Counter { n Int }
    fun Counter.next(c &mut Counter) ?Int { None() }
    for x in Counter(0) { let y Str = x }
}
```

```error
test.met:4:39: type mismatch: expected Str, got Int
        fun Counter.next(c &mut Counter) ?Int { None() }
        for x in Counter(0) { let y Str = x }
                                          ^
    }
```

**For in over an iterator cannot bind a reference**

```metall
{
    struct Counter { n Int }
    fun Counter.next(c &mut Counter) ?Int { None() }
    for &x in Counter(0) { }
}
```

```error
test.met:4:10: for-in over an iterator cannot bind a reference; have next() yield one
        fun Counter.next(c &mut Counter) ?Int { None() }
        for &x in Counter(0) { }
             ^
    }
```

**For in over a nocopy iterator is rejected**

```metall
{
    nocopy struct Counter { n Int }
    fun Counter.next(c &mut Counter) ?Int { None() }
    for x in Counter(0) { }
}
```

```error
test.met:4:14: cannot iterate over nocopy iterator Counter
        fun Counter.next(c &mut Counter) ?Int { None() }
        for x in Counter(0) { }
                 ^^^^^^^^^^
    }
```

**For in index binding on a range**

```metall
{ for x, i in 0..10 { } }
```

```types
Block: void
  For: void
    Range: struct01
      Int: Int
      Int: Int
    Block: void
---
struct01 = Range { start Int, end Int }
```

**For in by reference binds an immutable ref**

```metall
{ for &x in [1, 2, 3] { let y = x } }
```

```types
Block: void
  For: void
    ArrayLiteral: [3]Int
      Int: Int
      Int: Int
      Int: Int
    Block: void
      Var: void
        Ident: &Int
```

**For in by mutable reference over a mutable slice**

```metall
{ mut arr = [1, 2, 3] for &mut x in arr[..] { let y = x } }
```

```types
Block: void
  Var: void
    ArrayLiteral: [3]Int
      Int: Int
      Int: Int
      Int: Int
  For: void
    SubSlice: []mut Int
      Ident: [3]Int
      Range: struct01
    Block: void
      Var: void
        Ident: &mut Int
---
struct01 = Range { start Int, end Int }
```

**For in mutable reference over a mutable array binds an &mut element**

```metall
{ mut arr = [1, 2, 3] for &mut x in arr { let y = x } }
```

```types
Block: void
  Var: void
    ArrayLiteral: [3]Int
      Int: Int
      Int: Int
      Int: Int
  For: void
    Ident: [3]Int
    Block: void
      Var: void
        Ident: &mut Int
```

**For in mutable reference over an immutable slice is rejected**

```metall
{ for &mut x in [1, 2, 3][..] { } }
```

```error
test.met:1:12: `for &mut` requires a mutable slice ([]mut T) or a mutable array, got []Int
    { for &mut x in [1, 2, 3][..] { } }
               ^
```

**For in mutable reference over an immutable array binding is rejected**

```metall
{ let arr = [1, 2, 3] for &mut x in arr { } }
```

```error
test.met:1:32: `for &mut` requires a mutable slice ([]mut T) or a mutable array, got [Int 3]
    { let arr = [1, 2, 3] for &mut x in arr { } }
                                   ^
```

**For in mutable reference over an array literal is rejected**

```metall
{ for &mut x in [1, 2, 3] { } }
```

```error
test.met:1:12: `for &mut` requires a mutable slice ([]mut T) or a mutable array, got [Int 3]
    { for &mut x in [1, 2, 3] { } }
               ^
```

**For in reference over a range is rejected**

```metall
{ for &x in 0..3 { } }
```

```error
test.met:1:8: for-in over an iterator cannot bind a reference; have next() yield one
    { for &x in 0..3 { } }
           ^
```

## Methods

**Method call basic**

```metall
{ struct Foo { one Int } fun Foo.get(f Foo) Int { f.one } let x = Foo(42) x.get() }
```

```types
Block: Int
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Fun: fun01
    FunParam: struct01
      SimpleType: struct01
    SimpleType: Int
    Block: Int
      FieldAccess: Int
        Ident: struct01
  Var: void
    TypeConstruction: struct01
      Ident: struct01
      Int: Int
  Call: Int
    FieldAccess: fun01
      Ident: struct01
---
struct01 = Foo { one Int }
fun01    = sync fun(struct01) Int
```

**Method call with args**

```metall
{ struct Foo { one Int } fun Foo.add(f Foo, n Int) Int { f.one + n } let x = Foo(10) x.add(5) }
```

```types
Block: Int
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Fun: fun01
    FunParam: struct01
      SimpleType: struct01
    FunParam: Int
      SimpleType: Int
    SimpleType: Int
    Block: Int
      Binary: Int
        FieldAccess: Int
          Ident: struct01
        Ident: Int
  Var: void
    TypeConstruction: struct01
      Ident: struct01
      Int: Int
  Call: Int
    FieldAccess: fun01
      Ident: struct01
    Int: Int
---
struct01 = Foo { one Int }
fun01    = sync fun(struct01, Int) Int
```

**Method call on &ref receiver**

```metall
{ struct Foo { one Int } fun Foo.get(f &Foo) Int { f.one } let x = Foo(42) let y = &x y.get() }
```

```types
Block: Int
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Fun: fun01
    FunParam: &struct01
      RefType: &struct01
        SimpleType: struct01
    SimpleType: Int
    Block: Int
      FieldAccess: Int
        Ident: &struct01
  Var: void
    TypeConstruction: struct01
      Ident: struct01
      Int: Int
  Var: void
    Ref: &struct01
      Ident: struct01
  Call: Int
    FieldAccess: fun01
      Ident: &struct01
---
struct01 = Foo { one Int }
fun01    = fun(&struct01) Int
```

**Method fun declaration type**

```metall
{ struct Foo { one Int } fun Foo.get(f Foo) Int { f.one } }
```

```types
Block: fun01
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Fun: fun01
    FunParam: struct01
      SimpleType: struct01
    SimpleType: Int
    Block: Int
      FieldAccess: Int
        Ident: struct01
---
struct01 = Foo { one Int }
fun01    = sync fun(struct01) Int
```

**Direct qualified call**

```metall
{ struct Foo { one Int } fun Foo.get(f Foo) Int { f.one } let x = Foo(42) Foo.get(x) }
```

```types
Block: Int
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Fun: fun01
    FunParam: struct01
      SimpleType: struct01
    SimpleType: Int
    Block: Int
      FieldAccess: Int
        Ident: struct01
  Var: void
    TypeConstruction: struct01
      Ident: struct01
      Int: Int
  Call: Int
    Ident: fun01
    Ident: struct01
---
struct01 = Foo { one Int }
fun01    = sync fun(struct01) Int
```

**Direct qualified call with extra args**

```metall
{ struct Foo { one Int } fun Foo.add(f Foo, n Int) Int { f.one + n } let x = Foo(10) Foo.add(x, 5) }
```

```types
Block: Int
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Fun: fun01
    FunParam: struct01
      SimpleType: struct01
    FunParam: Int
      SimpleType: Int
    SimpleType: Int
    Block: Int
      Binary: Int
        FieldAccess: Int
          Ident: struct01
        Ident: Int
  Var: void
    TypeConstruction: struct01
      Ident: struct01
      Int: Int
  Call: Int
    Ident: fun01
    Ident: struct01
    Int: Int
---
struct01 = Foo { one Int }
fun01    = sync fun(struct01, Int) Int
```

**Method call on builtin type**

```metall
{ fun Int.double(self Int) Int { self + self } let x = 21 x.double() }
```

```types
Block: Int
  Fun: fun01
    FunParam: Int
      SimpleType: Int
    SimpleType: Int
    Block: Int
      Binary: Int
        Ident: Int
        Ident: Int
  Var: void
    Int: Int
  Call: Int
    FieldAccess: fun01
      Ident: Int
---
fun01 = sync fun(Int) Int
```

**Direct qualified call on builtin type**

```metall
{ fun Int.double(self Int) Int { self + self } Int.double(21) }
```

```types
Block: Int
  Fun: fun01
    FunParam: Int
      SimpleType: Int
    SimpleType: Int
    Block: Int
      Binary: Int
        Ident: Int
        Ident: Int
  Call: Int
    Ident: fun01
    Int: Int
---
fun01 = sync fun(Int) Int
```

**Method call in namespace**

```metall
fun ns() Int { struct Foo { one Int } fun Foo.get(f Foo) Int { f.one } let x = Foo(42) x.get() }
```

```types
Fun: fun01
  SimpleType: Int
  Block: Int
    Struct: struct01
      StructField: ?
        SimpleType: ?
    Fun: fun02
      FunParam: struct01
        SimpleType: struct01
      SimpleType: Int
      Block: Int
        FieldAccess: Int
          Ident: struct01
    Var: void
      TypeConstruction: struct01
        Ident: struct01
        Int: Int
    Call: Int
      FieldAccess: fun02
        Ident: struct01
---
fun01    = sync fun() Int
struct01 = Foo { one Int }
fun02    = sync fun(struct01) Int
```

**Direct qualified call in namespace**

```metall
fun ns() Int { struct Foo { one Int } fun Foo.get(f Foo) Int { f.one } let x = Foo(42) Foo.get(x) }
```

```types
Fun: fun01
  SimpleType: Int
  Block: Int
    Struct: struct01
      StructField: ?
        SimpleType: ?
    Fun: fun02
      FunParam: struct01
        SimpleType: struct01
      SimpleType: Int
      Block: Int
        FieldAccess: Int
          Ident: struct01
    Var: void
      TypeConstruction: struct01
        Ident: struct01
        Int: Int
    Call: Int
      Ident: fun02
      Ident: struct01
---
fun01    = sync fun() Int
struct01 = Foo { one Int }
fun02    = sync fun(struct01) Int
```

## Generics - Structs

**Generic struct**

```metall
{ struct Foo<T> { value T } let x = Foo<Int>(42) x.value }
```

```types
Block: Int
  Struct: struct01
    TypeParam: ?
    StructField: ?
      SimpleType: ?
  Var: void
    TypeConstruction: struct02
      Ident: struct02
      Int: Int
  FieldAccess: Int
    Ident: struct02
---
struct01 = Foo { value T }
struct02 = Foo<Int> { value Int }
```

**Generic struct two params nested**

```metall
{
    struct Pair<A, B> { first A second B }
    struct Box<T> { value T }
    let x = Pair<Box<Int>, Str>(Box<Int>(42), "hello")
    x.first.value
}
```

```types
Block: Int
  Struct: struct01
    TypeParam: ?
    TypeParam: ?
    StructField: ?
      SimpleType: ?
    StructField: ?
      SimpleType: ?
  Struct: struct02
    TypeParam: ?
    StructField: ?
      SimpleType: ?
  Var: void
    TypeConstruction: struct03
      Ident: struct03
      TypeConstruction: struct04
        Ident: struct04
        Int: Int
      String: Str
  FieldAccess: Int
    FieldAccess: struct04
      Ident: struct03
---
struct01 = Pair { first A, second B }
struct02 = Box { value T }
struct04 = Box<Int> { value Int }
struct03 = Pair<struct04, Str> { first struct04, second Str }
```

**Generic struct identity and distinctness**

```metall
{ struct Foo<T> { value T } let a = Foo<Int>(1) let b = Foo<Int>(2) let c = Foo<Str>("x") }
```

```types
Block: void
  Struct: struct01
    TypeParam: ?
    StructField: ?
      SimpleType: ?
  Var: void
    TypeConstruction: struct02
      Ident: struct02
      Int: Int
  Var: void
    TypeConstruction: struct02
      Ident: struct02
      Int: Int
  Var: void
    TypeConstruction: struct03
      Ident: struct03
      String: Str
---
struct01 = Foo { value T }
struct02 = Foo<Int> { value Int }
struct03 = Foo<Str> { value Str }
```

**Generic struct recursive**

```metall
{
    struct Node<T> { value T next &Node<T> }
    fun foo(n &Node<Int>) Int { n.next.value }
}
```

```types
Block: fun01
  Struct: struct02
    TypeParam: ?
    StructField: ?
      SimpleType: ?
    StructField: ?
      RefType: ?
        SimpleType: ?
          SimpleType: ?
  Fun: fun01
    FunParam: &struct01
      RefType: &struct01
        SimpleType: struct01
    SimpleType: Int
    Block: Int
      FieldAccess: Int
        FieldAccess: &struct01
          Ident: &struct01
---
struct01 = Node<Int> { value Int, next &struct01 }
fun01    = fun(&struct01) Int
struct03 = Node<T> { value T, next &struct03 }
struct02 = Node { value T, next &struct03 }
```

**Generic struct shadowed**

```metall
{
    struct Foo<T> { one T }
    let a = Foo<Int>(1)
    _ = {
        struct Foo<T> { one T two T }
        let b = Foo<Int>(2, 3)
        b.two
    }
    a.one
}
```

```types
Block: Int
  Struct: struct01
    TypeParam: ?
    StructField: ?
      SimpleType: ?
  Var: void
    TypeConstruction: struct02
      Ident: struct02
      Int: Int
  Assign: void
    Ident: ?
    Block: Int
      Struct: struct03
        TypeParam: ?
        StructField: ?
          SimpleType: ?
        StructField: ?
          SimpleType: ?
      Var: void
        TypeConstruction: struct04
          Ident: struct04
          Int: Int
          Int: Int
      FieldAccess: Int
        Ident: struct04
  FieldAccess: Int
    Ident: struct02
---
struct01 = Foo { one T }
struct02 = Foo<Int> { one Int }
struct03 = Foo { one T, two T }
struct04 = Foo<Int> { one Int, two Int }
```

**Generic struct nested type arg**

```metall
{
    struct Foo<T> { value T }
    struct Bar<T> { inner Foo<T> }
    let x = Bar<Int>(Foo<Int>(42))
    x.inner.value
}
```

```types
Block: Int
  Struct: struct01
    TypeParam: ?
    StructField: ?
      SimpleType: ?
  Struct: struct02
    TypeParam: ?
    StructField: ?
      SimpleType: ?
        SimpleType: ?
  Var: void
    TypeConstruction: struct04
      Ident: struct04
      TypeConstruction: struct05
        Ident: struct05
        Int: Int
  FieldAccess: Int
    FieldAccess: struct05
      Ident: struct04
---
struct01 = Foo { value T }
struct03 = Foo<T> { value T }
struct02 = Bar { inner struct03 }
struct05 = Foo<Int> { value Int }
struct04 = Bar<Int> { inner struct05 }
```

## Generics - Functions

**Generic fun**

```metall
{
    struct Box<T> { value T }
    fun id<T>(x T) T { x }
    _ = id<Int>(42)
    _ = id<Str>("hello")
    let b = id<Box<Int>>(Box<Int>(99))
    b.value
}
```

```types
Block: Int
  Struct: struct01
    TypeParam: ?
    StructField: ?
      SimpleType: ?
  Fun: fun01
    TypeParam: T
    FunParam: T
      SimpleType: T
    SimpleType: T
    Block: T
      Ident: T
  Assign: void
    Ident: ?
    Call: Int
      Ident: fun02
      Int: Int
  Assign: void
    Ident: ?
    Call: Str
      Ident: fun03
      String: Str
  Var: void
    Call: struct02
      Ident: fun04
      TypeConstruction: struct02
        Ident: struct02
        Int: Int
  FieldAccess: Int
    Ident: struct02
---
struct01 = Box { value T }
fun01    = fun(T) T
fun02    = fun(Int) Int
fun03    = fun(Str) Str
struct02 = Box<Int> { value Int }
fun04    = fun(struct02) struct02
```

**Generic fun two type params**

```metall
{
    fun first<A, B>(a A, b B) A { a }
    first<Int, Str>(1, "x")
}
```

```types
Block: Int
  Fun: fun01
    TypeParam: A
    TypeParam: B
    FunParam: A
      SimpleType: A
    FunParam: B
      SimpleType: B
    SimpleType: A
    Block: A
      Ident: A
  Call: Int
    Ident: fun02
    Int: Int
    String: Str
---
fun01 = fun(A, B) A
fun02 = fun(Int, Str) Int
```

**Generic fun dedup same type arg**

```metall
{
    fun id<T>(x T) T { x }
    _ = id<Int>(1)
    id<Int>(2)
}
```

```types
Block: Int
  Fun: fun01
    TypeParam: T
    FunParam: T
      SimpleType: T
    SimpleType: T
    Block: T
      Ident: T
  Assign: void
    Ident: ?
    Call: Int
      Ident: fun02
      Int: Int
  Call: Int
    Ident: fun02
    Int: Int
---
fun01 = fun(T) T
fun02 = fun(Int) Int
```

**Method on generic struct**

```metall
{
    struct Foo<T> { one T }
    fun Foo.bar(f Foo, a T, b Bool) T { if b { return f.one } a }
    let x = Foo<Int>(42)
    x.bar(123, true)
}
```

```types
Block: Int
  Struct: struct01
    TypeParam: ?
    StructField: ?
      SimpleType: ?
  Fun: fun01
    TypeParam: T
    FunParam: struct02
      SimpleType: struct02
        SimpleType: T
    FunParam: T
      SimpleType: T
    FunParam: Bool
      SimpleType: Bool
    SimpleType: T
    Block: T
      If: void
        Ident: Bool
        Block: never
          Return: never
            FieldAccess: T
              Ident: struct02
      Ident: T
  Var: void
    TypeConstruction: struct03
      Ident: struct03
      Int: Int
  Call: Int
    FieldAccess: fun02
      Ident: struct03
    Int: Int
    Bool: Bool
---
struct01 = Foo { one T }
struct02 = Foo<T> { one T }
fun01    = fun(struct02, T, Bool) T
struct03 = Foo<Int> { one Int }
fun02    = fun(struct03, Int, Bool) Int
```

**Generic method on non-generic struct**

```metall
{
    struct Foo { value Int }
    fun Foo.get<T>(f Foo, x T) T { x }
    let f = Foo(42)
    f.get<Str>("hello")
}
```

```types
Block: Str
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Fun: fun01
    TypeParam: T
    FunParam: struct01
      SimpleType: struct01
    FunParam: T
      SimpleType: T
    SimpleType: T
    Block: T
      Ident: T
  Var: void
    TypeConstruction: struct01
      Ident: struct01
      Int: Int
  Call: Str
    FieldAccess: fun02
      Ident: struct01
    String: Str
---
struct01 = Foo { value Int }
fun01    = fun(struct01, T) T
fun02    = fun(struct01, Str) Str
```

**Generic method with extra type param on generic struct**

```metall
{
    struct Foo<T> { one T }
    fun Foo.bar<U>(f Foo, a U) U { a }
    let x = Foo<Int>(42)
    x.bar<Str>("hello")
}
```

```types
Block: Str
  Struct: struct01
    TypeParam: ?
    StructField: ?
      SimpleType: ?
  Fun: fun01
    TypeParam: T
    TypeParam: U
    FunParam: struct02
      SimpleType: struct02
        SimpleType: T
    FunParam: U
      SimpleType: U
    SimpleType: U
    Block: U
      Ident: U
  Var: void
    TypeConstruction: struct03
      Ident: struct03
      Int: Int
  Call: Str
    FieldAccess: fun02
      Ident: struct03
    String: Str
---
struct01 = Foo { one T }
struct02 = Foo<T> { one T }
fun01    = fun(struct02, U) U
struct03 = Foo<Int> { one Int }
fun02    = fun(struct03, Str) Str
```

**Generic fun calls generic fun**

```metall
{
    fun id<T>(x T) T { x }
    fun wrap<T>(x T) T { id<T>(x) }
    wrap<Int>(42)
}
```

```types
Block: Int
  Fun: fun01
    TypeParam: T
    FunParam: T
      SimpleType: T
    SimpleType: T
    Block: T
      Ident: T
  Fun: fun02
    TypeParam: T
    FunParam: T
      SimpleType: T
    SimpleType: T
    Block: T
      Call: T
        Ident: fun03
          SimpleType: T
        Ident: T
  Call: Int
    Ident: fun04
    Int: Int
---
fun01 = fun(T) T
fun02 = fun(T) T
fun03 = fun(T) T
fun04 = fun(Int) Int
```

**Generic fun shadowing**

```metall
{
    fun foo<T>(x T) T { x }
    let a = foo<Int>(1)
    {
        fun foo<T>(x T) Int { 99 }
        let b = foo<Str>("hi")
        b
    }
}
```

```types
Block: Int
  Fun: fun01
    TypeParam: T
    FunParam: T
      SimpleType: T
    SimpleType: T
    Block: T
      Ident: T
  Var: void
    Call: Int
      Ident: fun02
      Int: Int
  Block: Int
    Fun: fun03
      TypeParam: T
      FunParam: T
        SimpleType: T
      SimpleType: Int
      Block: Int
        Int: Int
    Var: void
      Call: Int
        Ident: fun04
        String: Str
    Ident: Int
---
fun01 = fun(T) T
fun02 = fun(Int) Int
fun03 = fun(T) Int
fun04 = fun(Str) Int
```

**Materialized generic resolves shadowed param to concrete type**

```metall
{ 
    fun foo<T>(it T) T {
        mut it = it
        it 
    }

    foo(42)
}
```

```error
```

**Generic fun creates struct from type param**

```metall
{
    struct Box<T> { value T }
    fun box<T>(x T) Box<T> { Box<T>(x) }
    let b = box<Int>(42)
    b.value
}
```

```types
Block: Int
  Struct: struct01
    TypeParam: ?
    StructField: ?
      SimpleType: ?
  Fun: fun01
    TypeParam: T
    FunParam: T
      SimpleType: T
    SimpleType: struct02
      SimpleType: T
    Block: struct02
      TypeConstruction: struct02
        Ident: struct02
          SimpleType: T
        Ident: T
  Var: void
    Call: struct03
      Ident: fun02
      Int: Int
  FieldAccess: Int
    Ident: struct03
---
struct01 = Box { value T }
struct02 = Box<T> { value T }
fun01    = fun(T) struct02
struct03 = Box<Int> { value Int }
fun02    = fun(Int) struct03
```

**Generic fun with ref param**

```metall
{
    fun deref<T>(x &T) T { x.* }
    let x = 42
    deref<Int>(&x)
}
```

```types
Block: Int
  Fun: fun01
    TypeParam: T
    FunParam: &T
      RefType: &T
        SimpleType: T
    SimpleType: T
    Block: T
      Deref: T
        Ident: &T
  Var: void
    Int: Int
  Call: Int
    Ident: fun02
    Ref: &Int
      Ident: Int
---
fun01 = fun(&T) T
fun02 = fun(&Int) Int
```

**Generic fun as value**

```metall
{
    fun id<T>(x T) T { x }
    let f = id<Int>
    f(42)
}
```

```types
Block: Int
  Fun: fun01
    TypeParam: T
    FunParam: T
      SimpleType: T
    SimpleType: T
    Block: T
      Ident: T
  Var: void
    Ident: fun02
  Call: Int
    Ident: fun02
    Int: Int
---
fun01 = fun(T) T
fun02 = fun(Int) Int
```

**Generic method as value**

```metall
{
    struct Foo { value Int }
    fun Foo.get<T>(f Foo, x T) T { x }
    let g = Foo.get<Str>
    _ = g(Foo(1), "hello")
    let h = Foo.get<Int>
    h(Foo(1), 42)
}
```

```types
Block: Int
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Fun: fun01
    TypeParam: T
    FunParam: struct01
      SimpleType: struct01
    FunParam: T
      SimpleType: T
    SimpleType: T
    Block: T
      Ident: T
  Var: void
    Ident: fun02
  Assign: void
    Ident: ?
    Call: Str
      Ident: fun02
      TypeConstruction: struct01
        Ident: struct01
        Int: Int
      String: Str
  Var: void
    Ident: fun03
  Call: Int
    Ident: fun03
    TypeConstruction: struct01
      Ident: struct01
      Int: Int
    Int: Int
---
struct01 = Foo { value Int }
fun01    = fun(struct01, T) T
fun02    = fun(struct01, Str) Str
fun03    = fun(struct01, Int) Int
```

**Generic fun with mut ref param**

```metall
{
    fun set<T>(x &mut T, v T) void { x.* = v }
    mut x = 1
    set<Int>(&mut x, 42)
}
```

```types
Block: void
  Fun: fun01
    TypeParam: T
    FunParam: &mut T
      RefType: &mut T
        SimpleType: T
    FunParam: T
      SimpleType: T
    SimpleType: void
    Block: void
      Assign: void
        Deref: T
          Ident: &mut T
        Ident: T
  Var: void
    Int: Int
  Call: void
    Ident: fun02
    Ref: &mut Int
      Ident: Int
    Int: Int
---
fun01 = fun(&mut T, T) void
fun02 = fun(&mut Int, Int) void
```

**Generic method with ref to generic struct**

```metall
{
    struct Box<V> { value V }
    fun Box.get(b &Box) V { b.value }
    fun wrap<V>(b &Box<V>) V { b.get() }
    let b = Box<Int>(42)
    wrap<Int>(&b)
}
```

```types
Block: Int
  Struct: struct01
    TypeParam: ?
    StructField: ?
      SimpleType: ?
  Fun: fun01
    TypeParam: V
    FunParam: &struct02
      RefType: &struct02
        SimpleType: struct02
          SimpleType: V
    SimpleType: V
    Block: V
      FieldAccess: V
        Ident: &struct02
  Fun: fun02
    TypeParam: V
    FunParam: &struct03
      RefType: &struct03
        SimpleType: struct03
          SimpleType: V
    SimpleType: V
    Block: V
      Call: V
        FieldAccess: fun03
          Ident: &struct03
  Var: void
    TypeConstruction: struct04
      Ident: struct04
      Int: Int
  Call: Int
    Ident: fun04
    Ref: &struct04
      Ident: struct04
---
struct01 = Box { value V }
struct02 = Box<V> { value V }
fun01    = fun(&struct02) V
struct03 = Box<V> { value V }
fun02    = fun(&struct03) V
fun03    = fun(&struct03) V
struct04 = Box<Int> { value Int }
fun04    = fun(&struct04) Int
```

**Generic fun with slice of type param**

```metall
{
    struct Bag<V> { items []V }
    fun Bag.len(b &Bag) Int { b.items.len }
    fun count<V>(b &Bag<V>) Int { b.len() }
    let @a = Arena()
    let items = @a.slice<Str>(2, "")
    let b = Bag<Str>(items)
    count<Str>(&b)
}
```

```types
Block: Int
  Struct: struct01
    TypeParam: ?
    StructField: ?
      SliceType: ?
        SimpleType: ?
  Fun: fun01
    TypeParam: V
    FunParam: &struct02
      RefType: &struct02
        SimpleType: struct02
          SimpleType: V
    SimpleType: Int
    Block: Int
      FieldAccess: Int
        FieldAccess: []V
          Ident: &struct02
  Fun: fun02
    TypeParam: V
    FunParam: &struct03
      RefType: &struct03
        SimpleType: struct03
          SimpleType: V
    SimpleType: Int
    Block: Int
      Call: Int
        FieldAccess: fun03
          Ident: &struct03
  AllocatorVar: void
    TypeConstruction: Arena
      Ident: Arena
  Var: void
    Call: []mut Str
      FieldAccess: fun04
        Ident: Arena
        SimpleType: Str
      Int: Int
      String: Str
  Var: void
    TypeConstruction: struct04
      Ident: struct04
      Ident: []mut Str
  Call: Int
    Ident: fun05
    Ref: &struct04
      Ident: struct04
---
struct01 = Bag { items []V }
struct02 = Bag<V> { items []V }
fun01    = fun(&struct02) Int
struct03 = Bag<V> { items []V }
fun02    = fun(&struct03) Int
fun03    = fun(&struct03) Int
fun04    = fun(Arena, Int, Str) []mut Str
struct04 = Bag<Str> { items []Str }
fun05    = fun(&struct04) Int
```

**Generic fun with fun-typed param**

```metall
{
    fun apply<T>(x T, f fun(T) Int) Int { f(x) }
    fun to_len(s Str) Int { s.data.len }
    apply<Str>("hi", to_len)
}
```

```types
Block: Int
  Fun: fun01
    TypeParam: T
    FunParam: T
      SimpleType: T
    FunParam: fun02
      FunType: fun02
        SimpleType: T
        SimpleType: Int
    SimpleType: Int
    Block: Int
      Call: Int
        Ident: fun02
        Ident: T
  Fun: fun03
    FunParam: Str
      SimpleType: Str
    SimpleType: Int
    Block: Int
      FieldAccess: Int
        FieldAccess: []U8
          Ident: Str
  Call: Int
    Ident: fun04
    String: Str
    Ident: fun03
---
fun02 = fun(T) Int
fun01 = fun(T, fun02) Int
fun03 = sync fun(Str) Int
fun05 = fun(Str) Int
fun04 = fun(Str, fun05) Int
```

**Polymorphic recursion is bounded, not infinite**

Each call wraps the argument one level deeper (`Box<Box<...>>`), so the
monomorphizer would instantiate forever; the depth limit stops it with a
diagnostic instead of hanging.

```metall module
struct Box<T> { v T }

fun depth<T>(x T, n Int) Int {
    if n == 0 { return 0 }
    depth(Box(x), n - 1) + 1
}

fun main() void {
    DebugIntern.print_int(depth(1, 5))
}
```

```error
test.met:5:5: generic instantiation of depth nests deeper than 64 levels; likely unbounded recursion
        if n == 0 { return 0 }
        depth(Box(x), n - 1) + 1
        ^^^^^
    }
```

**Constructing a phantom type param is rejected**

`T` appears in no field, so a value cannot pin it down. Without an explicit type
argument or hint it is unresolvable, rejected here instead of reaching codegen as
an unsized type.

```metall module
struct Box<T> {
    value Int
}

fun main() void {
    let _ = Box(42)
}
```

```error
test.met:6:13: cannot infer type arguments for Box
    fun main() void {
        let _ = Box(42)
                ^^^^^^^
    }
```

## Template Shorthand Syntax

**Bare owner param and return carry owner params**

```metall
{
    struct Box<T> { value T }
    fun Box.get(b Box) T { b.value }
    fun Box.keep(b Box) Box { b }
    let x Int = Box<Int>(42).get()
    let y Str = Box<Str>("hi").get()
    let z Int = Box<Int>(7).keep().value
}
```

```error
```

**Method-local param must be declared**

```metall
{
    struct Box<T> { value T }
    fun Box.replace(b Box, value U) Box { Box(value) }
}
```

```error
test.met:3:34: symbol not defined: U
        struct Box<T> { value T }
        fun Box.replace(b Box, value U) Box { Box(value) }
                                     ^
    }
```

**Declared method-local param can return different owner type**

```metall
{
    struct Box<T> { value T }
    fun Box.replace<U>(b Box, value U) Box<U> { Box(value) }
    let x Str = Box<Int>(42).replace("hi").value
}
```

```error
```

**Explicit method param is added after owner params**

```metall
{
    struct Pair<A, B> { first A second B }
    fun Pair.with_first<C>(p Pair, value C) Pair<C, B> { Pair(value, p.second) }
    let x Str = Pair<Int, Str>(1, "tail").with_first("head").first
}
```

```error
```

**Owner param can be used without a bare owner type**

```metall
{
    struct Box<T> { value T }
    fun Box.echo(value T) T { value }
    let x Int = Box.echo(42)
}
```

```error
```

**Owner param keeps its shape constraint**

```metall
{
    shape Same { fun Same.same(a Same, b Same) Bool }
    struct Score { n Int }
    fun Score.same(a Score, b Score) Bool { a.n == b.n }
    struct EqualBox<T Same> { value T }
    fun EqualBox.same(a EqualBox, b EqualBox) Bool { a.value.same(b.value) }
    let x Bool = EqualBox(Score(1)).same(EqualBox(Score(1)))
}
```

```error
```

**Explicit method param can type a concrete owner**

```metall
{
    shape Same { fun Same.same(a Same, b Same) Bool }
    struct Score { n Int }
    fun Score.same(a Score, b Score) Bool { a.n == b.n }
    struct Box<T> { value T }
    fun Box.same<U Same>(a Box<U>, b Box<U>) Bool { a.value.same(b.value) }
    let x Bool = Box<Score>(Score(1)).same(Box<Score>(Score(1)))
}
```

```error
```

**Shape member shorthand carries shape params**

```metall
{
    shape Cell<Value> { fun Cell.get(c Cell) Value }
    struct IntCell { value Int }
    fun IntCell.get(c IntCell) Int { c.value }
    fun read<C Cell>(c C) C.Value { c.get() }
    let x Int = read(IntCell(5))
}
```

```error
```

**Attached slot used in field and function types**

```metall
{
    shape Cell<Value> { fun Cell.get(c Cell) Value }
    struct IntCell { value Int }
    fun IntCell.get(c IntCell) Int { c.value }

    struct Mapped<In Cell, Out> {
        input In
        map fun(In.Value) Out
    }
    fun Mapped.value(m Mapped) Out { m.map(m.input.get()) }

    fun add1(x Int) Int { x + 1 }
    let x Int = Mapped(IntCell(4), add1).value()
}
```

```error
```

**Attached slot used as another constraint argument**

```metall
{
    shape Cell<Value> { fun Cell.get(c Cell) Value }
    struct IntCell { value Int }
    fun IntCell.get(c IntCell) Int { c.value }
    struct OtherIntCell { value Int }
    fun OtherIntCell.get(c OtherIntCell) Int { c.value }

    struct SameValuePair<Left Cell, Right Cell<Left.Value>> {
        left Left
        right Right
    }
    fun SameValuePair.right_value(p SameValuePair) Left.Value { p.right.get() }

    let x Int = SameValuePair(IntCell(1), OtherIntCell(2)).right_value()
}
```

```error
```

**Associated slot projection must be introduced first**

```metall
{
    shape Cell<Value> { fun Cell.get(c Cell) Value }
    struct Pair<Right Cell<Left.Value>, Left Cell> {
        right Right
        left Left
    }
}
```

```error
test.met:3:28: unknown associated type: Left.Value
        shape Cell<Value> { fun Cell.get(c Cell) Value }
        struct Pair<Right Cell<Left.Value>, Left Cell> {
                               ^^^^^^^^^^
            right Right
```

**Explicit shape argument introduces associated slot**

```metall
{
    shape Source<Item> { fun Source.next(s &mut Source) ?Item }
    struct Counter { value Int }
    fun Counter.next(c &mut Counter) ?Int { c.value }
    fun first<S Source<Int>>(s &mut S) ?S.Item { s.next() }
    mut c = Counter(3)
    let x ?Int = first(&mut c)
}
```

```error
```

**Explicit shape argument fixes associated slot**

```metall
{
    shape Source<Item> { fun Source.next(s &mut Source) ?Item }
    struct Counter { value Int }
    fun Counter.next(c &mut Counter) ?Int { c.value }
    fun use_item<S Source<Int>>(s &mut S, value S.Item) void {
        _ = s
        _ = value
    }
    mut c = Counter(3)
    use_item(&mut c, "wrong")
}
```

```error
test.met:10:22: type mismatch at argument 2: expected Int, got Str
        mut c = Counter(3)
        use_item(&mut c, "wrong")
                         ^^^^^^^
    }
```

**Attached slots can be chained**

```metall
{
    shape Cell<Value> { fun Cell.get(c Cell) Value }
    shape Holder<Inner Cell> { fun Holder.inner(h Holder) Inner }
    struct IntCell { value Int }
    fun IntCell.get(c IntCell) Int { c.value }
    struct IntHolder { cell IntCell }
    fun IntHolder.inner(h IntHolder) IntCell { h.cell }

    struct Boxed<H Holder> { holder H }
    fun Boxed.value(b Boxed) H.Inner.Value { b.holder.inner().get() }

    let x Int = Boxed(IntHolder(IntCell(7))).value()
}
```

```error
```

**Open shape slot is rejected**

```metall
{
    shape Marker<Item> {}
}
```

```error
test.met:2:18: open shape slot: Item is not used by the shape contract
    {
        shape Marker<Item> {}
                     ^^^^
    }
```

**Unknown associated type projection is rejected**

```metall
{
    shape Source<T> { fun Source.next(s &mut Source) ?T }
    struct Counter { value Int }
    fun Counter.next(c &mut Counter) ?Int { c.value }
    struct Pipeline<In Source> { source In }
    fun Pipeline.next(p &mut Pipeline) ?In.Item {
        (&mut p.source).next()
    }
}
```

```error
test.met:6:41: unknown associated type: In.Item
        struct Pipeline<In Source> { source In }
        fun Pipeline.next(p &mut Pipeline) ?In.Item {
                                            ^^^^^^^
            (&mut p.source).next()
```

**Associated type constraint reports shape mismatch**

```metall
{
    shape Source<Item> { fun Source.next(s &mut Source) ?Item }
    struct Counter { value Int }
    fun Counter.next(c &mut Counter) ?Int { c.value }
    fun wants_str<S Source<Str>>(s &mut S) void {
        _ = s
    }
    mut c = Counter(8)
    wants_str(&mut c)
}
```

```error
test.met:9:5: type mismatch: expected Source<Str>, got Source<Int>
        mut c = Counter(8)
        wants_str(&mut c)
        ^^^^^^^^^^^^^^^^^
    }
```

**Type parameter constraint reports associated type mismatch**

```metall
{
    shape Source<Item> { fun Source.next(s &mut Source) ?Item }
    fun needs_int<S Source<Int>>(s &mut S) void {
        _ = s
    }
    fun forwards_str<S Source<Str>>(s &mut S) void {
        needs_int<S>(s)
    }
}
```

```error
test.met:7:9: type mismatch: expected Source<Int>, got Source<Str>
        fun forwards_str<S Source<Str>>(s &mut S) void {
            needs_int<S>(s)
            ^^^^^^^^^^^^
        }
```

**Shape subset checks associated type values**

```metall
{
    shape Source<Item> { fun Source.next(s &mut Source) ?Item }
    shape Rewindable<Item> {
        fun Rewindable.next(s &mut Rewindable) ?Item
        fun Rewindable.rewind(s &mut Rewindable) void
    }
    fun needs_int<S Source<Int>>(s &mut S) void {
        _ = s
    }
    fun forwards_rewindable_str<S Rewindable<Str>>(s &mut S) void {
        needs_int<S>(s)
    }
}
```

```error
test.met:11:9: type mismatch: expected Source<Int>, got Rewindable<Str>
        fun forwards_rewindable_str<S Rewindable<Str>>(s &mut S) void {
            needs_int<S>(s)
            ^^^^^^^^^^^^
        }
```

**Explicit method param does not replace owner param**

```metall
{
    shape Same { fun Same.same(a Same, b Same) Bool }
    struct Box<T> { value T }
    fun Box.same<T Same>(a Box, b Box) Bool { true }
}
```

```error
test.met:4:18: type parameter T shadows owner type parameter
        struct Box<T> { value T }
        fun Box.same<T Same>(a Box, b Box) Bool { true }
                     ^
    }
```

## Shapes

**Shape method call**

```metall
{
    shape Showable {
        fun Showable.show(self Showable) Str
    }
    struct Guitar {
        name Str
    }
    fun Guitar.show(g Guitar) Str { g.name }
    fun display<T Showable>(t T) Str { t.show() }
    display<Guitar>(Guitar("Telecaster"))
}
```

```types
Block: Str
  Shape: shape01
    FunDecl: ?
      FunParam: shape01
        SimpleType: shape01
      SimpleType: Str
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Fun: fun01
    FunParam: struct01
      SimpleType: struct01
    SimpleType: Str
    Block: Str
      FieldAccess: Str
        Ident: struct01
  Fun: fun02
    TypeParam: T
      SimpleType: shape01
    FunParam: T
      SimpleType: T
    SimpleType: Str
    Block: Str
      Call: Str
        FieldAccess: fun03
          Ident: T
  Call: Str
    Ident: fun04
    TypeConstruction: struct01
      Ident: struct01
      String: Str
---
shape01  = Showable {  }
struct01 = Guitar { name Str }
fun01    = sync fun(struct01) Str
fun02    = fun(T) Str
fun03    = fun(T) Str
fun04    = fun(struct01) Str
```

**Shape method call with ref receiver**

```metall
{
    shape HasValue {
        fun HasValue.val(self &HasValue) Int
    }
    struct Foo { one Int }
    fun Foo.val(self &Foo) Int { self.one }
    fun peek<T HasValue>(t &T) Int { t.val() }
    let f = Foo(42)
    peek<Foo>(&f)
}
```

```types
Block: Int
  Shape: shape01
    FunDecl: ?
      FunParam: &shape01
        RefType: &shape01
          SimpleType: shape01
      SimpleType: Int
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Fun: fun01
    FunParam: &struct01
      RefType: &struct01
        SimpleType: struct01
    SimpleType: Int
    Block: Int
      FieldAccess: Int
        Ident: &struct01
  Fun: fun02
    TypeParam: T
      SimpleType: shape01
    FunParam: &T
      RefType: &T
        SimpleType: T
    SimpleType: Int
    Block: Int
      Call: Int
        FieldAccess: fun03
          Ident: &T
  Var: void
    TypeConstruction: struct01
      Ident: struct01
      Int: Int
  Call: Int
    Ident: fun04
    Ref: &struct01
      Ident: struct01
---
shape01  = HasValue {  }
struct01 = Foo { one Int }
fun01    = fun(&struct01) Int
fun02    = fun(&T) Int
fun03    = fun(&T) Int
fun04    = fun(&struct01) Int
```

**Shape method call with mut ref to immutable ref coercion**

```metall
{
    shape HasValue {
        fun HasValue.val(self &HasValue) Int
    }
    struct Foo { one Int }
    fun Foo.val(self &Foo) Int { self.one }
    fun peek<T HasValue>(t &mut T) Int { t.val() }
    mut f = Foo(42)
    peek<Foo>(&mut f)
}
```

```types
Block: Int
  Shape: shape01
    FunDecl: ?
      FunParam: &shape01
        RefType: &shape01
          SimpleType: shape01
      SimpleType: Int
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Fun: fun01
    FunParam: &struct01
      RefType: &struct01
        SimpleType: struct01
    SimpleType: Int
    Block: Int
      FieldAccess: Int
        Ident: &struct01
  Fun: fun02
    TypeParam: T
      SimpleType: shape01
    FunParam: &mut T
      RefType: &mut T
        SimpleType: T
    SimpleType: Int
    Block: Int
      Call: Int
        FieldAccess: fun03
          Ident: &mut T
  Var: void
    TypeConstruction: struct01
      Ident: struct01
      Int: Int
  Call: Int
    Ident: fun04
    Ref: &mut struct01
      Ident: struct01
---
shape01  = HasValue {  }
struct01 = Foo { one Int }
fun01    = fun(&struct01) Int
fun02    = fun(&mut T) Int
fun03    = fun(&T) Int
fun04    = fun(&mut struct01) Int
```

**Shape two constrained type params**

```metall
{
    shape HasName { fun HasName.name(self HasName) Str }
    shape HasAge { fun HasAge.age(self HasAge) Int }
    struct Foo { n Str }
    struct Bar { a Int }
    fun Foo.name(f Foo) Str { f.n }
    fun Bar.age(b Bar) Int { b.a }
    fun combine<A HasName, B HasAge>(a A, b B) Str { a.name() }
    combine<Foo, Bar>(Foo("x"), Bar(1))
}
```

```types
Block: Str
  Shape: shape01
    FunDecl: ?
      FunParam: shape01
        SimpleType: shape01
      SimpleType: Str
  Shape: shape02
    FunDecl: ?
      FunParam: shape02
        SimpleType: shape02
      SimpleType: Int
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Struct: struct02
    StructField: ?
      SimpleType: ?
  Fun: fun01
    FunParam: struct01
      SimpleType: struct01
    SimpleType: Str
    Block: Str
      FieldAccess: Str
        Ident: struct01
  Fun: fun02
    FunParam: struct02
      SimpleType: struct02
    SimpleType: Int
    Block: Int
      FieldAccess: Int
        Ident: struct02
  Fun: fun03
    TypeParam: A
      SimpleType: shape01
    TypeParam: B
      SimpleType: shape02
    FunParam: A
      SimpleType: A
    FunParam: B
      SimpleType: B
    SimpleType: Str
    Block: Str
      Call: Str
        FieldAccess: fun04
          Ident: A
  Call: Str
    Ident: fun05
    TypeConstruction: struct01
      Ident: struct01
      String: Str
    TypeConstruction: struct02
      Ident: struct02
      Int: Int
---
shape01  = HasName {  }
shape02  = HasAge {  }
struct01 = Foo { n Str }
struct02 = Bar { a Int }
fun01    = sync fun(struct01) Str
fun02    = sync fun(struct02) Int
fun03    = fun(A, B) Str
fun04    = fun(A) Str
fun05    = fun(struct01, struct02) Str
```

**Shape method returns self type**

```metall
{
    shape Clonable {
        fun Clonable.clone(self Clonable) Clonable
    }
    struct Foo { x Int }
    fun Foo.clone(f Foo) Foo { Foo(f.x) }
    fun dup<T Clonable>(t T) T { t.clone() }
    dup<Foo>(Foo(1))
}
```

```types
Block: struct01
  Shape: shape01
    FunDecl: ?
      FunParam: shape01
        SimpleType: shape01
      SimpleType: shape01
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Fun: fun01
    FunParam: struct01
      SimpleType: struct01
    SimpleType: struct01
    Block: struct01
      TypeConstruction: struct01
        Ident: struct01
        FieldAccess: Int
          Ident: struct01
  Fun: fun02
    TypeParam: T
      SimpleType: shape01
    FunParam: T
      SimpleType: T
    SimpleType: T
    Block: T
      Call: T
        FieldAccess: fun03
          Ident: T
  Call: struct01
    Ident: fun04
    TypeConstruction: struct01
      Ident: struct01
      Int: Int
---
struct01 = Foo { x Int }
shape01  = Clonable {  }
fun01    = sync fun(struct01) struct01
fun02    = fun(T) T
fun03    = fun(T) T
fun04    = fun(struct01) struct01
```

**Shape method with two shape params**

```metall
{
    shape Eq {
        fun Eq.eq(self Eq, other Eq) Bool
    }
    struct Num { x Int }
    fun Num.eq(a Num, b Num) Bool { a.x == b.x }
    fun same<T Eq>(a T, b T) Bool { a.eq(b) }
    same<Num>(Num(1), Num(1))
}
```

```types
Block: Bool
  Shape: shape01
    FunDecl: ?
      FunParam: shape01
        SimpleType: shape01
      FunParam: shape01
        SimpleType: shape01
      SimpleType: Bool
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Fun: fun01
    FunParam: struct01
      SimpleType: struct01
    FunParam: struct01
      SimpleType: struct01
    SimpleType: Bool
    Block: Bool
      Binary: Bool
        FieldAccess: Int
          Ident: struct01
        FieldAccess: Int
          Ident: struct01
  Fun: fun02
    TypeParam: T
      SimpleType: shape01
    FunParam: T
      SimpleType: T
    FunParam: T
      SimpleType: T
    SimpleType: Bool
    Block: Bool
      Call: Bool
        FieldAccess: fun03
          Ident: T
        Ident: T
  Call: Bool
    Ident: fun04
    TypeConstruction: struct01
      Ident: struct01
      Int: Int
    TypeConstruction: struct01
      Ident: struct01
      Int: Int
---
shape01  = Eq {  }
struct01 = Num { x Int }
fun01    = sync fun(struct01, struct01) Bool
fun02    = fun(T, T) Bool
fun03    = fun(T, T) Bool
fun04    = fun(struct01, struct01) Bool
```

**Shape two structs same shape**

```metall
{
    shape Showable {
        fun Showable.show(self Showable) Str
    }
    struct A { }
    struct B { }
    fun A.show(a A) Str { "a" }
    fun B.show(b B) Str { "b" }
    fun display<T Showable>(t T) Str { t.show() }
    let x = display<A>(A())
    display<B>(B())
}
```

```types
Block: Str
  Shape: shape01
    FunDecl: ?
      FunParam: shape01
        SimpleType: shape01
      SimpleType: Str
  Struct: struct01
  Struct: struct02
  Fun: fun01
    FunParam: struct01
      SimpleType: struct01
    SimpleType: Str
    Block: Str
      String: Str
  Fun: fun02
    FunParam: struct02
      SimpleType: struct02
    SimpleType: Str
    Block: Str
      String: Str
  Fun: fun03
    TypeParam: T
      SimpleType: shape01
    FunParam: T
      SimpleType: T
    SimpleType: Str
    Block: Str
      Call: Str
        FieldAccess: fun04
          Ident: T
  Var: void
    Call: Str
      Ident: fun05
      TypeConstruction: struct01
        Ident: struct01
  Call: Str
    Ident: fun06
    TypeConstruction: struct02
      Ident: struct02
---
shape01  = Showable {  }
struct01 = A {  }
struct02 = B {  }
fun01    = sync fun(struct01) Str
fun02    = sync fun(struct02) Str
fun03    = fun(T) Str
fun04    = fun(T) Str
fun05    = fun(struct01) Str
fun06    = fun(struct02) Str
```

**Shape multiple methods**

```metall
{
    shape S {
        fun S.foo(self S) Int
        fun S.bar(self S) Str
    }
    struct X { }
    fun X.foo(x X) Int { 1 }
    fun X.bar(x X) Str { "x" }
    fun test<T S>(t T) Str { let n = t.foo() t.bar() }
    test<X>(X())
}
```

```types
Block: Str
  Shape: shape01
    FunDecl: ?
      FunParam: shape01
        SimpleType: shape01
      SimpleType: Int
    FunDecl: ?
      FunParam: shape01
        SimpleType: shape01
      SimpleType: Str
  Struct: struct01
  Fun: fun01
    FunParam: struct01
      SimpleType: struct01
    SimpleType: Int
    Block: Int
      Int: Int
  Fun: fun02
    FunParam: struct01
      SimpleType: struct01
    SimpleType: Str
    Block: Str
      String: Str
  Fun: fun03
    TypeParam: T
      SimpleType: shape01
    FunParam: T
      SimpleType: T
    SimpleType: Str
    Block: Str
      Var: void
        Call: Int
          FieldAccess: fun04
            Ident: T
      Call: Str
        FieldAccess: fun05
          Ident: T
  Call: Str
    Ident: fun06
    TypeConstruction: struct01
      Ident: struct01
---
shape01  = S {  }
struct01 = X {  }
fun01    = sync fun(struct01) Int
fun02    = sync fun(struct01) Str
fun03    = fun(T) Str
fun04    = fun(T) Int
fun05    = fun(T) Str
fun06    = fun(struct01) Str
```

**Shape satisfied by non-struct type**

```metall
{
    shape Displayable {
        fun Displayable.display(self Displayable) Int
    }
    fun Int.display(i Int) Int { i }
    fun show<T Displayable>(t T) Int { t.display() }
    show<Int>(42)
}
```

```types
Block: Int
  Shape: shape01
    FunDecl: ?
      FunParam: shape01
        SimpleType: shape01
      SimpleType: Int
  Fun: fun01
    FunParam: Int
      SimpleType: Int
    SimpleType: Int
    Block: Int
      Ident: Int
  Fun: fun02
    TypeParam: T
      SimpleType: shape01
    FunParam: T
      SimpleType: T
    SimpleType: Int
    Block: Int
      Call: Int
        FieldAccess: fun03
          Ident: T
  Call: Int
    Ident: fun04
    Int: Int
---
shape01 = Displayable {  }
fun01   = sync fun(Int) Int
fun02   = fun(T) Int
fun03   = fun(T) Int
fun04   = fun(Int) Int
```

**Subset enum inherits methods and satisfies shapes via its open root**

```metall
{
    shape Labeled {
        fun Labeled.label(self Labeled) Str
    }
    enum AppErr U32
    enum IOErr AppErr = file_not_found | broken_pipe
    fun AppErr.label(e AppErr) Str { e.debug_name }
    fun describe<T Labeled>(t T) Str { t.label() }
    _ = IOErr.file_not_found.label()
    describe<IOErr>(IOErr.broken_pipe)
}
```

```types
Block: Str
  Shape: shape01
    FunDecl: ?
      FunParam: shape01
        SimpleType: shape01
      SimpleType: Str
  Enum: enum01
    SimpleType: U32
  Enum: enum02
    SimpleType: enum01
    EnumVariant: ?
    EnumVariant: ?
  Fun: fun01
    FunParam: enum01
      SimpleType: enum01
    SimpleType: Str
    Block: Str
      FieldAccess: Str
        Ident: enum01
  Fun: fun02
    TypeParam: T
      SimpleType: shape01
    FunParam: T
      SimpleType: T
    SimpleType: Str
    Block: Str
      Call: Str
        FieldAccess: fun03
          Ident: T
  Assign: void
    Ident: ?
    Call: Str
      FieldAccess: fun01
        Ident: enum02
  Call: Str
    Ident: fun04
    Ident: enum02
---
shape01 = Labeled {  }
enum01  = AppErr
enum02  = IOErr = file_not_found | broken_pipe
fun01   = sync fun(enum01) Str
fun02   = fun(T) Str
fun03   = fun(T) Str
fun04   = fun(enum02) Str
```

**Generic type param unifies same-family enum args to their open root**

```metall
{
    shape HasEq {
        fun HasEq.eq(e HasEq, other HasEq) Bool
    }
    enum AppErr U32
    enum IOErr AppErr = file_not_found | broken_pipe
    fun AppErr.eq(e AppErr, other AppErr) Bool { e == other }
    fun check<T HasEq>(want T, got T) Bool { want.eq(got) }
    let e AppErr = IOErr.file_not_found
    check(IOErr.file_not_found, e)
}
```

```types
Block: Bool
  Shape: shape01
    FunDecl: ?
      FunParam: shape01
        SimpleType: shape01
      FunParam: shape01
        SimpleType: shape01
      SimpleType: Bool
  Enum: enum01
    SimpleType: U32
  Enum: enum02
    SimpleType: enum01
    EnumVariant: ?
    EnumVariant: ?
  Fun: fun01
    FunParam: enum01
      SimpleType: enum01
    FunParam: enum01
      SimpleType: enum01
    SimpleType: Bool
    Block: Bool
      Binary: Bool
        Ident: enum01
        Ident: enum01
  Fun: fun02
    TypeParam: T
      SimpleType: shape01
    FunParam: T
      SimpleType: T
    FunParam: T
      SimpleType: T
    SimpleType: Bool
    Block: Bool
      Call: Bool
        FieldAccess: fun03
          Ident: T
        Ident: T
  Var: void
    SimpleType: enum01
    Ident: enum02
  Call: Bool
    Ident: fun04
    Ident: enum02
    Ident: enum01
---
shape01 = HasEq {  }
enum01  = AppErr
enum02  = IOErr = file_not_found | broken_pipe
fun01   = sync fun(enum01, enum01) Bool
fun02   = fun(T, T) Bool
fun03   = fun(T, T) Bool
fun04   = fun(enum01, enum01) Bool
```

**Type param satisfies its own constraint**

```metall
{
    shape Displayable {
        fun Displayable.display(self Displayable) Int
    }
    fun Int.display(i Int) Int { i }
    fun show<T Displayable>(t T) Int { t.display() }
    fun wrap<K Displayable>(k K) Int { show<K>(k) }
    wrap<Int>(7)
}
```

```types
Block: Int
  Shape: shape01
    FunDecl: ?
      FunParam: shape01
        SimpleType: shape01
      SimpleType: Int
  Fun: fun01
    FunParam: Int
      SimpleType: Int
    SimpleType: Int
    Block: Int
      Ident: Int
  Fun: fun02
    TypeParam: T
      SimpleType: shape01
    FunParam: T
      SimpleType: T
    SimpleType: Int
    Block: Int
      Call: Int
        FieldAccess: fun03
          Ident: T
  Fun: fun04
    TypeParam: K
      SimpleType: shape01
    FunParam: K
      SimpleType: K
    SimpleType: Int
    Block: Int
      Call: Int
        Ident: fun05
          SimpleType: K
        Ident: K
  Call: Int
    Ident: fun06
    Int: Int
---
shape01 = Displayable {  }
fun01   = sync fun(Int) Int
fun02   = fun(T) Int
fun03   = fun(T) Int
fun04   = fun(K) Int
fun05   = fun(K) Int
fun06   = fun(Int) Int
```

**Type param with superset shape constraint satisfies subset shape**

```metall
{
    shape HasFmt {
        fun HasFmt.fmt(self HasFmt, x Int) Int
    }
    shape HasEqFmt {
        fun HasEqFmt.eq(self HasEqFmt, other HasEqFmt) Bool
        fun HasEqFmt.fmt(self HasEqFmt, x Int) Int
    }
    fun Int.fmt(i Int, x Int) Int { i + x }
    fun Int.eq(i Int, other Int) Bool { true }
    fun format<T HasFmt>(t T, x Int) Int { t.fmt(x) }
    fun compare_and_format<T HasEqFmt>(a T, b T, x Int) Int {
        if a.eq(b) { format<T>(a, x) } else { 0 }
    }
    compare_and_format<Int>(1, 2, 3)
}
```

```types
Block: Int
  Shape: shape01
    FunDecl: ?
      FunParam: shape01
        SimpleType: shape01
      FunParam: Int
        SimpleType: Int
      SimpleType: Int
  Shape: shape02
    FunDecl: ?
      FunParam: shape02
        SimpleType: shape02
      FunParam: shape02
        SimpleType: shape02
      SimpleType: Bool
    FunDecl: ?
      FunParam: shape02
        SimpleType: shape02
      FunParam: Int
        SimpleType: Int
      SimpleType: Int
  Fun: fun01
    FunParam: Int
      SimpleType: Int
    FunParam: Int
      SimpleType: Int
    SimpleType: Int
    Block: Int
      Binary: Int
        Ident: Int
        Ident: Int
  Fun: fun02
    FunParam: Int
      SimpleType: Int
    FunParam: Int
      SimpleType: Int
    SimpleType: Bool
    Block: Bool
      Bool: Bool
  Fun: fun03
    TypeParam: T
      SimpleType: shape01
    FunParam: T
      SimpleType: T
    FunParam: Int
      SimpleType: Int
    SimpleType: Int
    Block: Int
      Call: Int
        FieldAccess: fun04
          Ident: T
        Ident: Int
  Fun: fun05
    TypeParam: T
      SimpleType: shape02
    FunParam: T
      SimpleType: T
    FunParam: T
      SimpleType: T
    FunParam: Int
      SimpleType: Int
    SimpleType: Int
    Block: Int
      If: Int
        Call: Bool
          FieldAccess: fun06
            Ident: T
          Ident: T
        Block: Int
          Call: Int
            Ident: fun07
              SimpleType: T
            Ident: T
            Ident: Int
        Block: Int
          Int: Int
  Call: Int
    Ident: fun08
    Int: Int
    Int: Int
    Int: Int
---
shape01 = HasFmt {  }
shape02 = HasEqFmt {  }
fun01   = sync fun(Int, Int) Int
fun02   = sync fun(Int, Int) Bool
fun03   = fun(T, Int) Int
fun04   = fun(T, Int) Int
fun05   = fun(T, T, Int) Int
fun06   = fun(T, T) Bool
fun07   = fun(T, Int) Int
fun08   = fun(Int, Int, Int) Int
```

**Shape satisfied by generic struct**

```metall
{
    shape Displayable {
        fun Displayable.display(self Displayable) Int
    }
    struct Wrapper<T> { value T }
    fun Wrapper.display<U Displayable>(w Wrapper<U>) Int { w.value.display() }
    fun Int.display(i Int) Int { i }
    fun show<T Displayable>(t T) Int { t.display() }
    show<Wrapper<Int>>(Wrapper<Int>(42))
}
```

```types
Block: Int
  Shape: shape01
    FunDecl: ?
      FunParam: shape01
        SimpleType: shape01
      SimpleType: Int
  Struct: struct01
    TypeParam: ?
    StructField: ?
      SimpleType: ?
  Fun: fun01
    TypeParam: U
      SimpleType: shape01
    FunParam: struct02
      SimpleType: struct02
        SimpleType: U
    SimpleType: Int
    Block: Int
      Call: Int
        FieldAccess: fun02
          FieldAccess: U
            Ident: struct02
  Fun: fun03
    FunParam: Int
      SimpleType: Int
    SimpleType: Int
    Block: Int
      Ident: Int
  Fun: fun04
    TypeParam: T
      SimpleType: shape01
    FunParam: T
      SimpleType: T
    SimpleType: Int
    Block: Int
      Call: Int
        FieldAccess: fun05
          Ident: T
  Call: Int
    Ident: fun06
    TypeConstruction: struct03
      Ident: struct03
      Int: Int
---
shape01  = Displayable {  }
struct01 = Wrapper { value T }
struct02 = Wrapper<U> { value U }
fun01    = fun(struct02) Int
fun02    = fun(U) Int
fun03    = sync fun(Int) Int
fun04    = fun(T) Int
fun05    = fun(T) Int
struct03 = Wrapper<Int> { value Int }
fun06    = fun(struct03) Int
```

**Generic shape**

```metall
{
    shape Seq<T> {
        fun Seq.next(it Seq) ?T
    }
    struct Counter { cur Int max Int }
    fun Counter.next(r Counter) ?Int {
        if r.cur >= r.max { return None() }
        Option(r.cur)
    }
    fun sum<T Seq<Int>>(it T) Int { 0 }
    sum<Counter>(Counter(0, 10))
}
```

```types
Block: Int
  Shape: shape01
    TypeParam: ?
    FunDecl: ?
      FunParam: ?
        SimpleType: ?
      SimpleType: ?
        SimpleType: ?
  Struct: struct01
    StructField: ?
      SimpleType: ?
    StructField: ?
      SimpleType: ?
  Fun: fun01
    FunParam: struct01
      SimpleType: struct01
    SimpleType: union01
    Block: union01
      If: void
        Binary: Bool
          FieldAccess: Int
            Ident: struct01
          FieldAccess: Int
            Ident: struct01
        Block: never
          Return: never
            TypeConstruction: struct02
              Ident: struct02
      TypeConstruction: union01
        Ident: union01
        FieldAccess: Int
          Ident: struct01
  Fun: fun02
    TypeParam: T
      SimpleType: shape02
        SimpleType: Int
    FunParam: T
      SimpleType: T
    SimpleType: Int
    Block: Int
      Int: Int
  Call: Int
    Ident: fun03
    TypeConstruction: struct01
      Ident: struct01
      Int: Int
      Int: Int
---
shape01  = Seq {  }
struct01 = Counter { cur Int, max Int }
struct02 = None {  }
union01  = Option<Int> = Int | struct02
fun01    = sync fun(struct01) union01
fun02    = fun(T) Int
shape02  = Seq {  }
fun03    = fun(struct01) Int
```

**Generic shape method call**

```metall
{
    shape Seq<T> {
        fun Seq.next(it &mut Seq) ?T
    }
    struct Nums { i Int }
    fun Nums.next(n &mut Nums) ?Int { let v = n.i n.i = n.i + 1 Option(v) }
    fun first<E, T Seq<E>>(it &mut T) ?E { it.next() }
    mut n = Nums(0)
    first<Int, Nums>(&mut n)
}
```

```types
Block: union01
  Shape: shape01
    TypeParam: ?
    FunDecl: ?
      FunParam: ?
        RefType: ?
          SimpleType: ?
      SimpleType: ?
        SimpleType: ?
  Struct: struct02
    StructField: ?
      SimpleType: ?
  Fun: fun01
    FunParam: &mut struct02
      RefType: &mut struct02
        SimpleType: struct02
    SimpleType: union01
    Block: union01
      Var: void
        FieldAccess: Int
          Ident: &mut struct02
      Assign: void
        FieldAccess: Int
          Ident: &mut struct02
        Binary: Int
          FieldAccess: Int
            Ident: &mut struct02
          Int: Int
      TypeConstruction: union01
        Ident: union01
        Ident: Int
  Fun: fun02
    TypeParam: E
    TypeParam: T
      SimpleType: shape02
        SimpleType: E
    FunParam: &mut T
      RefType: &mut T
        SimpleType: T
    SimpleType: union02
      SimpleType: E
    Block: union02
      Call: union02
        FieldAccess: fun03
          Ident: &mut T
  Var: void
    TypeConstruction: struct02
      Ident: struct02
      Int: Int
  Call: union01
    Ident: fun04
    Ref: &mut struct02
      Ident: struct02
---
struct01 = None {  }
union01  = Option<Int> = Int | struct01
shape01  = Seq {  }
struct02 = Nums { i Int }
fun01    = fun(&mut struct02) union01
union02  = Option<E> = E | struct01
fun02    = fun(&mut T) union02
shape02  = Seq {  }
fun03    = fun(&mut T) union02
fun04    = fun(&mut struct02) union01
```

**Infer type args from shape constraints**

```metall
{
    shape Seq<T> {
        fun Seq.next(it &mut Seq) ?T
    }
    struct Nums { i Int }
    fun Nums.next(n &mut Nums) ?Int { let v = n.i n.i = n.i + 1 Option(v) }
    fun first<E, T Seq<E>>(it &mut T) ?E { it.next() }
    mut n = Nums(0)
    first(&mut n)
}
```

```types
Block: union01
  Shape: shape01
    TypeParam: ?
    FunDecl: ?
      FunParam: ?
        RefType: ?
          SimpleType: ?
      SimpleType: ?
        SimpleType: ?
  Struct: struct02
    StructField: ?
      SimpleType: ?
  Fun: fun01
    FunParam: &mut struct02
      RefType: &mut struct02
        SimpleType: struct02
    SimpleType: union01
    Block: union01
      Var: void
        FieldAccess: Int
          Ident: &mut struct02
      Assign: void
        FieldAccess: Int
          Ident: &mut struct02
        Binary: Int
          FieldAccess: Int
            Ident: &mut struct02
          Int: Int
      TypeConstruction: union01
        Ident: union01
        Ident: Int
  Fun: fun02
    TypeParam: E
    TypeParam: T
      SimpleType: shape02
        SimpleType: E
    FunParam: &mut T
      RefType: &mut T
        SimpleType: T
    SimpleType: union02
      SimpleType: E
    Block: union02
      Call: union02
        FieldAccess: fun03
          Ident: &mut T
  Var: void
    TypeConstruction: struct02
      Ident: struct02
      Int: Int
  Call: union01
    Ident: fun04
    Ref: &mut struct02
      Ident: struct02
---
struct01 = None {  }
union01  = Option<Int> = Int | struct01
shape01  = Seq {  }
struct02 = Nums { i Int }
fun01    = fun(&mut struct02) union01
union02  = Option<E> = E | struct01
fun02    = fun(&mut T) union02
shape02  = Seq {  }
fun03    = fun(&mut T) union02
fun04    = fun(&mut struct02) union01
```

**Generic shape satisfied when type arg is a ref**

```metall module
shape RC<T> {
    fun RC.read(r &RC, buf []T) ![]T
}

struct Buf<T> {}

fun Buf.read(b &Buf, buf []T) ![]T { buf }

fun use_rc<T, R RC<T>>(r R) !Int { 1 }

fun main() !void {
    let b = Buf<Int>()
    _ = try use_rc(&b)
}
```

```error
```

**Generic shape not satisfied**

```metall
{
    shape Seq<T> {
        fun Seq.next(it &mut Seq) ?T
    }
    struct Foo { }
    fun Foo.next(f &mut Foo) ?Str { None() }
    fun first<E, T Seq<E>>(it &mut T) ?E { it.next() }
    mut f = Foo()
    first<Int, Foo>(&mut f)
}
```

```error
test.met:9:5: type mismatch: expected Seq<Int>, got Seq<Str>
        mut f = Foo()
        first<Int, Foo>(&mut f)
        ^^^^^^^^^^^^^^^
    }
```

**Ref type satisfies shape**

```metall
{
    shape S { fun S.eq(a S, b S) Bool }
    struct Foo { x Int }
    fun Foo.eq(a &Foo, b &Foo) Bool { a.x == b.x }
    fun check<T S>(a T, b T) Bool { a.eq(b) }
    let f = Foo(1)
    _ = check(&f, &f)
}
```

```error
```

**Mut ref type satisfies shape**

```metall
{
    shape S { fun S.eq(a S, b S) Bool }
    struct Foo { x Int }
    fun Foo.eq(a &mut Foo, b &mut Foo) Bool { a.x == b.x }
    fun check<T S>(a T, b T) Bool { a.eq(b) }
    mut f = Foo(1)
    _ = check(&mut f, &mut f)
}
```

```error
```

**Mut ref to generic struct satisfies shape**

```metall
{
    shape S { fun S.eq(a S, b S) Bool }
    struct Box<T> { value T }
    fun Box.eq<U S>(a &mut Box<U>, b &mut Box<U>) Bool { a.value.eq(b.value) }
    fun Int.eq(a Int, b Int) Bool { a == b }
    fun check<T S>(a T, b T) Bool { a.eq(b) }
    mut x = Box<Int>(1)
    mut y = Box<Int>(2)
    _ = check(&mut x, &mut y)
}
```

```error
```

**Mut ref type satisfies shape expecting immutable ref**

```metall
{
    shape S { fun S.eq(a S, b S) Bool }
    struct Foo { x Int }
    fun Foo.eq(a &Foo, b &Foo) Bool { a.x == b.x }
    fun check<T S>(a T, b T) Bool { a.eq(b) }
    mut f = Foo(1)
    _ = check(&mut f, &mut f)
}
```

```error
```

## Default Type Args

**Default type arg on struct**

```metall
{
    struct Box<T = Int> { value T }
    let x = Box(42)
    let y = Box<Str>("hello")
    x.value
}
```

```types
Block: Int
  Struct: struct01
    TypeParam: ?
      SimpleType: ?
    StructField: ?
      SimpleType: ?
  Var: void
    TypeConstruction: struct02
      Ident: struct02
      Int: Int
  Var: void
    TypeConstruction: struct03
      Ident: struct03
      String: Str
  FieldAccess: Int
    Ident: struct02
---
struct01 = Box { value T }
struct02 = Box<Int> { value Int }
struct03 = Box<Str> { value Str }
```

**Default type arg with partial params**

```metall
{
    struct Pair<A, B = Int> { a A b B }
    let x = Pair<Str>("hello", 42)
    x.a
}
```

```types
Block: Str
  Struct: struct01
    TypeParam: ?
    TypeParam: ?
      SimpleType: ?
    StructField: ?
      SimpleType: ?
    StructField: ?
      SimpleType: ?
  Var: void
    TypeConstruction: struct02
      Ident: struct02
      String: Str
      Int: Int
  FieldAccess: Str
    Ident: struct02
---
struct01 = Pair { a A, b B }
struct02 = Pair<Str, Int> { a Str, b Int }
```

**Default type arg on function with constraint**

```metall
{
    shape ToStr {
        fun ToStr.to_str(self ToStr) Str
    }
    fun Str.to_str(s Str) Str { s }
    fun show<T ToStr = Str>(x T) Str { x.to_str() }
    show("hello")
}
```

```types
Block: Str
  Shape: shape01
    FunDecl: ?
      FunParam: shape01
        SimpleType: shape01
      SimpleType: Str
  Fun: fun01
    FunParam: Str
      SimpleType: Str
    SimpleType: Str
    Block: Str
      Ident: Str
  Fun: fun02
    TypeParam: T
      SimpleType: shape01
      SimpleType: Str
    FunParam: T
      SimpleType: T
    SimpleType: Str
    Block: Str
      Call: Str
        FieldAccess: fun03
          Ident: T
  Call: Str
    Ident: fun04
    String: Str
---
shape01 = ToStr {  }
fun01   = sync fun(Str) Str
fun02   = fun(T) Str
fun03   = fun(T) Str
fun04   = fun(Str) Str
```

**Default type arg does not satisfy shape constraint**

```metall
{
    shape Showable { fun Showable.show(self Showable) Str }
    struct Box<T Showable = Int> { value T }
}
```

```error
test.met:3:29: type Int does not satisfy shape Showable: missing method show
        shape Showable { fun Showable.show(self Showable) Str }
        struct Box<T Showable = Int> { value T }
                                ^^^
    }
```

**Default type arg too many args**

```metall
{ struct Box<T = Int> { value T } fun bar(f Box<Int, Str>) void {} }
```

```error
test.met:1:45: type argument count mismatch: expected 0 to 1, got 2
    { struct Box<T = Int> { value T } fun bar(f Box<Int, Str>) void {} }
                                                ^^^^^^^^^^^^^
```

**Default type arg too few args**

```metall
{ struct Pair<A, B = Int> { a A b B } fun bar(f Pair) void {} }
```

```error
test.met:1:49: type argument count mismatch: expected 1 to 2, got 0
    { struct Pair<A, B = Int> { a A b B } fun bar(f Pair) void {} }
                                                    ^^^^
```

## Type Arg Inference

**Infer struct type args from constructor**

```metall
{
    struct Pair<A, B> { a A b B }
    let x = Pair("hello", 42)
    x.a
}
```

```types
Block: Str
  Struct: struct01
    TypeParam: ?
    TypeParam: ?
    StructField: ?
      SimpleType: ?
    StructField: ?
      SimpleType: ?
  Var: void
    TypeConstruction: struct02
      Ident: struct02
      String: Str
      Int: Int
  FieldAccess: Str
    Ident: struct02
---
struct01 = Pair { a A, b B }
struct02 = Pair<Str, Int> { a Str, b Int }
```

**Infer union type arg with default**

```metall
{
    union MyResult<T, E = Bool> = T | E
    MyResult(42)
}
```

```types
Block: union01
  Union: union02
    TypeParam: ?
    TypeParam: ?
      SimpleType: ?
    SimpleType: ?
    SimpleType: ?
  TypeConstruction: union01
    Ident: union01
    Int: Int
---
union01 = MyResult<Int, Int> = Int | Int
union02 = MyResult = T | E
```

**Infer function type args from call**

```metall
{
    fun id<T>(x T) T { x }
    id(42)
}
```

```types
Block: Int
  Fun: fun01
    TypeParam: T
    FunParam: T
      SimpleType: T
    SimpleType: T
    Block: T
      Ident: T
  Call: Int
    Ident: fun02
    Int: Int
---
fun01 = fun(T) T
fun02 = fun(Int) Int
```

**Infer fun type args materializes int literals for concrete params**

```metall
{
    fun foo<T>(x T, n U8) T { x }
    foo("hi", 42)
}
```

```types
Block: Str
  Fun: fun01
    TypeParam: T
    FunParam: T
      SimpleType: T
    FunParam: U8
      SimpleType: U8
    SimpleType: T
    Block: T
      Ident: T
  Call: Str
    Ident: fun02
    String: Str
    Int: U8
---
fun01 = fun(T, U8) T
fun02 = fun(Str, U8) Str
```

**Infer nested type args**

```metall
{
    struct Box<T> { value T }
    fun unbox<T>(b Box<T>) T { b.value }
    unbox(Box(42))
}
```

```types
Block: Int
  Struct: struct01
    TypeParam: ?
    StructField: ?
      SimpleType: ?
  Fun: fun01
    TypeParam: T
    FunParam: struct02
      SimpleType: struct02
        SimpleType: T
    SimpleType: T
    Block: T
      FieldAccess: T
        Ident: struct02
  Call: Int
    Ident: fun02
    TypeConstruction: struct03
      Ident: struct03
      Int: Int
---
struct01 = Box { value T }
struct02 = Box<T> { value T }
fun01    = fun(struct02) T
struct03 = Box<Int> { value Int }
fun02    = fun(struct03) Int
```

**Infer method type args from call**

```metall
{
    struct Foo { name Str }
    fun Foo.greet<T>(f Foo, x T) T { x }
    let f = Foo("hi")
    f.greet(42)
}
```

```types
Block: Int
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Fun: fun01
    TypeParam: T
    FunParam: struct01
      SimpleType: struct01
    FunParam: T
      SimpleType: T
    SimpleType: T
    Block: T
      Ident: T
  Var: void
    TypeConstruction: struct01
      Ident: struct01
      String: Str
  Call: Int
    FieldAccess: fun02
      Ident: struct01
    Int: Int
---
struct01 = Foo { name Str }
fun01    = fun(struct01, T) T
fun02    = fun(struct01, Int) Int
```

**Infer type args from assignment target**

```metall
{
    union Foo<T> = Str | T
    mut x = Foo(true)
    x = Foo("test")
}
```

```types
Block: void
  Union: union01
    TypeParam: ?
    SimpleType: ?
    SimpleType: ?
  Var: void
    TypeConstruction: union02
      Ident: union02
      Bool: Bool
  Assign: void
    Ident: union02
    TypeConstruction: union02
      Ident: union02
      String: Str
---
union01 = Foo = Str | T
union02 = Foo<Bool> = Str | Bool
```

**Infer type args from return type**

```metall
{
    union MyResult<T> = T | Str
    fun foo() MyResult<Int> { MyResult(42) }
    foo()
}
```

```types
Block: union01
  Union: union02
    TypeParam: ?
    SimpleType: ?
    SimpleType: ?
  Fun: fun01
    SimpleType: union01
    Block: union01
      TypeConstruction: union01
        Ident: union01
        Int: Int
  Call: union01
    Ident: fun01
---
union01 = MyResult<Int> = Int | Str
union02 = MyResult = T | Str
fun01   = sync fun() union01
```

**Infer type args from return type via if**

```metall
{
    union MyResult<T> = T | Str
    fun foo(x Bool) MyResult<Int> {
        if x { MyResult(42) }
        else { MyResult("err") }
    }
    foo(true)
}
```

```types
Block: union01
  Union: union02
    TypeParam: ?
    SimpleType: ?
    SimpleType: ?
  Fun: fun01
    FunParam: Bool
      SimpleType: Bool
    SimpleType: union01
    Block: union01
      If: union01
        Ident: Bool
        Block: union01
          TypeConstruction: union01
            Ident: union01
            Int: Int
        Block: union01
          TypeConstruction: union01
            Ident: union01
            String: Str
  Call: union01
    Ident: fun01
    Bool: Bool
---
union01 = MyResult<Int> = Int | Str
union02 = MyResult = T | Str
fun01   = sync fun(Bool) union01
```

**Infer type args from return type via match**

```metall
{
    union MyResult<T> = T | Str
    fun foo(x MyResult<Int>) MyResult<Int> {
        match x {
            case Int i: MyResult(i)
            case Str s: MyResult(s)
        }
    }
    foo(MyResult<Int>(1))
}
```

```types
Block: union01
  Union: union02
    TypeParam: ?
    SimpleType: ?
    SimpleType: ?
  Fun: fun01
    FunParam: union01
      SimpleType: union01
    SimpleType: union01
    Block: union01
      Match: union01
        Ident: union01
        SimpleType: Int
        Block: union01
          TypeConstruction: union01
            Ident: union01
            Ident: Int
        SimpleType: Str
        Block: union01
          TypeConstruction: union01
            Ident: union01
            Ident: Str
  Call: union01
    Ident: fun01
    TypeConstruction: union01
      Ident: union01
      Int: Int
---
union01 = MyResult<Int> = Int | Str
union02 = MyResult = T | Str
fun01   = sync fun(union01) union01
```

## Default Parameters

**Call with default arg omitted**

```metall
{ fun foo(a Int, b Int = 3) Int { a + b } foo(1) }
```

```types
Block: Int
  Fun: fun01
    FunParam: Int
      SimpleType: Int
    FunParam: Int
      SimpleType: Int
      Int: Int
    SimpleType: Int
    Block: Int
      Binary: Int
        Ident: Int
        Ident: Int
  Call: Int
    Ident: fun01
    Int: Int
    Int: Int
---
fun01 = sync fun(Int, Int) Int
```

**Call with default arg provided**

```metall
{ fun foo(a Int, b Int = 3) Int { a + b } foo(1, 5) }
```

```types
Block: Int
  Fun: fun01
    FunParam: Int
      SimpleType: Int
    FunParam: Int
      SimpleType: Int
      Int: Int
    SimpleType: Int
    Block: Int
      Binary: Int
        Ident: Int
        Ident: Int
  Call: Int
    Ident: fun01
    Int: Int
    Int: Int
---
fun01 = sync fun(Int, Int) Int
```

**Call with multiple defaults partially provided**

```metall
{ fun foo(a Int, b Int = 2, c Int = 3) Int { a + b + c } foo(1, 10) }
```

```types
Block: Int
  Fun: fun01
    FunParam: Int
      SimpleType: Int
    FunParam: Int
      SimpleType: Int
      Int: Int
    FunParam: Int
      SimpleType: Int
      Int: Int
    SimpleType: Int
    Block: Int
      Binary: Int
        Binary: Int
          Ident: Int
          Ident: Int
        Ident: Int
  Call: Int
    Ident: fun01
    Int: Int
    Int: Int
    Int: Int
---
fun01 = sync fun(Int, Int, Int) Int
```

**All defaults omitted**

```metall
{ fun foo(a Int = 1, b Int = 2) Int { a + b } foo() }
```

```types
Block: Int
  Fun: fun01
    FunParam: Int
      SimpleType: Int
      Int: Int
    FunParam: Int
      SimpleType: Int
      Int: Int
    SimpleType: Int
    Block: Int
      Binary: Int
        Ident: Int
        Ident: Int
  Call: Int
    Ident: fun01
    Int: Int
    Int: Int
---
fun01 = sync fun(Int, Int) Int
```

**Method with default arg**

```metall
{
    struct Foo { x Int }
    fun Foo.add(f Foo, n Int = 1) Int { f.x + n }
    Foo(10).add()
}
```

```types
Block: Int
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Fun: fun01
    FunParam: struct01
      SimpleType: struct01
    FunParam: Int
      SimpleType: Int
      Int: Int
    SimpleType: Int
    Block: Int
      Binary: Int
        FieldAccess: Int
          Ident: struct01
        Ident: Int
  Call: Int
    FieldAccess: fun01
      TypeConstruction: struct01
        Ident: struct01
        Int: Int
    Int: Int
---
struct01 = Foo { x Int }
fun01    = sync fun(struct01, Int) Int
```

**Default value type mismatch**

```metall
{ fun foo(a Int = "hello") Int { 0 } }
```

```error
test.met:1:19: default value type mismatch: expected Int, got Str
    { fun foo(a Int = "hello") Int { 0 } }
                      ^^^^^^^
```

**Default on ref param**

```metall
{ fun foo(a &mut Int = 3) void {} }
```

```error
test.met:1:24: default parameters cannot be references
    { fun foo(a &mut Int = 3) void {} }
                           ^
```

**Too few args even with defaults**

```metall
{ fun foo(a Int, b Int, c Int = 3) Int { 0 } foo() }
```

```error
test.met:1:46: argument count mismatch: expected 3, got 0
    { fun foo(a Int, b Int, c Int = 3) Int { 0 } foo() }
                                                 ^^^^^
```

**Defaults do not apply to struct construction**

```metall
{ struct Foo { x Int y Int } Foo(1) }
```

```error
test.met:1:30: argument count mismatch: expected 2, got 1
    { struct Foo { x Int y Int } Foo(1) }
                                 ^^^^^^
```

**Generic struct construction wrong arg count**

```metall
{ struct Foo<T> { a Int b T } fun bar<T>() !&Foo<T> { Foo(42) } }
```

```error
test.met:1:55: argument count mismatch: expected 2, got 1
    { struct Foo<T> { a Int b T } fun bar<T>() !&Foo<T> { Foo(42) } }
                                                          ^^^^^^^
```

**Defaults do not apply to union construction**

```metall
{ union U = Int | Str U() }
```

```error
test.met:1:23: union constructor takes exactly 1 argument, got 0
    { union U = Int | Str U() }
                          ^^^
```

**Generic method call with default arg and explicit type args**

```metall
{
    shape HasFmt {
        fun HasFmt.fmt(f HasFmt) Str
    }
    struct Foo { x Int }
    fun Foo.fmt(f Foo) Str { "foo" }
    fun show<T HasFmt>(t T, prefix Str = "") Str { t.fmt() }
    show<Foo>(Foo(1))
}
```

```types
Block: Str
  Shape: shape01
    FunDecl: ?
      FunParam: shape01
        SimpleType: shape01
      SimpleType: Str
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Fun: fun01
    FunParam: struct01
      SimpleType: struct01
    SimpleType: Str
    Block: Str
      String: Str
  Fun: fun02
    TypeParam: T
      SimpleType: shape01
    FunParam: T
      SimpleType: T
    FunParam: Str
      SimpleType: Str
      String: Str
    SimpleType: Str
    Block: Str
      Call: Str
        FieldAccess: fun03
          Ident: T
  Call: Str
    Ident: fun04
    TypeConstruction: struct01
      Ident: struct01
      Int: Int
    String: Str
---
shape01  = HasFmt {  }
struct01 = Foo { x Int }
fun01    = sync fun(struct01) Str
fun02    = fun(T, Str) Str
fun03    = fun(T) Str
fun04    = fun(struct01, Str) Str
```

## Named Arguments

**Named call args are reordered**

```metall
{ fun foo(a Int, b Int) Int { a - b } foo(b = 3, a = 10) }
```

```types
Block: Int
  Fun: fun01
    FunParam: Int
      SimpleType: Int
    FunParam: Int
      SimpleType: Int
    SimpleType: Int
    Block: Int
      Binary: Int
        Ident: Int
        Ident: Int
  Call: Int
    Ident: fun01
    Int: Int
    Int: Int
---
fun01 = sync fun(Int, Int) Int
```

**Named args fill the right defaults**

```metall
{ fun foo(a Int, b Int = 2, c Int = 3) Int { a + b + c } foo(a = 1, c = 9) }
```

```types
Block: Int
  Fun: fun01
    FunParam: Int
      SimpleType: Int
    FunParam: Int
      SimpleType: Int
      Int: Int
    FunParam: Int
      SimpleType: Int
      Int: Int
    SimpleType: Int
    Block: Int
      Binary: Int
        Binary: Int
          Ident: Int
          Ident: Int
        Ident: Int
  Call: Int
    Ident: fun01
    Int: Int
    Int: Int
    Int: Int
---
fun01 = sync fun(Int, Int, Int) Int
```

**Named struct construction is reordered**

```metall
{ struct Foo { x Int y Int } Foo(y = 2, x = 1) }
```

```types
Block: struct01
  Struct: struct01
    StructField: ?
      SimpleType: ?
    StructField: ?
      SimpleType: ?
  TypeConstruction: struct01
    Ident: struct01
    Int: Int
    Int: Int
---
struct01 = Foo { x Int, y Int }
```

**Unknown parameter name is rejected**

```metall
{ fun foo(a Int, b Int) Int { a } foo(z = 1, a = 2) }
```

```error
test.met:1:39: unknown parameter: z
    { fun foo(a Int, b Int) Int { a } foo(z = 1, a = 2) }
                                          ^
```

**Parameter specified twice is rejected**

```metall
{ fun foo(a Int, b Int) Int { a } foo(a = 1, a = 2) }
```

```error
test.met:1:46: parameter a specified more than once
    { fun foo(a Int, b Int) Int { a } foo(a = 1, a = 2) }
                                                 ^
```

**Positional and named for the same parameter is rejected**

```metall
{ fun foo(a Int, b Int) Int { a } foo(1, a = 2) }
```

```error
test.met:1:42: parameter a specified more than once
    { fun foo(a Int, b Int) Int { a } foo(1, a = 2) }
                                             ^
```

**Missing required parameter is rejected**

```metall
{ fun foo(a Int, b Int) Int { a } foo(b = 2) }
```

```error
test.met:1:35: missing argument for parameter: a
    { fun foo(a Int, b Int) Int { a } foo(b = 2) }
                                      ^^^^^^^^^^
```

**Named struct construction missing a field is rejected**

```metall
{ struct Foo { x Int y Int } Foo(x = 1) }
```

```error
test.met:1:30: missing argument for field: y
    { struct Foo { x Int y Int } Foo(x = 1) }
                                 ^^^^^^^^^^
```

**Named arguments on union construction are rejected**

```metall
{ union U = Int | Str U(v = 1) }
```

```error
test.met:1:23: named arguments are only supported when constructing a struct
    { union U = Int | Str U(v = 1) }
                          ^^^^^^^^
```

**Named arguments on indirect calls are rejected**

```metall
{ let f = fun(a Int) Int { a } f(a = 1) }
```

```error
test.met:1:32: named arguments are not supported for indirect calls
    { let f = fun(a Int) Int { a } f(a = 1) }
                                   ^^^^^^^^
```

## Errors

**Undefined symbol**

```metall
let y = x
```

```error
test.met:1:9: symbol not defined: x
    let y = x
            ^
```

**Method on undefined receiver type**

```metall
{
    fun Foo.bar() void {}
}
```

```error
test.met:2:9: method receiver type not found: Foo
    {
        fun Foo.bar() void {}
            ^^^^^^^
    }
```

**Undefined symbol forward ref**

```metall
{ x let x = 123 }
```

```error
test.met:1:3: symbol not defined: x
    { x let x = 123 }
      ^
```

**Duplicate var**

```metall
{ let x = 123 let x = 321 }
```

```error
test.met:1:19: symbol already defined: x
    { let x = 123 let x = 321 }
                      ^
```

**Duplicate fun**

```metall
{ fun foo() void {} fun foo() void {} }
```

```error
test.met:1:25: symbol already defined: foo
    { fun foo() void {} fun foo() void {} }
                            ^^^
```

**Redefining a prelude symbol is reserved**

```metall module
struct Int { x U8 }
```

```error
test.met:1:8: reserved symbol: Int (defined in prelude)
    struct Int { x U8 }
           ^^^
```

**Duplicate generic method called (module)**

```metall module
struct Foo<T> { one T }
fun Foo.bar(f &Foo) T { f.one }
fun Foo.bar(f &Foo) T { f.one }
fun main() void { let f = Foo<Int>(42) let r = &f _ = r.bar() }
```

```error
test.met:3:5: symbol already defined: test.Foo.bar
    fun Foo.bar(f &Foo) T { f.one }
    fun Foo.bar(f &Foo) T { f.one }
        ^^^^^^^
    fun main() void { let f = Foo<Int>(42) let r = &f _ = r.bar() }
```

**Fun return mismatch**

```metall
fun foo() Str { 123 }
```

```error
test.met:1:17: return type mismatch: expected Str, got Int
    fun foo() Str { 123 }
                    ^^^
```

**Return mismatch**

```metall
fun foo() Str { return 123 }
```

```error
test.met:1:24: return type mismatch: expected Str, got Int
    fun foo() Str { return 123 }
                           ^^^
```

**Unreachable code after return**

```metall
fun foo() Int { return 123 456 }
```

```error
test.met:1:28: unreachable code
    fun foo() Int { return 123 456 }
                               ^^^
```

**Unreachable code after break**

```metall
fun foo() Int { for { break 456 } }
```

```error
test.met:1:29: unreachable code
    fun foo() Int { for { break 456 } }
                                ^^^
```

**Unreachable code after continue**

```metall
fun foo() Int { for { continue 456 } }
```

```error
test.met:1:32: unreachable code
    fun foo() Int { for { continue 456 } }
                                   ^^^
```

**Assign type mismatch**

```metall
{ mut x = 123 x = "hello" }
```

```error
test.met:1:19: type mismatch: expected Int, got Str
    { mut x = 123 x = "hello" }
                      ^^^^^^^
```

**Var type annotation mismatch**

```metall
let x Str = 123
```

```error
test.met:1:13: type mismatch: expected Str, got Int
    let x Str = 123
                ^^^
```

**Var type annotation &ref not assignable to &mut**

```metall
{ let x = 1 let y &mut Int = &x }
```

```error
test.met:1:30: type mismatch: expected &mut Int, got &Int
    { let x = 1 let y &mut Int = &x }
                                 ^^
```

**Assign to let binding**

```metall
{ let x = 123 x = 321 }
```

```error
test.met:1:15: cannot assign to immutable variable: x
    { let x = 123 x = 321 }
                  ^
```

**Compound assign to non-integer**

```metall
{ mut s = "hi" s += "x" }
```

```error
test.met:1:16: compound assignment '+=' expects an integer or float, got Str
    { mut s = "hi" s += "x" }
                   ^
```

**Compound assign rhs type mismatch**

```metall
{ mut x = 1 x += "no" }
```

```error
test.met:1:18: type mismatch: expected Int, got Str
    { mut x = 1 x += "no" }
                     ^^^^
```

**Compound assign to immutable**

```metall
{ let x = 1 x += 2 }
```

```error
test.met:1:13: cannot assign to immutable variable: x
    { let x = 1 x += 2 }
                ^
```

**Call wrong arg count**

```metall
{ fun foo(a Int) Int { 123 } foo(1, 2, "hello") }
```

```error
test.met:1:30: argument count mismatch: expected 1, got 3
    { fun foo(a Int) Int { 123 } foo(1, 2, "hello") }
                                 ^^^^^^^^^^^^^^^^^^
```

**Call wrong arg type**

```metall
{ fun foo(a Int, b Int) Int { 123 } foo("hello", 2) }
```

```error
test.met:1:41: type mismatch at argument 1: expected Int, got Str
    { fun foo(a Int, b Int) Int { 123 } foo("hello", 2) }
                                            ^^^^^^^
```

**Call non-function**

```metall
{ 123() }
```

```error
test.met:1:3: cannot call non-function: Int
    { 123() }
      ^^^
```

**Main must return void or !void (module)**

```metall module
fun main() Int { 123 }
```

```error
test.met:1:12: main function must return void or !void
    fun main() Int { 123 }
               ^^^
```

**Main cannot return !Int (module)**

```metall module
fun main() !Int { 123 }
```

```error
test.met:1:12: main function must return void or !void
    fun main() !Int { 123 }
               ^^^^
```

**Main must not have params (module)**

```metall module
fun main(a Int, b Str) void { }
```

```error
test.met:1:10: main function cannot take arguments
    fun main(a Int, b Str) void { }
             ^^^^^^^^^^^^
```

**Field access on non-struct**

```metall
123.one
```

```error
test.met:1:5: unknown field: Int.one
    123.one
        ^^^
```

**Field access on function type**

```metall
{
    struct Foo { f fun() void }
    let x = Foo(fun() void {})
    x.f.something
}
```

```error
test.met:4:5: cannot access field on non-struct type: fun() void
        let x = Foo(fun() void {})
        x.f.something
        ^^^
    }
```

**Field access unknown field**

```metall
{ struct Foo{one Str} let x = Foo("hello") x.three }
```

```error
test.met:1:46: unknown field: Foo.three
    { struct Foo{one Str} let x = Foo("hello") x.three }
                                                 ^^^^^
```

**Type error in fun body does not poison fun type**

```metall
{ fun foo(s Str) Int { s.nope } foo("hello") }
```

```error
test.met:1:26: unknown field: Str.nope
    { fun foo(s Str) Int { s.nope } foo("hello") }
                             ^^^^
```

**Str cannot be constructed directly**

```metall
{ let @a = Arena() let d = @a.slice<U8>(1, 0) Str(d) }
```

```error
test.met:1:47: Str cannot be constructed directly; use Str.from_utf8_lossy() instead
    { let @a = Arena() let d = @a.slice<U8>(1, 0) Str(d) }
                                                  ^^^^^^
```

**If condition must be bool**

```metall
{ if 123 { } }
```

```error
test.met:1:6: if condition must evaluate to a boolean value, got Int
    { if 123 { } }
         ^^^
```

**If branches must match**

```metall
{ if true { 123 } else { "hello" } }
```

```error
test.met:1:24: if branch type mismatch: expected Int, got Str
    { if true { 123 } else { "hello" } }
                           ^^^^^^^^^^^
```

**Deref non-reference**

```metall
{ let x = 5 x.* }
```

```error
test.met:1:13: dereference: expected reference, got Int
    { let x = 5 x.* }
                ^
```

**Deref assign through immutable ref**

```metall
{ let x = 5 let y = &x y.* = 321 }
```

```error
test.met:1:24: cannot assign through dereference: expected mutable reference, got &Int
    { let x = 5 let y = &x y.* = 321 }
                           ^^^
```

**Pass value to &ref param**

```metall
{ fun foo(a &Int) void {} let x = 123 foo(x) }
```

```error
test.met:1:43: type mismatch at argument 1: expected &Int, got Int
    { fun foo(a &Int) void {} let x = 123 foo(x) }
                                              ^
```

**Deref assign through immutable ref param**

```metall
{ fun foo(a &Int) void { a.* = 123 }}
```

```error
test.met:1:26: cannot assign through dereference: expected mutable reference, got &Int
    { fun foo(a &Int) void { a.* = 123 }}
                             ^^^
```

**&mut of let binding**

```metall
{ let x = 123 let y = &mut x }
```

```error
test.met:1:23: cannot take mutable reference to immutable value
    { let x = 123 let y = &mut x }
                          ^^^^^^
```

**Field write on let binding**

```metall
{ struct Foo{one Str} let x = Foo("hello") x.one = "bye" }
```

```error
test.met:1:44: cannot assign to field of immutable value
    { struct Foo{one Str} let x = Foo("hello") x.one = "bye" }
                                               ^^^^^
```

**Nested field write on let binding**

```metall
{ struct Foo{one Int} struct Bar{one Foo} let x = Bar(Foo(1)) x.one.one = 2 }
```

```error
test.met:1:63: cannot assign to field of immutable value
    { struct Foo{one Int} struct Bar{one Foo} let x = Bar(Foo(1)) x.one.one = 2 }
                                                                  ^^^^^^^^^
```

**Field write through immutable ref**

```metall
{ struct Foo{one Str} let x = Foo("hello") let y = &x y.one = "X" }
```

```error
test.met:1:55: cannot assign to field of immutable value
    { struct Foo{one Str} let x = Foo("hello") let y = &x y.one = "X" }
                                                          ^^^^^
```

**Field write through immutable ref param**

```metall
{ struct Foo{one Str} fun foo(a &Foo) void { a.one = "X" } }
```

```error
test.met:1:46: cannot assign to field of immutable value
    { struct Foo{one Str} fun foo(a &Foo) void { a.one = "X" } }
                                                 ^^^^^
```

**Pass &ref where field type is &mut**

```metall
{ struct Foo { one &mut Int } let x = 123 let y = Foo(&x) }
```

```error
test.met:1:55: type mismatch at argument 1: expected &mut Int, got &Int
    { struct Foo { one &mut Int } let x = 123 let y = Foo(&x) }
                                                          ^^
```

**Deref assign through &ref field**

```metall
{ struct Foo { one &Int } let x = 123 let y = Foo(&x) y.one.* = 42 }
```

```error
test.met:1:55: cannot assign through dereference: expected mutable reference, got &Int
    { struct Foo { one &Int } let x = 123 let y = Foo(&x) y.one.* = 42 }
                                                          ^^^^^^^
```

**Assign &ref to field of type &mut**

```metall
{ struct Foo { one &mut Int } mut x = 1 mut y = 2 mut z = Foo(&mut x) z.one = &y }
```

```error
test.met:1:79: type mismatch: expected &mut Int, got &Int
    { struct Foo { one &mut Int } mut x = 1 mut y = 2 mut z = Foo(&mut x) z.one = &y }
                                                                                  ^^
```

**Reassign &mut param**

```metall
{ fun foo(a &mut Int) void { mut x = 1 a = &x } }
```

```error
test.met:1:40: cannot assign to immutable variable: a
    { fun foo(a &mut Int) void { mut x = 1 a = &x } }
                                           ^
```

**Non-existing allocator**

```metall
{ struct Foo{one Str} let x = @myalloc.new<Foo>(Foo("hello")) }
```

```error
test.met:1:31: symbol not defined: @myalloc
    { struct Foo{one Str} let x = @myalloc.new<Foo>(Foo("hello")) }
                                  ^^^^^^^^
```

**Index on non-array**

```metall
{ let x = 123 x[0] }
```

```error
test.met:1:15: not an array or slice: Int
    { let x = 123 x[0] }
                  ^
```

**Index with non-int**

```metall
{ let x = [1, 2, 3] x["hello"] }
```

```error
test.met:1:23: index type mismatch: expected Int, got Str
    { let x = [1, 2, 3] x["hello"] }
                          ^^^^^^^
```

**Subslice on non-array**

```metall
{ let x = 123 x[0..1] }
```

```error
test.met:1:15: not an array or slice: Int
    { let x = 123 x[0..1] }
                  ^
```

**Subslice with non-int lo**

```metall
{ let x = [1, 2, 3] x["a"..2] }
```

```error
test.met:1:23: range bound must be Int, got Str
    { let x = [1, 2, 3] x["a"..2] }
                          ^^^
```

**Subslice with non-int hi**

```metall
{ let x = [1, 2, 3] x[0.."b"] }
```

```error
test.met:1:26: range bound must be Int, got Str
    { let x = [1, 2, 3] x[0.."b"] }
                             ^^^
```

**Add with non-int**

```metall
1 + "hello"
```

```error
test.met:1:5: type mismatch: expected type of LHS: Int, got Str
    1 + "hello"
        ^^^^^^^
```

**== with invalid type**

```metall
"hello" == "world"
```

```error
test.met:1:1: type mismatch: binary operation '==' expects an integer, float, or Bool, got Str
    "hello" == "world"
    ^^^^^^^
```

**and with invalid type**

```metall
true and 123
```

```error
test.met:1:10: type mismatch: expected type of LHS: Bool, got Int
    true and 123
             ^^^
```

**not on invalid type**

```metall
not 123
```

```error
test.met:1:5: type mismatch: expected Bool, got Int
    not 123
        ^^^
```

**unary minus on an unsigned integer**

```metall
-U8(5)
```

```error
test.met:1:2: type mismatch: unary minus expects a signed integer or float, got U8
    -U8(5)
     ^^^^^
```

**Non-boolean condition in for loop**

```metall
for 123 {}
```

```error
test.met:1:5: type mismatch: expected Bool, got Int
    for 123 {}
        ^^^
```

**For in range bound not Int**

```metall
{ for x in "a".."z" {} }
```

```error
test.met:1:12: range bound must be Int, got Str
    { for x in "a".."z" {} }
               ^^^
```

**For in binding is immutable**

```metall
{ for x in 0..10 { x = 5 } }
```

```error
test.met:1:20: cannot assign to immutable variable: x
    { for x in 0..10 { x = 5 } }
                       ^
```

**Break outside loop**

```metall
{ break }
```

```error
test.met:1:3: break statement outside of loop
    { break }
      ^^^^^
```

**Continue outside loop**

```metall
{ continue }
```

```error
test.met:1:3: continue statement outside of loop
    { continue }
      ^^^^^^^^
```

**Unknown field on slice**

```metall
{ let @a = Arena() let x = unsafe @a.slice_uninit<Int>(3) x.foo }
```

```error
test.met:1:61: unknown field: []mut Int.foo
    { let @a = Arena() let x = unsafe @a.slice_uninit<Int>(3) x.foo }
                                                                ^^^
```

**Unknown field on array**

```metall
{ let x = [1, 2, 3] x.foo }
```

```error
test.met:1:23: unknown field on array: foo
    { let x = [1, 2, 3] x.foo }
                          ^^^
```

**Make slice non-int length**

```metall
{ let @a = Arena() unsafe @a.slice_uninit<Int>("hello") }
```

```error
test.met:1:48: type mismatch at argument 1: expected Int, got Str
    { let @a = Arena() unsafe @a.slice_uninit<Int>("hello") }
                                                   ^^^^^^^
```

**Make wrong default type**

```metall
{ let @a = Arena() @a.slice<Int>(5, "hello") }
```

```error
test.met:1:37: type mismatch at argument 2: expected Int, got Str
    { let @a = Arena() @a.slice<Int>(5, "hello") }
                                        ^^^^^^^
```

**Make wrong default type 2**

```metall
{ let @a = Arena() @a.slice<Int>(3, "hello") }
```

```error
test.met:1:37: type mismatch at argument 2: expected Int, got Str
    { let @a = Arena() @a.slice<Int>(3, "hello") }
                                        ^^^^^^^
```

**Empty slice without context**

```metall
[]
```

```error
test.met:1:1: cannot infer type of empty slice []
    []
    ^^
```

**Empty slice in let binding**

```metall
let x = []
```

```error
test.met:1:9: cannot infer type of empty slice []
    let x = []
            ^^
```

**Mut binding of array literal subslice is mutable**

```metall
{
    mut sut = [3, 2, 4, 1][..]
    sut[0] = 99
}
```

```error
```

**Mut binding of array construction subslice is mutable**

```metall
{
    mut sut = [4 of Int(0)][..]
    sut[0] = 99
}
```

```error
```

**Subslice of function return is not mutable**

```metall
{
    fun arr() [4]Int { [1,2,3,4] }
    mut sut = arr()[..]
    sut[0] = 99
}
```

```error
test.met:4:5: cannot assign to element of immutable array or slice
        mut sut = arr()[..]
        sut[0] = 99
        ^^^^^^
    }
```

**Rebinding immutable slice param to mut does not make it mutable**

```metall
{
    fun foo(s []Int) void {
        mut s2 = s
        s2[0] = 99
    }
}
```

```error
test.met:4:9: cannot assign to element of immutable array or slice
            mut s2 = s
            s2[0] = 99
            ^^^^^
        }
```

**Mut binding of bytes literal can be rebound**

```metall
{
    mut a = b"abc"
    a = b"xyz"
}
```

```error
```

**Mut binding of bytes literal is not mutable**

```metall
{
    mut a = b"abc"
    a[0] = U8(99)
}
```

```error
test.met:3:5: cannot assign to element of immutable array or slice
        mut a = b"abc"
        a[0] = U8(99)
        ^^^^
    }
```

**Cannot return allocator from fun**

```metall
fun foo() Arena { }
```

```error
test.met:1:11: cannot return an allocator from a function
    fun foo() Arena { }
              ^^^^^
```

**U8 out of range positive**

```metall
U8(256)
```

```error
test.met:1:4: value 256 out of range for U8 (0..255)
    U8(256)
       ^^^
```

**Bool is not a type constructor**

```metall
Bool(true)
```

```error
test.met:1:1: not a struct or union: Bool
    Bool(true)
    ^^^^
```

**U8 + Int type mismatch**

```metall
{ let x = 123 U8(1) + x }
```

```error
test.met:1:23: type mismatch: expected type of LHS: U8, got Int
    { let x = 123 U8(1) + x }
                          ^
```

**Method call wrong arg count**

```metall
{ struct Foo { one Int } fun Foo.get(f Foo) Int { f.one } let x = Foo(42) x.get(1, 2) }
```

```error
test.met:1:75: argument count mismatch: expected 0, got 2
    { struct Foo { one Int } fun Foo.get(f Foo) Int { f.one } let x = Foo(42) x.get(1, 2) }
                                                                              ^^^^^^^^^^^
```

**Generic fun call wrong arg count**

```metall
{ fun foo<T>(a T, b T) T { a } foo(1) }
```

```error
test.met:1:32: argument count mismatch: expected 2, got 1
    { fun foo<T>(a T, b T) T { a } foo(1) }
                                   ^^^^^^
```

**Generic method call wrong arg count**

```metall
{ struct Foo<T> { one T } fun Foo.add(f Foo, n T) T { n } let x = Foo(42) x.add(1, 2) }
```

```error
test.met:1:75: argument count mismatch: expected 1, got 2
    { struct Foo<T> { one T } fun Foo.add(f Foo, n T) T { n } let x = Foo(42) x.add(1, 2) }
                                                                              ^^^^^^^^^^^
```

**Method call wrong arg type**

```metall
{ struct Foo { one Int } fun Foo.add(f Foo, n Int) Int { f.one + n } let x = Foo(42) x.add("hello") }
```

```error
test.met:1:92: type mismatch at argument 1: expected Int, got Str
    { struct Foo { one Int } fun Foo.add(f Foo, n Int) Int { f.one + n } let x = Foo(42) x.add("hello") }
                                                                                               ^^^^^^^
```

**Method on unknown field**

```metall
{ struct Foo { one Int } let x = Foo(42) x.nope() }
```

```error
test.met:1:44: unknown field: Foo.nope
    { struct Foo { one Int } let x = Foo(42) x.nope() }
                                               ^^^^
```

**Direct qualified call undefined**

```metall
{ struct Foo { one Int } Foo.nope(Foo(1)) }
```

```error
test.met:1:26: symbol not defined: Foo.nope
    { struct Foo { one Int } Foo.nope(Foo(1)) }
                             ^^^^^^^^
```

**Method call receiver ref mismatch**

```metall
{
    struct Foo { one Int }
    fun Foo.get(f Foo) Int { f.one }
    let x = Foo(42)
    let y = &x
    y.get()
}
```

```error
test.met:6:5: type mismatch at receiver: expected Foo, got &Foo
        let y = &x
        y.get()
        ^
    }
```

**Method receiver auto-ref of value**

```metall
{
    struct Foo { one Int }
    fun Foo.bump(f &mut Foo) void { f.one += 1 }
    fun Foo.read(f &Foo) Int { f.one }
    mut x = Foo(1)
    x.bump()
    x.read()
}
```

```error
```

**Method receiver auto-ref of immutable value rejected**

```metall
{
    struct Foo { one Int }
    fun Foo.bump(f &mut Foo) void { f.one += 1 }
    let x = Foo(1)
    x.bump()
}
```

```error
test.met:5:5: cannot call a method requiring a mutable receiver on an immutable value
        let x = Foo(1)
        x.bump()
        ^
    }
```

**Method receiver auto-ref of a `&mut` must match the place type exactly**

```metall
{
    fun Slice.overwrite(dst &mut []Int, src []Int) void { dst.* = src }
    mut a = [1, 2, 3]
    mut hot = a[..]
    let frozen = [4, 5, 6][..]
    hot.overwrite(frozen)
}
```

```error
test.met:6:5: type mismatch at receiver: expected &mut []Int, got []mut Int
        let frozen = [4, 5, 6][..]
        hot.overwrite(frozen)
        ^^^
    }
```

**Generic struct type arg count too few**

```metall
{ struct Foo<T, U> { one T two U } let x = Foo<Int>(42, 42) }
```

```error
test.met:1:44: type argument count mismatch: expected 2, got 1
    { struct Foo<T, U> { one T two U } let x = Foo<Int>(42, 42) }
                                               ^^^^^^^^
```

**Generic struct type arg count too many**

```metall
{ struct Foo<T> { value T } let x = Foo<Int, Str>(42) }
```

```error
test.met:1:37: type argument count mismatch: expected 1, got 2
    { struct Foo<T> { value T } let x = Foo<Int, Str>(42) }
                                        ^^^^^^^^^^^^^
```

**Generic struct duplicate type param**

```metall
{ struct Foo<T, T> { one T two T } }
```

```error
test.met:1:17: duplicate type parameter: T
    { struct Foo<T, T> { one T two T } }
                    ^
```

**Type args on non-generic type**

```metall
{ fun foo(x Int<Str>) void {} }
```

```error
test.met:1:13: type arguments on non-generic type: Int
    { fun foo(x Int<Str>) void {} }
                ^^^^^^^^
```

**Generic struct missing type args**

```metall
{ struct Foo<T> { value T } fun bar(f Foo) void {} }
```

```error
test.met:1:39: type argument count mismatch: expected 1, got 0
    { struct Foo<T> { value T } fun bar(f Foo) void {} }
                                          ^^^
```

**Generic fun type arg count too few**

```metall
{ fun foo<A, B>(a A, b B) A { a } foo<Int>(1, 2) }
```

```error
test.met:1:35: type argument count mismatch: expected 2, got 1
    { fun foo<A, B>(a A, b B) A { a } foo<Int>(1, 2) }
                                      ^^^^^^^^
```

**Generic fun type arg count too many**

```metall
{ fun foo<T>(x T) T { x } foo<Int, Str>(1) }
```

```error
test.met:1:27: type argument count mismatch: expected 1, got 2
    { fun foo<T>(x T) T { x } foo<Int, Str>(1) }
                              ^^^^^^^^^^^^^
```

**Generic fun duplicate type param**

```metall
{ fun foo<T, T>(x T) T { x } }
```

```error
test.met:1:14: duplicate type parameter: T
    { fun foo<T, T>(x T) T { x } }
                 ^
```

**Method on generic struct too few type args**

```metall
{ struct Foo<T> { one T } fun Foo.bar<U, V>(f Foo, a U) U { a } let x = Foo<Int>(1) x.bar<Str>("hi") }
```

```error
test.met:1:87: type argument count mismatch: expected 3, got 2
    { struct Foo<T> { one T } fun Foo.bar<U, V>(f Foo, a U) U { a } let x = Foo<Int>(1) x.bar<Str>("hi") }
                                                                                          ^^^
```

**Method on generic struct too many type args**

```metall
{ struct Foo<T> { one T } fun Foo.bar(f Foo) T { f.one } let x = Foo<Int>(1) x.bar<Int, Str>() }
```

```error
test.met:1:80: type argument count mismatch: expected 1, got 3
    { struct Foo<T> { one T } fun Foo.bar(f Foo) T { f.one } let x = Foo<Int>(1) x.bar<Int, Str>() }
                                                                                   ^^^
```

**Method on generic struct wrong first param type**

```metall
{ struct Foo<T> { one T } fun Foo.bar(f Int) Int { f } let x = Foo<Int>(1) x.bar() }
```

```error
test.met:1:76: type mismatch at receiver: expected Int, got Foo<Int>
    { struct Foo<T> { one T } fun Foo.bar(f Int) Int { f } let x = Foo<Int>(1) x.bar() }
                                                                               ^
```

**Method on generic struct type param must be in first position**

```metall
{ struct Foo<T> { one T } fun Foo.bar<U, V>(f Foo<V>, a U) U { a } let x = Foo<Int>(1) x.bar<Str>("hi") }
```

```error
test.met:1:88: type mismatch at receiver: expected Foo<Str>, got Foo<Int>
    { struct Foo<T> { one T } fun Foo.bar<U, V>(f Foo<V>, a U) U { a } let x = Foo<Int>(1) x.bar<Str>("hi") }
                                                                                           ^
```

**Method cannot have same name as struct field**

```metall
{ struct Foo { value Int } fun Foo.value(f Foo) Int { f.value } }
```

```error
test.met:1:32: method name conflicts with field: Foo.value
    { struct Foo { value Int } fun Foo.value(f Foo) Int { f.value } }
                                   ^^^^^^^^^
```

**Generic method cannot have same name as struct field**

```metall
{ struct Box<T> { value T } fun Box.value(b Box) T { b.value } }
```

```error
test.met:1:33: method name conflicts with field: Box.value
    { struct Box<T> { value T } fun Box.value(b Box) T { b.value } }
                                    ^^^^^^^^^
```

**Duplicate union name**

```metall
{ union Foo = Str | Int union Foo = Bool | Str }
```

```error
test.met:1:31: symbol already defined: Foo
    { union Foo = Str | Int union Foo = Bool | Str }
                                  ^^^
```

**Generic union wrong type arg count**

```metall
{ union Maybe<T> = T | Bool fun foo(a Maybe<Int, Str>) void {} }
```

```error
test.met:1:39: type argument count mismatch: expected 1, got 2
    { union Maybe<T> = T | Bool fun foo(a Maybe<Int, Str>) void {} }
                                          ^^^^^^^^^^^^^^^
```

**Generic union variant with unbound type param**

```metall
{ struct Box<T> { value T } union Foo<T> = Str | Box<V> }
```

```error
test.met:1:54: symbol not defined: V
    { struct Box<T> { value T } union Foo<T> = Str | Box<V> }
                                                         ^
```

**Generic union missing type args**

```metall
{ union Maybe<T> = T | Bool fun foo(a Maybe) void {} }
```

```error
test.met:1:39: type argument count mismatch: expected 1, got 0
    { union Maybe<T> = T | Bool fun foo(a Maybe) void {} }
                                          ^^^^^
```

**Union constructor wrong arg count**

```metall
{ union Foo = Str | Int Foo(1, 2) }
```

```error
test.met:1:25: union constructor takes exactly 1 argument, got 2
    { union Foo = Str | Int Foo(1, 2) }
                            ^^^^^^^^^
```

**Union constructor no args**

```metall
{ union Foo = Str | Int Foo() }
```

```error
test.met:1:25: union constructor takes exactly 1 argument, got 0
    { union Foo = Str | Int Foo() }
                            ^^^^^
```

**Union constructor wrong type**

```metall
{ union Foo = Str | Int Foo(true) }
```

```error
test.met:1:29: type Bool is not a variant of Foo
    { union Foo = Str | Int Foo(true) }
                                ^^^^
```

**Shape not satisfied missing method**

```metall
{ shape S { fun S.foo(s S) Int } struct Foo { } fun bar<T S>(t T) Int { t.foo() } bar<Foo>(Foo()) }
```

```error
test.met:1:83: type Foo does not satisfy shape S: missing method foo
    { shape S { fun S.foo(s S) Int } struct Foo { } fun bar<T S>(t T) Int { t.foo() } bar<Foo>(Foo()) }
                                                                                      ^^^^^^^^
```

**Shape not satisfied wrong method return type**

```metall
{ shape S { fun S.foo(s S) Int } struct Foo { } fun Foo.foo(f Foo) Str { "x" } fun bar<T S>(t T) Int { t.foo() } bar<Foo>(Foo()) }
```

```error
test.met:1:114: type Foo does not satisfy shape S: method foo has signature fun(Foo) Str, expected fun(S) Int
    { shape S { fun S.foo(s S) Int } struct Foo { } fun Foo.foo(f Foo) Str { "x" } fun bar<T S>(t T) Int { t.foo() } bar<Foo>(Foo()) }
                                                                                                                     ^^^^^^^^
```

**Shape not satisfied wrong method param type**

```metall
{ shape S { fun S.foo(s S, n Int) Int } struct Foo { } fun Foo.foo(f Foo, n Str) Int { 1 } fun bar<T S>(t T) Int { t.foo(1) } bar<Foo>(Foo()) }
```

```error
test.met:1:127: type Foo does not satisfy shape S: method foo has signature fun(Foo, Str) Int, expected fun(S, Int) Int
    { shape S { fun S.foo(s S, n Int) Int } struct Foo { } fun Foo.foo(f Foo, n Str) Int { 1 } fun bar<T S>(t T) Int { t.foo(1) } bar<Foo>(Foo()) }
                                                                                                                                  ^^^^^^^^
```

**Shape not satisfied wrong method param count**

```metall
{ shape S { fun S.foo(s S) Int } struct Foo { } fun Foo.foo(f Foo, n Int) Int { n } fun bar<T S>(t T) Int { t.foo() } bar<Foo>(Foo()) }
```

```error
test.met:1:119: type Foo does not satisfy shape S: method foo has signature fun(Foo, Int) Int, expected fun(S) Int
    { shape S { fun S.foo(s S) Int } struct Foo { } fun Foo.foo(f Foo, n Int) Int { n } fun bar<T S>(t T) Int { t.foo() } bar<Foo>(Foo()) }
                                                                                                                          ^^^^^^^^
```

**Type-param argument with incompatible shape constraint**

```metall module
shape A { fun A.foo(a A) Int }
shape B { fun B.bar(b B) Int }
fun call_a<T A>(t T) Int { t.foo() }
fun caller<U B>(u U) Int { call_a(u) }
```

```error
test.met:4:28: type parameter U with constraint test.B does not satisfy shape test.A
    fun call_a<T A>(t T) Int { t.foo() }
    fun caller<U B>(u U) Int { call_a(u) }
                               ^^^^^^^^^
```

**Type-param argument shape constraint with private method**

```metall module
shape PublicFoo { pub fun PublicFoo.foo(x PublicFoo) Int }
shape PrivateFoo { fun PrivateFoo.foo(x PrivateFoo) Int }
fun call_foo<T PublicFoo>(t T) Int { t.foo() }
fun caller<U PrivateFoo>(u U) Int { call_foo(u) }
```

```error
test.met:4:37: type mismatch: expected PublicFoo, got PrivateFoo
    fun call_foo<T PublicFoo>(t T) Int { t.foo() }
    fun caller<U PrivateFoo>(u U) Int { call_foo(u) }
                                        ^^^^^^^^^^^
```

**Method parameter cannot shadow constrained owner param (module)**

```metall module
shape X { fun X.to_str(x X) Str }
struct Value<T X> { value T }
fun Value.to_str<T>(v Value<T>) Str { v.value.to_str() }
```

```error
test.met:3:18: type parameter T shadows owner type parameter
    struct Value<T X> { value T }
    fun Value.to_str<T>(v Value<T>) Str { v.value.to_str() }
                     ^
```

**Shape duplicate method**

```metall
{ shape S { fun S.foo(s S) Int fun S.foo(s S) Str } }
```

```error
test.met:1:36: symbol already defined: S.foo
    { shape S { fun S.foo(s S) Int fun S.foo(s S) Str } }
                                       ^^^^^
```

**Cannot call method on function type**

```metall
{
    fun foo() void {}
    foo.bar()
}
```

```error
test.met:3:5: cannot access field on non-struct type: sync fun() void
        fun foo() void {}
        foo.bar()
        ^^^
    }
```

**Cannot call method on void**

```metall
{
    fun foo() void {}
    foo().bar()
}
```

```error
test.met:3:5: cannot access field on non-struct type: void
        fun foo() void {}
        foo().bar()
        ^^^^^
    }
```

**Float rejects modulo**

```metall
1.5 % 2.0
```

```error
test.met:1:1: type mismatch: binary operation '%' expects an integer, got Float
    1.5 % 2.0
    ^^^
```

**Float rejects bitwise and**

```metall
1.5 & 2.0
```

```error
test.met:1:1: type mismatch: binary operation '&' expects an integer, got Float
    1.5 & 2.0
    ^^^
```

**Float and F32 do not mix**

```metall
{ let x F32 = 1.5 let y = 2.5 x + y }
```

```error
test.met:1:35: type mismatch: expected type of LHS: F32, got Float
    { let x F32 = 1.5 let y = 2.5 x + y }
                                      ^
```

## Try

**Try as last expression in function body**

```metall module
fun foo() Result<void> {
    try Result<void>(void)
}
fun main() void {}
```

```types
Module: test
  Fun: fun01
    SimpleType: union01
    Block: union01
      Match: void
        TypeConstruction: union01
          Ident: union01
          Ident: void
        TryPattern: void
        Block: void
          Ident: void
        Block: never
          Return: never
            Ident: enum01
  Fun: fun02
    SimpleType: void
    Block: void
---
enum01  = Err
union01 = Result<void> = void | enum01
fun01   = sync fun() union01
fun02   = sync fun() void
```

## Module Constants

**Module-level let with all primitive types**

```metall module
let a = 42
let b = "hello"
let c = true
let d = 'x'
fun main() void {}
```

```types
Module: test
  Var: void
    Int: Int
  Var: void
    String: Str
  Var: void
    Bool: Bool
  Var: void
    RuneLiteral: Rune
  Fun: fun01
    SimpleType: void
    Block: void
---
fun01 = sync fun() void
```

**Module-level let with struct, array, and function usage**

```metall module
struct Point { x Int y Int }
let origin = Point(0, 0)
let primes = [2, 3, 5]
fun get_x() Int { origin.x }
fun first() Int { primes[0] }
fun main() void {}
```

```types
Module: test
  Struct: struct01
    StructField: ?
      SimpleType: ?
    StructField: ?
      SimpleType: ?
  Var: void
    TypeConstruction: struct01
      Ident: struct01
      Int: Int
      Int: Int
  Var: void
    ArrayLiteral: [3]Int
      Int: Int
      Int: Int
      Int: Int
  Fun: fun01
    SimpleType: Int
    Block: Int
      FieldAccess: Int
        Ident: struct01
  Fun: fun01
    SimpleType: Int
    Block: Int
      Index: Int
        Ident: [3]Int
        Int: Int
  Fun: fun02
    SimpleType: void
    Block: void
---
struct01 = Point { x Int, y Int }
fun01    = sync fun() Int
fun02    = sync fun() void
```

**Module-level let referencing earlier let**

```metall module
let x = 42
let y = x
fun main() void {}
```

```types
Module: test
  Var: void
    Int: Int
  Var: void
    Ident: Int
  Fun: fun01
    SimpleType: void
    Block: void
---
fun01 = sync fun() void
```

**Module-level let with reference to another constant**

```metall module
let x = 42
let y = &x
fun main() void {}
```

```types
Module: test
  Var: void
    Int: Int
  Var: void
    Ref: &Int
      Ident: Int
  Fun: fun01
    SimpleType: void
    Block: void
---
fun01 = sync fun() void
```

**Module-level let ordering: no forward references**

```metall module
let y = x
let x = 42
fun main() void {}
```

```error
test.met:1:9: symbol not defined: x
    let y = x
            ^
    let x = 42
```

**Module-level let duplicate name**

```metall module
let x = 1
let x = 2
fun main() void {}
```

```error
test.met:2:5: symbol already defined: x
    let x = 1
    let x = 2
        ^
    fun main() void {}
```


**Module-level let with type coercion is allowed**

```metall module
let x = U8(32)
fun main() void {}
```

```types
Module: test
  Var: void
    TypeConstruction: U8
      Ident: U8
      Int: U8
  Fun: fun01
    SimpleType: void
    Block: void
---
fun01 = sync fun() void
```

**Module-level let with function call is rejected**

```metall module
let x = 'a'.to_u32()
fun main() void {}
```

```error
test.met:1:9: function calls are not allowed in module-level constants
    let x = 'a'.to_u32()
            ^^^^^^^^^^^^
    fun main() void {}
```

## Unsafe

**Calling unsafe function without unsafe keyword**

```metall
{ let @a = Arena() @a.slice_uninit<Int>(5) }
```

```error
test.met:1:20: calling unsafe function requires the unsafe keyword
    { let @a = Arena() @a.slice_uninit<Int>(5) }
                       ^^^^^^^^^^^^^^^^^^^^^^^
```

**Using unsafe on non-unsafe function**

```metall
{ fun foo() Int { 42 } unsafe foo() }
```

```error
test.met:1:31: unsafe keyword can only be used on unsafe functions
    { fun foo() Int { 42 } unsafe foo() }
                                  ^^^^^
```

**Unsafe is not erased through an indirect binding**

Storing an unsafe function in a binding keeps the unsafe in its type, so the
call through that binding still requires the keyword.

```metall module
extern fun abs(n I32) I32
fun main() void {
    let f = abs
    _ = f(I32(-5))
}
```

```error
test.met:4:9: calling unsafe function requires the unsafe keyword
        let f = abs
        _ = f(I32(-5))
            ^^^^^^^^^^
    }
```

**Indirect unsafe call with the keyword is allowed**

```metall module
extern fun abs(n I32) I32
fun main() void {
    let f = abs
    let r = unsafe f(I32(-5))
    _ = r
}
```

```error
```

**Cannot pass an unsafe function where a safe one is expected**

```metall module
extern fun abs(n I32) I32
fun apply(f fun(I32) I32, x I32) I32 {
    f(x)
}
fun main() void {
    _ = apply(abs, I32(-5))
}
```

```error
test.met:6:15: type mismatch at argument 1: expected fun(I32) I32, got unsafe fun(I32) I32
    fun main() void {
        _ = apply(abs, I32(-5))
                  ^^^
    }
```

**A generic unsafe function stays unsafe when instantiated**

```metall module
unsafe fun id<T>(x T) T {
    x
}
fun main() void {
    _ = id(5)
}
```

```error
test.met:5:9: calling unsafe function requires the unsafe keyword
    fun main() void {
        _ = id(5)
            ^^^^^
    }
```

## Slice and Array Methods

**Slice method with shape constraint**

```metall
{
    shape Show { fun Show.show(self Show) Int }
    fun Slice.first<U Show>(s []U) Int { s[0].show() }
    fun Int.show(self Int) Int { self }
    let x = [1, 2, 3]
    x[..].first()
}
```

```types
Block: Int
  Shape: shape01
    FunDecl: ?
      FunParam: shape01
        SimpleType: shape01
      SimpleType: Int
  Fun: fun01
    TypeParam: U
      SimpleType: shape01
    FunParam: []U
      SliceType: []U
        SimpleType: U
    SimpleType: Int
    Block: Int
      Call: Int
        FieldAccess: fun02
          Index: U
            Ident: []U
            Int: Int
  Fun: fun03
    FunParam: Int
      SimpleType: Int
    SimpleType: Int
    Block: Int
      Ident: Int
  Var: void
    ArrayLiteral: [3]Int
      Int: Int
      Int: Int
      Int: Int
  Call: Int
    FieldAccess: fun04
      SubSlice: []Int
        Ident: [3]Int
        Range: struct01
---
shape01  = Show {  }
fun01    = fun([]U) Int
fun02    = fun(U) Int
fun03    = sync fun(Int) Int
fun04    = fun([]Int) Int
struct01 = Range { start Int, end Int }
```

## Panic

**panic results in type: never**
```metall
panic("boom")
```

```types
Call: never
  Ident: fun01
  String: Str
---
fun01 = sync fun(Str) never
```

**code after panic is unreachable**

```metall
{
    mut x = 12
    panic("boom")
    x = x + 1
}
```

```error
test.met:4:5: unreachable code
        panic("boom")
        x = x + 1
        ^^^^^^^^^
    }
```

## Extern Functions

**Extern function requires unsafe to call**

```metall module
extern fun foo() Int
fun main() void { _ = foo() }
```

```error
test.met:2:23: calling unsafe function requires the unsafe keyword
    extern fun foo() Int
    fun main() void { _ = foo() }
                          ^^^^^
```

**Extern function can be called with unsafe**

```metall module
extern fun foo() Int
fun main() void { _ = unsafe foo() }
```

```types
Module: test
  FunDecl: fun01
    SimpleType: Int
  Fun: fun02
    SimpleType: void
    Block: void
      Assign: void
        Ident: ?
        Call: Int
          Ident: fun01
---
fun01 = unsafe fun() Int
fun02 = sync fun() void
```

## Pub

**Shape requires pub method, struct has pub method**

```metall module
shape Greet { pub fun Greet.greet(g &Greet) Str }
struct Foo { name Str }
pub fun Foo.greet(f &Foo) Str { f.name }
fun foo<T Greet>(t &T) Str { t.greet() }
fun main() void { let f = Foo("hi") _ = foo<Foo>(&f) }
```

```error
```

**Shape requires pub method, struct has non-pub method**

```metall module
shape Greet { pub fun Greet.greet(g &Greet) Str }
struct Foo { name Str }
fun Foo.greet(f &Foo) Str { f.name }
fun foo<T Greet>(t &T) Str { t.greet() }
fun main() void { let f = Foo("hi") _ = foo<Foo>(&f) }
```

```error
test.met:5:41: type test.Foo does not satisfy shape Greet: method greet must be public
    fun foo<T Greet>(t &T) Str { t.greet() }
    fun main() void { let f = Foo("hi") _ = foo<Foo>(&f) }
                                            ^^^^^^^^
```

**Shape requires non-pub method, struct has pub method**

```metall module
shape Greet { fun Greet.greet(g &Greet) Str }
struct Foo { name Str }
pub fun Foo.greet(f &Foo) Str { f.name }
fun foo<T Greet>(t &T) Str { t.greet() }
fun main() void { let f = Foo("hi") _ = foo<Foo>(&f) }
```

```error
test.met:5:41: type test.Foo does not satisfy shape Greet: method greet must not be public
    fun foo<T Greet>(t &T) Str { t.greet() }
    fun main() void { let f = Foo("hi") _ = foo<Foo>(&f) }
                                            ^^^^^^^^
```

**Pub union with pub variants is ok**

```metall module
pub struct MyErr { pub msg Str }
pub union MyResult = Int | MyErr
fun main() void {}
```

```error
```

**Pub union with non-pub variant is rejected**

```metall module
struct Secret { msg Str }
pub union Bad = Int | Secret
fun main() void {}
```

```error
test.met:2:23: public union Bad contains non-public variant type test.Secret
    struct Secret { msg Str }
    pub union Bad = Int | Secret
                          ^^^^^^
    fun main() void {}
```

## Sync

**closure with sync captures is sync fun**

```metall
{
    fun takes_sync(f sync fun() void) void {}
    let x = 42
    takes_sync(fun[x]() void { _ = x })
}
```

```error
```

**non-capturing closure is sync fun**

```metall
{
    fun takes_sync(f sync fun() void) void {}
    takes_sync(fun() void {})
}
```

```error
```

**closure with ref capture is not sync fun**

```metall
{
    fun takes_sync(f sync fun() void) void {}
    mut x = 42
    takes_sync(fun[&mut x]() void { x.* = 1 })
}
```

```error
test.met:4:16: type mismatch at argument 1: expected sync fun() void, got fun() void
        mut x = 42
        takes_sync(fun[&mut x]() void { x.* = 1 })
                   ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
    }
```

**fun value is not assignable to sync fun**

```metall
{
    fun takes_sync(f sync fun() void) void {}
    fun not_sync(f fun() void) fun() void { f }
    let f = not_sync(fun() void {})
    takes_sync(f)
}
```

```error
test.met:5:16: type mismatch at argument 1: expected sync fun() void, got fun() void
        let f = not_sync(fun() void {})
        takes_sync(f)
                   ^
    }
```

**sync fun is assignable to fun**

```metall
{
    fun takes_fun(f fun() void) void {}
    let x = 42
    takes_fun(fun[x]() void { _ = x })
}
```

```error
```

**closure capturing non-sync value is not sync**

```metall
{
    fun takes_sync(f sync fun() void) void {}
    let v = 42
    let r = &v
    takes_sync(fun[r]() void { _ = r })
}
```

```error
test.met:5:16: type mismatch at argument 1: expected sync fun() void, got fun() void
        let r = &v
        takes_sync(fun[r]() void { _ = r })
                   ^^^^^^^^^^^^^^^^^^^^^^^
    }
```

**unsafe sync struct overrides field check**

```metall module
unsafe sync struct Safe { r &Int }
fun is_sync(x sync fun() void) void {}
fun main() void {
    let v = 42
    let s = Safe(&v)
    is_sync(fun[s]() void { _ = s })
}
```

```error
```

**sync struct without unsafe is rejected**

Declaring a struct shareable across threads is a soundness assertion, so it must
sit on the unsafe surface.

```metall module
sync struct Shared { data Int }
fun main() void {}
```

```error
test.met:1:6: a sync struct must be declared `unsafe sync struct`: asserting it is safe to share across threads is a soundness claim
    sync struct Shared { data Int }
         ^^^^^^^^^^^^^^^^^^^^^^^^^^
    fun main() void {}
```

**named function with all-sync params and return is sync**

```metall
{
    fun takes_sync(f sync fun(Int) Int) void {}
    fun add_one(x Int) Int { x }
    takes_sync(add_one)
}
```

```error
```

**named function with non-sync param is not sync**

```metall
{
    fun takes_sync(f sync fun(&Int) void) void {}
    fun use_ref(r &Int) void { _ = r }
    takes_sync(use_ref)
}
```

```error
test.met:4:16: type mismatch at argument 1: expected sync fun(&Int) void, got fun(&Int) void
        fun use_ref(r &Int) void { _ = r }
        takes_sync(use_ref)
                   ^^^^^^^
    }
```

**named function with non-sync return is not sync**

```metall module
fun get_ref() &Int {
    let v = 42
    &v
}
fun takes_sync(f sync fun() &Int) void {}
fun main() void {
    takes_sync(get_ref)
}
```

```error
test.met:7:16: type mismatch at argument 1: expected sync fun() &Int, got fun() &Int
    fun main() void {
        takes_sync(get_ref)
                   ^^^^^^^
    }
```

**assign sync fun to non-sync var (coercion)**

```metall
{
    fun add(x Int) Int { x }
    let f fun() void = fun() void {}
    _ = f
    _ = add
}
```

```error
```

**assign non-sync fun to sync var should not compile**

```metall
{
    fun takes_ref(r &Int) void { _ = r }
    mut x sync fun(&Int) void = takes_ref
    _ = x
}
```

```error
test.met:3:33: type mismatch: expected sync fun(&Int) void, got fun(&Int) void
        fun takes_ref(r &Int) void { _ = r }
        mut x sync fun(&Int) void = takes_ref
                                    ^^^^^^^^^
        _ = x

test.met:4:9: symbol not defined: x
        mut x sync fun(&Int) void = takes_ref
        _ = x
            ^
    }
```

**reassign sync var with non-sync fun should not compile**

```metall
{
    fun is_sync(x Int) void { _ = x }
    fun not_sync(r &Int) void { _ = r }
    mut x sync fun(Int) void = is_sync
    x = not_sync
}
```

```error
test.met:5:9: type mismatch: expected sync fun(Int) void, got fun(&Int) void
        mut x sync fun(Int) void = is_sync
        x = not_sync
            ^^^^^^^^
    }
```

**return sync fun from non-sync return type (coercion)**

```metall module
fun make_fun() fun() void {
    fun inner() void {}
    inner
}
fun main() void {
    _ = make_fun()
}
```

```error
```

**return non-sync fun from sync return type should not compile**

```metall module
fun make_sync() sync fun(&Int) void {
    fun inner(r &Int) void { _ = r }
    inner
}
fun main() void {
    _ = make_sync()
}
```

```error
test.met:3:5: return type mismatch: expected sync fun(&Int) void, got fun(&Int) void
        fun inner(r &Int) void { _ = r }
        inner
        ^^^^^
    }
```

**array of sync type is sync**

```metall
{
    fun takes_sync(f sync fun() void) void {}
    let arr = [1, 2, 3]
    takes_sync(fun[arr]() void { _ = arr })
}
```

```error
```

**array of non-sync type is not sync**

```metall
{
    fun takes_sync(f sync fun() void) void {}
    let v = 42
    let arr = [&v]
    takes_sync(fun[arr]() void { _ = arr })
}
```

```error
test.met:5:16: type mismatch at argument 1: expected sync fun() void, got fun() void
        let arr = [&v]
        takes_sync(fun[arr]() void { _ = arr })
                   ^^^^^^^^^^^^^^^^^^^^^^^^^^^
    }
```

**struct with all sync fields is sync**

```metall
{
    struct Pair { a Int b Bool }
    fun takes_sync(f sync fun() void) void {}
    let p = Pair(1, true)
    takes_sync(fun[p]() void { _ = p })
}
```

```error
```

**struct with ref field is not sync**

```metall
{
    struct HasRef { r &Int }
    fun takes_sync(f sync fun() void) void {}
    let v = 42
    let h = HasRef(&v)
    takes_sync(fun[h]() void { _ = h })
}
```

```error
test.met:6:16: type mismatch at argument 1: expected sync fun() void, got fun() void
        let h = HasRef(&v)
        takes_sync(fun[h]() void { _ = h })
                   ^^^^^^^^^^^^^^^^^^^^^^^
    }
```

**union with all sync variants is sync**

```metall module
union IntOrBool = Int | Bool
fun takes_sync(f sync fun() void) void {}
fun main() void {
    let u = IntOrBool(42)
    takes_sync(fun[u]() void { _ = u })
}
```

```error
```

**union with non-sync variant is not sync**

```metall module
union RefOrInt = &Int | Int
fun takes_sync(f sync fun() void) void {}
fun main() void {
    let v = 42
    let u = RefOrInt(&v)
    takes_sync(fun[u]() void { _ = u })
}
```

```error
test.met:6:16: type mismatch at argument 1: expected sync fun() void, got fun() void
        let u = RefOrInt(&v)
        takes_sync(fun[u]() void { _ = u })
                   ^^^^^^^^^^^^^^^^^^^^^^^
    }
```

**unsafe sync union overrides variant check**

```metall module
unsafe sync union MaybeRef = &Int | Int
fun is_sync(x sync fun() void) void {}
fun main() void {
    let v = 42
    let u = MaybeRef(&v)
    is_sync(fun[u]() void { _ = u })
}
```

```error
```

**sync union without unsafe is rejected**

```metall module
sync union MaybeRef = &Int | Int
fun main() void {}
```

```error
test.met:1:6: a sync union must be declared `unsafe sync union`: asserting it is safe to share across threads is a soundness claim
    sync union MaybeRef = &Int | Int
         ^^^^^^^^^^^^^^^^^^^^^^^^^^^
    fun main() void {}
```

**closure capturing sync fun is sync**

```metall
{
    fun takes_sync(f sync fun() void) void {}
    fun inner() void {}
    takes_sync(fun[inner]() void { inner() })
}
```

```error
```

**sync type param accepts sync type**

```metall
{
    struct Box<sync T> { value T }
    let b = Box<Int>(42)
}
```

```error
```

**sync type param rejects non-sync type**

```metall
{
    struct Box<sync T> { value T }
    struct Refs { r &Int }
    let x = 1
    let b = Box<Refs>(Refs(&x))
}
```

```error
test.met:5:13: type argument T must be sync, got Refs
        let x = 1
        let b = Box<Refs>(Refs(&x))
                ^^^^^^^^^
    }
```

## Unsync

**unsync struct with all-sync fields is not sync**

```metall module
unsync struct NotSync { x Int y Bool }
fun is_sync(f sync fun() void) void {}
fun main() void {
    let s = NotSync(1, true)
    is_sync(fun[s]() void { _ = s })
}
```

```error
test.met:5:13: type mismatch at argument 1: expected sync fun() void, got fun() void
        let s = NotSync(1, true)
        is_sync(fun[s]() void { _ = s })
                ^^^^^^^^^^^^^^^^^^^^^^^
    }
```

**unsync union with all-sync variants is not sync**

```metall module
unsync union NotSync = Int | Bool
fun is_sync(f sync fun() void) void {}
fun main() void {
    let u = NotSync(42)
    is_sync(fun[u]() void { _ = u })
}
```

```error
test.met:5:13: type mismatch at argument 1: expected sync fun() void, got fun() void
        let u = NotSync(42)
        is_sync(fun[u]() void { _ = u })
                ^^^^^^^^^^^^^^^^^^^^^^^
    }
```

**unsync fun with all-sync params is not sync**

```metall
{
    fun takes_sync(f sync fun() void) void {}
    unsync fun inner() void {}
    takes_sync(inner)
}
```

```error
test.met:4:16: type mismatch at argument 1: expected sync fun() void, got fun() void
        unsync fun inner() void {}
        takes_sync(inner)
                   ^^^^^
    }
```

**unsync fun closure with all-sync captures is not sync**

```metall
{
    fun takes_sync(f sync fun() void) void {}
    let x = 42
    takes_sync(unsync fun[x]() void { _ = x })
}
```

```error
test.met:4:23: type mismatch at argument 1: expected sync fun() void, got fun() void
        let x = 42
        takes_sync(unsync fun[x]() void { _ = x })
                          ^^^^^^^^^^^^^^^^^^^^^^^
    }
```

**pub unsync struct parses correctly**

```metall module
pub unsync struct Foo { pub x Int }
fun is_sync(f sync fun() void) void {}
fun main() void {
    let s = Foo(1)
    is_sync(fun[s]() void { _ = s })
}
```

```error
test.met:5:13: type mismatch at argument 1: expected sync fun() void, got fun() void
        let s = Foo(1)
        is_sync(fun[s]() void { _ = s })
                ^^^^^^^^^^^^^^^^^^^^^^^
    }
```

**pub unsync union parses correctly**

```metall module
pub unsync union Foo = Int | Bool
fun is_sync(f sync fun() void) void {}
fun main() void {
    let u = Foo(42)
    is_sync(fun[u]() void { _ = u })
}
```

```error
test.met:5:13: type mismatch at argument 1: expected sync fun() void, got fun() void
        let u = Foo(42)
        is_sync(fun[u]() void { _ = u })
                ^^^^^^^^^^^^^^^^^^^^^^^
    }
```

**unsync fun type annotation**

```metall
{
    fun is_sync(x Int) void { _ = x }
    mut x unsync fun(Int) void = is_sync
    _ = x
}
```

```error
```

**unsync fun assigned to sync var should not compile**

```metall
{
    fun takes_sync(f sync fun() void) void {}
    unsync fun inner() void {}
    mut x sync fun() void = inner
    _ = x
    takes_sync(inner)
}
```

```error
test.met:4:29: type mismatch: expected sync fun() void, got fun() void
        unsync fun inner() void {}
        mut x sync fun() void = inner
                                ^^^^^
        _ = x

test.met:5:9: symbol not defined: x
        mut x sync fun() void = inner
        _ = x
            ^
        takes_sync(inner)

test.met:6:16: type mismatch at argument 1: expected sync fun() void, got fun() void
        _ = x
        takes_sync(inner)
                   ^^^^^
    }
```

**struct with unsync field is not sync**

```metall module
unsync struct Inner { x Int }
struct Outer { i Inner }
fun is_sync(f sync fun() void) void {}
fun main() void {
    let o = Outer(Inner(1))
    is_sync(fun[o]() void { _ = o })
}
```

```error
test.met:6:13: type mismatch at argument 1: expected sync fun() void, got fun() void
        let o = Outer(Inner(1))
        is_sync(fun[o]() void { _ = o })
                ^^^^^^^^^^^^^^^^^^^^^^^
    }
```

## Noescape

**noescape param is reflected in function type**

```metall
{
    fun foo(x noescape &Int) void {}
}
```

```types
Block: fun01
  Fun: fun01
    FunParam: &Int
      RefType: &Int
        SimpleType: Int
    SimpleType: void
    Block: void
---
fun01 = fun(noescape &Int) void
```

```error
```

**fun with noescape param accepts fun without noescape param**

```metall
{
    fun read(x &Int) Int { x.* }
    fun apply(f fun(noescape &Int) Int) void {}
    apply(read)
}
```

```error
```

## Export

**export of Int-returning Int fun (module)**

```metall module
fun add(a Int, b Int) Int { a + b }
export metall_add = add
```

```error
```

**export of Bool-returning Bool fun (module)**

```metall module
fun ident(x Bool) Bool { x }
export metall_ident = ident
```

```error
```

**export of void-returning fun (module)**

```metall module
fun do_nothing() void { }
export metall_noop = do_nothing
```

```error
```

**duplicate export name rejected (module)**

```metall module
fun first(a Int) Int { a }
fun second(a Int) Int { a }
export metall_dup = first
export metall_dup = second
```

```error
test.met:3:8: export name already used: metall_dup
    fun second(a Int) Int { a }
    export metall_dup = first
           ^^^^^^^^^^
    export metall_dup = second

test.met:4:8: export name already used: metall_dup
    export metall_dup = first
    export metall_dup = second
           ^^^^^^^^^^
```

**export rejects non-function target (module)**

```metall module
let value = 42
export metall_value = value
```

```error
test.met:2:23: export target must be a function
    let value = 42
    export metall_value = value
                          ^^^^^
```

**export rejects bare generic reference (module)**

```metall module
fun id<T>(x T) T { x }
export metall_id = id
```

```error
test.met:2:20: type argument count mismatch: expected 1, got 0
    fun id<T>(x T) T { x }
    export metall_id = id
                       ^^
```

**export rejects extern target (module)**

```metall module
extern fun abs(x Int) Int
export metall_abs = abs
```

```error
test.met:2:21: export target must be a Metall function declared in the current module
    extern fun abs(x Int) Int
    export metall_abs = abs
                        ^^^
```

**export rejects Str param (module)**

```metall module
fun greet(s Str) void { }
export metall_greet = greet
```

```error
test.met:1:11: parameter type 'Str' is not exportable to C
    fun greet(s Str) void { }
              ^^^^^
    export metall_greet = greet
```

**export rejects Str return (module)**

```metall module
fun get_name() Str { "hello" }
export metall_get_name = get_name
```

```error
test.met:1:16: return type 'Str' is not exportable to C
    fun get_name() Str { "hello" }
                   ^^^
    export metall_get_name = get_name
```

**export rejects void param (module)**

```metall module
fun bad(v void) Int { 0 }
export metall_bad = bad
```

```error
test.met:1:9: parameter type 'void' is not exportable to C
    fun bad(v void) Int { 0 }
            ^^^^^^
    export metall_bad = bad
```

## Recursive Types

**Mutually recursive generics through references**

```metall
{
    struct Foo<T> { value T next &Bar<T> }
    struct Bar<T> { tag T prev &Foo<T> }
    fun build(f &Foo<Int>, b &Bar<Int>) Int { f.next.tag + b.prev.value }
}
```

```error
```

**Generic struct cached across distinct instantiations**

```metall
{
    struct Pair<A, B> { first A second B }
    fun mixer(p Pair<Int, Str>, q Pair<Str, Int>) Int { p.first + q.second }
}
```

```error
```

**Generic instantiation re-entered with same args returns cache hit**

```metall
{
    struct Wrap<T> { value T }
    fun pair(a Wrap<Int>, b Wrap<Int>) Int { a.value + b.value }
}
```

```error
```

## Enums

**Signed backing with negative discriminants**

```metall
{ enum Cmp I8 = less = -1 | equal = 0 | greater = 1 }
```

```error
```


**Mixed explicit and implicit tags**

```metall
{ enum Mix U8 = a = 1 | b }
```

```error
test.met:1:3: enum Mix: tags must be all-or-none
    { enum Mix U8 = a = 1 | b }
      ^^^^^^^^^^^^^^^^^^^^^^^
```

**Duplicate variant**

```metall
{ enum Dup U8 = a | a }
```

```error
test.met:1:21: duplicate enum variant: a
    { enum Dup U8 = a | a }
                        ^
```

**Backed by a non-integer, non-enum type**

```metall
{ struct S {} enum Bad S = a | b }
```

```error
test.met:1:24: enum Bad must be backed by an integer type or an open enum, got S
    { struct S {} enum Bad S = a | b }
                           ^
```

**Unknown backing referenced by a struct field**

A struct field naming the enum is completed before the enum, so the enum's
shared cached type must not be flipped to ok before its own completion fails on
the unknown backing.

```metall
{
    enum Bad DoesNotExist = a | b
    struct Holder {
        f Bad
    }
}
```

```error
test.met:2:14: symbol not defined: DoesNotExist
    {
        enum Bad DoesNotExist = a | b
                 ^^^^^^^^^^^^
        struct Holder {
```

**Subset variant cannot have an explicit tag**

```metall module
enum Root U32
enum Sub Root = a = 5 | b = 6
```

```error
test.met:2:21: subset enum variant a cannot have an explicit tag
    enum Root U32
    enum Sub Root = a = 5 | b = 6
                        ^
```

**Duplicate explicit tag value**

```metall
{ enum E U8 = a = 5 | b = 5 }
```

```error
test.met:1:27: duplicate enum tag: 5
    { enum E U8 = a = 5 | b = 5 }
                              ^
```

**Variant named debug_name is reserved**

```metall
{ enum E U8 = debug_name | other }
```

```error
test.met:1:15: debug_name is a reserved enum member name
    { enum E U8 = debug_name | other }
                  ^^^^^^^^^^
```

**Associated value referencing an unbound function is rejected**

Enum bodies are resolved before top-level functions are bound, so a forward
reference to one is an unbound symbol.

```metall module
fun f() U32 { 1 }
enum C(x U32) U8 = a(f())
```

```error
test.met:2:22: symbol not defined: f
    fun f() U32 { 1 }
    enum C(x U32) U8 = a(f())
                         ^
```

**Associated value cannot be a function call**

```metall
{ enum C(x U32) U8 = a(U8(1).to_u32()) }
```

```error
test.met:1:24: enum associated values cannot contain function calls
    { enum C(x U32) U8 = a(U8(1).to_u32()) }
                           ^^^^^^^^^^^^^^
```

**Missing required associated value**

```metall
{ enum C(x U32) U8 = a }
```

```error
test.met:1:22: enum variant a: missing associated value for x
    { enum C(x U32) U8 = a }
                         ^
```

**Too many associated values**

```metall
{ enum C(x U32) U8 = a(1, 2) }
```

```error
test.met:1:22: too many arguments: expected at most 1, got 2
    { enum C(x U32) U8 = a(1, 2) }
                         ^
```

**Nested subset is rejected**

```metall module
enum Root U32
enum Sub Root = a | b
enum Deep Sub = x | y
```

```error
test.met:3:11: enum Deep: test.Sub is not an open enum and cannot be subsetted
    enum Sub Root = a | b
    enum Deep Sub = x | y
              ^^^
```

**Associated-data field defaults are rejected**

```metall
{ enum C(x U32 = 5) U8 = a | b }
```

```error
test.met:1:10: associated-data field defaults are not supported; use ?T for an optional field
    { enum C(x U32 = 5) U8 = a | b }
             ^
```

**Explicit tag out of backing range**

```metall
{ enum C U8 = a = 300 }
```

```error
test.met:1:19: tag 300 does not fit backing type U8
    { enum C U8 = a = 300 }
                      ^^^
```

**Explicit tag below signed backing range**

```metall
{ enum C I8 = a = -200 }
```

```error
test.met:1:19: tag -200 does not fit backing type I8
    { enum C I8 = a = -200 }
                      ^^^^
```

**Subset cannot declare its own associated-data params**

```metall module
enum Root U32
enum Sub(x U8) Root = a | b
```

```error
test.met:2:1: subset enum Sub cannot declare its own associated-data params
    enum Root U32
    enum Sub(x U8) Root = a | b
    ^^^^^^^^^^^^^^^^^^^^^^^^^^^
```

**Subset of an open enum must have variants**

```metall module
enum Root U32
enum Sub Root
```

```error
test.met:2:1: enum Sub: a subset of an open enum must have variants
    enum Root U32
    enum Sub Root
    ^^^^^^^^^^^^^
```

**Associated value type mismatch**

```metall
{ enum C(x U32) U8 = a("hi") }
```

```error
test.met:1:24: associated value type mismatch for x: expected U32, got Str
    { enum C(x U32) U8 = a("hi") }
                           ^^^^
```

**Associated data on an enum that declares no params**

```metall
{ enum C U8 = a(1) }
```

```error
test.met:1:15: enum variant a has associated data but the enum declares no params
    { enum C U8 = a(1) }
                  ^
```

**debug_name reserved as an associated-data field**

```metall
{ enum C(debug_name Str) U8 = a | b }
```

```error
test.met:1:10: debug_name is a reserved associated-data field name
    { enum C(debug_name Str) U8 = a | b }
             ^^^^^^^^^^
```

**Associated value cannot read another enum's data**

```metall
{
    enum Zebra(x U8) U8 = z(7)
    enum Alpha(y U8) U8 = a(Zebra.z.x)
}
```

```error
test.met:3:29: enum associated values cannot read enum fields
        enum Zebra(x U8) U8 = z(7)
        enum Alpha(y U8) U8 = a(Zebra.z.x)
                                ^^^^^^^^^
    }
```

**Method cannot have same name as an enum field**

```metall
{ enum Color(rgb U32) U8 = red(1) fun Color.rgb(c Color) U32 { 0 } }
```

```error
test.met:1:39: method name conflicts with field: Color.rgb
    { enum Color(rgb U32) U8 = red(1) fun Color.rgb(c Color) U32 { 0 } }
                                          ^^^^^^^^^
```

**Method on an enum cannot be named debug_name**

```metall
{ enum Color U8 = red fun Color.debug_name(c Color) Str { "x" } }
```

```error
test.met:1:27: method name conflicts with field: Color.debug_name
    { enum Color U8 = red fun Color.debug_name(c Color) Str { "x" } }
                              ^^^^^^^^^^^^^^^^
```

**A non-enum does not satisfy the Enum constraint**

```metall module
struct RawEnum<T Enum> {
    value T.Item
}
fun main() void {
    let r = RawEnum<Int>(5)
}
```

```error
test.met:5:13: type Int does not satisfy Enum: not an enum type
    fun main() void {
        let r = RawEnum<Int>(5)
                ^^^^^^^^^^^^
    }
```

**Match pattern that is not a variant or subset**

```metall
{
    enum C U8 = a | b
    let c = C.a
    match c {
        case Int: 1
    }
}
```

```error
test.met:5:14: Int is not an enum variant or subset of C
        match c {
            case Int: 1
                 ^^^
        }
```

**== between different enums is rejected**

```metall
{
    enum A U8 = x | y
    enum B U8 = p | q
    let r = A.x == B.p
}
```

```error
test.met:4:20: type mismatch: expected type of LHS: A, got B
        enum B U8 = p | q
        let r = A.x == B.p
                       ^^^
    }
```

**== between an enum and an integer is rejected**

```metall
{
    enum A U8 = x | y
    let r = A.x == 5
}
```

```error
test.met:3:20: type mismatch: expected type of LHS: A, got Int
        enum A U8 = x | y
        let r = A.x == 5
                       ^
    }
```

**Enum value typing: method result, accessors, debug_name, coercion, T.Item**

A method returns its enum, the associated-data accessors and `debug_name` carry
their declared types, a subset coerces to its root, and `T.Item` is the backing
integer.

```metall module
enum Ordering U8 = less | equal | greater

pub fun Ordering.flip(o Ordering) Ordering {
    match o {
        case Ordering.less: Ordering.greater
        case Ordering.equal: Ordering.equal
        case Ordering.greater: Ordering.less
    }
}

enum Color(name Str, rgb U32) U8 = red("Red", 0xff0000)
enum AppErr U32
enum IOErr AppErr = oops

struct RawEnum<T Enum> {
    value T.Item
}

fun main() void {
    let flipped = Ordering.less.flip()
    let name = Color.red.name
    let code = Color.red.rgb
    let dn = Color.red.debug_name
    let coerced AppErr = IOErr.oops
    let raw = RawEnum<Color>(2).value
}
```

```bindings
Module: scope01
  Enum: scope02
    SimpleType: scope03
    EnumVariant: scope03
    EnumVariant: scope03
    EnumVariant: scope03
  Fun: scope02
    FunParam: scope04
      SimpleType: scope04
    SimpleType: scope04
    Block: scope04
      Match: scope05
        Ident: scope05
        FieldAccess: scope05
          SimpleType: scope05
        Block: scope05
          Ident: scope06
        FieldAccess: scope05
          SimpleType: scope05
        Block: scope05
          Ident: scope07
        FieldAccess: scope05
          SimpleType: scope05
        Block: scope05
          Ident: scope08
  Enum: scope02
    SimpleType: scope09
    FunParam: scope09
      SimpleType: scope09
    FunParam: scope09
      SimpleType: scope09
    EnumVariant: scope09
      String: scope09
      Int: scope09
  Enum: scope02
    SimpleType: scope10
  Enum: scope02
    SimpleType: scope11
    EnumVariant: scope11
  Struct: scope02
    TypeParam: scope12
      SimpleType: scope12
    StructField: scope12
      SimpleType: scope12
  Fun: scope02
    SimpleType: scope13
    Block: scope13
      Var: scope14
        Call: scope14
          FieldAccess: scope14
            Ident: scope14
      Var: scope14
        FieldAccess: scope14
          Ident: scope14
      Var: scope14
        FieldAccess: scope14
          Ident: scope14
      Var: scope14
        FieldAccess: scope14
          Ident: scope14
      Var: scope14
        SimpleType: scope14
        Ident: scope14
      Var: scope14
        FieldAccess: scope14
          TypeConstruction: scope14
            Ident: scope14
            Int: scope14
---
scope01:
scope02:
  AppErr: enum01
  Color: enum02
  IOErr: enum03
  Ordering: enum04
  RawEnum: struct01
  main: fun01
  test.Color.red: enum02
  test.IOErr.oops: enum03
  test.Ordering.equal: enum04
  test.Ordering.flip: fun02
  test.Ordering.greater: enum04
  test.Ordering.less: enum04
scope03:
scope04:
  o: enum04
scope05:
scope06:
scope07:
scope08:
scope09:
scope10:
scope11:
scope12:
  T: ?
scope13:
scope14:
  code: U32
  coerced: enum01
  dn: Str
  flipped: enum04
  name: Str
  raw: U8
enum01   = test.AppErr
enum02   = test.Color = red
enum03   = test.IOErr = oops
enum04   = test.Ordering = less | equal | greater
struct01 = RawEnum { value Item }
fun01    = sync fun() void
fun02    = sync fun(enum04) enum04
```

**== and != on same-family enums type-check to Bool**

```metall
{
    enum AppErr U32
    enum IOErr AppErr = a | b
    let cross AppErr = IOErr.a
    let same = IOErr.a == IOErr.a
    let diff = IOErr.a != IOErr.b
    let family = cross == IOErr.b
}
```

```bindings
Block: scope01
  Enum: scope02
    SimpleType: scope03
  Enum: scope02
    SimpleType: scope04
    EnumVariant: scope04
    EnumVariant: scope04
  Var: scope02
    SimpleType: scope02
    Ident: scope02
  Var: scope02
    Binary: scope02
      Ident: scope02
      Ident: scope02
  Var: scope02
    Binary: scope02
      Ident: scope02
      Ident: scope02
  Var: scope02
    Binary: scope02
      Ident: scope02
      Ident: scope02
---
scope01:
scope02:
  AppErr: enum01
  IOErr: enum02
  IOErr.a: enum02
  IOErr.b: enum02
  cross: enum01
  diff: Bool
  family: Bool
  same: Bool
scope03:
scope04:
enum01 = AppErr
enum02 = IOErr = a | b
```
