# Lifetime Tests

**stack ref escapes**

```metall
let x = { let y = 123 &y }
```

```error
test.met:1:23: reference escaping its allocation scope (via block result)
    let x = { let y = 123 &y }
                          ^^
```

**assign ref to outer**

```metall
{ mut x = 123 mut y = &x { mut z = 123 y = &z } }
```

```error
test.met:1:44: reference escaping its allocation scope (via mutation of outer variable)
    { mut x = 123 mut y = &x { mut z = 123 y = &z } }
                                               ^^
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
    struct Foo { mut one &Int }
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
    struct Foo { mut one &Int }
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
        @myalloc.slice_uninit<Int>(5)
    }
    x
}
```

```error
test.met:4:9: reference escaping its allocation scope (via block result)
            let @myalloc = Arena()
            @myalloc.slice_uninit<Int>(5)
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
        @a.slice_mut<Wrapper>(3, Wrapper(&local))
    }
}
```

```error
test.met:6:42: reference escaping its allocation scope (via block result)
            let local = 123
            @a.slice_mut<Wrapper>(3, Wrapper(&local))
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
        @a.slice_mut<&Int>(3, &local)
    }
}
```

```error
test.met:5:31: reference escaping its allocation scope (via block result)
            let local = 123
            @a.slice_mut<&Int>(3, &local)
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
    struct Wrapper { mut one &Int }
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
    struct Wrapper { mut one &Int }
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
    struct Foo { mut one [1]&mut Int }
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
    struct Foo { mut one [1]&mut Int }
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
    print_int(s[0])
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
{ fun foo(a &mut Int) void { a.* = 321 } mut x = 123 foo(&mut x) }
```

```error
```

**field mutation bypass**

```metall
{
    struct Foo { mut one &Int }
    fun foo(a &mut Foo, b &Int) void {
        a.one = b
    }
    mut x = 42
    mut y = Foo(&mut x)
    {
        mut z = 99
        foo(&mut y, &z)
    }
    print_int(y.one.*)
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
    struct Foo { mut one &Int }
    fun identity(a &mut Foo) &mut Foo { a }
    mut x = 42
    mut y = Foo(&mut x)
    {
        mut z = 99
        let w = identity(&mut y)
        w.one = &z
    }
    print_int(y.one.*)
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
    struct Foo { mut one &Int }
    fun identity(a &mut Foo) &mut Foo { a }
    mut x = 12742
    mut y = Foo(&mut x)
    {
        mut z = 99
        let w = identity(&mut y)
        w.one = &z
    }
    print_int(y.one.*)
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
    struct Foo { mut one &Int }
    fun foo(a &mut Foo, b &Int) void { a.one = b }
    let @myalloc = Arena()
    mut x = 1
    let y = @myalloc.new_mut<Foo>(Foo(&mut x))
    {
        mut z = 99
        foo(y, &z)
    }
    print_int(y.one.*)
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
    struct Foo { mut one &Int }
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
    print_int(y.one.*)
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
        mut one &Int
    }

    fun foo(a &mut Foo, b &Int) Int {
        bar(a, b)
        42
    }

    fun bar(a &mut Foo, b &Int) Int {
        foo(a, b)
        1337
    }

    mut x = 0
    mut y = Foo(&mut x)

    {
        mut z = 99
        foo(&mut y, &z)
    }

    print_int(y.one.*)
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
    struct Foo { mut one &Int }
    fun identity(a &mut Foo) &mut Foo { a }
    fun foo(a &mut Foo, b &Int) void { a.one = b }

    mut x = 12742
    mut y = Foo(&mut x)
    {
        mut z = 99
        foo(identity(&mut y), &z)
    }
    print_int(y.one.*)
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
    struct Foo { mut one &Int }
    mut x = 12742
    mut y = Foo(&mut x)
    mut z = &mut y
    mut w = &mut z
    {
        mut a = 99
        mut b = Foo(&mut a)
        w.*.* = b
    }
    print_int(y.one.*)
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
    struct Foo { mut one &mut &Int }
    mut x = 12742
    mut y = &x
    mut z = Foo(&mut y)
    {
        mut w = 99
        z.one.* = &w
    }
    print_int(z.one.*.*)
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
    struct Foo { mut one Str  mut two &Int }
    mut x = 12742
    mut y = Foo("hello", &x)
    {
        mut z = 99
        y.two = &z
        y.one = "bye"
    }
    print_int(y.two.*)
}
```

```error
test.met:7:17: reference escaping its allocation scope (via mutation of outer variable)
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
    fun bar(t Foo) &Int { let a = 42 t.escape(&a) }
    let r = {
        let f = Foo()
        bar(f)
    }
    r
}
```

```error
test.met:4:47: reference escaping its allocation scope (via block result)
        fun Foo.escape(f Foo, a &Int) &Int { a }
        fun bar(t Foo) &Int { let a = 42 t.escape(&a) }
                                                  ^^
        let r = {
```

**shape method ref escapes**

```metall
{
    shape Shape {
        fun Shape.escape(s Shape, a &Int) &Int
    }
    struct Foo { }
    fun Foo.escape(f Foo, a &Int) &Int { a }
    fun bar<T Shape>(t T) &Int { let a = 42 t.escape(&a) }
    let r = {
        let f = Foo()
        bar<Foo>(f)
    }
    r
}
```

```error
test.met:7:54: reference escaping its allocation scope (via block result)
        fun Foo.escape(f Foo, a &Int) &Int { a }
        fun bar<T Shape>(t T) &Int { let a = 42 t.escape(&a) }
                                                         ^^
        let r = {
```

**shape field ref escapes**

```metall
{
    shape Shape { 
        mut one &Int 
    }
    
    fun foo<T Shape>(s &mut T) void {
        let x = 42
        s.one = &x
    }
}
```

```error
test.met:8:17: reference escaping its allocation scope (via mutation of outer variable)
            let x = 42
            s.one = &x
                    ^^
        }
```

**shape mut param ref escapes via side effect**

```metall
{
    shape HasRef { mut one &Int }
    struct Foo { mut one &Int }
    struct Bar { mut one &Int }
    fun baz<T HasRef>(t &mut T, b &Int) void { t.one = b }
    mut x = 42
    mut foo = Foo(&mut x)
    {
        let z = 99
        baz<Foo>(&mut foo, &z)
    }
}
```

```error
test.met:10:28: reference escaping its allocation scope (via mutation of outer variable)
            let z = 99
            baz<Foo>(&mut foo, &z)
                               ^^
        }
```

**shape value param no escape when not written to mut ref**

```metall
{
    shape HasRef { one &Int }
    struct Foo { one &Int }
    struct Bar { mut one &Int }
    fun baz<T HasRef>(t T, b &mut Bar) void { b.one = b.one }
    let x = 42
    mut y = Bar(&x)
    {
        let z = 99
        baz<Foo>(Foo(&z), &mut y)
    }
    print_int(y.one.*)
}
```

```error
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

**shape value param ref escapes via side effect**

```metall
{
    shape HasRef { one &Int }
    struct Foo { one &Int }
    struct Bar { mut one &Int }
    fun baz<T HasRef>(t T, b &mut Bar) void { b.one = t.one }
    let x = 42
    mut y = Bar(&x)
    {
        let z = 99
        baz<Foo>(Foo(&z), &mut y)
    }
    print_int(y.one.*)
}
```

```error
test.met:10:22: reference escaping its allocation scope (via mutation of outer variable)
            let z = 99
            baz<Foo>(Foo(&z), &mut y)
                         ^^
        }
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
            else v:
                match v {
                    case Int: &x
                    case Wrapper w: w.one
                }
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

test.met:6:30: reference escaping its allocation scope (via return)
            match u {
                case Int: return &local
                                 ^^^^^^
                case Bool: return &local

test.met:6:30: reference escaping its allocation scope (via block result)
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
        case Int n: print_int(n)
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
        case &Int r: print_int(r.*)
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
        case Int n if n > 10: print_int(n)
        case Int n: print_int(n)
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

