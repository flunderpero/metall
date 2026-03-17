# Metall Parser Tests

## Examples

**A basic example of full module parsing**

```metall module
fun foo() Str { "hello" 123 } 
```

```ast
Module(fileName="test.met",name="test",main=true)
  decls=Fun(name="foo")
    returnType=SimpleType(name="Str")
    block=Block()
      exprs[0]=String(value="hello")
      exprs[1]=Int(value=123)
```

## Bindings And Assign

```metall
x = 123
```

```ast
Assign()
  lhs=Ident(name="x")
  rhs=Int(value=123)
```

**Let binding**

```metall
let x = 123
```

```ast
Var(name="x",mut=false)
  expr=Int(value=123)
```

**Mut binding**

```metall
mut x = 123
```

```ast
Var(name="x",mut=true)
  expr=Int(value=123)
```

**Let binding with type annotation**

```metall
let x Str = "hello"
```

```ast
Var(name="x",mut=false)
  type=SimpleType(name="Str")
  expr=String(value="hello")
```

**Let binding with invalid type annotation**

```metall
let x 123 = "hello"
```

```error
test.met:1:7: unexpected token: expected <type identifier> or &, got <number>
    let x 123 = "hello"
          ^^^
```

**Assigning to a type name should fail**

```metall
{ Str = "hello" }
```

```error
test.met:1:7: unexpected token: expected (, got =
    { Str = "hello" }
          ^
```

## Blocks

```metall
{ 0 "hello" }
```

```ast
Block()
  exprs[0]=Int(value=0)
  exprs[1]=String(value="hello")
```

**Empty block**

```metall
{ }
```

```ast
Block()
```

**Block must be closed even at EOF**

```metall
{
```

```error
test.met:1:1: unexpected end of file
    {
    ^
```

## Comments

**Line comment**

```metall
-- comment
 123
```

```ast
Int(value=123)
```

**Multi-line comment**

```metall
--- multi
    line
    comment
---
123
```

```ast
Int(value=123)
```

## Functions

**Fun with &mut param**

```metall
fun foo(a Int, b &mut Str) Int { 123 }
```

```ast
Fun(name="foo")
  params[0]=FunParam(name="a")
    type=SimpleType(name="Int")
  params[1]=FunParam(name="b")
    type=RefType(mut=true)
      type=SimpleType(name="Str")
  returnType=SimpleType(name="Int")
  block=Block()
    exprs=Int(value=123)
```

**Fun in block**

```metall
{ fun foo() Str { "hello" 123 } }
```

```ast
Block()
  exprs=Fun(name="foo")
    returnType=SimpleType(name="Str")
    block=Block()
      exprs[0]=String(value="hello")
      exprs[1]=Int(value=123)
```

**Void fun**

```metall
fun foo() void {}
```

```ast
Fun(name="foo")
  returnType=SimpleType(name="void")
  block=Block()
```

**Fun call**

```metall
foo(123, "hello")
```

```ast
Call()
  callee=Ident(name="foo")
  args[0]=Int(value=123)
  args[1]=String(value="hello")
```

**Call no args**

```metall
foo()
```

```ast
Call()
  callee=Ident(name="foo")
```

**Chained call**

```metall
foo()()
```

```ast
Call()
  callee=Call()
    callee=Ident(name="foo")
```

**Fun type**

```metall
fun foo(bar fun(Str, Int) Bool) void {}
```

```ast
Fun(name="foo")
  params=FunParam(name="bar")
    type=FunType()
      params[0]=SimpleType(name="Str")
      params[1]=SimpleType(name="Int")
      returnType=SimpleType(name="Bool")
  returnType=SimpleType(name="void")
  block=Block()
```

**Fun type with void return**

```metall
fun foo(bar fun(Int) void) void {}
```

```ast
Fun(name="foo")
  params=FunParam(name="bar")
    type=FunType()
      params=SimpleType(name="Int")
      returnType=SimpleType(name="void")
  returnType=SimpleType(name="void")
  block=Block()
```

**Void ident expr**

```metall
void
```

```ast
Ident(name="void")
```

**Return**

```metall
fun foo() Int { return 123 }
```

```ast
Fun(name="foo")
  returnType=SimpleType(name="Int")
  block=Block()
    exprs=Return()
      expr=Int(value=123)
```

**Return void**

```metall
fun foo() void { return void }
```

```ast
Fun(name="foo")
  returnType=SimpleType(name="void")
  block=Block()
    exprs=Return()
      expr=Ident(name="void")
```

**Namespaced fun**

```metall
fun Foo.bar(f Foo) Int { 123 }
```

```ast
Fun(name="Foo.bar")
  params=FunParam(name="f")
    type=SimpleType(name="Foo")
  returnType=SimpleType(name="Int")
  block=Block()
    exprs=Int(value=123)
```

**Namespaced fun in file**

```metall module
fun Foo.bar(f Foo) Int { 123 }
```

```ast
Module(fileName="test.met",name="test",main=true)
  decls=Fun(name="Foo.bar")
    params=FunParam(name="f")
      type=SimpleType(name="Foo")
    returnType=SimpleType(name="Int")
    block=Block()
      exprs=Int(value=123)
```

**Call with &ref arg**

```metall
{ fun foo(a &Int) void {} let x = 123 foo(&x) }
```

```ast
Block()
  exprs[0]=Fun(name="foo")
    params=FunParam(name="a")
      type=RefType(mut=false)
        type=SimpleType(name="Int")
    returnType=SimpleType(name="void")
    block=Block()
  exprs[1]=Var(name="x",mut=false)
    expr=Int(value=123)
  exprs[2]=Call()
    callee=Ident(name="foo")
    args=Ref(mut=false)
      target=Ident(name="x")
```

**Function literal**

```metall
let f = fun(a Int, b Int) Bool { true }
```

```ast
Var(name="f",mut=false)
  expr=Block()
    exprs[0]=Fun(name="__fun_lit_0")
      params[0]=FunParam(name="a")
        type=SimpleType(name="Int")
      params[1]=FunParam(name="b")
        type=SimpleType(name="Int")
      returnType=SimpleType(name="Bool")
      block=Block()
        exprs=Bool(value=true)
    exprs[1]=Ident(name="__fun_lit_0")
```

## Literals

**Bool true**

```metall
true
```

```ast
Bool(value=true)
```

**Bool false**

```metall
false
```

```ast
Bool(value=false)
```

**Rune literal**

```metall
'a'
```

```ast
RuneLiteral(value='a'(97))
```

**Rune literal unicode**

```metall
'é'
```

```ast
RuneLiteral(value='é'(233))
```

## If

**If then else**

```metall
if a { 42 } else { 123 }
```

```ast
If()
  cond=Ident(name="a")
  then=Block()
    exprs=Int(value=42)
  else=Block()
    exprs=Int(value=123)
```

## Structs

**Struct declaration**

```metall
struct Foo { one Str mut two Int }
```

```ast
Struct(name="Foo")
  fields[0]=StructField(name="one",mut=false)
    type=SimpleType(name="Str")
  fields[1]=StructField(name="two",mut=true)
    type=SimpleType(name="Int")
```

**Struct with allocator field**

```metall
struct Foo { @myalloc Arena }
```

```ast
Struct(name="Foo")
  fields=StructField(name="@myalloc",mut=false)
    type=SimpleType(name="Arena")
```

**Type construction**

```metall
Foo("hello", 123)
```

```ast
TypeConstruction()
  target=Ident(name="Foo")
  args[0]=String(value="hello")
  args[1]=Int(value=123)
```

**Type constructor U8**

Type constructors parse as type constructions — the type checker distinguishes them.

```metall
U8(42)
```

```ast
TypeConstruction()
  target=Ident(name="U8")
  args=Int(value=42)
```

**Type constructor Int**

```metall
Int(123)
```

```ast
TypeConstruction()
  target=Ident(name="Int")
  args=Int(value=123)
```

## Field Access

**Field read**

```metall
x.one
```

```ast
FieldAccess(field=one)
  target=Ident(name="x")
```

**Field write**

```metall
x.one = "hello"
```

```ast
Assign()
  lhs=FieldAccess(field=one)
    target=Ident(name="x")
  rhs=String(value="hello")
```

**Chained field access**

```metall
x.one.two
```

```ast
FieldAccess(field=two)
  target=FieldAccess(field=one)
    target=Ident(name="x")
```

**Call through field access**

```metall
x.one.two()
```

```ast
Call()
  callee=FieldAccess(field=two)
    target=FieldAccess(field=one)
      target=Ident(name="x")
```

## References And Derefs

**&ref**

```metall
&x
```

```ast
Ref(mut=false)
  target=Ident(name="x")
```

**& has lower precedence than field access**

```metall
&x.one.two
```

```ast
Ref(mut=false)
  target=FieldAccess(field=two)
    target=FieldAccess(field=one)
      target=Ident(name="x")
```

**&mut field access**

```metall
&mut x.one
```

```ast
Ref(mut=true)
  target=FieldAccess(field=one)
    target=Ident(name="x")
```

**& of index**

```metall
&x[1]
```

```ast
Ref(mut=false)
  target=Index()
    target=Ident(name="x")
    index=Int(value=1)
```

**& of deref**

```metall
&x.*
```

```ast
Ref(mut=false)
  target=Deref()
    expr=Ident(name="x")
```

**& of chained field and index**

```metall
&x.one[2].three
```

```ast
Ref(mut=false)
  target=FieldAccess(field=three)
    target=Index()
      target=FieldAccess(field=one)
        target=Ident(name="x")
      index=Int(value=2)
```

**&mut ref**

```metall
&mut x
```

```ast
Ref(mut=true)
  target=Ident(name="x")
```

**Deref**

```metall
x.*
```

```ast
Deref()
  expr=Ident(name="x")
```

**Nested deref**

```metall
x.*.*
```

```ast
Deref()
  expr=Deref()
    expr=Ident(name="x")
```

**Ref type**

```metall
fun foo() &Int {}
```

```ast
Fun(name="foo")
  returnType=RefType(mut=false)
    type=SimpleType(name="Int")
  block=Block()
```

**Nested ref type**

```metall
fun foo() &&Int {}
```

```ast
Fun(name="foo")
  returnType=RefType(mut=false)
    type=RefType(mut=false)
      type=SimpleType(name="Int")
  block=Block()
```

**Deref assign**

```metall
x.* = y
```

```ast
Assign()
  lhs=Deref()
    expr=Ident(name="x")
  rhs=Ident(name="y")
```

**Nested deref assign**

```metall
x.*.*.* = y
```

```ast
Assign()
  lhs=Deref()
    expr=Deref()
      expr=Deref()
        expr=Ident(name="x")
  rhs=Ident(name="y")
```

**Nested &ref should fail**

```metall
{ &&x }
```

```error
test.met:1:4: expected a place expression (variable, field, index, or deref)
    { &&x }
       ^^
```

**&ref of literal should fail**

```metall
{ &123 }
```

```error
test.met:1:4: expected a place expression (variable, field, index, or deref)
    { &123 }
       ^^^
```

## Allocators

**Allocator var**

```metall
let @myalloc = Arena(123)
```

```ast
AllocatorVar(name=@myalloc,allocator=Arena)
  args=Int(value=123)
```

**Heap alloc**

```metall
@myalloc.new<Foo>(Foo())
```

```ast
Call()
  callee=FieldAccess(field=new)
    target=Ident(name="@myalloc")
  args=TypeConstruction()
    target=Ident(name="Foo")
```

**Alloc fun param**

```metall
fun foo(@myalloc Arena, x Str, @youralloc Arena) void {}
```

```ast
Fun(name="foo")
  params[0]=FunParam(name="@myalloc")
    type=SimpleType(name="Arena")
  params[1]=FunParam(name="x")
    type=SimpleType(name="Str")
  params[2]=FunParam(name="@youralloc")
    type=SimpleType(name="Arena")
  returnType=SimpleType(name="void")
  block=Block()
```

**Pass alloc in call**

```metall
foo(@myalloc)
```

```ast
Call()
  callee=Ident(name="foo")
  args=Ident(name="@myalloc")
```

**Heap alloc from field**

```metall
x.@myalloc.new<Foo>(Foo("hello"))
```

```ast
Call()
  callee=FieldAccess(field=new)
    target=FieldAccess(field=@myalloc)
      target=Ident(name="x")
  args=TypeConstruction()
    target=Ident(name="Foo")
    args=String(value="hello")
```

**Heap alloc mut struct**

```metall
@myalloc.new_mut<Foo>(Foo())
```

```ast
Call()
  callee=FieldAccess(field=new_mut)
    target=Ident(name="@myalloc")
  args=TypeConstruction()
    target=Ident(name="Foo")
```

**Make slice**

```metall
@myalloc.make<[]Int>(n, 42)
```

```ast
Call()
  callee=FieldAccess(field=make)
    target=Ident(name="@myalloc")
  args[0]=Ident(name="n")
  args[1]=Int(value=42)
```

**Make uninit slice**

```metall
@myalloc.make_uninit<[]Int>(n)
```

```ast
Call()
  callee=FieldAccess(field=make_uninit)
    target=Ident(name="@myalloc")
  args=Ident(name="n")
```

**Mut allocator var should fail**

```metall
mut @a = Arena()
```

```error
test.met:1:5: allocator variables cannot be mutable
    mut @a = Arena()
        ^^
```

## Arrays And Slices

**Array type**

```metall
fun foo(a [5]Int) void {}
```

```ast
Fun(name="foo")
  params=FunParam(name="a")
    type=ArrayType(len=5)
      type=SimpleType(name="Int")
  returnType=SimpleType(name="void")
  block=Block()
```

**Multidimensional array type**

```metall
fun foo(a [3][4]Int) void {}
```

```ast
Fun(name="foo")
  params=FunParam(name="a")
    type=ArrayType(len=3)
      type=ArrayType(len=4)
        type=SimpleType(name="Int")
  returnType=SimpleType(name="void")
  block=Block()
```

**Multidimensional slice type**

```metall
fun foo(a [][]Str) void {}
```

```ast
Fun(name="foo")
  params=FunParam(name="a")
    type=SliceType(mut=false)
      type=SliceType(mut=false)
        type=SimpleType(name="Str")
  returnType=SimpleType(name="void")
  block=Block()
```

**Mixed array slice type**

```metall
fun foo(a [3][]Int) void {}
```

```ast
Fun(name="foo")
  params=FunParam(name="a")
    type=ArrayType(len=3)
      type=SliceType(mut=false)
        type=SimpleType(name="Int")
  returnType=SimpleType(name="void")
  block=Block()
```

**Mut slice type**

```metall
fun foo(a []mut Int) void {}
```

```ast
Fun(name="foo")
  params=FunParam(name="a")
    type=SliceType(mut=true)
      type=SimpleType(name="Int")
  returnType=SimpleType(name="void")
  block=Block()
```

**Array literal**

```metall
[1, 2, 3]
```

```ast
ArrayLiteral(len=3)
  first=Int(value=1)
```

**Empty slice**

```metall
[]
```

```ast
EmptySlice()
```

**Index read**

```metall
x[1]
```

```ast
Index()
  target=Ident(name="x")
  index=Int(value=1)
```

**Index write**

```metall
x[1] = 2
```

```ast
Assign()
  lhs=Index()
    target=Ident(name="x")
    index=Int(value=1)
  rhs=Int(value=2)
```

**Subslice lo..hi**

```metall
x[1..3]
```

```ast
SubSlice()
  target=Ident(name="x")
  range=Range(inclusive=false)
    lo=Int(value=1)
    hi=Int(value=3)
```

**Subslice lo..=hi**

```metall
x[1..=3]
```

```ast
SubSlice()
  target=Ident(name="x")
  range=Range(inclusive=true)
    lo=Int(value=1)
    hi=Int(value=3)
```

**Subslice ..hi**

```metall
x[..3]
```

```ast
SubSlice()
  target=Ident(name="x")
  range=Range(inclusive=false)
    hi=Int(value=3)
```

**Subslice lo..**

```metall
x[1..]
```

```ast
SubSlice()
  target=Ident(name="x")
  range=Range(inclusive=false)
    lo=Int(value=1)
```

**Subslice ..=hi**

```metall
x[..=3]
```

```ast
SubSlice()
  target=Ident(name="x")
  range=Range(inclusive=true)
    hi=Int(value=3)
```

**Subslice ..= without hi should fail**

```metall
x[..=]
```

```error
test.met:1:3: inclusive range (..=) requires an upper bound
    x[..=]
      ^^
```

**Subslice lo..= without hi should fail**

```metall
x[1..=]
```

```error
test.met:1:4: inclusive range (..=) requires an upper bound
    x[1..=]
       ^^
```

**Subslice missing ] should fail**

```metall
x[1..2
```

```error
test.met:1:6: unexpected end of file
    x[1..2
         ^
test.met:1:6: unexpected end of file
    x[1..2
         ^
```

## Operators

**int +**

```metall
1 + 2
```

```ast
Binary(op=+)
  lhs=Int(value=1)
  rhs=Int(value=2)
```

**int -**

```metall
1 - 2
```

```ast
Binary(op=-)
  lhs=Int(value=1)
  rhs=Int(value=2)
```

**int ***

```metall
1 * 2
```

```ast
Binary(op=*)
  lhs=Int(value=1)
  rhs=Int(value=2)
```

**int /**

```metall
1 / 2
```

```ast
Binary(op=/)
  lhs=Int(value=1)
  rhs=Int(value=2)
```

**int %**

```metall
1 % 2
```

```ast
Binary(op=%)
  lhs=Int(value=1)
  rhs=Int(value=2)
```

**int <**

```metall
1 < 2
```

```ast
Binary(op=<)
  lhs=Int(value=1)
  rhs=Int(value=2)
```

**int <=**

```metall
1 <= 2
```

```ast
Binary(op=<=)
  lhs=Int(value=1)
  rhs=Int(value=2)
```

**int >**

```metall
1 > 2
```

```ast
Binary(op=>)
  lhs=Int(value=1)
  rhs=Int(value=2)
```

**int >=**

```metall
1 >= 2
```

```ast
Binary(op=>=)
  lhs=Int(value=1)
  rhs=Int(value=2)
```

**Arithmetic operator precedence**

```metall
1 + 2 * 3 + 4
```

```ast
Binary(op=+)
  lhs=Binary(op=+)
    lhs=Int(value=1)
    rhs=Binary(op=*)
      lhs=Int(value=2)
      rhs=Int(value=3)
  rhs=Int(value=4)
```

**==**

```metall
1 == 2
```

```ast
Binary(op===)
  lhs=Int(value=1)
  rhs=Int(value=2)
```

**!=**

```metall
1 != 2
```

```ast
Binary(op=!=)
  lhs=Int(value=1)
  rhs=Int(value=2)
```

**Logical and, or, not**

```metall
true and false or not true
```

```ast
Binary(op=or)
  lhs=Binary(op=and)
    lhs=Bool(value=true)
    rhs=Bool(value=false)
  rhs=Unary(op=not)
    expr=Bool(value=true)
```

**And binds tighter than or**

```metall
true or false and true
```

```ast
Binary(op=or)
  lhs=Bool(value=true)
  rhs=Binary(op=and)
    lhs=Bool(value=false)
    rhs=Bool(value=true)
```

**Bitwise and**

```metall
1 & 2
```

```ast
Binary(op=&)
  lhs=Int(value=1)
  rhs=Int(value=2)
```

**Bitwise or**

```metall
1 | 2
```

```ast
Binary(op=|)
  lhs=Int(value=1)
  rhs=Int(value=2)
```

**Bitwise xor**

```metall
1 ^ 2
```

```ast
Binary(op=^)
  lhs=Int(value=1)
  rhs=Int(value=2)
```

**Shift left**

```metall
1 << 2
```

```ast
Binary(op=<<)
  lhs=Int(value=1)
  rhs=Int(value=2)
```

**Shift right**

```metall
1 >> 2
```

```ast
Binary(op=>>)
  lhs=Int(value=1)
  rhs=Int(value=2)
```

**Bitwise not**

```metall
~1
```

```ast
Unary(op=~)
  expr=Int(value=1)
```

**Bitwise precedence**

```metall
1 | 2 ^ 3 & 4
```

```ast
Binary(op=|)
  lhs=Int(value=1)
  rhs=Binary(op=^)
    lhs=Int(value=2)
    rhs=Binary(op=&)
      lhs=Int(value=3)
      rhs=Int(value=4)
```

**Shift precedence vs add**

```metall
1 + 2 << 3 + 4
```

```ast
Binary(op=<<)
  lhs=Binary(op=+)
    lhs=Int(value=1)
    rhs=Int(value=2)
  rhs=Binary(op=+)
    lhs=Int(value=3)
    rhs=Int(value=4)
```

**Grouped expressions**

```metall
(1 + 2) * 3 + 4
```

```ast
Binary(op=+)
  lhs=Binary(op=*)
    lhs=Binary(op=+)
      lhs=Int(value=1)
      rhs=Int(value=2)
    rhs=Int(value=3)
  rhs=Int(value=4)
```

## Loops

**Conditional for loop**

```metall
for true { 1 }
```

```ast
For()
  cond=Bool(value=true)
  body=Block()
    exprs=Int(value=1)
```

**Unconditional for loop**

```metall
for { 1 }
```

```ast
For()
  body=Block()
    exprs=Int(value=1)
```

**Break**

```metall
break
```

```ast
Break()
```

**Continue**

```metall
continue
```

```ast
Continue()
```

**For in range**

```metall
for x in 0..10 { 1 }
```

```ast
For(binding=x)
  cond=Range(inclusive=false)
    lo=Int(value=0)
    hi=Int(value=10)
  body=Block()
    exprs=Int(value=1)
```

**For in range inclusive**

```metall
for x in 0..=10 { 1 }
```

```ast
For(binding=x)
  cond=Range(inclusive=true)
    lo=Int(value=0)
    hi=Int(value=10)
  body=Block()
    exprs=Int(value=1)
```

**For in missing dotdot should fail**

```metall
for x in 0 { 1 }
```

```error
test.met:1:10: expected range expression (e.g. 0..10)
    for x in 0 { 1 }
             ^
```

**For in inclusive range without hi should fail**

```metall
for x in 0..= { 1 }
```

```error
test.met:1:11: inclusive range (..=) requires an upper bound
    for x in 0..= { 1 }
              ^^
```

## Generics

**Generic struct**

```metall
struct Foo<T> { value T }
```

```ast
Struct(name="Foo")
  typeParams=TypeParam(name="T")
  fields=StructField(name="value",mut=false)
    type=SimpleType(name="T")
```

**Generic struct two params**

```metall
struct Foo<A, B> { a A b B }
```

```ast
Struct(name="Foo")
  typeParams[0]=TypeParam(name="A")
  typeParams[1]=TypeParam(name="B")
  fields[0]=StructField(name="a",mut=false)
    type=SimpleType(name="A")
  fields[1]=StructField(name="b",mut=false)
    type=SimpleType(name="B")
```

**Generic fun**

```metall
fun foo<T>(x T) T { x }
```

```ast
Fun(name="foo")
  typeParams=TypeParam(name="T")
  params=FunParam(name="x")
    type=SimpleType(name="T")
  returnType=SimpleType(name="T")
  block=Block()
    exprs=Ident(name="x")
```

**Type arg in type**

```metall
fun foo(x Foo<Int>) void {}
```

```ast
Fun(name="foo")
  params=FunParam(name="x")
    type=SimpleType(name="Foo")
      typeArgs=SimpleType(name="Int")
  returnType=SimpleType(name="void")
  block=Block()
```

**Nested type args**

```metall
fun foo(x Foo<Bar<Int>, Baz<Str>>) void {}
```

```ast
Fun(name="foo")
  params=FunParam(name="x")
    type=SimpleType(name="Foo")
      typeArgs[0]=SimpleType(name="Bar")
        typeArgs=SimpleType(name="Int")
      typeArgs[1]=SimpleType(name="Baz")
        typeArgs=SimpleType(name="Str")
  returnType=SimpleType(name="void")
  block=Block()
```

**Void as type argument**

```metall
fun foo(x Result<void>) void {}
```

```ast
Fun(name="foo")
  params=FunParam(name="x")
    type=SimpleType(name="Result")
      typeArgs=SimpleType(name="void")
  returnType=SimpleType(name="void")
  block=Block()
```

**Void as type annotation**

```metall
let x void = void
```

```ast
Var(name="x",mut=false)
  type=SimpleType(name="void")
  expr=Ident(name="void")
```

**Type construction with type args**

```metall
Foo<Int>(42)
```

```ast
TypeConstruction()
  target=Ident(name="Foo")
    typeArgs=SimpleType(name="Int")
  args=Int(value=42)
```

**Type construction nested type args**

```metall
Foo<Bar<Int>>(42)
```

```ast
TypeConstruction()
  target=Ident(name="Foo")
    typeArgs=SimpleType(name="Bar")
      typeArgs=SimpleType(name="Int")
  args=Int(value=42)
```

**Call with type args**

```metall
foo<Int>(42)
```

```ast
Call()
  callee=Ident(name="foo")
    typeArgs=SimpleType(name="Int")
  args=Int(value=42)
```

**Call nested type args**

```metall
foo<Bar<Int>>(42)
```

```ast
Call()
  callee=Ident(name="foo")
    typeArgs=SimpleType(name="Bar")
      typeArgs=SimpleType(name="Int")
  args=Int(value=42)
```

**Constrained type param**

```metall
fun foo<T Showable>(t T) void { }
```

```ast
Fun(name="foo")
  typeParams=TypeParam(name="T")
    constraint=SimpleType(name="Showable")
  params=FunParam(name="t")
    type=SimpleType(name="T")
  returnType=SimpleType(name="void")
  block=Block()
```

**Nested type param in struct should fail**

```metall
struct Foo<T<A>> {}
```

```error
test.met:1:13: unexpected token: expected ,, got <<immediate>
    struct Foo<T<A>> {}
                ^
```

**Nested type param in fun should fail**

```metall
fun foo<T<A>>() void {}
```

```error
test.met:1:10: unexpected token: expected ,, got <<immediate>
    fun foo<T<A>>() void {}
             ^
```

**Empty type params in struct should fail**

```metall
struct Foo<> {}
```

```error
test.met:1:11: empty type parameter list
    struct Foo<> {}
              ^^
```

**Empty type params in fun should fail**

```metall
fun foo<>() void {}
```

```error
test.met:1:8: empty type parameter list
    fun foo<>() void {}
           ^^
```

**Empty type args in type construction should fail**

```metall
{ struct Foo<T> { value T } Foo<>(42) }
```

```error
test.met:1:32: empty type argument list
    { struct Foo<T> { value T } Foo<>(42) }
                                   ^^
```

**Empty type args in type position should fail**

```metall
struct Foo<T> { value Foo<> }
```

```error
test.met:1:26: empty type argument list
    struct Foo<T> { value Foo<> }
                             ^^
```

**Default type param**

```metall
struct Foo<T = Int> { value T }
```

```ast
Struct(name="Foo")
  typeParams=TypeParam(name="T")
    default=SimpleType(name="Int")
  fields=StructField(name="value",mut=false)
    type=SimpleType(name="T")
```

**Default type param with constraint**

```metall
fun foo<T Showable = Str>(t T) void { }
```

```ast
Fun(name="foo")
  typeParams=TypeParam(name="T")
    constraint=SimpleType(name="Showable")
    default=SimpleType(name="Str")
  params=FunParam(name="t")
    type=SimpleType(name="T")
  returnType=SimpleType(name="void")
  block=Block()
```

**Multiple type params with partial defaults**

```metall
struct Pair<A, B = Int> { a A b B }
```

```ast
Struct(name="Pair")
  typeParams[0]=TypeParam(name="A")
  typeParams[1]=TypeParam(name="B")
    default=SimpleType(name="Int")
  fields[0]=StructField(name="a",mut=false)
    type=SimpleType(name="A")
  fields[1]=StructField(name="b",mut=false)
    type=SimpleType(name="B")
```

**Default type param not rightmost should fail**

```metall
struct Foo<T = Int, U> { a T b U }
```

```error
test.met:1:21: type parameters with defaults must be last
    struct Foo<T = Int, U> { a T b U }
                        ^
```

**Option and Result type sugar**

```metall
fun foo(a ?Int, b !Int, c ?&Int, d !&mut Int, e ?[]Int, f !?Int) void { 1 }
```

```ast
Fun(name="foo")
  params[0]=FunParam(name="a")
    type=SimpleType(name="Option")
      typeArgs=SimpleType(name="Int")
  params[1]=FunParam(name="b")
    type=SimpleType(name="Result")
      typeArgs=SimpleType(name="Int")
  params[2]=FunParam(name="c")
    type=SimpleType(name="Option")
      typeArgs=RefType(mut=false)
        type=SimpleType(name="Int")
  params[3]=FunParam(name="d")
    type=SimpleType(name="Result")
      typeArgs=RefType(mut=true)
        type=SimpleType(name="Int")
  params[4]=FunParam(name="e")
    type=SimpleType(name="Option")
      typeArgs=SliceType(mut=false)
        type=SimpleType(name="Int")
  params[5]=FunParam(name="f")
    type=SimpleType(name="Result")
      typeArgs=SimpleType(name="Option")
        typeArgs=SimpleType(name="Int")
  returnType=SimpleType(name="void")
  block=Block()
    exprs=Int(value=1)
```

## Shapes

**Shape**

```metall
shape Foo { name Str }
```

```ast
Shape(name="Foo")
  fields=StructField(name="name",mut=false)
    type=SimpleType(name="Str")
```

**Shape with fun**

```metall
shape Foo { fun Foo.bar(f Foo) Str }
```

```ast
Shape(name="Foo")
  funs=FunDecl(name="Foo.bar")
    params=FunParam(name="f")
      type=SimpleType(name="Foo")
    returnType=SimpleType(name="Str")
```

**Shape with field and fun**

```metall
shape Foo { name Str fun Foo.bar(f Foo) Str }
```

```ast
Shape(name="Foo")
  fields=StructField(name="name",mut=false)
    type=SimpleType(name="Str")
  funs=FunDecl(name="Foo.bar")
    params=FunParam(name="f")
      type=SimpleType(name="Foo")
    returnType=SimpleType(name="Str")
```

## Unions

**Union**

```metall
union Foo = Str | Int
```

```ast
Union(name="Foo")
  variants[0]=SimpleType(name="Str")
  variants[1]=SimpleType(name="Int")
```

**Union three variants**

```metall
union Foo = Str | Int | Bool
```

```ast
Union(name="Foo")
  variants[0]=SimpleType(name="Str")
  variants[1]=SimpleType(name="Int")
  variants[2]=SimpleType(name="Bool")
```

**Generic union**

```metall
union Maybe<T> = T | None
```

```ast
Union(name="Maybe")
  typeParams=TypeParam(name="T")
  variants[0]=SimpleType(name="T")
  variants[1]=SimpleType(name="None")
```

**Generic union with type args**

```metall
union Foo<T> = Str | Bar<T> | Int
```

```ast
Union(name="Foo")
  typeParams=TypeParam(name="T")
  variants[0]=SimpleType(name="Str")
  variants[1]=SimpleType(name="Bar")
    typeArgs=SimpleType(name="T")
  variants[2]=SimpleType(name="Int")
```

**Union with ref variant**

```metall
union Foo = &Str | Int
```

```ast
Union(name="Foo")
  variants[0]=RefType(mut=false)
    type=SimpleType(name="Str")
  variants[1]=SimpleType(name="Int")
```

**Union with slice variant**

```metall
union Foo = []Int | Str
```

```ast
Union(name="Foo")
  variants[0]=SliceType(mut=false)
    type=SimpleType(name="Int")
  variants[1]=SimpleType(name="Str")
```

**Union single variant**

```metall
union Foo = Str
```

```error
test.met:1:13: union requires at least 2 variants
    union Foo = Str
                ^^^
```

**Union reserved word**

```metall
union Arena = Str | Int
```

```error
test.met:1:7: reserved word: Arena
    union Arena = Str | Int
          ^^^^^
```

## Match

**Match with else**

```metall
match x { case Int: 1 else: 2 }
```

```ast
Match(arms=1,arm[0].pattern=n2:SimpleType)
  expr=Ident(name="x")
  arm[0].body=Block()
    exprs=Int(value=1)
  else.body=Block()
    exprs=Int(value=2)
```

**Match with binding**

```metall
match x { case Int n: n case Str s: s }
```

```ast
Match(arms=2,arm[0].pattern=n2:SimpleType,arm[0].binding=n,arm[1].pattern=n5:SimpleType,arm[1].binding=s)
  expr=Ident(name="x")
  arm[0].body=Block()
    exprs=Ident(name="n")
  arm[1].body=Block()
    exprs=Ident(name="s")
```

**Match with guard**

```metall
match x { case Int n if n > 0: n case Int: 0 }
```

```ast
Match(arms=2,arm[0].pattern=n2:SimpleType,arm[0].binding=n,arm[1].pattern=n8:SimpleType)
  expr=Ident(name="x")
  arm[0].guard=Binary(op=>)
    lhs=Ident(name="n")
    rhs=Int(value=0)
  arm[0].body=Block()
    exprs=Ident(name="n")
  arm[1].body=Block()
    exprs=Int(value=0)
```

**Match else with guard**

```metall
match x { case Int: 1 else if true: 2 }
```

```error
test.met:1:31: else arm cannot have a guard condition
    match x { case Int: 1 else if true: 2 }
                                  ^^^^
```

## Imports And Paths

**Use simple path**

```metall module
use foo::bar
```

```ast
Module(fileName="test.met",name="test",main=true)
  imports=Import(path=foo::bar)
```

**Use deep path**

```metall module
use foo::bar::baz
```

```ast
Module(fileName="test.met",name="test",main=true)
  imports=Import(path=foo::bar::baz)
```

**Use with alias**

```metall module
use b = foo::bar
```

```ast
Module(fileName="test.met",name="test",main=true)
  imports=Import(alias="b",path=foo::bar)
```

**Use local import**

```metall module
use local::foo::bar
```

```ast
Module(fileName="test.met",name="test",main=true)
  imports=Import(path=local::foo::bar)
```

**Use local import with alias**

```metall module
use b = local::foo::bar
```

```ast
Module(fileName="test.met",name="test",main=true)
  imports=Import(alias="b",path=local::foo::bar)
```

**Path expression**

```metall
math::pow
```

```ast
Path(segments=math::pow)
```

**Path expression call**

```metall
math::pow(2, 5)
```

```ast
Call()
  callee=Path(segments=math::pow)
  args[0]=Int(value=2)
  args[1]=Int(value=5)
```

**Path expression with type ident**

```metall
lib::Point(1, 2)
```

```ast
TypeConstruction()
  target=Path(segments=lib::Point)
  args[0]=Int(value=1)
  args[1]=Int(value=2)
```

**Path type construction with type args**

```metall
lib::Foo<Int>(42)
```

```ast
TypeConstruction()
  target=Path(segments=lib::Foo)
    typeArg=SimpleType(name="Int")
  args=Int(value=42)
```

**Path type construction nested type args**

```metall
lib::Foo<Bar<Int>>(42)
```

```ast
TypeConstruction()
  target=Path(segments=lib::Foo)
    typeArg=SimpleType(name="Bar")
      typeArgs=SimpleType(name="Int")
  args=Int(value=42)
```

**Path call with type args**

```metall
lib::foo<Int>(42)
```

```ast
Call()
  callee=Path(segments=lib::foo)
    typeArg=SimpleType(name="Int")
  args=Int(value=42)
```

**Path in let binding**

```metall
let x = math::pow
```

```ast
Var(name="x",mut=false)
  expr=Path(segments=math::pow)
```

**Use in expression should fail**

```metall
use foo::bar
```

```error
test.met:1:1: unexpected token: expected start of an expression, got <use>
    use foo::bar
    ^^^
```

**Use after decl should fail**

```metall module
fun main() void {} use foo::bar
```

```error
test.met:1:20: unexpected token: <use>
    fun main() void {} use foo::bar
                       ^^^
```

**Match else not confused with if-else**

```metall
match x { case Int: if cond { 1 } else: 2 }
```

```ast
Match(arms=1,arm[0].pattern=n2:SimpleType)
  expr=Ident(name="x")
  arm[0].body=Block()
    exprs=If()
      cond=Ident(name="cond")
      then=Block()
        exprs=Int(value=1)
  else.body=Block()
    exprs=Int(value=2)
```

**Match else binding not confused with if-else**

```metall
match x { case Int: if cond { 1 } else v: 2 }
```

```ast
Match(arms=1,arm[0].pattern=n2:SimpleType,else.binding=v)
  expr=Ident(name="x")
  arm[0].body=Block()
    exprs=If()
      cond=Ident(name="cond")
      then=Block()
        exprs=Int(value=1)
  else.body=Block()
    exprs=Int(value=2)
```

## Try

**Try short form desugars to match**

```metall
try foo()
```

```ast
Match(arms=1,arm[0].pattern=n3:TryPattern,arm[0].binding=__try,else.binding=__try_e)
  expr=Call()
    callee=Ident(name="foo")
  arm[0].body=Block()
    exprs=Ident(name="__try")
  else.body=Block()
    exprs=Return()
      expr=Ident(name="__try_e")
```

**Try with is desugars to match**

```metall
try foo() is Int
```

```ast
Match(arms=1,arm[0].pattern=n3:SimpleType,arm[0].binding=__try,else.binding=__try_e)
  expr=Call()
    callee=Ident(name="foo")
  arm[0].body=Block()
    exprs=Ident(name="__try")
  else.body=Block()
    exprs=Return()
      expr=Ident(name="__try_e")
```

**Try with else desugars to match**

```metall
try foo() else e { return e }
```

```ast
Match(arms=1,arm[0].pattern=n3:TryPattern,arm[0].binding=__try,else.binding=e)
  expr=Call()
    callee=Ident(name="foo")
  arm[0].body=Block()
    exprs=Ident(name="__try")
  else.body=Block()
    exprs=Return()
      expr=Ident(name="e")
```

**Try with is and else desugars to match**

```metall
try foo() is Ok else e { return e }
```

```ast
Match(arms=1,arm[0].pattern=n3:SimpleType,arm[0].binding=__try,else.binding=e)
  expr=Call()
    callee=Ident(name="foo")
  arm[0].body=Block()
    exprs=Ident(name="__try")
  else.body=Block()
    exprs=Return()
      expr=Ident(name="e")
```

**Try inside match arm without else leaves else for match**

```metall
match x { case Int: try foo() else: 0 }
```

```ast
Match(arms=1,arm[0].pattern=n2:SimpleType)
  expr=Ident(name="x")
  arm[0].body=Block()
    exprs=Match(arms=1,arm[0].pattern=n5:TryPattern,arm[0].binding=__try,else.binding=__try_e)
      expr=Call()
        callee=Ident(name="foo")
      arm[0].body=Block()
        exprs=Ident(name="__try")
      else.body=Block()
        exprs=Return()
          expr=Ident(name="__try_e")
  else.body=Block()
    exprs=Int(value=0)
```

**Try inside match arm with else block**

```metall
match x { case Int: try foo() else { 0 } else: 0 }
```

```ast
Match(arms=1,arm[0].pattern=n2:SimpleType)
  expr=Ident(name="x")
  arm[0].body=Block()
    exprs=Match(arms=1,arm[0].pattern=n5:TryPattern,arm[0].binding=__try)
      expr=Call()
        callee=Ident(name="foo")
      arm[0].body=Block()
        exprs=Ident(name="__try")
      else.body=Block()
        exprs=Int(value=0)
  else.body=Block()
    exprs=Int(value=0)
```

**Try inside match arm with else binding and block**

```metall
match x { case Int: try foo() else e { e } else: 0 }
```

```ast
Match(arms=1,arm[0].pattern=n2:SimpleType)
  expr=Ident(name="x")
  arm[0].body=Block()
    exprs=Match(arms=1,arm[0].pattern=n5:TryPattern,arm[0].binding=__try,else.binding=e)
      expr=Call()
        callee=Ident(name="foo")
      arm[0].body=Block()
        exprs=Ident(name="__try")
      else.body=Block()
        exprs=Ident(name="e")
  else.body=Block()
    exprs=Int(value=0)
```

**Try inside match arm with else binding leaves match else binding**

```metall
match x { case Int: try foo() else y: y }
```

```ast
Match(arms=1,arm[0].pattern=n2:SimpleType,else.binding=y)
  expr=Ident(name="x")
  arm[0].body=Block()
    exprs=Match(arms=1,arm[0].pattern=n5:TryPattern,arm[0].binding=__try,else.binding=__try_e)
      expr=Call()
        callee=Ident(name="foo")
      arm[0].body=Block()
        exprs=Ident(name="__try")
      else.body=Block()
        exprs=Return()
          expr=Ident(name="__try_e")
  else.body=Block()
    exprs=Ident(name="y")
```

## Error Recovery

**Unexpected token**

```metall
=
```

```error
test.met:1:1: unexpected token: expected start of an expression, got =
    =
    ^
```

**Return expects expr**

```metall
{ return }
```

```error
test.met:1:10: unexpected token: expected start of an expression, got }
    { return }
             ^
```

**Reserved word Arena**

```metall
struct Arena{one Str}
```

```error
test.met:1:8: reserved word: Arena
    struct Arena{one Str}
           ^^^^^
```

**Reserved word panic (fun)**

```metall
fun panic() void {}
```

```error
test.met:1:5: reserved word: panic
    fun panic() void {}
        ^^^^^
```

**Reserved word panic (var)**

```metall
let panic = 123
```

```error
test.met:1:5: reserved word: panic
    let panic = 123
        ^^^^^
```

**Method fun missing method name**

```metall
fun Foo.() void {}
```

```error
test.met:1:9: unexpected token: expected <identifier>, got (
    fun Foo.() void {}
            ^
```
