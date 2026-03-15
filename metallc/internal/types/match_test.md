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
        case Int n: print_int(n)
        case Str s: print_str(s)
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
      Call: void
        Ident: fun01
        Ident: Int
    SimpleType: Str
    Block: void
      Call: void
        Ident: fun02
        Ident: Str
---
union01 = Foo = Str | Int
fun01   = fun(Int) void
fun02   = fun(Str) void
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
      SimpleType: Str
      Block: void
        Return: void
          Int: Int
      SimpleType: Int
      Block: void
        Return: void
          Int: Int
---
fun01   = fun() Int
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
      Block: void
        Return: void
          Int: Int
      SimpleType: Int
      Block: Int
        Ident: Int
      SimpleType: Bool
      Block: Int
        Int: Int
---
fun01   = fun() Int
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
        SimpleType: Int
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

**Match else binding single remaining variant**

When all variants except one are covered, the else binding has the remaining variant's type.

```metall
{
    union Foo = Str | Int
    let x = Foo(42)
    match x {
        case Str s: print_str(s)
        else i: print_int(i)
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
      Call: scope03
        Ident: scope03
        Ident: scope03
    Block: scope02
      Call: scope04
        Ident: scope04
        Ident: scope04
---
scope01:
scope02:
  Foo: union01
  x: union01
scope03:
  s: Str
scope04:
  i: Int
union01 = Foo = Str | Int
```

**Match else binding multiple remaining variants**

When multiple variants remain uncovered, the else binding has the union type.

```metall
{
    union Tri = Str | Int | Bool
    let x = Tri(42)
    match x {
        case Str s: print_str(s)
        else v:
            match v {
                case Int n: print_int(n)
                case Bool: print_str("bool")
                case Str: print_str("unreachable")
            }
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
      Call: scope03
        Ident: scope03
        Ident: scope03
    Block: scope02
      Match: scope04
        Ident: scope04
        SimpleType: scope04
        Block: scope04
          Call: scope05
            Ident: scope05
            Ident: scope05
        SimpleType: scope04
        Block: scope04
          Call: scope06
            Ident: scope06
            String: scope06
        SimpleType: scope04
        Block: scope04
          Call: scope07
            Ident: scope07
            String: scope07
---
scope01:
scope02:
  Tri: union01
  x: union01
scope03:
  s: Str
scope04:
  v: union01
scope05:
  n: Int
scope06:
scope07:
union01 = Tri = Str | Int | Bool
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
test.met:3:11: match expression must be a union type, got Int
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
test.met:4:5: cannot assign void to a variable
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
