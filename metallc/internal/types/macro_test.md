# Macro Tests

**setup macro modules**

```metall
```

```module.hello_macro
fun apply(sb &mut StrBuilder, @a Arena) void {
    sb.str("fun greet() void { DebugIntern.print_str(").rune('"').str("hi").rune('"').str(") }")
}
```

```module.no_macro_funs_macro
fun helper(x Int) Int { x }
```

```module.param_macro
fun apply(n Int, sb &mut StrBuilder, @a Arena) void {
    sb.str("fun value() Int { ").int(n).str(" }")
}
```

## Errors

**macro module without macro functions**

```metall
use local.no_macro_funs_macro

fun main() void {}
```

```error
local/no_macro_funs_macro.met:1:1: macro modules must contain at least one macro function
    fun helper(x Int) Int { x }
    ^^^^^^^^^^^^^^^^^^^^^^^^^^^
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
use local.hello_macro

hello_macro.apply()

fun main() void {}
```

```expander_error
compilation failed
```

```error
test.met:3:1: macro expansion failed: compilation failed
    
    hello_macro.apply()
    ^^^^^^^^^^^^^^^^^^^
```

**non-literal macro argument**

```metall
use local.param_macro

fun main() void {
    let x = 1
    param_macro.apply(x)
}
```

```expander
```

```error
test.met:5:23: macro arguments must be compile-time constants
        let x = 1
        param_macro.apply(x)
                          ^
    }
```

**macro expansion producing invalid code**

```metall
use local.hello_macro

hello_macro.apply()

fun main() void {}
```

```expander
this is not valid metall code }{][
```

```error
local/hello_macro.met.expanded:1:6: unexpected token: <is>
    this is not valid metall code }{][
         ^^
```

## Success

**macro expands function**

```metall
use local.hello_macro

hello_macro.apply()

fun main() void {
    greet()
}
```

```expander
fun greet() void { DebugIntern.print_str("hi") }
```
