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
  Union: ?
    SimpleType: ?
    SimpleType: ?
  Var: void
    TypeConstruction: ?
      Ident: ?
      String: Str
  Match: Str
    Ident: ?
    SimpleType: Str
    Block: Str
      String: Str
    SimpleType: Int
    Block: Str
      String: Str
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
  Union: ?
    SimpleType: ?
    SimpleType: ?
  Var: void
    TypeConstruction: ?
      Ident: ?
      Int: Int
  Match: void
    Ident: ?
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
fun01 = fun(Int) void
fun02 = fun(Str) void
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
  Union: ?
    SimpleType: ?
    SimpleType: ?
    SimpleType: ?
  Var: void
    TypeConstruction: ?
      Ident: ?
      String: Str
  Match: Str
    Ident: ?
    SimpleType: Str
    Block: Str
      String: Str
    Block: Str
      String: Str
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
    Union: ?
      SimpleType: ?
      SimpleType: ?
    Var: void
      TypeConstruction: ?
        Ident: ?
        Int: Int
    Match: void
      Ident: ?
      SimpleType: Str
      Block: void
        Return: void
          Int: Int
      SimpleType: Int
      Block: void
        Return: void
          Int: Int
---
fun01 = fun() Int
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
    Union: ?
      SimpleType: ?
      SimpleType: ?
      SimpleType: ?
    Var: void
      TypeConstruction: ?
        Ident: ?
        Int: Int
    Match: Int
      Ident: ?
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
fun01 = fun() Int
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
  Union: ?
    TypeParam: ?
    SimpleType: ?
    SimpleType: ?
  Var: void
    TypeConstruction: ?
      Ident: ?
        SimpleType: Int
      Int: Int
  Match: Int
    Ident: ?
    SimpleType: Int
    Block: Int
      Ident: Int
    SimpleType: Bool
    Block: Int
      Int: Int
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
  Union: ?
    SimpleType: ?
    SimpleType: ?
  Var: void
    TypeConstruction: ?
      Ident: ?
      Int: Int
  Match: Str
    Ident: ?
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
  Union: ?
    SimpleType: ?
    SimpleType: ?
    SimpleType: ?
  Var: void
    TypeConstruction: ?
      Ident: ?
      Int: Int
  Match: Str
    Ident: ?
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
  Union: ?
    SimpleType: ?
    SimpleType: ?
  Var: void
    TypeConstruction: ?
      Ident: ?
      Int: Int
  Match: Str
    Ident: ?
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
