# E2E Unoptimized Tests

Tests compiled without optimization passes (only `verify`), to catch
codegen bugs that optimizers might mask (e.g. dead code elimination
hiding a double-free).

## Arena Cleanup

Arena destroy calls must be emitted exactly once per arena, regardless
of how control exits a scope (fallthrough, return, break, continue).

**return destroys arenas from enclosing scopes**

```metall
fun foo() Int {
    let @a = Arena()
    if true {
        let @b = Arena()
        let x = @b.new<Int>(42)
        return x.*
    }
    let x = @a.new<Int>(137)
    x.*
}

fun main() void {
    DebugIntern.print_int(foo())
}
```

```output
42
```

**break and continue destroy loop-local arenas**

```metall
fun main() void {
    mut sum = 0
    for i in 0..10 {
        let @a = Arena()
        let x = @a.new<Int>(i)
        if x.* == 5 {
            continue
        }
        if x.* == 8 {
            break
        }
        sum = sum + x.*
    }
    DebugIntern.print_int(sum)
}
```

```output
23
```

**break does not double-free enclosing arena**

The arena is allocated in the function scope, outside the loop.
`break` must only destroy arenas from the loop body scope, not
from the enclosing function scope (which is cleaned up when the
function block exits).

The 1MB allocation forces the arena to overflow its inline stack
buffer and malloc a heap page, which makes ASAN detect the
double-free.

```metall
fun test() void {
    let @a = Arena()
    _ = @a.slice_mut<U8>(1000000, U8(0))
    mut i = 0
    for i < 1 {
        i = i + 1
        if true {
            break
        }
    }
}

fun main() void {
    test()
    DebugIntern.print_str("ok")
}
```

```output
ok
```

**continue does not double-free enclosing arena**

```metall
fun test() void {
    let @a = Arena()
    _ = @a.slice_mut<U8>(1000000, U8(0))
    for i in 0..3 {
        if true {
            continue
        }
    }
}

fun main() void {
    test()
    DebugIntern.print_str("ok")
}
```

```output
ok
```
