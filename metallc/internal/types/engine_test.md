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
{ 123 "hello" }
```

```types
Block: Str
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
      SimpleType: Str
    Int: Int
---
fun01 = fun(Int) void
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
fun01 = fun(Int, Str) Int
```

**Fun void return coerces body to void**

```metall
fun foo() void { 123 }
```

```types
Fun: fun01
  SimpleType: void
  Block: Int
    Int: Int
---
fun01 = fun() void
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
fun01 = fun(Int) Int
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
fun01 = fun(Int) void
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
fun01 = fun(Int) Int
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
fun01 = fun() void
```

**Builtin print_str**

```metall
DebugIntern.print_str("hello")
```

```types
Call: void
  Ident: fun01
  String: Str
---
fun01 = fun(Str) void
```

**Builtin print_int**

```metall
DebugIntern.print_int(123)
```

```types
Call: void
  Ident: fun01
  Int: Int
---
fun01 = fun(Int) void
```

**Builtin print_bool**

```metall
DebugIntern.print_bool(true)
```

```types
Call: void
  Ident: fun01
  Bool: Bool
---
fun01 = fun(Bool) void
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
  Block: void
    Return: void
      Int: Int
---
fun01 = fun() Int
```

**Return void**

```metall
fun foo() void { return void }
```

```types
Fun: fun01
  SimpleType: void
  Block: void
    Return: void
      Ident: void
---
fun01 = fun() void
```

**Return expr type is void**

```metall
fun foo() Int { return 123 }
```

```types
Fun: fun01
  SimpleType: Int
  Block: void
    Return: void
      Int: Int
---
fun01 = fun() Int
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
  Fun: fun02
    FunParam: Int
      SimpleType: Int
    SimpleType: Str
    Block: Str
      String: Str
  Fun: fun02
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
    FunParam: fun01
      FunType: fun01
        SimpleType: Int
        SimpleType: Int
    SimpleType: Int
    Block: Int
      Call: Int
        Ident: fun01
        Int: Int
  Call: Int
    Ident: fun02
    Ident: fun01
---
fun01 = fun(Int) Int
fun02 = fun(fun01) Int
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
fun01 = fun(Int) Int
```

**Nested function cannot reference outer variable**

```metall
{
    let x = 1
    fun(a Int) Int { x + a }
    fun bar(a Int) Int { x + a }
}
```

```error
test.met:4:26: symbol not defined: x
        fun(a Int) Int { x + a }
        fun bar(a Int) Int { x + a }
                             ^
    }

test.met:3:22: cannot reference "x" from outer scope
        let x = 1
        fun(a Int) Int { x + a }
                         ^
        fun bar(a Int) Int { x + a }
```

**Nested function cannot reference outer function parameter**

```metall
fun foo(a Int) fun(Int) Int {
    fun(b Int) Int { a + b }
    fun bar(b Int) Int { a + b }
}
```

```error
test.met:3:26: cannot reference "a" from outer scope
        fun(b Int) Int { a + b }
        fun bar(b Int) Int { a + b }
                             ^
    }

test.met:2:22: cannot reference "a" from outer scope
    fun foo(a Int) fun(Int) Int {
        fun(b Int) Int { a + b }
                         ^
        fun bar(b Int) Int { a + b }
```

**Nested function cannot reference outer allocator**

```metall
{
    let @a = Arena()
    fun() void { @a }
    fun bar() void { @a }
}
```

```error
test.met:4:22: symbol not defined: @a
        fun() void { @a }
        fun bar() void { @a }
                         ^^
    }

test.met:3:18: cannot reference "@a" from outer scope
        let @a = Arena()
        fun() void { @a }
                     ^^
        fun bar() void { @a }
```

**Nested function can reference outer function and struct**

```metall
{
    fun double(x Int) Int { x + x }
    struct Foo { x Int }
    fun(a Int) Int { double(a) }
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
fun01    = fun(Int) Int
struct01 = Foo { x Int }
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
fun01    = fun(struct01) void
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
{ struct Foo { one Str } let x = Foo("hello") &x }
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
{ struct Foo { mut one Str } mut x = Foo("hello") x.one = "bye" }
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
struct01 = Foo { mut one Str }
```

**Field write through mut ref param**

```metall
{ struct Foo { mut one Str } fun foo(a &mut Foo) void { a.one = "X" } mut x = Foo("hello") foo(&mut x) }
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
struct01 = Foo { mut one Str }
fun01    = fun(&mut struct01) void
```

**Nested field write on mut struct**

```metall
{ struct Foo { mut one Int } struct Bar { mut one Foo } mut x = Bar(Foo(1)) x.one.one = 2 }
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
struct01 = Foo { mut one Int }
struct02 = Bar { mut one struct01 }
```

**Field write through let binding of mut ref**

```metall
{ struct Foo { mut one Str } mut x = Foo("hello") let y = &mut x y.one = "X" }
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
struct01 = Foo { mut one Str }
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
fun01   = fun(union01) void
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
fun01    = fun(union01) void
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
        SimpleType: Int
    SimpleType: void
    Block: void
---
union01 = Maybe<Int> = Int | Bool
fun01   = fun(union01) void
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
        SimpleType: Int
    SimpleType: void
    Block: void
  Fun: fun02
    FunParam: union03
      SimpleType: union03
        SimpleType: Int
    SimpleType: void
    Block: void
  Fun: fun01
    FunParam: union01
      SimpleType: union01
        SimpleType: Str
    SimpleType: void
    Block: void
---
union01 = Maybe<Str> = Str | Bool
fun01   = fun(union01) void
union02 = Maybe = T | Bool
union03 = Maybe<Int> = Int | Bool
fun02   = fun(union03) void
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
fun01    = fun(union01, struct01) Int
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
        SimpleType: Int
    SimpleType: void
    Block: void
---
struct01 = Box<Int> { value Int }
union01  = Maybe<Int> = Int | struct01
fun01    = fun(union01) void
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
      SimpleType: Int
    Int: Int
---
union01 = Maybe<Int> = Int | Bool
union02 = Maybe = T | Bool
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
fun01   = fun(union01) void
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
    Block: void
      Return: void
        Int: Int
  Fun: fun01
    SimpleType: union01
    Block: union01
      String: Str
---
union01 = Foo = Str | Int
fun01   = fun() union01
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
fun01   = fun(Bool) union01
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
fun01   = fun() void
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
      SimpleType: scope02
    Int: scope02
  Var: scope02
    SimpleType: scope02
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
fun01    = fun(Bool) union02
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
fun foo() Int { if true { return 123 } else { "hello" } 321 }
```

```types
Fun: fun01
  SimpleType: Int
  Block: Int
    If: void
      Bool: Bool
      Block: void
        Return: void
          Int: Int
      Block: Str
        String: Str
    Int: Int
---
fun01 = fun() Int
```

**If with one branch break**

```metall
fun foo() void { for { if true { break } else { "hello" } } }
```

```types
Fun: fun01
  SimpleType: void
  Block: void
    For: void
      Block: void
        If: void
          Bool: Bool
          Block: void
            Break: void
          Block: Str
            String: Str
---
fun01 = fun() void
```

**If with one branch continue**

```metall
fun foo() void { for { if true { continue } else { "hello" } } }
```

```types
Fun: fun01
  SimpleType: void
  Block: void
    For: void
      Block: void
        If: void
          Bool: Bool
          Block: void
            Continue: void
          Block: Str
            String: Str
---
fun01 = fun() void
```

**If with both branches return**

```metall
fun foo() Int { if true { return 1 } else { return 2 } }
```

```types
Fun: fun01
  SimpleType: Int
  Block: void
    If: void
      Bool: Bool
      Block: void
        Return: void
          Int: Int
      Block: void
        Return: void
          Int: Int
---
fun01 = fun() Int
```

**Nested if with all branches return**

```metall
fun foo() Int { if true { if false { return 1 } else { return 2 } } else { return 3 } }
```

```types
Fun: fun01
  SimpleType: Int
  Block: void
    If: void
      Bool: Bool
      Block: void
        If: void
          Bool: Bool
          Block: void
            Return: void
              Int: Int
          Block: void
            Return: void
              Int: Int
      Block: void
        Return: void
          Int: Int
---
fun01 = fun() Int
```

**Nested return breaks outer if control flow**

```metall
fun foo(a Int) Int { if true { if a == 0 { return 1 } else { return 2 } } else { "hello" } 321 }
```

```types
Fun: fun01
  FunParam: Int
    SimpleType: Int
  SimpleType: Int
  Block: Int
    If: void
      Bool: Bool
      Block: void
        If: void
          Binary: Bool
            Ident: Int
            Int: Int
          Block: void
            Return: void
              Int: Int
          Block: void
            Return: void
              Int: Int
      Block: Str
        String: Str
    Int: Int
---
fun01 = fun(Int) Int
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

**Mut ref of mut field**

```metall
{ struct Foo { mut one Int } mut x = Foo(42) let y = &mut x.one y }
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
struct01 = Foo { mut one Int }
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

**Reassign mut field of mut ref type**

```metall
{ struct Foo { mut one &mut Int } mut x = 1 mut y = 2 mut z = Foo(&mut x) z.one = &mut y z.one.* = 99 }
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
struct01 = Foo { mut one &mut Int }
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
fun01 = fun() void
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
fun01 = fun(Int) Int
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
fun01 = fun(Int) Int
```

## Allocators

**Allocator var**

```metall
let @myalloc = Arena()
```

```types
AllocatorVar: void
```

```bindings
AllocatorVar: scope01
---
scope01:
  @myalloc: Arena
```

**Heap alloc struct**

```metall
{ let @myalloc = Arena() struct Foo{one Str} let x = @myalloc.new<Foo>(Foo("hello")) x }
```

```types
Block: &struct01
  AllocatorVar: void
  Struct: struct01
    StructField: ?
      SimpleType: ?
  Var: void
    Call: &struct01
      FieldAccess: fun01
        Ident: Arena
        SimpleType: struct01
      TypeConstruction: struct01
        Ident: struct01
        String: Str
  Ident: &struct01
---
struct01 = Foo { one Str }
fun01    = fun(Arena, struct01) &struct01
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
  Call: void
    Ident: fun01
    Ident: Arena
---
fun01 = fun(Arena) void
```

**Heap alloc mut struct**

```metall
{ let @a = Arena() struct Bar{one Str} @a.new_mut<Bar>(Bar("hello")) }
```

```types
Block: &mut struct01
  AllocatorVar: void
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
{ let @a = Arena() @a.slice_uninit_mut<Int>(5) }
```

```types
Block: []mut Int
  AllocatorVar: void
  Call: []mut Int
    FieldAccess: fun01
      Ident: Arena
      SimpleType: Int
    Int: Int
---
fun01 = fun(Arena, Int) []mut Int
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
  Var: void
    TypeConstruction: struct02
      Ident: struct02
      Ident: Arena
  Var: void
    Call: &struct01
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
fun01    = fun(Arena, struct01) &struct01
```

**Make uninit immutable slice**

```metall
{ let @myalloc = Arena() @myalloc.slice_uninit<Int>(5) }
```

```types
Block: []Int
  AllocatorVar: void
  Call: []Int
    FieldAccess: fun01
      Ident: Arena
      SimpleType: Int
    Int: Int
---
fun01 = fun(Arena, Int) []Int
```

**Make slice with default**

```metall
{ let @a = Arena() @a.slice<Int>(5, 42) }
```

```types
Block: []Int
  AllocatorVar: void
  Call: []Int
    FieldAccess: fun01
      Ident: Arena
      SimpleType: Int
    Int: Int
    Int: Int
---
fun01 = fun(Arena, Int, Int) []Int
```

**Make slice**

```metall
{ let @myalloc = Arena() @myalloc.slice_uninit<Int>(5) }
```

```types
Block: []Int
  AllocatorVar: void
  Call: []Int
    FieldAccess: fun01
      Ident: Arena
      SimpleType: Int
    Int: Int
---
fun01 = fun(Arena, Int) []Int
```

**Make slice default**

```metall
{ let @myalloc = Arena() @myalloc.slice<Int>(5, 42) }
```

```types
Block: []Int
  AllocatorVar: void
  Call: []Int
    FieldAccess: fun01
      Ident: Arena
      SimpleType: Int
    Int: Int
    Int: Int
---
fun01 = fun(Arena, Int, Int) []Int
```

**Make uninit Int slice**

```metall
{ let @a = Arena() let x = @a.slice_uninit<Int>(5) }
```

```types
Block: void
  AllocatorVar: void
  Var: void
    Call: []Int
      FieldAccess: fun01
        Ident: Arena
        SimpleType: Int
      Int: Int
---
fun01 = fun(Arena, Int) []Int
```

**Make uninit safe struct slice**

```metall
{ struct Foo{one Int two Int} let @a = Arena() let x = @a.slice_uninit<Foo>(3) }
```

```types
Block: void
  Struct: struct01
    StructField: ?
      SimpleType: ?
    StructField: ?
      SimpleType: ?
  AllocatorVar: void
  Var: void
    Call: []struct01
      FieldAccess: fun01
        Ident: Arena
        SimpleType: struct01
      Int: Int
---
struct01 = Foo { one Int, two Int }
fun01    = fun(Arena, Int) []struct01
```

**Make slice Bool with default**

```metall
{ let @a = Arena() let x = @a.slice<Bool>(5, false) }
```

```types
Block: void
  AllocatorVar: void
  Var: void
    Call: []Bool
      FieldAccess: fun01
        Ident: Arena
        SimpleType: Bool
      Int: Int
      Bool: Bool
---
fun01 = fun(Arena, Int, Bool) []Bool
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
fun01 = fun([5]Int) void
```

**Array type ids are stable**

```metall
fun foo(a [5]Int, b [5]Int, c [6]Int) void { [1, 2, 3, 4, 5]}
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
  Block: [5]Int
    ArrayLiteral: [5]Int
      Int: Int
      Int: Int
      Int: Int
      Int: Int
      Int: Int
---
fun01 = fun([5]Int, [5]Int, [6]Int) void
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
    Range: void
      Int: Int
      Int: Int
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
    Range: void
      Int: Int
      Int: Int
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
    Range: void
      Int: Int
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
    Range: void
      Int: Int
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
{ let @myalloc = Arena() let x = @myalloc.slice_uninit<Int>(3) x[1] }
```

```types
Block: Int
  AllocatorVar: void
  Var: void
    Call: []Int
      FieldAccess: fun01
        Ident: Arena
        SimpleType: Int
      Int: Int
  Index: Int
    Ident: []Int
    Int: Int
---
fun01 = fun(Arena, Int) []Int
```

**Slice index write**

```metall
{ let @myalloc = Arena() let x = @myalloc.slice_uninit_mut<Int>(3) x[1] = 5 }
```

```types
Block: void
  AllocatorVar: void
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
fun01 = fun(Arena, Int) []mut Int
```

**Slice len**

```metall
{ let @myalloc = Arena() let x = @myalloc.slice_uninit<Int>(3) x.len }
```

```types
Block: Int
  AllocatorVar: void
  Var: void
    Call: []Int
      FieldAccess: fun01
        Ident: Arena
        SimpleType: Int
      Int: Int
  FieldAccess: Int
    Ident: []Int
---
fun01 = fun(Arena, Int) []Int
```

**Slice as fun param**

```metall
{ let @a = Arena() fun foo(s []Int) Int { s[0] } let x = @a.slice_uninit<Int>(3) foo(x) }
```

```types
Block: Int
  AllocatorVar: void
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
    Call: []Int
      FieldAccess: fun02
        Ident: Arena
        SimpleType: Int
      Int: Int
  Call: Int
    Ident: fun01
    Ident: []Int
---
fun01 = fun([]Int) Int
fun02 = fun(Arena, Int) []Int
```

**Slice as fun param and return**

```metall
{ let @a = Arena() fun foo(s []Int) []Int { s } let x = @a.slice_uninit<Int>(3) foo(x) }
```

```types
Block: []Int
  AllocatorVar: void
  Fun: fun01
    FunParam: []Int
      SliceType: []Int
        SimpleType: Int
    SliceType: []Int
      SimpleType: Int
    Block: []Int
      Ident: []Int
  Var: void
    Call: []Int
      FieldAccess: fun02
        Ident: Arena
        SimpleType: Int
      Int: Int
  Call: []Int
    Ident: fun01
    Ident: []Int
---
fun01 = fun([]Int) []Int
fun02 = fun(Arena, Int) []Int
```

**Struct with slice field**

```metall
{ let @a = Arena() struct Foo { one []Int } let s = @a.slice_uninit<Int>(3) let x = Foo(s) x.one[0] }
```

```types
Block: Int
  AllocatorVar: void
  Struct: struct01
    StructField: ?
      SliceType: ?
        SimpleType: ?
  Var: void
    Call: []Int
      FieldAccess: fun01
        Ident: Arena
        SimpleType: Int
      Int: Int
  Var: void
    TypeConstruction: struct01
      Ident: struct01
      Ident: []Int
  Index: Int
    FieldAccess: []Int
      Ident: struct01
    Int: Int
---
struct01 = Foo { one []Int }
fun01    = fun(Arena, Int) []Int
```

**Ref to slice**

```metall
{ let @a = Arena() let x = @a.slice_uninit<Int>(3) &x }
```

```types
Block: &[]Int
  AllocatorVar: void
  Var: void
    Call: []Int
      FieldAccess: fun01
        Ident: Arena
        SimpleType: Int
      Int: Int
  Ref: &[]Int
    Ident: []Int
---
fun01 = fun(Arena, Int) []Int
```

**Slice index through ref**

```metall
{ let @a = Arena() let x = @a.slice_uninit<Int>(3) let y = &x y[0] }
```

```types
Block: Int
  AllocatorVar: void
  Var: void
    Call: []Int
      FieldAccess: fun01
        Ident: Arena
        SimpleType: Int
      Int: Int
  Var: void
    Ref: &[]Int
      Ident: []Int
  Index: Int
    Ident: &[]Int
    Int: Int
---
fun01 = fun(Arena, Int) []Int
```

**Slice len through ref**

```metall
{ let @a = Arena() let x = @a.slice_uninit<Int>(3) let y = &x y.len }
```

```types
Block: Int
  AllocatorVar: void
  Var: void
    Call: []Int
      FieldAccess: fun01
        Ident: Arena
        SimpleType: Int
      Int: Int
  Var: void
    Ref: &[]Int
      Ident: []Int
  FieldAccess: Int
    Ident: &[]Int
---
fun01 = fun(Arena, Int) []Int
```

**Mut ref slice index write**

```metall
{ let @a = Arena() mut x = @a.slice_uninit_mut<Int>(3) let y = &mut x y[0] = 42 }
```

```types
Block: void
  AllocatorVar: void
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
fun01 = fun(Arena, Int) []mut Int
```

**Make mut slice**

```metall
{ let @a = Arena() @a.slice_uninit_mut<Int>(5) }
```

```types
Block: []mut Int
  AllocatorVar: void
  Call: []mut Int
    FieldAccess: fun01
      Ident: Arena
      SimpleType: Int
    Int: Int
---
fun01 = fun(Arena, Int) []mut Int
```

**Mut slice assignable to immutable**

```metall
{ let @a = Arena() fun foo(s []Int) Int { s[0] } let x = @a.slice_uninit_mut<Int>(3) foo(x) }
```

```types
Block: Int
  AllocatorVar: void
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
fun02 = fun(Arena, Int) []mut Int
```

**Mut slice index write no mut binding**

```metall
{ let @a = Arena() let x = @a.slice_uninit_mut<Int>(3) x[0] = 5 }
```

```types
Block: void
  AllocatorVar: void
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
fun01 = fun(Arena, Int) []mut Int
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
    Range: void
      Int: Int
      Int: Int
```

**Subslice mut slice**

```metall
{ let @a = Arena() let x = @a.slice_uninit_mut<Int>(5) x[1..3] }
```

```types
Block: []mut Int
  AllocatorVar: void
  Var: void
    Call: []mut Int
      FieldAccess: fun01
        Ident: Arena
        SimpleType: Int
      Int: Int
  SubSlice: []mut Int
    Ident: []mut Int
    Range: void
      Int: Int
      Int: Int
---
fun01 = fun(Arena, Int) []mut Int
```

**Subslice mut slice through mut ref**

```metall
{ let @a = Arena() mut x = @a.slice_uninit_mut<Int>(5) let y = &mut x y[1..3] }
```

```types
Block: []mut Int
  AllocatorVar: void
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
    Range: void
      Int: Int
      Int: Int
---
fun01 = fun(Arena, Int) []mut Int
```

**Subslice mut slice through immutable ref**

```metall
{ let @a = Arena() let x = @a.slice_uninit_mut<Int>(5) let y = &x y[1..3] }
```

```types
Block: []Int
  AllocatorVar: void
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
    Range: void
      Int: Int
      Int: Int
---
fun01 = fun(Arena, Int) []mut Int
```

**Subslice slice**

```metall
{ let @a = Arena() let x = @a.slice_uninit<Int>(5) x[1..3] }
```

```types
Block: []Int
  AllocatorVar: void
  Var: void
    Call: []Int
      FieldAccess: fun01
        Ident: Arena
        SimpleType: Int
      Int: Int
  SubSlice: []Int
    Ident: []Int
    Range: void
      Int: Int
      Int: Int
---
fun01 = fun(Arena, Int) []Int
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
    Range: void
      Int: Int
      Int: Int
```

**Empty slice in make**

```metall
{ let @a = Arena() let x = @a.slice<[]Int>(2, []) }
```

```types
Block: void
  AllocatorVar: void
  Var: void
    Call: [][]Int
      FieldAccess: fun01
        Ident: Arena
        SliceType: []Int
          SimpleType: Int
      Int: Int
      EmptySlice: []Int
---
fun01 = fun(Arena, Int, []Int) [][]Int
```

**Empty slice in assignment**

```metall
{ let @a = Arena() mut x = @a.slice_uninit<Int>(3) x = [] }
```

```types
Block: void
  AllocatorVar: void
  Var: void
    Call: []Int
      FieldAccess: fun01
        Ident: Arena
        SimpleType: Int
      Int: Int
  Assign: void
    Ident: []Int
    EmptySlice: []Int
---
fun01 = fun(Arena, Int) []Int
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
  Var: void
    Call: [][]Int
      FieldAccess: fun01
        Ident: Arena
        SliceType: []Int
          SimpleType: Int
      Int: Int
      EmptySlice: []Int
---
fun01 = fun(Arena, Int, []Int) [][]Int
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
fun01 = fun([3][4]Int) void
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
fun01 = fun(U8) U8
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
    Range: void
      Int: Int
      Int: Int
    Block: void
```

**For in range inclusive**

```metall
{ for x in 0..=9 { } }
```

```types
Block: void
  For: void
    Range: void
      Int: Int
      Int: Int
    Block: void
```

**For in range with break**

```metall
{ for x in 0..10 { if x == 5 { break } } }
```

```types
Block: void
  For: void
    Range: void
      Int: Int
      Int: Int
    Block: void
      If: void
        Binary: Bool
          Ident: Int
          Int: Int
        Block: void
          Break: void
```

**For in range binding is Int**

```metall
{ for x in 0..10 { let y = x + 1 } }
```

```types
Block: void
  For: void
    Range: void
      Int: Int
      Int: Int
    Block: void
      Var: void
        Binary: Int
          Ident: Int
          Int: Int
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
    Range: void
      Int: Int
      Int: Int
    Block: void
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
fun01    = fun(struct01) Int
```

**Method call with args**

```metall
{ struct Foo { mut one Int } fun Foo.add(f Foo, n Int) Int { f.one + n } let x = Foo(10) x.add(5) }
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
struct01 = Foo { mut one Int }
fun01    = fun(struct01, Int) Int
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
fun01    = fun(struct01) Int
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
fun01    = fun(struct01) Int
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
fun01    = fun(struct01, Int) Int
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
fun01 = fun(Int) Int
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
fun01 = fun(Int) Int
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
fun01    = fun() Int
struct01 = Foo { one Int }
fun02    = fun(struct01) Int
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
fun01    = fun() Int
struct01 = Foo { one Int }
fun02    = fun(struct01) Int
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
        SimpleType: Int
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
        SimpleType: struct04
          SimpleType: Int
        SimpleType: Str
      TypeConstruction: struct04
        Ident: struct04
          SimpleType: Int
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
        SimpleType: Int
      Int: Int
  Var: void
    TypeConstruction: struct02
      Ident: struct02
        SimpleType: Int
      Int: Int
  Var: void
    TypeConstruction: struct03
      Ident: struct03
        SimpleType: Str
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
    {
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
        SimpleType: Int
      Int: Int
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
          SimpleType: Int
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
        SimpleType: Int
      TypeConstruction: struct05
        Ident: struct05
          SimpleType: Int
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
    id<Int>(42)
    id<Str>("hello")
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
  Call: Int
    Ident: fun02
      SimpleType: Int
    Int: Int
  Call: Str
    Ident: fun03
      SimpleType: Str
    String: Str
  Var: void
    Call: struct02
      Ident: fun04
        SimpleType: struct02
          SimpleType: Int
      TypeConstruction: struct02
        Ident: struct02
          SimpleType: Int
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
      SimpleType: Int
      SimpleType: Str
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
    id<Int>(1)
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
  Call: Int
    Ident: fun02
      SimpleType: Int
    Int: Int
  Call: Int
    Ident: fun02
      SimpleType: Int
    Int: Int
---
fun01 = fun(T) T
fun02 = fun(Int) Int
```

**Method on generic struct**

```metall
{
    struct Foo<T> { one T }
    fun Foo.bar<T>(f Foo<T>, a T, b Bool) T { if b { return f.one } a }
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
        Block: void
          Return: void
            FieldAccess: T
              Ident: struct02
      Ident: T
  Var: void
    TypeConstruction: struct03
      Ident: struct03
        SimpleType: Int
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
      SimpleType: Str
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
    fun Foo.bar<T, U>(f Foo<T>, a U) U { a }
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
        SimpleType: Int
      Int: Int
  Call: Str
    FieldAccess: fun02
      Ident: struct03
      SimpleType: Str
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
      SimpleType: Int
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
        SimpleType: Int
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
          SimpleType: Str
        String: Str
    Ident: Int
---
fun01 = fun(T) T
fun02 = fun(Int) Int
fun03 = fun(T) Int
fun04 = fun(Str) Int
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
        SimpleType: Int
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
      SimpleType: Int
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
      SimpleType: Int
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
    g(Foo(1), "hello")
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
      SimpleType: Str
  Call: Str
    Ident: fun02
    TypeConstruction: struct01
      Ident: struct01
      Int: Int
    String: Str
  Var: void
    Ident: fun03
      SimpleType: Int
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
      SimpleType: Int
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
    fun Box.get<V>(b &Box<V>) V { b.value }
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
        SimpleType: Int
      Int: Int
  Call: Int
    Ident: fun04
      SimpleType: Int
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
    fun Bag.len<V>(b &Bag<V>) Int { b.items.len }
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
  Var: void
    Call: []Str
      FieldAccess: fun04
        Ident: Arena
        SimpleType: Str
      Int: Int
      String: Str
  Var: void
    TypeConstruction: struct04
      Ident: struct04
        SimpleType: Str
      Ident: []Str
  Call: Int
    Ident: fun05
      SimpleType: Str
    Ref: &struct04
      Ident: struct04
---
struct01 = Bag { items []V }
struct02 = Bag<V> { items []V }
fun01    = fun(&struct02) Int
struct03 = Bag<V> { items []V }
fun02    = fun(&struct03) Int
fun03    = fun(&struct03) Int
fun04    = fun(Arena, Int, Str) []Str
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
      SimpleType: Str
    String: Str
    Ident: fun03
---
fun02 = fun(T) Int
fun01 = fun(T, fun02) Int
fun03 = fun(Str) Int
fun04 = fun(Str, fun03) Int
```

## Shapes

**Shape field access**

```metall
{
    shape HasPair { one Str two Int }
    struct Pair { one Str two Int }
    fun first<T HasPair>(t T) Str { t.one }
    first<Pair>(Pair("hello", 42))
}
```

```types
Block: Str
  Shape: shape01
    StructField: Str
      SimpleType: Str
    StructField: Int
      SimpleType: Int
  Struct: struct01
    StructField: ?
      SimpleType: ?
    StructField: ?
      SimpleType: ?
  Fun: fun01
    TypeParam: T
      SimpleType: shape01
    FunParam: T
      SimpleType: T
    SimpleType: Str
    Block: Str
      FieldAccess: Str
        Ident: T
  Call: Str
    Ident: fun02
      SimpleType: struct01
    TypeConstruction: struct01
      Ident: struct01
      String: Str
      Int: Int
---
shape01  = HasPair { one Str, two Int }
struct01 = Pair { one Str, two Int }
fun01    = fun(T) Str
fun02    = fun(struct01) Str
```

**Shape satisfied with extra fields and different order**

```metall
{
    shape HasPair { one Str two Int }
    struct Big { extra Bool two Int name Str one Str }
    fun first<T HasPair>(t T) Str { t.one }
    first<Big>(Big(true, 42, "world", "hello"))
}
```

```types
Block: Str
  Shape: shape01
    StructField: Str
      SimpleType: Str
    StructField: Int
      SimpleType: Int
  Struct: struct01
    StructField: ?
      SimpleType: ?
    StructField: ?
      SimpleType: ?
    StructField: ?
      SimpleType: ?
    StructField: ?
      SimpleType: ?
  Fun: fun01
    TypeParam: T
      SimpleType: shape01
    FunParam: T
      SimpleType: T
    SimpleType: Str
    Block: Str
      FieldAccess: Str
        Ident: T
  Call: Str
    Ident: fun02
      SimpleType: struct01
    TypeConstruction: struct01
      Ident: struct01
      Bool: Bool
      Int: Int
      String: Str
      String: Str
---
shape01  = HasPair { one Str, two Int }
struct01 = Big { extra Bool, two Int, name Str, one Str }
fun01    = fun(T) Str
fun02    = fun(struct01) Str
```

**Shape forward declared after struct**

```metall
{
    struct Pair { one Str two Int }
    shape HasPair { one Str two Int }
    fun first<T HasPair>(t T) Str { t.one }
    first<Pair>(Pair("hello", 42))
}
```

```types
Block: Str
  Struct: struct01
    StructField: ?
      SimpleType: ?
    StructField: ?
      SimpleType: ?
  Shape: shape01
    StructField: Str
      SimpleType: Str
    StructField: Int
      SimpleType: Int
  Fun: fun01
    TypeParam: T
      SimpleType: shape01
    FunParam: T
      SimpleType: T
    SimpleType: Str
    Block: Str
      FieldAccess: Str
        Ident: T
  Call: Str
    Ident: fun02
      SimpleType: struct01
    TypeConstruction: struct01
      Ident: struct01
      String: Str
      Int: Int
---
struct01 = Pair { one Str, two Int }
shape01  = HasPair { one Str, two Int }
fun01    = fun(T) Str
fun02    = fun(struct01) Str
```

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
      SimpleType: struct01
    TypeConstruction: struct01
      Ident: struct01
      String: Str
---
shape01  = Showable {  }
struct01 = Guitar { name Str }
fun01    = fun(struct01) Str
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
      SimpleType: struct01
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
      SimpleType: struct01
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
      SimpleType: struct01
      SimpleType: struct02
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
fun01    = fun(struct01) Str
fun02    = fun(struct02) Int
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
      SimpleType: struct01
    TypeConstruction: struct01
      Ident: struct01
      Int: Int
---
struct01 = Foo { x Int }
shape01  = Clonable {  }
fun01    = fun(struct01) struct01
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
      SimpleType: struct01
    TypeConstruction: struct01
      Ident: struct01
      Int: Int
    TypeConstruction: struct01
      Ident: struct01
      Int: Int
---
shape01  = Eq {  }
struct01 = Num { x Int }
fun01    = fun(struct01, struct01) Bool
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
        SimpleType: struct01
      TypeConstruction: struct01
        Ident: struct01
  Call: Str
    Ident: fun06
      SimpleType: struct02
    TypeConstruction: struct02
      Ident: struct02
---
shape01  = Showable {  }
struct01 = A {  }
struct02 = B {  }
fun01    = fun(struct01) Str
fun02    = fun(struct02) Str
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
      SimpleType: struct01
    TypeConstruction: struct01
      Ident: struct01
---
shape01  = S {  }
struct01 = X {  }
fun01    = fun(struct01) Int
fun02    = fun(struct01) Str
fun03    = fun(T) Str
fun04    = fun(T) Int
fun05    = fun(T) Str
fun06    = fun(struct01) Str
```

**Shape field and method combined**

```metall
{
    shape Named {
        name Str
        fun Named.greet(self Named) Str
    }
    struct Person { name Str age Int }
    fun Person.greet(p Person) Str { p.name }
    fun intro<T Named>(t T) Str {
        let n = t.name
        t.greet()
    }
    intro<Person>(Person("Alice", 30))
}
```

```types
Block: Str
  Shape: shape01
    StructField: Str
      SimpleType: Str
    FunDecl: ?
      FunParam: shape01
        SimpleType: shape01
      SimpleType: Str
  Struct: struct01
    StructField: ?
      SimpleType: ?
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
      Var: void
        FieldAccess: Str
          Ident: T
      Call: Str
        FieldAccess: fun03
          Ident: T
  Call: Str
    Ident: fun04
      SimpleType: struct01
    TypeConstruction: struct01
      Ident: struct01
      String: Str
      Int: Int
---
shape01  = Named { name Str }
struct01 = Person { name Str, age Int }
fun01    = fun(struct01) Str
fun02    = fun(T) Str
fun03    = fun(T) Str
fun04    = fun(struct01) Str
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
      SimpleType: Int
    Int: Int
---
shape01 = Displayable {  }
fun01   = fun(Int) Int
fun02   = fun(T) Int
fun03   = fun(T) Int
fun04   = fun(Int) Int
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
      SimpleType: Int
    Int: Int
---
shape01 = Displayable {  }
fun01   = fun(Int) Int
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
      SimpleType: Int
    Int: Int
    Int: Int
    Int: Int
---
shape01 = HasFmt {  }
shape02 = HasEqFmt {  }
fun01   = fun(Int, Int) Int
fun02   = fun(Int, Int) Bool
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
    fun Wrapper.display<T Displayable>(w Wrapper<T>) Int { w.value.display() }
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
    TypeParam: T
      SimpleType: shape01
    FunParam: struct02
      SimpleType: struct02
        SimpleType: T
    SimpleType: Int
    Block: Int
      Call: Int
        FieldAccess: fun02
          FieldAccess: T
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
      SimpleType: struct03
        SimpleType: Int
    TypeConstruction: struct03
      Ident: struct03
        SimpleType: Int
      Int: Int
---
shape01  = Displayable {  }
struct01 = Wrapper { value T }
struct02 = Wrapper<T> { value T }
fun01    = fun(struct02) Int
fun02    = fun(T) Int
fun03    = fun(Int) Int
fun04    = fun(T) Int
fun05    = fun(T) Int
struct03 = Wrapper<Int> { value Int }
fun06    = fun(struct03) Int
```

**Generic shape**

```metall
{
    shape Iter<T> {
        fun Iter.next(it Iter) ?T
    }
    struct Range { cur Int max Int }
    fun Range.next(r Range) ?Int {
        if r.cur >= r.max { return None() }
        Option(r.cur)
    }
    fun sum<T Iter<Int>>(it T) Int { 0 }
    sum<Range>(Range(0, 10))
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
      SimpleType: Int
    Block: union01
      If: void
        Binary: Bool
          FieldAccess: Int
            Ident: struct01
          FieldAccess: Int
            Ident: struct01
        Block: void
          Return: void
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
      SimpleType: struct01
    TypeConstruction: struct01
      Ident: struct01
      Int: Int
      Int: Int
---
shape01  = Iter {  }
struct01 = Range { cur Int, max Int }
struct02 = None {  }
union01  = Option<Int> = Int | struct02
fun01    = fun(struct01) union01
fun02    = fun(T) Int
shape02  = Iter {  }
fun03    = fun(struct01) Int
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
        SimpleType: Str
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
        SimpleType: Str
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
fun01   = fun(Str) Str
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
      SimpleType: Int
    Block: union01
      TypeConstruction: union01
        Ident: union01
        Int: Int
  Call: union01
    Ident: fun01
---
union01 = MyResult<Int> = Int | Str
union02 = MyResult = T | Str
fun01   = fun() union01
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
      SimpleType: Int
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
fun01   = fun(Bool) union01
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
        SimpleType: Int
    SimpleType: union01
      SimpleType: Int
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
        SimpleType: Int
      Int: Int
---
union01 = MyResult<Int> = Int | Str
union02 = MyResult = T | Str
fun01   = fun(union01) union01
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
---
fun01 = fun(Int, Int) Int
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
fun01 = fun(Int, Int) Int
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
---
fun01 = fun(Int, Int, Int) Int
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
---
fun01 = fun(Int, Int) Int
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
---
struct01 = Foo { x Int }
fun01    = fun(struct01, Int) Int
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
      SimpleType: struct01
    TypeConstruction: struct01
      Ident: struct01
      Int: Int
---
shape01  = HasFmt {  }
struct01 = Foo { x Int }
fun01    = fun(struct01) Str
fun02    = fun(T, Str) Str
fun03    = fun(T) Str
fun04    = fun(struct01, Str) Str
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

**Duplicate generic method called (module)**

```metall module
struct Foo<T> { one T }
fun Foo.bar<T>(f &Foo<T>) T { f.one }
fun Foo.bar<T>(f &Foo<T>) T { f.one }
fun main() void { let f = Foo<Int>(42) let r = &f r.bar() }
```

```error
test.met:3:5: symbol already defined: test.Foo.bar
    fun Foo.bar<T>(f &Foo<T>) T { f.one }
    fun Foo.bar<T>(f &Foo<T>) T { f.one }
        ^^^^^^^
    fun main() void { let f = Foo<Int>(42) let r = &f r.bar() }
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

**Main must return void (module)**

```metall module
fun main() Int { 123 }
```

```error
test.met:1:12: main function cannot return a value
    fun main() Int { 123 }
               ^^^
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
{ let @a = Arena() let d = @a.slice_uninit<U8>(1) Str(d) }
```

```error
test.met:1:51: Str cannot be constructed directly; use Str.from_utf8_lossy() instead
    { let @a = Arena() let d = @a.slice_uninit<U8>(1) Str(d) }
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

**&mut of immutable field**

```metall
{ struct Foo { one Int } mut x = Foo(42) let y = &mut x.one }
```

```error
test.met:1:50: cannot take mutable reference to immutable value
    { struct Foo { one Int } mut x = Foo(42) let y = &mut x.one }
                                                     ^^^^^^^^^^
```

**Field write on let binding**

```metall
{ struct Foo{mut one Str} let x = Foo("hello") x.one = "bye" }
```

```error
test.met:1:48: cannot assign to field of immutable value
    { struct Foo{mut one Str} let x = Foo("hello") x.one = "bye" }
                                                   ^^^^^
```

**Nested field write on let binding**

```metall
{ struct Foo{mut one Int} struct Bar{mut one Foo} let x = Bar(Foo(1)) x.one.one = 2 }
```

```error
test.met:1:71: cannot assign to field of immutable value
    { struct Foo{mut one Int} struct Bar{mut one Foo} let x = Bar(Foo(1)) x.one.one = 2 }
                                                                          ^^^^^^^^^
```

**Nested field write through non-mut field**

```metall
{ struct Foo{mut one Int} struct Bar{one Foo} mut x = Bar(Foo(1)) x.one.one = 2 }
```

```error
test.met:1:67: cannot assign to field of immutable value
    { struct Foo{mut one Int} struct Bar{one Foo} mut x = Bar(Foo(1)) x.one.one = 2 }
                                                                      ^^^^^^^^^
```

**Field write through immutable ref**

```metall
{ struct Foo{mut one Str} let x = Foo("hello") let y = &x y.one = "X" }
```

```error
test.met:1:59: cannot assign to field of immutable value
    { struct Foo{mut one Str} let x = Foo("hello") let y = &x y.one = "X" }
                                                              ^^^^^
```

**Field write through immutable ref param**

```metall
{ struct Foo{mut one Str} fun foo(a &Foo) void { a.one = "X" } }
```

```error
test.met:1:50: cannot assign to field of immutable value
    { struct Foo{mut one Str} fun foo(a &Foo) void { a.one = "X" } }
                                                     ^^^^^
```

**Field write on non-mut field**

```metall
{ struct Foo { one Str } mut x = Foo("hi") x.one = "bye" }
```

```error
test.met:1:44: cannot assign to immutable field: one
    { struct Foo { one Str } mut x = Foo("hi") x.one = "bye" }
                                               ^^^^^
```

**Pass &ref where &mut field expected**

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

**Reassign non-mut &mut field**

```metall
{ struct Foo { one &mut Int } mut x = 1 mut y = 2 mut z = Foo(&mut x) z.one = &y }
```

```error
test.met:1:71: cannot assign to immutable field: one
    { struct Foo { one &mut Int } mut x = 1 mut y = 2 mut z = Foo(&mut x) z.one = &y }
                                                                          ^^^^^
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
test.met:1:1: type mismatch: binary operation '==' expects an integer or Bool, got Str
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
{ let @a = Arena() let x = @a.slice_uninit<Int>(3) x.foo }
```

```error
test.met:1:54: unknown field on slice: foo
    { let @a = Arena() let x = @a.slice_uninit<Int>(3) x.foo }
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
{ let @a = Arena() @a.slice_uninit<Int>("hello") }
```

```error
test.met:1:41: type mismatch at argument 1: expected Int, got Str
    { let @a = Arena() @a.slice_uninit<Int>("hello") }
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

**Make uninit Bool**

```metall
{ let @a = Arena() @a.slice_uninit<Bool>(3) }
```

```error
test.met:1:36: Bool is not safe to leave uninitialized, use slice with a default value
    { let @a = Arena() @a.slice_uninit<Bool>(3) }
                                       ^^^^
```

**Make uninit Str**

```metall
{ let @a = Arena() @a.slice_uninit<Str>(3) }
```

```error
test.met:1:36: Str is not safe to leave uninitialized, use slice with a default value
    { let @a = Arena() @a.slice_uninit<Str>(3) }
                                       ^^^
```

**Make uninit ref**

```metall
{ struct Foo{one Int} let @a = Arena() @a.slice_uninit<&Foo>(3) }
```

```error
test.met:1:56: &Foo is not safe to leave uninitialized, use slice with a default value
    { struct Foo{one Int} let @a = Arena() @a.slice_uninit<&Foo>(3) }
                                                           ^^^^
```

**Make uninit struct with ref field**

```metall
{ struct Foo{one &Int} let @a = Arena() @a.slice_uninit<Foo>(3) }
```

```error
test.met:1:57: Foo is not safe to leave uninitialized, use slice with a default value
    { struct Foo{one &Int} let @a = Arena() @a.slice_uninit<Foo>(3) }
                                                            ^^^
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

**Write to immutable slice element**

```metall
{ let @a = Arena() let x = @a.slice_uninit<Int>(3) x[0] = 5 }
```

```error
test.met:1:52: cannot assign to element of immutable array or slice
    { let @a = Arena() let x = @a.slice_uninit<Int>(3) x[0] = 5 }
                                                       ^^^^
```

**Write through mut ref to immutable slice**

```metall
{ let @a = Arena() mut x = @a.slice_uninit<Int>(3) let y = &mut x y[0] = 5 }
```

```error
test.met:1:67: cannot assign to element of immutable array or slice
    { let @a = Arena() mut x = @a.slice_uninit<Int>(3) let y = &mut x y[0] = 5 }
                                                                      ^^^^
```

**Immutable slice not assignable to mut slice param**

```metall
{ let @a = Arena() fun foo(s []mut Int) void {} let x = @a.slice_uninit<Int>(3) foo(x) }
```

```error
test.met:1:85: type mismatch at argument 1: expected []mut Int, got []Int
    { let @a = Arena() fun foo(s []mut Int) void {} let x = @a.slice_uninit<Int>(3) foo(x) }
                                                                                        ^
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
{ struct Foo<T> { one T } fun Foo.bar<T, U>(f Foo<T>, a U) U { a } let x = Foo<Int>(1) x.bar() }
```

```error
test.met:1:90: type argument count mismatch: expected 2, got 1
    { struct Foo<T> { one T } fun Foo.bar<T, U>(f Foo<T>, a U) U { a } let x = Foo<Int>(1) x.bar() }
                                                                                             ^^^
```

**Method on generic struct too many type args**

```metall
{ struct Foo<T> { one T } fun Foo.bar<T>(f Foo<T>) T { f.one } let x = Foo<Int>(1) x.bar<Int, Str>() }
```

```error
test.met:1:86: type argument count mismatch: expected 1, got 3
    { struct Foo<T> { one T } fun Foo.bar<T>(f Foo<T>) T { f.one } let x = Foo<Int>(1) x.bar<Int, Str>() }
                                                                                         ^^^
```

**Method on generic struct wrong first param type**

```metall
{ struct Foo<T> { one T } fun Foo.bar<T>(f Int) T { f } let x = Foo<Int>(1) x.bar() }
```

```error
test.met:1:53: return type mismatch: expected T, got Int
    { struct Foo<T> { one T } fun Foo.bar<T>(f Int) T { f } let x = Foo<Int>(1) x.bar() }
                                                        ^
test.met:1:77: type mismatch at receiver: expected Int, got Foo<Int>
    { struct Foo<T> { one T } fun Foo.bar<T>(f Int) T { f } let x = Foo<Int>(1) x.bar() }
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

**Shape not satisfied missing field**

```metall
{ shape HasPair { one Str two Int } struct Foo { one Str } fun first<T HasPair>(t T) Str { t.one } first<Foo>(Foo("hello")) }
```

```error
test.met:1:100: type Foo does not satisfy shape HasPair: missing field two
    { shape HasPair { one Str two Int } struct Foo { one Str } fun first<T HasPair>(t T) Str { t.one } first<Foo>(Foo("hello")) }
                                                                                                       ^^^^^^^^^^
```

**Shape not satisfied wrong field type**

```metall
{ shape HasPair { one Str two Int } struct Foo { one Str two Str } fun first<T HasPair>(t T) Str { t.one } first<Foo>(Foo("hello", "world")) }
```

```error
test.met:1:108: type Foo does not satisfy shape HasPair: field two has type Str, expected Int
    { shape HasPair { one Str two Int } struct Foo { one Str two Str } fun first<T HasPair>(t T) Str { t.one } first<Foo>(Foo("hello", "world")) }
                                                                                                               ^^^^^^^^^^
```

**Shape not satisfied field not mut**

```metall
{ shape S { mut one Int } struct Foo { one Int } fun foo<T S>(t T) Int { t.one } foo<Foo>(Foo(1)) }
```

```error
test.met:1:82: type Foo does not satisfy shape S: field one must be mut
    { shape S { mut one Int } struct Foo { one Int } fun foo<T S>(t T) Int { t.one } foo<Foo>(Foo(1)) }
                                                                                     ^^^^^^^^
```

**Shape not satisfied ref vs mut ref**

```metall
{ shape S { one &mut Int } struct Foo { one &Int } fun foo<T S>(t T) Int { 1 } let x = 1 foo<Foo>(Foo(&x)) }
```

```error
test.met:1:90: type Foo does not satisfy shape S: field one has type &Int, expected &mut Int
    { shape S { one &mut Int } struct Foo { one &Int } fun foo<T S>(t T) Int { 1 } let x = 1 foo<Foo>(Foo(&x)) }
                                                                                             ^^^^^^^^
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

**Method on unconstrained type param (module)**

```metall module
shape X { fun X.to_str(x X) Str }
struct Value<T X> { value T }
fun Value.to_str<T>(v Value<T>) Str { v.value.to_str() }
```

```error
test.met:3:47: unconstrained type parameter has no fields or methods: T
    struct Value<T X> { value T }
    fun Value.to_str<T>(v Value<T>) Str { v.value.to_str() }
                                                  ^^^^^^
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
test.met:3:5: cannot access field on non-struct type: fun() void
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




