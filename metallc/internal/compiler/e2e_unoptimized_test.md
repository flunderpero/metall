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
    _ = @a.slice<U8>(1000000, U8(0))
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
    _ = @a.slice<U8>(1000000, U8(0))
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

**defer and arena cleanup with try early return**

```metall
enum UnicodeErr Err = malformed_utf8

fun fallible(fail Bool) !Int {
    if fail { return UnicodeErr.malformed_utf8 }
    Result(42)
}

fun test(fail Bool) !void {
    let @a = Arena()
    let x = @a.new<Int>(try fallible(fail))
    defer { DebugIntern.print_int(x.*) }
    DebugIntern.print_str("ok")
    Result(void)
}

fun main() void {
    _ = test(false)
    _ = test(true)
    DebugIntern.print_str("done")
}
```

```output
ok
42
done
```

## FFI

These extern (C) tests live in the unoptimized suite on purpose. The default
pass pipeline folds a call like `abs(-42)` to the literal `42` (LLVM knows the
`int abs(int)` libc prototype), so the call is never emitted and the FFI path
goes untested. Running with only `verify` keeps the real call. The wasm build
links `-nostdlib`, so the harness supplies `abs`. The narrow-int C-ABI tests
are `!wasm` because that sign/zero extension is a native ABI concern.

**call extern C function**

```metall
extern fun abs(n I32) I32

fun main() void {
    let x = unsafe abs(-42)
    DebugIntern.print_int(x.to_int())
}
```

```output
42
```

**extern U8 argument above 127 keeps its value**

```metall !wasm
extern("abs") fun abs_u8(n U8) I32

fun main() void {
    -- abs leaves a positive value unchanged, so the printed result is whatever
    -- value `abs` actually received. A U8 widened as signed turns 200 into -56.
    let lo = unsafe abs_u8(100)
    let hi = unsafe abs_u8(200)
    DebugIntern.print_int(lo.to_int())
    DebugIntern.print_int(hi.to_int())
}
```

```output
100
200
```

**a function value keeps the extern C ABI when chosen at runtime**

```metall !wasm
extern("abs") fun abs_u8(n U8) I32

-- unsync + unsafe so its type matches the extern's (externs are unsafe and not
-- sync), letting both flow through the same runtime-selected function value.
unsync unsafe fun plus_one(n U8) I32 {
    n.to_i32() + 1
}

-- `f` is the extern or a regular function, decided by a runtime bool, so the
-- C-ABI handling has to ride along with the chosen function value. abs(200) is
-- 200 and plus_one(200) is 201; a sign-extended U8 reaching abs would print 56.
fun run(use_extern Bool, n U8) I32 {
    let f = if use_extern { abs_u8 } else { plus_one }
    unsafe f(n)
}

fun main() void {
    DebugIntern.print_int(run(true, 200).to_int())
    DebugIntern.print_int(run(false, 200).to_int())
}
```

```output
200
201
```

**ffi strlen via slice_ptr**

```metall !wasm
use std.ffi

extern fun strlen(s ffi.Ptr<U8>) Int

fun main() void {
    let text = [U8(65), 65, 65, 0][..]
    let ptr = ffi.slice_ptr<U8>(text)
    let len = unsafe strlen(ptr)
    DebugIntern.print_int(len)
}
```

```output
3
```

**call extern function from imported module and main module**

```metall
use local.e2e_ffi

extern fun abs(n I32) I32

fun main() void {
    let x = unsafe e2e_ffi.abs(I32(-7))
    DebugIntern.print_int(x.to_int())
    let y = unsafe abs(I32(-42))
    DebugIntern.print_int(y.to_int())
}
```

```output
7
42
```

**extern functions don't pollute the root ns**

```metall
use local.e2e_ffi

fun abs(n Int) Int {
    if n < 0 { n * -1 } else { n }
}

fun main() void {
    let x = unsafe e2e_ffi.abs(I32(-7))
    DebugIntern.print_int(x.to_int())
    let y = abs(-42)
    DebugIntern.print_int(y)
}
```

```output
7
42
```

**ffi is_null with C function returning null**

```metall !wasm
use std.ffi

extern fun strchr(s ffi.Ptr<U8>, c I32) ffi.Ptr<U8>

fun main() void {
    let haystack = ffi.slice_ptr<U8>([U8('h'), 'i', 0][..])
    let not_found = unsafe strchr(haystack, U8('z').to_i32())
    DebugIntern.print_bool(not_found.is_null())
    let found = unsafe strchr(haystack, U8('h').to_i32())
    DebugIntern.print_bool(found.is_null())
}
```

```output
true
false
```

**extern function alias**

```metall
extern("abs") fun my_abs(n I32) I32

-- We can use abs as a name because the extern `abs` is aliased to `my_abs`.
fun abs(n I32) I32 {
    unsafe my_abs(n)
}

fun main() void {
    let x = unsafe my_abs(I32(-42))
    DebugIntern.print_int(x.to_int())
    let y = abs(-137)
    DebugIntern.print_int(y.to_int())
}
```

```output
42
137
```
