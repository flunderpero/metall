# E2E Std Tests

Tests that use the full prelude and standard library (`lib/` in include paths).

## Std

**io println**

```metall
use std.io

fun main() void {
    io.println("hello std")
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
use std.comp

pub fun type_name(name Str, info comp.Type, sw &mut StrWriter) void {
    sw.write("fun ")
    sw.write(name)
    sw.write("() Str { ")
    sw.write('"')
    match info {
        case comp.BoolType b: { sw.write("Bool") }
        case comp.StrType s: { sw.write("Str") }
        case comp.VoidType v: { sw.write("void") }
        case comp.NeverType v: { sw.write("never") }
        case comp.IntType i: { sw.write(i.name) }
        case comp.StructType s: { sw.write("struct ") sw.write(s.name) }
        case comp.UnionType u: { sw.write("union ") sw.write(u.name) }
        case comp.EnumType e: { sw.write("enum ") sw.write(e.name) }
    }
    sw.write('"')
    sw.write(" }")
    sw.write("\n")
}

pub fun field_count(name Str, info comp.Type, sw &mut StrWriter) void {
    match info {
        case comp.StructType s: {
            sw.write("fun ")
            sw.write(name)
            sw.write("() Int { ")
            sw.write(s.fields.len)
            sw.write(" }")
            sw.write("\n")
        }
        else: { }
    }
}
```

```module.fmtstr_macro
use std.comp

fun quote(sw &mut StrWriter) void {
    sw.write('"')
}

pub fun gen_fmt(info comp.Type, sw &mut StrWriter) void {
    match info {
        case comp.StructType s: {
            sw.write("fun ")
            sw.write(s.name)
            sw.write(".fmt(v ")
            sw.write(s.name)
            sw.write(", sw &mut StrWriter) void {")
            sw.write("\n")
            sw.write("    sw.write(")
            quote(sw)
            sw.write(s.name)
            sw.write("{")
            quote(sw)
            sw.write(")")
            sw.write("\n")
            for i in 0..s.fields.len {
                let f = s.fields[i]
                if i > 0 {
                    sw.write("    sw.write(")
                    quote(sw)
                    sw.write(", ")
                    sw.write(f.name)
                    sw.write("=")
                    quote(sw)
                    sw.write(")")
                    sw.write("\n")
                } else {
                    sw.write("    sw.write(")
                    quote(sw)
                    sw.write(f.name)
                    sw.write("=")
                    quote(sw)
                    sw.write(")")
                    sw.write("\n")
                }
                sw.write("    sw.write(v.") sw.write(f.name) sw.write(")") sw.write("\n")
            }
            sw.write("    sw.write(")
            quote(sw)
            sw.write("}")
            quote(sw)
            sw.write(")")
            sw.write("\n")
            sw.write("}")
            sw.write("\n")
        }
        case comp.BoolType b: { }
        case comp.StrType s: { }
        case comp.VoidType v: { }
        case comp.NeverType v: { }
        case comp.IntType i: { }
        case comp.UnionType u: { }
        case comp.EnumType e: { }
    }
}
```

**comp type_of**

The expected output interleaves `io.println` (which goes through `write`,
unbuffered) with `DebugIntern.print_int` (which goes through libc's
printf, line/pipe-buffered on native). On wasm there is no stdio
buffering, so `print_int` lands in source order instead of at flush-time,
giving a different-but-correct interleaving.

```metall !wasm
use std.comp
use std.io
use local.type_name_macro

type_name_macro.type_name("bool_name", comp.type_of<Bool>())
type_name_macro.type_name("str_name", comp.type_of<Str>())
type_name_macro.type_name("int_name", comp.type_of<Int>())
type_name_macro.type_name("u8_name", comp.type_of<U8>())
type_name_macro.type_name("u16_name", comp.type_of<U16>())
type_name_macro.type_name("u32_name", comp.type_of<U32>())
type_name_macro.type_name("u64_name", comp.type_of<U64>())
type_name_macro.type_name("i8_name", comp.type_of<I8>())
type_name_macro.type_name("i16_name", comp.type_of<I16>())
type_name_macro.type_name("i32_name", comp.type_of<I32>())
type_name_macro.type_name("rune_name", comp.type_of<Rune>())

struct Point { x Int y Int }

type_name_macro.type_name("point_name", comp.type_of<Point>())
type_name_macro.field_count("point_fields", comp.type_of<Point>())

union Shape = Point | Bool

type_name_macro.type_name("shape_name", comp.type_of<Shape>())

fun main() void {
    io.println(bool_name())
    io.println(str_name())
    io.println(int_name())
    io.println(u8_name())
    io.println(u16_name())
    io.println(u32_name())
    io.println(u64_name())
    io.println(i8_name())
    io.println(i16_name())
    io.println(i32_name())
    io.println(rune_name())
    io.println(point_name())
    DebugIntern.print_int(point_fields())
    io.println(shape_name())
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
2
union Shape
```

**fmtstr macro**

```metall
use std.comp
use std.io
use local.fmtstr_macro

struct Point { x Int y Int }

fmtstr_macro.gen_fmt(comp.type_of<Point>())

fun main() void {
    let @a = Arena()
    let sw = StrWriter.new(256, @a)
    let p = Point(10, 20)
    p.fmt(sw)
    io.println(sw.as_str())
}
```

```output
Point{x=10, y=20}
```

**macro inside function body**

```metall
use std.comp
use std.io
use local.fmtstr_macro

struct Pair { a Str b Int }

fun main() void {
    fmtstr_macro.gen_fmt(comp.type_of<Pair>())

    let @a = Arena()
    let sw = StrWriter.new(256, @a)
    Pair("hello", 42).fmt(sw)
    io.println(sw.as_str())
}
```

```output
Pair{a=hello, b=42}
```

**enum reflection macro**

```metall
use std.comp
use std.io
use local.enum_reflect_macro

enum Suit U8 = hearts | spades | clubs | diamonds

-- An open root has no variants of its own, so it reflects an empty variant list.
enum Event U16

enum_reflect_macro.variant_names("suits", comp.type_of<Suit>())
enum_reflect_macro.variant_names("events", comp.type_of<Event>())

fun main() void {
    io.println(suits())
    io.println(events())
}
```

```module.enum_reflect_macro
use std.comp

pub fun variant_names(name Str, info comp.Type, sw &mut StrWriter) void {
    match info {
        case comp.EnumType e: {
            sw.write("fun ") sw.write(name) sw.write("() Str { ")
            sw.write('"')
            sw.write(e.name)
            sw.write(":")
            for i in 0..e.variants.len {
                if i > 0 { sw.write(",") }
                sw.write(e.variants[i].name)
            }
            sw.write('"')
            sw.write(" }")
            sw.write("\n")
        }
        else: { }
    }
}
```

```output
Suit:hearts,spades,clubs,diamonds
Event:
```

**shape impl for non-top-level struct**

```metall
use std.io

shape Eq {
    fun Eq.eq(a Eq, b Eq) Bool
}

fun assert_eq<T Eq>(a T, b T) void {
    if a.eq(b) { io.println("equal") } else { io.println("not equal") }
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
use std.debug
use std.io

fun main() void {
    io.println(debug.location())
}
```

```output
test.met:5:16
```

**default parameters**

```metall
use std.io

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
    io.println(move_to().x)
    io.println(move_to().y)
    io.println(move_to(Point(10, 20)).x)
    io.println(move_to(Point(10, 20)).y)
    io.println(move_to(Point(10, 20), 5).x)

    -- default union argument
    io.println(sum())
    io.println(sum(Shape(Point(3, 4))))
    io.println(sum(Shape(99)))
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

**os.args returns at least the program name**

```metall !wasm
use std.os
use std.io

fun main() void {
    let a = os.args()
    io.println(a.len > 0)
}
```

```output
true
```

## References To Temporaries

**mut ref to call result, chained method call**

```metall
use std.io

fun main() void {
    let first = (&mut "xyz".iter()).next()
    match first {
        case Rune r: io.println(r.to_u32())
        else: io.println("none")
    }
}
```

```output
120
```

## Conditional Compilation

**conditional import with matching tag**

```metall tag:use_io
#if tag.use_io
use std.io
#end

fun main() void {
    io.println("imported")
}
```

```output
imported
```

**conditional import with non-matching tag skips import**

```metall
#if tag.nope
use std.io
#end

fun main() void {
    DebugIntern.print_str("no import")
}
```

```output
no import
```

## Enums

**variants builtin**

```metall
use std.io
use std.enums

enum Suit U8 = hearts | spades | clubs | diamonds

fun main() void {
    let all = enums.variants<Suit>()
    for i in 0..all.len {
        io.println(all[i].debug_name)
    }
}
```

```output
hearts
spades
clubs
diamonds
```

**tag and from_tag convert between an enum and its backing integer**

```metall
use std.io
use std.enums

enum Color U8 = red | green | blue

fun main() void {
    io.println(Color.green.tag)
    match enums.from_tag<Color>(2) {
        case None: io.println("none")
        else c: io.println(c.debug_name)
    }
    match enums.from_tag<Color>(7) {
        case None: io.println("out of range")
        else c: io.println("BAD")
    }
}
```

```output
1
blue
out of range
```

## Format Strings

**format string forms**

```metall
fun main() void {
    let @a = Arena()
    let x = 42
    -- Each line is labeled with the modifier it exercises, so the output maps
    -- one-to-one to the input.

    -- f: a string, an expression, a bool, and a float.
    DebugIntern.print_str(f"f: {"s"} {1 + 2} {true} {3.5}".build(@a))

    -- f#: #{...} interpolates; bare braces stay literal.
    DebugIntern.print_str(f#"f#: {bare} #{x}"#.build(@a))

    -- f##: needs ##{...}, so a lone #{ stays literal.
    DebugIntern.print_str(f##"f##: #{lone} ##{x}"##.build(@a))

    -- if-expression with braced, string-literal branches; the braces balance.
    DebugIntern.print_str(f"if: {if x > 0 { "pos" } else { "neg" }}".build(@a))

    -- fm: dedented; an interpolated value keeps its own newlines verbatim.
    let two = "p\n  q"
    DebugIntern.print_str(fm"
        fm: x={x}
        {two}
        ".build(@a))

    -- write_to: two f-strings append into one StrWriter, no intermediate Str.
    let sw = StrWriter.new(16, @a)
    f"wr: a={x}".write_to(sw)
    f" b={true}".write_to(sw)
    DebugIntern.print_str(sw.as_str())

    -- fb: bytes round-trip to text; é is multi-byte UTF-8.
    DebugIntern.print_str(Str.from_utf8_lossy(fb"fb: café {x}".build(@a), @a))

    -- fb byte length exceeds the character count: é is 2 bytes, \xff a raw byte.
    let raw = fb"é\xff".build(@a)
    DebugIntern.print_str(f"fb.len: {raw.len}".build(@a))

    -- fbm: multi-line bytes; \u{1F600} is a single 4-byte char.
    let smile = fbm"
        \u{1F600}
        ".build(@a)
    DebugIntern.print_str(f"fbm.len: {smile.len}".build(@a))
}
```

```output
f: s 3 true 3.5
f#: {bare} 42
f##: #{lone} 42
if: pos
fm: x=42
p
  q
wr: a=42 b=true
fb: café 42
fb.len: 3
fbm.len: 4
```

**format specifier dispatches to fmt_ext**

A `:` specifier routes an interpolation through the value's `fmt_ext` instead of
`fmt`, passing the StrWriter first and the parsed spec arguments after it. Named
arguments reach their parameter by name regardless of order, and a bare specifier
passes positionally.

```metall
struct Labeled { n Int }

-- A test-only stand-in: it echoes the spec arguments so the test can assert they
-- arrive in the right slots whether written named or positional.
fun Labeled.fmt_ext(l Labeled, sw &mut StrWriter, base Int, upper Bool) void {
    sw.write(l.n)
    sw.write("/")
    sw.write(base)
    sw.write("/")
    sw.write(upper)
}

fun main() void {
    let @a = Arena()
    let lab = Labeled(7)
    DebugIntern.print_str(f"named: {lab:upper=true, base=16}".build(@a))
    DebugIntern.print_str(f"pos: {lab:8, false}".build(@a))
}
```

```output
named: 7/16/true
pos: 7/8/false
```

**format specifiers on built-in types**

Every integer type takes the same `base`/`upper`/`width` spec (the narrow types delegate
to `Int`/`U64`), so each line below prints that type's max in upper-case hex and the column
doubles as a range table. `Float`/`F32` take `precision` and a space-padded `width`; `Bool`
takes the word printed for each case.

```metall
fun main() void {
    let @a = Arena()

    DebugIntern.print_str(f"I8  {I8(0x7F):base=16, upper=true}".build(@a))
    DebugIntern.print_str(f"I16 {I16(0x7FFF):base=16, upper=true}".build(@a))
    DebugIntern.print_str(f"I32 {I32(0x7FFF_FFFF):base=16, upper=true}".build(@a))
    DebugIntern.print_str(f"Int {Int(0x7FFF_FFFF_FFFF_FFFF):base=16, upper=true}".build(@a))
    DebugIntern.print_str(f"U8  {U8(0xFF):base=16, upper=true}".build(@a))
    DebugIntern.print_str(f"U16 {U16(0xFFFF):base=16, upper=true}".build(@a))
    DebugIntern.print_str(f"U32 {U32(0xFFFF_FFFF):base=16, upper=true}".build(@a))
    DebugIntern.print_str(f"U64 {U64(0xFFFF_FFFF_FFFF_FFFF):base=16, upper=true}".build(@a))

    -- base 2/8/16 (lower-case) and a zero-padded width, sign ahead of the padding.
    DebugIntern.print_str(f"bin {5:base=2} oct {64:base=8} hex {255:base=16}".build(@a))
    DebugIntern.print_str(f"pad {42:width=6} neg {-42:width=6}".build(@a))

    -- Float / F32: fixed precision and a space-padded width.
    DebugIntern.print_str(f"Float {3.14159:precision=2} {-3.14:width=8}".build(@a))
    DebugIntern.print_str(f"F32   {1.5.to_f32():precision=3} {2.5.to_f32():width=8}".build(@a))

    -- Bool: the word printed for each case.
    DebugIntern.print_str(f"Bool  {true:true_str="yes"} {false:false_str="no"}".build(@a))
}
```

```output
I8  7F
I16 7FFF
I32 7FFFFFFF
Int 7FFFFFFFFFFFFFFF
U8  FF
U16 FFFF
U32 FFFFFFFF
U64 FFFFFFFFFFFFFFFF
bin 101 oct 100 hex ff
pad     42 neg    -42
Float 3.14    -3.14
F32   1.500      2.5
Bool  yes no
```

## Arithmetic Overflow

`abs` is a full-prelude function, so its overflow test lives here rather than
with the minimal-prelude overflow tests in `e2e_test.md`. It is `!fast` because
`fast` disables the overflow check.

**abs of the most negative Int panics**

`Int.abs` negates via a checked subtract, so `abs(MIN)` overflows because MIN has
no positive counterpart. The trap fires inside the prelude, at the negation, so
the exact line:col is redacted.

```metall !fast
fun main() void {
    let min = -9223372036854775808
    DebugIntern.print_int(min.abs())
}
```

```panic
prelude.met:<ignored in test>: integer overflow
```
