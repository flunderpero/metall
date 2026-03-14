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
var arenaRuntimeIRTemplate string

//go:embed builtins.ll
var builtinsIR string

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

type matchArmInfo struct {
	label     Label
	armIndex  int
	tag       int
	guardFail Label
}

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
	ast          *ast.AST
	module       ast.Module
	symbols      map[ast.BindingID]Symbol
	regCounter   int
	constCounter int
	strConsts    map[string]int
	opts         IROpts
}

type IRFunGen struct {
	CodeWriter
	*IRGen
	env                *types.TypeEnv
	funRetLabel        Label
	funRetReg          string
	lastLabel          Label
	blockAllocatorRegs []string
	loopStack          []LoopLabels
	astCode            map[ast.NodeID]string
}

func NewIRGen(a *ast.AST, module ast.Module, opts IROpts) *IRGen {
	return &IRGen{
		CodeWriter:   *NewCodeWriter(),
		ast:          a,
		module:       module,
		symbols:      map[ast.BindingID]Symbol{},
		regCounter:   1,
		constCounter: 0,
		strConsts:    map[string]int{},
		opts:         opts,
	}
}

func (g *IRGen) newFunGen(env *types.TypeEnv) *IRFunGen {
	return &IRFunGen{ //nolint:exhaustruct
		CodeWriter: *NewCodeWriter(),
		IRGen:      g,
		env:        env,
		astCode:    map[ast.NodeID]string{},
	}
}

func (g *IRFunGen) Gen(id ast.NodeID) { //nolint:funlen
	node := g.ast.Node(id)
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
		g.genCall(id, kind, node.Span)
	case ast.Deref:
		g.genDeref(id, kind)
	case ast.If:
		g.genIf(id, kind)
	case ast.Match:
		g.genMatch(id, kind)
	case ast.For:
		g.genFor(id, kind)
	case ast.Break:
		g.genBreak(id)
	case ast.Continue:
		g.genContinue(id)
	case ast.Return:
		g.genReturn(id, kind)
	case ast.TypeConstruction:
		g.genTypeConstructionOnStack(id, kind)
	case ast.ArrayLiteral:
		g.genArrayLiteral(id, kind)
	case ast.EmptySlice:
		g.genEmptySlice(id)
	case ast.Index:
		g.genIndex(id, kind)
	case ast.SubSlice:
		g.genSubSlice(id, kind)
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
	case ast.RuneLiteral:
		g.genRuneLiteral(id, kind)
	case ast.Var:
		g.genVar(id, kind)
	case ast.Ref:
		g.genRef(id, kind)
	case ast.AllocatorVar:
		g.genAllocatorVar(id, kind)
	case ast.Struct,
		ast.Union,
		ast.Shape,
		ast.FunParam,
		ast.Fun,
		ast.FunDecl,
		ast.TypeParam,
		ast.SimpleType,
		ast.RefType,
		ast.ArrayType,
		ast.SliceType,
		ast.FunType,
		ast.Path,
		ast.Range:
	default:
		panic(base.Errorf("unknown node kind: %T", kind))
	}
	if unionTypeID, ok := g.env.UnionWrap(id); ok {
		g.genUnionAutoWrap(id, unionTypeID)
	}
}

func (g *IRGen) genStruct(env *types.TypeEnv, s types.StructWork) {
	astStruct := base.Cast[ast.Struct](g.ast.Node(s.NodeID).Kind)
	typ := env.Type(s.TypeID)
	structType := base.Cast[types.StructType](typ.Kind)
	g.write("%%%s = type { ; %s", s.TypeID, structType.Name)
	g.indent++
	for i, astFieldID := range astStruct.Fields {
		astField := base.Cast[ast.StructField](g.ast.Node(astFieldID).Kind)
		fieldIRType := irType(env, structType.Fields[i].Type)
		comma := ""
		if i < len(astStruct.Fields)-1 {
			comma = ","
		}
		g.write("%s%s ; %s", fieldIRType, comma, astField.Name.Name)
	}
	g.indent--
	g.write("}\n")
}

func (g *IRGen) genUnion(env *types.TypeEnv, u types.UnionWork) {
	typ := env.Type(u.TypeID)
	unionType := base.Cast[types.UnionType](typ.Kind)
	payloadSize := unionPayloadSize(env, unionType)
	g.write("%%%s = type { i64, [%d x i8] } ; %s\n", u.TypeID, payloadSize, unionType.Name)
}

func (g *IRFunGen) genArrayLiteral(id ast.NodeID, lit ast.ArrayLiteral) {
	for _, elem := range lit.Elems {
		g.Gen(elem)
	}
	arrTyp := base.Cast[types.ArrayType](g.typeOfNode(id).Kind)
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
	targetType := g.typeOfNode(index.Target)
	if refTyp, ok := targetType.Kind.(types.RefType); ok {
		targetType = g.env.Type(refTyp.Type)
	}
	g.boundsCheckIndex(id, indexReg, targetReg, targetType)
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

func (g *IRFunGen) genSubSlice(id ast.NodeID, sub ast.SubSlice) { //nolint:funlen
	g.Gen(sub.Target)
	targetReg := g.lookupCode(sub.Target)
	targetType := g.typeOfNode(sub.Target)
	if refTyp, ok := targetType.Kind.(types.RefType); ok {
		targetType = g.env.Type(refTyp.Type)
	}
	range_ := base.Cast[ast.Range](g.ast.Node(sub.Range).Kind)
	reg := g.reg()
	// Resolve lo bound (default 0).
	var loReg string
	if range_.Lo != nil {
		g.Gen(*range_.Lo)
		loReg = g.lookupCode(*range_.Lo)
	} else {
		loReg = "0"
	}
	// Resolve hi bound and base data pointer.
	var hiReg string
	var basePtrReg string
	switch kind := targetType.Kind.(type) {
	case types.ArrayType:
		basePtrReg = g.reg()
		arrIRType := g.irType(targetType.ID)
		g.write(
			"%s = getelementptr %s, %s* %s, i64 0, i64 0",
			basePtrReg, arrIRType, arrIRType, targetReg,
		)
		if range_.Hi != nil {
			g.Gen(*range_.Hi)
			hiReg = g.lookupCode(*range_.Hi)
		} else {
			hiReg = fmt.Sprintf("%d", kind.Len)
		}
	case types.SliceType:
		// Extract data pointer from {ptr, i64}.
		basePtrReg = g.reg()
		g.write(
			"%s_field = getelementptr {ptr, i64}, ptr %s, i32 0, i32 0",
			basePtrReg, targetReg,
		)
		g.write("%s = load ptr, ptr %s_field", basePtrReg, basePtrReg)
		if range_.Hi != nil {
			g.Gen(*range_.Hi)
			hiReg = g.lookupCode(*range_.Hi)
		} else {
			// Default hi is slice.len.
			hiReg = g.reg()
			g.write(
				"%s_field = getelementptr {ptr, i64}, ptr %s, i32 0, i32 1",
				hiReg, targetReg,
			)
			g.write("%s = load i64, ptr %s_field", hiReg, hiReg)
		}
	default:
		panic(base.Errorf("genSubSlice: unsupported target type %T", targetType.Kind))
	}
	// Inclusive: hi = hi + 1.
	if range_.Inclusive {
		incReg := g.reg()
		g.write("%s = add i64 %s, 1", incReg, hiReg)
		hiReg = incReg
	}
	g.boundsCheckSubSlice(id, loReg, hiReg, targetReg, targetType)
	// GEP to lo element.
	elemTypeID := base.Cast[types.SliceType](g.typeOfNode(id).Kind).Elem
	elemIRType := g.irType(elemTypeID)
	dataPtrReg := g.reg()
	g.write("%s = getelementptr %s, ptr %s, i64 %s", dataPtrReg, elemIRType, basePtrReg, loReg)
	// Compute len = hi - lo.
	lenReg := g.reg()
	g.write("%s = sub i64 %s, %s", lenReg, hiReg, loReg)
	// Build {ptr, i64} on the stack.
	g.write("%s = alloca {ptr, i64}", reg)
	g.write(
		"%s_ptr_field = getelementptr {ptr, i64}, ptr %s, i32 0, i32 0",
		reg, reg,
	)
	g.write("store ptr %s, ptr %s_ptr_field", dataPtrReg, reg)
	g.write(
		"%s_len_field = getelementptr {ptr, i64}, ptr %s, i32 0, i32 1",
		reg, reg,
	)
	g.write("store i64 %s, ptr %s_len_field", lenReg, reg)
	g.setCode(id, reg)
}

func (g *IRFunGen) genTypeConstructionOnStack(id ast.NodeID, lit ast.TypeConstruction) {
	targetTyp := g.typeOfNode(lit.Target)
	if _, ok := targetTyp.Kind.(types.IntType); ok {
		g.Gen(lit.Target)
		g.Gen(lit.Args[0])
		g.setCode(id, g.lookupCode(lit.Args[0]))
		return
	}
	if unionType, ok := targetTyp.Kind.(types.UnionType); ok {
		g.genUnionConstruction(id, lit, unionType, targetTyp.ID)
		return
	}
	irTyp := g.irType(targetTyp.ID)
	reg := g.reg()
	g.write("%s = alloca %s", reg, irTyp)
	g.genStructConstructionFields(id, lit, reg)
}

func (g *IRFunGen) genUnionConstruction(
	id ast.NodeID, lit ast.TypeConstruction, union types.UnionType, unionTypeID types.TypeID,
) {
	g.Gen(lit.Target)
	g.Gen(lit.Args[0])
	argReg := g.lookupCode(lit.Args[0])
	argTypeID := g.typeIDOfNode(lit.Args[0])
	tag := -1
	for i, variantID := range union.Variants {
		if argTypeID == variantID {
			tag = i
			break
		}
	}
	if tag < 0 {
		panic(base.Errorf("union construction: variant not found"))
	}
	unionIRType := g.irType(unionTypeID)
	reg := g.reg()
	g.write("%s = alloca %s", reg, unionIRType)
	tagPtr := g.reg()
	g.write("%s = getelementptr %s, ptr %s, i32 0, i32 0", tagPtr, unionIRType, reg)
	g.write("store i64 %d, ptr %s", tag, tagPtr)
	payloadPtr := g.reg()
	g.write("%s = getelementptr %s, ptr %s, i32 0, i32 1", payloadPtr, unionIRType, reg)
	g.storeValue(argReg, payloadPtr, argTypeID)
	g.setCode(id, reg)
}

func (g *IRFunGen) genUnionAutoWrap(id ast.NodeID, unionTypeID types.TypeID) {
	variantReg := g.lookupCode(id)
	variantTypeID := g.env.TypeOfNode(id).ID // Intentionally raw: need the pre-wrap variant type.
	unionType := base.Cast[types.UnionType](g.env.Type(unionTypeID).Kind)
	tag := -1
	for i, vID := range unionType.Variants {
		if variantTypeID == vID {
			tag = i
			break
		}
	}
	if tag < 0 {
		panic(base.Errorf("union auto-wrap: variant %s not found in union %s", variantTypeID, unionTypeID))
	}
	unionIRType := g.irType(unionTypeID)
	reg := g.reg()
	g.write("%s = alloca %s", reg, unionIRType)
	tagPtr := g.reg()
	g.write("%s = getelementptr %s, ptr %s, i32 0, i32 0", tagPtr, unionIRType, reg)
	g.write("store i64 %d, ptr %s", tag, tagPtr)
	payloadPtr := g.reg()
	g.write("%s = getelementptr %s, ptr %s, i32 0, i32 1", payloadPtr, unionIRType, reg)
	g.storeValue(variantReg, payloadPtr, variantTypeID)
	g.astCode[id] = reg
}

func (g *IRFunGen) genArenaNew(id ast.NodeID, call ast.Call, fa ast.FieldAccess) {
	g.Gen(fa.Target)
	allocReg := g.lookupCode(fa.Target)
	valueArg := call.Args[0]
	valueTypeID := g.typeIDOfNode(valueArg)
	reg := g.reg()
	size := irTypeSize(g.env, valueTypeID)
	g.write("%s = call ptr @arena_alloc(ptr %s, i64 %d)", reg, allocReg, size)
	if lit, ok := g.ast.Node(valueArg).Kind.(ast.TypeConstruction); ok {
		g.genStructConstructionFields(id, lit, reg)
	} else {
		g.Gen(valueArg)
		valReg := g.lookupCode(valueArg)
		g.storeValue(valReg, reg, valueTypeID)
		g.setCode(id, reg)
	}
}

func (g *IRFunGen) genArenaSlice(id ast.NodeID, call ast.Call, fa ast.FieldAccess) {
	g.Gen(fa.Target)
	allocReg := g.lookupCode(fa.Target)
	sliceType := base.Cast[types.SliceType](g.typeOfNode(id).Kind)
	reg := g.reg()
	irTyp := g.irType(sliceType.Elem)
	elemSize := irTypeSize(g.env, sliceType.Elem)
	g.Gen(call.Args[0])
	lenReg := g.lookupCode(call.Args[0])
	g.write("%s_size = mul i64 %d, %s", reg, elemSize, lenReg)
	g.write("%s_data = call ptr @arena_alloc(ptr %s, i64 %s_size)", reg, allocReg, reg)
	if len(call.Args) == 2 { // slice/slice_mut have a default value arg; slice_uninit variants don't
		g.Gen(call.Args[1])
		valReg := g.lookupCode(call.Args[1])
		g.genInitializeMemory(fmt.Sprintf("%s_data", reg), irTyp, valReg, sliceType.Elem, lenReg, nil)
	}
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
	if g.isAggregateType(elemTypeID) {
		g.genInitializeMemoryStruct(dataReg, valReg, elemTypeID, countReg)
	} else {
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
	valReg string,
	elemTypeID types.TypeID,
	countReg string,
) {
	elemSize := irTypeSize(g.env, elemTypeID)
	g.write("call void @__fill_cpy(ptr %s, ptr %s, i64 %d, i64 %s)", dataReg, valReg, elemSize, countReg)
}

func (g *IRFunGen) genStructConstructionFields(id ast.NodeID, lit ast.TypeConstruction, destReg string) {
	g.Gen(lit.Target)
	for _, arg := range lit.Args {
		g.Gen(arg)
	}
	targetTyp := g.typeOfNode(lit.Target)
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
	targetType := g.typeOfNode(fieldAccess.Target)
	if refTyp, ok := targetType.Kind.(types.RefType); ok {
		targetType = g.env.Type(refTyp.Type)
	}
	if _, ok := targetType.Kind.(types.SliceType); ok {
		g.genSliceFieldAccess(id, fieldAccess)
		return
	}
	if arrType, ok := targetType.Kind.(types.ArrayType); ok {
		if fieldAccess.Field.Name == "len" {
			g.setCode(id, fmt.Sprintf("%d", arrType.Len))
			return
		}
		panic(base.Errorf("unknown array field: %s", fieldAccess.Field.Name))
	}
	ptrReg := g.genFieldAccessPtr(fieldAccess)
	valReg := g.loadValue(ptrReg, g.typeIDOfNode(id))
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

func (g *IRFunGen) genFieldAccessPtr(fieldAccess ast.FieldAccess) string {
	g.Gen(fieldAccess.Target)
	targetType := g.typeOfNode(fieldAccess.Target)
	structReg := g.lookupCode(fieldAccess.Target)
	if refTyp, ok := targetType.Kind.(types.RefType); ok {
		// Auto de-reference: the loaded ref value is already a ptr to the struct data.
		targetType = g.env.Type(refTyp.Type)
	}
	structType := base.Cast[types.StructType](targetType.Kind)
	fieldIndex := indexOfStructField(structType, fieldAccess.Field.Name)
	irTyp := g.irType(targetType.ID)
	ptrReg := g.reg()
	g.write(
		"%s = getelementptr %s, %s* %s, i32 0, i32 %d",
		ptrReg,
		irTyp,
		irTyp,
		structReg,
		fieldIndex,
	)
	return ptrReg
}

func (g *IRFunGen) genFun(work types.FunWork) { //nolint:funlen
	id := work.NodeID
	astFun := base.Cast[ast.Fun](g.ast.Node(id).Kind)
	typ := g.env.Type(work.TypeID)
	fun, ok := typ.Kind.(types.FunType)
	if !ok {
		panic(base.Errorf("expected fun type, got %T", typ.Kind))
	}
	name := irName(work.Name)
	isMain := g.module.Main && name == irName(g.module.Name+".main")
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
		paramNode := g.ast.Node(paramNodeID)
		param, ok := paramNode.Kind.(ast.FunParam)
		if !ok {
			panic(base.Errorf("expected fun param, got %T", paramNode.Kind))
		}
		if params.Len() > 0 {
			params.WriteString(", ")
		}
		g.Gen(paramNodeID)
		preg := g.reg()
		paramTyp := g.typeOfNode(paramNodeID)
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
	bodyHasReturn := g.ast.BlockReturns(astFun.Block)
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
	retTyp := g.typeIDOfNode(return_.Expr)
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
	if forNode.Binding != nil {
		g.genForIn(id, forNode)
		return
	}
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

func (g *IRFunGen) genForIn(id ast.NodeID, forNode ast.For) {
	range_ := base.Cast[ast.Range](g.ast.Node(*forNode.Cond).Kind)
	g.Gen(*range_.Lo)
	g.Gen(*range_.Hi)
	loReg := g.lookupCode(*range_.Lo)
	hiReg := g.lookupCode(*range_.Hi)
	if range_.Inclusive {
		incReg := g.reg()
		g.write("%s = add i64 %s, 1", incReg, hiReg)
		hiReg = incReg
	}
	counterReg := g.reg()
	g.write("%s = alloca i64", counterReg)
	g.write("store i64 %s, ptr %s", loReg, counterReg)
	g.setSymbol(forNode.Body, forNode.Binding.Name, counterReg, "i64")
	labelCond := g.label("for", id)
	labelBody := g.label("body", id)
	labelIncr := g.label("incr", id)
	labelEnd := g.label("endfor", id)
	g.write("br label %%%s", labelCond)
	g.writeLabel(labelCond)
	iReg := g.reg()
	g.write("%s = load i64, ptr %s", iReg, counterReg)
	condReg := g.reg()
	g.write("%s = icmp slt i64 %s, %s", condReg, iReg, hiReg)
	g.write("br i1 %s, label %%%s, label %%%s", condReg, labelBody, labelEnd)
	g.writeLabel(labelBody)
	g.loopStack = append(g.loopStack, LoopLabels{labelIncr, labelEnd})
	defer func() { g.loopStack = g.loopStack[:len(g.loopStack)-1] }()
	g.Gen(forNode.Body)
	g.write("br label %%%s", labelIncr)
	g.writeLabel(labelIncr)
	nextReg := g.reg()
	g.write("%s = load i64, ptr %s", nextReg, counterReg)
	incrReg := g.reg()
	g.write("%s = add i64 %s, 1", incrReg, nextReg)
	g.write("store i64 %s, ptr %s", incrReg, counterReg)
	g.write("br label %%%s", labelCond)
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
	if !g.ast.BlockBreaksControlFlow(ifNode.Then, false) {
		g.write("br label %%%s", contLabel)
	}
	if ifNode.Else != nil {
		g.writeLabel(elseLabel)
		g.Gen(*ifNode.Else)
		if !g.ast.BlockBreaksControlFlow(*ifNode.Else, false) {
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
		thenType := g.typeOfNode(ifNode.Then)
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

func (g *IRFunGen) genMatch(id ast.NodeID, match ast.Match) {
	g.Gen(match.Expr)
	exprReg := g.lookupCode(match.Expr)
	exprType := g.typeOfNode(match.Expr)
	union := base.Cast[types.UnionType](exprType.Kind)
	unionIRType := g.irType(exprType.ID)
	tagPtr := g.reg()
	g.write("%s = getelementptr %s, ptr %s, i32 0, i32 0", tagPtr, unionIRType, exprReg)
	tagReg := g.reg()
	g.write("%s = load i64, ptr %s", tagReg, tagPtr)
	payloadPtr := g.reg()
	g.write("%s = getelementptr %s, ptr %s, i32 0, i32 1", payloadPtr, unionIRType, exprReg)
	contLabel := g.label("endmatch", id)
	var defaultLabel Label
	if match.Else != nil {
		defaultLabel = g.label("case_else", id)
	} else {
		defaultLabel = g.label("unreachable_match", id)
	}
	armInfos := g.buildMatchArmInfos(id, match, union, defaultLabel)
	targets := switchTargets(armInfos)
	sb := strings.Builder{}
	fmt.Fprintf(&sb, "switch i64 %s, label %%%s [", tagReg, defaultLabel)
	for _, info := range targets {
		fmt.Fprintf(&sb, " i64 %d, label %%%s", info.tag, info.label)
	}
	sb.WriteString(" ]")
	g.write(sb.String())
	if match.Else == nil {
		g.writeLabel(defaultLabel)
		g.write("unreachable")
	}
	g.genMatchArms(id, match, armInfos, contLabel, payloadPtr, defaultLabel)
}

func (g *IRFunGen) buildMatchArmInfos(
	id ast.NodeID,
	match ast.Match,
	union types.UnionType,
	elseLabel Label,
) []matchArmInfo {
	infos := make([]matchArmInfo, 0, len(match.Arms))
	for i, arm := range match.Arms {
		patternTypeID := g.typeIDOfNode(arm.Pattern)
		tag := -1
		for vi, vID := range union.Variants {
			if patternTypeID == vID {
				tag = vi
				break
			}
		}
		if tag < 0 {
			panic(base.Errorf("genMatch: variant not found"))
		}
		lbl := g.label(fmt.Sprintf("case_%d_%d", tag, i), id)
		infos = append(infos, matchArmInfo{lbl, i, tag, ""})
	}
	// Chain guarded arms: if a guard fails, fall through to the next arm
	// for the same variant tag. The last arm for a tag falls through to
	// the else label (or unreachable).
	for i := range infos {
		if match.Arms[infos[i].armIndex].Guard == nil {
			continue
		}
		nextLabel := elseLabel
		for j := i + 1; j < len(infos); j++ {
			if infos[j].tag == infos[i].tag {
				nextLabel = infos[j].label
				break
			}
		}
		infos[i].guardFail = nextLabel
	}
	return infos
}

// switchTargets returns one label per variant tag for the switch instruction.
// Only the first arm for each tag appears in the switch table.
func switchTargets(infos []matchArmInfo) []matchArmInfo {
	seen := map[int]bool{}
	var targets []matchArmInfo
	for _, info := range infos {
		if seen[info.tag] {
			continue
		}
		seen[info.tag] = true
		targets = append(targets, info)
	}
	return targets
}

func (g *IRFunGen) genMatchArms( //nolint:funlen
	id ast.NodeID,
	match ast.Match,
	armInfos []matchArmInfo,
	contLabel Label,
	payloadPtr string,
	elseLabel Label,
) {
	resultType := g.typeOfNode(id)
	resultIRType := g.irType(resultType.ID)
	if g.isAggregateType(resultType.ID) {
		resultIRType = "ptr"
	}
	type phiEntry struct {
		code  string
		label Label
	}
	var phiEntries []phiEntry
	for _, armInfo := range armInfos {
		arm := match.Arms[armInfo.armIndex]
		g.writeLabel(armInfo.label)
		g.genMatchArmBinding(arm, payloadPtr)
		if arm.Guard != nil {
			g.Gen(*arm.Guard)
			guardReg := g.lookupCode(*arm.Guard)
			bodyLabel := g.label(fmt.Sprintf("guard_ok_%d", armInfo.armIndex), id)
			g.write("br i1 %s, label %%%s, label %%%s", guardReg, bodyLabel, armInfo.guardFail)
			g.writeLabel(bodyLabel)
		}
		g.Gen(arm.Body)
		if !g.ast.BlockBreaksControlFlow(arm.Body, false) {
			g.write("br label %%%s", contLabel)
			phiEntries = append(phiEntries, phiEntry{g.lookupCode(arm.Body), g.lastLabel})
		}
	}
	if match.Else != nil {
		g.writeLabel(elseLabel)
		elseBody := base.Cast[ast.Block](g.ast.Node(match.Else.Body).Kind)
		if match.Else.Binding != nil && len(elseBody.Exprs) > 0 {
			bindNode := elseBody.Exprs[0]
			bindReg := g.lookupCode(match.Expr)
			g.setSymbol(bindNode, match.Else.Binding.Name, bindReg, "ptr")
		}
		g.Gen(match.Else.Body)
		if !g.ast.BlockBreaksControlFlow(match.Else.Body, false) {
			g.write("br label %%%s", contLabel)
			phiEntries = append(phiEntries, phiEntry{g.lookupCode(match.Else.Body), g.lastLabel})
		}
	}
	g.writeLabel(contLabel)
	if len(phiEntries) == 0 {
		g.write("unreachable")
		g.setCode(id, "void")
		return
	}
	if resultIRType == "void" {
		g.setCode(id, "void")
		return
	}
	phi := g.reg()
	phiSB := strings.Builder{}
	fmt.Fprintf(&phiSB, "%s = phi %s ", phi, resultIRType)
	for i, entry := range phiEntries {
		if i > 0 {
			phiSB.WriteString(", ")
		}
		fmt.Fprintf(&phiSB, "[%s, %%%s]", entry.code, entry.label)
	}
	g.write(phiSB.String())
	g.setCode(id, phi)
}

func (g *IRFunGen) genMatchArmBinding(arm ast.MatchArm, payloadPtr string) {
	body := base.Cast[ast.Block](g.ast.Node(arm.Body).Kind)
	if arm.Binding == nil || len(body.Exprs) == 0 {
		return
	}
	bindNode := body.Exprs[0]
	variantTypeID := g.typeIDOfNode(arm.Pattern)
	variantIRType := g.irType(variantTypeID)
	if g.isAggregateType(variantTypeID) {
		g.setSymbol(bindNode, arm.Binding.Name, payloadPtr, "ptr")
	} else {
		bindReg := g.reg()
		g.write("%s = load %s, ptr %s", bindReg, variantIRType, payloadPtr)
		allocReg := g.reg()
		g.write("%s = alloca %s", allocReg, variantIRType)
		g.write("store %s %s, ptr %s", variantIRType, bindReg, allocReg)
		g.setSymbol(bindNode, arm.Binding.Name, allocReg, variantIRType)
	}
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

func (g *IRFunGen) genAssign(id ast.NodeID, assign ast.Assign) {
	g.Gen(assign.RHS)
	rhs := g.lookupCode(assign.RHS)
	ptrReg := g.genPlaceAddr(assign.LHS)
	rhsTypeID := g.typeIDOfNode(assign.RHS)
	g.storeValue(rhs, ptrReg, rhsTypeID)
	g.setCode(id, "void")
}

// emitSafeIntOp emits an inline division/remainder with a zero-check.
// The location string is only materialized in the cold panic block.
func (g *IRFunGen) emitSafeIntOp(id ast.NodeID, reg, irTyp, op, lhs, rhs string) {
	span := g.ast.Node(id).Span
	locReg := g.addStrConst(span.String())
	panicLabel := g.label("divzero_panic", id)
	okLabel := g.label("divzero_ok", id)
	isZeroReg := g.reg()
	g.write("%s = icmp eq %s %s, 0", isZeroReg, irTyp, rhs)
	g.write("br i1 %s, label %%%s, label %%%s", isZeroReg, panicLabel, okLabel)
	g.writeLabel(panicLabel)
	g.write("call void @panic(ptr @str_division_by_zero, ptr %s)", locReg)
	g.write("unreachable")
	g.writeLabel(okLabel)
	g.write("%s = %s %s %s, %s", reg, op, irTyp, lhs, rhs)
}

func (g *IRFunGen) runeCheckIfNeeded(id ast.NodeID, reg string) {
	if intTyp, ok := g.typeOfNode(id).Kind.(types.IntType); ok && intTyp.Name == "Rune" {
		span := g.ast.Node(id).Span
		locReg := g.addStrConst(span.String())
		checkSurrogateLabel := g.label("rune_check_surrogate", id)
		panicLabel := g.label("rune_panic", id)
		okLabel := g.label("rune_ok", id)
		aboveMaxReg := g.reg()
		g.write("%s = icmp ugt i32 %s, 1114111", aboveMaxReg, reg)
		g.write("br i1 %s, label %%%s, label %%%s", aboveMaxReg, panicLabel, checkSurrogateLabel)
		g.writeLabel(checkSurrogateLabel)
		aboveD7FFReg := g.reg()
		belowE000Reg := g.reg()
		inSurrogateReg := g.reg()
		g.write("%s = icmp ugt i32 %s, 55295", aboveD7FFReg, reg)
		g.write("%s = icmp ult i32 %s, 57344", belowE000Reg, reg)
		g.write("%s = and i1 %s, %s", inSurrogateReg, aboveD7FFReg, belowE000Reg)
		g.write("br i1 %s, label %%%s, label %%%s", inSurrogateReg, panicLabel, okLabel)
		g.writeLabel(panicLabel)
		g.write("call void @panic(ptr @str_illegal_rune, ptr %s)", locReg)
		g.write("unreachable")
		g.writeLabel(okLabel)
	}
}

// boundsCheckIndex emits a bounds check for an index operation.
// Panics with "index out of bounds" if index >= len.
func (g *IRFunGen) boundsCheckIndex(id ast.NodeID, indexReg, targetReg string, targetType *types.Type) {
	span := g.ast.Node(id).Span
	locReg := g.addStrConst(span.String())
	panicLabel := g.label("oob_panic", id)
	okLabel := g.label("oob_ok", id)
	var lenReg string
	switch kind := targetType.Kind.(type) {
	case types.ArrayType:
		lenReg = fmt.Sprintf("%d", kind.Len)
	case types.SliceType:
		lenReg = g.reg()
		g.write("%s_field = getelementptr {ptr, i64}, ptr %s, i32 0, i32 1", lenReg, targetReg)
		g.write("%s = load i64, ptr %s_field", lenReg, lenReg)
	default:
		panic(base.Errorf("boundsCheckIndex: unsupported target type %T", targetType.Kind))
	}
	oobReg := g.reg()
	// Unsigned comparison catches negative indices too (sign bit makes them >= 2^63).
	g.write("%s = icmp uge i64 %s, %s", oobReg, indexReg, lenReg)
	g.write("br i1 %s, label %%%s, label %%%s", oobReg, panicLabel, okLabel)
	g.writeLabel(panicLabel)
	g.write("call void @panic(ptr @str_index_out_of_bounds, ptr %s)", locReg)
	g.write("unreachable")
	g.writeLabel(okLabel)
}

// boundsCheckSubSlice emits bounds checks for a sub-slice operation.
// Panics with "slice out of bounds" if lo > hi or hi > len.
func (g *IRFunGen) boundsCheckSubSlice(id ast.NodeID, loReg, hiReg, targetReg string, targetType *types.Type) {
	span := g.ast.Node(id).Span
	locReg := g.addStrConst(span.String())
	checkHiLenLabel := g.label("slice_check_hi_len", id)
	panicLabel := g.label("slice_oob_panic", id)
	okLabel := g.label("slice_oob_ok", id)
	var lenReg string
	switch kind := targetType.Kind.(type) {
	case types.ArrayType:
		lenReg = fmt.Sprintf("%d", kind.Len)
	case types.SliceType:
		lenReg = g.reg()
		g.write("%s_field = getelementptr {ptr, i64}, ptr %s, i32 0, i32 1", lenReg, targetReg)
		g.write("%s = load i64, ptr %s_field", lenReg, lenReg)
	default:
		panic(base.Errorf("boundsCheckSubSlice: unsupported target type %T", targetType.Kind))
	}
	// Check lo <= hi (unsigned: catches negative lo too).
	loGtHiReg := g.reg()
	g.write("%s = icmp ugt i64 %s, %s", loGtHiReg, loReg, hiReg)
	g.write("br i1 %s, label %%%s, label %%%s", loGtHiReg, panicLabel, checkHiLenLabel)
	g.writeLabel(checkHiLenLabel)
	// Check hi <= len.
	hiGtLenReg := g.reg()
	g.write("%s = icmp ugt i64 %s, %s", hiGtLenReg, hiReg, lenReg)
	g.write("br i1 %s, label %%%s, label %%%s", hiGtLenReg, panicLabel, okLabel)
	g.writeLabel(panicLabel)
	g.write("call void @panic(ptr @str_slice_out_of_bounds, ptr %s)", locReg)
	g.write("unreachable")
	g.writeLabel(okLabel)
}

func (g *IRFunGen) genUnary(id ast.NodeID, unary ast.Unary) {
	g.Gen(unary.Expr)
	expr := g.lookupCode(unary.Expr)
	reg := g.reg()
	switch unary.Op {
	case ast.UnaryOpNot:
		g.write("%s = xor i1 %s, 1", reg, expr)
	case ast.UnaryOpBitNot:
		irTyp := g.irTypeOfNode(unary.Expr)
		g.write("%s = xor %s %s, -1", reg, irTyp, expr)
		g.runeCheckIfNeeded(unary.Expr, reg)
	default:
		panic(base.Errorf("unknown unary operator: %s", unary.Op))
	}
	g.setCode(id, reg)
}

func (g *IRFunGen) genBinary(id ast.NodeID, binary ast.Binary) { //nolint:funlen
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
		if intTyp, ok := g.typeOfNode(binary.LHS).Kind.(types.IntType); ok && !intTyp.Signed {
			divOp = "udiv"
		}
		g.emitSafeIntOp(id, reg, irTyp, divOp, lhs, rhs)
	case ast.BinaryOpMod:
		remOp := "srem"
		if intTyp, ok := g.typeOfNode(binary.LHS).Kind.(types.IntType); ok && !intTyp.Signed {
			remOp = "urem"
		}
		g.emitSafeIntOp(id, reg, irTyp, remOp, lhs, rhs)
	case ast.BinaryOpEq:
		g.write("%s = icmp eq %s %s, %s", reg, irTyp, lhs, rhs)
	case ast.BinaryOpNeq:
		g.write("%s = icmp ne %s %s, %s", reg, irTyp, lhs, rhs)
	case ast.BinaryOpLt, ast.BinaryOpLte, ast.BinaryOpGt, ast.BinaryOpGte:
		intTyp := base.Cast[types.IntType](g.typeOfNode(binary.LHS).Kind)
		signed := intTyp.Signed
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
	case ast.BinaryOpBitAnd:
		g.write("%s = and %s %s, %s", reg, irTyp, lhs, rhs)
	case ast.BinaryOpBitOr:
		g.write("%s = or %s %s, %s", reg, irTyp, lhs, rhs)
	case ast.BinaryOpBitXor:
		g.write("%s = xor %s %s, %s", reg, irTyp, lhs, rhs)
	case ast.BinaryOpShl:
		g.write("%s = shl %s %s, %s", reg, irTyp, lhs, rhs)
	case ast.BinaryOpShr:
		shrOp := "ashr"
		if intTyp, ok := g.typeOfNode(binary.LHS).Kind.(types.IntType); ok && !intTyp.Signed {
			shrOp = "lshr"
		}
		g.write("%s = %s %s %s, %s", reg, shrOp, irTyp, lhs, rhs)
	default:
		panic(base.Errorf("unknown binary operator: %s", binary.Op))
	}
	switch binary.Op { //nolint:exhaustive
	case ast.BinaryOpAdd, ast.BinaryOpSub, ast.BinaryOpDiv, ast.BinaryOpMul, ast.BinaryOpMod,
		ast.BinaryOpBitAnd, ast.BinaryOpBitOr, ast.BinaryOpBitXor, ast.BinaryOpShl, ast.BinaryOpShr:
		g.runeCheckIfNeeded(binary.LHS, reg)
	}
	g.setCode(id, reg)
}

func (g *IRFunGen) arenaAllocMethod(call ast.Call) (string, ast.FieldAccess, bool) {
	name, ok := g.env.NamedFunRef(call.Callee)
	if !ok {
		return "", ast.FieldAccess{}, false
	}
	for _, method := range []string{
		"Arena.new_mut", "Arena.new",
		"Arena.slice_uninit_mut", "Arena.slice_uninit",
		"Arena.slice_mut", "Arena.slice",
	} {
		if strings.HasPrefix(name, method) {
			fa := base.Cast[ast.FieldAccess](g.ast.Node(call.Callee).Kind)
			return method, fa, true
		}
	}
	return "", ast.FieldAccess{}, false
}

func (g *IRFunGen) genCall(id ast.NodeID, call ast.Call, span base.Span) { //nolint:funlen
	if method, fa, ok := g.arenaAllocMethod(call); ok {
		switch method {
		case "Arena.new", "Arena.new_mut":
			g.genArenaNew(id, call, fa)
		default:
			g.genArenaSlice(id, call, fa)
		}
		return
	}
	calleeType := g.typeOfNode(call.Callee)
	fun, ok := calleeType.Kind.(types.FunType)
	if !ok {
		panic(base.Errorf("callee is not a function"))
	}
	var argNodes []ast.NodeID
	if target, ok := g.env.MethodCallReceiver(id); ok {
		argNodes = append(argNodes, target)
	}
	argNodes = append(argNodes, call.Args...)
	for _, nodeID := range argNodes {
		g.Gen(nodeID)
	}
	if funName, ok := g.env.NamedFunRef(call.Callee); ok && funName == "panic" {
		arg1Reg := g.lookupCode((argNodes[0]))
		locReg := g.addStrConst(span.String())
		g.write("call void @panic(ptr %s, ptr %s)", arg1Reg, locReg)
		g.setCode(id, "void")
		return
	}
	sb := strings.Builder{}
	retType := g.env.Type(fun.Return)
	isRetAggregate := g.isAggregateType(fun.Return)
	var resReg string
	if _, ok := retType.Kind.(types.VoidType); ok {
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
	if funName, ok := g.env.NamedFunRef(call.Callee); ok {
		fmt.Fprintf(&sb, " @%s", irName(funName))
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
		typeID := g.typeIDOfNode(nodeID)
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
	if name, ok := g.env.NamedFunRef(id); ok {
		g.setCode(id, "@%s", irName(name))
		return
	}
	if symbol, ok := g.lookupSymbol(id, ident.Name); ok {
		identType := g.typeOfNode(id)
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

func (g *IRFunGen) genRuneLiteral(id ast.NodeID, lit ast.RuneLiteral) {
	g.setCode(id, "%d", lit.Value)
}

func (g *IRFunGen) genBool(id ast.NodeID, bool_ ast.Bool) {
	v := 0
	if bool_.Value {
		v = 1
	}
	g.setCode(id, "%d", v)
}

func (g *IRFunGen) genString(id ast.NodeID, str ast.String) {
	c := g.addStrConst(str.Value)
	g.setCode(id, c)
}

func (g *IRFunGen) addStrConst(s string) string {
	cid, ok := g.strConsts[s]
	if !ok {
		cid = g.constCounter
		g.constCounter++
		g.strConsts[s] = cid
	}
	return fmt.Sprintf("@str.%d", cid)
}

func (g *IRFunGen) genRef(id ast.NodeID, ref ast.Ref) {
	ptrReg := g.genPlaceAddr(ref.Target)
	g.setCode(id, ptrReg)
}

func (g *IRFunGen) genPlaceAddr(nodeID ast.NodeID) string {
	node := g.ast.Node(nodeID)
	switch kind := node.Kind.(type) {
	case ast.Ident:
		if symbol, ok := g.lookupSymbol(nodeID, kind.Name); ok {
			return symbol.Reg
		}
		return kind.Name
	case ast.FieldAccess:
		return g.genFieldAccessPtr(kind)
	case ast.Index:
		return g.genIndexAddr(nodeID, kind)
	case ast.Deref:
		g.Gen(kind.Expr)
		return g.lookupCode(kind.Expr)
	default:
		panic(base.Errorf("genPlaceAddr: unsupported place expression: %T", kind))
	}
}

func (g *IRFunGen) genIndexAddr(id ast.NodeID, index ast.Index) string {
	g.Gen(index.Target)
	g.Gen(index.Index)
	indexReg := g.lookupCode(index.Index)
	targetReg := g.lookupCode(index.Target)
	targetType := g.typeOfNode(index.Target)
	if refTyp, ok := targetType.Kind.(types.RefType); ok {
		targetType = g.env.Type(refTyp.Type)
	}
	g.boundsCheckIndex(id, indexReg, targetReg, targetType)
	ptrReg := g.reg()
	switch targetType.Kind.(type) {
	case types.ArrayType:
		arrIRType := g.irType(targetType.ID)
		g.write("%s = getelementptr %s, %s* %s, i64 0, i64 %s", ptrReg, arrIRType, arrIRType, targetReg, indexReg)
	case types.SliceType:
		elemIRType := g.irType(base.Cast[types.SliceType](targetType.Kind).Elem)
		dataPtrReg := g.reg()
		g.write("%s_field = getelementptr {ptr, i64}, ptr %s, i32 0, i32 0", dataPtrReg, targetReg)
		g.write("%s = load ptr, ptr %s_field", dataPtrReg, dataPtrReg)
		g.write("%s = getelementptr %s, ptr %s, i64 %s", ptrReg, elemIRType, dataPtrReg, indexReg)
	default:
		panic(base.Errorf("genIndexAddr: unsupported target type %T", targetType.Kind))
	}
	return ptrReg
}

func (g *IRFunGen) genDeref(id ast.NodeID, deref ast.Deref) {
	g.Gen(deref.Expr)
	exprType := g.typeOfNode(deref.Expr)
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
	g.write("%s = alloca %%struct.Arena", reg)
	g.write("call void @arena_create(ptr %s)", reg)
	g.blockAllocatorRegs = append(g.blockAllocatorRegs, reg)
	g.setCode(id, reg)
	g.setSymbol(id, alloc.Name.Name, reg, "ptr")
}

func (g *IRFunGen) genVar(id ast.NodeID, v ast.Var) {
	g.Gen(v.Expr)
	exprReg := g.lookupCode(v.Expr)
	exprTypeID := g.typeIDOfNode(v.Expr)
	if g.isAggregateType(exprTypeID) {
		exprNode := g.ast.Node(v.Expr)
		isAutoWrapped := false
		if _, ok := g.env.UnionWrap(v.Expr); ok {
			isAutoWrapped = true
		}
		switch exprNode.Kind.(type) {
		case ast.TypeConstruction, ast.ArrayLiteral, ast.EmptySlice, ast.SubSlice, ast.Call:
			g.setCode(id, exprReg)
			g.setSymbol(id, v.Name.Name, exprReg, "ptr")
		default:
			if isAutoWrapped {
				g.setCode(id, exprReg)
				g.setSymbol(id, v.Name.Name, exprReg, "ptr")
			} else {
				irTyp := g.irType(exprTypeID)
				reg := g.reg()
				g.write("%s = alloca %s", reg, irTyp)
				tmp := g.reg()
				g.write("%s = load %s, ptr %s", tmp, irTyp, exprReg)
				g.write("store %s %s, ptr %s", irTyp, tmp, reg)
				g.setCode(id, reg)
				g.setSymbol(id, v.Name.Name, reg, "ptr")
			}
		}
		return
	}
	reg := g.reg()
	typ := g.irType(exprTypeID)
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

func (g *IRFunGen) typeOfNode(nodeID ast.NodeID) *types.Type {
	if wrapTypeID, ok := g.env.UnionWrap(nodeID); ok {
		if _, generated := g.astCode[nodeID]; generated {
			return g.env.Type(wrapTypeID)
		}
	}
	return g.env.TypeOfNode(nodeID)
}

func (g *IRFunGen) typeIDOfNode(nodeID ast.NodeID) types.TypeID {
	return g.typeOfNode(nodeID).ID
}

func (g *IRFunGen) irTypeOfNode(nodeID ast.NodeID) string {
	return g.irType(g.typeIDOfNode(nodeID))
}

func (g *IRFunGen) irType(typeID types.TypeID) string {
	return irType(g.env, typeID)
}

func (g *IRFunGen) isAggregateType(typeID types.TypeID) bool {
	typ := g.env.Type(typeID)
	switch typ.Kind.(type) {
	case types.StructType, types.UnionType:
		return true
	case types.ArrayType, types.SliceType:
		return true
	default:
		return false
	}
}

func irName(name string) string {
	return strings.ReplaceAll(name, "::", "$")
}

func irType(env *types.TypeEnv, typeID types.TypeID) string {
	typ := env.Type(typeID)
	switch kind := typ.Kind.(type) {
	case types.IntType:
		return fmt.Sprintf("i%d", kind.Bits)
	case types.BoolType:
		return "i1"
	case types.VoidType:
		return "void"
	case types.StructType:
		if kind.Name == "Str" {
			return "%Str"
		}
		return "%" + typeID.String()
	case types.UnionType:
		return "%" + typeID.String()
	case types.RefType, types.AllocatorType:
		return "ptr"
	case types.ArrayType:
		return fmt.Sprintf("[%d x %s]", kind.Len, irType(env, kind.Elem))
	case types.SliceType:
		return "{ptr, i64}"
	case types.FunType:
		return "ptr"
	default:
		panic(base.Errorf("unknown type kind: %T", typ.Kind))
	}
}

// validateDataLayout checks that the LLVM data layout string is compatible with
// our irTypeSize/irTypeAlign assumptions (64-bit pointers, natural alignment).
func validateDataLayout(dl string) {
	if !strings.Contains(dl, "i64:64") {
		panic(base.Errorf("unsupported target data layout: expected i64:64 (8-byte i64 alignment), got %q", dl))
	}
	if strings.Contains(dl, "p:32") {
		panic(base.Errorf("unsupported target data layout: 32-bit pointers are not supported, got %q", dl))
	}
}

func irTypeAlign(env *types.TypeEnv, typeID types.TypeID) int64 {
	typ := env.Type(typeID)
	switch kind := typ.Kind.(type) {
	case types.IntType:
		return int64(kind.Bits+7) / 8
	case types.BoolType:
		return 1
	case types.VoidType:
		return 1
	case types.RefType, types.AllocatorType, types.FunType:
		return 8
	case types.StructType:
		var maxAlign int64 = 1
		for _, field := range kind.Fields {
			if a := irTypeAlign(env, field.Type); a > maxAlign {
				maxAlign = a
			}
		}
		return maxAlign
	case types.UnionType:
		return 8
	case types.ArrayType:
		return irTypeAlign(env, kind.Elem)
	case types.SliceType:
		return 8
	default:
		panic(base.Errorf("irTypeAlign: unknown type kind: %T", typ.Kind))
	}
}

func alignUp(size, align int64) int64 {
	return (size + align - 1) / align * align
}

func irTypeSize(env *types.TypeEnv, typeID types.TypeID) int64 {
	typ := env.Type(typeID)
	switch kind := typ.Kind.(type) {
	case types.IntType:
		return int64(kind.Bits+7) / 8
	case types.BoolType:
		return 1
	case types.VoidType:
		return 0
	case types.RefType, types.AllocatorType, types.FunType:
		return 8
	case types.StructType:
		var offset int64
		var maxAlign int64 = 1
		for _, field := range kind.Fields {
			fieldAlign := irTypeAlign(env, field.Type)
			offset = alignUp(offset, fieldAlign)
			offset += irTypeSize(env, field.Type)
			if fieldAlign > maxAlign {
				maxAlign = fieldAlign
			}
		}
		return alignUp(offset, maxAlign)
	case types.UnionType:
		payload := unionPayloadSize(env, kind)
		return alignUp(8+payload, 8)
	case types.ArrayType:
		elemSize := irTypeSize(env, kind.Elem)
		return kind.Len * elemSize
	case types.SliceType:
		return 16
	default:
		panic(base.Errorf("irTypeSize: unknown type kind: %T", typ.Kind))
	}
}

func unionPayloadSize(env *types.TypeEnv, union types.UnionType) int64 {
	var maxSize int64
	for _, variantID := range union.Variants {
		size := irTypeSize(env, variantID)
		if size > maxSize {
			maxSize = size
		}
	}
	return maxSize
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

func (g *IRFunGen) setCode(astID ast.NodeID, code string, args ...any) {
	if _, ok := g.astCode[astID]; ok {
		panic(base.Errorf("code already set for %s", g.ast.Debug(astID, false, 0)))
	}
	if len(args) > 0 {
		code = fmt.Sprintf(code, args...)
	}
	g.astCode[astID] = code
}

func (g *IRFunGen) lookupCode(astID ast.NodeID) string {
	code, ok := g.astCode[astID]
	if !ok {
		panic(base.Errorf("no reg for %s", g.ast.Debug(astID, false, 0)))
	}
	return code
}

func (g *IRFunGen) setSymbol(nodeID ast.NodeID, name string, reg string, typ string) {
	b, ok := g.env.Lookup(nodeID, name)
	if !ok {
		panic(base.Errorf("symbol %s not found in node %s", name, nodeID))
	}
	g.symbols[b.ID] = Symbol{Name: name, Reg: reg, Type: typ}
}

func (g *IRFunGen) lookupSymbol(nodeID ast.NodeID, name string) (Symbol, bool) {
	b, ok := g.env.Lookup(nodeID, name)
	if !ok {
		return Symbol{}, false
	}
	symbol, ok := g.symbols[b.ID]
	return symbol, ok
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
	TargetDataLayout    string
	TargetTriple        string
	AddressSanitizer    bool
	ArenaDebug          bool
	ArenaStackBufSize   int
	ArenaPageMinSize    int
	ArenaPageMaxSize    int
	ArenaPageHeaderSize int
}

func GenIR(
	a *ast.AST,
	module ast.Module,
	funs []types.FunWork,
	structs []types.StructWork,
	unions []types.UnionWork,
	opts IROpts,
) (string, error) {
	g := NewIRGen(a, module, opts)
	validateDataLayout(opts.TargetDataLayout)
	g.write("; Generated by metallc")
	g.write("")
	g.write(`source_filename = "%s"`, module.FileName)
	g.write(`target datalayout = "%s"`, opts.TargetDataLayout)
	g.write(`target triple = "%s"`, opts.TargetTriple)
	g.write("")
	// Emit the Str type definition (built-in struct, no AST node).
	g.write("%Str = type { {ptr, i64} }")
	// Emit arena type definitions.
	g.write("%struct.PageHeader = type { ptr, ptr, ptr }") //nolint:dupword
	g.write("%%struct.FirstPage = type { %%struct.PageHeader, [%d x i8] }", opts.ArenaStackBufSize)
	g.write("%struct.Arena = type { i64, ptr, %struct.FirstPage }")
	g.write("")
	// Emit struct type definitions.
	for _, s := range structs {
		g.genStruct(s.Env, s)
	}
	// Emit union type definitions.
	for _, u := range unions {
		g.genUnion(u.Env, u)
	}
	// Emit all functions — each gets a fresh IRFunGen.
	for i := range funs {
		f := g.newFunGen(funs[i].Env)
		f.genFun(funs[i])
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
	g.write(builtinsIR)
	for _, bits := range []int{8, 16, 32, 64} {
		irType := fmt.Sprintf("i%d", bits)
		g.write(builtinFill(irType))
	}
	g.write(builtinFill("i1"))
	g.write("; >>> Arena runtime")
	g.write(arenaRuntimeIR(opts))
	return g.sb.String(), nil
}

func arenaRuntimeIR(opts IROpts) string {
	onCreate, onAlloc, onPageAlloc, onDestroy := "", "", "", ""
	declarations := ""
	if opts.ArenaDebug {
		declarations = `
@arena.fmt.create = private unnamed_addr constant [20 x i8] c"arena [%p]: create\0A\00"
@arena.fmt.alloc = private unnamed_addr constant [29 x i8] c"arena [%p]: alloc size=%llu\0A\00"
@arena.fmt.page_alloc = private unnamed_addr constant [54 x i8] c"arena [%p]: page_alloc size=%llu free_prev_page=%llu\0A\00"
@arena.fmt.destroy = private unnamed_addr constant [21 x i8] c"arena [%p]: destroy\0A\00"
declare i32 @dprintf(i32, ptr, ...)`
		onCreate = `call i32 (i32, ptr, ...) @dprintf(i32 2, ptr @arena.fmt.create, ptr %a)`
		onAlloc = `call i32 (i32, ptr, ...) @dprintf(i32 2, ptr @arena.fmt.alloc, ptr %a, i64 %size)`
		onPageAlloc = `%__dbg_end_i = ptrtoint ptr %end to i64
  %__dbg_cur_i = ptrtoint ptr %cursor to i64
  %__dbg_waste = sub i64 %__dbg_end_i, %__dbg_cur_i
  call i32 (i32, ptr, ...) @dprintf(i32 2, ptr @arena.fmt.page_alloc, ptr %a, i64 %alloc_cap, i64 %__dbg_waste)`
		onDestroy = `call i32 (i32, ptr, ...) @dprintf(i32 2, ptr @arena.fmt.destroy, ptr %a)`
	}
	r := strings.NewReplacer(
		"${arena.stack_buf_size}", fmt.Sprintf("%d", opts.ArenaStackBufSize),
		"${arena.page_min_size}", fmt.Sprintf("%d", opts.ArenaPageMinSize),
		"${arena.page_max_size}", fmt.Sprintf("%d", opts.ArenaPageMaxSize),
		"${arena.page_header_size}", fmt.Sprintf("%d", opts.ArenaPageHeaderSize),
		"${arena.on_create}", onCreate,
		"${arena.on_alloc}", onAlloc,
		"${arena.on_page_alloc}", onPageAlloc,
		"${arena.on_destroy}", onDestroy,
	)
	return declarations + "\n" + r.Replace(arenaRuntimeIRTemplate)
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
