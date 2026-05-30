# Metall - Systems Programming For Humans

Ok, that claim is a bit silly in today's day and age, but it perfectly sums
up the main goal __I__ had while designing and developing the language:

**I want to have fun in a systems programming language.**

"Fun" is very personal and to each their own. 

## TL;DR

If you don't read any further (hi, mr. tsoding), please read this.

### Goals And Non-Goals

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

## Enums

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

## Mutability

Everything in Metall is immutable by default.

## Basic Types

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
