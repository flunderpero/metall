# Not Fully Fleshed Out Ideas

## Templates And Interfaces

```
-- iter.met

trait Iter($IItem) {
} constraints {
    fun next(it &mut Iter($IItem)) $IItem | None
}

-- Default implementations. Available to any type that `impl Iter`.
-- Can be overridden by a function with matching signature in the
-- implementor's module. Resolution is simple: local function beats
-- imported default. Templates are just search-and-replace: the compiler
-- substitutes concrete types for placeholders and then type-checks the
-- result as if you had written it by hand.

fun count(it &mut Iter($IItem)) Int {
    mut count = 0
    loop {
        if it.next().is_none() { return count }
        count += 1
    }
}

fun find(it &mut Iter($IItem), pred fun($IItem) Bool) $IItem | None {
    loop {
        let item = it.next()
        if item.is_none() { return None }
        if pred(item) { return item }
    }
}

-- sized.met

trait Sized {
    -- Traits can require fields. These fields must be present in implementing structs.
    len Int
}

fun is_empty(s Sized) Bool {
    s.len == 0
}

-- list.met

import iter
import sized

struct List($Item) {
    len Int
    cap Int
    data []$Item
} constraints {
    impl Sized
}

fun new(type List($Item), initial []$Item) List($Item) {
    type(len=0, cap=0, data=initial)
}

struct ListIter($Item) {
    list List($Item)
    mut i Int
} constraints {
    impl Iter($Item)
}

fun next(it &mut ListIter($Item)) $Item | None {
    if it.i >= it.list.len {
        return None
    }
    it.i += 1
    it.list.data[it.i - 1]
}

fun iter(list List($Item)) ListIter($Item) {
    ListIter($Item)(list=list, i=0)
}

-- Usage:

let numbers = List(Int).new([1, 2, 3])
numbers.is_empty()             -- false, via Sized
let c = numbers.iter().count() -- 3, via Iter default
let n = numbers.iter().find({$1 == 2})

fun avg(it &mut Iter(Int)) Int {
    it.count() / it.sum()
}

fun debug_len(s Sized) {
    print(s.len)
}

-- ast.met

trait Node {
    node_id NodeId
    span Span
}

fun debug(n Node) Str { ... }
fun source_text(n Node) Str { ... }

-- ast_expr.met

import ast

struct If {
    node_id NodeId
    span Span
    cond NodeId
    then NodeId
    else NodeId | None
} constraints {
    impl Node
}

struct Call {
    node_id NodeId
    span Span
    callee NodeId
    args []NodeId
} constraints {
    impl Node
}

struct Int {
    node_id NodeId
    span Span
    value Int
} constraints {
    impl Node
}

-- Every node type explicitly declares node_id and span — no magic,
-- no hidden fields, no embedding. The constraint just verifies they
-- are there and unlocks UFCS for all functions in ast.met.

let my_node = Int(node_id=NodeId(1), span=some_span, value=42)
my_node.debug()        -- works, debug is in ast.met, Int impl Node
my_node.source_text()  -- works, same reason
```
