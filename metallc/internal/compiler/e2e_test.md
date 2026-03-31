# E2E Tests

## Compile

**print str**

```metall
fun main() void { DebugIntern.print_str("hello") }
```

```output
hello
```

**int literal**

```metall
fun main() void { DebugIntern.print_int(123) }
```

```output
123
```

**str var**

```metall
fun main() void { let x = "hello" DebugIntern.print_str(x) }
```

```output
hello
```

**int var**

```metall
fun main() void { let x = 123 DebugIntern.print_int(x) }
```

```output
123
```

**bool var**

```metall
fun main() void { let x = true DebugIntern.print_bool(x) }
```

```output
true
```

**mut var reassign**

```metall
fun main() void { mut x = 123 DebugIntern.print_int(x) x = 456 DebugIntern.print_int(x) }
```

```output
123
456
```

**fun returns int**

```metall
fun foo() Int { 123 } fun main() void { DebugIntern.print_int(foo()) }
```

```output
123
```

**fun returns str**

```metall
fun foo() Str { "hello" } fun main() void { DebugIntern.print_str(foo()) }
```

```output
hello
```

**fun returns bool**

```metall
fun foo() Bool { true } fun main() void { DebugIntern.print_bool(foo()) }
```

```output
true
```

**fun with int param**

```metall
fun foo(a Int) Int { a } fun main() void { DebugIntern.print_int(foo(123)) }
```

```output
123
```

**fun with str param**

```metall
fun foo(a Str) Str { a } fun main() void { let x = foo("hello") DebugIntern.print_str(x) }
```

```output
hello
```

**fun with bool param**

```metall
fun foo(a Bool) Bool { a } fun main() void { DebugIntern.print_bool(foo(true)) }
```

```output
true
```

**fun with return**

```metall
fun foo() Int { return 123 } fun main() void { DebugIntern.print_int(foo()) }
```

```output
123
```

**fun with return struct**

```metall
struct Foo { one Str }
fun foo() Foo { return Foo("hello") } 
fun main() void { DebugIntern.print_str(foo().one) }
```

```output
hello
```

**fun with multiple return**

```metall
fun foo(a Int) Int { 
    if a != 2 {
        if a == 0 {
            return 100
        } else {
            "just some expr"
        }
        return 101
    }
    return a + 200
} 

fun main() void { 
    DebugIntern.print_int(foo(0)) 
    DebugIntern.print_int(foo(1)) 
    DebugIntern.print_int(foo(2)) 
}
```

```output
100
101
202
```

**return void in if**

```metall
fun foo(x Int) void {
    if x > 0 {
        return void
    }
    DebugIntern.print_int(x)
}
fun main() void { foo(1) foo(0) }
```

```output
0
```

**free flowing fun**

```metall
fun double(a Int) Int { a + a }

fun math(a Int, f fun(Int) Int) Int { f(a) }

fun main() void {
    DebugIntern.print_int(math(7, double))
    let local_math = math
    let local_double = double
    DebugIntern.print_int(local_math(9, double))
}
```

```output
14
18
```

**free flowing fun reassign**

```metall
fun double(a Int) Int { a + a }
fun triple(a Int) Int { a + a + a }

fun main() void {
    mut f = double
    DebugIntern.print_int(f(5))
    f = triple
    DebugIntern.print_int(f(5))
}
```

```output
10
15
```

**free flowing fun qualified call**

```metall
struct Foo { x Int }
fun Foo.get(self Foo) Int { self.x }

fun main() void {
    let f = Foo(42)
    DebugIntern.print_int(Foo.get(f))
    let g = Foo.get
    DebugIntern.print_int(g(f))
}
```

```output
42
42
```

**free flowing fun return**

```metall
fun double(a Int) Int { a + a }

fun get_double() fun(Int) Int { double }

fun main() void {
    let f = get_double()
    DebugIntern.print_int(f(5))
}
```

```output
10
```

**free flowing fun in struct field**

```metall
fun double(a Int) Int { a + a }

struct Foo { one fun(Int) Int }

fun main() void {
    let x = Foo(double)
    DebugIntern.print_int(x.one(5))
    let y = x.one
    DebugIntern.print_int(y(5))
}
```

```output
10
10
```

**free flowing fun higher order**

```metall
fun double(a Int) Int { a + a }
fun apply_twice(f fun(Int) Int, x Int) Int { f(f(x)) }
fun main() void {
    DebugIntern.print_int(apply_twice(double, 3))
}
```

```output
12
```

**free flowing fun recursive as value**

```metall
fun factorial(n Int) Int {
    if n == 0 { 1 } else { n * factorial(n - 1) }
}
fun apply(f fun(Int) Int, x Int) Int { f(x) }
fun main() void {
    DebugIntern.print_int(apply(factorial, 5))
}
```

```output
120
```

**free flowing fun builtin as value**

```metall
fun apply(f fun(Int) void, x Int) void { f(x) }
fun main() void {
    let f = DebugIntern.print_int
    f(42)
    apply(DebugIntern.print_int, 99)
}
```

```output
42
99
```

**free flowing fun if branches**

```metall
fun double(a Int) Int { a + a }
fun triple(a Int) Int { a + a + a }
fun pick(use_double Bool) fun(Int) Int {
    if use_double { double } else { triple }
}
fun main() void {
    let f = pick(true)
    DebugIntern.print_int(f(5))
    let g = pick(false)
    DebugIntern.print_int(g(5))
}
```

```output
10
15
```

**nested fun**

```metall
fun foo() Int { 321 }
fun main() void {
    fun foo() Int { 
        fun foo() Int { 123 }
        foo()
    }
    DebugIntern.print_int(foo())
}
```

```output
123
```

**nested fun mutual recursion**

```metall
fun main() void {
    fun is_even(n Int) Bool {
        if n == 0 { true } else { is_odd(n - 1) }
    }
    fun is_odd(n Int) Bool {
        if n == 0 { false } else { is_even(n - 1) }
    }
    DebugIntern.print_bool(is_even(4))
    DebugIntern.print_bool(is_odd(4))
}
```

```output
true
false
```

**nested fun as value**

```metall
fun apply(f fun(Int) Int, x Int) Int { f(x) }
fun main() void {
    fun double(a Int) Int { a + a }
    DebugIntern.print_int(apply(double, 21))
}
```

```output
42
```

**nested struct**

```metall
struct Foo { x Int }
fun Foo.foo(self Foo) Int { self.x }

fun main() void {
    struct Foo { x Int y Str }
    fun Foo.bar(f Foo) Int { f.x + 1 }
    let f = Foo(42, "hello")
    DebugIntern.print_int(f.bar())
    DebugIntern.print_str(f.y)
}
```

```output
43
hello
```

**nested fun same name different scopes**

```metall
fun main() void {
    fun foo() Int {
        fun helper() Int { 10 }
        helper()
    }
    fun bar() Int {
        fun helper() Int { 20 }
        helper()
    }
    DebugIntern.print_int(foo() + bar())
}
```

```output
30
```

**block expression**

```metall
fun main() void { let x = { "hello" } DebugIntern.print_str(x) }
```

```output
hello
```

**var expr is void**

```metall
fun main() void { DebugIntern.print_str("hello") let x = 123 }
```

```output
hello
```

**assign expr is void**

```metall
fun main() void { DebugIntern.print_str("hello") mut x = 123 x = 321 }
```

```output
hello
```

**if true branch**

```metall
fun main() void { let x = if true { 123 } else { 321 } DebugIntern.print_int(x) }
```

```output
123
```

**if false branch**

```metall
fun main() void { let x = if false { 123 } else { 321 } DebugIntern.print_int(x) }
```

```output
321
```

**if assigns to mut var**

```metall
fun main() void { mut x = 1 if true { x = 123 } else { x = 321 } DebugIntern.print_int(x) }
```

```output
123
```

**nested if**

```metall
fun main() void {
    let x = if true {
        if false { 1 } else { 123 }
    } else {
        2
    }
    DebugIntern.print_int(x)
}
```

```output
123
```

**when branch chain**

```metall
fun main() void {
    let a = false
    let b = true
    let x = when {
        case a: 1
        case b: 2
        else: 3
    }
    DebugIntern.print_int(x)
}
```

```output
2
```

**ref deref**

```metall
fun main() void { mut x = 123 mut y = &mut x DebugIntern.print_int(y.*) y.* = 321 DebugIntern.print_int(x) }
```

```output
123
321
```

**nested ref deref**

```metall
fun main() void { 
    mut x = 123 
    mut y = &mut x
    mut z = &mut y
    DebugIntern.print_int(y.*)
    y.* = 321 
    DebugIntern.print_int(x)
    z.*.* = 111
    DebugIntern.print_int(x)
}
```

```output
123
321
111
```

**deref assign through &mut param**

```metall
fun foo(a &mut Int) void { 
    DebugIntern.print_int(a.*)
    a.* = 321 
}
fun main() void { 
    mut x = 123 
    foo(&mut x)
    DebugIntern.print_int(x)
}
```

```output
123
321
```

**struct field read and write**

```metall
struct Foo {
    mut one Str
    mut two Int
}

fun main() void {
    mut x = Foo("hello", 123)
    DebugIntern.print_str(x.one)
    DebugIntern.print_int(x.two)

    x.one = "bye"
    x.two = 456
    DebugIntern.print_str(x.one)
    DebugIntern.print_int(x.two)
}
```

```output
hello
123
bye
456
```

**struct as value param**

```metall
struct Foo {
    one Str
}

fun foo(a Foo) void {
    DebugIntern.print_str(a.one)
}

fun main() void {
    let x = Foo("hello")
    foo(x)
}
```

```output
hello
```

**struct &ref and &mut ref params**

```metall
struct Foo {
    mut one Str
}

fun foo(a &Foo) void {
    DebugIntern.print_str(a.one)
}

fun bar(a &mut Foo, b Str) void {
    a.one = b
}

fun main() void {
    mut x = Foo("hello")
    foo(&x)

    bar(&mut x, "bye")
    foo(&x)
}
```

```output
hello
bye
```

**fun returns struct**

```metall
struct Foo {
    one Str
}

fun foo() Foo {
    Foo("hello")
}

fun main() void {
    let x = foo()
    DebugIntern.print_str(x.one)
}
```

```output
hello
```

**nested struct field access**

```metall
struct Foo {
    mut one Str
}

struct Bar {
    one Foo
    mut two Foo
}

fun main() void {
    mut x = Bar(Foo("hello"), Foo("world"))
    DebugIntern.print_str(x.one.one)
    DebugIntern.print_str(x.two.one)
    x.two.one = "bye"
    DebugIntern.print_str(x.two.one)
}
```

```output
hello
world
bye
```

**struct value copy**

```metall
struct Foo {
    mut one Str
}

fun main() void {
    mut x = Foo("hello")
    mut y = x
    y.one = "world"
    DebugIntern.print_str(x.one)
    DebugIntern.print_str(y.one)
}
```

```output
hello
world
```

**nested struct value copy**

```metall
struct Foo {
    mut one Str
}

struct Bar {
    mut one Foo
}

fun main() void {
    mut x = Bar(Foo("hello"))
    mut y = Foo("world")
    x.one = y
    y.one = "bye"
    DebugIntern.print_str(x.one.one)
    DebugIntern.print_str(y.one)
}
```

```output
world
bye
```

**struct with &ref field**

```metall
struct Wrapper {
    one Int
    two &Int
}

fun main() void {
    mut x = 42
    let y = Wrapper(1, &x)
    DebugIntern.print_int(y.one)
    DebugIntern.print_int(y.two.*)
    x = 99
    DebugIntern.print_int(y.two.*)
}
```

```output
1
42
99
```

**struct ref alias sees mutation**

```metall
struct Foo {
    mut one Str
}

fun main() void {
    mut x = Foo("hello")
    let y = &x
    let z = y
    x.one = "world"
    DebugIntern.print_str(z.one)
}
```

```output
world
```

**struct in if else**

```metall
struct Foo {
    mut one Str
}

fun main() void {
    let x = if true { Foo("hello") } else { Foo("world") }
    DebugIntern.print_str(x.one)
    mut y = if false { Foo("hello") } else { Foo("world") }
    DebugIntern.print_str(y.one)
    y.one = "bye"
    DebugIntern.print_str(y.one)
}
```

```output
hello
world
bye
```

**struct reassign from if else**

```metall
struct Foo {
    one Str
}

fun main() void {
    mut x = Foo("hello")
    DebugIntern.print_str(x.one)
    x = if true { Foo("world") } else { Foo("bye") }
    DebugIntern.print_str(x.one)
}
```

```output
hello
world
```

**struct from block as arg**

```metall
struct Foo {
    one Str
}

fun foo(a Foo) void {
    DebugIntern.print_str(a.one)
}

fun main() void {
    foo({ Foo("hello") })
}
```

```output
hello
```

**generic struct**

```metall
struct Pair<T> {
    first T
    second T
}

fun main() void {
    let p = Pair<Int>(10, 20)
    DebugIntern.print_int(p.first)
    DebugIntern.print_int(p.second)
}
```

```output
10
20
```

**generic fun**

```metall
struct Box<T> { value T }
fun id<T>(x T) T { x }

fun main() void {
    DebugIntern.print_int(id<Int>(42))
    DebugIntern.print_str(id<Str>("hello"))
    let b = id<Box<Int>>(Box<Int>(99))
    DebugIntern.print_int(b.value)
}
```

```output
42
hello
99
```

**generic fun as value**

```metall
fun id<T>(x T) T { x }

fun main() void {
    let f = id<Int>
    DebugIntern.print_int(f(42))
    let g = id<Str>
    DebugIntern.print_str(g("hello"))
}
```

```output
42
hello
```

**method on generic struct**

```metall
struct Foo<T> { one T }
fun Foo.bar<T>(f Foo<T>, a T, b Bool) T { if b { return f.one } a }

fun main() void {
    let x = Foo<Int>(42)
    DebugIntern.print_int(x.bar(99, true))
    DebugIntern.print_int(x.bar(99, false))
}
```

```output
42
99
```

**generic method**

```metall
struct Foo { value Int }
fun Foo.get<T>(f Foo, x T) T { x }

fun main() void {
    let f = Foo(42)
    DebugIntern.print_int(f.get<Int>(1))
    DebugIntern.print_str(f.get<Str>("hello"))
}
```

```output
1
hello
```

**generic method with extra type param on generic struct**

```metall
struct Foo<T> { one T }
fun Foo.bar<T, U>(f Foo<T>, a U) U { a }

fun main() void {
    let x = Foo<Int>(42)
    DebugIntern.print_str(x.bar<Str>("hello"))
    DebugIntern.print_int(x.bar<Int>(99))
}
```

```output
hello
99
```

**method on generic struct accesses field**

```metall
struct Pair<A, B> { first A second B }
fun Pair.get_first<A, B>(p Pair<A, B>) A { p.first }
fun Pair.get_second<A, B>(p Pair<A, B>) B { p.second }

fun main() void {
    let p = Pair<Int, Str>(42, "hello")
    DebugIntern.print_int(p.get_first())
    DebugIntern.print_str(p.get_second())
}
```

```output
42
hello
```

**method on multi-param generic struct with extra type param**

```metall
struct Pair<A, B> { first A second B }
fun Pair.swap<A, B>(p Pair<A, B>) Pair<B, A> { Pair<B, A>(p.second, p.first) }
fun Pair.map_first<A, B, C>(p Pair<A, B>, f fun(A) C) Pair<C, B> { Pair<C, B>(f(p.first), p.second) }

fun to_str(x Int) Str { "mapped" }

fun main() void {
    let p = Pair<Int, Str>(42, "hello")
    let s = p.swap()
    DebugIntern.print_str(s.first)
    DebugIntern.print_int(s.second)
    let m = p.map_first<Str>(to_str)
    DebugIntern.print_str(m.first)
    DebugIntern.print_str(m.second)
}
```

```output
hello
42
mapped
hello
```

**generic method chain**

```metall
struct Wrap<T> { inner T }
fun Wrap.unwrap<T>(w Wrap<T>) T { w.inner }

fun main() void {
    let w = Wrap<Wrap<Int>>(Wrap<Int>(99))
    let inner = w.unwrap()
    DebugIntern.print_int(inner.unwrap())
}
```

```output
99
```

**generic struct method calls generic fun**

```metall
struct Box<T> { value T }
fun id<T>(x T) T { x }
fun Box.get_id<T>(b Box<T>) T { id<T>(b.value) }

fun main() void {
    let b = Box<Int>(7)
    DebugIntern.print_int(b.get_id())
}
```

```output
7
```

**generic shadowing**

```metall
struct Box<T> { value T }
fun id<T>(x T) T { x }

fun main() void {
    DebugIntern.print_int(id<Int>(1))
    DebugIntern.print_int(Box<Int>(2).value)
    {
        struct Box<T> { value T value2 T }
        fun id<T>(x T) T { x }
        DebugIntern.print_int(id<Int>(3))
        DebugIntern.print_int(Box<Int>(4, 5).value2)
    }
}
```

```output
1
2
3
5
```

**generic struct method called from generic fun**

```metall
struct Box<V> {
    mut items []V
}

fun Box.len<V>(b &Box<V>) Int {
    b.items.len
}

fun wrap<V>(@a Arena, v V) Int {
    let items = @a.slice<V>(2, v)
    let b = @a.new_mut<Box<V>>(Box<V>(items))
    b.len()
}

fun main() void {
    let @a = Arena()
    DebugIntern.print_int(wrap<Str>(@a, "x"))
}
```

```output
2
```

**forward declared fun**

```metall
fun main() void {
    DebugIntern.print_int(foo())
}

fun foo() Int {
    123
}
```

```output
123
```

**heap alloc with arena**

```metall
struct Foo {
    one Str
}

fun foo(@a Arena) &Foo {
    @a.new<Foo>(Foo("hello"))
}

fun main() void {
    let @a = Arena()
    let x = @a.new<Foo>(Foo("x"))
    let y = @a.new<Foo>(Foo("y"))
    {
        let @b = Arena()
        let z = @b.new<Foo>(Foo("z"))
        DebugIntern.print_str(z.one)
    }
    DebugIntern.print_str(y.one)
    DebugIntern.print_str(x.one)
    let w = foo(@a)
    DebugIntern.print_str(w.one)
}
```

```output
z
y
x
hello
```

**int array**

```metall
fun main() void {
    let x = [1, 2, 3]
    DebugIntern.print_int(x[2])
    DebugIntern.print_int(x[1])
    DebugIntern.print_int(x[0])
}
```

```output
3
2
1
```

**array index with variable**

```metall
fun main() void {
    mut x = [10, 20, 30]
    let i = 1
    DebugIntern.print_int(x[i])
    x[i] = 99
    DebugIntern.print_int(x[i])
}
```

```output
20
99
```

**struct array**

```metall
struct Foo {
    one Str
}

fun main() void {
    let x = [
        Foo("x"),
        Foo("y"),
        Foo("z"),
    ]
    DebugIntern.print_str(x[2].one)
    DebugIntern.print_str(x[1].one)
    DebugIntern.print_str(x[0].one)
}
```

```output
z
y
x
```

**nested array**

```metall
fun main() void {
    let x = [
        [1, 2],
        [3, 4],
        [5, 6],
    ]
    let y = x[0]
    DebugIntern.print_int(y[1])
    let z = x[1]
    DebugIntern.print_int(z[0])
    let w = x[2]
    DebugIntern.print_int(w[1])
}
```

```output
2
3
6
```

**array in struct**

```metall
struct Foo {
    one [3]Int
}

fun main() void {
    let x = Foo([1, 2, 3])
    DebugIntern.print_int(x.one[1])
}
```

```output
2
```

**array with refs**

```metall
struct Foo {
     one Str
}

fun main() void {
    let x = Foo("x")
    let y = Foo("y")
    let z = [x, y]
    DebugIntern.print_str(z[1].one)
    DebugIntern.print_str(z[0].one)

    let w = 1
    let v = 2
    let u = [&w, &v]
    DebugIntern.print_int(u[1].*)
    DebugIntern.print_int(u[0].*)
}
```

```output
y
x
2
1
```

**array index write**

```metall
fun main() void {
    mut x = [1, 2, 3]
    DebugIntern.print_int(x[1])
    x[1] = 4
    DebugIntern.print_int(x[1])
}
```

```output
2
4
```

**array struct index write**

```metall
struct Foo { one Str }

fun main() void {
    mut x = [Foo("x"), Foo("y")]
    DebugIntern.print_str(x[0].one)
    x[0] = Foo("z")
    DebugIntern.print_str(x[0].one)
}
```

```output
x
z
```

**array of refs index write**

```metall
struct Foo { one Str }

fun main() void {
    let x = Foo("x")
    let y = Foo("y")
    let z = Foo("z")
    mut w = [&x, &y]
    DebugIntern.print_str(w[0].one)
    w[0] = &z
    DebugIntern.print_str(w[0].one)
}
```

```output
x
z
```

**heap alloc slice**

```metall
fun main() void {
    let @a = Arena()
    let x = @a.slice_mut<Int>(5, 0)
    x[1] = 1
    x[2] = 2

    DebugIntern.print_int(x[0])
    DebugIntern.print_int(x[1])
    DebugIntern.print_int(x[2])
}
```

```output
0
1
2
```

**heap alloc struct is ref aliased**

```metall
struct Foo {
    mut one Str
}

fun main() void {
    let @a = Arena()
    mut x = @a.new_mut<Foo>(Foo("hello"))
    mut y = x
    y.one = "world"
    DebugIntern.print_str(x.one)
    DebugIntern.print_str(y.one)
}
```

```output
world
world
```

**heap alloc slice is aliased**

```metall
fun main() void {
    let @a = Arena()
    let x = unsafe @a.slice_uninit_mut<Int>(3)
    x[0] = 42
    let y = x
    y[0] = 99
    DebugIntern.print_int(x[0])
    DebugIntern.print_int(y[0])
}
```

```output
99
99
```

**heap alloc immutable struct read**

```metall
struct Foo {
    one Str
}

fun main() void {
    let @a = Arena()
    let x = @a.new<Foo>(Foo("hello"))
    DebugIntern.print_str(x.one)
}
```

```output
hello
```

**heap alloc mut struct as param**

```metall
struct Foo {
    mut one Str
}

fun set(a &mut Foo, b Str) void {
    a.one = b
}

fun main() void {
    let @a = Arena()
    let x = @a.new_mut<Foo>(Foo("hello"))
    set(x, "world")
    DebugIntern.print_str(x.one)
}
```

```output
world
```

**heap alloc slice read**

```metall
fun main() void {
    let @a = Arena()
    let x = unsafe @a.slice_uninit_mut<Int>(3)
    x[0] = 42
    let y = unsafe @a.slice_uninit<Int>(3)
    DebugIntern.print_int(x[0])
}
```

```output
42
```

**slice copy aliases underlying data**

```metall
fun main() void {
    let @a = Arena()
    let x = unsafe @a.slice_uninit_mut<Int>(3)
    x[0] = 42
    let y = x
    y[0] = 99
    DebugIntern.print_int(x[0])
    DebugIntern.print_int(y[0])
}
```

```output
99
99
```

**make slice**

```metall
fun main() void {
    let @a = Arena()
    let size = 3
    let x = unsafe @a.slice_uninit_mut<Int>(size)
    x[0] = 10
    x[1] = 20
    x[2] = 30

    DebugIntern.print_int(x[0])
    DebugIntern.print_int(x[1])
    DebugIntern.print_int(x[2])
    DebugIntern.print_int(x.len)
}
```

```output
10
20
30
3
```

**slice index with variable**

```metall
fun main() void {
    let @a = Arena()
    let x = unsafe @a.slice_uninit_mut<Int>(3)
    x[0] = 10
    x[1] = 20
    x[2] = 30
    let i = 2
    DebugIntern.print_int(x[i])
    x[i] = 99
    DebugIntern.print_int(x[i])
}
```

```output
30
99
```

**make slice with default value**

```metall
fun main() void {
    let @a = Arena()
    let x = @a.slice<Int>(100, 77)
    DebugIntern.print_int(x[0])
    DebugIntern.print_int(x[50])
    DebugIntern.print_int(x[99])
}
```

```output
77
77
77
```

**make slice with default value 2**

```metall
fun main() void {
    let @a = Arena()
    let x = @a.slice<Int>(100, 77)
    DebugIntern.print_int(x[0])
    DebugIntern.print_int(x[50])
    DebugIntern.print_int(x[99])
}
```

```output
77
77
77
```

**make uninit then write**

```metall
fun main() void {
    let @a = Arena()
    let x = unsafe @a.slice_uninit_mut<Int>(100)
    x[99] = 42
    DebugIntern.print_int(x[99])
}
```

```output
42
```

**make uninit slice then write**

```metall
fun main() void {
    let @a = Arena()
    let x = unsafe @a.slice_uninit_mut<Int>(100)
    x[99] = 42
    DebugIntern.print_int(x[99])
}
```

```output
42
```

**make struct slice with default value 1**

```metall
struct Foo {
    one Int
    two Str
}

fun main() void {
    let @a = Arena()
    let x = @a.slice<Foo>(100, Foo(42, "hello"))
    DebugIntern.print_int(x[0].one)
    DebugIntern.print_str(x[0].two)
    DebugIntern.print_int(x[50].one)
    DebugIntern.print_str(x[50].two)
    DebugIntern.print_int(x[99].one)
    DebugIntern.print_str(x[99].two)
}
```

```output
42
hello
42
hello
42
hello
```

**make struct slice with default value 2**

```metall
struct Foo {
    one Int
    two Str
}

fun main() void {
    let @a = Arena()
    let x = @a.slice<Foo>(100, Foo(42, "hello"))
    DebugIntern.print_int(x[0].one)
    DebugIntern.print_str(x[0].two)
    DebugIntern.print_int(x[50].one)
    DebugIntern.print_str(x[50].two)
    DebugIntern.print_int(x[99].one)
    DebugIntern.print_str(x[99].two)
}
```

```output
42
hello
42
hello
42
hello
```

**make ref slice with default value 1**

```metall
struct Foo {
    mut one Int
    two Str
}

fun main() void {
    let @a = Arena()
    let def = @a.new_mut<Foo>(Foo(42, "hello"))
    let x = @a.slice_mut<&mut Foo>(3, def)
    DebugIntern.print_int(x[0].one)
    DebugIntern.print_int(x[2].one)
    x[0].one = 99
    DebugIntern.print_int(x[1].one)
    DebugIntern.print_int(x[2].one)
}
```

```output
42
42
99
99
```

**make ref slice with default value 2**

```metall
struct Foo {
    mut one Int
    two Str
}

fun main() void {
    let @a = Arena()
    let def = @a.new_mut<Foo>(Foo(42, "hello"))
    let x = @a.slice_mut<&mut Foo>(3, def)
    DebugIntern.print_int(x[0].one)
    DebugIntern.print_int(x[2].one)
    x[0].one = 99
    DebugIntern.print_int(x[1].one)
    DebugIntern.print_int(x[2].one)
}
```

```output
42
42
99
99
```

**allocate multidimensional slice**

```metall
fun main() void {
    let @a = Arena()
    let m = @a.slice_mut<[]Int>(2, [])
    m[0] = @a.slice<Int>(3, 10)
    m[1] = @a.slice<Int>(3, 40)
    DebugIntern.print_int(m[0][1])
    DebugIntern.print_int(m[1][2])
}
```

```output
10
40
```

**make multidimensional slice**

```metall
fun main() void {
    let @a = Arena()
    let m = @a.slice_mut<[]Int>(2, [])
    m[0] = @a.slice<Int>(3, 20)
    m[1] = @a.slice<Int>(3, 60)
    DebugIntern.print_int(m[0][1])
    DebugIntern.print_int(m[1][2])
}
```

```output
20
60
```

**empty slice resets slice**

```metall
fun main() void {
    let @a = Arena()
    mut x = @a.slice<Int>(3, 42)
    DebugIntern.print_int(x[1])
    x = []
    DebugIntern.print_int(x.len)
}
```

```output
42
0
```

**update array element in place**

```metall
fun main() void {
    struct Foo { mut one Int }
    mut a = [Foo(1)]
    a[0].one = 42
    DebugIntern.print_int(a[0].one)
}
```

```output
42
```

**update slice element in place**

```metall
fun main() void {
    let @a = Arena()
    struct Foo { mut one Int }
    let a = unsafe @a.slice_uninit_mut<Foo>(1)
    a[0] = Foo(1)
    a[0].one = 42
    DebugIntern.print_int(a[0].one)
}
```

```output
42
```

**ref to array element**

```metall
fun main() void {
    struct Foo { mut one Int }
    mut a = [Foo(1)]
    mut b = &mut a[0]
    b.one = 42
    DebugIntern.print_int(a[0].one)
}
```

```output
42
```

**ref to slice element**

```metall
fun main() void {
    let @a = Arena()
    struct Foo { mut one Int }
    let a = unsafe @a.slice_uninit_mut<Foo>(1)
    a[0] = Foo(1)
    let b = &mut a[0]
    b.one = 42
    DebugIntern.print_int(a[0].one)
}
```

```output
42
```

**arena alloc struct with alignment padding**

```metall
struct Padded { flag Bool value Int }

fun main() void {
    let @a = Arena()
    let p = @a.new<Padded>(Padded(true, 42))
    DebugIntern.print_bool(p.flag)
    DebugIntern.print_int(p.value)
    let s = @a.slice<Padded>(3, Padded(false, 99))
    DebugIntern.print_bool(s[0].flag)
    DebugIntern.print_int(s[0].value)
    DebugIntern.print_bool(s[2].flag)
    DebugIntern.print_int(s[2].value)
}
```

```output
true
42
false
99
false
99
```


**subslice exclusive range**

```metall
fun main() void {
    let arr = [10, 20, 30, 40, 50]
    let s = arr[1..3]
    DebugIntern.print_int(s.len)
    DebugIntern.print_int(s[0])
    DebugIntern.print_int(s[1])
}
```

```output
2
20
30
```

**subslice inclusive range**

```metall
fun main() void {
    let arr = [10, 20, 30, 40, 50]
    let s = arr[1..=3]
    DebugIntern.print_int(s.len)
    DebugIntern.print_int(s[0])
    DebugIntern.print_int(s[2])
}
```

```output
3
20
40
```

**subslice open lo**

```metall
fun main() void {
    let arr = [10, 20, 30, 40, 50]
    let s = arr[..2]
    DebugIntern.print_int(s.len)
    DebugIntern.print_int(s[0])
    DebugIntern.print_int(s[1])
}
```

```output
2
10
20
```

**subslice open hi**

```metall
fun main() void {
    let arr = [10, 20, 30, 40, 50]
    let s = arr[3..]
    DebugIntern.print_int(s.len)
    DebugIntern.print_int(s[0])
    DebugIntern.print_int(s[1])
}
```

```output
2
40
50
```

**subslice of slice**

```metall
fun main() void {
    let @a = Arena()
    let sl = unsafe @a.slice_uninit_mut<Int>(5)
    sl[0] = 100
    sl[1] = 200
    sl[2] = 300
    sl[3] = 400
    sl[4] = 500
    let s = sl[2..4]
    DebugIntern.print_int(s.len)
    DebugIntern.print_int(s[0])
    DebugIntern.print_int(s[1])
}
```

```output
2
300
400
```

**mutate array through subslice**

```metall
fun main() void {
    mut arr = [10, 20, 30, 40, 50]
    mut s = arr[1..4]
    s[0] = 99
    s[2] = 88
    DebugIntern.print_int(arr[1])
    DebugIntern.print_int(arr[3])
}
```

```output
99
88
```

**mutate slice through subslice**

```metall
fun main() void {
    let @a = Arena()
    let sl = unsafe @a.slice_uninit_mut<Int>(4)
    sl[0] = 1
    sl[1] = 2
    sl[2] = 3
    sl[3] = 4
    let sub = sl[1..3]
    sub[0] = 77
    sub[1] = 88
    DebugIntern.print_int(sl[1])
    DebugIntern.print_int(sl[2])
}
```

```output
77
88
```

**array len**

```metall
fun main() void {
    let arr = [10, 20, 30, 40, 50]
    DebugIntern.print_int(arr.len)
}
```

```output
5
```

**int arithmetic**

```metall
fun main() void {
    DebugIntern.print_int(120 + 3)
    DebugIntern.print_int(44 - 2)
    DebugIntern.print_int(3 * 3)
    DebugIntern.print_int(9 / 3)
    DebugIntern.print_int(10 % 3)
    DebugIntern.print_int((U8(10) % 3).to_int())
}
```

```output
123
42
9
3
1
1
```

**wrapping arithmetic**

```metall
fun main() void {
    -- Int (i64, signed)
    let max = 9223372036854775807
    DebugIntern.print_int(max +% 1)
    DebugIntern.print_int(0 -% 1)
    DebugIntern.print_int(max *% 2)
    -- I8 (signed)
    DebugIntern.print_int((I8(127) +% I8(1)).to_int())
    DebugIntern.print_int((I8(-128) -% I8(1)).to_int())
    DebugIntern.print_int((I8(127) *% I8(2)).to_int())
    -- I16 (signed)
    DebugIntern.print_int((I16(32767) +% I16(1)).to_int())
    DebugIntern.print_int((I16(-32768) -% I16(1)).to_int())
    DebugIntern.print_int((I16(32767) *% I16(2)).to_int())
    -- I32 (signed)
    DebugIntern.print_int((I32(2147483647) +% I32(1)).to_int())
    DebugIntern.print_int((I32(-2147483648) -% I32(1)).to_int())
    DebugIntern.print_int((I32(2147483647) *% I32(2)).to_int())
    -- U8 (unsigned)
    DebugIntern.print_int((U8(255) +% U8(1)).to_int())
    DebugIntern.print_int((U8(0) -% U8(1)).to_int())
    DebugIntern.print_int((U8(255) *% U8(2)).to_int())
    -- U16 (unsigned)
    DebugIntern.print_int((U16(65535) +% U16(1)).to_int())
    DebugIntern.print_int((U16(0) -% U16(1)).to_int())
    DebugIntern.print_int((U16(65535) *% U16(2)).to_int())
    -- U32 (unsigned)
    DebugIntern.print_int((U32(4294967295) +% U32(1)).to_int())
    DebugIntern.print_int((U32(0) -% U32(1)).to_int())
    DebugIntern.print_int((U32(4294967295) *% U32(2)).to_int())
    -- U64 (unsigned)
    DebugIntern.print_uint(U64(18446744073709551615) +% U64(1))
    DebugIntern.print_uint(U64(0) -% U64(1))
    DebugIntern.print_uint(U64(18446744073709551615) *% U64(2))
}
```

```output
-9223372036854775808
-1
-2
-128
127
-2
-32768
32767
-2
-2147483648
2147483647
-2
0
255
254
0
65535
65534
0
4294967295
4294967294
0
18446744073709551615
18446744073709551614
```

**add overflow panics**

```metall
fun main() void {
    let max = 9223372036854775807
    _ = max + 1
}
```

```panic
test.met:3:9: integer overflow
```

**sub overflow panics**

```metall
fun main() void {
    let min = -9223372036854775808
    _ = min - 1
}
```

```panic
test.met:3:9: integer overflow
```

**mul overflow panics**

```metall
fun main() void {
    let max = 9223372036854775807
    _ = max * 2
}
```

```panic
test.met:3:9: integer overflow
```

**I8 overflow panics**

```metall
fun main() void {
    _ = I8(127) + I8(1)
}
```

```panic
test.met:2:9: integer overflow
```

**I16 overflow panics**

```metall
fun main() void {
    _ = I16(32767) + I16(1)
}
```

```panic
test.met:2:9: integer overflow
```

**I32 overflow panics**

```metall
fun main() void {
    _ = I32(2147483647) + I32(1)
}
```

```panic
test.met:2:9: integer overflow
```

**U8 overflow panics**

```metall
fun main() void {
    _ = U8(255) + U8(1)
}
```

```panic
test.met:2:9: integer overflow
```

**U8 underflow panics**

```metall
fun main() void {
    _ = U8(0) - U8(1)
}
```

```panic
test.met:2:9: integer overflow
```

**U16 overflow panics**

```metall
fun main() void {
    _ = U16(65535) + U16(1)
}
```

```panic
test.met:2:9: integer overflow
```

**U32 overflow panics**

```metall
fun main() void {
    _ = U32(4294967295) + U32(1)
}
```

```panic
test.met:2:9: integer overflow
```

**U64 overflow panics**

```metall
fun main() void {
    _ = U64(18446744073709551615) + U64(1)
}
```

```panic
test.met:2:9: integer overflow
```

**bool operators**

```metall
fun main() void {
    DebugIntern.print_bool(1 == 2)
    DebugIntern.print_bool(1 != 2)
    DebugIntern.print_bool(true == false)
    DebugIntern.print_bool(true != false)

    DebugIntern.print_bool(1 == 2 and 3 == 3)
    DebugIntern.print_bool(1 == 2 or 3 == 3)

    DebugIntern.print_bool(not true)
}
```

```output
false
true
false
true
false
true
false
```

**int comparison operators**

```metall
fun main() void {
    DebugIntern.print_bool(1 < 2)
    DebugIntern.print_bool(2 < 1)
    DebugIntern.print_bool(1 <= 1)
    DebugIntern.print_bool(1 <= 0)
    DebugIntern.print_bool(2 > 1)
    DebugIntern.print_bool(1 > 2)
    DebugIntern.print_bool(1 >= 1)
    DebugIntern.print_bool(0 >= 1)
    DebugIntern.print_bool(U8(1) < 2)
}
```

```output
true
false
true
false
true
false
true
false
true
```

**bitwise I8**

```metall
fun main() void {
    let a = I8(90)
    let b = I8(60)
    DebugIntern.print_int((a & b).to_int())
    DebugIntern.print_int((a | b).to_int())
    DebugIntern.print_int((a ^ b).to_int())
    DebugIntern.print_int((a << I8(1)).to_int())
    DebugIntern.print_int((a >> I8(2)).to_int())
    DebugIntern.print_int((~a).to_int())
    let c = I8(0) - I8(100)
    DebugIntern.print_int((c >> I8(2)).to_int())
    DebugIntern.print_int((~c).to_int())
}
```

```output
24
126
102
-76
22
-91
-25
99
```

**bitwise I16**

```metall
fun main() void {
    let a = I16(90)
    let b = I16(60)
    DebugIntern.print_int((a & b).to_int())
    DebugIntern.print_int((a | b).to_int())
    DebugIntern.print_int((a ^ b).to_int())
    DebugIntern.print_int((a << I16(4)).to_int())
    DebugIntern.print_int((a >> I16(2)).to_int())
    DebugIntern.print_int((~a).to_int())
    let c = I16(0) - I16(1000)
    DebugIntern.print_int((c >> I16(3)).to_int())
    DebugIntern.print_int((~c).to_int())
}
```

```output
24
126
102
1440
22
-91
-125
999
```

**bitwise I32**

```metall
fun main() void {
    let a = I32(65280)
    let b = I32(4080)
    DebugIntern.print_int((a & b).to_int())
    DebugIntern.print_int((a | b).to_int())
    DebugIntern.print_int((a ^ b).to_int())
    DebugIntern.print_int((a << I32(4)).to_int())
    DebugIntern.print_int((a >> I32(8)).to_int())
    DebugIntern.print_int((~a).to_int())
    let c = I32(0) - I32(1)
    DebugIntern.print_int((c >> I32(16)).to_int())
    DebugIntern.print_int((~c).to_int())
}
```

```output
3840
65520
61680
1044480
255
-65281
-1
0
```

**bitwise Int**

```metall
fun main() void {
    let a = 3735928559
    let b = 4294901760
    DebugIntern.print_int(a & b)
    DebugIntern.print_int(a | b)
    DebugIntern.print_int(a ^ b)
    DebugIntern.print_int(1 << 32)
    DebugIntern.print_int(a >> 16)
    DebugIntern.print_int(~a)
    let c = -1
    DebugIntern.print_int(c >> 32)
    DebugIntern.print_int(~c)
}
```

```output
3735879680
4294950639
559070959
4294967296
57005
-3735928560
-1
0
```

**bitwise U8**

```metall
fun main() void {
    let a = U8(172)
    let b = U8(58)
    DebugIntern.print_uint((a & b).to_u64())
    DebugIntern.print_uint((a | b).to_u64())
    DebugIntern.print_uint((a ^ b).to_u64())
    DebugIntern.print_uint((a << U8(1)).to_u64())
    DebugIntern.print_uint((a >> U8(3)).to_u64())
    DebugIntern.print_uint((~a).to_u64())
}
```

```output
40
190
150
88
21
83
```

**bitwise U16**

```metall
fun main() void {
    let a = U16(43981)
    let b = U16(255)
    DebugIntern.print_uint((a & b).to_u64())
    DebugIntern.print_uint((a | b).to_u64())
    DebugIntern.print_uint((a ^ b).to_u64())
    DebugIntern.print_uint((a << U16(4)).to_u64())
    DebugIntern.print_uint((a >> U16(8)).to_u64())
    DebugIntern.print_uint((~a).to_u64())
}
```

```output
205
44031
43826
48336
171
21554
```

**bitwise U32**

```metall
fun main() void {
    let a = U32(3735928559)
    let b = U32(4294901760)
    DebugIntern.print_uint((a & b).to_u64())
    DebugIntern.print_uint((a | b).to_u64())
    DebugIntern.print_uint((a ^ b).to_u64())
    DebugIntern.print_uint((U32(1) << U32(16)).to_u64())
    DebugIntern.print_uint((a >> U32(16)).to_u64())
    DebugIntern.print_uint((~a).to_u64())
}
```

```output
3735879680
4294950639
559070959
65536
57005
559038736
```

**bitwise U64**

```metall
fun main() void {
    let a = U64(16045690984503098046)
    let b = U64(18446744069414584320)
    DebugIntern.print_uint(a & b)
    DebugIntern.print_uint(a | b)
    DebugIntern.print_uint(a ^ b)
    DebugIntern.print_uint(U64(1) << U64(48))
    DebugIntern.print_uint(a >> U64(32))
    DebugIntern.print_uint(~a)
}
```

```output
16045690981097406464
18446744072820275902
2401053091722869438
281474976710656
3735928559
2401053089206453569
```

**conditional for loop**

```metall
fun main() void {
    mut x = 0
    for x != 3 {
        DebugIntern.print_int(x)
        x = x + 1
    }
}
```

```output
0
1
2
```

**unconditional for loop**

```metall
fun main() void {
    mut x = 0
    for {
        x = x + 1
        if x == 4 {
            break
        }
        if x == 2 {
            continue
        }
        DebugIntern.print_int(x)
    }
}
```

```output
1
3
```

**for-in range**

```metall
fun main() void {
    for i in 0..5 {
        DebugIntern.print_int(i)
    }
}
```

```output
0
1
2
3
4
```

**for-in range inclusive**

```metall
fun main() void {
    for i in 0..=3 {
        DebugIntern.print_int(i)
    }
}
```

```output
0
1
2
3
```

**for-in range with expressions**

```metall
fun main() void {
    let start = 2
    let end = 5
    for i in start..end {
        DebugIntern.print_int(i)
    }
}
```

```output
2
3
4
```

**for-in range with break**

```metall
fun main() void {
    for i in 0..10 {
        if i == 3 {
            break
        }
        DebugIntern.print_int(i)
    }
}
```

```output
0
1
2
```

**for-in range with continue**

```metall
fun main() void {
    for i in 0..5 {
        if i == 2 {
            continue
        }
        DebugIntern.print_int(i)
    }
}
```

```output
0
1
3
4
```

**for-in range zero iterations**

```metall
fun main() void {
    for i in 5..5 {
        DebugIntern.print_int(i)
    }
    DebugIntern.print_int(99)
}
```

```output
99
```

**integer types**

```metall
fun main() void {
    DebugIntern.print_int(I8(127).to_int())
    DebugIntern.print_int((I8(0) - I8(1)).to_int())
    DebugIntern.print_int(I16(32767).to_int())
    DebugIntern.print_int(I32(2147483647).to_int())
    DebugIntern.print_int(U8(255).to_int())
    DebugIntern.print_int(U16(65535).to_int())
    DebugIntern.print_int(U32(4294967295).to_int())
    DebugIntern.print_uint(U64(18446744073709551615))
}
```

```output
127
-1
32767
2147483647
255
65535
4294967295
18446744073709551615
```

**widening conversions**

```metall
fun main() void {
    let x = I8(42)
    DebugIntern.print_int(x.to_i16().to_int())
    DebugIntern.print_int(x.to_i32().to_int())
    DebugIntern.print_int(x.to_int())
    DebugIntern.print_uint(U8(200).to_u16().to_u32().to_u64())
    DebugIntern.print_int(U8(200).to_i32().to_int())
    DebugIntern.print_int((I8(0) - I8(1)).to_i32().to_int())
}
```

```output
42
42
42
200
200
-1
```

**wrapping conversions**

```metall
fun main() void {
    DebugIntern.print_int(I32(200).to_i8_wrapping().to_int())
    DebugIntern.print_int(2147483648.to_i32_wrapping().to_int())
    DebugIntern.print_int(U16(300).to_u8_wrapping().to_int())
    DebugIntern.print_int(U64(4294967296).to_u32_wrapping().to_int())
    DebugIntern.print_int((I8(0) - I8(1)).to_u8_wrapping().to_int())
    DebugIntern.print_uint((-1).to_u64_wrapping())
}
```

```output
-56
-2147483648
44
0
255
18446744073709551615
```

**typed int arithmetic**

```metall
fun main() void {
    DebugIntern.print_int((I32(10) + I32(20)).to_int())
    DebugIntern.print_int((I32(50) - I32(8)).to_int())
    DebugIntern.print_int((I32(6) * I32(7)).to_int())
    DebugIntern.print_int((I32(100) / I32(3)).to_int())
    DebugIntern.print_int((U8(255) / U8(2)).to_int())
}
```

```output
30
42
42
33
127
```

**rune literal and to_u32**

```metall
fun main() void {
    DebugIntern.print_uint('a'.to_u32().to_u64())
    DebugIntern.print_uint('z'.to_u32().to_u64())
    DebugIntern.print_uint('é'.to_u32().to_u64())
}
```

```output
97
122
233
```

**rune comparison**

```metall
fun main() void {
    DebugIntern.print_bool('a' == 'a')
    DebugIntern.print_bool('a' != 'b')
    DebugIntern.print_bool('a' == 'b')
}
```

```output
true
true
false
```

**rune arithmetic**

```metall
fun main() void {
    let next = 'a' + 1
    DebugIntern.print_uint(next.to_u32().to_u64())
    let diff = 'z' - 'a'
    DebugIntern.print_uint(diff.to_u32().to_u64())
}
```

```output
98
25
```

**rune let binding**

```metall
fun main() void {
    let r = 'x'
    DebugIntern.print_uint(r.to_u32().to_u64())
}
```

```output
120
```

**method call on struct**

```metall
struct Foo { x Int }
fun Foo.get_x(self Foo) Int { self.x }
fun main() void {
    let f = Foo(42)
    DebugIntern.print_int(f.get_x())
}
```

```output
42
```

**method call with args**

```metall
struct Foo { x Int }
fun Foo.add(self Foo, y Int) Int { self.x + y }
fun main() void {
    let f = Foo(10)
    DebugIntern.print_int(f.add(32))
}
```

```output
42
```

**method call on &ref**

```metall
struct Foo { x Int }
fun Foo.get_x(self &Foo) Int { self.x }
fun main() void {
    let f = Foo(42)
    let r = &f
    DebugIntern.print_int(r.get_x())
}
```

```output
42
```

**direct qualified call**

```metall
struct Foo { x Int }
fun Foo.add(self Foo, y Int) Int { self.x + y }
fun main() void {
    let f = Foo(10)
    DebugIntern.print_int(Foo.add(f, 32))
}
```

```output
42
```

**method call on Int**

```metall
fun Int.double(self Int) Int { self + self }
fun main() void {
    let x = 21
    DebugIntern.print_int(x.double())
}
```

```output
42
```

**Str.byte_len method**

```metall
fun Str.byte_len(self Str) Int { self.data.len }
fun main() void {
    let s = "hello"
    DebugIntern.print_int(s.byte_len())
    DebugIntern.print_int("".byte_len())
    DebugIntern.print_int("abc".byte_len())
}
```

```output
5
0
3
```

**shape field access**

```metall
shape HasPair { one Str two Int }
struct Pair { one Str two Int }
fun first<T HasPair>(t T) Str { t.one }
fun main() void {
    let p = Pair("hello", 42)
    DebugIntern.print_str(first<Pair>(p))
}
```

```output
hello
```

**shape method call**

```metall
shape Showable {
    fun Showable.show(self Showable) Str
}
struct Guitar {
    name Str
}
fun Guitar.show(g Guitar) Str { g.name }
fun display<T Showable>(t T) Str { t.show() }
fun main() void {
    DebugIntern.print_str(display<Guitar>(Guitar("Telecaster")))
}
```

```output
Telecaster
```

**import local module**

```metall
use local::e2e

fun main() void {
    e2e::say_hello()

    mut f = e2e::Foo(123)
    f.print()

    f.one = 321
    e2e::Foo.print(f)
}
```

```output
hello
123
321
```

**union match int variants**

```metall
union IntOrBool = Int | Bool

fun main() void {
    let x = IntOrBool(42)
    let y = IntOrBool(true)
    match x {
        case Int v: DebugIntern.print_int(v)
        case Bool v: DebugIntern.print_bool(v)
    }
    match y {
        case Int v: DebugIntern.print_int(v)
        case Bool v: DebugIntern.print_bool(v)
    }
}
```

```output
42
true
```

**union match with struct variant**

```metall
struct Foo { one Str }
union FooOrInt = Foo | Int

fun main() void {
    let x = FooOrInt(Foo("hello"))
    let y = FooOrInt(99)
    match x {
        case Foo f: DebugIntern.print_str(f.one)
        case Int v: DebugIntern.print_int(v)
    }
    match y {
        case Foo f: DebugIntern.print_str(f.one)
        case Int v: DebugIntern.print_int(v)
    }
}
```

```output
hello
99
```

**union match with else**

```metall
union ABC = Int | Bool | Str

fun main() void {
    let x = ABC(true)
    match x {
        case Int v: DebugIntern.print_int(v)
        else: DebugIntern.print_str("other")
    }
}
```

```output
other
```

**union match as expression**

```metall
union IntOrStr = Int | Str

fun main() void {
    let x = IntOrStr(42)
    let result = match x {
        case Int v: v + 1
        case Str: 0
    }
    DebugIntern.print_int(result)
}
```

```output
43
```

**union match without binding**

```metall
union AB = Int | Bool

fun main() void {
    let x = AB(42)
    match x {
        case Int: DebugIntern.print_str("int")
        case Bool: DebugIntern.print_str("bool")
    }
}
```

```output
int
```

**generic union match**

```metall
union Maybe<T> = T | Bool

fun main() void {
    let x = Maybe<Int>(42)
    let y = Maybe<Int>(false)
    match x {
        case Int v: DebugIntern.print_int(v)
        case Bool: DebugIntern.print_str("none")
    }
    match y {
        case Int v: DebugIntern.print_int(v)
        case Bool: DebugIntern.print_str("none")
    }
}
```

```output
42
none
```

**union match all arms return**

```metall
union IntOrBool = Int | Bool

fun describe(x IntOrBool) Str {
    match x {
        case Int: return "int"
        case Bool: return "bool"
    }
}

fun main() void {
    DebugIntern.print_str(describe(IntOrBool(1)))
    DebugIntern.print_str(describe(IntOrBool(true)))
}
```

```output
int
bool
```

**union match struct result**

```metall
struct Pair { a Int b Int }
union IntOrPair = Int | Pair

fun main() void {
    let x = IntOrPair(Pair(10, 20))
    let p = match x {
        case Int v: Pair(v, v)
        case Pair p: p
    }
    DebugIntern.print_int(p.a)
    DebugIntern.print_int(p.b)
}
```

```output
10
20
```

**union match with guard**

```metall
union IntOrStr = Int | Str

fun classify(x IntOrStr) Str {
    match x {
        case Int n if n > 100: return "big"
        case Int n if n > 0: return "positive"
        case Int: return "non-positive"
        case Str: return "string"
    }
}

fun main() void {
    DebugIntern.print_str(classify(IntOrStr(200)))
    DebugIntern.print_str(classify(IntOrStr(42)))
    DebugIntern.print_str(classify(IntOrStr(0)))
    DebugIntern.print_str(classify(IntOrStr("hello")))
}
```

```output
big
positive
non-positive
string
```

**union match guard with else**

```metall
union Tri = Int | Bool | Str

fun main() void {
    let x = Tri(42)
    let result = match x {
        case Int n if n > 10: "big int"
        case Int: "small int"
        else: "other"
    }
    DebugIntern.print_str(result)
}
```

```output
big int
```

**union match guard falls through to else**

```metall
union Tri = Int | Bool | Str

fun main() void {
    let x = Tri(5)
    let result = match x {
        case Int n if n > 10: "big int"
        else: "other"
    }
    DebugIntern.print_str(result)
}
```

```output
other
```

**union match else with binding**

```metall
struct Pair { a Int b Int }
union Three = Int | Bool | Pair

fun describe(x Three) Str {
    match x {
        case Int: return "int"
        else v:
            match v {
                case Int: "unreachable"
                case Bool: "bool"
                case Pair: "pair"
            }
    }
}

fun main() void {
    DebugIntern.print_str(describe(Three(42)))
    DebugIntern.print_str(describe(Three(true)))
    DebugIntern.print_str(describe(Three(Pair(1, 2))))
}
```

```output
int
bool
pair
```

**union match else binding with single remaining variant**

```metall
union Foo = Str | Int
fun main() void {
    let x = Foo("hello")
    match x {
        case Str s: DebugIntern.print_str(s)
        else i: DebugIntern.print_int(i)
    }
    let y = Foo(42)
    match y {
        case Str s: DebugIntern.print_str(s)
        else i: DebugIntern.print_int(i)
    }
}
```

```output
hello
42
```

**union match else single variant struct**

```metall
fun main() void {
    struct Pair { a Int b Int }
    union Foo = Str | Int | Pair
    let x = Foo(Pair(10, 20))
    match x {
        case Str: DebugIntern.print_str("str")
        case Int: DebugIntern.print_str("int")
        else p: DebugIntern.print_int(p.a + p.b)
    }
}
```

```output
30
```

## Union Auto-Wrap

**union auto-wrap in let binding**

```metall
fun main() void {
    union Foo = Str | Int
    let x Foo = 42
    match x {
        case Int n: DebugIntern.print_int(n)
        case Str: DebugIntern.print_str("str")
    }
}
```

```output
42
```

**union auto-wrap in function call**

```metall
fun main() void {
    union Foo = Str | Int
    fun check(f Foo) void {
        match f {
            case Int n: DebugIntern.print_int(n)
            case Str s: DebugIntern.print_str(s)
        }
    }
    check(42)
    check("hello")
}
```

```output
42
hello
```

**union auto-wrap in return**

```metall
fun main() void {
    union Foo = Str | Int
    fun make_foo() Foo {
        return 42
    }
    match make_foo() {
        case Int n: DebugIntern.print_int(n)
        case Str: DebugIntern.print_str("str")
    }
}
```

```output
42
```

**union auto-wrap implicit return**

```metall
fun main() void {
    union Foo = Str | Int
    fun make_foo() Foo {
        42
    }
    match make_foo() {
        case Int n: DebugIntern.print_int(n)
        case Str: DebugIntern.print_str("str")
    }
}
```

```output
42
```

**union auto-wrap in if-else branches**

```metall
fun main() void {
    union Foo = Str | Int
    fun pick(b Bool) Foo {
        if b { 42 } else { "hello" }
    }
    match pick(true) {
        case Int n: DebugIntern.print_int(n)
        case Str s: DebugIntern.print_str(s)
    }
    match pick(false) {
        case Int n: DebugIntern.print_int(n)
        case Str s: DebugIntern.print_str(s)
    }
}
```

```output
42
hello
```

**union auto-wrap in assignment**

```metall
fun main() void {
    union Foo = Str | Int
    mut x Foo = 42
    match x {
        case Int n: DebugIntern.print_int(n)
        case Str: DebugIntern.print_str("str")
    }
    x = "reassigned"
    match x {
        case Int: DebugIntern.print_str("int")
        case Str s: DebugIntern.print_str(s)
    }
}
```

```output
42
reassigned
```

**union auto-wrap with generic union**

```metall
fun main() void {
    union Maybe<T> = T | Bool
    let x Maybe<Int> = 99
    match x {
        case Int n: DebugIntern.print_int(n)
        case Bool: DebugIntern.print_str("bool")
    }
}
```

```output
99
```

**union auto-wrap no double wrap**

```metall
fun main() void {
    union Foo = Str | Int
    let x Foo = Foo(42)
    match x {
        case Int n: DebugIntern.print_int(n)
        case Str: DebugIntern.print_str("str")
    }
}
```

```output
42
```

**union auto-wrap through nested blocks**

```metall
fun main() void {
    union Foo = Str | Int
    let x Foo = { { 42 } }
    match x {
        case Int n: DebugIntern.print_int(n)
        case Str: DebugIntern.print_str("str")
    }
}
```

```output
42
```

**union auto-wrap struct variant**

```metall
fun main() void {
    struct Bar { value Int }
    union Foo = Bar | Int
    let x Foo = Bar(99)
    match x {
        case Bar b: DebugIntern.print_int(b.value)
        case Int: DebugIntern.print_str("int")
    }
    let y Foo = 42
    match y {
        case Bar: DebugIntern.print_str("bar")
        case Int n: DebugIntern.print_int(n)
    }
}
```

```output
99
42
```

**union auto-wrap in match arm result**

```metall
fun main() void {
    union Inner = Str | Int
    union Outer = Inner | Bool
    let x Outer = true
    match x {
        case Inner: DebugIntern.print_str("inner")
        case Bool b: {
            let result Inner = 42
            match result {
                case Int n: DebugIntern.print_int(n)
                case Str: DebugIntern.print_str("str")
            }
        }
    }
}
```

```output
42
```

**void as union type argument**

```metall
fun main() void {
    fun might_fail(ok Bool) Result<void> {
        if ok { void } else { Err("something went wrong") }
    }

    match might_fail(true) {
        case void: DebugIntern.print_str("ok")
        case Err e: DebugIntern.print_str(e.msg)
    }
    match might_fail(false) {
        case void: DebugIntern.print_str("ok")
        case Err e: DebugIntern.print_str(e.msg)
    }
}
```

```output
ok
something went wrong
```

**union auto-wrap deeply nested showcase**

Tests auto-wrap across if/else, match, nested blocks, explicit return,
implicit return, let bindings, function calls, and generic unions.

```metall
fun main() void {
    union Outcome<T> = T | Str

    fun transform(x Int) Outcome<Int> {
        if x > 100 {
            return "overflow"
        }
        {
            {
                x + 1
            }
        }
    }

    fun describe(r Outcome<Int>) Str {
        match r {
            case Int n: {
                if n > 50 { return "big" }
                "small"
            }
            case Str s: s
        }
    }

    fun pass_through(x Int) Str {
        describe(x)
    }

    DebugIntern.print_str(describe(transform(10)))
    DebugIntern.print_str(describe(transform(200)))
    DebugIntern.print_str(describe(42))
    DebugIntern.print_str(pass_through(99))

    mut acc Outcome<Int> = 0
    acc = "replaced"
    match acc {
        case Int: DebugIntern.print_str("int")
        case Str s: DebugIntern.print_str(s)
    }

    let choice Outcome<Int> = if true { 7 } else { "nope" }
    match choice {
        case Int n: DebugIntern.print_int(n)
        case Str s: DebugIntern.print_str(s)
    }
}
```

```output
small
overflow
small
big
replaced
7
```

**union auto-wrap with early return in match arm**

```metall
struct MyErr { msg Str }
union MyResult = void | MyErr

fun close() MyResult { MyResult(MyErr("fail")) }

fun try_it() MyResult {
    match close() {
        case MyErr e: return e
        case void: DebugIntern.print_str("ok")
    }
}

fun main() void {
    match try_it() {
        case void: DebugIntern.print_str("void")
        case MyErr e: DebugIntern.print_str(e.msg)
    }
}
```

```output
fail
```

**union auto-wrap in generic function call with struct construction**

```metall
struct Wrapper { value Str }
union Foo = Wrapper | Int

fun replace<T>(old T, new T) T { new }

fun main() void {
    let old Foo = 42
    let x = replace(old, Wrapper("hello"))
    match x {
        case Wrapper w: DebugIntern.print_str(w.value)
        else: DebugIntern.print_str("other")
    }
}
```

```output
hello
```

**union with struct payload and complex branching**

Regression test: a union with a multi-field struct payload (> 8 bytes) returned from
multiple `when` branches that perform inline computation can cause LLVM's SROA to
decompose stores into byte-level operations, preventing instcombine from reaching
a fixpoint.

```metall
struct Pos { a U32 b Int }

struct Iter {
    data []U8
    mut pos Int
}

fun Iter.next(it &mut Iter) ?Pos {
    if it.pos == it.data.len {
        return None()
    }
    let start = it.pos
    let b0 = it.data[it.pos].to_u32()
    it.pos = it.pos + 1
    when {
    case b0 < 128:
        Pos(b0, start)
    case b0 < 224:
        let b1 = it.data[it.pos].to_u32()
        it.pos = it.pos + 1
        Pos((b0 & 31) << 6 | (b1 & 63), start)
    case b0 < 240:
        let b1 = it.data[it.pos].to_u32()
        it.pos = it.pos + 1
        let b2 = it.data[it.pos].to_u32()
        it.pos = it.pos + 1
        Pos((b0 & 15) << 12 | (b1 & 63) << 6 | (b2 & 63), start)
    case b0 < 248:
        let b1 = it.data[it.pos].to_u32()
        it.pos = it.pos + 1
        let b2 = it.data[it.pos].to_u32()
        it.pos = it.pos + 1
        let b3 = it.data[it.pos].to_u32()
        it.pos = it.pos + 1
        Pos((b0 & 7) << 18 | (b1 & 63) << 12 | (b2 & 63) << 6 | (b3 & 63), start)
    else:
        panic("bad")
    }
}

fun main() void {
    mut it = Iter("hi".data, 0)
    match (&mut it).next() {
        case Pos p: DebugIntern.print_int(p.a.to_int())
        else: DebugIntern.print_str("none")
    }
}
```

```output
104
```

## Function Literals

**function literal basic**

```metall
fun apply(f fun(Int) Int, x Int) Int { f(x) }

fun main() void {
    let double = fun(x Int) Int { x + x }
    DebugIntern.print_int(apply(double, 21))
    DebugIntern.print_int(apply(fun(x Int) Int { x + 1 }, 99))
}
```

```output
42
100
```

**closures**

```metall
fun apply(f fun(Int) Int, x Int) Int { f(x) }

fun main() void {
    -- capture by value
    let x = 10
    DebugIntern.print_int(fun[x](a Int) Int { x + a }(5))
    -- multiple captures
    let a = 10
    let b = 20
    DebugIntern.print_int(fun[a, b]() Int { a + b }())
    -- capture by value is a snapshot
    mut y = 1
    let snap = fun[y]() Int { y }
    y = 99
    DebugIntern.print_int(snap())
    -- capture by ref
    let z = 42
    DebugIntern.print_int(fun[&z]() Int { z.* }())
    -- capture by mut ref
    mut w = 0
    let inc = fun[&mut w]() void { w.* = w.* + 1 }
    inc()
    inc()
    DebugIntern.print_int(w)
    -- capture existing ref by value
    let r = &z
    DebugIntern.print_int(fun[r]() Int { r.* }())
    -- capture existing mut ref by value
    mut m = 10
    let mr = &mut m
    fun[mr]() void { mr.* = mr.* + 1 }()
    DebugIntern.print_int(m)
    -- passed to higher-order function
    let offset = 100
    DebugIntern.print_int(apply(fun[offset](n Int) Int { offset + n }, 23))
}
```

```output
15
30
1
42
2
42
11
123
```

**closure captures allocator**

```metall
fun main() void {
    let @a = Arena()
    let make_slice = fun[@a]() []Int { @a.slice<Int>(3, 42) }
    let items = make_slice()
    DebugIntern.print_int(items[0])
    DebugIntern.print_int(items[1])
    DebugIntern.print_int(items[2])
}
```

```output
42
42
42
```

## Defer

**defer basic**

```metall
fun main() void {
    DebugIntern.print_int(1)
    defer { DebugIntern.print_int(3) }
    DebugIntern.print_int(2)
}
```

```output
1
2
3
```

**defer reverse order**

```metall
fun main() void {
    defer { DebugIntern.print_int(3) }
    defer { DebugIntern.print_int(2) }
    defer { DebugIntern.print_int(1) }
}
```

```output
1
2
3
```

**defer runs before return**

```metall
fun foo() Int {
    defer { DebugIntern.print_int(1) }
    return 42
}

fun main() void {
    DebugIntern.print_int(foo())
}
```

```output
1
42
```

**defer in inner block with return**

```metall
fun foo() Int {
    defer { DebugIntern.print_int(1) }
    if true {
        defer { DebugIntern.print_int(2) }
        return 42
    }
    0
}

fun main() void {
    DebugIntern.print_int(foo())
}
```

```output
2
1
42
```

**defer in loop with break**

```metall
fun main() void {
    for i in 0..3 {
        defer { DebugIntern.print_int(i * 10) }
        if i == 1 {
            break
        }
        DebugIntern.print_int(i)
    }
}
```

```output
0
0
10
```

**defer in loop with continue**

```metall
fun main() void {
    for i in 0..3 {
        defer { DebugIntern.print_int(i * 10) }
        if i == 1 {
            continue
        }
        DebugIntern.print_int(i)
    }
}
```

```output
0
0
10
2
20
```

**defer reads arena-allocated data before arena is destroyed**

```metall
fun foo() void {
    let @a = Arena()
    let x = @a.new<Int>(42)
    defer { DebugIntern.print_int(x.*) }
    DebugIntern.print_int(1)
}

fun main() void {
    foo()
}
```

```output
1
42
```

**defer allocates with surrounding arena**

```metall
fun foo() void {
    let @a = Arena()
    defer {
        let x = @a.new<Int>(99)
        DebugIntern.print_int(x.*)
    }
    let y = @a.new<Int>(1)
    DebugIntern.print_int(y.*)
}

fun main() void {
    foo()
}
```

```output
1
99
```

## Type Sugar

**Option and Result sugar**

```metall
fun maybe(x Bool) ?Int {
    if x { Option(42) }
    else { Option(None()) }
}

fun try_it(x Bool) !Int {
    if x { Result(1) }
    else { Result(Err("nope")) }
}

fun main() void {
    match maybe(true) {
        case Int i: DebugIntern.print_int(i)
        case None: DebugIntern.print_str("none")
    }
    match maybe(false) {
        case Int: DebugIntern.print_str("int")
        case None: DebugIntern.print_str("none")
    }
    match try_it(true) {
        case Int i: DebugIntern.print_int(i)
        case Err e: DebugIntern.print_str(e.msg)
    }
    match try_it(false) {
        case Int: DebugIntern.print_str("int")
        case Err e: DebugIntern.print_str(e.msg)
    }
}
```

```output
42
none
1
nope
```

## Try

**try expression**

```metall
fun main() void {
    fun might_fail(ok Bool) Result<Int> {
        if ok { 42 } else { Err("fail") }
    }

    fun short_form() Result<Int> {
        let x = try might_fail(true)
        DebugIntern.print_int(x)
        try might_fail(false)
    }
    match short_form() {
        case Int: DebugIntern.print_str("int")
        case Err e: DebugIntern.print_str(e.msg)
    }

    fun with_else() Str {
        let x = try might_fail(false) else e { return e.msg }
        DebugIntern.print_int(x)
        "ok"
    }
    DebugIntern.print_str(with_else())

    fun with_is() Err {
        let n = try might_fail(true) is Int else e { return e }
        DebugIntern.print_int(n)
        Err("done")
    }
    DebugIntern.print_str(with_is().msg)

    fun with_void() Err {
        fun check(ok Bool) Result<void> {
            if ok { void } else { Err("void fail") }
        }
        try check(true)
        try check(false)
        Err("all ok")
    }
    DebugIntern.print_str(with_void().msg)

    fun no_binding() Str {
        let n = try might_fail(false) else { return "no binding" }
        DebugIntern.print_int(n)
        "ok"
    }
    DebugIntern.print_str(no_binding())
}
```

```output
42
fail
fail
42
done
void fail
no binding
```

## Panic

**panic**

```metall
fun main() void {
    panic("hello")
}
```

```panic
test.met:2:5: hello
```

**int divide by zero**

```metall
fun main() void {
    _ = 1 / 0
}
```

```panic
test.met:2:9: division by zero
```

**int modulo by zero**

```metall
fun main() void {
    _ = 1 % 0
}
```

```panic
test.met:2:9: division by zero
```

**rune arithmetic overflow**

```metall
fun main() void {
    _ = '😀' * 9
}
```

```panic
test.met:2:9: illegal rune
```

**rune arithmetic underflow**

```metall
fun main() void {
    _ = 'a' - 'b'
}
```

```panic
test.met:2:9: integer overflow
```

**rune arithmetic into surrogate**

```metall
fun main() void {
    _ = '퟿' + 1
}
```

```panic
test.met:2:9: illegal rune
```

**array index out of bounds**

```metall
fun main() void {
    let arr = [10, 20, 30]
    DebugIntern.print_int(arr[3])
}
```

```panic
test.met:3:27: index out of bounds
```

**array index negative**

```metall
fun main() void {
    let arr = [10, 20, 30]
    let i = -1
    DebugIntern.print_int(arr[i])
}
```

```panic
test.met:4:27: index out of bounds
```

**slice index out of bounds**

```metall
fun main() void {
    let @a = Arena()
    let s = @a.slice<Int>(3, 0)
    DebugIntern.print_int(s[3])
}
```

```panic
test.met:4:27: index out of bounds
```

**array write index out of bounds**

```metall
fun main() void {
    mut arr = [10, 20, 30]
    arr[3] = 99
}
```

```panic
test.met:3:5: index out of bounds
```

**subslice hi out of bounds**

```metall
fun main() void {
    let arr = [10, 20, 30]
    let s = arr[0..4]
}
```

```panic
test.met:3:13: slice out of bounds
```

**subslice lo greater than hi**

```metall
fun main() void {
    let arr = [10, 20, 30]
    let s = arr[2..1]
}
```

```panic
test.met:3:13: slice out of bounds
```

**subslice of slice out of bounds**

```metall
fun main() void {
    let @a = Arena()
    let s = @a.slice<Int>(3, 0)
    let sub = s[0..4]
}
```

```panic
test.met:4:15: slice out of bounds
```

### Panic (i.e. unreachable) in all places

**panic in if then branch**

```metall
fun main() void {
    if false { panic("boom") } else { DebugIntern.print_str("ok") }
}
```

```output
ok
```

**panic in if else branch**

```metall
fun main() void {
    if true { DebugIntern.print_str("ok") } else { panic("boom") }
}
```

```output
ok
```

**panic in both if branches**

```metall
fun foo() Int {
    if true { panic("boom") } else { panic("boom2") }
}
fun main() void {
    _ = foo()
}
```

```panic
test.met:2:15: boom
```

**panic in match arm**

```metall
fun main() void {
    union AB = Int | Bool
    match AB(true) {
        case Int: panic("boom")
        case Bool: DebugIntern.print_str("ok")
    }
}
```

```output
ok
```

**panic in all match arms**

```metall
fun main() void {
    union AB = Int | Bool
    match AB(1) {
        case Int: panic("boom")
        case Bool: panic("boom2")
    }
}
```

```panic
test.met:4:19: boom
```

**panic in match else**

```metall
fun main() void {
    union ABC = Int | Bool | Str
    match ABC(1) {
        case Int: DebugIntern.print_str("ok")
        else: panic("boom")
    }
}
```

```output
ok
```

**panic in when arm**

```metall
fun main() void {
    when {
        case false: panic("boom")
        else: DebugIntern.print_str("ok")
    }
}
```

```output
ok
```

**panic in when else**

```metall
fun main() void {
    when {
        case true: DebugIntern.print_str("ok")
        else: panic("boom")
    }
}
```

```output
ok
```

**panic in all when branches**

```metall
fun foo() Int {
    when {
        case false: panic("a")
        case false: panic("b")
        else: panic("c")
    }
}
fun main() void {
    _ = foo()
}
```

```panic
test.met:5:15: c
```

**panic in try else**

```metall
fun main() void {
    fun fail() Result<Int> { Err("fail") }
    let x = try fail() else { panic("boom") }
    DebugIntern.print_int(x)
}
```

```panic
test.met:3:31: boom
```

**panic in loop body**

```metall
fun main() void {
    mut i = 0
    for {
        if i == 3 { panic("done") }
        i = i + 1
    }
}
```

```panic
test.met:4:21: done
```

**panic as function return**

```metall
fun never_returns() Int {
    panic("nope")
}

fun main() void {
    _ = never_returns()
}
```

```panic
test.met:2:5: nope
```

## Macros

**simple macro**

```metall
use local::hello_macro

hello_macro::apply()

fun main() void {
    greet()
}
```

```output
hello from macro
```

**macro with argument**

```metall
use local::repeat_macro

repeat_macro::apply(3)

fun main() void {
    repeat()
}
```

```output
hello 
hello 
hello 
```

**and short-circuits on false**

```metall
fun side_effect() Bool {
    DebugIntern.print_str("rhs")
    true
}

fun main() void {
    if false and side_effect() {
        DebugIntern.print_str("branch")
    }
    DebugIntern.print_str("done")
}
```

```output
done
```

**and evaluates rhs on true**

```metall
fun side_effect() Bool {
    DebugIntern.print_str("rhs")
    true
}

fun main() void {
    if true and side_effect() {
        DebugIntern.print_str("branch")
    }
    DebugIntern.print_str("done")
}
```

```output
rhs
branch
done
```

**or short-circuits on true**

```metall
fun side_effect() Bool {
    DebugIntern.print_str("rhs")
    true
}

fun main() void {
    if true or side_effect() {
        DebugIntern.print_str("branch")
    }
    DebugIntern.print_str("done")
}
```

```output
branch
done
```

**or evaluates rhs on false**

```metall
fun side_effect() Bool {
    DebugIntern.print_str("rhs")
    true
}

fun main() void {
    if false or side_effect() {
        DebugIntern.print_str("branch")
    }
    DebugIntern.print_str("done")
}
```

```output
rhs
branch
done
```

**and short-circuits in variable binding**

```metall
fun side_effect() Bool {
    DebugIntern.print_str("rhs")
    true
}

fun main() void {
    let x = false and side_effect()
    DebugIntern.print_bool(x)
}
```

```output
false
```

**or short-circuits in variable binding**

```metall
fun side_effect() Bool {
    DebugIntern.print_str("rhs")
    false
}

fun main() void {
    let x = true or side_effect()
    DebugIntern.print_bool(x)
}
```

```output
true
```

## Module Constants

**module-level let with all allowed types**

```metall
struct Point { x Int y Int }

let a = 42
let b = "hello"
let c = true
let d = 'A'
let e = Point(1, 2)
let f = [10, 20, 30]
let g = &a
let h = a + 1
let i = U8(32)

fun get_point_x() Int { e.x }

fun main() void {
    DebugIntern.print_int(a)
    DebugIntern.print_str(b)
    DebugIntern.print_bool(c)
    DebugIntern.print_uint(d.to_u32().to_u64())
    DebugIntern.print_int(e.x)
    DebugIntern.print_int(e.y)
    DebugIntern.print_int(f[0])
    DebugIntern.print_int(f[1])
    DebugIntern.print_int(f[2])
    DebugIntern.print_int(g.*)
    DebugIntern.print_int(h)
    DebugIntern.print_int(get_point_x())
    DebugIntern.print_uint(i.to_u64())
}
```

```output
42
hello
true
65
1
2
10
20
30
42
43
1
32
```

**module-level let imported from another module**

```metall
use local::e2e

let local_answer = e2e::the_answer + 1

fun main() void {
    DebugIntern.print_int(e2e::the_answer)
    DebugIntern.print_int(local_answer)
}
```

```output
42
43
```

## Unsafe

**unsafe uninit slice of ref type**

```metall
fun main() void {
    let @a = Arena()
    let x = unsafe @a.slice_uninit_mut<&Int>(3)
    mut v = 42
    x[0] = &v
    DebugIntern.print_int(x[0].*)
}
```

```output
42
```

## Slice Methods

**slice method**

```metall
fun Slice.first<T>(s []T) T { s[0] }

fun main() void {
    let x = [10, 20, 30]
    DebugIntern.print_int(x[..].first())
}
```

```output
10
```

**module-level let with subslice**

```metall
struct Wrapper { data []U8 }
let delim = [U8(47)][..]
let w = Wrapper([U8(1), 2, 3][1..])

fun main() void {
    DebugIntern.print_int(delim.len)
    DebugIntern.print_int(delim[0].to_int())
    DebugIntern.print_int(w.data.len)
    DebugIntern.print_int(w.data[0].to_int())
    DebugIntern.print_int(w.data[1].to_int())
}
```

```output
1
47
2
2
3
```

## Main returning !void

**main returns !void success**

```metall
fun main() !void {}
```

```output
```

**main returns !void error**

```metall
fun main() !void {
    Err("something went wrong")
}
```

```panic
something went wrong
```

**main returns !void error but panics**

```metall
fun main() !void {
    panic("something went wrong")
}
```

```panic
test.met:2:5: something went wrong
```

## FFI

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

**ffi sizeof and pointers**

```metall
use std::ffi

fun main() void {
    DebugIntern.print_int(ffi::sizeof<U8>())
    DebugIntern.print_int(ffi::sizeof<I32>())
    DebugIntern.print_int(ffi::sizeof<Int>())
    DebugIntern.print_int(ffi::sizeof<Bool>())
    DebugIntern.print_int(ffi::sizeof<[]U8>())
    DebugIntern.print_int(ffi::sizeof<Str>())
    struct Pair { a Int b Int }
    DebugIntern.print_int(ffi::sizeof<Pair>())

    let x = 42
    let p = ffi::ref_ptr<Int>(&x)
    DebugIntern.print_bool(ffi::sizeof<Int>() > 0)

    let text = [U8(65), 65, 65, 0][..]
    let ptr = ffi::slice_ptr<U8>(text)
    DebugIntern.print_bool(ffi::sizeof<ffi::Ptr<U8>>() > 0)
}
```

```output
1
4
8
1
16
16
16
true
true
```

**ffi strlen via slice_ptr**

```metall
use std::ffi

extern fun strlen(s ffi::Ptr<U8>) Int

fun main() void {
    let text = [U8(65), 65, 65, 0][..]
    let ptr = ffi::slice_ptr<U8>(text)
    let len = unsafe strlen(ptr)
    DebugIntern.print_int(len)
}
```

```output
3
```

**call extern function from imported module and main module**

```metall
use local::e2e_ffi

extern fun abs(n I32) I32

fun main() void {
    let x = unsafe e2e_ffi::abs(I32(-7))
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
use local::e2e_ffi

fun abs(n Int) Int {
    if n < 0 { n * -1 } else { n }
}

fun main() void {
    let x = unsafe e2e_ffi::abs(I32(-7))
    DebugIntern.print_int(x.to_int())
    let y = abs(-42)
    DebugIntern.print_int(y)
}
```

```output
7
42
```

**ffi PtrMut.write**

```metall
use std::ffi

struct Pair { mut a Int mut b Int }

fun main() void {
    mut x = 0
    let p = ffi::ref_ptr_mut<Int>(&mut x)
    unsafe p.write(42)
    DebugIntern.print_int(x)

    -- PtrMut.write copies the value; mutating the source doesn't affect the target.
    mut target = Pair(0, 0)
    let tp = ffi::ref_ptr_mut<Pair>(&mut target)
    mut source = Pair(1, 2)
    unsafe tp.write(source)
    source.a = 99
    source.b = 99
    DebugIntern.print_int(target.a)
    DebugIntern.print_int(target.b)
}
```

```output
42
1
2
```

**ffi Ptr.read and PtrMut.read**

```metall
use std::ffi

struct Pair { a Int b Int }

fun main() void {
    -- Ptr.read on immutable reference
    let x = 42
    let p = ffi::ref_ptr<Int>(&x)
    DebugIntern.print_int(unsafe p.read())

    -- PtrMut.read on mutable reference
    mut y = 99
    let pm = ffi::ref_ptr_mut<Int>(&mut y)
    DebugIntern.print_int(unsafe pm.read())

    -- read a struct through a pointer
    let pair = Pair(1, 2)
    let pp = ffi::ref_ptr<Pair>(&pair)
    let copy = unsafe pp.read()
    DebugIntern.print_int(copy.a)
    DebugIntern.print_int(copy.b)
}
```

```output
42
99
1
2
```

**ffi pointer arithmetic on struct slice**

```metall
use std::ffi

struct Vec2 { mut x Int mut y Int }

fun main() void {
    let @a = Arena()
    let data = @a.slice_mut<Vec2>(3, Vec2(0, 0))
    let base = ffi::slice_ptr_mut<Vec2>(data)

    -- write all elements via pointer arithmetic
    unsafe base.offset(0).write(Vec2(10, 20))
    unsafe base.offset(1).write(Vec2(30, 40))
    unsafe base.offset(2).write(Vec2(50, 60))

    -- read all elements back via pointer arithmetic on immutable ptr
    let rp = ffi::slice_ptr<Vec2>(data)
    let v0 = unsafe rp.offset(0).read()
    let v1 = unsafe rp.offset(1).read()
    let v2 = unsafe rp.offset(2).read()
    DebugIntern.print_int(v0.x)
    DebugIntern.print_int(v0.y)
    DebugIntern.print_int(v1.x)
    DebugIntern.print_int(v1.y)
    DebugIntern.print_int(v2.x)
    DebugIntern.print_int(v2.y)

    -- also works on primitive types
    let ints = @a.slice_mut<Int>(2, 0)
    let ip = ffi::slice_ptr_mut<Int>(ints)
    unsafe ip.offset(0).write(100)
    unsafe ip.offset(1).write(200)
    let irp = ffi::slice_ptr<Int>(ints)
    DebugIntern.print_int(unsafe irp.offset(0).read())
    DebugIntern.print_int(unsafe irp.offset(1).read())
}
```

```output
10
20
30
40
50
60
100
200
```

**ffi Ptr.as_slice and PtrMut.as_slice**

```metall
use std::ffi

fun main() void {
    let @a = Arena()
    let data = @a.slice_mut<Int>(3, 0)

    -- write via PtrMut, read back via as_slice
    let wp = ffi::slice_ptr_mut<Int>(data)
    unsafe wp.offset(0).write(10)
    unsafe wp.offset(1).write(20)
    unsafe wp.offset(2).write(30)
    let rp = ffi::slice_ptr<Int>(data)
    let s = unsafe rp.as_slice(3)
    DebugIntern.print_int(s[0])
    DebugIntern.print_int(s[1])
    DebugIntern.print_int(s[2])
    DebugIntern.print_int(s.len)

    -- PtrMut.as_slice returns a mutable slice
    let ms = unsafe wp.as_slice(3)
    ms[0] = 99
    DebugIntern.print_int(data[0])
}
```

```output
10
20
30
3
99
```

**ffi is_null with C function returning null**

```metall
use std::ffi

extern fun fopen(path ffi::Ptr<U8>, mode ffi::Ptr<U8>) ffi::PtrMut<U8>

fun main() void {
    let path = ffi::slice_ptr<U8>([U8(0)][..])
    let mode = ffi::slice_ptr<U8>([U8(0)][..])
    let fp = unsafe fopen(path, mode)
    DebugIntern.print_bool(fp.is_null())
    let x = 42
    let p = ffi::ref_ptr<Int>(&x)
    DebugIntern.print_bool(p.is_null())
}
```

```output
true
false
```

**extern function alias**

```metall
extern my_abs = fun abs(n I32) I32

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
