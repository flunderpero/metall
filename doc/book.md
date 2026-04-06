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
use std::io

fun main() void {
    let @main = Arena()
    let bytes = { 
        let @inner = Arena()

        -- `i` is bound to the scope of `@inner` and cannot outlive it.
        -- We could not yield `i` from this block.
        let i = @inner.new<U64>(137)
        -- But we can pass it to functions. Yes, that's Zig-style deref.
        io::println(i.*)
        
        -- This allocation outlives the current scope up to scope of `@main`.
        @main.slice_mut(3, 65)
    }
    io::println(bytes)
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
use std::io

fun control_flow(a Int) void {

    -- if/else:
    if a > 5 {
        io::print("hello ")
    } else {
        io::print("no ")
    }

    -- There is no `else if`! Use `when`:
    when {
        case a > 10: io::print("there ")
        case a < 10: io::print("never ")
        else: io::print("otherwise ")
    }

    -- Sum types (tagged unions)
    union Foo = Str | Int | Bool
    let foo = Foo(a)

    -- `match` is _only_ a type discriminator.
    match foo {
        case Str s: io::print(s)
        case Int i if i < 10: io::print("small")
        case Int i: io::print(i)
        -- `match` has to be exhaustive, i.e. all possible variants of
        -- union type have to be included. In this case we either need
        -- a `case Bool` or `else`.
        else b: io::print(b)
    }
    io::println("!")
}

fun main() void {
    control_flow(42)
}
```

```output
hello there 42!
```

## Mutability

Everything in Metall is immutable by default.

## Basic Types

## Parameterized Types ("Generics")

## Shapes

## Errors

## Optionals

## Modules
