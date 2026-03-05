package gen

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/types"
)

//go:embed arena.ll
var arenaRuntime string

type CodeWriter struct {
	indent int
	sb     strings.Builder
}

func NewCodeWriter() *CodeWriter {
	return &CodeWriter{indent: 0, sb: strings.Builder{}}
}

func (g *CodeWriter) write(args ...any) {
	if len(args) == 0 {
		return
	}
	indent := strings.Repeat("    ", g.indent)
	g.sb.WriteString(indent)
	arg0 := base.Cast[string](args[0])
	arg0 = strings.ReplaceAll(arg0, "\n", "\n"+indent)
	if len(args) > 1 {
		fmt.Fprintf(&g.sb, arg0, args[1:]...)
	} else {
		g.sb.WriteString(arg0)
	}
	g.sb.WriteString("\n")
}

type Label string

type Symbol struct {
	Name string
	Reg  string
	Type string
}

type LoopLabels struct {
	continue_ Label
	break_    Label
}

type IRGen struct {
	CodeWriter
	engine       *types.Engine
	module       ast.Module
	symbols      map[types.ScopeID]map[string]Symbol
	regCounter   int
	constCounter int
	strConsts    map[string]int
	astCode      map[ast.NodeID]string
	opts         IROpts
}

type IRFunGen struct {
	CodeWriter
	*IRGen
	funRetLabel        Label
	funRetReg          string
	lastLabel          Label
	blockAllocatorRegs []string
	loopStack          []LoopLabels
}

func NewIRGen(engine *types.Engine, module ast.Module, opts IROpts) *IRGen {
	return &IRGen{
		CodeWriter:   *NewCodeWriter(),
		engine:       engine,
		module:       module,
		symbols:      map[types.ScopeID]map[string]Symbol{},
		regCounter:   1,
		constCounter: 0,
		strConsts:    map[string]int{},
		astCode:      map[ast.NodeID]string{},
		opts:         opts,
	}
}

func (g *IRGen) newFunGen() *IRFunGen {
	return &IRFunGen{CodeWriter: *NewCodeWriter(), IRGen: g} //nolint:exhaustruct
}

func (g *IRFunGen) Gen(id ast.NodeID) { //nolint:funlen
	node := g.engine.Node(id)
	switch kind := node.Kind.(type) {
	case ast.Assign:
		g.genAssign(id, kind)
	case ast.Binary:
		g.genBinary(id, kind)
	case ast.Unary:
		g.genUnary(id, kind)
	case ast.Block:
		g.genBlock(id, kind)
	case ast.Call:
		g.genCall(id, kind)
	case ast.Deref:
		g.genDeref(id, kind)
	case ast.If:
		g.genIf(id, kind)
	case ast.For:
		g.genFor(id, kind)
	case ast.Break:
		g.genBreak(id)
	case ast.Continue:
		g.genContinue(id)
	case ast.Return:
		g.genReturn(id, kind)
	case ast.StructLiteral:
		g.genStructLiteralOnStack(id, kind)
	case ast.New:
		g.genNew(id, kind)
	case ast.ArrayLiteral:
		g.genArrayLiteral(id, kind)
	case ast.EmptySlice:
		g.genEmptySlice(id)
	case ast.Index:
		g.genIndex(id, kind)
	case ast.FieldAccess:
		g.genFieldAccess(id, kind)
	case ast.Ident:
		g.genIdent(id, kind)
	case ast.Int:
		g.genInt(id, kind)
	case ast.Bool:
		g.genBool(id, kind)
	case ast.String:
		g.genString(id, kind)
	case ast.Var:
		g.genVar(id, kind)
	case ast.Ref:
		g.genRef(id, kind)
	case ast.MakeSlice:
		g.genMakeSlice(id, kind)
	case ast.AllocatorVar:
		g.genAllocatorVar(id, kind)
	case ast.Struct,
		ast.FunParam,
		ast.Fun,
		ast.SimpleType,
		ast.RefType,
		ast.ArrayType,
		ast.SliceType,
		ast.FunType,
		ast.NewArray:
	default:
		panic(base.Errorf("unknown node kind: %T", kind))
	}
}

func (g *IRGen) genStruct(id ast.NodeID) {
	astStruct := base.Cast[ast.Struct](g.engine.Node(id).Kind)
	typ := g.engine.TypeOfNode(id)
	structType := base.Cast[types.StructType](typ.Kind)
	g.write("%%%s = type { ; %s", typ.ID, astStruct.Name.Name)
	g.indent++
	for i, astFieldID := range astStruct.Fields {
		astField := base.Cast[ast.StructField](g.engine.Node(astFieldID).Kind)
		fieldIRType := g.irType(structType.Fields[i].Type)
		comma := ""
		if i < len(astStruct.Fields)-1 {
			comma = ","
		}
		g.write("%s%s ; %s", fieldIRType, comma, astField.Name.Name)
	}
	g.indent--
	g.write("}\n")
}

func (g *IRFunGen) genArrayLiteral(id ast.NodeID, lit ast.ArrayLiteral) {
	for _, elem := range lit.Elems {
		g.Gen(elem)
	}
	arrTyp := base.Cast[types.ArrayType](g.engine.TypeOfNode(id).Kind)
	arrIRType := g.irTypeOfNode(id)
	reg := g.reg()
	g.write("%s = alloca %s", reg, arrIRType)
	g.setCode(id, reg)
	for i, elem := range lit.Elems {
		ptrReg := g.reg()
		g.write("%s = getelementptr %s, %s* %s, i32 0, i32 %d", ptrReg, arrIRType, arrIRType, reg, i)
		elemReg := g.lookupCode(elem)
		g.storeValue(elemReg, ptrReg, arrTyp.Elem)
	}
}

func (g *IRFunGen) genEmptySlice(id ast.NodeID) {
	reg := g.reg()
	g.write("%s = alloca {ptr, i64}", reg)
	g.write("store {ptr, i64} zeroinitializer, ptr %s", reg)
	g.setCode(id, reg)
}

func (g *IRFunGen) genIndex(id ast.NodeID, index ast.Index) {
	g.Gen(index.Target)
	g.Gen(index.Index)
	indexReg := g.lookupCode(index.Index)
	targetReg := g.lookupCode(index.Target)
	targetType := g.engine.TypeOfNode(index.Target)
	if refTyp, ok := targetType.Kind.(types.RefType); ok {
		targetType = g.engine.Type(refTyp.Type)
	}
	switch kind := targetType.Kind.(type) {
	case types.ArrayType:
		arrIRType := g.irType(targetType.ID)
		ptrReg := g.reg()
		g.write("%s = getelementptr %s, %s* %s, i64 0, i64 %s", ptrReg, arrIRType, arrIRType, targetReg, indexReg)
		valReg := g.loadValue(ptrReg, kind.Elem)
		g.setCode(id, valReg)
	case types.SliceType:
		elemIRType := g.irType(kind.Elem)
		dataPtrReg := g.reg()
		g.write("%s_field = getelementptr {ptr, i64}, ptr %s, i32 0, i32 0", dataPtrReg, targetReg)
		g.write("%s = load ptr, ptr %s_field", dataPtrReg, dataPtrReg)
		ptrReg := g.reg()
		g.write("%s = getelementptr %s, ptr %s, i64 %s", ptrReg, elemIRType, dataPtrReg, indexReg)
		valReg := g.loadValue(ptrReg, kind.Elem)
		g.setCode(id, valReg)
	default:
		panic(base.Errorf("genIndex: unsupported target type %T", targetType.Kind))
	}
}

func (g *IRFunGen) genStructLiteralOnStack(id ast.NodeID, lit ast.StructLiteral) {
	targetTyp := g.engine.TypeOfNode(lit.Target)
	if _, ok := targetTyp.Kind.(types.BuiltInType); ok {
		g.genTypeConstructor(id, lit)
		return
	}
	irTyp := g.irType(targetTyp.ID)
	reg := g.reg()
	g.write("%s = alloca %s", reg, irTyp)
	g.genStructLiteralFields(id, lit, reg)
}

func (g *IRFunGen) genTypeConstructor(id ast.NodeID, lit ast.StructLiteral) {
	g.Gen(lit.Target)
	g.Gen(lit.Args[0])
	argReg := g.lookupCode(lit.Args[0])
	targetInfo, _ := g.engine.IntTypeInfo(g.engine.TypeOfNode(lit.Target).ID)
	argInfo, _ := g.engine.IntTypeInfo(g.engine.TypeOfNode(lit.Args[0]).ID)
	if targetInfo.Bits == argInfo.Bits {
		g.setCode(id, argReg)
		return
	}
	reg := g.reg()
	srcIR := fmt.Sprintf("i%d", argInfo.Bits)
	dstIR := fmt.Sprintf("i%d", targetInfo.Bits)
	if targetInfo.Bits > argInfo.Bits {
		// Widening: choose sign-extend or zero-extend based on the SOURCE signedness.
		op := "zext"
		if argInfo.Signed {
			op = "sext"
		}
		g.write("%[1]s = %[2]s %[3]s %[4]s to %[5]s", reg, op, srcIR, argReg, dstIR)
	} else {
		// Narrowing: truncate.
		g.write("%[1]s = trunc %[2]s %[3]s to %[4]s", reg, srcIR, argReg, dstIR)
	}
	g.setCode(id, reg)
}

func (g *IRFunGen) genNew(id ast.NodeID, alloc ast.New) {
	g.Gen(alloc.Allocator)
	allocReg := g.lookupCode(alloc.Allocator)
	lit := g.engine.Node(alloc.Target).Kind
	switch lit := lit.(type) {
	case ast.NewArray:
		arrType := base.Cast[ast.ArrayType](g.engine.Node(lit.Type).Kind)
		reg := g.reg()
		irTyp := g.irTypeOfNode(arrType.Elem)
		g.write("%s_elm_size_ptr = getelementptr %s, ptr null, i32 1", reg, irTyp)
		g.write("%s_elm_size = ptrtoint ptr %s_elm_size_ptr to i64", reg, reg)
		g.write("%s_size = mul i64 %s_elm_size, %d", reg, reg, arrType.Len)
		g.write("%s = call ptr @arena_alloc(ptr %s, i64 %s_size)", reg, allocReg, reg)
		if lit.DefaultValue != nil {
			if _, ok := g.engine.Node(*lit.DefaultValue).Kind.(ast.EmptySlice); ok {
				// EmptySlice is all zeros — use memset.
				g.write("call void @llvm.memset.p0.i64(ptr %s, i8 0, i64 %s_size, i1 false)", reg, reg)
			} else {
				g.Gen(*lit.DefaultValue)
				valReg := g.lookupCode(*lit.DefaultValue)
				elemTypeID := g.engine.TypeOfNode(arrType.Elem).ID
				count := arrType.Len
				g.genInitializeMemory(reg, irTyp, valReg, elemTypeID, fmt.Sprintf("%d", count), &count)
			}
		}
		g.setCode(id, reg)
	case ast.StructLiteral:
		irTyp := g.irTypeOfNode(lit.Target)
		reg := g.reg()
		g.write("%s_size_ptr = getelementptr %s, ptr null, i32 1", reg, irTyp)
		g.write("%s_size = ptrtoint ptr %s_size_ptr to i64", reg, reg)
		g.write("%s = call ptr @arena_alloc(ptr %s, i64 %s_size)", reg, allocReg, reg)
		g.genStructLiteralFields(id, lit, reg)
	default:
		panic(base.Errorf("unsupported allocation type %T", lit))
	}
}

func (g *IRFunGen) genMakeSlice(id ast.NodeID, makeSlice ast.MakeSlice) {
	g.Gen(makeSlice.Allocator)
	allocReg := g.lookupCode(makeSlice.Allocator)
	sliceType := base.Cast[ast.SliceType](g.engine.Node(makeSlice.Type).Kind)
	reg := g.reg()
	irTyp := g.irTypeOfNode(sliceType.Elem)
	g.write("%s_elm_size_ptr = getelementptr %s, ptr null, i32 1", reg, irTyp)
	g.write("%s_elm_size = ptrtoint ptr %s_elm_size_ptr to i64", reg, reg)
	g.Gen(makeSlice.Len)
	lenReg := g.lookupCode(makeSlice.Len)
	g.write("%s_size = mul i64 %s_elm_size, %s", reg, reg, lenReg)
	g.write("%s_data = call ptr @arena_alloc(ptr %s, i64 %s_size)", reg, allocReg, reg)
	if makeSlice.DefaultValue != nil {
		if _, ok := g.engine.Node(*makeSlice.DefaultValue).Kind.(ast.EmptySlice); ok {
			// EmptySlice is all zeros — use memset.
			g.write("call void @llvm.memset.p0.i64(ptr %s_data, i8 0, i64 %s_size, i1 false)", reg, reg)
		} else {
			g.Gen(*makeSlice.DefaultValue)
			valReg := g.lookupCode(*makeSlice.DefaultValue)
			elemTypeID := g.engine.TypeOfNode(sliceType.Elem).ID
			g.genInitializeMemory(fmt.Sprintf("%s_data", reg), irTyp, valReg, elemTypeID, lenReg, nil)
		}
	}
	// Build the slice {ptr, i64} on the stack.
	g.write("%s = alloca {ptr, i64}", reg)
	g.write("%s_ptr_field = getelementptr {ptr, i64}, ptr %s, i32 0, i32 0", reg, reg)
	g.write("store ptr %s_data, ptr %s_ptr_field", reg, reg)
	g.write("%s_len_field = getelementptr {ptr, i64}, ptr %s, i32 0, i32 1", reg, reg)
	g.write("store i64 %s, ptr %s_len_field", lenReg, reg)
	g.setCode(id, reg)
}

func (g *IRFunGen) genInitializeMemory(
	dataReg string,
	irElemType string,
	valReg string,
	elemTypeID types.TypeID,
	countReg string,
	compileTimeCount *int64,
) {
	typ := g.engine.Type(elemTypeID)
	switch typ.Kind.(type) {
	case types.StructType, types.SliceType:
		g.genInitializeMemoryStruct(dataReg, irElemType, valReg, countReg)
	default:
		g.genInitializeMemoryScalar(dataReg, irElemType, valReg, countReg, compileTimeCount)
	}
}

func (g *IRFunGen) genInitializeMemoryScalar(
	dataReg string,
	irElemType string,
	valReg string,
	countReg string,
	compileTimeCount *int64,
) {
	// Check if the value is the constant 0.
	if valReg == "0" {
		if compileTimeCount != nil {
			totalBytes := *compileTimeCount * irScalarSize(irElemType)
			g.write("call void @llvm.memset.inline.p0.i64(ptr %s, i8 0, i64 %d, i1 false)", dataReg, totalBytes)
		} else {
			sizeReg := g.reg()
			g.write("%s_elm = mul i64 %s, %d", sizeReg, countReg, irScalarSize(irElemType))
			g.write("call void @llvm.memset.p0.i64(ptr %s, i8 0, i64 %s_elm, i1 false)", dataReg, sizeReg)
		}
		return
	}
	// Non-zero: use a prelude fill function.
	fillValReg := valReg
	fillIRType := irElemType
	if irElemType == "ptr" {
		fillIRType = "i64"
		fillValReg = g.reg()
		g.write("%s = ptrtoint ptr %s to i64", fillValReg, valReg)
	}
	fillFn := fmt.Sprintf("@__fill_%s", fillIRType)
	g.write("call void %s(ptr %s, %s %s, i64 %s)", fillFn, dataReg, fillIRType, fillValReg, countReg)
}

func (g *IRFunGen) genInitializeMemoryStruct(
	dataReg string,
	irElemType string,
	valReg string,
	countReg string,
) {
	elemSizeReg := g.reg()
	g.write("%s_ptr = getelementptr %s, ptr null, i32 1", elemSizeReg, irElemType)
	g.write("%s = ptrtoint ptr %s_ptr to i64", elemSizeReg, elemSizeReg)
	g.write("call void @__fill_cpy(ptr %s, ptr %s, i64 %s, i64 %s)", dataReg, valReg, elemSizeReg, countReg)
}

func (g *IRFunGen) genStructLiteralFields(id ast.NodeID, lit ast.StructLiteral, destReg string) {
	g.Gen(lit.Target)
	for _, arg := range lit.Args {
		g.Gen(arg)
	}
	targetTyp := g.engine.TypeOfNode(lit.Target)
	structTyp := base.Cast[types.StructType](targetTyp.Kind)
	irTyp := g.irType(targetTyp.ID)
	for i, arg := range lit.Args {
		fieldReg := g.reg()
		g.write("%s = getelementptr %s, %s* %s, i32 0, i32 %d", fieldReg, irTyp, irTyp, destReg, i)
		argReg := g.lookupCode(arg)
		g.storeValue(argReg, fieldReg, structTyp.Fields[i].Type)
	}
	g.setCode(id, destReg)
}

func (g *IRFunGen) genFieldAccess(id ast.NodeID, fieldAccess ast.FieldAccess) {
	targetType := g.engine.TypeOfNode(fieldAccess.Target)
	if refTyp, ok := targetType.Kind.(types.RefType); ok {
		targetType = g.engine.Type(refTyp.Type)
	}
	if _, ok := targetType.Kind.(types.SliceType); ok {
		g.genSliceFieldAccess(id, fieldAccess)
		return
	}
	_, ptrReg := g.genFieldAccessPtr(fieldAccess)
	valReg := g.loadValue(ptrReg, g.engine.TypeOfNode(id).ID)
	g.setCode(id, valReg)
}

func (g *IRFunGen) genSliceFieldAccess(id ast.NodeID, fieldAccess ast.FieldAccess) {
	g.Gen(fieldAccess.Target)
	sliceReg := g.lookupCode(fieldAccess.Target)
	switch fieldAccess.Field.Name {
	case "len":
		ptrReg := g.reg()
		g.write("%s = getelementptr {ptr, i64}, ptr %s, i32 0, i32 1", ptrReg, sliceReg)
		valReg := g.reg()
		g.write("%s = load i64, ptr %s", valReg, ptrReg)
		g.setCode(id, valReg)
	default:
		panic(base.Errorf("unknown slice field: %s", fieldAccess.Field.Name))
	}
}

func (g *IRFunGen) genFieldAccessPtr(fieldAccess ast.FieldAccess) (fieldType string, ptrReg string) {
	g.Gen(fieldAccess.Target)
	targetType := g.engine.TypeOfNode(fieldAccess.Target)
	structReg := g.lookupCode(fieldAccess.Target)
	if refTyp, ok := targetType.Kind.(types.RefType); ok {
		// Auto de-reference: the loaded ref value is already a ptr to the struct data.
		targetType = g.engine.Type(refTyp.Type)
	}
	structType := base.Cast[types.StructType](targetType.Kind)
	fieldIndex := indexOfStructField(structType, fieldAccess.Field.Name)
	fieldType = g.irType(structType.Fields[fieldIndex].Type)
	irTyp := g.irType(targetType.ID)
	ptrReg = g.reg()
	g.write(
		"%s = getelementptr %s, %s* %s, i32 0, i32 %d",
		ptrReg,
		irTyp,
		irTyp,
		structReg,
		fieldIndex,
	)
	return fieldType, ptrReg
}

func (g *IRFunGen) genFun(id ast.NodeID) { //nolint:funlen
	astFun := base.Cast[ast.Fun](g.engine.Node(id).Kind)
	typ := g.engine.TypeOfNode(id)
	fun, ok := typ.Kind.(types.FunType)
	if !ok {
		panic(base.Errorf("expected fun type, got %T", typ.Kind))
	}
	name, ok := g.engine.NamedFunRef(id)
	if !ok {
		panic(base.Errorf("no namespaced name for function %s", astFun.Name.Name))
	}
	isMain := g.module.Main && name == g.module.Name+".main"
	if isMain {
		name = "main"
	}
	isRetAggregate := g.isAggregateType(fun.Return)
	retIRTyp := g.irType(fun.Return)
	signatureIRTyp := retIRTyp
	params := strings.Builder{}
	if isMain {
		signatureIRTyp = "i32"
	} else if isRetAggregate {
		signatureIRTyp = "void"
		fmt.Fprintf(&params, "ptr sret(%s) %%out_ptr", g.irType(fun.Return))
	}
	g.funRetLabel = g.label("ret", id)
	g.funRetReg = g.reg()
	paramAllocas := strings.Builder{}
	for _, paramNodeID := range astFun.Params {
		paramNode := g.engine.Node(paramNodeID)
		param, ok := paramNode.Kind.(ast.FunParam)
		if !ok {
			panic(base.Errorf("expected fun param, got %T", paramNode.Kind))
		}
		if params.Len() > 0 {
			params.WriteString(", ")
		}
		g.Gen(paramNodeID)
		preg := g.reg()
		paramTyp := g.engine.TypeOfNode(paramNodeID)
		paramIRTyp := g.irTypeOfNode(paramNodeID)
		if _, ok := paramTyp.Kind.(types.AllocatorType); ok {
			// Allocator param: passed as a raw ptr, no alloca wrapping.
			params.WriteString("ptr ")
			params.WriteString(preg)
			g.setSymbol(paramNodeID, param.Name.Name, preg, "ptr")
		} else if g.isAggregateType(paramTyp.ID) {
			// Aggregate param: byval gives us a ptr to the callee's copy directly.
			// symbol.Reg = preg (single indirection, no alloca ptr wrapper).
			fmt.Fprintf(&params, "ptr byval(%s) ", paramIRTyp)
			params.WriteString(preg)
			g.setSymbol(paramNodeID, param.Name.Name, preg, "ptr")
		} else {
			params.WriteString(paramIRTyp)
			params.WriteString(" ")
			params.WriteString(preg)
			areg := g.reg()
			fmt.Fprintf(&paramAllocas, "%s = alloca %s\n", areg, paramIRTyp)
			fmt.Fprintf(&paramAllocas, "store %s %s, ptr %s\n", paramIRTyp, preg, areg)
			g.setSymbol(paramNodeID, param.Name.Name, areg, paramIRTyp)
		}
	}
	attrs := ""
	if g.opts.AddressSanitizer {
		attrs = "sanitize_address "
	}
	internal := ""
	if !isMain {
		internal = " internal"
	}
	g.write("define%s %s @%s(%s) %s{", internal, signatureIRTyp, name, params.String(), attrs)
	g.indent++
	// We use a return alloca to store values for early returns (i.e. `return <expr>`).
	bodyHasReturn := g.engine.BlockReturns(astFun.Block)
	if retIRTyp != "void" {
		g.write("%s = alloca %s", g.funRetReg, retIRTyp)
	}
	if len(astFun.Params) > 0 {
		g.write(paramAllocas.String())
	}
	g.Gen(astFun.Block)
	// Write the result of the block into the ret reg.
	lastCode := g.lookupCode(astFun.Block)
	if retIRTyp != "void" && !bodyHasReturn {
		g.storeValue(lastCode, g.funRetReg, fun.Return)
	}
	if !bodyHasReturn {
		g.write("br label %%%s", g.funRetLabel)
	}
	g.writeLabel(g.funRetLabel)
	switch {
	case isMain:
		g.write("ret i32 0")
	case isRetAggregate:
		resReg := g.reg()
		g.write("%s = load %s, ptr %s", resReg, retIRTyp, g.funRetReg)
		g.write("store %s %s, ptr %%out_ptr", retIRTyp, resReg)
		g.write("ret void")
	default:
		if retIRTyp == "void" {
			g.write("ret void")
		} else {
			resReg := g.reg()
			g.write("%s = load %s, ptr %s", resReg, retIRTyp, g.funRetReg)
			g.write("ret %s %s", retIRTyp, resReg)
		}
	}
	g.indent--
	g.write("}\n")
}

func (g *IRFunGen) genReturn(id ast.NodeID, return_ ast.Return) {
	g.Gen(return_.Expr)
	exprReg := g.lookupCode(return_.Expr)
	retTyp := g.engine.TypeOfNode(return_.Expr).ID
	if g.irType(retTyp) != "void" {
		g.storeValue(exprReg, g.funRetReg, retTyp)
	}
	g.write("br label %%%s", g.funRetLabel)
	g.setCode(id, exprReg)
}

func (g *IRFunGen) genBlock(id ast.NodeID, block ast.Block) {
	savedAllocatorRegs := g.blockAllocatorRegs
	g.blockAllocatorRegs = nil
	for _, expr := range block.Exprs {
		g.Gen(expr)
	}
	for _, reg := range g.blockAllocatorRegs {
		g.write("call void @arena_destroy (ptr %s)", reg)
	}
	g.blockAllocatorRegs = savedAllocatorRegs
	code := "void"
	if len(block.Exprs) > 0 {
		code = g.lookupCode(block.Exprs[len(block.Exprs)-1])
	}
	g.setCode(id, code)
}

func (g *IRFunGen) genFor(id ast.NodeID, forNode ast.For) {
	labelStart := g.label("for", id)
	labelBody := g.label("body", id)
	labelEnd := g.label("endfor", id)
	g.write("br label %%%s", labelStart)
	g.writeLabel(labelStart)
	if forNode.Cond != nil {
		g.Gen(*forNode.Cond)
		cond := g.lookupCode(*forNode.Cond)
		g.write("br i1 %s, label %%%s, label %%%s", cond, labelBody, labelEnd)
		g.writeLabel(labelBody)
	}
	g.loopStack = append(g.loopStack, LoopLabels{labelStart, labelEnd})
	defer func() { g.loopStack = g.loopStack[:len(g.loopStack)-1] }()
	g.Gen(forNode.Body)
	g.write("br label %%%s", labelStart)
	g.writeLabel(labelEnd)
	g.setCode(id, "void")
}

func (g *IRFunGen) genBreak(id ast.NodeID) {
	loopLabel := g.loopStack[len(g.loopStack)-1]
	g.write("br label %%%s", loopLabel.break_)
	g.setCode(id, "void")
}

func (g *IRFunGen) genContinue(id ast.NodeID) {
	loopLabel := g.loopStack[len(g.loopStack)-1]
	g.write("br label %%%s", loopLabel.continue_)
	g.setCode(id, "void")
}

func (g *IRFunGen) genIf(id ast.NodeID, ifNode ast.If) {
	g.Gen(ifNode.Cond)
	cond := g.lookupCode(ifNode.Cond)
	thenLabel := g.label("then", id)
	contLabel := g.label("endif", id)
	elseLabel := contLabel
	if ifNode.Else != nil {
		elseLabel = g.label("else", id)
	}
	g.write("br i1 %s, label %%%s, label %%%s", cond, thenLabel, elseLabel)
	g.writeLabel(thenLabel)
	g.Gen(ifNode.Then)
	phiThenLabel := g.lastLabel
	if !g.engine.BlockBreaksControlFlow(ifNode.Then, false) {
		g.write("br label %%%s", contLabel)
	}
	if ifNode.Else != nil {
		g.writeLabel(elseLabel)
		g.Gen(*ifNode.Else)
		if !g.engine.BlockBreaksControlFlow(*ifNode.Else, false) {
			g.write("br label %%%s", contLabel)
		}
	}
	phiElseLabel := g.lastLabel
	g.writeLabel(contLabel)
	code := "void"
	if ifNode.Else != nil {
		phi := g.reg()
		thenCode := g.lookupCode(ifNode.Then)
		elseCode := g.lookupCode(*ifNode.Else)
		thenType := g.engine.TypeOfNode(ifNode.Then)
		typ := g.irType(thenType.ID)
		if g.isAggregateType(thenType.ID) {
			typ = "ptr" // Aggregate values flow as pointers in code registers.
		}
		if typ != "void" {
			g.write(
				"%s = phi %s [%s, %%%s], [%s, %%%s]",
				phi,
				typ,
				thenCode,
				phiThenLabel,
				elseCode,
				phiElseLabel,
			)
			code = phi
		}
	}
	g.setCode(id, code)
}

func (g *IRGen) label(name string, id ast.NodeID) Label {
	return Label(fmt.Sprintf("%s_%s", name, id))
}

func (g *IRFunGen) writeLabel(label Label) {
	g.lastLabel = label
	i := g.indent
	g.indent = 0
	g.write("%s:", label)
	g.indent = i
}

func (g *IRFunGen) genAssign(id ast.NodeID, assign ast.Assign) { //nolint:funlen
	g.Gen(assign.RHS)
	rhs := g.lookupCode(assign.RHS)
	lhsNode := g.engine.Node(assign.LHS)
	switch lhsKind := lhsNode.Kind.(type) {
	case ast.Ident:
		symbol, ok := g.lookupSymbol(assign.LHS, lhsKind.Name)
		if !ok {
			panic(base.Errorf("assign to unknown variable: %s", lhsKind.Name))
		}
		rhsType := g.engine.TypeOfNode(assign.RHS)
		if g.isAggregateType(rhsType.ID) {
			irTyp := g.irType(rhsType.ID)
			tmp := g.reg()
			g.write("%s = load %s, ptr %s", tmp, irTyp, rhs)
			g.write("store %s %s, ptr %s", irTyp, tmp, symbol.Reg)
		} else {
			g.write("store %s %s, ptr %s", symbol.Type, rhs, symbol.Reg)
		}
	case ast.FieldAccess:
		fieldType, ptrReg := g.genFieldAccessPtr(lhsKind)
		rhsType := g.engine.TypeOfNode(assign.RHS)
		if g.isAggregateType(rhsType.ID) {
			tmp := g.reg()
			g.write("%s = load %s, ptr %s", tmp, fieldType, rhs)
			g.write("store %s %s, ptr %s", fieldType, tmp, ptrReg)
		} else {
			g.write("store %s %s, ptr %s", fieldType, rhs, ptrReg)
		}
	case ast.Index:
		g.Gen(lhsKind.Target)
		g.Gen(lhsKind.Index)
		targetReg := g.lookupCode(lhsKind.Target)
		indexReg := g.lookupCode(lhsKind.Index)
		ptrReg := g.reg()
		targetType := g.engine.TypeOfNode(lhsKind.Target)
		if refTyp, ok := targetType.Kind.(types.RefType); ok {
			targetType = g.engine.Type(refTyp.Type)
		}
		switch kind := targetType.Kind.(type) {
		case types.ArrayType:
			arrIRType := g.irType(targetType.ID)
			g.write("%s = getelementptr %s, %s* %s, i64 0, i64 %s", ptrReg, arrIRType, arrIRType, targetReg, indexReg)
			g.storeValue(rhs, ptrReg, kind.Elem)
		case types.SliceType:
			elemIRType := g.irType(kind.Elem)
			dataPtrReg := g.reg()
			g.write("%s_field = getelementptr {ptr, i64}, ptr %s, i32 0, i32 0", dataPtrReg, targetReg)
			g.write("%s = load ptr, ptr %s_field", dataPtrReg, dataPtrReg)
			g.write("%s = getelementptr %s, ptr %s, i64 %s", ptrReg, elemIRType, dataPtrReg, indexReg)
			g.storeValue(rhs, ptrReg, kind.Elem)
		default:
			panic(base.Errorf("genAssign index: unsupported target type %T", targetType.Kind))
		}
	case ast.Deref:
		g.Gen(assign.LHS)
		ptr := g.lookupCode(lhsKind.Expr)
		rhsType := g.engine.TypeOfNode(assign.RHS)
		if g.isAggregateType(rhsType.ID) {
			irTyp := g.irType(rhsType.ID)
			tmp := g.reg()
			g.write("%s = load %s, ptr %s", tmp, irTyp, rhs)
			g.write("store %s %s, ptr %s", irTyp, tmp, ptr)
		} else {
			g.write("store %s %s, ptr %s", g.irTypeOfNode(assign.LHS), rhs, ptr)
		}
	default:
		panic(base.Errorf("assign to unknown expression: %T", lhsKind))
	}
	g.setCode(id, "void")
}

func (g *IRFunGen) genUnary(id ast.NodeID, unary ast.Unary) {
	g.Gen(unary.Expr)
	expr := g.lookupCode(unary.Expr)
	reg := g.reg()
	switch unary.Op {
	case ast.UnaryOpNot:
		g.write("%s = xor i1 %s, 1", reg, expr)
	default:
		panic(base.Errorf("unknown unary operator: %s", unary.Op))
	}
	g.setCode(id, reg)
}

func (g *IRFunGen) genBinary(id ast.NodeID, binary ast.Binary) {
	g.Gen(binary.LHS)
	g.Gen(binary.RHS)
	lhs := g.lookupCode(binary.LHS)
	rhs := g.lookupCode(binary.RHS)
	irTyp := g.irTypeOfNode(binary.LHS)
	reg := g.reg()
	switch binary.Op {
	case ast.BinaryOpAdd:
		g.write("%s = add %s %s, %s", reg, g.irTypeOfNode(binary.LHS), lhs, rhs)
	case ast.BinaryOpSub:
		g.write("%s = sub %s %s, %s", reg, g.irTypeOfNode(binary.LHS), lhs, rhs)
	case ast.BinaryOpMul:
		g.write("%s = mul %s %s, %s", reg, g.irTypeOfNode(binary.LHS), lhs, rhs)
	case ast.BinaryOpDiv:
		divOp := "sdiv"
		if info, _ := g.engine.IntTypeInfo(g.engine.TypeOfNode(binary.LHS).ID); !info.Signed {
			divOp = "udiv"
		}
		g.write("%[1]s = call %[2]s @__safe_%[3]s_%[2]s(%[2]s %[4]s, %[2]s %[5]s)", reg, irTyp, divOp, lhs, rhs)
	case ast.BinaryOpMod:
		remOp := "srem"
		if info, _ := g.engine.IntTypeInfo(g.engine.TypeOfNode(binary.LHS).ID); !info.Signed {
			remOp = "urem"
		}
		g.write("%[1]s = call %[2]s @__safe_%[3]s_%[2]s(%[2]s %[4]s, %[2]s %[5]s)", reg, irTyp, remOp, lhs, rhs)
	case ast.BinaryOpEq:
		g.write("%s = icmp eq %s %s, %s", reg, irTyp, lhs, rhs)
	case ast.BinaryOpNeq:
		g.write("%s = icmp ne %s %s, %s", reg, irTyp, lhs, rhs)
	case ast.BinaryOpLt, ast.BinaryOpLte, ast.BinaryOpGt, ast.BinaryOpGte:
		signed := true
		if info, ok := g.engine.IntTypeInfo(g.engine.TypeOfNode(binary.LHS).ID); ok {
			signed = info.Signed
		}
		cmpOp := map[ast.BinaryOp]string{
			ast.BinaryOpLt:  "slt",
			ast.BinaryOpLte: "sle",
			ast.BinaryOpGt:  "sgt",
			ast.BinaryOpGte: "sge",
		}[binary.Op]
		if !signed {
			cmpOp = "u" + cmpOp[1:]
		}
		g.write("%s = icmp %s %s %s, %s", reg, cmpOp, irTyp, lhs, rhs)
	case ast.BinaryOpAnd:
		g.write("%s = and %s %s, %s", reg, irTyp, lhs, rhs)
	case ast.BinaryOpOr:
		g.write("%s = or %s %s, %s", reg, irTyp, lhs, rhs)
	default:
		panic(base.Errorf("unknown binary operator: %s", binary.Op))
	}
	g.setCode(id, reg)
}

func (g *IRFunGen) genCall(id ast.NodeID, call ast.Call) { //nolint:funlen
	calleeType := g.engine.TypeOfNode(call.Callee)
	fun, ok := calleeType.Kind.(types.FunType)
	if !ok {
		panic(base.Errorf("callee is not a function"))
	}
	var argNodes []ast.NodeID
	if target, ok := g.engine.MethodCallReceiver(id); ok {
		argNodes = append(argNodes, target)
	}
	argNodes = append(argNodes, call.Args...)
	for _, nodeID := range argNodes {
		g.Gen(nodeID)
	}
	sb := strings.Builder{}
	retType := g.engine.Type(fun.Return)
	isRetAggregate := g.isAggregateType(fun.Return)
	var resReg string
	if builtin, ok := retType.Kind.(types.BuiltInType); ok && builtin.Name == "void" {
		g.setCode(id, "void")
	} else if isRetAggregate {
		resReg = g.reg()
		g.write("%s = alloca %s", resReg, g.irType(fun.Return))
		g.setCode(id, resReg)
	} else {
		reg := g.reg()
		sb.WriteString(reg + " = ")
		g.setCode(id, reg)
	}
	sb.WriteString("call ")
	if isRetAggregate {
		sb.WriteString("void")
	} else {
		sb.WriteString(g.irType(fun.Return))
	}
	// Resolve the callee. Direct calls use @name, indirect calls go through a loaded ptr.
	if funName, ok := g.engine.NamedFunRef(call.Callee); ok {
		fmt.Fprintf(&sb, " @%s", funName)
	} else {
		g.Gen(call.Callee)
		fmt.Fprintf(&sb, " %s", g.lookupCode(call.Callee))
	}
	sb.WriteString(" (")
	hasArg := false
	if isRetAggregate {
		fmt.Fprintf(&sb, "ptr sret(%s) %s", g.irType(fun.Return), resReg)
		hasArg = true
	}
	for _, nodeID := range argNodes {
		if hasArg {
			sb.WriteString(", ")
		}
		typeID := g.engine.TypeOfNode(nodeID).ID
		reg := g.lookupCode(nodeID)
		if g.isAggregateType(typeID) {
			fmt.Fprintf(&sb, "ptr byval(%s) %s", g.irType(typeID), reg)
		} else {
			fmt.Fprintf(&sb, "%s %s", g.irType(typeID), reg)
		}
		hasArg = true
	}
	sb.WriteString(")")
	g.write(sb.String())
}

func (g *IRFunGen) genIdent(id ast.NodeID, ident ast.Ident) {
	// Named function reference — emit @name directly (no load needed).
	if name, ok := g.engine.NamedFunRef(id); ok {
		g.setCode(id, "@%s", name)
		return
	}
	if symbol, ok := g.lookupSymbol(id, ident.Name); ok {
		identType := g.engine.TypeOfNode(id)
		if _, ok := identType.Kind.(types.AllocatorType); ok || g.isAggregateType(identType.ID) {
			// Aggregate/Allocator: symbol.Reg is already the raw ptr (single indirection, no alloca).
			g.setCode(id, symbol.Reg)
			return
		}
		ptrreg := g.reg()
		g.write("%s = load %s, ptr %s", ptrreg, symbol.Type, symbol.Reg)
		g.setCode(id, ptrreg)
		return
	}
	g.setCode(id, ident.Name)
}

func (g *IRFunGen) genInt(id ast.NodeID, int_ ast.Int) {
	g.setCode(id, int_.Value.String())
}

func (g *IRFunGen) genBool(id ast.NodeID, bool_ ast.Bool) {
	v := 0
	if bool_.Value {
		v = 1
	}
	g.setCode(id, "%d", v)
}

func (g *IRFunGen) genString(id ast.NodeID, str ast.String) {
	cid, ok := g.strConsts[str.Value]
	if !ok {
		cid = g.constCounter
		g.constCounter++
		g.strConsts[str.Value] = cid
	}
	g.setCode(id, "@str.%d", cid)
}

func (g *IRFunGen) genRef(id ast.NodeID, ref ast.Ref) {
	if symbol, ok := g.lookupSymbol(id, ref.Name.Name); ok {
		g.setCode(id, symbol.Reg)
		return
	}
	g.setCode(id, "ptr %s", ref.Name.Name)
}

func (g *IRFunGen) genDeref(id ast.NodeID, deref ast.Deref) {
	g.Gen(deref.Expr)
	exprType := g.engine.TypeOfNode(deref.Expr)
	ref, ok := exprType.Kind.(types.RefType)
	if !ok {
		panic(base.Errorf("dereference: expected reference, got %T", exprType.Kind))
	}
	code := g.lookupCode(deref.Expr)
	valReg := g.loadValue(code, ref.Type)
	g.setCode(id, valReg)
}

func (g *IRFunGen) genAllocatorVar(id ast.NodeID, alloc ast.AllocatorVar) {
	reg := g.reg()
	g.write("%s = call ptr @arena_create(i64 4096)", reg)
	g.blockAllocatorRegs = append(g.blockAllocatorRegs, reg)
	g.setCode(id, reg)
	g.setSymbol(id, alloc.Name.Name, reg, "ptr")
}

func (g *IRFunGen) genVar(id ast.NodeID, v ast.Var) {
	g.Gen(v.Expr)
	exprReg := g.lookupCode(v.Expr)
	exprType := g.engine.TypeOfNode(v.Expr)
	if g.isAggregateType(exprType.ID) {
		exprNode := g.engine.Node(v.Expr)
		switch exprNode.Kind.(type) {
		case ast.StructLiteral, ast.ArrayLiteral, ast.EmptySlice, ast.MakeSlice, ast.Call:
			// The result value is already a copy.
			g.setCode(id, exprReg)
			g.setSymbol(id, v.Name.Name, exprReg, "ptr")
		default:
			// Copy by value so each variable owns independent data.
			irTyp := g.irType(exprType.ID)
			reg := g.reg()
			g.write("%s = alloca %s", reg, irTyp)
			tmp := g.reg()
			g.write("%s = load %s, ptr %s", tmp, irTyp, exprReg)
			g.write("store %s %s, ptr %s", irTyp, tmp, reg)
			g.setCode(id, reg)
			g.setSymbol(id, v.Name.Name, reg, "ptr")
		}
		return
	}
	reg := g.reg()
	typ := g.irTypeOfNode(v.Expr)
	g.write("%s = alloca %s", reg, typ)
	g.write("store %s %s, ptr %s", typ, exprReg, reg)
	g.setCode(id, reg)
	g.setSymbol(id, v.Name.Name, reg, typ)
}

func (g *IRGen) reg() string {
	id := g.regCounter
	g.regCounter++
	return fmt.Sprintf("%%r%d", id)
}

func (g *IRGen) irTypeOfNode(nodeID ast.NodeID) string {
	typ := g.engine.TypeOfNode(nodeID)
	return g.irType(typ.ID)
}

func (g *IRGen) irType(typeID types.TypeID) string {
	typ := g.engine.Type(typeID)
	switch kind := typ.Kind.(type) {
	case types.BuiltInType:
		if info, ok := g.engine.IntTypeInfo(typeID); ok {
			return fmt.Sprintf("i%d", info.Bits)
		}
		switch kind.Name {
		case "Bool":
			return "i1"
		case "void":
			return "void"
		default:
			panic(base.Errorf("unknown builtin type: %s", kind.Name))
		}
	case types.RefType, types.AllocatorType:
		return "ptr"
	case types.StructType:
		if kind.Name == "Str" {
			return "%Str"
		}
		return "%" + typeID.String()
	case types.ArrayType:
		return fmt.Sprintf("[%d x %s]", kind.Len, g.irType(kind.Elem))
	case types.SliceType:
		return "{ptr, i64}"
	case types.FunType:
		return "ptr"
	default:
		panic(base.Errorf("unknown type kind: %T", typ.Kind))
	}
}

func irScalarSize(irType string) int64 {
	switch irType {
	case "i1", "i8":
		return 1
	case "i16":
		return 2
	case "i32":
		return 4
	case "i64", "ptr":
		return 8
	default:
		panic(base.Errorf("unknown scalar IR type: %s", irType))
	}
}

func (g *IRGen) setCode(astID ast.NodeID, code string, args ...any) {
	if _, ok := g.astCode[astID]; ok {
		panic(base.Errorf("code already set for %s", g.engine.AST.Debug(astID, false, 0)))
	}
	if len(args) > 0 {
		code = fmt.Sprintf(code, args...)
	}
	g.astCode[astID] = code
}

func (g *IRGen) lookupCode(astID ast.NodeID) string {
	code, ok := g.astCode[astID]
	if !ok {
		panic(base.Errorf("no reg for %s", g.engine.AST.Debug(astID, false, 0)))
	}
	return code
}

func (g *IRGen) setSymbol(nodeID ast.NodeID, name string, reg string, typ string) {
	scope := g.engine.ScopeGraph.NodeScope(nodeID)
	if g.symbols[scope.ID] == nil {
		g.symbols[scope.ID] = map[string]Symbol{}
	}
	if _, ok := g.symbols[scope.ID][name]; ok {
		panic(base.Errorf("symbol %s already defined in scope %d", name, scope.ID))
	}
	g.symbols[scope.ID][name] = Symbol{Name: name, Reg: reg, Type: typ}
}

func (g *IRGen) lookupSymbol(nodeID ast.NodeID, name string) (Symbol, bool) {
	scope := g.engine.ScopeGraph.NodeScope(nodeID)
	for scope != nil {
		if symbols, ok := g.symbols[scope.ID]; ok {
			if symbol, ok := symbols[name]; ok {
				return symbol, true
			}
		}
		scope = scope.Parent
	}
	return Symbol{}, false
}

func (g *IRGen) isAggregateType(typeID types.TypeID) bool {
	switch g.engine.Type(typeID).Kind.(type) {
	case types.StructType, types.ArrayType, types.SliceType:
		return true
	default:
		return false
	}
}

func (g *IRFunGen) loadValue(ptrReg string, typeID types.TypeID) string {
	if g.isAggregateType(typeID) {
		return ptrReg
	}
	reg := g.reg()
	g.write("%s = load %s, ptr %s", reg, g.irType(typeID), ptrReg)
	return reg
}

func (g *IRFunGen) storeValue(srcReg string, dstReg string, typeID types.TypeID) {
	irTyp := g.irType(typeID)
	if g.isAggregateType(typeID) {
		tmp := g.reg()
		g.write("%s = load %s, ptr %s", tmp, irTyp, srcReg)
		g.write("store %s %s, ptr %s", irTyp, tmp, dstReg)
	} else {
		g.write("store %s %s, ptr %s", irTyp, srcReg, dstReg)
	}
}

func indexOfStructField(s types.StructType, name string) int {
	for i, field := range s.Fields {
		if field.Name == name {
			return i
		}
	}
	panic(base.Errorf("field %q not found in struct %q", name, s.Name))
}

type IROpts struct {
	AddressSanitizer bool
}

func GenIR(moduleID ast.NodeID, engine *types.Engine, opts IROpts) (string, error) {
	module := base.Cast[ast.Module](engine.Node(moduleID).Kind)
	g := NewIRGen(engine, module, opts)
	g.write("; Generated by metallc")
	g.write("")
	g.write(`source_filename = "%s"`, module.FileName)
	g.write("")
	// Emit the Str type definition (built-in struct, no AST node).
	g.write("%Str = type { {ptr, i64} }\n")
	// Emit struct type definitions.
	for _, id := range engine.Structs {
		g.genStruct(id)
	}
	// Emit all functions — each gets a fresh IRFunGen.
	for _, id := range engine.Funs {
		f := g.newFunGen()
		f.genFun(id)
		g.sb.WriteString(f.sb.String())
	}
	// Emit string constants.
	g.write("; Global constants.")
	g.write("")
	consts := make([]string, len(g.strConsts))
	for value, id := range g.strConsts {
		consts[id] = value
	}
	for id, value := range consts {
		n := len(value)
		g.write(`@str.%d.data = private constant [%d x i8] c"%s"`, id, n, value)
		g.write(`@str.%d = private constant %%Str { {ptr, i64} { ptr @str.%d.data, i64 %d } }`, id, id, n)
	}
	g.write(builtins)
	g.write(builtinPrintStr)
	for _, bits := range []int{8, 16, 32, 64} {
		irType := fmt.Sprintf("i%d", bits)
		g.write(builtinSafeDiv(irType, "sdiv"))
		g.write(builtinSafeDiv(irType, "udiv"))
		g.write(builtinSafeDiv(irType, "srem"))
		g.write(builtinSafeDiv(irType, "urem"))
		g.write(builtinFill(irType))
	}
	g.write(builtinFill("i1"))
	g.write("; >>> Arena runtime")
	g.write(arenaRuntime)
	return g.sb.String(), nil
}

func builtinSafeDiv(irType string, op string) string {
	return fmt.Sprintf(`define internal %[1]s @__safe_%[2]s_%[1]s(%[1]s %%a, %[1]s %%b) alwaysinline {
    %%is_zero = icmp eq %[1]s %%b, 0
    br i1 %%is_zero, label %%panic, label %%ok
panic:
    call void @llvm.trap()
    unreachable
ok:
    %%result = %[2]s %[1]s %%a, %%b
    ret %[1]s %%result
}
`, irType, op)
}

func builtinFill(irType string) string {
	// For i1 (Bool), LLVM stores require i8 values, so the fill value
	// parameter and store use i8 while the GEP stride uses i1.
	valType := irType
	if irType == "i1" {
		valType = "i8"
	}
	return fmt.Sprintf(`define internal void @__fill_%[1]s(ptr %%dst, %[2]s %%val, i64 %%count) {
entry:
    br label %%loop
loop:
    %%i = phi i64 [ 0, %%entry ], [ %%next, %%body ]
    %%done = icmp sge i64 %%i, %%count
    br i1 %%done, label %%exit, label %%body
body:
    %%ptr = getelementptr %[1]s, ptr %%dst, i64 %%i
    store %[2]s %%val, ptr %%ptr
    %%next = add i64 %%i, 1
    br label %%loop
exit:
    ret void
}
`, irType, valType)
}

const builtinPrintStr = `
@fmt_str = private constant [6 x i8] c"%.*s\0A\00"
define internal void @print_str(ptr byval(%Str) %s) {
    %data_field = getelementptr %Str, ptr %s, i32 0, i32 0, i32 0
    %data = load ptr, ptr %data_field
    %len_field = getelementptr %Str, ptr %s, i32 0, i32 0, i32 1
    %len = load i64, ptr %len_field
    %len32 = trunc i64 %len to i32
    call i32 (ptr, ...) @printf(ptr @fmt_str, i32 %len32, ptr %data)
    ret void
}
`

const builtins = `
; >>> Runtime

;     External functions.

declare i32 @puts(ptr)
declare i32 @printf(ptr, ...)

;      Builtin functions.

@fmt_int = private constant [6 x i8] c"%lld\0A\00"
define internal void @print_int(i64 %n) {
    call i32 (ptr, ...) @printf(ptr @fmt_int, i64 %n)
    ret void
}

@fmt_uint = private constant [6 x i8] c"%llu\0A\00"
define internal void @print_uint(i64 %n) {
    call i32 (ptr, ...) @printf(ptr @fmt_uint, i64 %n)
    ret void
}

@str_true = private constant [5 x i8] c"true\00"
@str_false = private constant [6 x i8] c"false\00"
define internal void @print_bool(i1 %n) {
	br i1 %n, label %true, label %false
	true:
	    call i32 @puts(ptr @str_true)
	    ret void
	false:
		call i32 @puts(ptr @str_false)
		ret void
}

define internal void @__fill_cpy(ptr %dst, ptr %val, i64 %elem_size, i64 %count) {
entry:
    br label %loop
loop:
    %i = phi i64 [ 0, %entry ], [ %next_i, %body ]
    %curr_ptr = phi ptr [ %dst, %entry ], [ %next_ptr, %body ] 
    %done = icmp sge i64 %i, %count
    br i1 %done, label %exit, label %body
body:
    call void @llvm.memcpy.p0.p0.i64(ptr %curr_ptr, ptr %val, i64 %elem_size, i1 false)
    %next_i = add i64 %i, 1
    %next_ptr = getelementptr i8, ptr %curr_ptr, i64 %elem_size
    br label %loop
exit:
    ret void
}
`
