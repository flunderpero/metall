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

fun type_name(name Str, info comp::Type, sb &mut StrBuilder, @a Arena) void {
    sb.str("fun ").str(name).str("() Str { ").rune('"')
    match info {
        case comp::BoolType b: { sb.str("Bool") void }
        case comp::StrType s: { sb.str("Str") void }
        case comp::VoidType v: { sb.str("void") void }
        case comp::IntType i: { sb.str(i.name) void }
        case comp::StructType s: { sb.str("struct ").str(s.name) void }
        case comp::UnionType u: { sb.str("union ").str(u.name) void }
    }
    sb.rune('"').str(" }").nl()
}

fun field_count(name Str, info comp::Type, sb &mut StrBuilder, @a Arena) void {
    match info {
        case comp::StructType s: {
            sb.str("fun ").str(name).str("() Int { ").int(s.fields.len).str(" }").nl()
            void
        }
        else: { void }
    }
}
```

```module.fmtstr_macro
use std::comp

fun quote(sb &mut StrBuilder) void {
    sb.rune(34)
    void
}

fun gen_fmt_str(info comp::Type, sb &mut StrBuilder, @a Arena) void {
    match info {
        case comp::StructType s: {
            sb.str("fun ").str(s.name).str(".fmt_str(v ").str(s.name).str(", sb &mut StrBuilder) void {").nl()
            sb.str("    sb.str(")
            quote(sb)
            sb.str(s.name).str("{")
            quote(sb)
            sb.str(")").nl()
            for i in 0..s.fields.len {
                let f = s.fields[i]
                if i > 0 {
                    sb.str("    sb.str(")
                    quote(sb)
                    sb.str(", ").str(f.name).str("=")
                    quote(sb)
                    sb.str(")").nl()
                    void
                } else {
                    sb.str("    sb.str(")
                    quote(sb)
                    sb.str(f.name).str("=")
                    quote(sb)
                    sb.str(")").nl()
                    void
                }
                sb.str("    sb.fmt(v.").str(f.name).str(")").nl()
                void
            }
            sb.str("    sb.str(")
            quote(sb)
            sb.str("}")
            quote(sb)
            sb.str(")").nl()
            sb.str("}").nl()
            void
        }
        case comp::BoolType b: { void }
        case comp::StrType s: { void }
        case comp::VoidType v: { void }
        case comp::IntType i: { void }
        case comp::UnionType u: { void }
    }
}
```

**comp type_of**

```metall
use std::comp
use std::io
use local::type_name_macro

type_name_macro::type_name("bool_name", comp::type_of<Bool>())
type_name_macro::type_name("str_name", comp::type_of<Str>())
type_name_macro::type_name("int_name", comp::type_of<Int>())
type_name_macro::type_name("u8_name", comp::type_of<U8>())
type_name_macro::type_name("u16_name", comp::type_of<U16>())
type_name_macro::type_name("u32_name", comp::type_of<U32>())
type_name_macro::type_name("u64_name", comp::type_of<U64>())
type_name_macro::type_name("i8_name", comp::type_of<I8>())
type_name_macro::type_name("i16_name", comp::type_of<I16>())
type_name_macro::type_name("i32_name", comp::type_of<I32>())
type_name_macro::type_name("rune_name", comp::type_of<Rune>())

struct Point { x Int y Int }

type_name_macro::type_name("point_name", comp::type_of<Point>())
type_name_macro::field_count("point_fields", comp::type_of<Point>())

union Shape = Point | Bool

type_name_macro::type_name("shape_name", comp::type_of<Shape>())

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
    DebugIntern.print_int(point_fields())
    io::println(shape_name())
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
struct Point
union Shape
2
```

**fmtstr macro**

```metall
use std::comp
use std::io
use local::fmtstr_macro

struct Point { x Int y Int }

fmtstr_macro::gen_fmt_str(comp::type_of<Point>())

fun main() void {
    let @a = Arena()
    let sb = StrBuilder.new(256, @a)
    let p = Point(10, 20)
    p.fmt_str(sb)
    io::println(sb.to_str())
}
```

```output
Point{x=10, y=20}
```

**macro inside function body**

```metall
use std::comp
use std::io
use local::fmtstr_macro

struct Pair { a Str b Int }

fun main() void {
    fmtstr_macro::gen_fmt_str(comp::type_of<Pair>())

    let @a = Arena()
    let sb = StrBuilder.new(256, @a)
    Pair("hello", 42).fmt_str(sb)
    io::println(sb.to_str())
}
```

```output
Pair{a=hello, b=42}
```

**shape impl for non-top-level struct**

```metall
use std::io

shape Eq {
    fun Eq.eq(a Eq, b Eq) Bool
}

fun assert_eq<T Eq>(a T, b T) void {
    if a.eq(b) { io::println("equal") } else { io::println("not equal") }
}

fun main() void {
    struct Point { x Int y Int }
    fun Point.eq(a Point, b Point) Bool { a.x == b.x and a.y == b.y }

    assert_eq(Point(1, 2), Point(1, 2))
    assert_eq(Point(1, 2), Point(3, 4))
}
```

```output
equal
not equal
```

**debug location**

```metall
use std::debug
use std::io

fun main() void {
    io::println(debug::location())
}
```

```output
test.met:5:17
```

**default parameters**

```metall
use std::io

struct Point { x Int y Int }

union Shape = Point | Int

fun move_to(target Point = Point(0, 0), dx Int = 1) Point {
    Point(target.x + dx, target.y)
}

fun sum(s Shape = Shape(0)) Int {
    match s {
        case Point p: p.x + p.y
        case Int n: n
    }
}

fun main() void {
    -- default struct argument
    io::println(move_to().x)
    io::println(move_to().y)
    io::println(move_to(Point(10, 20)).x)
    io::println(move_to(Point(10, 20)).y)
    io::println(move_to(Point(10, 20), 5).x)

    -- default union argument
    io::println(sum())
    io::println(sum(Shape(Point(3, 4))))
    io::println(sum(Shape(99)))
}
```

```output
1
0
11
20
15
0
7
99
```
