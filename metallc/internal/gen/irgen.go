package gen

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/types"
)

// voidValue is the LLVM IR literal for a value of the void type ({}).
const voidValue = "zeroinitializer"

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
	continue_      Label
	break_         Label
	arenaStackBase int // arenaRegStack depth at loop entry
}

type IRGen struct {
	CodeWriter
	ast            *ast.AST
	module         ast.Module
	symbols        map[ast.BindingID]Symbol
	regCounter     int
	constCounter   int
	strConsts      map[string]string // dedup: value → global name
	constGlobals   strings.Builder   // global constant declarations (strings, const arrays)
	funValWrappers map[string]string // irName → wrapper name
	opts           IROpts
}

type IRFunGen struct {
	CodeWriter
	*IRGen
	env           *types.TypeEnv
	funRetLabel   Label
	funRetReg     string
	lastLabel     Label
	arenaRegStack [][]string     // stack of arena regs per block scope
	deferStack    [][]ast.NodeID // stack of defer block IDs per block scope
	loopStack     []LoopLabels
	astCode       map[ast.NodeID]string
	entryAllocas  strings.Builder
	constInit     bool // true when generating module-level constant init
	wrapperBuf    strings.Builder
}

func NewIRGen(a *ast.AST, module ast.Module, opts IROpts) *IRGen {
	return &IRGen{
		CodeWriter:     *NewCodeWriter(),
		ast:            a,
		module:         module,
		symbols:        map[ast.BindingID]Symbol{},
		regCounter:     1,
		constCounter:   0,
		strConsts:      map[string]string{},
		constGlobals:   strings.Builder{},
		funValWrappers: map[string]string{},
		opts:           opts,
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
	case ast.When:
		g.genWhen(id, kind)
	case ast.Match:
		g.genMatch(id, kind)
	case ast.For:
		g.genFor(id, kind)
	case ast.Break:
		g.genBreak(id)
	case ast.Continue:
		g.genContinue(id)
	case ast.Defer:
		// Defer blocks are collected during genBlock and emitted at exit points.
		g.setCode(id, voidValue)
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
		ast.Capture,
		ast.FunParam,
		ast.Fun,
		ast.FunDecl,
		ast.TypeParam,
		ast.SimpleType,
		ast.TryPattern,
		ast.RefType,
		ast.ArrayType,
		ast.SliceType,
		ast.FunType,
		ast.Range:
	default:
		panic(base.Errorf("unknown node kind: %T", kind))
	}
	if g.breaksControlFlow(id) {
		g.write("unreachable")
	} else if unionTypeID, variantTag, ok := g.env.UnionWrap(id); ok {
		g.genUnionAutoWrap(id, unionTypeID, variantTag)
	}
}

func (g *IRGen) genStruct(env *types.TypeEnv, s types.TypeWork) {
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

func (g *IRGen) genUnion(env *types.TypeEnv, u types.TypeWork) {
	typ := env.Type(u.TypeID)
	unionType := base.Cast[types.UnionType](typ.Kind)
	// LLVM has no native union type. The standard approach is to use a struct
	// whose payload field is the type of the largest variant.
	// Using [N x i8] instead would cause SROA to decompose stores into
	// byte-level operations, which can make instcombine fail to reach a fixpoint.
	// See https://mapping-high-level-constructs-to-llvm-ir.readthedocs.io/en/latest/basic-constructs/unions.html
	payloadIRType := unionPayloadIRType(env, unionType)
	g.write("%%%s = type { i64, %s } ; %s\n", u.TypeID, payloadIRType, unionType.Name)
}

func unionPayloadIRType(env *types.TypeEnv, union types.UnionType) string {
	var maxSize int64
	var maxIRType string
	for _, variantID := range union.Variants {
		size := irTypeSize(env, variantID)
		if size > maxSize {
			maxSize = size
			maxIRType = irType(env, variantID)
		}
	}
	if maxSize == 0 {
		return "[0 x i8]"
	}
	return maxIRType
}

func (g *IRFunGen) genArrayLiteral(id ast.NodeID, lit ast.ArrayLiteral) {
	for _, elem := range lit.Elems {
		g.Gen(elem)
	}
	arrTyp := base.Cast[types.ArrayType](g.typeOfNode(id).Kind)
	arrIRType := g.irTypeOfNode(id)
	if g.constInit || g.env.IsConstArray(id) {
		// Emit array as a global so subslices pointing into it remain
		// valid after the enclosing function/init returns.
		cid := g.constCounter
		g.constCounter++
		globalName := fmt.Sprintf("@__const_arr_%d", cid)
		fmt.Fprintf(&g.constGlobals, "%s = internal global %s zeroinitializer\n", globalName, arrIRType)
		g.setCode(id, globalName)
		for i, elem := range lit.Elems {
			ptrReg := g.reg()
			g.write("%s = getelementptr %s, %s* %s, i32 0, i32 %d", ptrReg, arrIRType, arrIRType, globalName, i)
			elemReg := g.lookupCode(elem)
			g.storeValue(elemReg, ptrReg, arrTyp.Elem)
		}
		return
	}
	reg := g.reg()
	g.writeAlloca(reg, arrIRType)
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
	g.writeAlloca(reg, "{ptr, i64}")
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
	g.writeAlloca(reg, "{ptr, i64}")
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
	g.writeAlloca(reg, irTyp)
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
	g.writeAlloca(reg, unionIRType)
	tagPtr := g.reg()
	g.write("%s = getelementptr %s, ptr %s, i32 0, i32 0", tagPtr, unionIRType, reg)
	g.write("store i64 %d, ptr %s", tag, tagPtr)
	payloadPtr := g.reg()
	g.write("%s = getelementptr %s, ptr %s, i32 0, i32 1", payloadPtr, unionIRType, reg)
	g.storeValue(argReg, payloadPtr, argTypeID)
	g.setCode(id, reg)
}

func (g *IRFunGen) genUnionAutoWrap(id ast.NodeID, unionTypeID types.TypeID, tag int) {
	variantReg := g.lookupCode(id)
	unionType := base.Cast[types.UnionType](g.env.Type(unionTypeID).Kind)
	variantTypeID := unionType.Variants[tag]
	unionIRType := g.irType(unionTypeID)
	reg := g.reg()
	g.writeAlloca(reg, unionIRType)
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
	g.write("%s = call ptr @runtime$arena.arena_alloc(ptr %s, i64 %d)", reg, allocReg, size)
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
	g.write("%s_data = call ptr @runtime$arena.arena_alloc(ptr %s, i64 %s_size)", reg, allocReg, reg)
	if len(call.Args) == 2 { // slice has a default value arg; slice_uninit doesn't
		g.Gen(call.Args[1])
		valReg := g.lookupCode(call.Args[1])
		g.genInitializeMemory(fmt.Sprintf("%s_data", reg), irTyp, valReg, sliceType.Elem, lenReg, nil)
	}
	g.writeAlloca(reg, "{ptr, i64}")
	g.write("%s_ptr_field = getelementptr {ptr, i64}, ptr %s, i32 0, i32 0", reg, reg)
	g.write("store ptr %s_data, ptr %s_ptr_field", reg, reg)
	g.write("%s_len_field = getelementptr {ptr, i64}, ptr %s, i32 0, i32 1", reg, reg)
	g.write("store i64 %s, ptr %s_len_field", lenReg, reg)
	g.setCode(id, reg)
}

func (g *IRFunGen) genArenaGrow(id ast.NodeID, call ast.Call, fa ast.FieldAccess) {
	// Args: s []T, new_len Int[, default T]
	g.Gen(fa.Target)
	allocReg := g.lookupCode(fa.Target)
	sliceType := base.Cast[types.SliceType](g.typeOfNode(id).Kind)
	reg := g.reg()
	irTyp := g.irType(sliceType.Elem)
	elemSize := irTypeSize(g.env, sliceType.Elem)

	// Load old slice data ptr and length.
	g.Gen(call.Args[0])
	sliceReg := g.lookupCode(call.Args[0])
	g.write("%s_old_ptr_field = getelementptr {ptr, i64}, ptr %s, i32 0, i32 0", reg, sliceReg)
	g.write("%s_old_ptr = load ptr, ptr %s_old_ptr_field", reg, reg)
	g.write("%s_old_len_field = getelementptr {ptr, i64}, ptr %s, i32 0, i32 1", reg, sliceReg)
	g.write("%s_old_len = load i64, ptr %s_old_len_field", reg, reg)
	g.write("%s_old_size = mul i64 %d, %s_old_len", reg, elemSize, reg)

	// Compute new size in bytes.
	g.Gen(call.Args[1])
	newLenReg := g.lookupCode(call.Args[1])
	g.write("%s_new_size = mul i64 %d, %s", reg, elemSize, newLenReg)

	// Call arena_realloc.
	g.write("%s_data = call ptr @runtime$arena.arena_realloc(ptr %s, ptr %s_old_ptr, i64 %s_old_size, i64 %s_new_size)",
		reg, allocReg, reg, reg, reg)

	// Initialize only the new elements [old_len..new_len) with default if provided.
	if len(call.Args) == 3 { // grow has a default value arg; grow_uninit doesn't
		g.Gen(call.Args[2])
		valReg := g.lookupCode(call.Args[2])
		fillCountReg := g.reg()
		g.write("%s = sub i64 %s, %s_old_len", fillCountReg, newLenReg, reg)
		fillStartReg := g.reg()
		g.write("%s = getelementptr %s, ptr %s_data, i64 %s_old_len", fillStartReg, irTyp, reg, reg)
		g.genInitializeMemory(fillStartReg, irTyp, valReg, sliceType.Elem, fillCountReg, nil)
	}

	// Build result slice {ptr, i64}.
	g.writeAlloca(reg, "{ptr, i64}")
	g.write("%s_ptr_field = getelementptr {ptr, i64}, ptr %s, i32 0, i32 0", reg, reg)
	g.write("store ptr %s_data, ptr %s_ptr_field", reg, reg)
	g.write("%s_len_field = getelementptr {ptr, i64}, ptr %s, i32 0, i32 1", reg, reg)
	g.write("store i64 %s, ptr %s_len_field", newLenReg, reg)
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
	if _, ok := targetType.Kind.(types.ModuleType); ok {
		if name, ok := g.env.NamedFunRef(id); ok {
			g.emitFunValue(id, name)
			return
		}
		if b, ok := g.env.PathBinding(id); ok {
			if symbol, ok := g.symbols[b.ID]; ok {
				pathType := g.typeOfNode(id)
				if g.isAggregateType(pathType.ID) {
					g.setCode(id, symbol.Reg)
					return
				}
				ptrreg := g.reg()
				g.write("%s = load %s, ptr %s", ptrreg, symbol.Type, symbol.Reg)
				g.setCode(id, ptrreg)
				return
			}
		}
		// Module member used as type reference (no codegen needed).
		return
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
	isClosure := len(astFun.Captures) > 0
	isRetAggregate := g.isAggregateType(fun.Return)
	retIRTyp := g.irType(fun.Return)
	signatureIRTyp := retIRTyp
	params := strings.Builder{}
	if isClosure {
		params.WriteString("ptr %__ctx")
	}
	if isMain {
		signatureIRTyp = "i32"
		if params.Len() > 0 {
			params.WriteString(", ")
		}
		params.WriteString("i32 %argc, ptr %argv")
	} else if isRetAggregate {
		signatureIRTyp = "void"
		if params.Len() > 0 {
			params.WriteString(", ")
		}
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
			g.setSymbol(ast.BindingID(paramNodeID), param.Name.Name, preg, "ptr")
		} else if g.isAggregateType(paramTyp.ID) {
			// Aggregate param: byval gives us a ptr to the callee's copy directly.
			fmt.Fprintf(&params, "ptr byval(%s) ", paramIRTyp)
			params.WriteString(preg)
			g.setSymbol(ast.BindingID(paramNodeID), param.Name.Name, preg, "ptr")
		} else {
			params.WriteString(paramIRTyp)
			params.WriteString(" ")
			params.WriteString(preg)
			areg := g.reg()
			fmt.Fprintf(&paramAllocas, "%s = alloca %s\n", areg, paramIRTyp)
			fmt.Fprintf(&paramAllocas, "store %s %s, ptr %s\n", paramIRTyp, preg, areg)
			g.setSymbol(ast.BindingID(paramNodeID), param.Name.Name, areg, paramIRTyp)
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
	g.write("%s = alloca %s", g.funRetReg, retIRTyp)
	if len(astFun.Params) > 0 {
		g.write(paramAllocas.String())
	}
	// Load captures from the capture context struct.
	if isClosure {
		ctxType := g.closureCtxType(astFun)
		for i, capNodeID := range astFun.Captures {
			capture := base.Cast[ast.Capture](g.ast.Node(capNodeID).Kind)
			bindID := ast.BindingID(capNodeID)
			capTypeID, _ := g.env.BindingType(bindID)
			capIRTyp := g.irType(capTypeID)
			gepReg := g.reg()
			g.write("%s = getelementptr %s, ptr %%__ctx, i32 0, i32 %d", gepReg, ctxType, i)
			capTyp := g.env.Type(capTypeID)
			if g.isAggregateType(capTypeID) {
				g.symbols[bindID] = Symbol{Name: capture.Name.Name, Reg: gepReg, Type: "ptr"}
			} else if _, isAlloc := capTyp.Kind.(types.AllocatorType); isAlloc {
				// Allocator captures: load the arena pointer and use it directly.
				valReg := g.reg()
				g.write("%s = load ptr, ptr %s", valReg, gepReg)
				g.symbols[bindID] = Symbol{Name: capture.Name.Name, Reg: valReg, Type: "ptr"}
			} else {
				valReg := g.reg()
				g.write("%s = load %s, ptr %s", valReg, capIRTyp, gepReg)
				allocReg := g.reg()
				g.write("%s = alloca %s", allocReg, capIRTyp)
				g.write("store %s %s, ptr %s", capIRTyp, valReg, allocReg)
				g.symbols[bindID] = Symbol{Name: capture.Name.Name, Reg: allocReg, Type: capIRTyp}
			}
		}
	}
	if isMain {
		g.write("call void @__os_args_init(i32 %argc, ptr %argv)")
		g.write("call void @__const_init()")
	}
	// Record where to insert entry-block allocas that are collected during
	// body code generation.
	entryAllocaInsertPos := g.sb.Len()
	g.Gen(astFun.Block)
	if g.entryAllocas.Len() > 0 {
		ir := g.sb.String()
		g.sb.Reset()
		g.sb.WriteString(ir[:entryAllocaInsertPos])
		g.sb.WriteString(g.entryAllocas.String())
		g.sb.WriteString(ir[entryAllocaInsertPos:])
	}
	// Write the result of the block into the ret reg.
	lastCode := g.lookupCode(astFun.Block)
	if !g.breaksControlFlow(astFun.Block) {
		g.storeValue(lastCode, g.funRetReg, fun.Return)
		g.write("br label %%%s", g.funRetLabel)
	}
	g.writeLabel(g.funRetLabel)
	switch {
	case isMain:
		if _, isUnion := g.env.Type(fun.Return).Kind.(types.UnionType); isUnion {
			exitCode := g.reg()
			g.write("%s = call i32 @__main_check_result(ptr %s)", exitCode, g.funRetReg)
			g.write("ret i32 %s", exitCode)
		} else {
			g.write("ret i32 0")
		}
	case isRetAggregate:
		resReg := g.reg()
		g.write("%s = load %s, ptr %s", resReg, retIRTyp, g.funRetReg)
		g.write("store %s %s, ptr %%out_ptr", retIRTyp, resReg)
		g.write("ret void")
	default:
		resReg := g.reg()
		g.write("%s = load %s, ptr %s", resReg, retIRTyp, g.funRetReg)
		g.write("ret %s %s", retIRTyp, resReg)
	}
	g.indent--
	g.write("}\n")
}

func (g *IRFunGen) genReturn(id ast.NodeID, return_ ast.Return) {
	g.Gen(return_.Expr)
	exprReg := g.lookupCode(return_.Expr)
	retTyp := g.typeIDOfNode(return_.Expr)
	g.storeValue(exprReg, g.funRetReg, retTyp)
	g.emitAllBlockCleanups(0)
	g.write("br label %%%s", g.funRetLabel)
	g.setCode(id, exprReg)
}

// emitBlockCleanup emits defer blocks (reverse order) and arena destroys for
// a single stack level.
func (g *IRFunGen) emitBlockCleanup(arenaLevel, deferLevel int) {
	defers := g.deferStack[deferLevel]
	for i := len(defers) - 1; i >= 0; i-- {
		// Use a fresh astCode so that re-emitting the same defer block at
		// multiple exit points (normal exit + break/continue) doesn't conflict.
		saved := g.astCode
		g.astCode = map[ast.NodeID]string{}
		g.Gen(defers[i])
		g.astCode = saved
	}
	for _, reg := range g.arenaRegStack[arenaLevel] {
		g.write("call i64 @runtime$arena.arena_destroy(ptr %s)", reg)
	}
}

// emitAllBlockCleanups emits defers and arena destroys for all enclosing block
// scopes down to (and including) the given base level. Innermost first.
// Called before return/break/continue branches.
func (g *IRFunGen) emitAllBlockCleanups(base int) {
	for i := len(g.arenaRegStack) - 1; i >= base; i-- {
		g.emitBlockCleanup(i, i)
	}
}

func (g *IRFunGen) genBlock(id ast.NodeID, block ast.Block) {
	g.arenaRegStack = append(g.arenaRegStack, nil)
	g.deferStack = append(g.deferStack, nil)
	for _, expr := range block.Exprs {
		if d, ok := g.ast.Node(expr).Kind.(ast.Defer); ok {
			top := len(g.deferStack) - 1
			g.deferStack[top] = append(g.deferStack[top], d.Block)
			g.setCode(expr, voidValue)
		} else {
			g.Gen(expr)
		}
	}
	// Emit defers and arena destroys unless the block unconditionally branched
	// away (return/break/continue/never), which already emitted cleanup or doesn't need it.
	if !g.breaksControlFlow(id) {
		g.emitBlockCleanup(len(g.arenaRegStack)-1, len(g.deferStack)-1)
	}
	g.arenaRegStack = g.arenaRegStack[:len(g.arenaRegStack)-1]
	g.deferStack = g.deferStack[:len(g.deferStack)-1]
	code := voidValue
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
	g.loopStack = append(g.loopStack, LoopLabels{labelStart, labelEnd, len(g.arenaRegStack)})
	defer func() { g.loopStack = g.loopStack[:len(g.loopStack)-1] }()
	g.Gen(forNode.Body)
	g.write("br label %%%s", labelStart)
	g.writeLabel(labelEnd)
	g.setCode(id, voidValue)
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
	g.writeAlloca(counterReg, "i64")
	g.write("store i64 %s, ptr %s", loReg, counterReg)
	g.setSymbol(ast.BindingID(forNode.Body), forNode.Binding.Name, counterReg, "i64")
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
	g.loopStack = append(g.loopStack, LoopLabels{labelIncr, labelEnd, len(g.arenaRegStack)})
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
	g.setCode(id, voidValue)
}

func (g *IRFunGen) genBreak(id ast.NodeID) {
	loopLabel := g.loopStack[len(g.loopStack)-1]
	g.emitAllBlockCleanups(loopLabel.arenaStackBase)
	g.write("br label %%%s", loopLabel.break_)
	g.setCode(id, voidValue)
}

func (g *IRFunGen) genContinue(id ast.NodeID) {
	loopLabel := g.loopStack[len(g.loopStack)-1]
	g.emitAllBlockCleanups(loopLabel.arenaStackBase)
	g.write("br label %%%s", loopLabel.continue_)
	g.setCode(id, voidValue)
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
	if !g.breaksControlFlow(ifNode.Then) {
		g.write("br label %%%s", contLabel)
	}
	if ifNode.Else != nil {
		g.writeLabel(elseLabel)
		g.Gen(*ifNode.Else)
		if !g.breaksControlFlow(*ifNode.Else) {
			g.write("br label %%%s", contLabel)
		}
	}
	phiElseLabel := g.lastLabel
	g.writeLabel(contLabel)
	code := voidValue
	if ifNode.Else != nil {
		phi := g.reg()
		thenCode := g.lookupCode(ifNode.Then)
		elseCode := g.lookupCode(*ifNode.Else)
		thenType := g.typeOfNode(ifNode.Then)
		typ := g.irType(thenType.ID)
		if g.isAggregateType(thenType.ID) {
			typ = "ptr" // Aggregate values flow as pointers in code registers.
		}
		if typ != "{}" && typ != "void" {
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

func (g *IRFunGen) genWhen(id ast.NodeID, when ast.When) { //nolint:funlen
	caseLabels := make([]Label, len(when.Cases))
	nextLabels := make([]Label, len(when.Cases))
	for i := range when.Cases {
		caseLabels[i] = g.label(fmt.Sprintf("when_case_%d", i), id)
		if i+1 < len(when.Cases) {
			nextLabels[i] = g.label(fmt.Sprintf("when_next_%d", i), id)
		}
	}
	contLabel := g.label("endwhen", id)
	elseLabel := contLabel
	if when.Else != nil {
		elseLabel = g.label("when_else", id)
	}
	for i, case_ := range when.Cases {
		g.Gen(case_.Cond)
		cond := g.lookupCode(case_.Cond)
		targetFalse := elseLabel
		if i+1 < len(when.Cases) {
			targetFalse = nextLabels[i]
		}
		g.write("br i1 %s, label %%%s, label %%%s", cond, caseLabels[i], targetFalse)
		g.writeLabel(caseLabels[i])
		g.Gen(case_.Body)
		caseLabels[i] = g.lastLabel
		if !g.breaksControlFlow(case_.Body) {
			g.write("br label %%%s", contLabel)
		}
		if i+1 < len(when.Cases) {
			g.writeLabel(nextLabels[i])
		}
	}
	phiLabels := make([]Label, 0, len(when.Cases)+1)
	phiNodes := make([]ast.NodeID, 0, len(when.Cases)+1)
	for i, case_ := range when.Cases {
		if g.breaksControlFlow(case_.Body) {
			continue
		}
		phiLabels = append(phiLabels, caseLabels[i])
		phiNodes = append(phiNodes, case_.Body)
	}
	if when.Else != nil {
		g.writeLabel(elseLabel)
		g.Gen(*when.Else)
		phiElseLabel := g.lastLabel
		if !g.breaksControlFlow(*when.Else) {
			g.write("br label %%%s", contLabel)
			phiLabels = append(phiLabels, phiElseLabel)
			phiNodes = append(phiNodes, *when.Else)
		}
	}
	g.writeLabel(contLabel)
	code := voidValue
	if len(phiNodes) > 0 {
		firstType := g.typeOfNode(phiNodes[0])
		typ := g.irType(firstType.ID)
		if g.isAggregateType(firstType.ID) {
			typ = "ptr"
		}
		if typ != "{}" && typ != "void" {
			phi := g.reg()
			parts := make([]string, len(phiNodes))
			for i, nodeID := range phiNodes {
				parts[i] = fmt.Sprintf("[%s, %%%s]", g.lookupCode(nodeID), phiLabels[i])
			}
			g.write("%s = phi %s %s", phi, typ, strings.Join(parts, ", "))
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
		if !g.breaksControlFlow(arm.Body) {
			g.write("br label %%%s", contLabel)
			phiEntries = append(phiEntries, phiEntry{g.lookupCode(arm.Body), g.lastLabel})
		}
	}
	if match.Else != nil {
		g.writeLabel(elseLabel)
		elseBody := base.Cast[ast.Block](g.ast.Node(match.Else.Body).Kind)
		if match.Else.Binding != nil && len(elseBody.Exprs) > 0 {
			g.genMatchElseBinding(match, payloadPtr)
		}
		g.Gen(match.Else.Body)
		if !g.breaksControlFlow(match.Else.Body) {
			g.write("br label %%%s", contLabel)
			phiEntries = append(phiEntries, phiEntry{g.lookupCode(match.Else.Body), g.lastLabel})
		}
	}
	g.writeLabel(contLabel)
	if len(phiEntries) == 0 {
		g.write("unreachable")
		g.setCode(id, voidValue)
		return
	}
	if resultIRType == "{}" || resultIRType == "void" {
		g.setCode(id, voidValue)
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
	g.genMatchBinding(ast.BindingID(arm.Body), arm.Binding.Name, g.typeIDOfNode(arm.Pattern), payloadPtr)
}

func (g *IRFunGen) genMatchElseBinding(match ast.Match, payloadPtr string) {
	bindID := ast.BindingID(match.Else.Body)
	bindTypeID, _ := g.env.BindingType(bindID)
	if bindTypeID == g.typeIDOfNode(match.Expr) {
		g.setSymbol(bindID, match.Else.Binding.Name, g.lookupCode(match.Expr), "ptr")
		return
	}
	g.genMatchBinding(bindID, match.Else.Binding.Name, bindTypeID, payloadPtr)
}

func (g *IRFunGen) genMatchBinding(bindID ast.BindingID, name string, typeID types.TypeID, ptr string) {
	valReg := g.loadValue(ptr, typeID)
	irTyp := g.irType(typeID)
	if irTyp == "{}" || g.isAggregateType(typeID) {
		g.setSymbol(bindID, name, valReg, "ptr")
		return
	}
	allocReg := g.reg()
	g.writeAlloca(allocReg, irTyp)
	g.write("store %s %s, ptr %s", irTyp, valReg, allocReg)
	g.setSymbol(bindID, name, allocReg, irTyp)
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
	// `_ = expr` discards the result.
	if ident, ok := g.ast.Node(assign.LHS).Kind.(ast.Ident); ok && ident.Name == "_" {
		g.setCode(id, voidValue)
		return
	}
	rhs := g.lookupCode(assign.RHS)
	ptrReg := g.genPlaceAddr(assign.LHS)
	rhsTypeID := g.typeIDOfNode(assign.RHS)
	g.storeValue(rhs, ptrReg, rhsTypeID)
	g.setCode(id, voidValue)
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

// emitCheckedArithmeticOp emits a call to a checked add/sub/mul builtin
// that panics on overflow if `opts.ArithmeticOverflowCheck` is set.
func (g *IRFunGen) emitCheckedArithmeticOp(id ast.NodeID, reg, irTyp, op, lhs, rhs string, signed bool) {
	if !g.opts.ArithmeticOverflowCheck {
		g.write("%s = %s %s %s, %s", reg, op, irTyp, lhs, rhs)
		return
	}
	prefix := "s"
	if !signed {
		prefix = "u"
	}
	span := g.ast.Node(id).Span
	locReg := g.addStrConst(span.String())
	fn := fmt.Sprintf("@__checked_%s%s_%s", prefix, op, irTyp)
	g.write("%s = call %s %s(%s %s, %s %s, ptr %s)", reg, irTyp, fn, irTyp, lhs, irTyp, rhs, locReg)
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
	if binary.Op == ast.BinaryOpAnd || binary.Op == ast.BinaryOpOr {
		g.genShortCircuit(id, binary)
		return
	}
	g.Gen(binary.LHS)
	g.Gen(binary.RHS)
	lhs := g.lookupCode(binary.LHS)
	rhs := g.lookupCode(binary.RHS)
	irTyp := g.irTypeOfNode(binary.LHS)
	intTyp, isInt := g.typeOfNode(binary.LHS).Kind.(types.IntType)
	signed := isInt && intTyp.Signed
	reg := g.reg()
	switch binary.Op { //nolint:exhaustive
	case ast.BinaryOpAdd:
		g.emitCheckedArithmeticOp(id, reg, irTyp, "add", lhs, rhs, signed)
	case ast.BinaryOpSub:
		g.emitCheckedArithmeticOp(id, reg, irTyp, "sub", lhs, rhs, signed)
	case ast.BinaryOpMul:
		g.emitCheckedArithmeticOp(id, reg, irTyp, "mul", lhs, rhs, signed)
	case ast.BinaryOpWrapAdd:
		g.write("%s = add %s %s, %s", reg, irTyp, lhs, rhs)
	case ast.BinaryOpWrapSub:
		g.write("%s = sub %s %s, %s", reg, irTyp, lhs, rhs)
	case ast.BinaryOpWrapMul:
		g.write("%s = mul %s %s, %s", reg, irTyp, lhs, rhs)
	case ast.BinaryOpDiv:
		divOp := "sdiv"
		if !signed {
			divOp = "udiv"
		}
		g.emitSafeIntOp(id, reg, irTyp, divOp, lhs, rhs)
	case ast.BinaryOpMod:
		remOp := "srem"
		if !signed {
			remOp = "urem"
		}
		g.emitSafeIntOp(id, reg, irTyp, remOp, lhs, rhs)
	case ast.BinaryOpEq:
		g.write("%s = icmp eq %s %s, %s", reg, irTyp, lhs, rhs)
	case ast.BinaryOpNeq:
		g.write("%s = icmp ne %s %s, %s", reg, irTyp, lhs, rhs)
	case ast.BinaryOpLt, ast.BinaryOpLte, ast.BinaryOpGt, ast.BinaryOpGte:
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
		if !signed {
			shrOp = "lshr"
		}
		g.write("%s = %s %s %s, %s", reg, shrOp, irTyp, lhs, rhs)
	default:
		panic(base.Errorf("unknown binary operator: %s", binary.Op))
	}
	switch binary.Op { //nolint:exhaustive
	case ast.BinaryOpAdd, ast.BinaryOpSub, ast.BinaryOpDiv, ast.BinaryOpMul, ast.BinaryOpMod,
		ast.BinaryOpWrapAdd, ast.BinaryOpWrapSub, ast.BinaryOpWrapMul,
		ast.BinaryOpBitAnd, ast.BinaryOpBitOr, ast.BinaryOpBitXor, ast.BinaryOpShl, ast.BinaryOpShr:
		g.runeCheckIfNeeded(binary.LHS, reg)
	}
	g.setCode(id, reg)
}

func (g *IRFunGen) genShortCircuit(id ast.NodeID, binary ast.Binary) {
	g.Gen(binary.LHS)
	lhs := g.lookupCode(binary.LHS)
	lhsLabel := g.label("sc_lhs", id)
	rhsLabel := g.label("sc_rhs", id)
	endLabel := g.label("sc_end", id)
	g.write("br label %%%s", lhsLabel)
	g.writeLabel(lhsLabel)
	if binary.Op == ast.BinaryOpAnd {
		g.write("br i1 %s, label %%%s, label %%%s", lhs, rhsLabel, endLabel)
	} else {
		g.write("br i1 %s, label %%%s, label %%%s", lhs, endLabel, rhsLabel)
	}
	g.writeLabel(rhsLabel)
	g.Gen(binary.RHS)
	rhs := g.lookupCode(binary.RHS)
	rhsEndLabel := g.lastLabel
	g.write("br label %%%s", endLabel)
	g.writeLabel(endLabel)
	reg := g.reg()
	if binary.Op == ast.BinaryOpAnd {
		g.write("%s = phi i1 [false, %%%s], [%s, %%%s]", reg, lhsLabel, rhs, rhsEndLabel)
	} else {
		g.write("%s = phi i1 [true, %%%s], [%s, %%%s]", reg, lhsLabel, rhs, rhsEndLabel)
	}
	g.setCode(id, reg)
}

func (g *IRFunGen) arenaAllocMethod(call ast.Call) (string, ast.FieldAccess, bool) {
	name, ok := g.env.NamedFunRef(call.Callee)
	if !ok {
		return "", ast.FieldAccess{}, false
	}
	for _, method := range []string{
		"Arena.new",
		"Arena.slice_uninit",
		"Arena.slice",
		"Arena.grow_uninit",
		"Arena.grow",
	} {
		if strings.HasPrefix(name, method) {
			fa := base.Cast[ast.FieldAccess](g.ast.Node(call.Callee).Kind)
			return method, fa, true
		}
	}
	return "", ast.FieldAccess{}, false
}

func (g *IRFunGen) genBuiltinFun(id ast.NodeID, call ast.Call, span base.Span) bool { //nolint:funlen
	ref, ok := g.env.NamedFunRef(call.Callee)
	if !ok {
		return false
	}
	name := types.BuiltinName(ref)
	switch name {
	case "ffi::sizeof":
		kind := base.Cast[ast.FieldAccess](g.ast.Node(call.Callee).Kind)
		argTypeID := g.typeIDOfNode(kind.TypeArgs[0])
		size := irTypeSize(g.env, argTypeID)
		g.setCode(id, "%d", size)
		return true
	case "ffi::Ptr.as_u64", "ffi::PtrMut.as_u64":
		receiver, ok := g.env.MethodCallReceiver(id)
		if !ok {
			panic(fmt.Sprintf("expected a method call receiver for %s", name))
		}
		g.Gen(receiver)
		ptrReg := g.lookupCode(receiver)
		reg := g.reg()
		g.write("%s = ptrtoint ptr %s to i64", reg, ptrReg)
		g.setCode(id, reg)
		return true
	case "ffi::Ptr.is_null", "ffi::PtrMut.is_null":
		receiver, ok := g.env.MethodCallReceiver(id)
		if !ok {
			panic(fmt.Sprintf("expected a method call receiver for %s", name))
		}
		g.Gen(receiver)
		ptrReg := g.lookupCode(receiver)
		reg := g.reg()
		g.write("%s = icmp eq ptr %s, null", reg, ptrReg)
		g.setCode(id, reg)
		return true
	case "ffi::Ptr.null", "ffi::PtrMut.null":
		g.setCode(id, "null")
		return true
	case "ffi::Ptr.cast", "ffi::PtrMut.cast":
		receiver, ok := g.env.MethodCallReceiver(id)
		if !ok {
			panic(fmt.Sprintf("expected a method call receiver for %s", name))
		}
		g.Gen(receiver)
		g.setCode(id, g.lookupCode(receiver))
		return true
	case "ffi::ref_ptr", "ffi::ref_ptr_mut":
		g.Gen(call.Args[0])
		g.setCode(id, g.lookupCode(call.Args[0]))
		return true
	case "ffi::PtrMut.as_ptr":
		receiver, ok := g.env.MethodCallReceiver(id)
		if !ok {
			panic(fmt.Sprintf("expected a method call receiver for %s", name))
		}
		g.Gen(receiver)
		g.setCode(id, g.lookupCode(receiver))
		return true
	case "ffi::slice_ptr", "ffi::slice_ptr_mut":
		g.Gen(call.Args[0])
		sliceReg := g.lookupCode(call.Args[0])
		dataPtrReg := g.reg()
		g.write("%s = getelementptr {ptr, i64}, ptr %s, i32 0, i32 0", dataPtrReg, sliceReg)
		ptrReg := g.reg()
		g.write("%s = load ptr, ptr %s", ptrReg, dataPtrReg)
		g.setCode(id, ptrReg)
		return true
	case "ffi::Ptr.read", "ffi::PtrMut.read":
		receiver, ok := g.env.MethodCallReceiver(id)
		if !ok {
			panic(fmt.Sprintf("expected a method call receiver for %s", name))
		}
		g.Gen(receiver)
		ptrReg := g.lookupCode(receiver)
		retTypeID := base.Cast[types.FunType](g.env.Type(g.typeIDOfNode(call.Callee)).Kind).Return
		g.setCode(id, g.loadValue(ptrReg, retTypeID))
		return true
	case "ffi::Ptr.as_slice", "ffi::PtrMut.as_slice":
		receiver, ok := g.env.MethodCallReceiver(id)
		if !ok {
			panic(fmt.Sprintf("expected a method call receiver for %s", name))
		}
		g.Gen(receiver)
		ptrReg := g.lookupCode(receiver)
		g.Gen(call.Args[0])
		lenReg := g.lookupCode(call.Args[0])
		// Build a slice {ptr, i64} on the stack.
		sliceReg := g.reg()
		g.writeAlloca(sliceReg, "{ptr, i64}")
		ptrField := g.reg()
		g.write("%s = getelementptr {ptr, i64}, ptr %s, i32 0, i32 0", ptrField, sliceReg)
		g.write("store ptr %s, ptr %s", ptrReg, ptrField)
		lenField := g.reg()
		g.write("%s = getelementptr {ptr, i64}, ptr %s, i32 0, i32 1", lenField, sliceReg)
		g.write("store i64 %s, ptr %s", lenReg, lenField)
		g.setCode(id, sliceReg)
		return true
	case "ffi::Ptr.offset", "ffi::PtrMut.offset":
		receiver, ok := g.env.MethodCallReceiver(id)
		if !ok {
			panic(fmt.Sprintf("expected a method call receiver for %s", name))
		}
		g.Gen(receiver)
		ptrReg := g.lookupCode(receiver)
		g.Gen(call.Args[0])
		offsetReg := g.lookupCode(call.Args[0])
		// GEP with the element type to get sizeof(T)-strided pointer arithmetic.
		retTypeID := base.Cast[types.FunType](g.env.Type(g.typeIDOfNode(call.Callee)).Kind).Return
		// Return type is Ptr<T>/PtrMut<T> — extract T's TypeArgs[0].
		ptrStruct := base.Cast[types.StructType](g.env.Type(retTypeID).Kind)
		elemIR := irType(g.env, ptrStruct.TypeArgs[0])
		reg := g.reg()
		g.write("%s = getelementptr %s, ptr %s, i64 %s", reg, elemIR, ptrReg, offsetReg)
		g.setCode(id, reg)
		return true
	case "ffi::fun_ptr":
		// fun_ptr(f) takes a fun() void ({ptr, ptr}) and returns FunPtr ({ptr, ptr}).
		// Same layout -- just pass through the pointer to the aggregate.
		g.Gen(call.Args[0])
		g.setCode(id, g.lookupCode(call.Args[0]))
		return true
	case "ffi::fun_ptr_alloc":
		// fun_ptr_alloc(@a, f) copies the closure context to the arena.
		// See emitClosureValue for context layout and __fun_ptr_ctx_copy in builtins.ll.
		g.Gen(call.Args[0])
		arenaReg := g.lookupCode(call.Args[0])
		g.Gen(call.Args[1])
		funReg := g.lookupCode(call.Args[1])
		// Extract data_ptr (field 1 of the {ptr, ptr} fat pointer).
		dataField := g.reg()
		g.write("%s = getelementptr {ptr, ptr}, ptr %s, i32 0, i32 1", dataField, funReg)
		dataPtr := g.reg()
		g.write("%s = load ptr, ptr %s", dataPtr, dataField)
		newCtx := g.reg()
		g.write("%s = call ptr @__fun_ptr_ctx_copy(ptr %s, ptr %s)", newCtx, arenaReg, dataPtr)
		// Copy the input FunPtr and replace the data pointer.
		g.write("store ptr %s, ptr %s", newCtx, dataField)
		g.setCode(id, funReg)
		return true
	case "ffi::FunPtr.call":
		// FunPtr.call(f) extracts fn ptr and data ptr, calls fn(data).
		receiver, ok := g.env.MethodCallReceiver(id)
		if !ok {
			panic(fmt.Sprintf("expected a method call receiver for %s", name))
		}
		g.Gen(receiver)
		funPtrReg := g.lookupCode(receiver)
		fnField := g.reg()
		g.write("%s = getelementptr {ptr, ptr}, ptr %s, i32 0, i32 0", fnField, funPtrReg)
		fnPtr := g.reg()
		g.write("%s = load ptr, ptr %s", fnPtr, fnField)
		dataField := g.reg()
		g.write("%s = getelementptr {ptr, ptr}, ptr %s, i32 0, i32 1", dataField, funPtrReg)
		dataPtr := g.reg()
		g.write("%s = load ptr, ptr %s", dataPtr, dataField)
		g.write("call void %s(ptr %s)", fnPtr, dataPtr)
		g.setCode(id, voidValue)
		return true
	case "ffi::PtrMut.write":
		receiver, ok := g.env.MethodCallReceiver(id)
		if !ok {
			panic(fmt.Sprintf("expected a method call receiver for %s", name))
		}
		g.Gen(receiver)
		ptrReg := g.lookupCode(receiver)
		g.Gen(call.Args[0])
		valReg := g.lookupCode(call.Args[0])
		valTypeID := g.typeIDOfNode(call.Args[0])
		g.storeValue(valReg, ptrReg, valTypeID)
		g.setCode(id, voidValue)
		return true
	case "os::args":
		reg := g.reg()
		g.writeAlloca(reg, "{ptr, i64}")
		valReg := g.reg()
		g.write("%s = load {ptr, i64}, ptr @__os_args", valReg)
		g.write("store {ptr, i64} %s, ptr %s", valReg, reg)
		g.setCode(id, reg)
		return true
	}
	if method, fa, ok := g.arenaAllocMethod(call); ok {
		switch method {
		case "Arena.new":
			g.genArenaNew(id, call, fa)
		case "Arena.grow", "Arena.grow_uninit":
			g.genArenaGrow(id, call, fa)
		default:
			g.genArenaSlice(id, call, fa)
		}
		return true
	}
	if ref == "panic" {
		g.Gen(call.Args[0])
		arg1Reg := g.lookupCode(call.Args[0])
		locReg := g.addStrConst(span.String())
		g.write("call void @panic(ptr %s, ptr %s)", arg1Reg, locReg)
		g.setCode(id, voidValue)
		return true
	}
	if ref == "std::debug.location" {
		locReg := g.addStrConst(span.String())
		g.setCode(id, locReg)
		return true
	}
	return false
}

func (g *IRFunGen) genCall(id ast.NodeID, call ast.Call, span base.Span) { //nolint:funlen
	if g.genBuiltinFun(id, call, span) {
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
	if defaults, ok := g.env.CallDefaults(id); ok {
		argNodes = append(argNodes, defaults...)
	}
	for _, nodeID := range argNodes {
		if _, ok := g.astCode[nodeID]; !ok {
			g.Gen(nodeID)
		}
	}
	if _, isDirect := g.env.NamedFunRef(call.Callee); !isDirect {
		g.genIndirectCall(id, call, fun, argNodes)
		return
	}
	sb := strings.Builder{}
	funName, _ := g.env.NamedFunRef(call.Callee)
	isRetAggregate := g.isAggregateType(fun.Return)
	retIR := g.irReturnType(fun.Return)
	var resReg string
	switch {
	case isRetAggregate:
		resReg = g.reg()
		g.writeAlloca(resReg, g.irType(fun.Return))
		g.setCode(id, resReg)
	case retIR == "void":
		g.setCode(id, voidValue)
	default:
		reg := g.reg()
		sb.WriteString(reg + " = ")
		g.setCode(id, reg)
	}
	sb.WriteString("call ")
	if isRetAggregate {
		sb.WriteString("void")
	} else {
		sb.WriteString(retIR)
	}
	fmt.Fprintf(&sb, " @%s", irName(funName))
	sb.WriteString("(")
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

// genIndirectCall emits an indirect call through a fat pointer {fn, ctx}.
// Always passes ctx as first arg — for non-capturing functions, fn points to
// a wrapper that accepts and ignores ctx.
func (g *IRFunGen) genIndirectCall(id ast.NodeID, call ast.Call, fun types.FunType, argNodes []ast.NodeID) {
	g.Gen(call.Callee)
	calleeReg := g.lookupCode(call.Callee)
	fatReg := g.reg()
	g.write("%s = load {ptr, ptr}, ptr %s", fatReg, calleeReg)
	fnReg := g.reg()
	g.write("%s = extractvalue {ptr, ptr} %s, 0", fnReg, fatReg)
	ctxReg := g.reg()
	g.write("%s = extractvalue {ptr, ptr} %s, 1", ctxReg, fatReg)

	isRetAggregate := g.isAggregateType(fun.Return)
	retIRTyp := g.irType(fun.Return)

	var sb strings.Builder
	callRetType := retIRTyp
	if isRetAggregate {
		callRetType = "void"
	}

	var resReg string
	switch {
	case isRetAggregate:
		resReg = g.reg()
		g.writeAlloca(resReg, retIRTyp)
		g.setCode(id, resReg)
	default:
		resReg = g.reg()
		sb.WriteString(resReg + " = ")
		g.setCode(id, resReg)
	}

	fmt.Fprintf(&sb, "call %s %s(ptr %s", callRetType, fnReg, ctxReg)
	if isRetAggregate {
		fmt.Fprintf(&sb, ", ptr sret(%s) %s", retIRTyp, resReg)
	}
	for _, nodeID := range argNodes {
		typeID := g.typeIDOfNode(nodeID)
		reg := g.lookupCode(nodeID)
		if g.isAggregateType(typeID) {
			fmt.Fprintf(&sb, ", ptr byval(%s) %s", g.irType(typeID), reg)
		} else {
			fmt.Fprintf(&sb, ", %s %s", g.irType(typeID), reg)
		}
	}
	sb.WriteString(")")
	g.write(sb.String())
}

func (g *IRFunGen) genIdent(id ast.NodeID, ident ast.Ident) {
	if ident.Name == "void" {
		g.setCode(id, voidValue)
		return
	}
	// Named function reference — emit fat pointer.
	if name, ok := g.env.NamedFunRef(id); ok {
		// Check if this is a closure with captures.
		if declID, ok := g.env.FunDeclNode(id); ok {
			if fun, ok := g.ast.Node(declID).Kind.(ast.Fun); ok && len(fun.Captures) > 0 {
				g.emitClosureValue(id, name, fun)
				return
			}
		}
		g.emitFunValue(id, name)
		return
	}
	if symbol, ok := g.lookupSymbol(id); ok {
		identType := g.typeOfNode(id)
		if _, ok := identType.Kind.(types.AllocatorType); ok ||
			g.isAggregateType(identType.ID) ||
			g.irType(identType.ID) == "{}" {
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

// emitFunValue emits a fat pointer {wrapper, null} for a non-capturing function
// value. The wrapper accepts ptr %__ctx as first param (ignored) so that
// indirect calls can uniformly pass ctx.
func (g *IRFunGen) emitFunValue(id ast.NodeID, name string) {
	wrapperName := g.genFunValWrapperIfNeeded(id, name)
	reg := g.reg()
	g.writeAlloca(reg, "{ptr, ptr}")
	fnField := g.reg()
	g.write("%s = getelementptr {ptr, ptr}, ptr %s, i32 0, i32 0", fnField, reg)
	g.write("store ptr @%s, ptr %s", wrapperName, fnField)
	ctxField := g.reg()
	g.write("%s = getelementptr {ptr, ptr}, ptr %s, i32 0, i32 1", ctxField, reg)
	g.write("store ptr null, ptr %s", ctxField)
	g.setCode(id, reg)
}

// genFunValWrapperIfNeeded generates a wrapper for a non-capturing function so
// it can be called indirectly with the uniform {fn, ctx} convention. The wrapper
// accepts ptr %__ctx as first param (ignored) and forwards to the real function.
func (g *IRFunGen) genFunValWrapperIfNeeded(id ast.NodeID, name string) string {
	irN := irName(name)
	if tn, ok := g.funValWrappers[irN]; ok {
		return tn
	}
	wrapperName := "__fn_val_" + irN
	g.funValWrappers[irN] = wrapperName

	funType := base.Cast[types.FunType](g.typeOfNode(id).Kind)
	isRetAggregate := g.isAggregateType(funType.Return)
	retIRTyp := g.irType(funType.Return)

	var sig, fwd strings.Builder
	sig.WriteString("ptr %__ctx")
	hasFwd := false
	if isRetAggregate {
		sig.WriteString(", ")
		fmt.Fprintf(&sig, "ptr sret(%s) %%out_ptr", retIRTyp)
		fmt.Fprintf(&fwd, "ptr sret(%s) %%out_ptr", retIRTyp)
		hasFwd = true
	}
	for i, paramTypeID := range funType.Params {
		sig.WriteString(", ")
		if hasFwd {
			fwd.WriteString(", ")
		}
		pname := fmt.Sprintf("%%p%d", i)
		paramIRTyp := g.irType(paramTypeID)
		if g.isAggregateType(paramTypeID) {
			fmt.Fprintf(&sig, "ptr byval(%s) %s", paramIRTyp, pname)
			fmt.Fprintf(&fwd, "ptr byval(%s) %s", paramIRTyp, pname)
		} else {
			fmt.Fprintf(&sig, "%s %s", paramIRTyp, pname)
			fmt.Fprintf(&fwd, "%s %s", paramIRTyp, pname)
		}
		hasFwd = true
	}

	callRetType := retIRTyp
	if isRetAggregate {
		callRetType = "void"
	}

	var w strings.Builder
	fmt.Fprintf(&w, "define internal %s @%s(%s) alwaysinline {\n", callRetType, wrapperName, sig.String())
	if callRetType == "void" {
		fmt.Fprintf(&w, "  call void @%s(%s)\n", irN, fwd.String())
		w.WriteString("  ret void\n")
	} else {
		fmt.Fprintf(&w, "  %%r = call %s @%s(%s)\n", retIRTyp, irN, fwd.String())
		fmt.Fprintf(&w, "  ret %s %%r\n", retIRTyp)
	}
	w.WriteString("}\n\n")
	g.wrapperBuf.WriteString(w.String())
	return wrapperName
}

func (g *IRFunGen) closureCtxSize(fun ast.Fun) int64 {
	var total int64
	for _, capNodeID := range fun.Captures {
		capTypeID, _ := g.env.BindingType(ast.BindingID(capNodeID))
		total += irTypeSize(g.env, capTypeID)
	}
	return total
}

// closureCtxType returns the LLVM struct type for a closure's capture context,
// e.g. "{i64, ptr}" for two captures of those types.
func (g *IRFunGen) closureCtxType(fun ast.Fun) string {
	var sb strings.Builder
	sb.WriteString("{")
	for i, capNodeID := range fun.Captures {
		if i > 0 {
			sb.WriteString(", ")
		}
		capTypeID, _ := g.env.BindingType(ast.BindingID(capNodeID))
		sb.WriteString(g.irType(capTypeID))
	}
	sb.WriteString("}")
	return sb.String()
}

// emitClosureValue allocates the capture context, stores captures, and emits
// emitClosureValue builds a function value {fn_ptr, data_ptr} for a closure.
// The capture context is prefixed by an i64 with its total byte size.
func (g *IRFunGen) emitClosureValue(id ast.NodeID, name string, fun ast.Fun) {
	ctxType := g.closureCtxType(fun)
	wrapperType := fmt.Sprintf("{i64, %s}", ctxType)
	baseReg := g.reg()
	g.writeAlloca(baseReg, wrapperType)
	// Store total size at offset 0.
	ctxSize := g.closureCtxSize(fun)
	totalSize := 8 + ctxSize // i64 prefix + context
	sizeField := g.reg()
	g.write("%s = getelementptr %s, ptr %s, i32 0, i32 0", sizeField, wrapperType, baseReg)
	g.write("store i64 %d, ptr %s", totalSize, sizeField)
	// Context pointer is at offset 1 (past the i64).
	ctxReg := g.reg()
	g.write("%s = getelementptr %s, ptr %s, i32 0, i32 1", ctxReg, wrapperType, baseReg)

	for i, capNodeID := range fun.Captures {
		capture := base.Cast[ast.Capture](g.ast.Node(capNodeID).Kind)
		outerBinding, ok := g.env.CaptureOrigin(capNodeID)
		if !ok {
			panic(base.Errorf("capture %s not found in parent scope", capture.Name.Name))
		}
		sym, ok := g.symbols[outerBinding.ID]
		if !ok {
			panic(base.Errorf("capture %s has no symbol", capture.Name.Name))
		}
		gepReg := g.reg()
		g.write("%s = getelementptr %s, ptr %s, i32 0, i32 %d", gepReg, ctxType, ctxReg, i)
		switch capture.Mode {
		case ast.CaptureByRef, ast.CaptureByMutRef:
			g.write("store ptr %s, ptr %s", sym.Reg, gepReg)
		case ast.CaptureByValue:
			capTyp := g.env.Type(outerBinding.TypeID)
			if _, isAlloc := capTyp.Kind.(types.AllocatorType); isAlloc {
				g.write("store ptr %s, ptr %s", sym.Reg, gepReg)
			} else {
				capIRTyp := g.irType(outerBinding.TypeID)
				val := g.reg()
				g.write("%s = load %s, ptr %s", val, capIRTyp, sym.Reg)
				g.write("store %s %s, ptr %s", capIRTyp, val, gepReg)
			}
		}
	}

	reg := g.reg()
	g.writeAlloca(reg, "{ptr, ptr}")
	fnField := g.reg()
	g.write("%s = getelementptr {ptr, ptr}, ptr %s, i32 0, i32 0", fnField, reg)
	g.write("store ptr @%s, ptr %s", irName(name), fnField)
	ctxField := g.reg()
	g.write("%s = getelementptr {ptr, ptr}, ptr %s, i32 0, i32 1", ctxField, reg)
	g.write("store ptr %s, ptr %s", ctxReg, ctxField)
	g.setCode(id, reg)
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
	if name, ok := g.strConsts[s]; ok {
		return name
	}
	id := g.constCounter
	g.constCounter++
	name := fmt.Sprintf("@str.%d", id)
	g.strConsts[s] = name
	n := len(s)
	fmt.Fprintf(&g.constGlobals, "%s.data = private constant [%d x i8] c\"%s\"\n", name, n, s)
	fmt.Fprintf(&g.constGlobals, "%s = private constant %%Str { {ptr, i64} { ptr %s.data, i64 %d } }\n", name, name, n)
	return name
}

func (g *IRFunGen) genRef(id ast.NodeID, ref ast.Ref) {
	ptrReg := g.genPlaceAddr(ref.Target)
	g.setCode(id, ptrReg)
}

func (g *IRFunGen) genPlaceAddr(nodeID ast.NodeID) string {
	node := g.ast.Node(nodeID)
	switch kind := node.Kind.(type) {
	case ast.Ident:
		if symbol, ok := g.lookupSymbol(nodeID); ok {
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
	bufSize := g.opts.ArenaStackBufSize
	// ArenaAllocator struct hard coded, see `lib/runtime/arena.met`
	g.writeAlloca(reg, "{ptr, ptr, ptr, ptr, i64, i64, i1}") //nolint:dupword
	buf := g.reg()
	g.writeAlloca(buf, fmt.Sprintf("[%d x i8]", bufSize))
	debug := 0
	if g.opts.ArenaDebug {
		debug = 1
	}
	g.write(
		"call void @runtime$arena.arena_create(ptr %s, ptr %s, i64 %d, i64 %d, i64 %d, i1 %d)",
		reg,
		buf,
		bufSize,
		g.opts.ArenaPageMinSize,
		g.opts.ArenaPageMaxSize,
		debug,
	)
	top := len(g.arenaRegStack) - 1
	g.arenaRegStack[top] = append(g.arenaRegStack[top], reg)
	g.setCode(id, reg)
	g.setSymbol(ast.BindingID(id), alloc.Name.Name, reg, "ptr")
}

func (g *IRFunGen) genVar(id ast.NodeID, v ast.Var) {
	g.Gen(v.Expr)
	exprReg := g.lookupCode(v.Expr)
	exprTypeID := g.typeIDOfNode(v.Expr)
	if g.isAggregateType(exprTypeID) {
		exprNode := g.ast.Node(v.Expr)
		isAutoWrapped := false
		if _, _, ok := g.env.UnionWrap(v.Expr); ok {
			isAutoWrapped = true
		}
		switch exprNode.Kind.(type) {
		case ast.TypeConstruction, ast.ArrayLiteral, ast.EmptySlice, ast.SubSlice, ast.Call:
			g.setCode(id, exprReg)
			g.setSymbol(ast.BindingID(id), v.Name.Name, exprReg, "ptr")
		default:
			if isAutoWrapped {
				g.setCode(id, exprReg)
				g.setSymbol(ast.BindingID(id), v.Name.Name, exprReg, "ptr")
			} else {
				irTyp := g.irType(exprTypeID)
				reg := g.reg()
				g.writeAlloca(reg, irTyp)
				tmp := g.reg()
				g.write("%s = load %s, ptr %s", tmp, irTyp, exprReg)
				g.write("store %s %s, ptr %s", irTyp, tmp, reg)
				g.setCode(id, reg)
				g.setSymbol(ast.BindingID(id), v.Name.Name, reg, "ptr")
			}
		}
		return
	}
	reg := g.reg()
	typ := g.irType(exprTypeID)
	g.writeAlloca(reg, typ)
	g.write("store %s %s, ptr %s", typ, exprReg, reg)
	g.setCode(id, reg)
	g.setSymbol(ast.BindingID(id), v.Name.Name, reg, typ)
}

func (g *IRGen) reg() string {
	id := g.regCounter
	g.regCounter++
	return fmt.Sprintf("%%r%d", id)
}

func (g *IRFunGen) typeOfNode(nodeID ast.NodeID) *types.Type {
	if wrapTypeID, _, ok := g.env.UnionWrap(nodeID); ok {
		if _, generated := g.astCode[nodeID]; generated {
			return g.env.Type(wrapTypeID)
		}
	}
	typ := g.env.TypeOfNode(nodeID)
	// Strip the sync flag -- it's irrelevant for codegen.
	stripped := types.StripSyncFlag(typ.ID)
	if stripped != typ.ID {
		return g.env.Type(stripped)
	}
	return typ
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

// irReturnType returns the LLVM IR type for a function return type.
// Unlike irType, this maps void to "void" (not "{}") since LLVM
// requires `call void` and `declare void` for void-returning functions.
func (g *IRFunGen) irReturnType(typeID types.TypeID) string {
	return irReturnType(g.env, typeID)
}

func irReturnType(env *types.TypeEnv, typeID types.TypeID) string {
	if _, ok := env.Type(typeID).Kind.(types.VoidType); ok {
		return "void"
	}
	return irType(env, typeID)
}

func (g *IRFunGen) isAggregateType(typeID types.TypeID) bool {
	typ := g.env.Type(typeID)
	switch kind := typ.Kind.(type) {
	case types.StructType:
		return !types.IsBuiltinPtrStruct(kind)
	case types.UnionType:
		return true
	case types.ArrayType, types.SliceType:
		return true
	case types.FunType:
		return true
	default:
		return false
	}
}

func (g *IRFunGen) breaksControlFlow(nodeID ast.NodeID) bool {
	_, ok := g.typeOfNode(nodeID).Kind.(types.NeverType)
	return ok
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
		return "{}"
	case types.NeverType:
		return "void"
	case types.StructType:
		if kind.Name == "Str" {
			return "%Str"
		}
		if kind.Name == "CStr" {
			return "%CStr"
		}
		if types.IsBuiltinPtrStruct(kind) {
			return "ptr"
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
		return "{ptr, ptr}"
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
	case types.RefType, types.AllocatorType:
		return 8
	case types.FunType:
		return 8
	case types.StructType:
		if types.IsBuiltinPtrStruct(kind) {
			return 8
		}
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
	case types.RefType, types.AllocatorType:
		return 8
	case types.FunType:
		return 16
	case types.StructType:
		if types.IsBuiltinPtrStruct(kind) {
			return 8
		}
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

func (g *IRFunGen) setSymbol(id ast.BindingID, name string, reg string, typ string) {
	g.symbols[id] = Symbol{Name: name, Reg: reg, Type: typ}
}

func (g *IRFunGen) lookupSymbol(nodeID ast.NodeID) (Symbol, bool) {
	b, ok := g.env.PathBinding(nodeID)
	if !ok {
		return Symbol{}, false
	}
	symbol, ok := g.symbols[b.ID]
	return symbol, ok
}

func (g *IRFunGen) loadValue(ptrReg string, typeID types.TypeID) string {
	irTyp := g.irType(typeID)
	if irTyp == "{}" {
		return voidValue
	}
	if g.isAggregateType(typeID) {
		return ptrReg
	}
	reg := g.reg()
	g.write("%s = load %s, ptr %s", reg, irTyp, ptrReg)
	return reg
}

func (g *IRFunGen) storeValue(srcReg string, dstReg string, typeID types.TypeID) {
	irTyp := g.irType(typeID)
	if irTyp == "{}" {
		return
	}
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

// writeAlloca emits an alloca into the function's entry block.
func (g *IRFunGen) writeAlloca(reg, irTyp string) {
	fmt.Fprintf(&g.entryAllocas, "    %s = alloca %s\n", reg, irTyp)
}

type IROpts struct {
	TargetDataLayout        string
	TargetTriple            string
	ArithmeticOverflowCheck bool
	AddressSanitizer        bool
	ArenaDebug              bool
	ArenaStackBufSize       int
	ArenaPageMinSize        int
	ArenaPageMaxSize        int
}

func (g *IRGen) genExternDecls(funs []types.FunWork) {
	if len(funs) == 0 {
		return
	}
	env := funs[0].Env
	emitted := map[string]bool{
		// These are already declared in `builtins.ll`:
		"putchar":           true,
		"puts":              true,
		"printf":            true,
		"fflush":            true,
		"write":             true,
		"arena_debug_print": true,
		"strlen":            true,
		"malloc":            true,
	}
	g.ast.Iter(func(nodeID ast.NodeID) bool {
		funDecl, ok := g.ast.Node(nodeID).Kind.(ast.FunDecl)
		if !ok || !funDecl.Extern {
			return true
		}
		name := funDecl.ExternName
		if emitted[name] {
			return true
		}
		emitted[name] = true
		funTypeID := env.TypeOfNode(nodeID).ID
		funType := base.Cast[types.FunType](env.Type(funTypeID).Kind)
		retIR := irReturnType(env, funType.Return)
		params := strings.Builder{}
		for i, paramTypeID := range funType.Params {
			if i > 0 {
				params.WriteString(", ")
			}
			params.WriteString(irType(env, paramTypeID))
		}
		g.write("declare %s @%s(%s)", retIR, name, params.String())
		return true
	})
	g.write("")
}

func (g *IRGen) genModuleConsts(consts []types.ConstWork) {
	// Declare globals for each constant.
	if len(consts) == 0 {
		g.write("define internal void @__const_init() { ret void }")
		g.write("")
		return
	}
	for _, c := range consts {
		irTyp := irType(c.Env, c.TypeID)
		globalName := irName(c.Name)
		g.write("@%s = internal global %s zeroinitializer", globalName, irTyp)
	}
	// Generate init function that evaluates expressions and stores into globals.
	f := g.newFunGen(consts[0].Env)
	f.constInit = true
	f.write("define internal void @__const_init() {")
	f.indent++
	entryAllocaInsertPos := f.sb.Len()
	for _, c := range consts {
		varNode := base.Cast[ast.Var](f.ast.Node(c.NodeID).Kind)
		irTyp := irType(c.Env, c.TypeID)
		globalName := irName(c.Name)
		globalRef := fmt.Sprintf("@%s", globalName)
		f.Gen(varNode.Expr)
		exprReg := f.lookupCode(varNode.Expr)
		f.storeValue(exprReg, globalRef, c.TypeID)
		b, ok := c.Env.Lookup(c.NodeID, varNode.Name.Name, -1)
		if !ok {
			panic(base.Errorf("constant binding not found: %s", varNode.Name.Name))
		}
		g.symbols[b.ID] = Symbol{Name: varNode.Name.Name, Reg: globalRef, Type: irTyp}
	}
	if f.entryAllocas.Len() > 0 {
		ir := f.sb.String()
		f.sb.Reset()
		f.sb.WriteString(ir[:entryAllocaInsertPos])
		f.sb.WriteString(f.entryAllocas.String())
		f.sb.WriteString(ir[entryAllocaInsertPos:])
	}
	f.write("ret void")
	f.indent--
	f.write("}")
	f.write("")
	g.sb.WriteString(f.sb.String())
}

func GenIR(
	a *ast.AST,
	module ast.Module,
	funs []types.FunWork,
	structs []types.TypeWork,
	unions []types.TypeWork,
	consts []types.ConstWork,
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
	g.write("%CStr = type { {ptr, i64} }")
	g.write("")
	// Emit struct type definitions.
	for _, s := range structs {
		g.genStruct(s.Env, s)
	}
	// Emit union type definitions.
	for _, u := range unions {
		g.genUnion(u.Env, u)
	}
	// Emit extern function declarations (FFI).
	g.genExternDecls(funs)
	// Emit module-level constants as globals + init function.
	g.genModuleConsts(consts)
	// Emit all functions — each gets a fresh IRFunGen.
	for i := range funs {
		f := g.newFunGen(funs[i].Env)
		f.genFun(funs[i])
		g.sb.WriteString(f.sb.String())
		g.sb.WriteString(f.wrapperBuf.String())
	}
	// Emit global constants (strings, const-init arrays).
	g.write("; Global constants.")
	g.write("")
	g.sb.WriteString(g.constGlobals.String())
	g.write(builtinsIR)
	for _, bits := range []int{8, 16, 32, 64} {
		irType := fmt.Sprintf("i%d", bits)
		g.write(builtinFill(irType))
		for _, prefix := range []string{"s", "u"} {
			for _, op := range []string{"add", "sub", "mul"} {
				g.write(builtinCheckedArithmetic(prefix, op, irType))
			}
		}
	}
	g.write(builtinFill("i1"))
	return g.sb.String(), nil
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

func builtinCheckedArithmetic(prefix, op, irType string) string {
	return fmt.Sprintf(
		`define internal %[3]s @__checked_%[1]s%[2]s_%[3]s(%[3]s %%a, %[3]s %%b, ptr %%loc) alwaysinline {
    %%r = call {%[3]s, i1} @llvm.%[1]s%[2]s.with.overflow.%[3]s(%[3]s %%a, %[3]s %%b)
    %%val = extractvalue {%[3]s, i1} %%r, 0
    %%ov = extractvalue {%[3]s, i1} %%r, 1
    br i1 %%ov, label %%panic, label %%ok
panic:
    call void @panic(ptr @str_integer_overflow, ptr %%loc)
    unreachable
ok:
    ret %[3]s %%val
}
`,
		prefix,
		op,
		irType,
	)
}
