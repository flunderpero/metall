# Lifetime Tests

**stack ref escapes**

```metall
let x = {
    let y = 123
    &y
}
```

```error
test.met:3:5: reference escaping its allocation scope (via block result)
        let y = 123
        &y
        ^^
    }
```

**assign ref to outer**

```metall
{
    mut x = 123
    mut y = &x
    {
        mut z = 123
        y = &z
    }
}
```

```error
test.met:6:13: reference escaping its allocation scope (via mutation of outer variable)
            mut z = 123
            y = &z
                ^^
        }
```

**nested block escape**

```metall
{
    mut x = 123
    mut y = &x
    {
        mut z = 123
        {
            y = &z
        }
    }
}
```

```error
test.met:7:17: reference escaping its allocation scope (via mutation of outer variable)
            {
                y = &z
                    ^^
            }
```

**deref assign escapes**

```metall
{
    mut x = 123
    mut y = &x
    mut z = &mut y
    {
      mut w = 456
      z.* = &w
    }
}
```

```error
test.met:7:13: reference escaping its allocation scope (via deref assignment)
          mut w = 456
          z.* = &w
                ^^
        }
```

**deref assign multi-level escape**

```metall
{
    mut x = 123
    mut y = &x
    {
        mut z = 456
        mut w = &mut y
        {
            mut v = 789
            w.* = &v
        }
    }
}
```

```error
test.met:9:19: reference escaping its allocation scope (via deref assignment)
                mut v = 789
                w.* = &v
                      ^^
            }
```

**deref assign escapes from a nested block below the taint scope**

```metall
{
    mut sink = &0
    let pp = &mut sink
    {
        mut local = 777
        { pp.* = &local }
    }
}
```

```error
test.met:6:18: reference escaping its allocation scope (via deref assignment)
            mut local = 777
            { pp.* = &local }
                     ^^^^^^
        }
```

**valid same scope ref**

```metall
{
    mut x = 123
    mut y = &x
    mut z = 456
    y = &z
}
```

```error
```

**valid outer ref to inner**

```metall
{
    mut x = 123
    let r = {
        mut y = &x
        y
    }
}
```

```error
```

**when branch escapes to outer**

```metall
{
    mut x = 123
    mut y = &x
    when {
        case true:
            mut z = 456
            y = &z
        else:
    }
}
```

```error
test.met:7:17: reference escaping its allocation scope (via mutation of outer variable)
                mut z = 456
                y = &z
                    ^^
            else:
```

**when branch merge no escape**

```metall
{
    mut x = 123
    mut y = &x
    mut z = 456
    when {
        case true:
            y = &z
        else:
    }
}
```

```error
```

**deref assign through ref chain escapes**

```metall
{
    mut x = 123
    mut y = &x
    mut z = &mut y
    {
        mut w = 456
        z.* = &w
    }
}
```

```error
test.met:7:15: reference escaping its allocation scope (via deref assignment)
            mut w = 456
            z.* = &w
                  ^^
        }
```

**field write escapes**

```metall
{
    struct Foo { one &Int }
    mut x = 123
    mut y = Foo(&x)
    {
        mut z = 456
        y.one = &z
    }
}
```

```error
test.met:7:17: reference escaping its allocation scope (via mutation of outer variable)
            mut z = 456
            y.one = &z
                    ^^
        }
```

**valid field write**

```metall
{
    struct Foo { one &Int }
    mut x = 123
    mut y = 456
    mut z = Foo(&x)
    z.one = &y
}
```

```error
```

**return ref to local**

```metall
{
    fun foo() &Int {
        mut x = 42
        &x
    }
}
```

```error
test.met:4:9: reference escaping its allocation scope (via block result)
            mut x = 42
            &x
            ^^
        }
```

**return expr ref to local**

```metall
{
    fun foo() &Int {
        mut x = 42
        return &x
    }
}
```

```error
test.met:4:16: reference escaping its allocation scope (via return)
            mut x = 42
            return &x
                   ^^
        }
```

**escape via return in if branch**

```metall
{
    fun foo(a &Int) &Int {
        mut x = 42
        if true { return &x }
        a
    }
}
```

```error
test.met:4:26: reference escaping its allocation scope (via return)
            mut x = 42
            if true { return &x }
                             ^^
            a
```

**ref of field escapes**

```metall
{
    struct Foo { one Int }
    let y = {
        let x = Foo(42)
        &x.one
    }
}
```

```error
test.met:5:9: reference escaping its allocation scope (via block result)
            let x = Foo(42)
            &x.one
            ^^^^^^
        }
```

**ref of nested field escapes**

```metall
{
    struct Bar { one Int }
    struct Foo { bar Bar }
    let y = {
        let x = Foo(Bar(1))
        &x.bar.one
    }
}
```

```error
test.met:6:9: reference escaping its allocation scope (via block result)
            let x = Foo(Bar(1))
            &x.bar.one
            ^^^^^^^^^^
        }
```

**ref of index escapes**

```metall
{
    let y = {
        let @a = Arena()
        let x = @a.slice<Int>(5, 0)
        &x[0]
    }
}
```

```error
test.met:5:9: reference escaping its allocation scope (via block result)
            let x = @a.slice<Int>(5, 0)
            &x[0]
            ^^^^^
        }
```

**deref on rhs escapes**

```metall
{
    mut x = 0
    mut y = &x
    {
        mut z = 456
        mut w = &z
        mut v = &w
        y = v.*
    }
}
```

```error
test.met:6:17: reference escaping its allocation scope (via mutation of outer variable)
            mut z = 456
            mut w = &z
                    ^^
            mut v = &w
```

**call returns ref to local**

```metall
{
    fun identity(a &Int) &Int { a }
    let x = {
        let y = 42
        identity(&y)
    }
    x
}
```

```error
test.met:5:18: reference escaping its allocation scope (via block result)
            let y = 42
            identity(&y)
                     ^^
        }
```

**transitive call returns ref to local**

```metall
{
    fun identity(a &Int) &Int { a }
    fun foo(a &Int) &Int { identity(a) }
    let x = {
        let y = 42
        foo(&y)
    }
    x
}
```

```error
test.met:6:13: reference escaping its allocation scope (via block result)
            let y = 42
            foo(&y)
                ^^
        }
```

**call returns struct with ref to local**

```metall
{
    struct Wrapper { one &Int }
    fun foo(a &Int) Wrapper { Wrapper(a) }
    let x = {
        let y = 42
        foo(&y)
    }
    x
}
```

```error
test.met:6:13: reference escaping its allocation scope (via block result)
            let y = 42
            foo(&y)
                ^^
        }
```

**nested struct construction ref escapes**

```metall
{
    struct Foo { one &Int }
    struct Bar { one Foo }
    let x = {
        let y = 42
        Bar(Foo(&y))
    }
    x
}
```

```error
test.met:6:17: reference escaping its allocation scope (via block result)
            let y = 42
            Bar(Foo(&y))
                    ^^
        }
```

**field read propagates ref escape**

```metall
{
    struct Wrapper { one &Int }
    let x = {
        let y = 42
        let z = Wrapper(&y)
        z.one
    }
    x
}
```

```error
test.met:5:25: reference escaping its allocation scope (via block result)
            let y = 42
            let z = Wrapper(&y)
                            ^^
            z.one
```

**field read through ref propagates escape**

```metall
{
    struct Wrapper { one &Int }
    let x = {
        let y = 42
        let z = Wrapper(&y)
        let w = &z
        w.one
    }
    x
}
```

```error
test.met:5:25: reference escaping its allocation scope (via block result)
            let y = 42
            let z = Wrapper(&y)
                            ^^
            let w = &z
```

**nested field read propagates escape**

```metall
{
    struct Foo { one &Int }
    struct Bar { one Foo }
    let x = {
        let y = 42
        let z = Bar(Foo(&y))
        let w = &z
        w.one.one
    }
    x
}
```

```error
test.met:6:25: reference escaping its allocation scope (via block result)
            let y = 42
            let z = Bar(Foo(&y))
                            ^^
            let w = &z
```

**field read after reassign escapes**

```metall
{
    struct Wrapper { one &Int }
    let x = 1
    mut y = Wrapper(&x)
    let z = {
        let w = 42
        y = Wrapper(&w)
        y.one
    }
}
```

```error
test.met:7:21: reference escaping its allocation scope (via mutation of outer variable)
            let w = 42
            y = Wrapper(&w)
                        ^^
            y.one

test.met:7:21: reference escaping its allocation scope (via block result)
            let w = 42
            y = Wrapper(&w)
                        ^^
            y.one
```

**heap alloc escapes**

```metall
{
    struct Foo { one Str }
    let x = {
        let @myalloc = Arena()
        @myalloc.new<Foo>(Foo("hello"))
    }
    x
}
```

```error
test.met:5:9: reference escaping its allocation scope (via block result)
            let @myalloc = Arena()
            @myalloc.new<Foo>(Foo("hello"))
            ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
        }
```

**heap alloc slice escapes**

```metall
{
    let x = {
        let @myalloc = Arena()
        unsafe @myalloc.slice_uninit<Int>(5)
    }
    x
}
```

```error
test.met:4:16: reference escaping its allocation scope (via block result)
            let @myalloc = Arena()
            unsafe @myalloc.slice_uninit<Int>(5)
                   ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
        }
```

**valid heap alloc through param**

```metall
{
    struct Foo { one Str }
    fun foo(@a Arena) &Foo { @a.new<Foo>(Foo("hello")) }
    let @a = Arena()
    let x = foo(@a)
}
```

```error
```

**heap alloc nested escape**

```metall
{
    struct Foo { one Str }
    let @youralloc = Arena()
    let x = {
        let @myalloc = Arena()
        @myalloc.new<Foo>(Foo("hello"))
    }
    x
}
```

```error
test.met:6:9: reference escaping its allocation scope (via block result)
            let @myalloc = Arena()
            @myalloc.new<Foo>(Foo("hello"))
            ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
        }
```

**aliased inner allocator heap alloc escapes**

An alias inherits its source's taint. Aliasing the *inner* arena and letting the
allocated value reach the outer binding `x` must be rejected.

```metall
{
    struct Foo { one Str }
    let @outer = Arena()
    let x = {
        let @inner = Arena()
        let @alias = @inner
        @alias.new<Foo>(Foo("hello"))
    }
}
```

```error
test.met:7:9: reference escaping its allocation scope (via block result)
            let @alias = @inner
            @alias.new<Foo>(Foo("hello"))
            ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
        }
```

**aliased outer allocator heap alloc valid**

The matched opposite: same shape, but `@alias` targets the *outer* arena, so the
value lives as long as `@outer` and may reach the outer binding `x`.

```metall
{
    struct Foo { one Str }
    let @outer = Arena()
    let x = {
        let @alias = @outer
        @alias.new<Foo>(Foo("hello"))
    }
}
```

```error
```

**aliased param allocator heap alloc valid**

An alias of a parameter arena carries the caller's taint, so a value allocated
through it may escape the function.

```metall
{
    struct Foo { one Str }
    fun foo(@a Arena) &Foo {
        let @b = @a
        @b.new<Foo>(Foo("hello"))
    }
    let @a = Arena()
    let x = foo(@a)
}
```

```error
```

**heap alloc ref assignment escapes**

```metall
{
    struct Foo { one Str }
    fun foo(@a Arena) &Foo { @a.new<Foo>(Foo("hello")) }
    fun identity(a &Foo) &Foo { a }
    let x = {
        let @a = Arena()
        let y = foo(@a)
        let z = y
        identity(z)
    }
    x
}
```

```error
test.met:9:9: reference escaping its allocation scope (via block result)
            let z = y
            identity(z)
            ^^^^^^^^^^^
        }
```

**heap alloc call escape**

```metall
{
    struct Foo { one Str }
    fun foo(@a Arena) &Foo { @a.new<Foo>(Foo("hello")) }
    let x = {
        let @youralloc = Arena()
        foo(@youralloc)
    }
    x
}
```

```error
test.met:6:9: reference escaping its allocation scope (via block result)
            let @youralloc = Arena()
            foo(@youralloc)
            ^^^^^^^^^^^^^^^
        }
```

**allocator field escape**

```metall
{
    struct Foo { one Str }
    struct Bar { @myalloc Arena }
    fun foo(a Bar) &Foo {
        a.@myalloc.new<Foo>(Foo("hello"))
    }
    let x = {
        let @myalloc = Arena()
        let y = Bar(@myalloc)
        foo(y)
    }
    x
}
```

```error
test.met:10:9: reference escaping its allocation scope (via block result)
            let y = Bar(@myalloc)
            foo(y)
            ^^^^^^
        }
```

**valid struct allocator**

```metall
{
    struct Foo { one Str }
    struct Bar { @myalloc Arena }
    let @myalloc = Arena()
    let x = Bar(@myalloc)
    let y = x.@myalloc.new<Foo>(Foo("hello"))
}
```

```error
```

**nested allocator escape**

```metall
{
    struct Foo { one Str }
    struct Bar { @myalloc Arena }
    struct Baz { one Bar }
    fun foo(a Baz) &Foo {
        a.one.@myalloc.new<Foo>(Foo("hello"))
    }
    let x = {
        let @myalloc = Arena()
        let y = Baz(Bar(@myalloc))
        foo(y)
    }
    x
}
```

```error
test.met:11:9: reference escaping its allocation scope (via block result)
            let y = Baz(Bar(@myalloc))
            foo(y)
            ^^^^^^
        }
```

**make default ref escapes**

```metall
{
    struct Wrapper { one &Int }
    let @a = Arena()
    let x = {
        let local = 123
        @a.slice<Wrapper>(3, Wrapper(&local))
    }
}
```

```error
test.met:6:38: reference escaping its allocation scope (via block result)
            let local = 123
            @a.slice<Wrapper>(3, Wrapper(&local))
                                         ^^^^^^
        }
```

**make default ref escapes immutable**

```metall
{
    struct Wrapper { one &Int }
    let @a = Arena()
    let x = {
        let local = 123
        @a.slice<Wrapper>(3, Wrapper(&local))
    }
}
```

```error
test.met:6:38: reference escaping its allocation scope (via block result)
            let local = 123
            @a.slice<Wrapper>(3, Wrapper(&local))
                                         ^^^^^^
        }
```

**make ref default escapes**

```metall
{
    let @a = Arena()
    let x = {
        let local = 123
        @a.slice<&Int>(3, &local)
    }
}
```

```error
test.met:5:27: reference escaping its allocation scope (via block result)
            let local = 123
            @a.slice<&Int>(3, &local)
                              ^^^^^^
        }
```

**make ref default escapes immutable**

```metall
{
    let @a = Arena()
    let x = {
        let local = 123
        @a.slice<&Int>(3, &local)
    }
}
```

```error
test.met:5:27: reference escaping its allocation scope (via block result)
            let local = 123
            @a.slice<&Int>(3, &local)
                              ^^^^^^
        }
```

**valid shadowed ref**

```metall
{
    mut x = 123
    mut y = &x
    {
        mut z = 456
        mut y = &mut z
        y.* = 789
    }
}
```

```error
```

**if branch ref escapes**

```metall
{
    let x = 1
    let y = {
        let z = 42
        if true { &z } else { &x }
    }
}
```

```error
test.met:5:19: reference escaping its allocation scope (via block result)
            let z = 42
            if true { &z } else { &x }
                      ^^
        }
```

**call with mixed-scope refs escapes**

```metall
{
    fun foo(a &Int, b &Int) &Int { if true { a } else { b } }
    let x = 42
    let y = {
        let z = 99
        foo(&x, &z)
    }
}
```

```error
test.met:6:17: reference escaping its allocation scope (via block result)
            let z = 99
            foo(&x, &z)
                    ^^
        }
```

**valid call with same-scope refs**

```metall
{
    fun foo(a &Int, b &Int) &Int { if true { a } else { b } }
    let x = 42
    let y = 99
    let z = foo(&x, &y)
}
```

```error
```

**struct construction ref escapes**

```metall
{
    struct Wrapper { one &Int }
    let x = {
        let y = 42
        let z = Wrapper(&y)
        z
    }
    x
}
```

```error
test.met:5:25: reference escaping its allocation scope (via block result)
            let y = 42
            let z = Wrapper(&y)
                            ^^
            z
```

**array literal ref escapes**

```metall
{
    let x = {
        let y = 42
        let z = [&y]
        z
    }
    x
}
```

```error
test.met:4:18: reference escaping its allocation scope (via block result)
            let y = 42
            let z = [&y]
                     ^^
            z
```

**constant array literal subslice can be returned**

```metall
fun foo() []U8 {
    [U8(1), 2, 3][..]
}
```

```error
```

**valid array literal ref**

```metall
{
    let x = 42
    let y = [&x]
}
```

```error
```

**index field write escapes**

```metall
{
    struct Wrapper { one &Int }
    mut x = 123
    mut y = [Wrapper(&x)]
    {
        mut z = 456
        y[0].one = &z
    }
}
```

```error
test.met:7:20: reference escaping its allocation scope (via mutation of outer variable)
            mut z = 456
            y[0].one = &z
                       ^^
        }
```

**valid index field write**

```metall
{
    struct Wrapper { one &Int }
    mut x = 123
    mut y = 456
    mut z = [Wrapper(&x)]
    z[0].one = &y
}
```

```error
```

**field index write escape**

```metall
{
    struct Foo { one [1]&mut Int }
    mut x = 123
    mut y = Foo([&mut x])
    {
        mut z = 456
        y.one[0] = &mut z
    }
}
```

```error
test.met:7:20: reference escaping its allocation scope (via mutation of outer variable)
            mut z = 456
            y.one[0] = &mut z
                       ^^^^^^
        }
```

**valid field index write**

```metall
{
    struct Foo { one [1]&mut Int }
    mut x = 123
    mut y = 456
    mut z = Foo([&mut x])
    z.one[0] = &mut y
}
```

```error
```

**valid subslice same scope**

```metall
{
    mut x = [1, 2, 3, 4, 5]
    let s = x[1..3]
    let y = s[0]
}
```

```error
```

**valid subslice to outer scope**

```metall
{
    let x = [1, 2, 3, 4, 5]
    let s = {
        x[1..3]
    }
}
```

```error
```

**valid subslice to struct's slice scope**

```metall
{
    struct Wrapper { data []Int }
    let x = [1, 2, 3, 4, 5]
    let w = Wrapper(x[..])
    let s = {
        w.data[1..3]
    }
}
```

```error
```

**valid subslice to Str slice**

```metall
{
    fun Str.as_bytes(s Str) []U8 { s.data }

    let s = {
        "test".as_bytes()[0..3]
    }
}
```

```error
```

**valid subslice to Str slice fun**

```metall
{
    fun Str.as_bytes(s Str) []U8 { s.data }

    struct Wrapper { text []U8 }

    fun Wrapper.new(s Str) Wrapper {
        let text = s.as_bytes()
        Wrapper(text[0..text.len])
    }

    let x = Wrapper.new("test")
}
```

```error
```

**valid subslice to slice wrapper slice fun**

```metall
{

    struct Wrapper { text []U8 }

    fun Wrapper.as_bytes(w Wrapper) []U8 { w.text }

    fun Wrapper.new(w Wrapper) Wrapper {
        let text = w.as_bytes()
        Wrapper(text[0..text.len])
    }

    let w = Wrapper([U8(1), 2, 3][..])
    let x = Wrapper.new(w)
}
```

```error
```

**subslice escapes scope**

```metall
{
    let s = {
        let x = [1, 2, 3, 4, 5]
        x[1..3]
    }
}
```

```error
test.met:4:9: reference escaping its allocation scope (via block result)
            let x = [1, 2, 3, 4, 5]
            x[1..3]
            ^^^^^^^
        }
```

**ref of subslice element escapes**

```metall
{
    let y = {
        mut x = [1, 2, 3, 4, 5]
        let s = x[1..3]
        &s[0]
    }
}
```

```error
test.met:4:17: reference escaping its allocation scope (via block result)
            mut x = [1, 2, 3, 4, 5]
            let s = x[1..3]
                    ^^^^^^^
            &s[0]
```

**subslice propagates taint**

```metall
{
    mut outer = 0
    mut r = &outer
    {
        mut x = [1, 2, 3]
        let s = x[0..2]
        r = &s[0]
    }
}
```

```error
test.met:6:17: reference escaping its allocation scope (via mutation of outer variable)
            mut x = [1, 2, 3]
            let s = x[0..2]
                    ^^^^^^^
            r = &s[0]
```

**return ref to reassigned local**

```metall
{
    fun foo(a &Int) &Int {
        mut x = 1
        x = a.*
        &x
    }
    let y = 42
    let r = foo(&y)
}
```

```error
test.met:5:9: reference escaping its allocation scope (via block result)
            x = a.*
            &x
            ^^
        }
```

**return ref to reassigned heap alloc local**

```metall
{
    struct Foo { @myalloc Arena }
    fun foo(a &Foo) &Foo {
        let @youralloc = Arena()
        mut x = Foo(@youralloc)
        x = a.*
        &x
    }
    let @myalloc = Arena()
    let x = Foo(@myalloc)
    let r = foo(&x)
}
```

```error
test.met:7:9: reference escaping its allocation scope (via block result)
            x = a.*
            &x
            ^^
        }
```

**ref to local after reassign escapes**

```metall
{
    let x = {
        mut y = 1
        y = 2
        &y
    }
    x
}
```

```error
test.met:5:9: reference escaping its allocation scope (via block result)
            y = 2
            &y
            ^^
        }
```

**no escape through unused mut ref param**

```metall
{
    fun foo(a &mut Int) void { a.* = 321 }
    mut x = 123
    foo(&mut x)
}
```

```error
```

**field mutation bypass**

```metall
{
    struct Foo { one &Int }
    fun foo(a &mut Foo, b &Int) void {
        a.one = b
    }
    mut x = 42
    mut y = Foo(&mut x)
    {
        mut z = 99
        foo(&mut y, &z)
    }
}
```

```error
test.met:10:21: reference escaping its allocation scope (via mutation of outer variable)
            mut z = 99
            foo(&mut y, &z)
                        ^^
        }
```

**transitive mutation bypass**

```metall
{
    struct Foo { one &Int }
    fun identity(a &mut Foo) &mut Foo { a }
    mut x = 42
    mut y = Foo(&mut x)
    {
        mut z = 99
        let w = identity(&mut y)
        w.one = &z
    }
}
```

```error
test.met:9:17: reference escaping its allocation scope (via mutation of outer variable)
            let w = identity(&mut y)
            w.one = &z
                    ^^
        }
```

**returned ref bypass**

```metall
{
    struct Foo { one &Int }
    fun identity(a &mut Foo) &mut Foo { a }
    mut x = 12742
    mut y = Foo(&mut x)
    {
        mut z = 99
        let w = identity(&mut y)
        w.one = &z
    }
}
```

```error
test.met:9:17: reference escaping its allocation scope (via mutation of outer variable)
            let w = identity(&mut y)
            w.one = &z
                    ^^
        }
```

**heap alloc stack-ref bypass**

```metall
{
    struct Foo { one &Int }
    fun foo(a &mut Foo, b &Int) void { a.one = b }
    let @myalloc = Arena()
    mut x = 1
    let y = @myalloc.new<Foo>(Foo(&mut x))
    {
        mut z = 99
        foo(y, &z)
    }
}
```

```error
test.met:9:16: reference escaping its allocation scope (via mutation of outer variable)
            mut z = 99
            foo(y, &z)
                   ^^
        }
```

**forward declare bypass**

```metall
{
    struct Foo { one &Int }
    fun foo(a &mut Foo, b &Int) void {
        bar(a, b)
    }
    fun bar(a &mut Foo, b &Int) void {
        a.one = b
    }
    mut x = 42
    mut y = Foo(&mut x)
    {
        mut z = 99
        foo(&mut y, &z)
    }
}
```

```error
test.met:13:21: reference escaping its allocation scope (via mutation of outer variable)
            mut z = 99
            foo(&mut y, &z)
                        ^^
        }
```

**pessimistic no return taint for void**

```metall
{
    fun foo(a &Int) void { foo(a) }
    let x = 42
    foo(&x)
}
```

```error
```

**pessimistic no return taint for int**

```metall
{
    fun foo(a &Int) Int { foo(a) }
    let x = 42
    foo(&x)
}
```

```error
```

**pessimistic return taint for ref**

```metall
{
    fun foo(a &Int) &Int { foo(a) }
    let x = 42
    foo(&x)
}
```

```error
test.met:4:9: reference escaping its allocation scope (via block result)
        let x = 42
        foo(&x)
            ^^
    }
```

**mutual recursion bypass**

```metall
{
    struct Foo {
        one &Int
    }

    fun foo(a &mut Foo, b &Int) Int {
        _ = bar(a, b)
        42
    }

    fun bar(a &mut Foo, b &Int) Int {
        _ = foo(a, b)
        1337
    }

    mut x = 0
    mut y = Foo(&mut x)

    {
        mut z = 99
        foo(&mut y, &z)
    }

}
```

```error
test.met:21:21: reference escaping its allocation scope (via mutation of outer variable)
            mut z = 99
            foo(&mut y, &z)
                        ^^
        }
```

**side-effect bypass**

```metall
{
    struct Foo { one &Int }
    fun identity(a &mut Foo) &mut Foo { a }
    fun foo(a &mut Foo, b &Int) void { a.one = b }

    mut x = 12742
    mut y = Foo(&mut x)
    {
        mut z = 99
        foo(identity(&mut y), &z)
    }
}
```

```error
test.met:10:31: reference escaping its allocation scope (via mutation of outer variable)
            mut z = 99
            foo(identity(&mut y), &z)
                                  ^^
        }
```

**multi-level deref mutation escapes**

```metall
{
    struct Foo { one &Int }
    mut x = 12742
    mut y = Foo(&mut x)
    mut z = &mut y
    mut w = &mut z
    {
        mut a = 99
        mut b = Foo(&mut a)
        w.*.* = b
    }
}
```

```error
test.met:9:21: reference escaping its allocation scope (via deref assignment)
            mut a = 99
            mut b = Foo(&mut a)
                        ^^^^^^
            w.*.* = b
```

**deref field ref mutation escapes**

```metall
{
    struct Foo { one &mut &Int }
    mut x = 12742
    mut y = &x
    mut z = Foo(&mut y)
    {
        mut w = 99
        z.one.* = &w
    }
}
```

```error
test.met:8:19: reference escaping its allocation scope (via deref assignment)
            mut w = 99
            z.one.* = &w
                      ^^
        }
```

**field overwrite doesn't mask escape**

```metall
{
    struct Foo {
        one Str
        two &Int
    }
    mut x = 12742
    mut y = Foo("hello", &x)
    {
        mut z = 99
        y.two = &z
        y.one = "bye"
    }
}
```

```error
test.met:10:17: reference escaping its allocation scope (via mutation of outer variable)
            mut z = 99
            y.two = &z
                    ^^
            y.one = "bye"
```

**for loop ref escapes**

```metall
{
    mut x = 0
    mut y = &x
    for {
        mut z = 99
        y = &z
        break
    }
}
```

```error
test.met:6:13: reference escaping its allocation scope (via mutation of outer variable)
            mut z = 99
            y = &z
                ^^
            break
```

**method ref escapes via return**

```metall
{
    struct Foo { }
    fun Foo.escape(f Foo, a &Int) &Int { a }
    fun bar(t Foo) &Int {
        let a = 42
        t.escape(&a)
    }
    let r = {
        let f = Foo()
        bar(f)
    }
    r
}
```

```error
test.met:6:18: reference escaping its allocation scope (via block result)
            let a = 42
            t.escape(&a)
                     ^^
        }
```

**for-in ref to binding escapes via outer assignment**

```metall
{
    mut x = 0
    mut r = &x
    for i in 0..10 {
        r = &i
    }
}
```

```error
test.met:5:13: reference escaping its allocation scope (via mutation of outer variable)
        for i in 0..10 {
            r = &i
                ^^
        }
```

**for-in slice ref to element escapes via outer assignment**

```metall
{
    mut x = 0
    mut r = &x
    for v in [1, 2, 3] {
        r = &v
    }
}
```

```error
test.met:5:13: reference escaping its allocation scope (via mutation of outer variable)
        for v in [1, 2, 3] {
            r = &v
                ^^
        }
```

**for &x binding escapes via outer assignment**

```metall
{
    mut r = &0
    for &x in [1, 2, 3] {
        r = x
    }
}
```

```error
test.met:4:9: reference escaping its allocation scope (via mutation of outer variable)
        for &x in [1, 2, 3] {
            r = x
            ^^^^^
        }
```

**for &mut x binding escapes via outer assignment**

```metall
{
    mut arr = [1, 2, 3]
    let s = arr[..]
    mut r = &mut s[0]
    for &mut x in s {
        r = x
    }
}
```

```error
test.met:6:9: reference escaping its allocation scope (via mutation of outer variable)
        for &mut x in s {
            r = x
            ^^^^^
        }
```

**for &x binding escapes via return**

```metall
{
    fun first(s []Int, fb &Int) &Int {
        for &x in s {
            return x
        }
        fb
    }
}
```

```error
test.met:4:20: reference escaping its allocation scope (via return)
            for &x in s {
                return x
                       ^
            }
```

**for &x binding escapes via outer struct field**

```metall
{
    struct Box { p &Int }
    mut b = Box(&0)
    for &x in [1, 2, 3] {
        b.p = x
    }
}
```

```error
test.met:5:9: reference escaping its allocation scope (via mutation of outer variable)
        for &x in [1, 2, 3] {
            b.p = x
            ^^^^^^^
        }
```

**for &x binding escapes via nested block**

```metall
{
    mut r = &0
    for &x in [1, 2, 3] {
        { r = x }
    }
}
```

```error
test.met:4:11: reference escaping its allocation scope (via mutation of outer variable)
        for &x in [1, 2, 3] {
            { r = x }
              ^^^^^
        }
```

**for &x binding used only inside the loop is fine**

```metall
{
    mut sum = 0
    for &x in [1, 2, 3] {
        sum = sum + x.*
    }
}
```

```error
```

**for &mut x writing through the ref inside the loop is fine**

```metall
{
    mut arr = [1, 2, 3]
    for &mut x in arr[..] {
        x.* = 9
    }
}
```

```error
```

**for &x binding escapes via deref assignment**

```metall
{
    mut sink = &0
    let pp = &mut sink
    for &x in [1, 2, 3] {
        pp.* = x
    }
}
```

```error
test.met:5:9: reference escaping its allocation scope (via deref assignment)
        for &x in [1, 2, 3] {
            pp.* = x
            ^^^^^^^^
        }
```

**for &x binding escapes via deref assignment from a nested block**

```metall
{
    mut sink = &0
    let pp = &mut sink
    for &x in [1, 2, 3] {
        { pp.* = x }
    }
}
```

```error
test.met:5:11: reference escaping its allocation scope (via deref assignment)
        for &x in [1, 2, 3] {
            { pp.* = x }
              ^^^^^^^^
        }
```

**for &x ref to element field escapes via outer assignment**

```metall
{
    struct P { a Int }
    mut r = &0
    for &p in [P(1), P(2)] {
        r = &p.a
    }
}
```

```error
test.met:5:13: reference escaping its allocation scope (via mutation of outer variable)
        for &p in [P(1), P(2)] {
            r = &p.a
                ^^^^
        }
```

**for &x ref to element field escapes via return**

```metall
{
    struct P { a Int }
    fun first(ps []P, fb &Int) &Int {
        for &p in ps {
            return &p.a
        }
        fb
    }
}
```

```error
test.met:5:20: reference escaping its allocation scope (via return)
            for &p in ps {
                return &p.a
                       ^^^^
            }
```

**for &mut x writing through a field of the ref is fine**

```metall
{
    struct P { a Int }
    mut ps = [P(1), P(2)]
    for &mut p in ps[..] {
        p.a = p.a + 1
    }
}
```

```error
```

**for &mut x over a mutable array binding escapes via outer assignment**

```metall
{
    mut arr = [1, 2, 3]
    mut r = &mut arr[0]
    for &mut x in arr {
        r = x
    }
}
```

```error
test.met:5:9: reference escaping its allocation scope (via mutation of outer variable)
        for &mut x in arr {
            r = x
            ^^^^^
        }
```

**for &mut x over a mutable array writing through the ref is fine**

```metall
{
    mut arr = [1, 2, 3]
    for &mut x in arr {
        x.* = x.* + 1
    }
}
```

```error
```

**match arm ref escapes via result**

```metall
{
    union Foo = Int | Bool
    let x = 1
    let y = {
        let z = 42
        let u = Foo(z)
        match u {
            case Int: &z
            case Bool: &x
        }
    }
}
```

```error
test.met:8:23: reference escaping its allocation scope (via block result)
            match u {
                case Int: &z
                          ^^
                case Bool: &x
```

**match with else ref escapes via result**

```metall
{
    union Foo = Int | Bool
    let x = 1
    let y = {
        let z = 42
        let u = Foo(z)
        match u {
            case Int: &x
            else: &z
        }
    }
}
```

```error
test.met:9:19: reference escaping its allocation scope (via block result)
                case Int: &x
                else: &z
                      ^^
            }
```

**match binding tainted by function call result**

```metall
{
    struct Wrapper { one &Int }
    union Holder = Wrapper | Int
    fun make(r &Int) Holder { Holder(Wrapper(r)) }
    let x = 42
    let y = {
        let z = 99
        let h = make(&z)
        match h {
            case Wrapper w: w.one
            case Int: &x
        }
    }
}
```

```error
test.met:8:22: reference escaping its allocation scope (via block result)
            let z = 99
            let h = make(&z)
                         ^^
            match h {
```

**match binding tainted by outer variable escapes**

```metall
{
    struct Wrapper { one &Int }
    union Holder = Wrapper | Bool
    let x = 42
    let y = {
        let z = 99
        let h = Holder(Wrapper(&z))
        match h {
            case Wrapper w: w.one
            case Bool: &x
        }
    }
}
```

```error
test.met:7:32: reference escaping its allocation scope (via block result)
            let z = 99
            let h = Holder(Wrapper(&z))
                                   ^^
            match h {
```

**match ref to non-ref binding escapes**

```metall
{
    union Foo = Int | Bool
    let x = 1
    let y = {
        let u = Foo(42)
        match u {
            case Int n: &n
            case Bool: &x
        }
    }
}
```

```error
test.met:7:25: reference escaping its allocation scope (via block result)
            match u {
                case Int n: &n
                            ^^
                case Bool: &x
```

**pessimistic return taint for union with ref**

```metall
{
    struct Wrapper { one &Int }
    union Holder = Wrapper | Bool
    fun foo(h Holder) Holder { foo(h) }
    let x = {
        let y = 42
        foo(Holder(Wrapper(&y)))
    }
}
```

```error
test.met:7:28: reference escaping its allocation scope (via block result)
            let y = 42
            foo(Holder(Wrapper(&y)))
                               ^^
        }
```

**match binding ref assigned to outer variable**

```metall
{
    union Foo = Int | Bool
    mut x = 0
    mut r = &x
    let u = Foo(42)
    match u {
        case Int n: r = &n
        case Bool: {}
    }
}
```

```error
test.met:7:25: reference escaping its allocation scope (via mutation of outer variable)
        match u {
            case Int n: r = &n
                            ^^
            case Bool: {}
```

**match else binding ref field escapes**

```metall
{
    struct Wrapper { one &Int }
    union Holder = Int | Wrapper
    let x = 42
    let y = {
        let z = 99
        let h = Holder(Wrapper(&z))
        match h {
            case Int: &x
            else w: w.one
        }
    }
}
```

```error
test.met:7:32: reference escaping its allocation scope (via block result)
            let z = 99
            let h = Holder(Wrapper(&z))
                                   ^^
            match h {
```

**match arm return ref to local**

```metall
{
    union Foo = Int | Bool
    fun bar(u Foo) &Int {
        let local = 99
        match u {
            case Int: return &local
            case Bool: return &local
        }
    }
}
```

```error
test.met:6:30: reference escaping its allocation scope (via return)
            match u {
                case Int: return &local
                                 ^^^^^^
                case Bool: return &local
```

**match binding used safely no escape**

```metall
{
    union Foo = Int | Bool
    let u = Foo(42)
    match u {
        case Int n: let x = n
        case Bool: {}
    }
}
```

```error
```

**match ref variant binding used safely**

```metall
{
    union RefOrInt = &Int | Int
    let x = 42
    let u = RefOrInt(&x)
    match u {
        case &Int r: let y = r.*
        case Int: {}
    }
}
```

```error
```

**nested match ref escapes**

```metall
{
    union Outer = Int | Bool
    union Inner = Int | Bool
    let x = 1
    let y = {
        let z = 42
        let o = Outer(z)
        match o {
            case Int n:
                let i = Inner(n)
                match i {
                    case Int: &z
                    case Bool: &x
                }
            case Bool: &x
        }
    }
}
```

```error
test.met:12:31: reference escaping its allocation scope (via block result)
                    match i {
                        case Int: &z
                                  ^^
                        case Bool: &x
```

**match guard binding used safely**

```metall
{
    union Foo = Int | Bool
    let u = Foo(42)
    match u {
        case Int n if n > 10: let x = n
        case Int n: let y = n
        case Bool: {}
    }
}
```

```error
```

**match guard ref to binding escapes**

```metall
{
    union Foo = Int | Bool
    mut x = 0
    mut r = &x
    let u = Foo(42)
    match u {
        case Int n if n > 0:
            r = &n
        case Int: {}
        case Bool: {}
    }
}
```

```error
test.met:8:17: reference escaping its allocation scope (via mutation of outer variable)
            case Int n if n > 0:
                r = &n
                    ^^
            case Int: {}
```

**match on union param with ref variant escapes**

```metall
{
    struct Wrapper { one &Int }
    union WOrBool = Wrapper | Bool
    fun extract(u WOrBool, fallback &Int) &Int {
        match u {
            case Wrapper w: w.one
            case Bool: fallback
        }
    }
    let x = 42
    let y = {
        let z = 99
        extract(WOrBool(Wrapper(&z)), &x)
    }
}
```

```error
test.met:13:33: reference escaping its allocation scope (via block result)
            let z = 99
            extract(WOrBool(Wrapper(&z)), &x)
                                    ^^
        }
```

**match value-type binding does not carry ref taint from other variants**

```metall
{
    struct MyErr { msg Str }
    union ValOrRef = &Int | MyErr
    let x = 42
    let r = ValOrRef(&x)
    let result = match r {
        case MyErr e: e
        case &Int: MyErr("not an error")
    }
}
```

```error
```

**match ref-type binding still carries taint from ref variant**

```metall
{
    union IntOrRef = Int | &Int
    let result = {
        let x = 42
        let r = IntOrRef(&x)
        match r {
            case &Int i: i
            case Int: &x
        }
    }
}
```

```error
test.met:5:26: reference escaping its allocation scope (via block result)
            let x = 42
            let r = IntOrRef(&x)
                             ^^
            match r {
```

**match else binding with value-type uncovered variants does not carry taint**

```metall
{
    struct MyErr { msg Str }
    union FileResult = &Int | MyErr
    fun handle(r FileResult) MyErr {
        match r {
            case &Int: MyErr("not an error")
            else e: e
        }
    }
    let x = 42
    handle(FileResult(&x))
}
```

```error
```

**subslice of by-value param array field escapes via return**

```metall
{
    struct Foo {
        bytes [4]U8
        len Int
    }
    fun Foo.as_slice(f Foo) []U8 {
        f.bytes[0..f.len]
    }
    let f = Foo([U8(65), 0, 0, 0], 1)
    f.as_slice()
}
```

```error
test.met:7:9: reference escaping its allocation scope (via block result)
        fun Foo.as_slice(f Foo) []U8 {
            f.bytes[0..f.len]
            ^^^^^^^^^^^^^^^^^
        }
```

**subslice of ref param array field does not escape**

```metall
{
    struct Foo {
        bytes [4]U8
        len Int
    }
    fun Foo.as_slice(f &Foo) []U8 {
        f.bytes[0..f.len]
    }
    mut f = Foo([U8(65), 0, 0, 0], 1)
    _ = Foo.as_slice(&f)
}
```

```error
```

## Cross-Module Lifetime Tests

**setup lib module**

```metall
```

```module.lib
pub fun safe(a &Int) &Int { a }
pub fun leaky(a &Int) &Int {
    mut local = 42
    &local
}
```

**functions with same signature in external module get independent effects**

```metall module
use lib

fun main() void {
    mut x = 42
    _ = lib.safe(&x)
    _ = lib.leaky(&x)
}
```

```error
lib/lib.met:4:5: reference escaping its allocation scope (via block result)
        mut local = 42
        &local
        ^^^^^^
    }
```

**arena alloc size arg does not alias source into result**

```metall
{
    struct W { data []Int }
    fun alloc_ints(@a Arena, n Int, default Int) []mut Int {
        @a.slice<Int>(n, default)
    }
    fun alloc_ws(@a Arena, n Int, default W) []mut W {
        @a.slice<W>(n, default)
    }
    fun process(items []Int) void {
        let @a = Arena()
        let buf = alloc_ws(@a, items.len, W([0][0..0]))
        for i in 0..items.len {
            buf[i] = W(alloc_ints(@a, 1, items[i]))
        }
    }
    process([1, 2, 3][..])
}
```

```error
```

## Module Constants

**Module-level constants have static lifetime and can be referenced from functions**

```metall module
struct Foo { one Int }
let x = 42
let y = Foo(1)
let z = &x

fun take_ref(a &Int) Int { a.* }
fun take_struct_ref(a &Foo) Int { a.one }

fun main() void {
    let a = &x
    let b = &y
    _ = take_ref(a)
    _ = take_struct_ref(b)
    _ = take_ref(z)
}
```

```error
```

**Module-level constant ref does not escape (returned from function is fine)**

```metall module
let x = 42

fun get_ref() &Int { &x }

fun main() void {
    let r = get_ref()
    _ = r.*
}
```

```error
```

**closure capture by ref used locally does not escape**

```metall
fun foo() Int {
    let x = 42
    let get = fun[&x]() Int { x.* }
    get()
}
```

```error
```

**by-value capture bubbles up out of its subscope without escaping**

`n` and the closure are declared in a subscope; the closure value bubbles up to
`g` but stays inside `foo`. The by-value capture copies `n` into the context,
which lives on `foo`'s frame and so outlives the subscope. Nothing escapes. (The
context taint is the enclosing function, not the subscope, so this is not a false
escape.)

```metall
fun foo() Int {
    let g = {
        let n = 42
        fun[n](x Int) Int { x + n }
    }
    g(8)
}
```

```error
```

**by-ref capture of a subscope local escapes even when the closure stays in the function**

Same shape with `&n`: the closure never leaves `foo`, but it borrows `n`, which
dies when the subscope ends, so the borrow escapes its subscope. This is the
by-value/by-ref distinction the previous test pins down.

```metall
fun foo() Int {
    let g = {
        let n = 42
        fun[&n](x Int) Int { x + n.* }
    }
    g(8)
}
```

```error
test.met:4:9: reference escaping its allocation scope (via block result)
            let n = 42
            fun[&n](x Int) Int { x + n.* }
            ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
        }
```

**closure capturing a value by value escapes when returned**

A closure with any capture carries its context, which lives on the enclosing
function's frame. Returning it escapes that frame, even though `x` is copied into
the context by value.

```metall
fun foo() fun() Int {
    let x = 42
    fun[x]() Int { x }
}
```

```error
test.met:3:5: reference escaping its allocation scope (via block result)
        let x = 42
        fun[x]() Int { x }
        ^^^^^^^^^^^^^^^^^^
    }
```

**closure capturing a parameter by value escapes when returned**

```metall
fun make_adder(n Int) fun(Int) Int {
    fun[n](x Int) Int { x + n }
}
```

```error
test.met:2:5: reference escaping its allocation scope (via block result)
    fun make_adder(n Int) fun(Int) Int {
        fun[n](x Int) Int { x + n }
        ^^^^^^^^^^^^^^^^^^^^^^^^^^^
    }
```

**closure capture ref by value escapes when returned**

```metall
fun foo() fun() &Int {
    let x = 42
    let r = &x
    fun[r]() &Int { r }
}
```

```error
test.met:3:13: reference escaping its allocation scope (via block result)
        let x = 42
        let r = &x
                ^^
        fun[r]() &Int { r }
```

**closure capture by ref escapes when returned**

```metall
fun foo() fun() &Int {
    let x = 42
    fun[&x]() &Int { x }
}
```

```error
test.met:3:5: reference escaping its allocation scope (via block result)
        let x = 42
        fun[&x]() &Int { x }
        ^^^^^^^^^^^^^^^^^^^^
    }
```

**closure captures allocator by value used locally**

```metall
fun foo() void {
    let @a = Arena()
    let alloc = fun[@a]() []Int { @a.slice<Int>(3, 0) }
    _ = alloc()
}
```

```error
```

**closure capturing allocator escapes allocator scope**

```metall
fun foo() fun() []Int {
    let @a = Arena()
    fun[@a]() []Int { @a.slice<Int>(3, 0) }
}
```

```error
test.met:3:5: reference escaping its allocation scope (via block result)
        let @a = Arena()
        fun[@a]() []Int { @a.slice<Int>(3, 0) }
        ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
    }
```

**closure with inferred types escapes ref via return**

```metall
fun foo() fun() &Int {
    let x = 42
    let r = &x
    fun[r]() { r }
}
```

```error
test.met:3:13: reference escaping its allocation scope (via block result)
        let x = 42
        let r = &x
                ^^
        fun[r]() { r }
```

**defer block reading local ref does not escape**

```metall
fun foo() void {
    let x = 42
    let r = &x
    defer { _ = r.* }
}
```

```error
```

**defer block assigning to outer mut ref does not escape**

```metall
fun foo(out &mut Int) void {
    let x = 42
    defer { out.* = x }
}
```

```error
```

**defer block cannot store local ref in outer variable**

```metall
{
    let y = 0
    mut r = &y
    {
        let x = 42
        defer { r = &x }
    }
}
```

```error
test.met:6:21: reference escaping its allocation scope (via mutation of outer variable)
            let x = 42
            defer { r = &x }
                        ^^
        }
```

## FFI

**Ptr to stack variable escapes via return**

```metall module
use std.ffi

fun leak() ffi.Ptr<Int> {
    let x = 42
    ffi.ref_ptr<Int>(&x)
}

fun main() void { _ = leak() }
```

```error
test.met:5:22: reference escaping its allocation scope (via block result)
        let x = 42
        ffi.ref_ptr<Int>(&x)
                         ^^
    }
```

**Ptr.as_slice from arena-allocated memory escapes**

```metall module
use std.ffi

fun get_slice() []Int {
    let @a = Arena()
    let data = @a.slice<Int>(3, 0)
    let p = ffi.slice_ptr<Int>(data)
    unsafe p.as_slice(3)
}

fun main() void { _ = get_slice() }
```

```error
test.met:7:12: reference escaping its allocation scope (via block result)
        let p = ffi.slice_ptr<Int>(data)
        unsafe p.as_slice(3)
               ^^^^^^^^^^^^^
    }
```

**Ptr from extern function has no lifetime (can escape)**

```metall module
use std.ffi

extern fun get_buf() ffi.Ptr<U8>

fun get_slice() []U8 {
    let p = unsafe get_buf()
    unsafe p.as_slice(10)
}

fun main() void { _ = get_slice() }
```

```error
```

**Ptr.cast_ptr propagates the receiver's lifetime (escape caught)**

```metall module
use std.ffi

fun leak() ffi.Ptr<U8> {
    let @a = Arena()
    let data = @a.slice<Int>(3, 0)
    let p = ffi.slice_ptr<Int>(data)
    unsafe p.cast_ptr<U8>()
}

fun main() void { _ = leak() }
```

```error
test.met:7:12: reference escaping its allocation scope (via block result)
        let p = ffi.slice_ptr<Int>(data)
        unsafe p.cast_ptr<U8>()
               ^^^^^^^^^^^^^^^^
    }
```

**PtrMut.cast_ptr propagates the receiver's lifetime (escape caught)**

```metall module
use std.ffi

fun leak() ffi.PtrMut<U8> {
    let @a = Arena()
    let data = @a.slice<Int>(3, 0)
    let p = ffi.slice_ptr_mut<Int>(data)
    unsafe p.cast_ptr<U8>()
}

fun main() void { _ = leak() }
```

```error
test.met:7:12: reference escaping its allocation scope (via block result)
        let p = ffi.slice_ptr_mut<Int>(data)
        unsafe p.cast_ptr<U8>()
               ^^^^^^^^^^^^^^^^
    }
```

**ffi.Ptr inside a struct keeps its lifetime through a `try` unwrap**

An `ffi.Ptr` derived from arena memory carries that arena's lifetime even when it
sits in a struct field. Unwrapping the `!Box` with `try` must keep that borrow, so
returning the unwrapped `Box` past its arena is caught.

```metall module
use std.ffi

struct Box { p ffi.Ptr<Int> }

fun alloc(@a Arena) !Box {
    let s = unsafe @a.slice_uninit<Int>(1)
    Box(ffi.slice_ptr_mut<Int>(s).as_ptr())
}

fun leak() !Box {
    let @a = Arena()
    try alloc(@a)
}

fun main() void { _ = try leak() else { return void } }
```

```error
test.met:12:5: reference escaping its allocation scope (via block result)
        let @a = Arena()
        try alloc(@a)
        ^^^^^^^^^^^^^
    }
```

## Shape

**shape method ref escapes**

```metall
{
    shape Shape {
        fun Shape.escape(s Shape, a &Int) &Int
    }
    struct Foo { }
    fun Foo.escape(f Foo, a &Int) &Int { a }
    fun bar<T Shape>(t T) &Int {
        let a = 42
        t.escape(&a)
    }
    let r = {
        let f = Foo()
        bar<Foo>(f)
    }
    r
}
```

```error
test.met:9:18: reference escaping its allocation scope (via block result)
            let a = 42
            t.escape(&a)
                     ^^
        }
```

**shape value param no escape when not written to mut ref**

```metall
{
    shape HasRef { fun HasRef.get(t HasRef) &Int }
    struct Foo { one &Int }
    struct Bar { one &Int }
    fun Foo.get(t Foo) &Int { t.one }
    fun baz<T HasRef>(t T, b &mut Bar) void { b.one = b.one }
    let x = 42
    mut y = Bar(&x)
    {
        let z = 99
        baz<Foo>(Foo(&z), &mut y)
    }
}
```

```error
```

**shape value param ref escapes via side effect**

```metall
{
    shape HasRef { fun HasRef.get(t HasRef) &Int }
    struct Foo { one &Int }
    struct Bar { one &Int }
    fun Foo.get(t Foo) &Int { t.one }
    fun baz<T HasRef>(t T, b &mut Bar) void { b.one = t.get() }
    let x = 42
    mut y = Bar(&x)
    {
        let z = 99
        baz<Foo>(Foo(&z), &mut y)
    }
}
```

```error
test.met:11:22: reference escaping its allocation scope (via mutation of outer variable)
            let z = 99
            baz<Foo>(Foo(&z), &mut y)
                         ^^
        }
```

**shape method with mut self does not taint self with args**

```metall
{
    shape S {
        fun S.do(s &mut S, x []Int) void
    }

    struct Foo { n Int }

    fun Foo.do(f &mut Foo, x []Int) void {
        f.n = x.len
    }

    fun bar<T S>(t &mut T) void {
        let a = [1, 2, 3]
        t.do(a[..])
    }

    mut f = Foo(0)
    bar<Foo>(&mut f)
}
```

```error
```

**shape method can store ref when lifetime fits**

```metall
{
    shape Store<Item> {
        fun Store.put(s &mut Store, value Item) void
    }
    struct RefStore { one &Int }
    fun RefStore.put(s &mut RefStore, value &Int) void {
        s.one = value
    }
    fun put_value<S Store<&Int>>(s &mut S, value &Int) void {
        s.put(value)
    }
    let initial = 0
    mut store = RefStore(&initial)
    let next = 1
    put_value(&mut store, &next)
}
```

```error
```

**shape method stored ref escapes through mut self**

```metall
{
    shape S {
        fun S.do(s &mut S, x &Int) void
    }
    struct Foo { one &Int }
    fun Foo.do(f &mut Foo, x &Int) void {
        f.one = x
    }
    fun bar<T S>(t &mut T) void {
        let a = 123
        t.do(&a)
    }
    let z = 0
    mut f = Foo(&z)
    bar<Foo>(&mut f)
}
```

```error
test.met:11:14: reference escaping its allocation scope (via mutation of outer variable)
            let a = 123
            t.do(&a)
                 ^^
        }
```

## Noescape

**return ref directly**

```metall
{
    fun leak(x noescape &Int) &Int { x }
}
```

```error
test.met:2:14: noescape parameter "x" must not escape through the return value
    {
        fun leak(x noescape &Int) &Int { x }
                 ^^^^^^^^^^^^^^^
    }
```

**return deref value is ok**

```metall
{
    fun read(x noescape &Int) Int { x.* }
}
```

```error
```

**return inner ref from struct**

```metall
{
    struct Holder { r &Int }
    fun steal(h noescape &Holder) &Int { h.r }
}
```

```error
test.met:3:15: noescape parameter "h" must not escape through the return value
        struct Holder { r &Int }
        fun steal(h noescape &Holder) &Int { h.r }
                  ^^^^^^^^^^^^^^^^^^
    }
```

**return inner slice from struct**

```metall
{
    struct Data { items []Int }
    fun steal(d noescape &Data) []Int { d.items }
}
```

```error
test.met:3:15: noescape parameter "d" must not escape through the return value
        struct Data { items []Int }
        fun steal(d noescape &Data) []Int { d.items }
                  ^^^^^^^^^^^^^^^^
    }
```

**return noescape slice directly**

```metall
{
    fun steal(s noescape []Int) []Int { s }
}
```

```error
test.met:2:15: noescape parameter "s" must not escape through the return value
    {
        fun steal(s noescape []Int) []Int { s }
                  ^^^^^^^^^^^^^^^^
    }
```

**write value into &mut Int is ok**

```metall
{
    fun store(dst &mut Int, src noescape &Int) void { dst.* = src.* }
}
```

```error
```

**noescape ref flows into &mut param holding ref**

```metall
{
    struct Box { r &Int }
    fun store(dst &mut Box, src noescape &Int) void { dst.* = Box(src) }
}
```

```error
test.met:3:29: noescape parameter "src" must not escape through other parameters
        struct Box { r &Int }
        fun store(dst &mut Box, src noescape &Int) void { dst.* = Box(src) }
                                ^^^^^^^^^^^^^^^^^
    }
```

**noescape slice flows into &mut param holding slice**

A slice borrows its backing storage, so storing a `noescape` slice param into a
`&mut` param leaks it through that parameter, exactly like the ref case above.

```metall
{
    struct Buf { data []U8 }
    fun store(dst &mut Buf, src noescape []U8) void { dst.* = Buf(src) }
}
```

```error
test.met:3:29: noescape parameter "src" must not escape through other parameters
        struct Buf { data []U8 }
        fun store(dst &mut Buf, src noescape []U8) void { dst.* = Buf(src) }
                                ^^^^^^^^^^^^^^^^^
    }
```

**slice stored through a &mut param escapes a shorter-lived caller slice**

`store` records "dst gets src's borrow" as a side effect, so calling it with a
block-local array's slice and a longer-lived `buf` is caught at the call site.

```metall
{
    struct Buf { data []Int }
    fun store(dst &mut Buf, src []Int) void { dst.* = Buf(src) }
    fun main() void {
        mut buf = Buf([0][..])
        {
            mut arr = [111, 222, 333]
            store(&mut buf, arr[0..3])
        }
    }
}
```

```error
test.met:8:29: reference escaping its allocation scope (via mutation of outer variable)
                mut arr = [111, 222, 333]
                store(&mut buf, arr[0..3])
                                ^^^^^^^^^
            }
```

**slice stored through a &mut param into a same-scope place does not escape**

When the source slice and the `&mut` destination live in the same scope, the
borrow does not outlive its backing, so the side effect is not an escape.

```metall
{
    struct Buf { data []Int }
    fun store(dst &mut Buf, src []Int) void { dst.* = Buf(src) }
    fun main() void {
        mut arr = [111, 222, 333]
        mut buf = Buf([0][..])
        store(&mut buf, arr[0..3])
    }
}
```

```error
```

**function type with noescape: violating function rejected**

```metall
{
    fun id(x &Int) &Int { x }
    fun apply(f fun(noescape &Int) &Int) void {
        let x = 1
        let _ = f(&x)
    }
    apply(id)
}
```

```error
test.met:7:11: noescape parameter "param 0" must not escape through the return value
        }
        apply(id)
              ^^
    }
```

**function type with noescape: ok function accepted**

```metall
{
    fun read(x &Int) Int { x.* }
    fun apply(f fun(noescape &Int) Int) Int {
        let x = 42
        f(&x)
    }
    apply(read)
}
```

```error
```

**concrete method satisfies shape contract**

```metall
{
    shape S {
        fun S.do(s &mut S, x &Int) void
    }
    struct Foo { n Int }
    fun Foo.do(f &mut Foo, x &Int) void {
        f.n = 1
    }
    fun bar<T S>(t &mut T) void {
        let a = 123
        t.do(&a)
    }
    mut f = Foo(0)
    bar<Foo>(&mut f)
}
```

```error
```

## Noescape return references

**noescape return via struct: without noescape is ok**

```metall
{
    struct Holder { r &Int }
    fun get_ref(h &Holder) &Int { h.r }
    fun caller(h &Holder) &Int {
        get_ref(h)
    }
}
```

```error
```

**noescape return via struct: escape through return rejected**

```metall
{
    struct Holder { r &Int }
    fun get_ref(h &Holder) noescape &Int { h.r }
    fun caller(h &Holder) &Int {
        get_ref(h)
    }
}
```

```error
test.met:5:9: reference escaping its allocation scope (via block result)
        fun caller(h &Holder) &Int {
            get_ref(h)
            ^^^^^^^^^^
        }
```

**noescape return via closure: without noescape is ok**

```metall
{
    fun use_fn(f fun() &Int) &Int {
        f()
    }
}
```

```error
```

**noescape return via closure: escape through return rejected**

```metall
{
    fun use_fn(f fun() noescape &Int) &Int {
        f()
    }
}
```

```error
test.met:3:9: reference escaping its allocation scope (via block result)
        fun use_fn(f fun() noescape &Int) &Int {
            f()
            ^^^
        }
```

**noescape return: non-noescape function rejected for noescape return fun-type**

```metall
{
    fun get(x &Int) &Int { x }
    fun apply(f fun(&Int) noescape &Int) void {}
    apply(get)
}
```

```error
test.met:4:11: function does not return noescape
        fun apply(f fun(&Int) noescape &Int) void {}
        apply(get)
              ^^^
    }
```

**noescape return: noescape function accepted for noescape return fun-type**

```metall
{
    fun get(x &Int) noescape &Int { x }
    fun apply(f fun(&Int) noescape &Int) void {}
    apply(get)
}
```

```error
```

**noescape return confined to the call scope**

```metall
{
    fun a(x &Int) noescape &Int { x }
    mut outer = 0
    mut sink = &outer
    {
        sink = a(&outer)
    }
}
```

```error
test.met:6:9: reference escaping its allocation scope (via mutation of outer variable)
        {
            sink = a(&outer)
            ^^^^^^^^^^^^^^^^
        }
```

**without noescape the result may leave the call scope**

```metall
{
    fun a(x &Int) &Int { x }
    mut outer = 0
    mut sink = &outer
    {
        sink = a(&outer)
    }
}
```

```error
```

**noescape return used in place is fine**

```metall
{
    fun a(x &Int) noescape &Int { x }
    mut outer = 0
    let _ = a(&outer).*
}
```

```error
```

**noescape confines a ref field reached through the result**

```metall module
let g = 0
struct Holder { value &Int }
fun a(h &Holder) noescape &Holder { h }
fun main() void {
    mut h = Holder(&g)
    mut sink = &g
    {
        sink = a(&h).value
    }
}
```

```error
test.met:8:9: reference escaping its allocation scope (via mutation of outer variable)
        {
            sink = a(&h).value
            ^^^^^^^^^^^^^^^^^^
        }
```

**without noescape a reached ref field may leave**

```metall module
let g = 0
struct Holder { value &Int }
fun a(h &Holder) &Holder { h }
fun main() void {
    mut h = Holder(&g)
    mut sink = &g
    {
        sink = a(&h).value
    }
}
```

```error
```

**noescape confines a ref held inside a returned value**

```metall module
let g = 0
struct Holder { value &Int }
fun a(h Holder) noescape Holder { h }
fun main() void {
    let h = Holder(&g)
    mut sink = &g
    {
        sink = a(h).value
    }
}
```

```error
test.met:8:9: reference escaping its allocation scope (via mutation of outer variable)
        {
            sink = a(h).value
            ^^^^^^^^^^^^^^^^^
        }
```

**noescape on a non-reference return is rejected**

```metall
{
    fun first(x Int) noescape Int { x }
}
```

```error
test.met:2:31: noescape is meaningless on a return type that cannot carry a reference
    {
        fun first(x Int) noescape Int { x }
                                  ^^^
    }
```

**noescape on a non-reference parameter is rejected**

```metall
{
    fun first(n noescape Int) Int { n }
}
```

```error
test.met:2:15: noescape is meaningless on a parameter that cannot carry a reference
    {
        fun first(n noescape Int) Int { n }
                  ^^^^^^^^^^^^^^
    }
```

**noescape on a type parameter is allowed (may carry a reference once bound)**

```metall module
let g = 0
fun id<T>(x &T) noescape &T { x }
fun keep<T>(x &T) noescape T { x.* }
fun main() void { let _ = id<Int>(&g).* }
```

```error
```

**self-recursive noescape return escapes (forwards a noescape result)**

```metall module
let g = 0
fun deep(x &Int, n Int) noescape &Int {
    if n <= 0 { x } else { deep(x, n - 1) }
}
fun main() void {
    let _ = deep(&g, 3).*
}
```

```error
test.met:3:28: reference escaping its allocation scope (via block result)
    fun deep(x &Int, n Int) noescape &Int {
        if n <= 0 { x } else { deep(x, n - 1) }
                               ^^^^^^^^^^^^^^
    }
```

**noescape delegation: returning another noescape call's result escapes**

```metall
{
    mut g = 0
    fun inner(x &Int) noescape &Int { x }
    fun outer(x &Int) noescape &Int { inner(x) }
    let _ = outer(&g).*
}
```

```error
test.met:4:39: reference escaping its allocation scope (via block result)
        fun inner(x &Int) noescape &Int { x }
        fun outer(x &Int) noescape &Int { inner(x) }
                                          ^^^^^^^^
        let _ = outer(&g).*
```

**mutual recursion of noescape functions escapes**

```metall module
let g = 0
fun ping(x &Int, n Int) noescape &Int { if n <= 0 { x } else { pong(x, n - 1) } }
fun pong(x &Int, n Int) noescape &Int { if n <= 0 { x } else { ping(x, n - 1) } }
fun main() void { let _ = ping(&g, 3).* }
```

```error
test.met:3:64: reference escaping its allocation scope (via block result)
    fun ping(x &Int, n Int) noescape &Int { if n <= 0 { x } else { pong(x, n - 1) } }
    fun pong(x &Int, n Int) noescape &Int { if n <= 0 { x } else { ping(x, n - 1) } }
                                                                   ^^^^^^^^^^^^^^
    fun main() void { let _ = ping(&g, 3).* }

test.met:2:64: reference escaping its allocation scope (via block result)
    let g = 0
    fun ping(x &Int, n Int) noescape &Int { if n <= 0 { x } else { pong(x, n - 1) } }
                                                                   ^^^^^^^^^^^^^^
    fun pong(x &Int, n Int) noescape &Int { if n <= 0 { x } else { ping(x, n - 1) } }
```

**a non-noescape wrapper returning a noescape result escapes**

```metall
{
    fun inner(x &Int) noescape &Int { x }
    fun bad(x &Int) &Int { inner(x) }
}
```

```error
test.met:3:28: reference escaping its allocation scope (via block result)
        fun inner(x &Int) noescape &Int { x }
        fun bad(x &Int) &Int { inner(x) }
                               ^^^^^^^^
    }
```

**non-tail return of a noescape result from a non-noescape fun escapes**

```metall
{
    fun inner(x &Int) noescape &Int { x }
    fun bad(x &Int, c Bool) &Int {
        if c { return inner(x) }
        x
    }
}
```

```error
test.met:4:23: reference escaping its allocation scope (via return)
        fun bad(x &Int, c Bool) &Int {
            if c { return inner(x) }
                          ^^^^^^^^
            x
```

**non-tail return of a noescape result escapes even in a noescape fun**

```metall
{
    fun inner(x &Int) noescape &Int { x }
    fun good(x &Int, c Bool) noescape &Int {
        if c { return inner(x) }
        x
    }
}
```

```error
test.met:4:23: reference escaping its allocation scope (via return)
        fun good(x &Int, c Bool) noescape &Int {
            if c { return inner(x) }
                          ^^^^^^^^
            x
```

**block-wrapped noescape result extracted to the function body escapes**

```metall module
let g = 0
fun inner(x &Int) noescape &Int { x }
fun outer(x &Int) noescape &Int {
    let r = { inner(x) }
    r
}
fun main() void { let _ = outer(&g).* }
```

```error
test.met:4:15: reference escaping its allocation scope (via block result)
    fun outer(x &Int) noescape &Int {
        let r = { inner(x) }
                  ^^^^^^^^
        r
```

**noescape result stored into a struct field then returned escapes**

```metall module
let g = 0
struct Box { p &Int }
fun source() noescape &Int { &g }
fun extract() Box {
    mut b = Box(&g)
    b.p = source()
    b
}
fun main() void { let b = extract()  let _ = b.p.* }
```

```error
test.met:7:5: reference escaping its allocation scope (via block result)
        b.p = source()
        b
        ^
    }
```

**noescape result stored into an array element then returned escapes**

```metall module
let g = 0
fun source() noescape &Int { &g }
fun extract() [2]&Int {
    mut arr = [&g, &g]
    arr[0] = source()
    arr
}
fun main() void { let a = extract()  let _ = a[0].* }
```

```error
test.met:6:5: reference escaping its allocation scope (via block result)
        arr[0] = source()
        arr
        ^^^
    }
```

**confined ref captured into a body-local closure is fine**

```metall module
let g = 0
fun source() noescape &Int { &g }
fun mk() noescape &Int {
    let r = source()
    let f = fun[r]() &Int { r }
    &g
}
fun main() void { let _ = mk().* }
```

```error
```

**closure escaping with a confined capture escapes**

```metall module
let g = 0
fun source() noescape &Int { &g }
fun mk() fun() &Int {
    let r = source()
    fun[r]() &Int { r }
}
fun main() void { let f = mk()  let _ = f().* }
```

```error
test.met:5:5: reference escaping its allocation scope (via block result)
        let r = source()
        fun[r]() &Int { r }
        ^^^^^^^^^^^^^^^^^^^
    }
```

**mutual recursion through a non-noescape entry leaks and escapes**

```metall module
let g = 0
fun src() noescape &Int { &g }
fun ping(n Int) noescape &Int { if n <= 0 { src() } else { pong(n - 1) } }
fun pong(n Int) &Int { ping(n) }
fun main() void {
    mut sink = &g
    {
        sink = pong(3)
    }
    let _ = sink.*
}
```

```error
test.met:3:45: reference escaping its allocation scope (via block result)
    fun src() noescape &Int { &g }
    fun ping(n Int) noescape &Int { if n <= 0 { src() } else { pong(n - 1) } }
                                                ^^^^^
    fun pong(n Int) &Int { ping(n) }

test.met:4:24: reference escaping its allocation scope (via block result)
    fun ping(n Int) noescape &Int { if n <= 0 { src() } else { pong(n - 1) } }
    fun pong(n Int) &Int { ping(n) }
                           ^^^^^^^
    fun main() void {
```

**noescape closure return is confined like any reference**

```metall module
let g = 0
fun ac() noescape fun() &Int { fun[]() &Int { &g } }
fun main() void {
    mut csink = fun[]() &Int { &g }
    {
        csink = ac()
    }
    let _ = csink().*
}
```

```error
test.met:6:9: reference escaping its allocation scope (via mutation of outer variable)
        {
            csink = ac()
            ^^^^^^^^^^^^
        }
```

## Generic instantiation

A generic function is analyzed per concrete instantiation, so borrows its type
parameter carries are tracked. Analyzing the generic body once with an abstract
T loses them (typeContainsRefOrAlloc(T) is false), which is why the effects are
recomputed against each instance's concrete env.

**borrow carried through a generic type parameter escapes**

```metall module
let g = 0
struct Box<T> { value T }
fun unbox<T>(b Box<T>) T { b.value }
fun main() void {
    mut sink = &g
    {
        let inner = 5
        sink = unbox<&Int>(Box<&Int>(&inner))
    }
}
```

```error
test.met:8:38: reference escaping its allocation scope (via mutation of outer variable)
            let inner = 5
            sink = unbox<&Int>(Box<&Int>(&inner))
                                         ^^^^^^
        }
```

**generic noescape return is enforced**

```metall module
let g = 0
fun borrow<T>(x &T) noescape &T { x }
fun main() void {
    mut sink = &g
    {
        sink = borrow<Int>(&g)
    }
}
```

```error
test.met:6:9: reference escaping its allocation scope (via mutation of outer variable)
        {
            sink = borrow<Int>(&g)
            ^^^^^^^^^^^^^^^^^^^^^^
        }
```

**generic noescape return used in place is fine**

```metall module
let g = 0
fun borrow<T>(x &T) noescape &T { x }
fun main() void {
    let _ = borrow<Int>(&g).*
}
```

```error
```

**generic self-recursive noescape return escapes**

```metall module
let g = 0
fun deep<T>(x &T, n Int) noescape &T {
    if n <= 0 { x } else { deep<T>(x, n - 1) }
}
fun main() void {
    let _ = deep<Int>(&g, 3).*
}
```

```error
test.met:3:28: reference escaping its allocation scope (via block result)
    fun deep<T>(x &T, n Int) noescape &T {
        if n <= 0 { x } else { deep<T>(x, n - 1) }
                               ^^^^^^^^^^^^^^^^^
    }
```

## References To Temporaries

**ref to call result used in place is fine**

```metall
{
    fun make() Int { 1 }
    fun take(x &Int) void {}
    take(&make())
}
```

```error
```

**ref to call result cannot escape via block result**

```metall
{
    fun make() Int { 1 }
    let r = {
        &make()
    }
    r
}
```

```error
test.met:4:9: reference escaping its allocation scope (via block result)
        let r = {
            &make()
            ^^^^^^^
        }
```

**mut ref to call result cannot escape via return**

```metall
{
    fun make() Int { 1 }
    fun leak() &mut Int { &mut make() }
    leak()
}
```

```error
test.met:3:27: reference escaping its allocation scope (via block result)
        fun make() Int { 1 }
        fun leak() &mut Int { &mut make() }
                              ^^^^^^^^^^^
        leak()
```

**ref to struct literal cannot escape via return**

```metall
{
    struct Pair {
        a Int
        b Int
    }
    fun leak() &Pair { &Pair(1, 2) }
    leak()
}
```

```error
test.met:6:24: reference escaping its allocation scope (via block result)
        }
        fun leak() &Pair { &Pair(1, 2) }
                           ^^^^^^^^^^^
        leak()
```

## Shape contract verification

**concrete impl with side effect into self escapes via shape call**

```metall
{
    shape S {
        fun S.do(s &mut S, x &Int) void
    }
    struct Bag { r &Int }
    fun Bag.do(b &mut Bag, x &Int) void {
        b.r = x
    }
    fun caller<T S>(t &mut T) void {
        let a = 1
        t.do(&a)
    }
    mut b = Bag(&0)
    caller<Bag>(&mut b)
}
```

```error
test.met:11:14: reference escaping its allocation scope (via mutation of outer variable)
            let a = 1
            t.do(&a)
                 ^^
        }
```

## Deref-assign through ref/heap projection

`let pp = &mut b.p; pp.* = <inner ref>` must escape when `b` is a value, a heap
allocation, or a ref: `b.p` lives in storage that outlives the inner reference.

**deref-assign into stack struct field escapes**

```metall
{
    struct Box { p &Int }
    mut seed = 0
    mut b = Box(&seed)
    {
        mut w = 99
        let pp = &mut b.p
        pp.* = &w
    }
}
```

```error
test.met:8:16: reference escaping its allocation scope (via deref assignment)
            let pp = &mut b.p
            pp.* = &w
                   ^^
        }
```

**deref-assign into heap struct field escapes**

```metall
{
    struct Box { p &Int }
    let @a = Arena()
    mut seed = 0
    let b = @a.new<Box>(Box(&seed))
    {
        mut w = 99
        let pp = &mut b.p
        pp.* = &w
    }
}
```

```error
test.met:9:16: reference escaping its allocation scope (via deref assignment)
            let pp = &mut b.p
            pp.* = &w
                   ^^
        }
```

**deref-assign into ref-projected struct field escapes**

```metall
{
    struct Box { p &Int }
    let @a = Arena()
    let inner = @a.new<Int>(0)
    mut holder = Box(inner)
    let b = &mut holder
    {
        mut w = 99
        let pp = &mut b.p
        pp.* = &w
    }
}
```

```error
test.met:10:16: reference escaping its allocation scope (via deref assignment)
            let pp = &mut b.p
            pp.* = &w
                   ^^
        }
```

**for &x deref-assign into heap struct field escapes**

```metall
{
    struct Box { p &Int }
    let @a = Arena()
    mut seed = 0
    let b = @a.new<Box>(Box(&seed))
    for &x in [1, 2, 3] {
        let pp = &mut b.p
        pp.* = x
    }
}
```

```error
test.met:8:9: reference escaping its allocation scope (via deref assignment)
            let pp = &mut b.p
            pp.* = x
            ^^^^^^^^
        }
```

**deref-assign into heap field with no alias escapes**

The referent's field also points at the heap, so there is no alias to write
through; the escape is caught from the storage taint `pp` carries.

```metall
{
    struct Box { p &Int }
    let @a = Arena()
    let inner = @a.new<Int>(0)
    let b = @a.new<Box>(Box(inner))
    {
        mut w = 99
        let pp = &mut b.p
        pp.* = &w
    }
}
```

```error
test.met:9:16: reference escaping its allocation scope (via deref assignment)
            let pp = &mut b.p
            pp.* = &w
                   ^^
        }
```

**deref-assign into same-scope heap field is fine**

```metall
{
    struct Box { p &Int }
    let @a = Arena()
    let inner = @a.new<Int>(0)
    let b = @a.new<Box>(Box(inner))
    mut w = 99
    let pp = &mut b.p
    pp.* = &w
}
```

```error
```

**deref-assign through conditional two-arena referent escapes**

`sel` may point into `@outer` or `@inner`, so `pp` carries both storage taints.
Writing a middle-scope ref escapes when the referent is the longer-lived one.

```metall
{
    struct Box { p &Int }
    let @outer = Arena()
    let oi = @outer.new<Int>(0)
    let bo = @outer.new<Box>(Box(oi))
    {
        mut mid = 7
        {
            let @inner = Arena()
            let ii = @inner.new<Int>(0)
            let bi = @inner.new<Box>(Box(ii))
            let sel = if true { bo } else { bi }
            let pp = &mut sel.p
            pp.* = &mid
        }
    }
}
```

```error
test.met:14:20: reference escaping its allocation scope (via deref assignment)
                let pp = &mut sel.p
                pp.* = &mid
                       ^^^^
            }
```

## Overhaul guard + target

These two pin down the storage-vs-value distinction the analysis must keep
straight. `&x` must mean "the storage where x lives", never "what x points at".

**guard: deref-assign rewriting a same-scope ref must stay fine**

`pp` points at `local`'s own slot (this scope), not at `outer`. Overwriting it
with a same-scope ref is safe and must not be flagged.

```metall
{
    mut outer = 0
    {
        mut local = &outer
        mut pp = &mut local
        mut z = 5
        pp.* = &z
    }
}
```

```error
```

**target: deref-assign through a merged stack/heap pointer escapes**

`sel` may be the stack box (this scope) or the heap box (`@outer`). Writing a
middle-scope ref must escape, because the referent might be the longer-lived
heap one. This is currently a false negative.

```metall
{
    struct Box { p &Int }
    let @outer = Arena()
    let oi = @outer.new<Int>(0)
    let bh = @outer.new<Box>(Box(oi))
    {
        mut mid = 7
        {
            mut sbox = Box(oi)
            let sel = if true { &mut sbox } else { bh }
            let pp = &mut sel.p
            pp.* = &mid
        }
    }
}
```

```error
test.met:12:20: reference escaping its allocation scope (via deref assignment)
                let pp = &mut sel.p
                pp.* = &mid
                       ^^^^
            }
```

## Multi-level deref soundness

The deref-chain model is sound at any deref depth. `chain[d]` gives the exact
depth-d storage, so a write through `w.*.*` is checked against the depth-2 slot,
not conflated with the depth-1 slot. carriedTaints (the union of all levels) is
the conservative reach used for block-result and return escapes.

**depth-2 deref write of a dangling ref escapes**

`w.*.*` writes into `y`'s slot (outer). `&v` (inner) dangles there.

```metall
{
    mut x = 0
    mut y = &x
    {
        mut z = &mut y
        mut w = &mut z
        mut v = 99
        w.*.* = &v
    }
}
```

```error
test.met:6:17: reference escaping its allocation scope (via deref assignment)
            mut z = &mut y
            mut w = &mut z
                    ^^^^^^
            mut v = 99
```

**depth-3 deref write of a dangling ref escapes**

`u.*.*.*` writes into `y`'s slot (outer). `&v` (inner) dangles there.

```metall
{
    mut x = 0
    mut y = &x
    {
        mut z = &mut y
        mut w = &mut z
        mut u = &mut w
        mut v = 99
        u.*.*.* = &v
    }
}
```

```error
test.met:6:17: reference escaping its allocation scope (via deref assignment)
            mut z = &mut y
            mut w = &mut z
                    ^^^^^^
            mut u = &mut w
```

**depth-2 write through pointer-to-pointer struct field escapes**

`ppb.*.*.p` writes into `b`'s `p` field (outer). `&local` (inner) dangles there.

```metall
{
    struct Box { p &Int }
    mut x = 0
    mut b = Box(&x)
    {
        mut pb = &mut b
        mut ppb = &mut pb
        mut local = 99
        ppb.*.*.p = &local
    }
}
```

```error
test.met:7:19: reference escaping its allocation scope (via mutation of outer variable)
            mut pb = &mut b
            mut ppb = &mut pb
                      ^^^^^^^
            mut local = 99
```

**same-scope depth-2 deref write does not escape**

`w.*.*` writes into `y`'s slot, which is the same scope as `&v`'s referent, so
nothing dangles.

```metall
{
    mut x = 0
    mut y = &x
    mut z = &mut y
    mut w = &mut z
    mut v = 99
    w.*.* = &v
}
```

```error
```

**closure side-effect into captured &mut struct field escapes**

The closure captures `box` by `&mut` and writes a body-local ref into its field.
`box` resolves to the outer scope, so `&local` (closure-body-local) escapes.

```metall
fun foo() void {
    struct Box { p &Int }
    mut x = 0
    mut box = Box(&x)
    let store = fun[&mut box]() void {
        mut local = 99
        box.p = &local
    }
    store()
}
```

```error
test.met:7:17: reference escaping its allocation scope (via mutation of outer variable)
            mut local = 99
            box.p = &local
                    ^^^^^^
        }
```

**closure side-effect into captured &mut struct field with a same-scope ref does not escape**

The closure captures `box` by `&mut` and `other` by `&`, then writes the
same-scope ref `other` into the field. Nothing dangles.

```metall
fun foo() void {
    struct Box { p &Int }
    mut x = 0
    mut other = 7
    mut box = Box(&x)
    let store = fun[&mut box, &other]() void {
        box.p = other
    }
    store()
}
```

```error
```

## Closure escape soundness

A closure that captures an inner local and is INVOKED must not lose the captured
taint at the call boundary. The captured taints live in the closure VALUE's
chain; merging the callee's chain into a call result (when the return can
escape) carries them through a direct call and through a higher-order function.

**captured local returned through closure call escapes**

```metall
fun bar() &Int {
    mut local = 99
    let g = fun[&local]() &Int { local }
    g()
}
```

```error
test.met:4:5: reference escaping its allocation scope (via block result)
        let g = fun[&local]() &Int { local }
        g()
        ^^^
    }
```

**captured local escapes through closure call bound to a let**

Inline-call syntax `(fun...)()` does not type-check, so this is the let-bound
variant: the call result flows into `r`, then escapes via the block result.

```metall
fun bar() &Int {
    mut local = 99
    let g = fun[&local]() &Int { local }
    let r = g()
    r
}
```

```error
test.met:5:5: reference escaping its allocation scope (via block result)
        let r = g()
        r
        ^
    }
```

**captured local escapes through a higher-order function**

```metall
{
    fun apply(f fun() &Int) &Int { f() }
    fun foo() &Int {
        mut local = 99
        apply(fun[&local]() &Int { local })
    }
}
```

```error
test.met:5:9: reference escaping its allocation scope (via block result)
            mut local = 99
            apply(fun[&local]() &Int { local })
            ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
        }
```

**captured-by-value ref returned through closure call escapes**

```metall
fun bar() &Int {
    mut local = 99
    let r = &local
    let g = fun[r]() &Int { r }
    g()
}
```

```error
test.met:3:13: reference escaping its allocation scope (via block result)
        mut local = 99
        let r = &local
                ^^^^^^
        let g = fun[r]() &Int { r }
```

**closure call returning a captured param does not escape**

```metall
fun bar(outer &Int) &Int {
    let g = fun[outer]() &Int { outer }
    g()
}
```

```error
```

**closure read out of a struct field keeps its captures**

A closure stored in a struct field and read back through `h.f` is hidden behind
a projection, so the call result must still carry the capture of `local`.

```metall
fun bar() &Int {
    struct Holder { f fun() &Int }
    mut local = 99
    let h = Holder(fun[&local]() &Int { local })
    let fn = h.f
    fn()
}
```

```error
test.met:6:5: reference escaping its allocation scope (via block result)
        let fn = h.f
        fn()
        ^^^^
    }
```

**closure returning a longer-lived capture does not escape an unreturned capture**

The closure captures `local` (body-local) but does NOT return it; it returns the
caller's param `p`, so the call result must not be rejected.

```metall
fun bar(p &Int) &Int {
    mut local = 99
    let g = fun[&local, p]() &Int { p }
    g()
}
```

```error
```

**by-ref capture alongside a param capture returns the local and escapes**

Mirror of the previous case, but the body RETURNS the captured `local` (not the
param `p`), so the call result dangles.

```metall
fun bar(p &Int) &Int {
    mut local = 99
    let g = fun[&local, p]() &Int { local }
    g()
}
```

```error
test.met:4:5: reference escaping its allocation scope (via block result)
        let g = fun[&local, p]() &Int { local }
        g()
        ^^^
    }
```

**closure in a nested struct read two projections deep then called escapes**

The closure capturing `&local` is stored two struct levels deep
(`Outer.inner.f`); reading it back through `o.inner.f` must still carry the
capture, so the call result dangles.

```metall
fun bar() &Int {
    struct Inner { f fun() &Int }
    struct Outer { inner Inner }
    mut local = 99
    let o = Outer(Inner(fun[&local]() &Int { local }))
    o.inner.f()
}
```

```error
test.met:6:5: reference escaping its allocation scope (via block result)
        let o = Outer(Inner(fun[&local]() &Int { local }))
        o.inner.f()
        ^^^^^^^^^^^
    }
```

## Accepted false positives

We accept the following false positives to keep the implementation lean. We might
revisit them later.

**closure captures are merged into a single taint set**

So crossing an abstraction boundary makes the analyzer attribute ALL of a
closure's captures to a CALL result. Sound (never a missed dangling), but it
rejects valid programs. Here `apply` returns the closure's result, the long-lived
param `p`, so nothing dangles, yet the captured local `scratch` is wrongly
reported escaping.

```metall
{
    fun apply(f fun() &Int) &Int { f() }
    fun foo(p &Int) &Int {
        mut scratch = 99
        apply(fun[&scratch, p]() &Int {
            let _ = scratch.*
            p
        })
    }
}
```

```error
test.met:5:9: reference escaping its allocation scope (via block result)
            mut scratch = 99
            apply(fun[&scratch, p]() &Int {
            ^
                let _ = scratch.*
                p
            })
             ^
        }
```

## Match reference bindings

A `case Foo &x` / `case Foo &mut x` binding aliases the matched value's
storage, so escape analysis must treat the binding as borrowing the matched value.

**ref binding escapes when it borrows a local union**

```metall
{
    struct A { v Int }
    struct B { v Int }
    union U = A | B
    fun foo() &mut A {
        mut u = U(A(1))
        match &mut u {
            case A &mut x: return x
            case B &mut y: return foo()
        }
    }
}
```

```error
test.met:7:15: reference escaping its allocation scope (via return)
            mut u = U(A(1))
            match &mut u {
                  ^^^^^^
                case A &mut x: return x
```

**ref binding borrowing a reference parameter is valid**

```metall
{
    struct A { v Int }
    struct B { v Int }
    union U = A | B
    fun foo(u &mut U) &mut A {
        match u {
            case A &mut x: return x
            case B &mut y: return foo(u)
        }
    }
}
```

```error
```

**reading through a ref binding stays local**

```metall
{
    struct A { v Int }
    struct B { v Int }
    union U = A | B
    fun sum(u &U) Int {
        match u {
            case A &x: x.v
            case B &y: y.v
        }
    }
}
```

```error
```

**else ref binding escapes when it borrows a local union**

```metall
{
    struct A { v Int }
    struct B { v Int }
    union U = A | B
    fun foo() &A {
        mut u = U(A(1))
        match &u {
            case B &b: return foo()
            else &a: return a
        }
    }
}
```

```error
test.met:7:15: reference escaping its allocation scope (via return)
            mut u = U(A(1))
            match &u {
                  ^^
                case B &b: return foo()
```

**storing a ref binding into an outer variable escapes**

```metall
{
    struct A { v Int }
    struct B { v Int }
    union U = A | B
    mut sink = A(0)
    mut keep = &mut sink
    {
        mut u = U(A(1))
        match &mut u {
            case A &mut x: keep = x
            case B &mut y: keep = &mut sink
        }
    }
}
```

```error
test.met:9:15: reference escaping its allocation scope (via mutation of outer variable)
            mut u = U(A(1))
            match &mut u {
                  ^^^^^^
                case A &mut x: keep = x
```

**mutating in place through a ref binding is valid**

```metall
{
    struct A { v Int }
    struct B { v Int }
    union U = A | B
    fun bump(u &mut U) void {
        match u {
            case A &mut x: x.v = x.v + 1
            case B &mut y: y.v = y.v + 1
        }
    }
}
```

```error
```

**reading enum associated data through a ref binding is valid**

```metall
{
    enum Coin(cents Int) U8 = penny(1) | dime(10)
    fun worth(c &Coin) Int {
        match c {
            case Coin.penny &p: p.cents
            case Coin.dime &d: d.cents
        }
    }
}
```

```error
```
