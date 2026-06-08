# Match Tests

## OK

**Match union returns variant type**

```metall
{
    union Foo = Str | Int
    let x = Foo("hello")
    match x {
        case Str: "str"
        case Int: "int"
    }
}
```

```types
Block: Str
  Union: union01
    SimpleType: ?
    SimpleType: ?
  Var: void
    TypeConstruction: union01
      Ident: union01
      String: Str
  Match: Str
    Ident: union01
    SimpleType: Str
    Block: Str
      String: Str
    SimpleType: Int
    Block: Str
      String: Str
---
union01 = Foo = Str | Int
```

**Match union with binding**

```metall
{
    union Foo = Str | Int
    let x = Foo(42)
    match x {
        case Int n: let y = n
        case Str s: let y = s
    }
}
```

```types
Block: void
  Union: union01
    SimpleType: ?
    SimpleType: ?
  Var: void
    TypeConstruction: union01
      Ident: union01
      Int: Int
  Match: void
    Ident: union01
    SimpleType: Int
    Block: void
      Var: void
        Ident: Int
    SimpleType: Str
    Block: void
      Var: void
        Ident: Str
---
union01 = Foo = Str | Int
```

**Match union with else**

```metall
{
    union Foo = Str | Int | Bool
    let x = Foo("hello")
    match x {
        case Str: "found str"
        else: "other"
    }
}
```

```types
Block: Str
  Union: union01
    SimpleType: ?
    SimpleType: ?
    SimpleType: ?
  Var: void
    TypeConstruction: union01
      Ident: union01
      String: Str
  Match: Str
    Ident: union01
    SimpleType: Str
    Block: Str
      String: Str
    Block: Str
      String: Str
---
union01 = Foo = Str | Int | Bool
```

**Match union all arms diverge**

```metall
fun foo() Int {
    union Foo = Str | Int
    let x = Foo(42)
    match x {
        case Str: return 1
        case Int: return 2
    }
}
```

```types
Fun: fun01
  SimpleType: Int
  Block: never
    Union: union01
      SimpleType: ?
      SimpleType: ?
    Var: void
      TypeConstruction: union01
        Ident: union01
        Int: Int
    Match: never
      Ident: union01
      SimpleType: Str
      Block: never
        Return: never
          Int: Int
      SimpleType: Int
      Block: never
        Return: never
          Int: Int
---
fun01   = sync fun() Int
union01 = Foo = Str | Int
```

**Match diverging arm excluded from result type**

```metall
fun foo() Int {
    union Tri = Int | Bool | Str
    let x = Tri(42)
    match x {
        case Str: return 0
        case Int n: n
        case Bool: 99
    }
}
```

```types
Fun: fun01
  SimpleType: Int
  Block: Int
    Union: union01
      SimpleType: ?
      SimpleType: ?
      SimpleType: ?
    Var: void
      TypeConstruction: union01
        Ident: union01
        Int: Int
    Match: Int
      Ident: union01
      SimpleType: Str
      Block: never
        Return: never
          Int: Int
      SimpleType: Int
      Block: Int
        Ident: Int
      SimpleType: Bool
      Block: Int
        Int: Int
---
fun01   = sync fun() Int
union01 = Tri = Int | Bool | Str
```

**Match union with generic type**

```metall
{
    union Maybe<T> = T | Bool
    let x = Maybe<Int>(42)
    match x {
        case Int n: n
        case Bool: 0
    }
}
```

```types
Block: Int
  Union: union01
    TypeParam: ?
    SimpleType: ?
    SimpleType: ?
  Var: void
    TypeConstruction: union02
      Ident: union02
      Int: Int
  Match: Int
    Ident: union02
    SimpleType: Int
    Block: Int
      Ident: Int
    SimpleType: Bool
    Block: Int
      Int: Int
---
union01 = Maybe = T | Bool
union02 = Maybe<Int> = Int | Bool
```

**Match with guard**

```metall
{
    union Foo = Int | Str
    let x = Foo(42)
    match x {
        case Int n if n > 10: "big"
        case Int: "small"
        case Str: "str"
    }
}
```

```types
Block: Str
  Union: union01
    SimpleType: ?
    SimpleType: ?
  Var: void
    TypeConstruction: union01
      Ident: union01
      Int: Int
  Match: Str
    Ident: union01
    SimpleType: Int
    Binary: Bool
      Ident: Int
      Int: Int
    Block: Str
      String: Str
    SimpleType: Int
    Block: Str
      String: Str
    SimpleType: Str
    Block: Str
      String: Str
---
union01 = Foo = Int | Str
```

**Match with guard and else**

```metall
{
    union Tri = Int | Bool | Str
    let x = Tri(42)
    match x {
        case Int n if n > 0: "positive"
        case Int: "non-positive"
        else: "other"
    }
}
```

```types
Block: Str
  Union: union01
    SimpleType: ?
    SimpleType: ?
    SimpleType: ?
  Var: void
    TypeConstruction: union01
      Ident: union01
      Int: Int
  Match: Str
    Ident: union01
    SimpleType: Int
    Binary: Bool
      Ident: Int
      Int: Int
    Block: Str
      String: Str
    SimpleType: Int
    Block: Str
      String: Str
    Block: Str
      String: Str
---
union01 = Tri = Int | Bool | Str
```

**Match with multiple guarded arms same variant**

```metall
{
    union Foo = Int | Bool
    let x = Foo(42)
    match x {
        case Int n if n > 100: "big"
        case Int n if n > 10: "medium"
        case Int: "small"
        case Bool: "bool"
    }
}
```

```types
Block: Str
  Union: union01
    SimpleType: ?
    SimpleType: ?
  Var: void
    TypeConstruction: union01
      Ident: union01
      Int: Int
  Match: Str
    Ident: union01
    SimpleType: Int
    Binary: Bool
      Ident: Int
      Int: Int
    Block: Str
      String: Str
    SimpleType: Int
    Binary: Bool
      Ident: Int
      Int: Int
    Block: Str
      String: Str
    SimpleType: Int
    Block: Str
      String: Str
    SimpleType: Bool
    Block: Str
      String: Str
---
union01 = Foo = Int | Bool
```

**Match all guarded arms still allows else**

A guarded arm covers no variant, so when every arm is guarded the match is not
exhaustive and an else is still allowed.

```metall
{
    union Foo = Int | Bool
    let x = Foo(42)
    match x {
        case Int n if n > 0: "pos int"
        case Bool b if b: "true bool"
        else: "other"
    }
}
```

```types
Block: Str
  Union: union01
    SimpleType: ?
    SimpleType: ?
  Var: void
    TypeConstruction: union01
      Ident: union01
      Int: Int
  Match: Str
    Ident: union01
    SimpleType: Int
    Binary: Bool
      Ident: Int
      Int: Int
    Block: Str
      String: Str
    SimpleType: Bool
    Ident: Bool
    Block: Str
      String: Str
    Block: Str
      String: Str
---
union01 = Foo = Int | Bool
```

**Match else binding single remaining variant**

When all variants except one are covered, the else binding has the remaining variant's type.

```metall
{
    union Foo = Str | Int
    let x = Foo(42)
    match x {
        case Str s: let y = s
        else i: let y = i
    }
}
```

```bindings
Block: scope01
  Union: scope02
    SimpleType: scope02
    SimpleType: scope02
  Var: scope02
    TypeConstruction: scope02
      Ident: scope02
      Int: scope02
  Match: scope02
    Ident: scope02
    SimpleType: scope02
    Block: scope02
      Var: scope03
        Ident: scope03
    Block: scope02
      Var: scope04
        Ident: scope04
---
scope01:
scope02:
  Foo: union01
  x: union01
scope03:
  s: Str
  y: Str
scope04:
  i: Int
  y: Int
union01 = Foo = Str | Int
```

**Match else binding multiple remaining variants**

When multiple variants remain uncovered, the else binding has the union type.

```metall
{
    union Tri = Str | Int | Bool
    let x = Tri(42)
    match x {
        case Str s: let str_match = s
        else e: let else_match = e
    }
}
```

```bindings
Block: scope01
  Union: scope02
    SimpleType: scope02
    SimpleType: scope02
    SimpleType: scope02
  Var: scope02
    TypeConstruction: scope02
      Ident: scope02
      Int: scope02
  Match: scope02
    Ident: scope02
    SimpleType: scope02
    Block: scope02
      Var: scope03
        Ident: scope03
    Block: scope02
      Var: scope04
        Ident: scope04
---
scope01:
scope02:
  Tri: union01
  x: union01
scope03:
  s: Str
  str_match: Str
scope04:
  e: union01
  else_match: union01
union01 = Tri = Str | Int | Bool
```

**Closed enum match is exhaustive over its variants**

```metall
{
    enum Color U8 = red | green | blue
    let c = Color.red
    match c {
        case Color.red: 1
        case Color.green: 2
        case Color.blue: 3
    }
}
```

```error
```

**Open enum match with a subset arm, a variant arm, and else**

```metall
{
    enum AppErr U32
    enum IOErr AppErr = file_not_found | broken_pipe
    let e AppErr = IOErr.file_not_found
    match e {
        case IOErr.file_not_found: 1
        case IOErr io: 2
        else: 3
    }
}
```

```error
```

**Enum match binds the subset type, else binds the root**

```metall
{
    enum AppErr U32
    enum IOErr AppErr = file_not_found | broken_pipe
    let e AppErr = IOErr.file_not_found
    match e {
        case IOErr io: let got = io
        else other: let got = other
    }
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
  Match: scope02
    Ident: scope02
    SimpleType: scope02
    Block: scope02
      Var: scope05
        Ident: scope05
    Block: scope02
      Var: scope06
        Ident: scope06
---
scope01:
scope02:
  AppErr: enum01
  IOErr: enum02
  IOErr.broken_pipe: enum02
  IOErr.file_not_found: enum02
  e: enum01
scope03:
scope04:
scope05:
  got: enum02
  io: enum02
scope06:
  got: enum01
  other: enum01
enum01 = AppErr
enum02 = IOErr = file_not_found | broken_pipe
```

**try narrows an open enum to a subset**

```metall
{
    enum AppErr U32
    enum IOErr AppErr = file_not_found | broken_pipe
    fun narrow(e AppErr) AppErr {
        let io = try e is IOErr
        io
    }
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
  Fun: scope02
    FunParam: scope05
      SimpleType: scope05
    SimpleType: scope05
    Block: scope05
      Var: scope06
        Match: scope06
          Ident: scope06
          SimpleType: scope06
          Block: scope06
            Ident: scope07
          Block: scope06
            Return: scope08
              Ident: scope08
      Ident: scope06
---
scope01:
scope02:
  AppErr: enum01
  IOErr: enum02
  IOErr.broken_pipe: enum02
  IOErr.file_not_found: enum02
  narrow: fun01
scope03:
scope04:
scope05:
  e: enum01
scope06:
  io: enum02
scope07:
  __try: enum02
scope08:
  __try_e: enum01
enum01 = AppErr
enum02 = IOErr = file_not_found | broken_pipe
fun01  = sync fun(enum01) enum01
```

**Enum match with guard chaining a variant to an unguarded arm**

A guarded arm proves nothing, so an unguarded arm for the same variant still
satisfies exhaustiveness.

```metall
{
    enum Color U8 = red | green | blue
    let c = Color.red
    match c {
        case Color.red if true: 1
        case Color.red: 2
        case Color.green: 3
        case Color.blue: 4
    }
}
```

```error
```

**Match on a mutable reference binds references**

A `&mut` matched value projects through the ref: each arm binds a reference into the
variant. A `&mut x` binding is mutable; a `&y` binding coerces down from `&mut`.

```metall
{
    struct A { v Int }
    struct B { v Int }
    union U = A | B
    mut u = U(A(1))
    match &mut u {
        case A &mut x: let p = x
        case B &y: let q = y
    }
}
```

```bindings
Block: scope01
  Struct: scope02
    StructField: scope03
      SimpleType: scope03
  Struct: scope02
    StructField: scope04
      SimpleType: scope04
  Union: scope02
    SimpleType: scope02
    SimpleType: scope02
  Var: scope02
    TypeConstruction: scope02
      Ident: scope02
      TypeConstruction: scope02
        Ident: scope02
        Int: scope02
  Match: scope02
    Ref: scope02
      Ident: scope02
    SimpleType: scope02
    Block: scope02
      Var: scope05
        Ident: scope05
    SimpleType: scope02
    Block: scope02
      Var: scope06
        Ident: scope06
---
scope01:
scope02:
  A: struct01
  B: struct02
  U: union01
  u: union01 (mut)
scope03:
scope04:
scope05:
  p: &mut struct01
  x: &mut struct01
scope06:
  q: &struct02
  y: &struct02
struct01 = A { v Int }
struct02 = B { v Int }
union01  = U = struct01 | struct02
```

**Match else on a reference binds a reference**

The else binding when matching a reference is itself a reference; with one variant
uncovered it narrows to that variant.

```metall
{
    union Foo = Str | Int
    mut x = Foo(42)
    match &x {
        case Str &s: let y = s
        else &i: let y = i
    }
}
```

```bindings
Block: scope01
  Union: scope02
    SimpleType: scope02
    SimpleType: scope02
  Var: scope02
    TypeConstruction: scope02
      Ident: scope02
      Int: scope02
  Match: scope02
    Ref: scope02
      Ident: scope02
    SimpleType: scope02
    Block: scope02
      Var: scope03
        Ident: scope03
    Block: scope02
      Var: scope04
        Ident: scope04
---
scope01:
scope02:
  Foo: union01
  x: union01 (mut)
scope03:
  s: &Str
  y: &Str
scope04:
  i: &Int
  y: &Int
union01 = Foo = Str | Int
```

**Match on a reference to an enum binds references**

```metall
{
    enum Coin(cents U8) U8 = penny(1) | dime(10)
    let c = Coin.dime
    match &c {
        case Coin.penny &p: let a = p
        case Coin.dime &d: let b = d
    }
}
```

```bindings
Block: scope01
  Enum: scope02
    SimpleType: scope03
    FunParam: scope03
      SimpleType: scope03
    EnumVariant: scope03
      Int: scope03
    EnumVariant: scope03
      Int: scope03
  Var: scope02
    Ident: scope02
  Match: scope02
    Ref: scope02
      Ident: scope02
    FieldAccess: scope02
      SimpleType: scope02
    Block: scope02
      Var: scope04
        Ident: scope04
    FieldAccess: scope02
      SimpleType: scope02
    Block: scope02
      Var: scope05
        Ident: scope05
---
scope01:
scope02:
  Coin: enum01
  Coin.dime: enum01
  Coin.penny: enum01
  c: enum01
scope03:
scope04:
  a: &enum01
  p: &enum01
scope05:
  b: &enum01
  d: &enum01
enum01 = Coin = penny | dime
```

## Errors

**Match on non-union**

```metall
{
    let x = 42
    match x { case Int: 1 }
}
```

```error
test.met:3:11: match expression must be a union or enum type, got Int
        let x = 42
        match x { case Int: 1 }
              ^
    }
```

**Match non-exhaustive**

```metall
{
    union Foo = Str | Int
    let x = Foo("hi")
    match x { case Str: "s" }
}
```

```error
test.met:4:5: non-exhaustive match: missing variant Int
        let x = Foo("hi")
        match x { case Str: "s" }
        ^^^^^^^^^^^^^^^^^^^^^^^^^
    }
```

**Match duplicate arm**

```metall
{
    union Foo = Str | Int
    let x = Foo(1)
    match x {
        case Str: "s"
        case Str: "s2"
        case Int: "i"
    }
}
```

```error
test.met:6:14: duplicate match arm for variant Str
            case Str: "s"
            case Str: "s2"
                 ^^^
            case Int: "i"
```

**Match arm type mismatch**

```metall
{
    union Foo = Str | Int
    let x = Foo("hi")
    match x {
        case Str: "s"
        case Int: 42
    }
}
```

```error
test.met:6:19: match arm type mismatch: expected Str, got Int
            case Str: "s"
            case Int: 42
                      ^^
        }
```

**Match not a variant**

```metall
{
    union Foo = Str | Int
    let x = Foo(1)
    match x {
        case Bool: true
        case Str: "s"
        case Int: "i"
    }
}
```

```error
test.met:5:14: type Bool is not a variant of Foo
        match x {
            case Bool: true
                 ^^^^
            case Str: "s"
```

**Match guard not bool**

```metall
{
    union Foo = Int | Str
    let x = Foo(42)
    match x {
        case Int n if n: "int"
        case Int: "int"
        case Str: "str"
    }
}
```

```error
test.met:5:23: guard condition must be Bool, got Int
        match x {
            case Int n if n: "int"
                          ^
            case Int: "int"
```

**Match guard non-exhaustive without unguarded arm**

```metall
{
    union Foo = Int | Str
    let x = Foo(42)
    match x {
        case Int n if n > 0: "positive"
        case Str: "str"
    }
}
```

```error
test.met:4:5: non-exhaustive match: missing variant Int
        let x = Foo(42)
        match x {
        ^
            case Int n if n > 0: "positive"
            case Str: "str"
        }
        ^
    }
```

**Match redundant else on exhaustive union**

An else over a union match that already covers every variant is redundant and
rejected, mirroring the closed-enum rule.

```metall
{
    union Foo = Int | Str
    let x = Foo(42)
    match x {
        case Int: "i"
        case Str: "s"
        else: "other"
    }
}
```

```error
test.met:4:5: else is not allowed in a match on Foo; it is exhaustive
        let x = Foo(42)
        match x {
        ^
            case Int: "i"
            case Str: "s"
            else: "other"
        }
        ^
    }
```

**Match all arms diverge cannot assign**

```metall
fun foo() Int {
    union Foo = Str | Int
    let x = Foo(42)
    let y = match x {
        case Str: return 1
        case Int: return 2
    }
    y
}
```

```error
test.met:4:5: cannot assign expression of type 'never' to a variable
        let x = Foo(42)
        let y = match x {
        ^
            case Str: return 1
            case Int: return 2
        }
        ^
        y

test.met:8:5: symbol not defined: y
        }
        y
        ^
    }
```

**Try on non-union**

```metall
fun foo() Int {
    let x = try 42
}
```

```error
test.met:2:17: match expression must be a union or enum type, got Int
    fun foo() Int {
        let x = try 42
                    ^^
    }
```

**Try with is wrong variant**

```metall
{
    union Foo = Int | Str
    fun foo() Foo {
        let x = try Foo(42) is Bool
    }
}
```

```error
test.met:4:32: type Bool is not a variant of Foo
        fun foo() Foo {
            let x = try Foo(42) is Bool
                                   ^^^^
        }
```

**Try short form return type mismatch**

```metall
{
    union Foo = Int | Str
    fun foo() Bool {
        let x = try Foo(42)
    }
}
```

```error
test.met:4:17: return type mismatch: expected Bool, got Str
        fun foo() Bool {
            let x = try Foo(42)
                    ^^^
        }
```

**Try else must break control flow**

```metall
{
    union Foo = Int | Str
    fun foo() Int {
        try Foo(42) else e {
            42
        }
    }
}
```

```error
test.met:4:28: try else block must break control flow
        fun foo() Int {
            try Foo(42) else e {
                               ^
                42
            }
            ^
        }
```

**Non-exhaustive closed enum match**

```metall
{
    enum Color U8 = red | green | blue
    let c = Color.red
    match c {
        case Color.red: 1
        case Color.green: 2
    }
}
```

```error
test.met:4:5: non-exhaustive match: missing variant Color.blue
        let c = Color.red
        match c {
        ^
            case Color.red: 1
            case Color.green: 2
        }
        ^
    }
```

**Else not allowed on closed enum match**

```metall
{
    enum Color U8 = red | green
    let c = Color.red
    match c {
        case Color.red: 1
        case Color.green: 2
        else: 3
    }
}
```

```error
test.met:4:5: else is not allowed in a match on closed enum Color; it is exhaustive
        let c = Color.red
        match c {
        ^
            case Color.red: 1
            case Color.green: 2
            else: 3
        }
        ^
    }
```

**Else allowed on partial closed enum match**

```metall
{
    enum Color U8 = red | green | blue
    let c = Color.red
    match c {
        case Color.red: 1
        else: 2
    }
}
```

```error
```

**Open enum match requires else**

```metall
{
    enum AppErr U32
    enum IOErr AppErr = a | b
    let e AppErr = IOErr.a
    match e {
        case IOErr.a: 1
        case IOErr.b: 2
    }
}
```

```error
test.met:5:5: non-exhaustive match on open enum AppErr: an else arm is required
        let e AppErr = IOErr.a
        match e {
        ^
            case IOErr.a: 1
            case IOErr.b: 2
        }
        ^
    }
```

**Match arm variant from a different enum**

```metall
{
    enum Color U8 = red | green
    enum Size U8 = small | big
    let c = Color.red
    match c {
        case Color.red: 1
        case Size.small: 2
    }
}
```

```error
test.met:7:14: Size.small is not a variant of Color
            case Color.red: 1
            case Size.small: 2
                 ^^^^^^^^^^
        }
```

**Unreachable enum match arm**

```metall
{
    enum AppErr U32
    enum IOErr AppErr = a | b
    let e AppErr = IOErr.a
    match e {
        case IOErr e2: 1
        case IOErr.a: 2
        else: 3
    }
}
```

```error
test.met:7:14: unreachable match arm: all variants already covered
            case IOErr e2: 1
            case IOErr.a: 2
                 ^^^^^^^
            else: 3
```

**Closed enum non-exhaustive when a variant is only guarded**

A guarded arm proves nothing, so it cannot satisfy exhaustiveness, and a closed
enum has no else to fall back on.

```metall
{
    enum Color U8 = red | green | blue
    let c = Color.red
    match c {
        case Color.red if true: 1
        case Color.green: 2
        case Color.blue: 3
    }
}
```

```error
test.met:4:5: non-exhaustive match: missing variant Color.red
        let c = Color.red
        match c {
        ^
            case Color.red if true: 1
            case Color.green: 2
            case Color.blue: 3
        }
        ^
    }
```

**Bare try on an enum is rejected**

An enum value has no payload to unwrap, so bare `try` has nothing to bind.

```metall
{
    enum AppErr U32
    enum IOErr AppErr = a | b
    fun f(e AppErr) AppErr {
        let x = try e
        e
    }
}
```

```error
test.met:5:17: `try` on an enum requires a subset pattern, e.g. `try e is IOErr`
        fun f(e AppErr) AppErr {
            let x = try e
                    ^^^
            e
```

**try on a closed enum has no subset to narrow to**

```metall
{
    enum Color U8 = red | green | blue
    fun f(c Color) Color {
        try c is Color else { return c }
        c
    }
}
```

```error
test.met:4:18: Color is not a subset of Color
        fun f(c Color) Color {
            try c is Color else { return c }
                     ^^^^^
            c
```

**Reference binding when matching a value**

A value cannot bind references; match on `&value` instead.

```metall
{
    union Foo = Str | Int
    let x = Foo(42)
    match x {
        case Str &s: 1
        case Int &i: 2
    }
}
```

```error
test.met:5:14: cannot bind a reference here: the matched value is not a reference; match on `&value` to bind references
        match x {
            case Str &s: 1
                 ^^^
            case Int &i: 2
```

**Value binding when matching a reference**

Matching a reference requires reference bindings.

```metall
{
    union Foo = Str | Int
    mut x = Foo(42)
    match &x {
        case Str s: 1
        case Int i: 2
    }
}
```

```error
test.met:5:14: the matched value is a reference; bind with `&` or `&mut`, not by value
        match &x {
            case Str s: 1
                 ^^^
            case Int i: 2
```

**Mutable binding from an immutable reference**

```metall
{
    union Foo = Str | Int
    mut x = Foo(42)
    match &x {
        case Str &mut s: 1
        case Int &mut i: 2
    }
}
```

```error
test.met:5:14: cannot take a `&mut` binding from a `&` value
        match &x {
            case Str &mut s: 1
                 ^^^
            case Int &mut i: 2
```

**Assign through an immutable reference binding**

```metall
{
    struct A { v Int }
    struct B { v Int }
    union U = A | B
    mut u = U(A(1))
    match &u {
        case A &x: x.v = 5
        case B &y: y.v = 6
    }
}
```

```error
test.met:7:20: cannot assign to field of immutable value
        match &u {
            case A &x: x.v = 5
                       ^^^
            case B &y: y.v = 6
```
