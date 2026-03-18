# Macro Tests

**setup macro modules**

```metall
```

```module.hello_macro
fun apply(sb &mut StrBuilder, @a Arena) void {
    sb.str("fun greet() void { DebugIntern.print_str(").rune('"').str("hi").rune('"').str(") }")
}
```

```module.bad_macro
fun apply(sb &mut StrBuilder) void {
    sb.str("nope")
}
```

```module.multi_decl_macro
struct Oops { x Int }
fun apply(sb &mut StrBuilder, @a Arena) void {
    sb.str("nope")
}
```

```module.no_apply_macro
fun something(sb &mut StrBuilder, @a Arena) void {
    sb.str("nope")
}
```

## Errors

**macro module without apply function**

```metall
use local::no_apply_macro

fun main() void {}
```

```error
local/no_apply_macro.met:1:1: macro modules must contain an `apply` function
    fun something(sb &mut StrBuilder, @a Arena) void {
    ^
        sb.str("nope")
    }
    ^
```

**macro module with non-apply declaration**

```metall
use local::multi_decl_macro

fun main() void {}
```

```error
```

**macro module missing sb and @a params**

```metall
use local::bad_macro

fun main() void {}
```

```error
local/bad_macro.met:1:1: macro `apply` must have at least `sb &mut StrBuilder` and `@a Arena` parameters
    fun apply(sb &mut StrBuilder) void {
    ^
        sb.str("nope")
    }
    ^
```

**non-macro call at top level**

```metall
fun foo() void {}

foo()

fun main() void {}
```

```error
test.met:3:1: only macro calls are allowed at the top level
    
    foo()
    ^^^^^
```

**macro expansion failure**

```metall
use local::hello_macro

hello_macro::apply()

fun main() void {}
```

```expander_error
compilation failed
```

```error
test.met:3:1: macro expansion failed: compilation failed
    
    hello_macro::apply()
    ^^^^^^^^^^^^^^^^^^^^
```

## Success

**macro expands function**

```metall
use local::hello_macro

hello_macro::apply()

fun main() void {
    greet()
}
```

```expander
fun greet() void { DebugIntern.print_str("hi") }
```
