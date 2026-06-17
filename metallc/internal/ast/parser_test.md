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

**Compound assign**

```metall
x += 5
```

```ast
Assign(op=+)
  lhs=Ident(name="x")
  rhs=Int(value=5)
```

**Compound assign wrapping**

```metall
x +%= 5
```

```ast
Assign(op=+%)
  lhs=Ident(name="x")
  rhs=Int(value=5)
```

**Let binding**

```metall
let x = 123
```

```ast
Var(name="x")
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
Var(name="x")
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

**Named call args**

```metall
foo(1, b = 2, a = 3)
```

```ast
Call(args[1].name="b",args[2].name="a")
  callee=Ident(name="foo")
  args[0]=Int(value=1)
  args[1]=Int(value=2)
  args[2]=Int(value=3)
```

**Named type construction**

```metall
Foo(y = 2, x = 1)
```

```ast
TypeConstruction(args[0].name="y",args[1].name="x")
  target=Ident(name="Foo")
  args[0]=Int(value=2)
  args[1]=Int(value=1)
```

**Positional argument after named argument is rejected**

```metall
foo(a = 1, 2)
```

```error
test.met:1:12: positional argument after named argument
    foo(a = 1, 2)
               ^
```

**Fun type**

```metall
fun foo(bar fun(Str, Int) Bool) void {}
```

```ast
Fun(name="foo")
  params=FunParam(name="bar")
    type=FunType()
      paramTypes[0]=SimpleType(name="Str")
      paramTypes[1]=SimpleType(name="Int")
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
      paramTypes=SimpleType(name="Int")
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

**Namespaced fun with keyword method name**

```metall
fun Foo.match(f Foo) Int { 123 }
```

```ast
Fun(name="Foo.match")
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
      type=RefType()
        type=SimpleType(name="Int")
    returnType=SimpleType(name="void")
    block=Block()
  exprs[1]=Var(name="x")
    expr=Int(value=123)
  exprs[2]=Call()
    callee=Ident(name="foo")
    args=Ref()
      target=Ident(name="x")
```

**Function literal**

```metall
let f = fun(a Int, b Int) Bool { true }
```

```ast
Var(name="f")
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

**Closure with value capture**

```metall
fun[x](a Int) Int { x }
```

```ast
Block()
  exprs[0]=Fun(name="__fun_lit_0")
    captures=Capture(name="x")
    params=FunParam(name="a")
      type=SimpleType(name="Int")
    returnType=SimpleType(name="Int")
    block=Block()
      exprs=Ident(name="x")
  exprs[1]=Ident(name="__fun_lit_0")
```

**Closure with ref and mut ref captures**

```metall
fun[a, &b, &mut c]() void {}
```

```ast
Block()
  exprs[0]=Fun(name="__fun_lit_0")
    captures[0]=Capture(name="a")
    captures[1]=Capture(name="b",mode=ref)
    captures[2]=Capture(name="c",mode=mut_ref)
    returnType=SimpleType(name="void")
    block=Block()
  exprs[1]=Ident(name="__fun_lit_0")
```

**Function literal with inferred param types**

```metall
let f = fun(a, b) { true }
```

```ast
Var(name="f")
  expr=Block()
    exprs[0]=Fun(name="__fun_lit_0",returnType=<inferred>)
      params[0]=FunParam(name="a",type=<inferred>)
      params[1]=FunParam(name="b",type=<inferred>)
      block=Block()
        exprs=Bool(value=true)
    exprs[1]=Ident(name="__fun_lit_0")
```

**Function literal with inferred return type only**

```metall
let f = fun(a Int) { a }
```

```ast
Var(name="f")
  expr=Block()
    exprs[0]=Fun(name="__fun_lit_0",returnType=<inferred>)
      params=FunParam(name="a")
        type=SimpleType(name="Int")
      block=Block()
        exprs=Ident(name="a")
    exprs[1]=Ident(name="__fun_lit_0")
```

**Function literal with mixed param types (some inferred, some explicit)**

```metall
let f = fun(a, b Int) { b }
```

```ast
Var(name="f")
  expr=Block()
    exprs[0]=Fun(name="__fun_lit_0",returnType=<inferred>)
      params[0]=FunParam(name="a",type=<inferred>)
      params[1]=FunParam(name="b")
        type=SimpleType(name="Int")
      block=Block()
        exprs=Ident(name="b")
    exprs[1]=Ident(name="__fun_lit_0")
```

**Fun with default param**

```metall
fun foo(a Int, b Int = 3) void {}
```

```ast
Fun(name="foo")
  params[0]=FunParam(name="a")
    type=SimpleType(name="Int")
  params[1]=FunParam(name="b")
    type=SimpleType(name="Int")
    default=Int(value=3)
  returnType=SimpleType(name="void")
  block=Block()
```

**Default param must be last**

```metall
fun foo(a Int = 3, b Int) void {}
```

```error
test.met:1:20: parameters with default values must be last
    fun foo(a Int = 3, b Int) void {}
                       ^
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

**Negative integer literal**

```metall
-123
```

```ast
Int(value=-123)
```

**Float literal**

```metall
3.14
```

```ast
Float(value=3.14)
```

**Float literal with exponent**

```metall
1.5e-3
```

```ast
Float(value=0.0015)
```

**Unary minus on a float literal**

```metall
-2.5
```

```ast
Unary(op=-)
  expr=Float(value=2.5)
```

**Negative integer literal in expression**

```metall
x = -42
```

```ast
Assign()
  lhs=Ident(name="x")
  rhs=Int(value=-42)
```

**Subtraction should not be parsed as negative literal**

```metall
a - 123
```

```ast
Binary(op=-)
  lhs=Ident(name="a")
  rhs=Int(value=123)
```

**Hex literal**

```metall
0xFF
```

```ast
Int(value=255)
```

**Hex literal with underscores**

```metall
0xDEAD_BEEF
```

```ast
Int(value=3735928559)
```

**Negative hex literal**

```metall
-0xff
```

```ast
Int(value=-255)
```

**Octal literal**

```metall
0o755
```

```ast
Int(value=493)
```

**Binary literal**

```metall
0b1010_1010
```

```ast
Int(value=170)
```

**Decimal underscore literal**

```metall
1_000_000
```

```ast
Int(value=1000000)
```

## Strings

**String literal**

```metall
"hello"
```

```ast
String(value="hello")
```

**Empty string**

```metall
""
```

```ast
String(value="")
```

**String escapes**

```metall
"a\nb\tc\rd\0e"
```

```ast
String(value="a\nb\tc\rd\x00e")
```

**String escaped quote and backslash**

```metall
"q=\" p=\\"
```

```ast
String(value="q=\" p=\\")
```

**String hex and unicode escapes**

```metall
"\x41\u{20AC}\u{1F600}"
```

```ast
String(value="A€😀")
```

**String hex byte is encoded as UTF-8**

```metall
"\xc0"
```

```ast
String(value="À")
```

**String unknown escape**

```metall
"\q"
```

```error
test.met:1:1: unknown escape sequence '\q'
    "\q"
    ^^^^
```

**String invalid hex escape**

```metall
"\xZZ"
```

```error
test.met:1:1: invalid byte escape sequence
    "\xZZ"
    ^^^^^^
```

**String invalid unicode escape**

```metall
"\u{XY}"
```

```error
test.met:1:1: invalid unicode escape sequence
    "\u{XY}"
    ^^^^^^^^
```

**Single-line string rejects a raw newline**

```metall
"line one
line two"
```

```error
test.met:1:1: newline in single-line string; use a multi-line string
    "line one
    ^
    line two"
            ^
```

**Unterminated string literal**

```metall
"abc
```

```error
test.met:1:1: unterminated string literal
    "abc
    ^^^^
```

**String line continuation**

```metall
"a \
  b"
```

```ast
String(value="a b")
```

**String line continuation drops the next line's indentation**

```metall
"a\
      b"
```

```ast
String(value="ab")
```

**String line continuation ignores whitespace after the backslash**

```metall
"a \   
b"
```

```ast
String(value="a b")
```

**String multiple line continuations**

```metall
"a\
b\
c"
```

```ast
String(value="abc")
```

**String line continuation at the start**

```metall
"\
  x"
```

```ast
String(value="x")
```

**Backslash then whitespace then non-newline is rejected**

```metall
"a\ b"
```

```error
test.met:1:1: expected a newline after a line-continuation backslash
    "a\ b"
    ^^^^^^
```

**Bytes literal**

```metall
b"abc"
```

```ast
String(value="abc",bytes=true)
```

**Bytes literal with escapes**

```metall
b"\n\t\xFF"
```

```ast
String(value="\n\t\xff",bytes=true)
```

**Bytes literal keeps a raw hex byte**

```metall
b"\xc0"
```

```ast
String(value="\xc0",bytes=true)
```

**Bytes line continuation**

```metall
b"x\
  y"
```

```ast
String(value="xy",bytes=true)
```

**Bytes multi-line**

```metall
bm"
  AB
  "
```

```ast
String(value="AB",bytes=true)
```

**String with sigils carries bare quotes**

```metall
#"he said "hi""#
```

```ast
String(value="he said \"hi\"")
```

**String with triple sigils**

```metall
###"x"###
```

```ast
String(value="x")
```

**Bytes literal with sigils**

```metall
b###"raw"###
```

```ast
String(value="raw",bytes=true)
```

**Escapes still apply inside sigil strings**

```metall
#"a\nb"#
```

```ast
String(value="a\nb")
```

**Sigil count mismatch is unterminated**

```metall
#"abc"
```

```error
test.met:1:1: unterminated string literal
    #"abc"
    ^^^^^^
```

**Unknown string modifier**

```metall
boo"x"
```

```error
test.met:1:1: unknown string modifier "o"
    boo"x"
    ^^^^^^
```

**Multi-line bytes (modifiers in f, b, m order)**

```metall
bm"
  AB
  "
```

```ast
String(value="AB",bytes=true)
```

**String modifiers out of order**

```metall
mb"x"
```

```error
test.met:1:1: string modifiers must be written in the order f, b, m, each at most once
    mb"x"
    ^^^^^
```

**Multi-line string is dedented**

```metall
m"
    hello
    world
    "
```

```ast
String(value="hello\nworld")
```

**Empty multi-line string**

```metall
m"
"
```

```ast
String(value="")
```

**Multi-line string ignores the closing quote indentation when it is shallower**

```metall
m"
    a
    b
"
```

```ast
String(value="a\nb")
```

**Multi-line string ignores the closing quote indentation when it is deeper**

```metall
m"
  a
  b
      "
```

```ast
String(value="a\nb")
```

**Multi-line string dedents by the least-indented line**

```metall
m"
    a
      b
    "
```

```ast
String(value="a\n  b")
```

**Multi-line string keeps relative indentation**

```metall
m"
    outer
        inner
    "
```

```ast
String(value="outer\n    inner")
```

**Multi-line string single line**

```metall
m"
    only
    "
```

```ast
String(value="only")
```

**Multi-line string keeps blank lines**

```metall
m"
    a

    b
    "
```

```ast
String(value="a\n\nb")
```

**Multi-line string treats a whitespace-only line as blank**

```metall
m"
    a
      
    b
    "
```

```ast
String(value="a\n\nb")
```

**Multi-line string keeps trailing whitespace on a line**

```metall
m"
    a   
    "
```

```ast
String(value="a   ")
```

**Multi-line string ignores whitespace after the opening quote**

```metall
m"   
    x
    "
```

```ast
String(value="x")
```

**Multi-line string with tab indentation**

```metall
m"
		a
		b
		"
```

```ast
String(value="a\nb")
```

**Multi-line string with a line continuation**

```metall
m"
    a \
    b
    "
```

```ast
String(value="a b")
```

**Multi-line string opening quote must be followed by a newline**

```metall
m"oops
    "
```

```error
test.met:1:1: multi-line string: the opening quote must be followed by a newline
    m"oops
    ^
        "
        ^
```

**Multi-line string closing quote must be on its own line**

```metall
m"
    text"
```

```error
test.met:1:1: multi-line string: the closing quote must be on its own line
    m"
    ^
        text"
            ^
```

## Format Strings

An f-string desugars at parse time into a block that builds the result with a
StrWriter, so the rest of the compiler sees only ordinary calls.

**Simple interpolation**

```metall
f"hi {x}".build(@a)
```

```ast
Block()
  exprs[0]=Var(name="$fstr")
    expr=Call()
      callee=Ident(name="StrWriter.new")
      args[0]=Int(value=35)
      args[1]=Ident(name="@a")
  exprs[1]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=String(value="hi ")
  exprs[2]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=Ident(name="x")
  exprs[3]=Call()
    callee=FieldAccess(field=as_str)
      target=Ident(name="$fstr")
```

**Several interpolations and literals**

```metall
f"{a} and {b}!".build(@a)
```

```ast
Block()
  exprs[0]=Var(name="$fstr")
    expr=Call()
      callee=Ident(name="StrWriter.new")
      args[0]=Int(value=54)
      args[1]=Ident(name="@a")
  exprs[1]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=Ident(name="a")
  exprs[2]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=String(value=" and ")
  exprs[3]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=Ident(name="b")
  exprs[4]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=String(value="!")
  exprs[5]=Call()
    callee=FieldAccess(field=as_str)
      target=Ident(name="$fstr")
```

**No interpolation**

```metall
f"plain".build(@a)
```

```ast
Block()
  exprs[0]=Var(name="$fstr")
    expr=Call()
      callee=Ident(name="StrWriter.new")
      args[0]=Int(value=21)
      args[1]=Ident(name="@a")
  exprs[1]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=String(value="plain")
  exprs[2]=Call()
    callee=FieldAccess(field=as_str)
      target=Ident(name="$fstr")
```

**Empty format string**

```metall
f"".build(@a)
```

```ast
Block()
  exprs[0]=Var(name="$fstr")
    expr=Call()
      callee=Ident(name="StrWriter.new")
      args[0]=Int(value=16)
      args[1]=Ident(name="@a")
  exprs[1]=Call()
    callee=FieldAccess(field=as_str)
      target=Ident(name="$fstr")
```

**An interpolation may hold any expression**

```metall
f"{a + b * 2}".build(@a)
```

```ast
Block()
  exprs[0]=Var(name="$fstr")
    expr=Call()
      callee=Ident(name="StrWriter.new")
      args[0]=Int(value=32)
      args[1]=Ident(name="@a")
  exprs[1]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=Binary(op=+)
      lhs=Ident(name="a")
      rhs=Binary(op=*)
        lhs=Ident(name="b")
        rhs=Int(value=2)
  exprs[2]=Call()
    callee=FieldAccess(field=as_str)
      target=Ident(name="$fstr")
```

**An interpolation may hold a method call**

```metall
f"{p.dist(q)}".build(@a)
```

```ast
Block()
  exprs[0]=Var(name="$fstr")
    expr=Call()
      callee=Ident(name="StrWriter.new")
      args[0]=Int(value=32)
      args[1]=Ident(name="@a")
  exprs[1]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=Call()
      callee=FieldAccess(field=dist)
        target=Ident(name="p")
      args=Ident(name="q")
  exprs[2]=Call()
    callee=FieldAccess(field=as_str)
      target=Ident(name="$fstr")
```

**An interpolation may hold a string literal, even one containing braces**

```metall
f"{join(parts, "}")}".build(@a)
```

```ast
Block()
  exprs[0]=Var(name="$fstr")
    expr=Call()
      callee=Ident(name="StrWriter.new")
      args[0]=Int(value=32)
      args[1]=Ident(name="@a")
  exprs[1]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=Call()
      callee=Ident(name="join")
      args[0]=Ident(name="parts")
      args[1]=String(value="}")
  exprs[2]=Call()
    callee=FieldAccess(field=as_str)
      target=Ident(name="$fstr")
```

**An interpolation may hold an if-expression with braced branches**

```metall
f#"status: #{if ok { "up" } else { "down" }}"#.build(@a)
```

```ast
Block()
  exprs[0]=Var(name="$fstr")
    expr=Call()
      callee=Ident(name="StrWriter.new")
      args[0]=Int(value=40)
      args[1]=Ident(name="@a")
  exprs[1]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=String(value="status: ")
  exprs[2]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=If()
      cond=Ident(name="ok")
      then=Block()
        exprs=String(value="up")
      else=Block()
        exprs=String(value="down")
  exprs[3]=Call()
    callee=FieldAccess(field=as_str)
      target=Ident(name="$fstr")
```

**Sigils carry literal braces**

```metall
f#"a #{x} {b}"#.build(@a)
```

```ast
Block()
  exprs[0]=Var(name="$fstr")
    expr=Call()
      callee=Ident(name="StrWriter.new")
      args[0]=Int(value=38)
      args[1]=Ident(name="@a")
  exprs[1]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=String(value="a ")
  exprs[2]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=Ident(name="x")
  exprs[3]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=String(value=" {b}")
  exprs[4]=Call()
    callee=FieldAccess(field=as_str)
      target=Ident(name="$fstr")
```

**Sigils with several hashes**

```metall
f##"hi {literal braces}  ##{x}"##.build(@a)
```

```ast
Block()
  exprs[0]=Var(name="$fstr")
    expr=Call()
      callee=Ident(name="StrWriter.new")
      args[0]=Int(value=53)
      args[1]=Ident(name="@a")
  exprs[1]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=String(value="hi {literal braces}  ")
  exprs[2]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=Ident(name="x")
  exprs[3]=Call()
    callee=FieldAccess(field=as_str)
      target=Ident(name="$fstr")
```

**Bytes format string**

```metall
fb"x{n}".build(@a)
```

```ast
Block()
  exprs[0]=Var(name="$fstr")
    expr=Call()
      callee=Ident(name="StrWriter.new")
      args[0]=Int(value=33)
      args[1]=Ident(name="@a")
  exprs[1]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=String(value="x")
  exprs[2]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=Ident(name="n")
  exprs[3]=Call()
    callee=FieldAccess(field=as_bytes)
      target=Call()
        callee=FieldAccess(field=as_str)
          target=Ident(name="$fstr")
```

**Multi-line format string**

```metall
fm"
    Hello {name}
    bye
    ".build(@a)
```

```ast
Block()
  exprs[0]=Var(name="$fstr")
    expr=Call()
      callee=Ident(name="StrWriter.new")
      args[0]=Int(value=42)
      args[1]=Ident(name="@a")
  exprs[1]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=String(value="Hello ")
  exprs[2]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=Ident(name="name")
  exprs[3]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=String(value="\nbye")
  exprs[4]=Call()
    callee=FieldAccess(field=as_str)
      target=Ident(name="$fstr")
```

**Write directly into a StrWriter with .write_to**

```metall
f"v={x}".write_to(sw)
```

```ast
Block()
  exprs[0]=Var(name="$fstr")
    expr=Ident(name="sw")
  exprs[1]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=String(value="v=")
  exprs[2]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=Ident(name="x")
```

**The .write_to target is bound once, never re-evaluated per segment**

```metall
f"{a}{b}".write_to(make())
```

```ast
Block()
  exprs[0]=Var(name="$fstr")
    expr=Call()
      callee=Ident(name="make")
  exprs[1]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=Ident(name="a")
  exprs[2]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=Ident(name="b")
```

**A bytes format string cannot use .write_to**

```metall
fb"{x}".write_to(sw)
```

```error
test.met:1:9: a bytes format string (b) cannot be used with .write_to; use .build(@a)
    fb"{x}".write_to(sw)
            ^^^^^^^^
```

**Format string requires a .build or .write_to suffix**

```metall
f"{x}"
```

```error
test.met:1:1: a format string must be followed by .build(@a) or .write_to(sw)
    f"{x}"
    ^^
```

**Empty interpolation is rejected**

```metall
f"{}".build(@a)
```

```error
test.met:1:3: empty interpolation in format string
    f"{}".build(@a)
      ^
```

**Unterminated interpolation is rejected**

```metall
f"{x".build(@a)
```

```error
test.met:1:4: unterminated string literal
    f"{x".build(@a)
       ^^^^^^^^^^^^
```

**Unmatched closing brace is rejected**

```metall
f"a}".build(@a)
```

```error
test.met:1:4: unmatched '}' in format string (use f#"..."# for literal braces)
    f"a}".build(@a)
       ^
```

**A malformed interpolation expression is rejected**

```metall
f"{a +}".build(@a)
```

```error
test.met:1:7: unexpected token: expected start of an expression, got <f-string interpolation end>
    f"{a +}".build(@a)
          ^
```

**Escapes are decoded in literal text**

```metall
f"a\tb={x}\nz".build(@a)
```

```ast
Block()
  exprs[0]=Var(name="$fstr")
    expr=Call()
      callee=Ident(name="StrWriter.new")
      args[0]=Int(value=40)
      args[1]=Ident(name="@a")
  exprs[1]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=String(value="a\tb=")
  exprs[2]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=Ident(name="x")
  exprs[3]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=String(value="\nz")
  exprs[4]=Call()
    callee=FieldAccess(field=as_str)
      target=Ident(name="$fstr")
```

**Line continuation works in literal text**

```metall
f"a \
  b {x}".build(@a)
```

```ast
Block()
  exprs[0]=Var(name="$fstr")
    expr=Call()
      callee=Ident(name="StrWriter.new")
      args[0]=Int(value=40)
      args[1]=Ident(name="@a")
  exprs[1]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=String(value="a b ")
  exprs[2]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=Ident(name="x")
  exprs[3]=Call()
    callee=FieldAccess(field=as_str)
      target=Ident(name="$fstr")
```

**Adjacent interpolations need no literal between them**

```metall
f"{a}{b}".build(@a)
```

```ast
Block()
  exprs[0]=Var(name="$fstr")
    expr=Call()
      callee=Ident(name="StrWriter.new")
      args[0]=Int(value=48)
      args[1]=Ident(name="@a")
  exprs[1]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=Ident(name="a")
  exprs[2]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=Ident(name="b")
  exprs[3]=Call()
    callee=FieldAccess(field=as_str)
      target=Ident(name="$fstr")
```

**An interpolation may span lines**

```metall
f"{
    a + b
}".build(@a)
```

```ast
Block()
  exprs[0]=Var(name="$fstr")
    expr=Call()
      callee=Ident(name="StrWriter.new")
      args[0]=Int(value=32)
      args[1]=Ident(name="@a")
  exprs[1]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=Binary(op=+)
      lhs=Ident(name="a")
      rhs=Ident(name="b")
  exprs[2]=Call()
    callee=FieldAccess(field=as_str)
      target=Ident(name="$fstr")
```

**A lone sigil character stays literal**

```metall
f#"a # b #{x}"#.build(@a)
```

```ast
Block()
  exprs[0]=Var(name="$fstr")
    expr=Call()
      callee=Ident(name="StrWriter.new")
      args[0]=Int(value=38)
      args[1]=Ident(name="@a")
  exprs[1]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=String(value="a # b ")
  exprs[2]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=Ident(name="x")
  exprs[3]=Call()
    callee=FieldAccess(field=as_str)
      target=Ident(name="$fstr")
```

**A bytes format string keeps a raw byte in its literal**

```metall
fb"\xff{x}".build(@a)
```

```ast
Block()
  exprs[0]=Var(name="$fstr")
    expr=Call()
      callee=Ident(name="StrWriter.new")
      args[0]=Int(value=36)
      args[1]=Ident(name="@a")
  exprs[1]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=String(value="\xff")
  exprs[2]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=Ident(name="x")
  exprs[3]=Call()
    callee=FieldAccess(field=as_bytes)
      target=Call()
        callee=FieldAccess(field=as_str)
          target=Ident(name="$fstr")
```

**A raw newline in a single-line literal is rejected**

```metall
f"a
b {x}".build(@a)
```

```error
test.met:1:1: newline in single-line string; use a multi-line string
    f"a
    ^^
    b {x}".build(@a)
```

**A format specifier dispatches to .fmt_ext with the parsed arguments**

A `:` after an interpolation's expression introduces a format specifier. Its
arguments are parsed like call arguments and handed to `.fmt_ext` with the StrWriter
first, so a value with a specifier formats through `fmt_ext` instead of the default
`fmt`.

```metall
f"age {age:width=20, base=16, upper=true}".build(@a)
```

```ast
Block()
  exprs[0]=Var(name="$fstr")
    expr=Call()
      callee=Ident(name="StrWriter.new")
      args[0]=Int(value=36)
      args[1]=Ident(name="@a")
  exprs[1]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=String(value="age ")
  exprs[2]=Call(args[1].name="width",args[2].name="base",args[3].name="upper")
    callee=FieldAccess(field=fmt_ext)
      target=Ident(name="age")
    args[0]=Ident(name="$fstr")
    args[1]=Int(value=20)
    args[2]=Int(value=16)
    args[3]=Bool(value=true)
  exprs[3]=Call()
    callee=FieldAccess(field=as_str)
      target=Ident(name="$fstr")
```

**A format specifier may be positional**

```metall
f"{n:16}".build(@a)
```

```ast
Block()
  exprs[0]=Var(name="$fstr")
    expr=Call()
      callee=Ident(name="StrWriter.new")
      args[0]=Int(value=32)
      args[1]=Ident(name="@a")
  exprs[1]=Call()
    callee=FieldAccess(field=fmt_ext)
      target=Ident(name="n")
    args[0]=Ident(name="$fstr")
    args[1]=Int(value=16)
  exprs[2]=Call()
    callee=FieldAccess(field=as_str)
      target=Ident(name="$fstr")
```

**A colon at interpolation depth zero is the specifier, even past a nested when**

The only other `:` in the grammar is the when/match case delimiter, which is always
nested inside its own braces, so the specifier colon is found at depth zero after the
whole expression.

```metall
f"{when { case ok: 1 else: 0 }:width=4}".build(@a)
```

```ast
Block()
  exprs[0]=Var(name="$fstr")
    expr=Call()
      callee=Ident(name="StrWriter.new")
      args[0]=Int(value=32)
      args[1]=Ident(name="@a")
  exprs[1]=Call(args[1].name="width")
    callee=FieldAccess(field=fmt_ext)
      target=When(cases=1)
        case[0].cond=Ident(name="ok")
        case[0].body=Block()
          exprs=Int(value=1)
        else=Block()
          exprs=Int(value=0)
    args[0]=Ident(name="$fstr")
    args[1]=Int(value=4)
  exprs[2]=Call()
    callee=FieldAccess(field=as_str)
      target=Ident(name="$fstr")
```

**A colon in literal text is not a specifier**

```metall
f"t 12:30 {x}".build(@a)
```

```ast
Block()
  exprs[0]=Var(name="$fstr")
    expr=Call()
      callee=Ident(name="StrWriter.new")
      args[0]=Int(value=40)
      args[1]=Ident(name="@a")
  exprs[1]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=String(value="t 12:30 ")
  exprs[2]=Call()
    callee=FieldAccess(field=write)
      target=Ident(name="$fstr")
    args=Ident(name="x")
  exprs[3]=Call()
    callee=FieldAccess(field=as_str)
      target=Ident(name="$fstr")
```

**An empty format specifier is rejected**

```metall
f"{x:}".build(@a)
```

```error
test.met:1:5: empty format specifier
    f"{x:}".build(@a)
        ^
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

**When expression**

```metall
when {
    case a: 1
    case b: 2
    else: 3
}
```

```ast
When(cases=2)
  case[0].cond=Ident(name="a")
  case[0].body=Block()
    exprs=Int(value=1)
  case[1].cond=Ident(name="b")
  case[1].body=Block()
    exprs=Int(value=2)
  else=Block()
    exprs=Int(value=3)
```

## Structs

**Struct declaration**

```metall
struct Foo { one Str pub two Int }
```

```ast
Struct(name="Foo")
  fields[0]=StructField(name="one")
    type=SimpleType(name="Str")
  fields[1]=StructField(name="two",pub=true)
    type=SimpleType(name="Int")
```

**Struct with allocator field**

```metall
struct Foo { @myalloc Arena }
```

```ast
Struct(name="Foo")
  fields=StructField(name="@myalloc")
    type=SimpleType(name="Arena")
```

**Pub struct with pub fields**

```metall module
pub struct Foo { pub one Str pub two Int three Bool }
```

```ast
Module(fileName="test.met",name="test",main=true)
  decls=Struct(name="Foo",pub=true)
    fields[0]=StructField(name="one",pub=true)
      type=SimpleType(name="Str")
    fields[1]=StructField(name="two",pub=true)
      type=SimpleType(name="Int")
    fields[2]=StructField(name="three")
      type=SimpleType(name="Bool")
```

**Nocopy struct**

```metall module
pub nocopy struct Handle { id Int }
```

```ast
Module(fileName="test.met",name="test",main=true)
  decls=Struct(name="Handle",pub=true,nocopy=true)
    fields=StructField(name="id")
      type=SimpleType(name="Int")
```

**Nocopy union**

```metall
nocopy union Resource = Int | Str
```

```ast
Union(name="Resource",nocopy=true)
  variants[0]=SimpleType(name="Int")
  variants[1]=SimpleType(name="Str")
```

**Unsafe sync struct**

```metall module
pub unsafe sync struct Mutex { locked Bool }
```

```ast
Module(fileName="test.met",name="test",main=true)
  decls=Struct(name="Mutex",pub=true,unsafe=true,sync=true)
    fields=StructField(name="locked")
      type=SimpleType(name="Bool")
```

**Sync type parameter**

```metall
fun foo<sync T>(x T) void {}
```

```ast
Fun(name="foo")
  typeParams=TypeParam(name="T",sync=true)
  params=FunParam(name="x")
    type=SimpleType(name="T")
  returnType=SimpleType(name="void")
  block=Block()
```

**Sync fun type**

```metall
fun foo(f sync fun() void) void {}
```

```ast
Fun(name="foo")
  params=FunParam(name="f")
    type=FunType(sync=true)
      returnType=SimpleType(name="void")
  returnType=SimpleType(name="void")
  block=Block()
```

**Pub fun**

```metall module
pub fun foo() void {}
```

```ast
Module(fileName="test.met",name="test",main=true)
  decls=Fun(name="foo",pub=true)
    returnType=SimpleType(name="void")
    block=Block()
```

**Pub unsafe fun**

```metall module
pub unsafe fun foo() void {}
```

```ast
Module(fileName="test.met",name="test",main=true)
  decls=Fun(name="foo",pub=true)
    returnType=SimpleType(name="void")
    block=Block()
```

**Pub extern fun**

```metall module
pub extern fun abs(n I32) I32
```

```ast
Module(fileName="test.met",name="test",main=true)
  decls=FunDecl(name="abs",pub=true)
    params=FunParam(name="n")
      type=SimpleType(name="I32")
    returnType=SimpleType(name="I32")
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
Ref()
  target=Ident(name="x")
```

**& has lower precedence than field access**

```metall
&x.one.two
```

```ast
Ref()
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
Ref()
  target=Index()
    target=Ident(name="x")
    index=Int(value=1)
```

**& of deref**

```metall
&x.*
```

```ast
Ref()
  target=Deref()
    expr=Ident(name="x")
```

**& of chained field and index**

```metall
&x.one[2].three
```

```ast
Ref()
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
  returnType=RefType()
    type=SimpleType(name="Int")
  block=Block()
```

**Nested ref type**

```metall
fun foo() &&Int {}
```

```ast
Fun(name="foo")
  returnType=RefType()
    type=RefType()
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

**Nested &ref (ref to temporary)**

```metall
{ &&x }
```

```ast
Block()
  exprs=Ref()
    target=Ref()
      target=Ident(name="x")
```

**&ref of literal (ref to temporary)**

```metall
{ &123 }
```

```ast
Block()
  exprs=Ref()
    target=Int(value=123)
```

**&ref of call (ref to temporary)**

```metall
{ &mut foo() }
```

```ast
Block()
  exprs=Ref(mut=true)
    target=Call()
      callee=Ident(name="foo")
```

## Allocators

**Allocator var**

```metall
let @myalloc = Arena()
```

```ast
AllocatorVar(name=@myalloc)
  expr=TypeConstruction()
    target=Ident(name="Arena")
```

**Allocator var from an arbitrary expression**

```metall
let @b = x.@myalloc
```

```ast
AllocatorVar(name=@b)
  expr=FieldAccess(field=@myalloc)
    target=Ident(name="x")
```

**Heap alloc**

```metall
@myalloc.new<Foo>(Foo())
```

```ast
Call()
  callee=FieldAccess(field=new)
    target=Ident(name="@myalloc")
    typeArgs=SimpleType(name="Foo")
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
    typeArgs=SimpleType(name="Foo")
  args=TypeConstruction()
    target=Ident(name="Foo")
    args=String(value="hello")
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
    type=SliceType()
      type=SliceType()
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
      type=SliceType()
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

**Index compound write**

```metall
x[1] <<= 2
```

```ast
Assign(op=<<)
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
```

**Range expression lo..hi**

```metall
1..3
```

```ast
Range(inclusive=false)
  lo=Int(value=1)
  hi=Int(value=3)
```

**Range expression lo..=hi**

```metall
1..=3
```

```ast
Range(inclusive=true)
  lo=Int(value=1)
  hi=Int(value=3)
```

**Range expression binds looser than arithmetic**

```metall
1 + 1..2 * 3
```

```ast
Range(inclusive=false)
  lo=Binary(op=+)
    lhs=Int(value=1)
    rhs=Int(value=1)
  hi=Binary(op=*)
    lhs=Int(value=2)
    rhs=Int(value=3)
```

**Array fill construction**

```metall
[4 of U8(42)]
```

```ast
ArrayConstruction(len=4)
  fill=TypeConstruction()
    target=Ident(name="U8")
    args=Int(value=42)
```

**Array uninitialized construction**

```metall
unsafe [3 uninit Int]
```

```ast
ArrayConstruction(len=3,unsafe=true)
  elem=SimpleType(name="Int")
```

**of is a keyword only right after the count, an identifier elsewhere**

```metall
{ let of = 99 of + 1 }
```

```ast
Block()
  exprs[0]=Var(name="of")
    expr=Int(value=99)
  exprs[1]=Binary(op=+)
    lhs=Ident(name="of")
    rhs=Int(value=1)
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

**int +%**

```metall
1 +% 2
```

```ast
Binary(op=+%)
  lhs=Int(value=1)
  rhs=Int(value=2)
```

**int -%**

```metall
1 -% 2
```

```ast
Binary(op=-%)
  lhs=Int(value=1)
  rhs=Int(value=2)
```

**int *%**

```metall
1 *% 2
```

```ast
Binary(op=*%)
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

**Unary minus**

```metall
-x
```

```ast
Unary(op=-)
  expr=Ident(name="x")
```

**Glued minus after a value is subtraction**

```metall
a-1
```

```ast
Binary(op=-)
  lhs=Ident(name="a")
  rhs=Int(value=1)
```

**Unary minus binds tighter than binary**

```metall
-a + b
```

```ast
Binary(op=+)
  lhs=Unary(op=-)
    expr=Ident(name="a")
  rhs=Ident(name="b")
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

**Bitwise binds tighter than comparison**

```metall
a & b == c
```

```ast
Binary(op===)
  lhs=Binary(op=&)
    lhs=Ident(name="a")
    rhs=Ident(name="b")
  rhs=Ident(name="c")
```

**Tilde binds tighter than binary operators**

```metall
~a | b
```

```ast
Binary(op=|)
  lhs=Unary(op=~)
    expr=Ident(name="a")
  rhs=Ident(name="b")
```

**not binds tighter than comparison**

```metall
not a == b
```

```ast
Binary(op===)
  lhs=Unary(op=not)
    expr=Ident(name="a")
  rhs=Ident(name="b")
```

**Ampersand and minus disambiguate by position, not spacing**

`a &b` is infix bitwise-and. Inside the call, mid-line `& x` / `& mut y` are
references regardless of spacing. A line-initial `&w` or `-v` starts a new
expression instead of continuing the previous one.

```metall
{
    a &b
    f(& x, & mut y)
    &w
    -v
}
```

```ast
Block()
  exprs[0]=Binary(op=&)
    lhs=Ident(name="a")
    rhs=Ident(name="b")
  exprs[1]=Call()
    callee=Ident(name="f")
    args[0]=Ref()
      target=Ident(name="x")
    args[1]=Ref(mut=true)
      target=Ident(name="y")
  exprs[2]=Ref()
    target=Ident(name="w")
  exprs[3]=Unary(op=-)
    expr=Ident(name="v")
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
for { break }
```

```ast
For()
  body=Block()
    exprs=Break()
```

**Continue**

```metall
for { continue }
```

```ast
For()
  body=Block()
    exprs=Continue()
```

**break is rejected inside an expression**

```metall
1 + break
```

```error
test.met:1:5: break may only be used as a statement, not inside an expression
    1 + break
        ^^^^^
```

**continue is rejected inside an expression**

```metall
1 + continue
```

```error
test.met:1:5: continue may only be used as a statement, not inside an expression
    1 + continue
        ^^^^^^^^
```

**return is rejected inside an expression**

```metall
1 + return 0
```

```error
test.met:1:5: return may only be used as a statement, not inside an expression
    1 + return 0
        ^^^^^^
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

**For in slice literal**

```metall
for x in [1, 2, 3] { 1 }
```

```ast
For(binding=x)
  cond=ArrayLiteral(len=3)
    first=Int(value=1)
  body=Block()
    exprs=Int(value=1)
```

**For in with index binding**

```metall
for x, i in xs { 1 }
```

```ast
For(binding=x,index=i)
  cond=Ident(name="xs")
  body=Block()
    exprs=Int(value=1)
```

**For in over a plain expression parses (iterability is a type error)**

```metall
for x in 0 { 1 }
```

```ast
For(binding=x)
  cond=Int(value=0)
  body=Block()
    exprs=Int(value=1)
```

**For in by reference**

```metall
for &x in xs { 1 }
```

```ast
For(binding=x,ref=true)
  cond=Ident(name="xs")
  body=Block()
    exprs=Int(value=1)
```

**For in by mutable reference with index**

```metall
for &mut x, i in xs { 1 }
```

```ast
For(binding=x,ref=true,mut=true,index=i)
  cond=Ident(name="xs")
  body=Block()
    exprs=Int(value=1)
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
  fields=StructField(name="value")
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
  fields[0]=StructField(name="a")
    type=SimpleType(name="A")
  fields[1]=StructField(name="b")
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
Var(name="x")
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
  fields=StructField(name="value")
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

**Constrained type param with path**

```metall
fun foo<T lib.Showable>(t T) void { }
```

```ast
Fun(name="foo")
  typeParams=TypeParam(name="T")
    constraint=FieldAccess(field=Showable)
      target=Ident(name="lib")
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
  fields[0]=StructField(name="a")
    type=SimpleType(name="A")
  fields[1]=StructField(name="b")
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
      typeArgs=RefType()
        type=SimpleType(name="Int")
  params[3]=FunParam(name="d")
    type=SimpleType(name="Result")
      typeArgs=RefType(mut=true)
        type=SimpleType(name="Int")
  params[4]=FunParam(name="e")
    type=SimpleType(name="Option")
      typeArgs=SliceType()
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

**Shape rejects fields**

```metall
shape Foo { name Str }
```

```error
test.met:1:13: unexpected token: expected <fun>, got <identifier>
    shape Foo { name Str }
                ^^^^
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

**Pub shape with pub fun**

```metall module
pub shape Foo { fun Foo.bar(f Foo) Str pub fun Foo.baz(f Foo) Int }
```

```ast
Module(fileName="test.met",name="test",main=true)
  decls=Shape(name="Foo",pub=true)
    funs[0]=FunDecl(name="Foo.bar")
      params=FunParam(name="f")
        type=SimpleType(name="Foo")
      returnType=SimpleType(name="Str")
    funs[1]=FunDecl(name="Foo.baz",pub=true)
      params=FunParam(name="f")
        type=SimpleType(name="Foo")
      returnType=SimpleType(name="Int")
```

**Pub union**

```metall module
pub union Foo = Str | Int
```

```ast
Module(fileName="test.met",name="test",main=true)
  decls=Union(name="Foo",pub=true)
    variants[0]=SimpleType(name="Str")
    variants[1]=SimpleType(name="Int")
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

**Union with a leading pipe**

A leading `|` is optional, so multi-line variant lists read cleanly.

```metall
union Foo =
    | Str
    | Int
    | Bool
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
  variants[0]=RefType()
    type=SimpleType(name="Str")
  variants[1]=SimpleType(name="Int")
```

**Union with slice variant**

```metall
union Foo = []Int | Str
```

```ast
Union(name="Foo")
  variants[0]=SliceType()
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

## Enums

**Closed enum, compiler-assigned**

```metall
enum Ordering U8 = less | equal | greater
```

```ast
Enum(name="Ordering")
  backing=SimpleType(name="U8")
  variants[0]=EnumVariant(name="less")
  variants[1]=EnumVariant(name="equal")
  variants[2]=EnumVariant(name="greater")
```

**Closed enum with a leading pipe**

```metall
enum Ordering U8 =
    | less
    | equal
    | greater
```

```ast
Enum(name="Ordering")
  backing=SimpleType(name="U8")
  variants[0]=EnumVariant(name="less")
  variants[1]=EnumVariant(name="equal")
  variants[2]=EnumVariant(name="greater")
```

**A closed enum with `=` but no variants is rejected**

```metall
enum Empty U8 =
```

```error
test.met:1:15: enum Empty: expected at least one variant after '='
    enum Empty U8 =
                  ^
```

**Closed enum, explicit and negative tags**

```metall
enum Temp I8 = cold = -1 | mild = 0 | hot = 1
```

```ast
Enum(name="Temp")
  backing=SimpleType(name="I8")
  variants[0]=EnumVariant(name="cold")
    tag=Int(value=-1)
  variants[1]=EnumVariant(name="mild")
    tag=Int(value=0)
  variants[2]=EnumVariant(name="hot")
    tag=Int(value=1)
```

**Open root, no body**

```metall
enum AppErr U32
```

```ast
Enum(name="AppErr",open=true)
  backing=SimpleType(name="U32")
```

**Open root with associated-data schema**

```metall
enum AppErr(posix_code ?I32) U32
```

```ast
Enum(name="AppErr",open=true)
  backing=SimpleType(name="U32")
  params=FunParam(name="posix_code")
    type=SimpleType(name="Option")
      typeArgs=SimpleType(name="I32")
```

**Closed subset of an open root**

```metall
enum IOErr AppErr = file_not_found(2) | broken_pipe(32)
```

```ast
Enum(name="IOErr")
  backing=SimpleType(name="AppErr")
  variants[0]=EnumVariant(name="file_not_found")
    args=Int(value=2)
  variants[1]=EnumVariant(name="broken_pipe")
    args=Int(value=32)
```

**Associated data with tag**

```metall
enum Color(name Str, rgb U32) U8 = red("Red", 0xff0000) = 1 | green("Green", 0x00ff00) = 2
```

```ast
Enum(name="Color")
  backing=SimpleType(name="U8")
  params[0]=FunParam(name="name")
    type=SimpleType(name="Str")
  params[1]=FunParam(name="rgb")
    type=SimpleType(name="U32")
  variants[0]=EnumVariant(name="red")
    args[0]=String(value="Red")
    args[1]=Int(value=16711680)
    tag=Int(value=1)
  variants[1]=EnumVariant(name="green")
    args[0]=String(value="Green")
    args[1]=Int(value=65280)
    tag=Int(value=2)
```

**Associated data with named values**

```metall
enum Color(name Str, rgb U32) U8 = red(rgb = 0xff0000, name = "Red")
```

```ast
Enum(name="Color")
  backing=SimpleType(name="U8")
  params[0]=FunParam(name="name")
    type=SimpleType(name="Str")
  params[1]=FunParam(name="rgb")
    type=SimpleType(name="U32")
  variants=EnumVariant(name="red",args[0].name="rgb",args[1].name="name")
    args[0]=Int(value=16711680)
    args[1]=String(value="Red")
```

**Generic enums are rejected**

```metall module
enum Bad<T> U8 = a | b
```

```error
test.met:1:9: enums cannot be generic
    enum Bad<T> U8 = a | b
            ^
test.met:1:9: unexpected token: <<immediate>
    enum Bad<T> U8 = a | b
            ^
```

**Enums reject copy/sync modifiers**

```metall module
sync enum E U8 = a | b
```

```error
test.met:1:1: unexpected token: expected <enum>, got <sync>
    sync enum E U8 = a | b
    ^^^^
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

**Match with enum variant and subset patterns**

```metall
match e { case Color.red: 1 case IOErr io: 2 else: 3 }
```

```ast
Match(arms=2,arm[0].pattern=n3:FieldAccess,arm[1].pattern=n6:SimpleType,arm[1].binding=io)
  expr=Ident(name="e")
  arm[0].body=Block()
    exprs=Int(value=1)
  arm[1].body=Block()
    exprs=Int(value=2)
  else.body=Block()
    exprs=Int(value=3)
```

**Match or-pattern**

```metall
match c {
    case Color.red or Color.green: 1
    case Color.blue: 2
}
```

```ast
Match(arms=2,arm[0].pattern=n3:FieldAccess,arm[0].pattern.or[0]=n5:FieldAccess,arm[1].pattern=n9:FieldAccess)
  expr=Ident(name="c")
  arm[0].body=Block()
    exprs=Int(value=1)
  arm[1].body=Block()
    exprs=Int(value=2)
```

**Match guard cannot combine with an or-pattern**

```metall
match c {
    case Color.red or Color.green if true: 1
    else: 2
}
```

```error
test.met:2:38: a guard cannot be combined with an or-pattern
    match c {
        case Color.red or Color.green if true: 1
                                         ^^^^
        else: 2
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
use foo.bar
```

```ast
Module(fileName="test.met",name="test",main=true)
  decls=Import(path=foo::bar)
```

**Use deep path**

```metall module
use foo.bar.baz
```

```ast
Module(fileName="test.met",name="test",main=true)
  decls=Import(path=foo::bar::baz)
```

**Use with alias**

```metall module
use b = foo.bar
```

```ast
Module(fileName="test.met",name="test",main=true)
  decls=Import(alias="b",path=foo::bar)
```

**Use local import**

```metall module
use local.foo.bar
```

```ast
Module(fileName="test.met",name="test",main=true)
  decls=Import(path=local::foo::bar)
```

**Use local import with alias**

```metall module
use b = local.foo.bar
```

```ast
Module(fileName="test.met",name="test",main=true)
  decls=Import(alias="b",path=local::foo::bar)
```

**Dot expression (module member)**

```metall
math.pow
```

```ast
FieldAccess(field=pow)
  target=Ident(name="math")
```

**Dot expression call**

```metall
math.pow(2, 5)
```

```ast
Call()
  callee=FieldAccess(field=pow)
    target=Ident(name="math")
  args[0]=Int(value=2)
  args[1]=Int(value=5)
```

**Dot expression with type ident**

```metall
lib.Point(1, 2)
```

```ast
TypeConstruction()
  target=FieldAccess(field=Point)
    target=Ident(name="lib")
  args[0]=Int(value=1)
  args[1]=Int(value=2)
```

**Dot type construction with type args**

```metall
lib.Foo<Int>(42)
```

```ast
TypeConstruction()
  target=FieldAccess(field=Foo)
    target=Ident(name="lib")
    typeArgs=SimpleType(name="Int")
  args=Int(value=42)
```

**Dot type construction nested type args**

```metall
lib.Foo<Bar<Int>>(42)
```

```ast
TypeConstruction()
  target=FieldAccess(field=Foo)
    target=Ident(name="lib")
    typeArgs=SimpleType(name="Bar")
      typeArgs=SimpleType(name="Int")
  args=Int(value=42)
```

**Dot call with type args**

```metall
lib.foo<Int>(42)
```

```ast
Call()
  callee=FieldAccess(field=foo)
    target=Ident(name="lib")
    typeArgs=SimpleType(name="Int")
  args=Int(value=42)
```

**Dot expression in let binding**

```metall
let x = math.pow
```

```ast
Var(name="x")
  expr=FieldAccess(field=pow)
    target=Ident(name="math")
```

**Use in expression should fail**

```metall
use foo.bar
```

```error
test.met:1:1: unexpected token: expected start of an expression, got <use>
    use foo.bar
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

**Try with a qualified variant is rejected**

```metall
try foo() is Color.red
```

```error
test.met:1:14: `try ... is` expects a whole enum subset, not a qualified variant
    try foo() is Color.red
                 ^^^^^
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

**Defer block**

```metall
defer { 1 }
```

```ast
Defer()
  block=Block()
    exprs=Int(value=1)
```

## Extern Functions

**Extern function declaration**

```metall module
extern fun foo(x Int) Int
```

```ast
Module(fileName="test.met",name="test",main=true)
  decls=FunDecl(name="foo")
    params=FunParam(name="x")
      type=SimpleType(name="Int")
    returnType=SimpleType(name="Int")
```

**Extern function with link name**

```metall module
extern("chdir") fun my_chdir(path Int) Int
```

```ast
Module(fileName="test.met",name="test",main=true)
  decls=FunDecl(name="my_chdir",externName="chdir")
    params=FunParam(name="path")
      type=SimpleType(name="Int")
    returnType=SimpleType(name="Int")
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

**Method fun missing method name**

```metall
fun Foo.() void {}
```

```error
test.met:1:9: unexpected token: expected <identifier>, got (
    fun Foo.() void {}
            ^
```

## Module Constants

**Module-level let bindings**

```metall module
struct Foo { one Int two Str }
let a = 42
let b = Foo(1, "hello")
let c = [1, 2, 3]
fun main() void {}
```

```ast
Module(fileName="test.met",name="test",main=true)
  decls[0]=Struct(name="Foo")
    fields[0]=StructField(name="one")
      type=SimpleType(name="Int")
    fields[1]=StructField(name="two")
      type=SimpleType(name="Str")
  decls[1]=Var(name="a")
    expr=Int(value=42)
  decls[2]=Var(name="b")
    expr=TypeConstruction()
      target=Ident(name="Foo")
      args[0]=Int(value=1)
      args[1]=String(value="hello")
  decls[3]=Var(name="c")
    expr=ArrayLiteral(len=3)
      first=Int(value=1)
  decls[4]=Fun(name="main")
    returnType=SimpleType(name="void")
    block=Block()
```

**Module-level mut binding is rejected**

```metall module
mut x = 123
```

```error
test.met:1:1: unexpected token: <mut>
    mut x = 123
    ^^^
```

**noescape parameter**

```metall
fun foo(x noescape &Int) Int { x.* }
```

```ast
Fun(name="foo")
  params=FunParam(name="x",noescape=true)
    type=RefType()
      type=SimpleType(name="Int")
  returnType=SimpleType(name="Int")
  block=Block()
    exprs=Deref()
      expr=Ident(name="x")
```

**noescape in function type**

```metall
fun apply(f fun(noescape &Int) void) void {}
```

```ast
Fun(name="apply")
  params=FunParam(name="f")
    type=FunType(noescape=[true])
      paramTypes=RefType()
        type=SimpleType(name="Int")
      returnType=SimpleType(name="void")
  returnType=SimpleType(name="void")
  block=Block()
```

**noescape return**

```metall
fun foo(x &Int) noescape &Int { x }
```

```ast
Fun(name="foo",noescapeReturn=true)
  params=FunParam(name="x")
    type=RefType()
      type=SimpleType(name="Int")
  returnType=RefType()
    type=SimpleType(name="Int")
  block=Block()
    exprs=Ident(name="x")
```

**noescape return in function type**

```metall
fun apply(f fun() noescape &Int) void {}
```

```ast
Fun(name="apply")
  params=FunParam(name="f")
    type=FunType(noescapeReturn=true)
      returnType=RefType()
        type=SimpleType(name="Int")
  returnType=SimpleType(name="void")
  block=Block()
```

## Conditional Compilation

**simple compile-if at module level**

```metall module
#if os.darwin
fun foo() void {}
#end
```

```ast
Module(fileName="test.met",name="test",main=true)
  decls=CompIf()
    cond=FieldAccess(field=darwin)
      target=Ident(name="os")
    body=Fun(name="foo")
      returnType=SimpleType(name="void")
      block=Block()
```

**compile-if with boolean logic**

```metall module
#if os.darwin and not arch.x86_64
let x = 1
#end
```

```ast
Module(fileName="test.met",name="test",main=true)
  decls=CompIf()
    cond=Binary(op=and)
      lhs=FieldAccess(field=darwin)
        target=Ident(name="os")
      rhs=Unary(op=not)
        expr=FieldAccess(field=x86_64)
          target=Ident(name="arch")
    body=Var(name="x")
      expr=Int(value=1)
```

**compile-if with or**

```metall module
#if os.darwin or os.linux
let x = 1
#end
```

```ast
Module(fileName="test.met",name="test",main=true)
  decls=CompIf()
    cond=Binary(op=or)
      lhs=FieldAccess(field=darwin)
        target=Ident(name="os")
      rhs=FieldAccess(field=linux)
        target=Ident(name="os")
    body=Var(name="x")
      expr=Int(value=1)
```

**compile-if in block**

```metall
{
  let a = 1
  #if tag.debug
  let b = 2
  #end
  a
}
```

```ast
Block()
  exprs[0]=Var(name="a")
    expr=Int(value=1)
  exprs[1]=CompIf()
    cond=FieldAccess(field=debug)
      target=Ident(name="tag")
    body=Var(name="b")
      expr=Int(value=2)
  exprs[2]=Ident(name="a")
```

**compile-if missing #end**

```metall module
#if os.darwin
let x = 1
```

```error
test.met:2:9: unexpected end of file
    #if os.darwin
    let x = 1
            ^
```
