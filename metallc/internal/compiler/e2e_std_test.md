# E2E Std Tests

Tests that use the full prelude and standard library (`lib/` in include paths).

## Std

**io println**

```metall
use std::io

fun main() void {
    io::println("hello std")
}
```

```output
hello std
```

## Macros

**setup macro modules**

```metall
```

```module.type_name_macro
use std::comp

fun apply(name Str, info comp::Type, sb &mut StrBuilder, @a Arena) void {
    sb.str("fun ").str(name).str("() Str { ").rune('"')
    match info {
        case comp::BoolType b: { sb.str("Bool") void }
        case comp::StrType s: { sb.str("Str") void }
        case comp::VoidType v: { sb.str("void") void }
        case comp::IntType i: { sb.str(i.name) void }
        case comp::StructType s: { sb.str("struct ").str(s.name) void }
    }
    sb.rune('"').str(" }").nl()
}
```

**comp type_of**

```metall
use std::comp
use std::io
use local::type_name_macro

type_name_macro::apply("bool_name", comp::type_of<Bool>())
type_name_macro::apply("str_name", comp::type_of<Str>())
type_name_macro::apply("int_name", comp::type_of<Int>())
type_name_macro::apply("u8_name", comp::type_of<U8>())
type_name_macro::apply("u16_name", comp::type_of<U16>())
type_name_macro::apply("u32_name", comp::type_of<U32>())
type_name_macro::apply("u64_name", comp::type_of<U64>())
type_name_macro::apply("i8_name", comp::type_of<I8>())
type_name_macro::apply("i16_name", comp::type_of<I16>())
type_name_macro::apply("i32_name", comp::type_of<I32>())
type_name_macro::apply("rune_name", comp::type_of<Rune>())

struct Point { x Int y Int }

type_name_macro::apply("point_name", comp::type_of<Point>())

fun main() void {
    io::println(bool_name())
    io::println(str_name())
    io::println(int_name())
    io::println(u8_name())
    io::println(u16_name())
    io::println(u32_name())
    io::println(u64_name())
    io::println(i8_name())
    io::println(i16_name())
    io::println(i32_name())
    io::println(rune_name())
    io::println(point_name())
}
```

```output
Bool
Str
Int
U8
U16
U32
U64
I8
I16
I32
Rune
struct test.Point
```
