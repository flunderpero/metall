# Error return trace tests

All tests in this file run with `ErrorTracing` enabled. They pin the behavior of
the automatic error return trace, including its limitations.

## Basics

**error propagates through a try chain to main**

A fresh error is returned three frames deep and propagates via `try` up to
`main`, which prints `failed: <name>` followed by the recorded trace, deepest
frame first.

```metall
enum AppErr Err = not_found

fun deep() !Int {
    return AppErr.not_found
}

fun middle() !Int {
    try deep()
}

fun top() !Int {
    let n = try middle()
    n
}

fun main() !void {
    let n = try top()
    DebugIntern.print_int(n)
}
```

```panic
failed: test.AppErr.not_found
error trace:
  test.met:4:5 in test.deep
  test.met:8:5 in test.middle
  test.met:12:13 in test.top
  test.met:17:13 in test.main
```

**successful error-union chain records nothing**

When no error is returned, the happy path produces no trace and exits cleanly.

```metall
enum AppErr Err = not_found

fun deep(fail Bool) !Int {
    if fail { return AppErr.not_found }
    7
}

fun main() !void {
    let n = try deep(false)
    DebugIntern.print_int(n)
}
```

```output
7
```

## Nested error carriers

**error from a `?!T` (Option<Result<T>>) return is traced precisely**

A `?!Int` producer returns an error (an explicit `Result` that wraps into the
`Option`). The recording drills through the `Option` then the `Result` to find
the `Err` discriminant, so the producer frame appears in the trace.

```metall
enum AppErr Err = boom

fun produce() ?!Int {
    return Result<Int>(AppErr.boom)
}

fun consume() !Int {
    match produce() {
        case None: return AppErr.boom
        else r: try r
    }
}

fun main() !void {
    let n = try consume()
    DebugIntern.print_int(n)
}
```

```panic
failed: test.AppErr.boom
error trace:
  test.met:4:5 in test.produce
  test.met:10:17 in test.consume
  test.met:15:13 in test.main
```

## Printing the trace yourself

**errors.dump() prints a caught error's trace without main failing**

A handler that catches a propagating error can print the trace on demand via the
public `errors.dump()`. Here `main` recovers and exits successfully.

```metall
use std.errors

enum AppErr Err = boom

fun deep() !Int {
    return AppErr.boom
}

fun caught() Int {
    try deep() else {
        errors.dump()
        return -1
    }
}

fun main() void {
    DebugIntern.print_int(caught())
}
```

```output
error trace:
  test.met:6:5 in test.deep
-1
```

## Reset between error chains

A freshly originated error resets the buffer (`origin`), so each error owns
exactly its own frames and an unrelated earlier error cannot leak in. Errors
propagated by `try` append (`record`).

**two independent caught errors each show only their own trace**

Two unrelated errors are each caught and dumped in their own helper. The second
error's `origin` reset clears the first, so each dump shows just its own frame.

```metall
use std.errors

enum AppErr Err = a | b

fun do_first() !Int {
    return AppErr.a
}

fun first() Int {
    try (do_first()) else {
        errors.dump()
        return -1
    }
}

fun do_second() !Int {
    return AppErr.b
}

fun second() Int {
    try (do_second()) else {
        errors.dump()
        return -2
    }
}

fun main() void {
    _ = first()
    _ = second()
}
```

```output
error trace:
  test.met:6:5 in test.do_first
error trace:
  test.met:17:5 in test.do_second
```

**a swallowed error does not leak into a later, unrelated fatal trace**

`caught` swallows its error (no dump, recovers). A genuinely fatal, unrelated
error later reaches `main`; its `origin` reset discards the swallowed error, so
the fatal trace shows only `bad` and `main`.

```metall
enum AppErr Err = a | b

fun do_a() !Int {
    return AppErr.a
}

fun caught() Int {
    try (do_a()) else {
        return -1
    }
}

fun bad() !Int {
    return AppErr.b
}

fun main() !void {
    _ = caught()
    let n = try bad()
    DebugIntern.print_int(n)
}
```

```panic
failed: test.AppErr.b
error trace:
  test.met:14:5 in test.bad
  test.met:19:13 in test.main
```

**re-raising a different error reports the new origin, not the caught one**

`middle` catches `deep`'s error and raises a fresh, different error. The
re-raise is a new `origin`, so the trace shows `middle` (where the new error was
born) and `main`, not `deep`.

```metall
enum AppErr Err = first | second

fun deep() !Int {
    return AppErr.first
}

fun middle() !Int {
    try deep() else {
        return AppErr.second
    }
}

fun main() !void {
    let n = try middle()
    DebugIntern.print_int(n)
}
```

```panic
failed: test.AppErr.second
error trace:
  test.met:9:9 in test.middle
  test.met:14:13 in test.main
```

**errors.dump() is re-printable, not draining**

Calling `errors.dump()` twice prints the same block twice: dump only reads the
buffer, it never consumes it.

```metall
use std.errors

enum AppErr Err = boom

fun deep() !Int {
    return AppErr.boom
}

fun caught() Int {
    try (deep()) else {
        errors.dump()
        errors.dump()
        return -1
    }
}

fun main() void {
    DebugIntern.print_int(caught())
}
```

```output
error trace:
  test.met:6:5 in test.deep
error trace:
  test.met:6:5 in test.deep
-1
```

## Truncation

**deep recursion truncates at 32 frames and reports how many were dropped**

A 40-deep recursive chain records 42 frames but only 32 are kept. Truncation
keeps the earliest frames (the origin), drops the latest — including `main` —
and notes the elided count (`count` keeps counting past the cap).

```metall
enum AppErr Err = boom

fun rec(n Int) !Int {
    if n == 0 { return AppErr.boom }
    let r = try rec(n - 1)
    r
}

fun main() !void {
    let n = try rec(40)
    DebugIntern.print_int(n)
}
```

```panic
failed: test.AppErr.boom
error trace:
  test.met:4:17 in test.rec
  test.met:5:13 in test.rec
  test.met:5:13 in test.rec
  test.met:5:13 in test.rec
  test.met:5:13 in test.rec
  test.met:5:13 in test.rec
  test.met:5:13 in test.rec
  test.met:5:13 in test.rec
  test.met:5:13 in test.rec
  test.met:5:13 in test.rec
  test.met:5:13 in test.rec
  test.met:5:13 in test.rec
  test.met:5:13 in test.rec
  test.met:5:13 in test.rec
  test.met:5:13 in test.rec
  test.met:5:13 in test.rec
  test.met:5:13 in test.rec
  test.met:5:13 in test.rec
  test.met:5:13 in test.rec
  test.met:5:13 in test.rec
  test.met:5:13 in test.rec
  test.met:5:13 in test.rec
  test.met:5:13 in test.rec
  test.met:5:13 in test.rec
  test.met:5:13 in test.rec
  test.met:5:13 in test.rec
  test.met:5:13 in test.rec
  test.met:5:13 in test.rec
  test.met:5:13 in test.rec
  test.met:5:13 in test.rec
  test.met:5:13 in test.rec
  test.met:5:13 in test.rec
  ... 10 more frames elided
```

## Recorded location

**a tail-yielded error records the function's return, the whole match**

Recording happens where an error leaves the function. When an arm yields a bare
error as the function's tail expression, the error leaves via the function's
implicit return of the whole `match`, so that is the recorded location, not the
arm. (Errors that leave via `return` or `try` are recorded at that statement, so
those stay per-arm precise.)

```metall
enum Sel U8 = a | b
enum AppErr Err = boom

fun pick(s Sel) !Int {
    match s {
        case Sel.a: AppErr.boom
        case Sel.b: 7
    }
}

fun main() !void {
    let n = try pick(Sel.a)
    DebugIntern.print_int(n)
}
```

```panic
failed: test.AppErr.boom
error trace:
  test.met:5:5 in test.pick
  test.met:12:13 in test.main
```

**origin is where an error is returned, not where it is constructed**

An error value is built, a second unused error is built, and the first is
returned later. Only the return site is recorded; constructed-but-unreturned
errors leave no trace.

```metall
enum AppErr Err = boom

fun produce() !Int {
    let e = AppErr.boom
    let other = AppErr.boom
    return e
}

fun main() !void {
    let n = try produce()
    DebugIntern.print_int(n)
}
```

```panic
failed: test.AppErr.boom
error trace:
  test.met:6:5 in test.produce
  test.met:10:13 in test.main
```

**a bare tail call forwards the callee's error (propagation, not a new origin)**

`forward`'s body is a bare call with no `try`, so it forwards `deep`'s error.
The forward is classified as propagation (a tail call), so `deep`'s origin is
kept rather than reset.

```metall
enum AppErr Err = boom

fun deep() !Int {
    return AppErr.boom
}

fun forward() !Int {
    deep()
}

fun main() !void {
    let n = try forward()
    DebugIntern.print_int(n)
}
```

```panic
failed: test.AppErr.boom
error trace:
  test.met:4:5 in test.deep
  test.met:8:5 in test.forward
  test.met:12:13 in test.main
```

## Threads

**a spawned thread's error trace survives the join**

A thread fails; its trace is snapshotted before it exits and restored into the
joiner, so the propagated error shows the worker's origin, not just the parent
side of the join.

```metall !wasm
use std.thread
use std.errors

enum WorkErr Err = boom

fun work() !Int {
    return WorkErr.boom
}

fun main() !void {
    let @a = Arena()
    let t = try thread.spawn(@a, fun() !Int { try work() })
    let n = try (try t.join())
    DebugIntern.print_int(n)
}
```

```panic
failed: test.WorkErr.boom
error trace:
  test.met:7:5 in test.work
  test.met:12:47 in test.main.__fun_lit_0
  test.met:13:13 in test.main
```

**a successful thread leaves no trace**

```metall !wasm
use std.thread
use std.errors

fun work() !Int {
    7
}

fun main() !void {
    let @a = Arena()
    let t = try thread.spawn(@a, fun() !Int { try work() })
    let n = try (try t.join())
    DebugIntern.print_int(n)
}
```

```output
7
```

**spawn used as recovery: the worker's failure replaces the recovered error**

`attempt` recovers from `primary_work`'s failure by spawning a worker, which
itself fails. The restored worker trace replaces the (already handled) primary
error, so the final trace shows the worker's origin, not `primary_work`.

```metall !wasm
use std.thread
use std.errors

enum AppErr Err = primary | worker

fun primary_work() !Int {
    return AppErr.primary
}

fun worker_work() !Int {
    return AppErr.worker
}

fun attempt(@a Arena) !Int {
    try primary_work() else {
        let t = try thread.spawn(@a, fun() !Int { try worker_work() })
        return try (try t.join())
    }
}

fun main() !void {
    let @a = Arena()
    let n = try attempt(@a)
    DebugIntern.print_int(n)
}
```

```panic
failed: test.AppErr.worker
error trace:
  test.met:11:5 in test.worker_work
  test.met:16:51 in test.attempt.__fun_lit_0
  test.met:17:16 in test.attempt
  test.met:23:13 in test.main
```
