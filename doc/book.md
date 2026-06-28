# Metall - Systems Programming For Humans

Ok, that claim is a bit silly in today's day and age, but it perfectly sums
up the main goal __I__ had while designing and developing the language:

**I want to have fun in a systems programming language.**

"Fun" is very personal and to each their own. 

### Goals And Non-Goals

Metall shall be:

- A systems programming language close to the ... Metal(l).
- Faster than Rust (build and execute) and on par with C (see benchmarks/).
- Memory safe.
- Safe to write concurrent code.
- As complex as needed but not more.
- Pragmatic and not in your way.

### Allocators And Lifetimes

Metall is a memory-safe language that uses scoped arena allocators and 
automatic, scope-based lifetime analysis. No need for lifetime annotations.

```metall
use std.io

fun main() void {
    let @main = Arena()
    let bytes = { 
        let @inner = Arena()

        -- `i` is bound to the scope of `@inner` and cannot outlive it.
        -- We could not yield `i` from this block.
        let i = @inner.new<U64>(137)
        -- But we can pass it to functions. Yes, that's Zig-style deref.
        io.println(i.*)
        
        -- This allocation outlives the current scope up to scope of `@main`.
        @main.slice(3, 65)
    }
    io.println(bytes)
}
```

```output
137
[65, 65, 65]
```

An object cannot outlive its allocation:

```metall
fun main() void {
    let x = {
        let @inner = Arena()
        @inner.new(5)
    }
}
```

```error
test.met:4:9: reference escaping its allocation scope (via block result)
            let @inner = Arena()
            @inner.new(5)
            ^^^^^^^^^^^^^
        }
```

The first allocations of an allocator are always on the stack. The size
of that (cheap) stack page can be configured - the default is 1024 bytes.

## Control Flow

```metall
use std.io

fun control_flow(a Int) void {

    -- if/else:
    if a > 5 {
        io.print("hello ")
    } else {
        io.print("no ")
    }

    -- There is no `else if`! Use `when`:
    when {
        case a > 10: io.print("there ")
        case a < 10: io.print("never ")
        else: io.print("otherwise ")
    }

    -- Sum types (tagged unions)
    union Foo = Str | Int | Bool
    let foo = Foo(a)

    -- `match` is _only_ a type discriminator.
    match foo {
        case Str s: io.print(s)
        case Int i if i < 10: io.print("small")
        case Int i: io.print(i)
        -- `match` has to be exhaustive, i.e. all possible variants of
        -- union type have to be included. In this case we either need
        -- a `case Bool` or `else`.
        else b: io.print(b)
    }
    io.println("!")
}

fun main() void {
    control_flow(42)
}
```

```output
hello there 42!
```

### For Loops

`for` is the only loop in Metall, and it takes three forms. It walks an iterable,
repeats while a condition holds, or, written bare, loops until a `break`. An
iterable is a slice, an array, a range, or any type with a `next` method. The loop
binds each item in turn. Add `, i` to also bind the index, or `&mut x` to change
items in place. `break` and `continue` work in every form.

```metall
use std.io

fun main() void {
    for fruit, i in ["apple", "pear", "plum"][..] {
        io.print(i)
        io.print(" = ")
        io.println(fruit)
    }
}
```

```output
0 = apple
1 = pear
2 = plum
```

A boolean condition makes it a `while`:

```metall
use std.io

fun main() void {
    mut count = 3
    for count > 0 {
        io.println(count)
        count = count - 1
    }
}
```

```output
3
2
1
```

### Ranges

`lo..hi` counts from `lo` up to but not including `hi`. `lo..=hi` includes `hi`.

```metall
use std.io

fun main() void {
    for i in 0..3 {
        io.print(i)
    }
    io.println("")
    for i in 0..=3 {
        io.print(i)
    }
    io.println("")
}
```

```output
012
0123
```

A range is a value, not just loop syntax: name it, pass it, return it, then iterate
it whenever you like.

```metall
use std.io

fun main() void {
    let r = 1..4
    for x in r {
        io.println(x)
    }
}
```

```output
1
2
3
```

### Slice Ranges

Inside `[]`, a range slices a slice or array instead. Either bound may be
left off: a missing lower bound starts at the front, a missing upper bound
runs to the end, and `[..]` is the whole slice.

```metall
use std.io

fun main() void {
    let xs = [10, 20, 30, 40][..]
    io.println(xs[1..3])
    io.println(xs[2..])
    io.println(xs[..2])
    io.println(xs[..])
}
```

```output
[20, 30]
[30, 40]
[10, 20]
[10, 20, 30, 40]
```

## Mutability

Everything in Metall is immutable by default.

## Basic Types

### Enums

An enum is a set of named, integer-backed constants. Variants can carry constant
associated data and methods, and you `match` on them like on a union.

```metall
use std.io

-- Discriminants are auto-assigned from 0 if not set explicitly. 
enum Perm U8 = execute = 1 | write = 2 | read = 4

-- Methods are namespaced functions, just like on a struct.
pub fun Perm.symbol(p Perm) Str {
    match p {
        case Perm.read: "r"
        case Perm.write: "w"
        case Perm.execute: "x"
    }
}

fun main() void {
    io.println(Perm.read.symbol())
    io.println(Perm.execute.symbol())
}
```

```output
r
x
```

Each associated-data field becomes a read-only accessor, and every variant also
gets a generated `debug_name`.

```metall
use std.io

enum Planet(label Str, moons U8) U8 =
    | mercury("Mercury", 0) = 1
    | earth("Earth", 1) = 2
    | mars("Mars", 2) = 3

fun main() void {
    io.println(Planet.mars.label)
    io.println(Planet.mars.moons)
    io.println(Planet.earth.debug_name)
}
```

```output
Mars
2
earth
```

An _open_ enum declares no variants of its own. Subsets add variants and coerce
up to the open root, so a hierarchy can grow across modules.

```metall
use std.io

enum Event U16
enum Mouse Event = press | release | move
enum Key Event = down | up

fun describe(e Event) Str {
    match e {
        -- A bare subset matches all of its variants. A dotted name matches one.
        case Mouse: "mouse event"
        case Key.down: "key down"
        case Key.up: "key up"
        -- A match on an open enum is never exhaustive, so it needs an else.
        else: "other event"
    }
}

fun main() void {
    -- A subset value coerces to the open root.
    let e = Mouse.move
    io.println(describe(e))
    io.println(describe(Key.down))
}
```

```output
mouse event
key down
```

`try e is Mouse` narrows the open enum to the subset, or short-circuits through
the else.

```metall
use std.io

enum Event U16
enum Mouse Event = press | release | move
enum Key Event = down | up

fun on_mouse(e Event) Str {
    let m = try e is Mouse else { return "not a mouse event" }
    -- m has type Mouse here, so the match is over just the subset.
    match m {
        case Mouse.press: "press"
        case Mouse.release: "release"
        case Mouse.move: "move"
    }
}

fun main() void {
    io.println(on_mouse(Mouse.move))
    io.println(on_mouse(Key.down))
}
```

```output
move
not a mouse event
```

`enums.variants<T>()` returns every variant of an enum.

```metall
use std.io
use std.enums

enum Planet(label Str, moons U8) U8 =
    | mercury("Mercury", 0)
    | earth("Earth", 1)
    | mars("Mars", 2)

fun main() void {
    let all = enums.variants<Planet>()
    for i in 0..all.len {
        io.println(all[i].label)
    }
}
```

```output
Mercury
Earth
Mars
```

### Arrays

A fixed-size array `[N]T` stores its `N` elements inline. Its length is part of
the type, which you can let the compiler infer or write out yourself. An array
is not printable on its own. `[..]` views it as a slice.

```metall
use std.io

fun main() void {
    let xs = [1, 2, 3]
    let ys [3]Int = [4, 5, 6]
    io.println(xs[..])
    io.println(ys[0])
}
```

```output
[1, 2, 3]
4
```

`[N of v]` builds one by filling every element with a copy of `v`. The element
type comes from the value, so this is a `[4]U8`.

```metall
use std.io

fun main() void {
    let zeros = [4 of U8(0)]
    io.println(zeros[..])
}
```

```output
[0, 0, 0, 0]
```

Slicing a freshly built array as you define it hands you a mutable slice you
own, so you can fill it in place.

```metall
use std.io

fun main() void {
    mut buf = [3 of Int(7)][..]
    buf[1] = 0
    io.println(buf)
}
```

```output
[7, 0, 7]
```

When every element will be overwritten anyway, `unsafe [N uninit T]` skips the
initialization. The `unsafe` marks that the storage starts undefined.

```metall
use std.io

fun main() void {
    mut scratch = unsafe [3 uninit Int]
    scratch[0] = 10
    scratch[1] = 20
    scratch[2] = 30
    io.println(scratch[..])
}
```

```output
[10, 20, 30]
```

## Strings

A string literal is a `Str`: UTF-8 text. A backslash starts an escape: `\n` is a
newline, `\t` a tab, `\"` a quote, `\\` a backslash, `\xNN` the code point with that
two-hex-digit value (so `\xe4` is `U+00E4`, 'ä'), and `\u{...}` any Unicode code
point by its hex number.

```metall
use std.io

fun main() void {
    io.println("line one\nline two")
    io.println("quote \" backslash \\ umlaut \xe4 (same as \u{E4})")
}
```

```output
line one
line two
quote " backslash \ umlaut ä (same as ä)
```

A byte string, written `b"..."`, is a `[]U8` of raw bytes rather than text. Reach
for it when you are working with binary data. Here `\xNN` is a literal byte rather
than a code point, so `\xe4` is the single byte `228` (in a `Str` it would be the
two UTF-8 bytes of 'ä').

```metall
use std.io

fun main() void {
    let bytes = b"AB\xe4"
    io.println(bytes.len)
    io.println(bytes)
}
```

```output
3
[65, 66, 228]
```

A multi-line string, written `m"..."`, spans several lines and lets you indent the
text to match your code. Two rules make that work:

- The opening `"` ends its line, and the closing `"` sits on its own line. The newline
  after the opening quote, and the whitespace and newline before the closing quote,
  are not part of the string. They are there only so each quote can sit on its own
  line. The string is exactly the lines in between, with no blank line above or below.
- The closing quote marks the left edge. The spaces up to its column are stripped
  from every line, so you indent the block to match your code and that indentation
  drops out, while anything indented past the edge is kept. Every line must reach the
  edge with spaces: a tab there, or text that starts before the edge, is an error.

Below, the same block appears twice, differing only in where the closing quote sits.
The first closes at eight spaces, so eight come off every line and the list items (two
further in) keep those two. The second closes at four, so only four come off and the
whole block stays indented:

```metall
use std.io

fun main() void {
    io.println(m"
        shopping list:
          - apples
          - pears
        ")
    io.println(m"
        shopping list:
          - apples
          - pears
    ")
}
```

```output
shopping list:
  - apples
  - pears
    shopping list:
      - apples
      - pears
```

## Format Strings

An f-string, written `f"..."`, fills its `{...}` placeholders with the values of the
expressions inside them. Each value must be printable: it needs a `fmt` method (the
`HasFmt` shape). Every built-in type has one, and you can add one to your own types.

On its own, an f-string is not yet a string. You choose where its text goes.
`.build(@a)` allocates a new `Str` in arena `@a`. `.write_to(sw)` appends to a
`StrWriter` you already have and allocates nothing.

```metall
use std.io

fun main() void {
    let @a = Arena()
    let name = "Ada"
    let age = 36
    io.println(f"{name} is {age}".build(@a))
    io.println(f"next year: {age + 1}".build(@a))
}
```

```output
Ada is 36
next year: 37
```

For text that contains braces, wrap the f-string in `#` marks, written `f#"..."#`.
Inside, braces are ordinary characters, and you interpolate with `#{...}`.

```metall
use std.io

fun main() void {
    let @a = Arena()
    let n = 3
    io.println(f#"the set {1, 2, 3} has #{n} members"#.build(@a))
}
```

```output
the set {1, 2, 3} has 3 members
```

Just like plain strings, an f-string can be a byte string (`fb"..."`) or multi-line
(`fm"..."`).

```metall
use std.io

fun main() void {
    let @a = Arena()
    let n = 42
    let bytes = fb"n={n}".build(@a)
    io.println(bytes.len)
    io.println(fm"
        name: Ada
        n:    {n}
        ".build(@a))
}
```

```output
4
name: Ada
n:    42
```

Put a `:` after an expression to format it. Each type accepts its own settings:
integers take a `base` with an optional `upper` and `width`, floats take a
`precision` and a `width`, and bools take the word to print for each case.

```metall
use std.io

fun main() void {
    let @a = Arena()
    io.println(f"hex: {255:base=16, upper=true}".build(@a))
    io.println(f"padded: {7:width=3}".build(@a))
    io.println(f"pi: {3.14159:precision=2}".build(@a))
    io.println(f"flag: {true:true_str="yes", false_str="no"}".build(@a))
}
```

```output
hex: FF
padded:   7
pi: 3.14
flag: yes
```

To make your own type accept those settings, give it a `fmt_ext` method whose extra
parameters are the settings. A plain `fmt` is enough for `{x}`. The `{x:...}` form
calls `fmt_ext` instead.

```metall
use std.io

struct Temp { degrees Int }

-- `fmt` makes Temp printable, so it works in any f-string.
pub fun Temp.fmt(t Temp, sw &mut StrWriter) void {
    sw.write(t.degrees)
}

-- `fmt_ext` adds a setting for the `:` form: which unit to append.
pub fun Temp.fmt_ext(t Temp, sw &mut StrWriter, unit Str = "C") void {
    sw.write(t.degrees)
    sw.write(unit)
}

fun main() void {
    let @a = Arena()
    let t = Temp(20)
    io.println(f"plain {t}".build(@a))
    io.println(f"with unit {t:unit="F"}".build(@a))
}
```

```output
plain 20
with unit 20F
```

`.write_to` assembles a string from several pieces without allocating for each one:

```metall
use std.io

fun main() void {
    let @a = Arena()
    let sw = StrWriter.new(64, @a)
    f"x = {1}".write_to(sw)
    f", y = {2}".write_to(sw)
    io.println(sw.as_str())
}
```

```output
x = 1, y = 2
```

## Integer Arithmetic And Optimization Levels

Integer arithmetic is checked by default. `+`, `-`, and `*` panic on overflow, `<<`
and `>>` panic when the shift amount is out of range for the type, and `INT_MIN / -1`
panics. When you want wraparound, ask for it: the `+%`, `-%`, and `*%` operators wrap.

```metall
use std.io

fun main() void {
    -- A checked `+` would panic here. `+%` wraps on purpose.
    io.println(U8(255) +% U8(1))
}
```

```output
0
```

The compiler's `--opt` flag selects how aggressively the compiler optimizes
and whether those arithmetic checks survive:

- `none`: checks on, no optimization. The development default.
- `safe`: checks on, full optimization. Same behavior as `none`, faster code.
- `fast`: full optimization, and the overflow, shift-range, and `INT_MIN / -1`
  checks are stripped. A plain `+` that overflows now wraps like `+%`, and `1 << 70`
  produces an undefined value instead of panicking. Bounds checks and the
  divide-by-zero check stay on at every level.

So the same source line can panic under `none`/`safe` and silently wrap under `fast`.
Reach for `fast` only once you have measured the need, and write the explicit `+%`
family wherever wraparound is intended, so the behavior does not depend on the
optimization level.

## Terminology

### Generic Type Terms

**Generic declaration**

A declaration with type parameters. Generic structs, unions, shapes, and
functions can be instantiated with different type arguments.

```metall
struct Box<T> {
    value T
}

fun main() void {
    DebugIntern.print_int(Box<Int>(42).value)
}
```

```output
42
```

**Type parameter**

A name declared by a generic declaration. In `Box<T>`, `T` is a type parameter.

**Type argument**

A type supplied for a type parameter. In `Box<Int>`, `Int` is a type argument.

**Type constraint**

A requirement on a type parameter. In `T HasValue`, `HasValue` is a type
constraint on `T`. Other languages often call this a bound.

```metall
shape HasValue {
    -- A type conforms to HasValue by providing this method.
    fun HasValue.value(x HasValue) Int
}

struct Score {
    n Int
}

-- Score conforms to HasValue because this method matches the shape.
fun Score.value(s Score) Int {
    s.n
}

fun print_value<T HasValue>(x T) void {
    DebugIntern.print_int(x.value())
}

fun main() void {
    print_value(Score(7))
}
```

```output
7
```

**Phantom type parameter**

A type parameter that is not used by the runtime representation of a type.

```metall
struct Open {}
struct Closed {}

struct File<State> {
    fd Int
}

fun open_file() File<Open> {
    File<Open>(7)
}

fun read_open(f File<Open>) Int {
    f.fd
}

fun close(f File<Open>) File<Closed> {
    File<Closed>(f.fd)
}

fun main() void {
    let open = open_file()
    DebugIntern.print_int(read_open(open))
    let closed = close(open)
    _ = closed
}
```

```output
7
```

`File<Open>` and `File<Closed>` have the same runtime representation, but they
are different types. This is useful for state, capabilities, brands, units, and
typed handles.

### Shape And Associated Type Terms

**Shape**

A contract describing what a type must provide.

**Associated type**

A type member of a shape. Metall declares a shape's associated types in the
shape parameter list.

```metall
shape Source<Item> {
    -- Item is fixed by the return type of the required next method.
    fun Source.next(s &mut Source) ?Item
}

struct Counter {
    current Int
}

-- Counter conforms to Source, and this method fixes Counter.Item to Int.
fun Counter.next(c &mut Counter) ?Int {
    let value = c.current
    c.current = c.current + 1
    value
}

fun first<S Source>(s &mut S) ?S.Item {
    s.next()
}

fun main() void {
    mut c = Counter(5)
    match first(&mut c) {
        case None: DebugIntern.print_int(0)
        else value: DebugIntern.print_int(value)
    }
}
```

```output
5
```

Here `Item` is an associated type of `Source`.

Not every type parameter is an associated type. `Box<T>` declares a generic
type parameter. `Source<Item>` declares an associated type because `Item` can be
projected from an unknown type that satisfies `Source`, as in `S.Item`.

**Associated type projection**

An associated type selected through a constrained type parameter. If `In` is
constrained by `Source`, `In.Item` projects the `Item` associated type from
`In`.

```metall
shape Source<Item> {
    -- Item is the element type produced by a conforming source.
    fun Source.next(s &mut Source) ?Item
}

struct Once<Item> {
    value ?Item
}

-- Once<Item> conforms to Source, with Source.Item equal to its own Item.
fun Once.next(o &mut Once) ?Item {
    let value = o.value
    o.value = None()
    value
}

struct MapSource<In Source, Out> {
    source In
    -- In.Item is the associated type projected from the Source constraint.
    mapper fun(In.Item) Out
}

fun MapSource.next(m &mut MapSource) ?Out {
    match (&mut m.source).next() {
        case None: None()
        else value: m.mapper(value)
    }
}

fun double(x Int) Int {
    x * 2
}

fun main() void {
    mut source = MapSource(Once(Option(21)), double)
    match (&mut source).next() {
        case None: DebugIntern.print_int(0)
        else value: DebugIntern.print_int(value)
    }
}
```

```output
42
```

`In Source` means that `In` satisfies `Source<In.Item>`. The slot name is part
of the public shape API: renaming `Item` changes code that writes `In.Item`.

Associated type projections can be used in generic data declarations too:

```metall
shape Boxer<Inner> {
    -- Inner is fixed by the return type of the required inner method.
    fun Boxer.inner(b Boxer) Inner
}

struct IntBoxer {
    value Int
}

-- IntBoxer conforms to Boxer, and this method fixes IntBoxer.Inner to Int.
fun IntBoxer.inner(b IntBoxer) Int {
    b.value
}

struct Box<T Boxer> {
    -- T.Inner belongs to the Boxer constraint on T, not to Box itself.
    boxer T.Inner
}

fun main() void {
    DebugIntern.print_int(Box<IntBoxer>(9).boxer)
}
```

```output
9
```

Here `Inner` is still an associated type of `Boxer`, not of `Box`. `Box<T>`
uses the projection `T.Inner` because its type parameter `T` is constrained by
`Boxer`.

**Unconstrained associated type**

An associated type that is not fixed by the shape contract and has no default.
Unconstrained associated types are rejected.

```metall
shape Marker<Item> {}

fun main() void {}
```

```error
test.met:1:14: open shape slot: Item is not used by the shape contract
    shape Marker<Item> {}
                 ^^^^
```

`Item` has nowhere to come from. A concrete `Token` could satisfy
`Marker<Int>`, `Marker<Str>`, or any other `Marker<X>` equally, so `Item` is not
associated with `Token` in a useful way.

### Compiler Terms

**Template**

The compiler-internal form of a generic declaration before concrete type
arguments are known.

**Monomorphization**

The compiler strategy of generating concrete code for each required
instantiation of a generic declaration.

**Monomorphizer**

The compiler pass that performs monomorphization.

**Instantiation**

The act of applying type arguments to a generic declaration, e.g. `Box<Int>`.

**Synthetic type parameter**

A compiler-created type parameter. The monomorphizer uses synthetic type
parameters internally to represent associated type projections such as
`In.Item` while it checks and instantiates templates.

## Parameterized Types ("Generics")

TODO

## Shapes

TODO

## Errors

## Optionals

## Modules

## TODO's

Things to document:

- Union autwrap wraps into the first matching variant. In general, there is no ambiguity because variants - like all types - are nominally typed.
  The only thing to watch out for would be mutability: `union Foo = &Int | &mut Int` would wrap a `&mut Int` into `&Int` because it comes first.
