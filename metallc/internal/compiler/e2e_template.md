# E2E Template Tests

These cases exercise the template shorthand syntax end to end. The typechecker
suite covers the tiny rule probes; this file keeps runnable examples that show
the generated code behaving correctly.

## Owner Shorthand

**bare owner type carries owner params**

```metall
struct Box<T> {
    value T
}

fun Box.get(b Box) T {
    b.value
}

fun Box.keep(b Box) Box {
    b
}

fun Box.pick_int(b Box, other Box<Int>) Int {
    other.value
}

fun main() void {
    DebugIntern.print_int(Box<Int>(40).keep().get() + 2)
    DebugIntern.print_str(Box<Str>("ok").get())
    DebugIntern.print_int(Box<Str>("ignored").pick_int(Box<Int>(7)))
}
```

```output
42
ok
7
```

**method-local params and explicit extras**

```metall
struct Box<T> {
    value T
}

struct Pair<A, B> {
    left A
    right B
}

fun Box.get(b Box) T {
    b.value
}

fun Box.replace<U>(b Box, value U) Box<U> {
    Box(value)
}

fun Box.pair_with<U>(b Box, value U) Pair<T, U> {
    Pair(b.value, value)
}

fun main() void {
    DebugIntern.print_str(Box<Int>(1).replace("changed").get())

    let p = Box<Int>(5).pair_with<Str>("tail")
    DebugIntern.print_int(p.left)
    DebugIntern.print_str(p.right)
}
```

```output
changed
5
tail
```

**owner param can be used without receiver**

```metall
struct Box<T> {
    value T
}

fun Box.echo(value T) T {
    value
}

fun main() void {
    DebugIntern.print_int(Box.echo<Int>(42))
    DebugIntern.print_str(Box.echo<Str>("echo"))
}
```

```output
42
echo
```

## Constraints

**owner constraint is retained by shorthand**

```metall
shape Same {
    fun Same.same(a Same, b Same) Bool
}

struct Score {
    points Int
}

fun Score.same(a Score, b Score) Bool {
    a.points == b.points
}

struct EqualBox<T Same> {
    value T
}

fun EqualBox.same_value(a EqualBox, b EqualBox) Bool {
    a.value.same(b.value)
}

fun EqualBox.get(a EqualBox) T {
    a.value
}

fun main() void {
    DebugIntern.print_bool(EqualBox(Score(3)).same_value(EqualBox(Score(3))))
    DebugIntern.print_int(EqualBox(Score(4)).get().points)
}
```

```output
true
4
```

**explicit method param can type the owner**

```metall
shape Same {
    fun Same.same(a Same, b Same) Bool
}

struct Score {
    points Int
}

fun Score.same(a Score, b Score) Bool {
    a.points == b.points
}

struct Box<T> {
    value T
}

fun Box.same<U Same>(a Box<U>, b Box<U>) Bool {
    a.value.same(b.value)
}

fun main() void {
    DebugIntern.print_bool(Box<Score>(Score(8)).same(Box<Score>(Score(8))))
}
```

```output
true
```

## Attached Slots

**shape member shorthand exposes attached slot**

```metall
shape Cell<Value> {
    fun Cell.get(c Cell) Value
}

struct IntCell {
    value Int
}

fun IntCell.get(c IntCell) Int {
    c.value
}

fun read<C Cell>(c C) C.Value {
    c.get()
}

fun main() void {
    DebugIntern.print_int(read(IntCell(42)))
}
```

```output
42
```

**attached slot feeds mapper**

```metall
shape Cell<Value> {
    fun Cell.get(c Cell) Value
}

struct IntCell {
    value Int
}

fun IntCell.get(c IntCell) Int {
    c.value
}

struct Mapped<In Cell, Out> {
    input In
    map fun(In.Value) Out
}

fun Mapped.get(m Mapped) Out {
    m.map(m.input.get())
}

fun double(x Int) Int {
    x * 2
}

fun main() void {
    DebugIntern.print_int(Mapped(IntCell(21), double).get())
}
```

```output
42
```

**attached slot constrains another parameter**

```metall
shape Cell<Value> {
    fun Cell.get(c Cell) Value
}

struct IntCell {
    value Int
}

fun IntCell.get(c IntCell) Int {
    c.value
}

struct OtherIntCell {
    value Int
}

fun OtherIntCell.get(c OtherIntCell) Int {
    c.value
}

struct SameValuePair<Left Cell, Right Cell<Left.Value>> {
    left Left
    right Right
}

fun SameValuePair.right_value(p SameValuePair) Left.Value {
    p.right.get()
}

fun main() void {
    DebugIntern.print_int(SameValuePair(IntCell(1), OtherIntCell(2)).right_value())
}
```

```output
2
```

**attached slots can be chained**

```metall
shape Cell<Value> {
    fun Cell.get(c Cell) Value
}

shape Holder<Inner Cell> {
    fun Holder.inner(h Holder) Inner
}

struct IntCell {
    value Int
}

fun IntCell.get(c IntCell) Int {
    c.value
}

struct IntHolder {
    cell IntCell
}

fun IntHolder.inner(h IntHolder) IntCell {
    h.cell
}

struct Wrapped<H Holder> {
    holder H
}

fun Wrapped.get(w Wrapped) H.Inner.Value {
    w.holder.inner().get()
}

fun main() void {
    DebugIntern.print_int(Wrapped(IntHolder(IntCell(7))).get())
}
```

```output
7
```

## Composed Cases

**iterator-style source pipeline**

```metall
shape Source<Item> {
    fun Source.next(s &mut Source) ?Item
}

shape SourceBox<Inner Source> {
    fun SourceBox.source(b &mut SourceBox) &mut Inner
}

struct Counter {
    current Int
    stop Int
}

fun Counter.next(c &mut Counter) ?Int {
    if c.current >= c.stop {
        return None()
    }
    let value = c.current
    c.current = c.current + 1
    value
}

struct CounterBox {
    wrapped Counter
}

fun CounterBox.source(b &mut CounterBox) &mut Counter {
    &mut b.wrapped
}

struct MapSource<In Source, Out> {
    source In
    mapper fun(In.Item) Out
}

fun MapSource.next(m &mut MapSource) ?Out {
    match (&mut m.source).next() {
        case None: None()
        else value: m.mapper(value)
    }
}

struct Pipeline<In Source> {
    source In
}

fun pipe<In Source>(source In) Pipeline<In> {
    Pipeline(source)
}

fun Pipeline.map<Out>(p Pipeline, mapper fun(In.Item) Out) Pipeline<MapSource<In, Out>> {
    Pipeline(MapSource(p.source, mapper))
}

fun Pipeline.next(p &mut Pipeline) ?In.Item {
    (&mut p.source).next()
}

struct BoxPipeline<Box SourceBox> {
    box Box
}

fun BoxPipeline.next(p &mut BoxPipeline) ?Box.Inner.Item {
    (&mut p.box).source().next()
}

fun triple(x Int) Int {
    x * 3
}

fun main() void {
    mut p = pipe(Counter(0, 4)).map(triple)
    for {
        match (&mut p).next() {
            case None: break
            else value: DebugIntern.print_int(value)
        }
    }

    mut bp = BoxPipeline(CounterBox(Counter(10, 12)))
    for {
        match (&mut bp).next() {
            case None: break
            else value: DebugIntern.print_int(value)
        }
    }
}
```

```output
0
3
6
9
10
11
```

## Boundaries

**unconstrained type parameter has no methods**

```metall
struct Guitar {
    value Int
}

fun Guitar.answer(g Guitar) Int {
    g.value
}

fun unchecked<T>(x T) Int {
    x.answer()
}

fun main() void {
    DebugIntern.print_int(unchecked(Guitar(42)))
}
```

```error
test.met:10:7: unconstrained type parameter has no fields or methods: T
    fun unchecked<T>(x T) Int {
        x.answer()
          ^^^^^^
    }
```

**method parameter cannot shadow owner parameter**

```metall
struct Box<T> {
    value T
}

fun Box.same<T>(a Box, b Box) Bool {
    true
}

fun main() void {}
```

```error
test.met:5:14: type parameter T shadows owner type parameter
    
    fun Box.same<T>(a Box, b Box) Bool {
                 ^
        true
```
