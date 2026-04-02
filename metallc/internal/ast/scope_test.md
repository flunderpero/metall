# Scope Tests

**Simple var**

```metall
let x = 1
```

```scopes
a:-
```

```nodes
n1:Int(value=1):a
n2:Var(name="x",expr=n1:Int):a
```

**Block creates scope**

```metall
{ let x = 1 }
```

```scopes
a:-
b:a
```

```nodes
n1:Int(value=1):b
n2:Var(name="x",expr=n1:Int):b
n3:Block(exprs=[n2:Var]):a
```

**Nested blocks**

```metall
{ let x = 1 { let y = 2 } }
```

```scopes
a:-
b:a
c:b
```

```nodes
n1:Int(value=1):b
n2:Var(name="x",expr=n1:Int):b
n3:Int(value=2):c
n4:Var(name="y",expr=n3:Int):c
n5:Block(exprs=[n4:Var]):b
n6:Block(exprs=[n2:Var,n5:Block]):a
```

**Function**

```metall
fun foo(a Int) Int { a }
```

```scopes
a:-
b:a
c:b
```

```nodes
n1:SimpleType(name="Int"):b
n2:FunParam(name="a",type=n1:SimpleType):b
n3:SimpleType(name="Int"):b
n4:Ident(name="a"):c
n5:Block(exprs=[n4:Ident]):b
n6:Fun(name="foo",params=[n2:FunParam],returnType=n3:SimpleType,block=n5:Block):a
```

**Function with nested block**

```metall
fun foo() void { { 1 } }
```

```scopes
a:-
b:a
c:b
d:c
```

```nodes
n1:SimpleType(name="void"):b
n2:Int(value=1):d
n3:Block(exprs=[n2:Int]):c
n4:Block(exprs=[n3:Block]):b
n5:Fun(name="foo",params=[],returnType=n1:SimpleType,block=n4:Block):a
```

**Struct creates scope**

```metall
struct Foo { one Int }
```

```scopes
a:-
b:a
```

```nodes
n1:SimpleType(name="Int"):b
n2:StructField(name="one",type=n1:SimpleType):b
n3:Struct(name="Foo",fields=[n2:StructField]):a
```

**Shape scope**

```metall
shape Showable { name Str fun Showable.str(self Showable) Str }
```

```scopes
a:-
b:a
c:b
```

```nodes
n1:SimpleType(name="Str"):b
n2:StructField(name="name",type=n1:SimpleType):b
n3:SimpleType(name="Showable"):c
n4:FunParam(name="self",type=n3:SimpleType):c
n5:SimpleType(name="Str"):c
n6:FunDecl(name="Showable.str",params=[n4:FunParam],returnType=n5:SimpleType):b
n7:Shape(name="Showable",fields=[n2:StructField],funs=[n6:FunDecl]):a
```

**Generic struct scope**

```metall
struct Foo<T> { value T }
```

```scopes
a:-
b:a
```

```nodes
n1:TypeParam(name="T"):b
n2:SimpleType(name="T"):b
n3:StructField(name="value",type=n2:SimpleType):b
n4:Struct(name="Foo",typeParams=[n1:TypeParam],fields=[n3:StructField]):a
```

**Match arms create scopes**

```metall
match x { case Int n: n case Str: 0 }
```

```scopes
a:-
b:a
c:a
```

```nodes
n1:Ident(name="x"):a
n2:SimpleType(name="Int"):a
n3:Ident(name="n"):b
n4:Block(exprs=[n3:Ident]):a
n5:SimpleType(name="Str"):a
n6:Int(value=0):c
n7:Block(exprs=[n6:Int]):a
n8:Match(arms=2,expr=n1:Ident):a
```

**Match guard lives in body scope**

```metall
match x { case Int n if n > 0: n case Int: 0 }
```

```scopes
a:-
b:a
c:a
```

```nodes
n1:Ident(name="x"):a
n2:SimpleType(name="Int"):a
n3:Ident(name="n"):b
n4:Int(value=0):b
n5:Binary(op=>,lhs=n3:Ident,rhs=n4:Int):b
n6:Ident(name="n"):b
n7:Block(exprs=[n6:Ident]):a
n8:SimpleType(name="Int"):a
n9:Int(value=0):c
n10:Block(exprs=[n9:Int]):a
n11:Match(arms=2,expr=n1:Ident):a
```
