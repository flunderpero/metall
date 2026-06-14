package gen

import (
	_ "embed"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/types"
)

// voidValue is the LLVM IR literal for a value of the void type ({}).
const voidValue = "zeroinitializer"

//go:embed builtins.ll
var builtinsIR string

//go:embed builtins_posix.ll
var builtinsPosixIR string

//go:embed builtins_wasm.ll
var builtinsWasmIR string

//go:embed builtins_wasm32.ll
var builtinsWasm32IR string

//go:embed builtins_wasm64.ll
var builtinsWasm64IR string

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
	// discrs is non-empty for an enum-component arm (`case Color.red` / `case IOErr.x`):
	// the enum discriminants to test within the variant's payload, with enumIR the
	// enum's int IR type. A whole-variant arm matches its tag unconditionally.
	discrs []string
	enumIR string
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
	env              *types.TypeEnv
	funRetLabel      Label
	funRetReg        string
	lastLabel        Label
	arenaRegStack    [][]string     // stack of arena regs per block scope
	deferStack       [][]ast.NodeID // stack of defer block IDs per block scope
	loopStack        []LoopLabels
	astCode          map[ast.NodeID]string
	labelSalt        string // appended to generated labels to keep re-emitted defer bodies distinct
	deferEmitSeq     int    // monotonic counter feeding labelSalt, one bump per defer body emission
	entryAllocas     strings.Builder
	constInit        bool // true when generating module-level constant init
	wrapperBuf       strings.Builder
	errtraceShape    []errTraceLevel // non-nil if this function's return can carry an Err
	errtraceFuncName string
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

func (g *IRGen) ptrSize() int64 {
	return g.opts.Target.PointerSize()
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
	case ast.ArrayConstruction:
		g.genArrayConstruction(id, kind)
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
	case ast.Float:
		g.genFloat(id, kind)
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
	case ast.Range:
		g.genRange(id, kind)
	case ast.Struct,
		ast.Union,
		ast.Enum,
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
		ast.FunType:
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
	// LLVM has no native union type; we model it as { i64 tag, payload }.
	payloadIRType := g.unionPayloadIRType(env, unionType)
	g.write("%%%s = type { i64, %s } ; %s\n", u.TypeID, payloadIRType, unionType.Name)
}

// unionPayloadIRType picks an IR type for the union's payload slot sized to
// fit the largest variant. We use [N x i64] rather than the largest variant's
// own type because a variant like {ptr, i64} has internal padding on wasm32
// (ptr=4, pad=4, i64=8); SROA would then split a store of an 8-byte scalar
// variant into the 4-byte ptr sub-slot and truncate the value. [N x i64]
// gives SROA clean, padding-free word slots. Rounding up to i64 costs nothing
// because the i64 tag already forces 8-byte struct alignment.
func (g *IRGen) unionPayloadIRType(env *types.TypeEnv, union types.UnionType) string {
	var maxSize int64
	for _, variantID := range union.Variants {
		size := g.irTypeSize(env, variantID)
		if size > maxSize {
			maxSize = size
		}
	}
	return fmt.Sprintf("[%d x i64]", (maxSize+7)/8)
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
			g.storeValue(g.lookupCode(elem), g.fieldPtr(arrIRType, globalName, i), arrTyp.Elem)
		}
		return
	}
	reg := g.reg()
	g.writeAlloca(reg, arrIRType)
	g.setCode(id, reg)
	for i, elem := range lit.Elems {
		g.storeValue(g.lookupCode(elem), g.fieldPtr(arrIRType, reg, i), arrTyp.Elem)
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
	if _, ok := g.typeOfNode(id).Kind.(types.AllocatorType); ok {
		g.genArenaConstruction(id)
		return
	}
	targetTyp := g.typeOfNode(lit.Target)
	_, isInt := targetTyp.Kind.(types.IntType)
	_, isFloat := targetTyp.Kind.(types.FloatType)
	if isInt || isFloat {
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
	g.genStructConstructionFields(id, id, lit, reg)
}

// genArrayConstruction lowers `[N of v]` (fill) and `unsafe [N uninit T]`
// (uninit). The array is a stack value; fill reuses the same memory-init path
// as the arena slice constructors, with a compile-time element count.
func (g *IRFunGen) genArrayConstruction(id ast.NodeID, ac ast.ArrayConstruction) {
	arrTyp := base.Cast[types.ArrayType](g.typeOfNode(id).Kind)
	arrIRType := g.irTypeOfNode(id)
	reg := g.reg()
	g.writeAlloca(reg, arrIRType)
	g.setCode(id, reg)
	if ac.Fill == nil {
		return
	}
	g.Gen(*ac.Fill)
	valReg := g.lookupCode(*ac.Fill)
	dataReg := g.fieldPtr(arrIRType, reg, 0)
	n := arrTyp.Len
	g.genInitializeMemory(dataReg, g.irType(arrTyp.Elem), valReg, arrTyp.Elem, fmt.Sprintf("%d", n), &n)
}

func (g *IRFunGen) genUnionConstruction(
	id ast.NodeID, lit ast.TypeConstruction, union types.UnionType, unionTypeID types.TypeID,
) {
	g.Gen(lit.Target)
	g.Gen(lit.Args[0])
	argReg := g.lookupCode(lit.Args[0])
	argTypeID := g.typeIDOfNode(lit.Args[0])
	tag := g.unionVariantTag(argTypeID, union)
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

// unionVariantTag finds the variant the arg fills. A subset enum widens to its
// open root, so it matches the root variant rather than by exact identity.
func (g *IRFunGen) unionVariantTag(argTypeID types.TypeID, union types.UnionType) int {
	for i, variantID := range union.Variants {
		if argTypeID == variantID {
			return i
		}
		if enumKind, ok := g.env.Type(argTypeID).Kind.(types.EnumType); ok &&
			enumKind.Root != types.InvalidTypeID && enumKind.Root == variantID {
			return i
		}
	}
	return -1
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
	size := g.irTypeSize(g.env, valueTypeID)
	g.write("%s = call ptr @runtime$arena.arena_alloc(ptr %s, i64 %d)", reg, allocReg, size)
	if lit, ok := g.ast.Node(valueArg).Kind.(ast.TypeConstruction); ok {
		g.genStructConstructionFields(id, valueArg, lit, reg)
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
	elemSize := g.irTypeSize(g.env, sliceType.Elem)
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
	elemSize := g.irTypeSize(g.env, sliceType.Elem)

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
			totalBytes := *compileTimeCount * g.irScalarSize(irElemType)
			g.write("call void @llvm.memset.inline.p0.i64(ptr %s, i8 0, i64 %d, i1 false)", dataReg, totalBytes)
		} else {
			sizeReg := g.reg()
			g.write("%s_elm = mul i64 %s, %d", sizeReg, countReg, g.irScalarSize(irElemType))
			g.write("call void @llvm.memset.p0.i64(ptr %s, i8 0, i64 %s_elm, i1 false)", dataReg, sizeReg)
		}
		return
	}
	// Non-zero: use a prelude fill function.
	fillValReg := valReg
	fillIRType := irElemType
	if irElemType == "ptr" {
		if g.ptrSize() == 4 {
			fillIRType = "i32"
		} else {
			fillIRType = "i64"
		}
		fillValReg = g.reg()
		g.write("%s = ptrtoint ptr %s to %s", fillValReg, valReg, fillIRType)
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
	elemSize := g.irTypeSize(g.env, elemTypeID)
	g.write("call void @__fill_cpy(ptr %s, ptr %s, i64 %d, i64 %s)", dataReg, valReg, elemSize, countReg)
}

// genStructConstructionFields lowers a struct construction's fields into destReg.
// constructionID is the TypeConstruction node (where any named-argument order is
// recorded); resultID is the node whose result register is set (they differ when
// the construction is nested inside another expression like `arena.new(...)`).
func (g *IRFunGen) genStructConstructionFields(
	resultID, constructionID ast.NodeID, lit ast.TypeConstruction, destReg string,
) {
	g.Gen(lit.Target)
	args := lit.Args
	if order, ok := g.env.ArgOrder(constructionID); ok {
		args = order
	}
	for _, arg := range args {
		g.Gen(arg)
	}
	targetTyp := g.typeOfNode(lit.Target)
	structTyp := base.Cast[types.StructType](targetTyp.Kind)
	irTyp := g.irType(targetTyp.ID)
	for i, arg := range args {
		fieldReg := g.fieldPtr(irTyp, destReg, i)
		g.storeValue(g.lookupCode(arg), fieldReg, structTyp.Fields[i].Type)
	}
	g.setCode(resultID, destReg)
}

func (g *IRFunGen) genFieldAccess(id ast.NodeID, fieldAccess ast.FieldAccess) {
	if g.tryGenEnumVariantRef(id) {
		return
	}
	// Check for named function references first - this handles both module-level
	// functions (e.g. `mod.fun`) and type-namespaced methods (e.g. `mod.Type.method`).
	if name, ok := g.env.NamedFunRef(id); ok {
		g.emitFunValue(id, name)
		return
	}
	targetType := g.typeOfNode(fieldAccess.Target)
	if refTyp, ok := targetType.Kind.(types.RefType); ok {
		targetType = g.env.Type(refTyp.Type)
	}
	if _, ok := targetType.Kind.(types.ModuleType); ok {
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
	if _, ok := targetType.Kind.(types.EnumType); ok {
		g.genEnumFieldAccess(id, fieldAccess)
		return
	}
	ptrReg := g.genFieldAccessPtr(fieldAccess)
	valReg := g.loadValue(ptrReg, g.typeIDOfNode(id))
	g.setCode(id, valReg)
}

// genEnumFieldAccess reads a generated member (debug_name or an associated-data
// field) by mapping the value's tag to its assoc-struct row, then doing
// a normal struct field access.
func (g *IRFunGen) genEnumFieldAccess(id ast.NodeID, fieldAccess ast.FieldAccess) {
	g.Gen(fieldAccess.Target)
	discrReg := g.lookupCode(fieldAccess.Target)
	targetType := g.typeOfNode(fieldAccess.Target)
	if refTyp, ok := targetType.Kind.(types.RefType); ok {
		// `&E` target: the register is a pointer to the discriminant, so load the
		// tag before mapping it to its assoc-struct row.
		targetType = g.env.Type(refTyp.Type)
		loaded := g.reg()
		g.write("%s = load %s, ptr %s", loaded, g.irType(targetType.ID), discrReg)
		discrReg = loaded
	}
	enumTypeID := targetType.ID
	enum := base.Cast[types.EnumType](g.env.Type(enumTypeID).Kind)
	assoc := base.Cast[types.StructType](g.env.Type(enum.AssociatedDataStruct).Kind)
	fieldIdx := indexOfStructField(assoc, fieldAccess.Field.Name)
	rowPtr := g.genEnumRowPtr(id, enumTypeID, enum, discrReg)
	fieldPtr := g.fieldPtr(g.irType(enum.AssociatedDataStruct), rowPtr, fieldIdx)
	g.setCode(id, g.loadValue(fieldPtr, assoc.Fields[fieldIdx].Type))
}

// genEnumRowPtr maps the tag in tagReg to a pointer into the owning
// enum's assoc-struct table via a switch + phi (handles sparse tags).
func (g *IRFunGen) genEnumRowPtr(
	id ast.NodeID, enumTypeID types.TypeID, enum types.EnumType, tagReg string,
) string {
	tableOwner := enumTypeID
	if enum.Root != types.InvalidTypeID {
		tableOwner = enum.Root
	}
	rootVars := g.env.EnumFamilyVariants(tableOwner)
	tableIR := enumTableIR(g.env, enum.AssociatedDataStruct, len(rootVars))
	tableGlobal := fmt.Sprintf("@enum.%s", tableOwner)
	ordOf := map[string]int{}
	for i, v := range rootVars {
		ordOf[v.Tag.String()] = i
	}
	switchVars := g.env.EnumFamilyVariants(enumTypeID)
	intIR := g.irType(enumTypeID)
	contLabel := g.label("enumcont", id)
	defaultLabel := g.label("enumdefault", id)
	caseLabels := make([]Label, len(switchVars))
	sb := strings.Builder{}
	fmt.Fprintf(&sb, "switch %s %s, label %%%s [", intIR, tagReg, defaultLabel)
	for i, v := range switchVars {
		caseLabels[i] = g.label(fmt.Sprintf("enumcase_%d", i), id)
		fmt.Fprintf(&sb, " %s %s, label %%%s", intIR, v.Tag.String(), caseLabels[i])
	}
	sb.WriteString(" ]")
	g.write(sb.String())
	g.writeLabel(defaultLabel)
	g.write("unreachable")
	type entry struct {
		reg   string
		label Label
	}
	entries := make([]entry, len(switchVars))
	for i, v := range switchVars {
		g.writeLabel(caseLabels[i])
		row := g.fieldPtr(tableIR, tableGlobal, ordOf[v.Tag.String()])
		entries[i] = entry{row, g.lastLabel}
		g.write("br label %%%s", contLabel)
	}
	g.writeLabel(contLabel)
	phi := g.reg()
	phiSB := strings.Builder{}
	fmt.Fprintf(&phiSB, "%s = phi ptr ", phi)
	for i, e := range entries {
		if i > 0 {
			phiSB.WriteString(", ")
		}
		fmt.Fprintf(&phiSB, "[%s, %%%s]", e.reg, e.label)
	}
	g.write(phiSB.String())
	return phi
}

// calleeTypeArgs returns the explicit type arguments on a call's callee.
func (g *IRFunGen) calleeTypeArgs(call ast.Call) []ast.NodeID {
	switch kind := g.ast.Node(call.Callee).Kind.(type) {
	case ast.FieldAccess:
		return kind.TypeArgs
	case ast.Ident:
		return kind.TypeArgs
	default:
		panic(fmt.Sprintf("unexpected callee kind: %T", kind))
	}
}

// genEnumVariants lowers enums.variants<T>() to a rodata slice of every
// variant's tag (an enum value is its backing int).
func (g *IRFunGen) genEnumVariants(id ast.NodeID, call ast.Call) {
	enumTypeID := g.typeIDOfNode(g.calleeTypeArgs(call)[0])
	enum := base.Cast[types.EnumType](g.env.Type(enumTypeID).Kind)
	variants := g.env.EnumFamilyVariants(enumTypeID)
	elemIR := g.irType(enum.Backing)
	cid := g.constCounter
	g.constCounter++
	dataName := fmt.Sprintf("@__enum_variants_%d", cid)
	elems := make([]string, len(variants))
	for i, v := range variants {
		elems[i] = fmt.Sprintf("%s %s", elemIR, v.Tag.String())
	}
	fmt.Fprintf(&g.constGlobals, "%s.data = private constant [%d x %s] [%s]\n",
		dataName, len(variants), elemIR, strings.Join(elems, ", "))
	fmt.Fprintf(&g.constGlobals, "%s = private constant {ptr, i64} { ptr %s.data, i64 %d }\n",
		dataName, dataName, len(variants))
	reg := g.reg()
	g.writeAlloca(reg, "{ptr, i64}")
	valReg := g.reg()
	g.write("%s = load {ptr, i64}, ptr %s", valReg, dataName)
	g.write("store {ptr, i64} %s, ptr %s", valReg, reg)
	g.setCode(id, reg)
}

// genFromTag lowers enums.from_tag<T>(v) to a checked int-to-enum: the
// result is Some(v) when v is one of T's tags, else None.
func (g *IRFunGen) genFromTag(id ast.NodeID, call ast.Call) {
	enumTypeID := g.typeIDOfNode(g.calleeTypeArgs(call)[0])
	enum := base.Cast[types.EnumType](g.env.Type(enumTypeID).Kind)
	intIR := g.irType(enum.Backing)
	variants := g.env.EnumFamilyVariants(enumTypeID)

	g.Gen(call.Args[0])
	vReg := g.lookupCode(call.Args[0])

	optionTypeID := g.typeIDOfNode(id)
	optionUnion := base.Cast[types.UnionType](g.env.Type(optionTypeID).Kind)
	optionIR := g.irType(optionTypeID)
	someTag, noneTag := 0, 0
	for i, vID := range optionUnion.Variants {
		if vID == enumTypeID {
			someTag = i
		} else {
			noneTag = i
		}
	}

	optReg := g.reg()
	g.writeAlloca(optReg, optionIR)
	g.write("store %s zeroinitializer, ptr %s", optionIR, optReg)
	someLabel := g.label("from_tag_some", id)
	noneLabel := g.label("from_tag_none", id)
	contLabel := g.label("from_tag_cont", id)
	sb := strings.Builder{}
	fmt.Fprintf(&sb, "switch %s %s, label %%%s [", intIR, vReg, noneLabel)
	for _, v := range variants {
		fmt.Fprintf(&sb, " %s %s, label %%%s", intIR, v.Tag.String(), someLabel)
	}
	sb.WriteString(" ]")
	g.write(sb.String())

	g.writeLabel(someLabel)
	g.write("store i64 %d, ptr %s", someTag, g.fieldPtr(optionIR, optReg, 0))
	g.storeValue(vReg, g.fieldPtr(optionIR, optReg, 1), enumTypeID)
	g.write("br label %%%s", contLabel)

	g.writeLabel(noneLabel)
	g.write("store i64 %d, ptr %s", noneTag, g.fieldPtr(optionIR, optReg, 0))
	g.write("br label %%%s", contLabel)

	g.writeLabel(contLabel)
	g.setCode(id, optReg)
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
	return g.fieldPtr(g.irType(targetType.ID), structReg, indexOfStructField(structType, fieldAccess.Field.Name))
}

// fieldPtr points at field/element idx of the struct or fixed array of IR type
// aggIR based at baseReg.
func (g *IRFunGen) fieldPtr(aggIR, baseReg string, idx int) string {
	reg := g.reg()
	g.write("%s = getelementptr %s, ptr %s, i32 0, i32 %d", reg, aggIR, baseReg, idx)
	return reg
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
	if g.opts.ErrorTracing {
		g.errtraceShape = g.errTraceShape(fun.Return)
		g.errtraceFuncName = work.Name
	}
	isRetAggregate := g.isAggregateType(fun.Return)
	retIRTyp := g.irType(fun.Return)
	signatureIRTyp := retIRTyp
	params := strings.Builder{}
	if isClosure {
		params.WriteString("ptr %__ctx")
	}
	if isMain {
		signatureIRTyp = "i32"
		if !g.opts.Target.IsWasm() {
			if params.Len() > 0 {
				params.WriteString(", ")
			}
			params.WriteString("i32 %argc, ptr %argv")
		}
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
		if g.opts.Target.IsWasm() {
			// Seed the bump allocator before __const_init runs: const
			// initializers may allocate (via arena -> malloc).
			g.write("call {} @runtime$wasmalloc.init(ptr @__heap_base)")
		} else {
			g.write("call void @__os_args_init(i32 %argc, ptr %argv)")
		}
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
		g.recordErrTraceReturn(astFun.Block, g.tailExprSpan(astFun.Block), !g.tailExprIsCall(astFun.Block))
		g.write("br label %%%s", g.funRetLabel)
	}
	g.writeLabel(g.funRetLabel)
	switch {
	case isMain:
		if union, isUnion := g.env.Type(fun.Return).Kind.(types.UnionType); isUnion {
			g.genMainResultCheck(astFun.Block, fun.Return, union)
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

// genMainResultCheck renders main's `!void` result: success exits 0, an error
// prints "failed: <debug_name>" to stderr and exits 1. It is generated per
// program because resolving the error's debug_name walks that enum family's
// associated-data table, which a static runtime function cannot reference.
func (g *IRFunGen) genMainResultCheck(id ast.NodeID, unionTypeID types.TypeID, union types.UnionType) {
	unionIR := g.irType(unionTypeID)
	tagPtr := g.reg()
	g.write("%s = getelementptr %s, ptr %s, i32 0, i32 0", tagPtr, unionIR, g.funRetReg)
	tag := g.reg()
	g.write("%s = load i64, ptr %s", tag, tagPtr)
	errLabel := g.label("main_err", id)
	okLabel := g.label("main_ok", id)
	isErr := g.reg()
	g.write("%s = icmp ne i64 %s, 0", isErr, tag)
	g.write("br i1 %s, label %%%s, label %%%s", isErr, errLabel, okLabel)
	g.writeLabel(errLabel)
	errTypeID := types.InvalidTypeID
	for _, v := range union.Variants {
		if _, ok := g.env.Type(v).Kind.(types.EnumType); ok {
			errTypeID = v
			break
		}
	}
	// An empty error family means no error value can be constructed: the branch
	// is dead but must stay valid IR, so just exit non-zero without a name.
	if errTypeID != types.InvalidTypeID && len(g.env.EnumFamilyVariants(errTypeID)) > 0 {
		errEnum := base.Cast[types.EnumType](g.env.Type(errTypeID).Kind)
		assoc := base.Cast[types.StructType](g.env.Type(errEnum.AssociatedDataStruct).Kind)
		payloadPtr := g.reg()
		g.write("%s = getelementptr %s, ptr %s, i32 0, i32 1", payloadPtr, unionIR, g.funRetReg)
		discrReg := g.loadValue(payloadPtr, errTypeID)
		rowPtr := g.genEnumRowPtr(id, errTypeID, errEnum, discrReg)
		namePtr := g.fieldPtr(g.irType(errEnum.AssociatedDataStruct), rowPtr, indexOfStructField(assoc, "debug_name"))
		g.write("call void @__main_print_failed(ptr %s)", namePtr)
	}
	g.emitErrTraceMainDump()
	g.write("ret i32 1")
	g.writeLabel(okLabel)
	g.write("ret i32 0")
}

func (g *IRFunGen) genReturn(id ast.NodeID, return_ ast.Return) {
	g.Gen(return_.Expr)
	exprReg := g.lookupCode(return_.Expr)
	retTyp := g.typeIDOfNode(return_.Expr)
	g.storeValue(exprReg, g.funRetReg, retTyp)
	g.recordErrTraceReturn(id, g.ast.Node(id).Span, !g.isTryPropagation(return_.Expr))
	g.emitAllBlockCleanups(0)
	g.write("br label %%%s", g.funRetLabel)
	g.setCode(id, exprReg)
}

// emitBlockCleanup emits defer blocks (reverse order) and arena destroys for
// a single stack level.
func (g *IRFunGen) emitBlockCleanup(arenaLevel, deferLevel int) {
	defers := g.deferStack[deferLevel]
	for i := len(defers) - 1; i >= 0; i-- {
		// Re-emitting the same defer body at multiple exit points reuses its AST
		// node IDs, so without distinct per-emission state we'd emit a duplicate
		// astCode value and duplicate basic-block labels. LLVM merges same-named
		// blocks, leaving a terminator mid-block. A fresh astCode and a unique
		// label salt keep each emission self-contained.
		savedCode, savedSalt := g.astCode, g.labelSalt
		g.astCode = map[ast.NodeID]string{}
		g.deferEmitSeq++
		g.labelSalt = fmt.Sprintf(".d%d", g.deferEmitSeq)
		g.Gen(defers[i])
		g.astCode, g.labelSalt = savedCode, savedSalt
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
		if _, isIter := g.env.ForIterRet(id); isIter {
			g.genForInIter(id, forNode)
		} else {
			g.genForInSlice(id, forNode)
		}
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

func (g *IRFunGen) genRange(id ast.NodeID, range_ ast.Range) {
	g.Gen(*range_.Lo)
	g.Gen(*range_.Hi)
	loReg := g.lookupCode(*range_.Lo)
	hiReg := g.lookupCode(*range_.Hi)
	if range_.Inclusive {
		incReg := g.reg()
		g.write("%s = add i64 %s, 1", incReg, hiReg)
		hiReg = incReg
	}
	rangeTyp := g.typeOfNode(id)
	structTyp := base.Cast[types.StructType](rangeTyp.Kind)
	irTyp := g.irType(rangeTyp.ID)
	reg := g.reg()
	g.writeAlloca(reg, irTyp)
	g.storeValue(loReg, g.fieldPtr(irTyp, reg, 0), structTyp.Fields[0].Type)
	g.storeValue(hiReg, g.fieldPtr(irTyp, reg, 1), structTyp.Fields[1].Type)
	g.setCode(id, reg)
}

func (g *IRFunGen) genForInSlice(id ast.NodeID, forNode ast.For) { //nolint:funlen
	g.Gen(*forNode.Cond)
	targetReg := g.lookupCode(*forNode.Cond)
	targetType := g.typeOfNode(*forNode.Cond)
	var basePtrReg, lenReg string
	var elemTypeID types.TypeID
	switch kind := targetType.Kind.(type) {
	case types.ArrayType:
		elemTypeID = kind.Elem
		arrIRType := g.irType(targetType.ID)
		basePtrReg = g.reg()
		g.write("%s = getelementptr %s, %s* %s, i64 0, i64 0", basePtrReg, arrIRType, arrIRType, targetReg)
		lenReg = fmt.Sprintf("%d", kind.Len)
	case types.SliceType:
		elemTypeID = kind.Elem
		basePtrReg = g.reg()
		g.write("%s_field = getelementptr {ptr, i64}, ptr %s, i32 0, i32 0", basePtrReg, targetReg)
		g.write("%s = load ptr, ptr %s_field", basePtrReg, basePtrReg)
		lenReg = g.reg()
		g.write("%s_field = getelementptr {ptr, i64}, ptr %s, i32 0, i32 1", lenReg, targetReg)
		g.write("%s = load i64, ptr %s_field", lenReg, lenReg)
	default:
		panic(base.Errorf("genForSlice: unsupported target type %T", targetType.Kind))
	}
	elemIRType := g.irType(elemTypeID)

	idxReg := g.reg()
	g.writeAlloca(idxReg, "i64")
	g.write("store i64 0, ptr %s", idxReg)
	// A `&x` / `&mut x` binding holds a pointer into the storage; a plain `x`
	// binding holds a per-iteration copy of the element.
	xReg := g.reg()
	xType := elemIRType
	if forNode.Ref {
		xType = "ptr"
	}
	g.writeAlloca(xReg, xType)
	g.setSymbol(ast.BindingID(forNode.Body), forNode.Binding.Name, xReg, xType)
	if forNode.Index != nil {
		g.setSymbol(ast.BindingID(*forNode.Cond), forNode.Index.Name, idxReg, "i64")
	}

	labelCond := g.label("for", id)
	labelBody := g.label("body", id)
	labelIncr := g.label("incr", id)
	labelEnd := g.label("endfor", id)
	g.write("br label %%%s", labelCond)
	g.writeLabel(labelCond)
	iReg := g.reg()
	g.write("%s = load i64, ptr %s", iReg, idxReg)
	condReg := g.reg()
	g.write("%s = icmp slt i64 %s, %s", condReg, iReg, lenReg)
	g.write("br i1 %s, label %%%s, label %%%s", condReg, labelBody, labelEnd)
	g.writeLabel(labelBody)
	elemPtr := g.reg()
	g.write("%s = getelementptr %s, ptr %s, i64 %s", elemPtr, elemIRType, basePtrReg, iReg)
	if forNode.Ref {
		g.write("store ptr %s, ptr %s", elemPtr, xReg)
	} else {
		g.storeValue(g.loadValue(elemPtr, elemTypeID), xReg, elemTypeID)
	}
	g.loopStack = append(g.loopStack, LoopLabels{labelIncr, labelEnd, len(g.arenaRegStack)})
	defer func() { g.loopStack = g.loopStack[:len(g.loopStack)-1] }()
	g.Gen(forNode.Body)
	g.write("br label %%%s", labelIncr)
	g.writeLabel(labelIncr)
	nextReg := g.reg()
	g.write("%s = load i64, ptr %s", nextReg, idxReg)
	incrReg := g.reg()
	// Plain add, no overflow check: the index only ever reaches len, which is
	// itself an i64, so the increment can never overflow.
	g.write("%s = add i64 %s, 1", incrReg, nextReg)
	g.write("store i64 %s, ptr %s", incrReg, idxReg)
	g.write("br label %%%s", labelCond)
	g.writeLabel(labelEnd)
	g.setCode(id, voidValue)
}

// genForInIter lowers `for x in <iter>` over a type whose next() returns ?T.
// next() fills the optional result into one reused slot via sret each iteration;
// x binds directly to the stable payload address.
func (g *IRFunGen) genForInIter(id ast.NodeID, forNode ast.For) { //nolint:funlen
	g.Gen(*forNode.Cond)
	srcPtr := g.lookupCode(*forNode.Cond)
	// Iterate a private copy: next() mutates the iterator and the source may be
	// an immutable binding.
	iterType := g.typeOfNode(*forNode.Cond)
	iterPtr := g.reg()
	g.writeAlloca(iterPtr, g.irType(iterType.ID))
	g.storeValue(g.loadValue(srcPtr, iterType.ID), iterPtr, iterType.ID)

	nextName, ok := g.env.NamedFunRef(id)
	if !ok {
		panic(base.Errorf("genForInIter: no next() recorded for %s", id))
	}
	optTypeID, _ := g.env.ForIterRet(id)
	optIR := g.irType(optTypeID)
	optUnion := base.Cast[types.UnionType](g.env.Type(optTypeID).Kind)
	elemTypeID := optUnion.TypeArgs[0]
	noneTag := 0
	for i, vID := range optUnion.Variants {
		if vID != elemTypeID {
			noneTag = i
			break
		}
	}

	resReg := g.reg()
	g.writeAlloca(resReg, optIR)
	tagPtr := g.reg()
	g.write("%s = getelementptr %s, ptr %s, i32 0, i32 0", tagPtr, optIR, resReg)
	payloadPtr := g.reg()
	g.write("%s = getelementptr %s, ptr %s, i32 0, i32 1", payloadPtr, optIR, resReg)
	xType := g.irType(elemTypeID)
	if g.isAggregateType(elemTypeID) {
		xType = "ptr"
	}
	g.setSymbol(ast.BindingID(forNode.Body), forNode.Binding.Name, payloadPtr, xType)

	var idxReg string
	if forNode.Index != nil {
		idxReg = g.reg()
		g.writeAlloca(idxReg, "i64")
		g.write("store i64 0, ptr %s", idxReg)
		g.setSymbol(ast.BindingID(*forNode.Cond), forNode.Index.Name, idxReg, "i64")
	}

	labelCond := g.label("for", id)
	labelBody := g.label("body", id)
	labelIncr := g.label("incr", id)
	labelEnd := g.label("endfor", id)
	g.write("br label %%%s", labelCond)
	g.writeLabel(labelCond)
	g.write("call void @%s(ptr sret(%s) %s, ptr %s)", irName(nextName), optIR, resReg, iterPtr)
	tagReg := g.reg()
	g.write("%s = load i64, ptr %s", tagReg, tagPtr)
	condReg := g.reg()
	g.write("%s = icmp ne i64 %s, %d", condReg, tagReg, noneTag)
	g.write("br i1 %s, label %%%s, label %%%s", condReg, labelBody, labelEnd)
	g.writeLabel(labelBody)
	g.loopStack = append(g.loopStack, LoopLabels{labelIncr, labelEnd, len(g.arenaRegStack)})
	defer func() { g.loopStack = g.loopStack[:len(g.loopStack)-1] }()
	g.Gen(forNode.Body)
	g.write("br label %%%s", labelIncr)
	g.writeLabel(labelIncr)
	if forNode.Index != nil {
		cur := g.reg()
		g.write("%s = load i64, ptr %s", cur, idxReg)
		inc := g.reg()
		g.write("%s = add i64 %s, 1", inc, cur)
		g.write("store i64 %s, ptr %s", inc, idxReg)
	}
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
		// A diverging branch never reaches contLabel, so it contributes no phi
		// entry; the live branch alone gives the result type.
		thenDiverges := g.breaksControlFlow(ifNode.Then)
		elseDiverges := g.breaksControlFlow(*ifNode.Else)
		liveType := g.typeOfNode(ifNode.Then)
		if thenDiverges {
			liveType = g.typeOfNode(*ifNode.Else)
		}
		typ := g.irType(liveType.ID)
		if g.isAggregateType(liveType.ID) {
			typ = "ptr" // Aggregate values flow as pointers in code registers.
		}
		var entries []string
		if !thenDiverges {
			entries = append(entries, fmt.Sprintf("[%s, %%%s]", g.lookupCode(ifNode.Then), phiThenLabel))
		}
		if !elseDiverges {
			entries = append(entries, fmt.Sprintf("[%s, %%%s]", g.lookupCode(*ifNode.Else), phiElseLabel))
		}
		if typ != "{}" && typ != "void" && len(entries) > 0 {
			phi := g.reg()
			g.write("%s = phi %s %s", phi, typ, strings.Join(entries, ", "))
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
	exprType := g.typeOfNode(match.Expr)
	// Matching on `&U`/`&mut U` projects through the ref. The matched-value register
	// is already the union/enum address (a ref variable lowers to a ptr value),
	// so only the dispatch and IR type need the inner type.
	innerTypeID := exprType.ID
	if refTyp, ok := exprType.Kind.(types.RefType); ok {
		innerTypeID = refTyp.Type
	}
	innerKind := g.env.Type(innerTypeID).Kind
	if _, ok := innerKind.(types.EnumType); ok {
		g.genEnumMatch(id, match, innerTypeID)
		return
	}
	exprReg := g.lookupCode(match.Expr)
	union := base.Cast[types.UnionType](innerKind)
	unionIRType := g.irType(innerTypeID)
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

// unionPatternTag returns the union variant tag a match pattern dispatches to.
// For an enum-component pattern (`case Color.red` / `case IOErr.x`) it also
// returns the enum discriminants it matches and the enum's int IR type; a
// whole-variant pattern returns nil discriminants (matches its tag unconditionally).
func (g *IRFunGen) unionPatternTag(pat ast.NodeID, union types.UnionType) (tag int, discrs []string, enumIR string) {
	if enumID, variant, ok := g.env.EnumVariantRef(pat); ok {
		enum := base.Cast[types.EnumType](g.env.Type(enumID).Kind)
		for i, vID := range union.Variants {
			if enumID == vID || enum.Root == vID {
				return i, []string{enum.Variants[enum.VariantIndex(variant)].Tag.String()}, g.irType(enumID)
			}
		}
		panic(base.Errorf("unionPatternTag: enum variant ref not in union"))
	}
	patTypeID := g.typeIDOfNode(pat)
	for i, vID := range union.Variants {
		if patTypeID == vID {
			return i, nil, ""
		}
	}
	if enum, ok := g.env.Type(patTypeID).Kind.(types.EnumType); ok {
		for i, vID := range union.Variants {
			if enum.Root == vID {
				ds := make([]string, len(enum.Variants))
				for j, v := range enum.Variants {
					ds[j] = v.Tag.String()
				}
				return i, ds, g.irType(patTypeID)
			}
		}
	}
	panic(base.Errorf("unionPatternTag: pattern not a variant"))
}

func (g *IRFunGen) buildMatchArmInfos(
	id ast.NodeID,
	match ast.Match,
	union types.UnionType,
	elseLabel Label,
) []matchArmInfo {
	infos := make([]matchArmInfo, 0, len(match.Arms))
	for i, arm := range match.Arms {
		// An or-pattern contributes one switch target per variant, all jumping to
		// the same arm body.
		for _, pat := range arm.Patterns {
			tag, discrs, enumIR := g.unionPatternTag(pat, union)
			lbl := g.label(fmt.Sprintf("case_%d_%d", tag, i), id)
			infos = append(infos, matchArmInfo{lbl, i, tag, "", discrs, enumIR})
		}
	}
	// Chain guarded arms: if a guard fails, fall through to the next arm
	// for the same variant tag. The last arm for a tag falls through to
	// the else label (or unreachable).
	for i := range infos {
		// Guarded and enum-component arms can fall through (a guard fails or a
		// discriminant misses), so they chain to the next arm for the same variant tag.
		if match.Arms[infos[i].armIndex].Guard == nil && len(infos[i].discrs) == 0 {
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

func (g *IRFunGen) genMatchArms(
	id ast.NodeID,
	match ast.Match,
	armInfos []matchArmInfo,
	contLabel Label,
	payloadPtr string,
	elseLabel Label,
) {
	var phiEntries []matchPhiEntry
	// An or-pattern produces one switch target per variant, all converging on a
	// single body. The first target for an arm emits the body; the rest branch to
	// it. (Or-patterns carry no guard, so the body's binding is variant-agnostic.)
	armEntry := map[int]Label{}
	for _, armInfo := range armInfos {
		g.writeLabel(armInfo.label)
		if entry, ok := armEntry[armInfo.armIndex]; ok {
			g.write("br label %%%s", entry)
			continue
		}
		armEntry[armInfo.armIndex] = armInfo.label
		arm := match.Arms[armInfo.armIndex]
		if len(armInfo.discrs) > 0 {
			// Enum-component arm: the variant payload is an enum value. Test its
			// discriminant and fall through to the next same-tag arm on a miss.
			discrReg := g.reg()
			g.write("%s = load %s, ptr %s", discrReg, armInfo.enumIR, payloadPtr)
			matchReg := g.reg()
			g.write("%s = icmp eq %s %s, %s", matchReg, armInfo.enumIR, discrReg, armInfo.discrs[0])
			for _, d := range armInfo.discrs[1:] {
				eq := g.reg()
				g.write("%s = icmp eq %s %s, %s", eq, armInfo.enumIR, discrReg, d)
				combined := g.reg()
				g.write("%s = or i1 %s, %s", combined, matchReg, eq)
				matchReg = combined
			}
			bodyLabel := g.label(fmt.Sprintf("discr_ok_%d", armInfo.armIndex), id)
			g.write("br i1 %s, label %%%s, label %%%s", matchReg, bodyLabel, armInfo.guardFail)
			g.writeLabel(bodyLabel)
		}
		g.genMatchArmBinding(match, arm, payloadPtr)
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
			phiEntries = append(phiEntries, matchPhiEntry{g.lookupCode(arm.Body), g.lastLabel})
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
			phiEntries = append(phiEntries, matchPhiEntry{g.lookupCode(match.Else.Body), g.lastLabel})
		}
	}
	g.emitMatchPhi(id, contLabel, phiEntries)
}

type matchPhiEntry struct {
	code  string
	label Label
}

// emitMatchPhi writes the continuation label and a phi over the arm results
// (or `unreachable` when no arm falls through).
func (g *IRFunGen) emitMatchPhi(id ast.NodeID, contLabel Label, entries []matchPhiEntry) {
	resultType := g.typeOfNode(id)
	resultIRType := g.irType(resultType.ID)
	if g.isAggregateType(resultType.ID) {
		resultIRType = "ptr"
	}
	g.writeLabel(contLabel)
	if len(entries) == 0 {
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
	for i, entry := range entries {
		if i > 0 {
			phiSB.WriteString(", ")
		}
		fmt.Fprintf(&phiSB, "[%s, %%%s]", entry.code, entry.label)
	}
	g.write(phiSB.String())
	g.setCode(id, phi)
}

// genEnumMatch lowers an enum match as a linear chain. It tries each arm in
// source order, and the first whose tag matches and whose guard holds
// wins. Any miss falls through to the next arm, then to else (or `unreachable`
// for an exhaustive closed enum). A guarded arm proves nothing, so it falls
// through on guard failure exactly like a non-matching one. This also lowers
// `try`, which the parser desugars into a match with a diverging else.
func (g *IRFunGen) genEnumMatch(id ast.NodeID, match ast.Match, enumTypeID types.TypeID) { //nolint:funlen
	intIR := g.irType(enumTypeID)
	// On a `&E`/`&mut E` matched value the register is a pointer to the discriminant;
	// load the int for the tests and keep the pointer for reference bindings.
	discrReg := g.lookupCode(match.Expr)
	var enumPtr string
	if _, ok := g.typeOfNode(match.Expr).Kind.(types.RefType); ok {
		enumPtr = discrReg
		discrReg = g.reg()
		g.write("%s = load %s, ptr %s", discrReg, intIR, enumPtr)
	}
	contLabel := g.label("endmatch", id)
	elseLabel := g.label("case_else", id)
	testLabels := make([]Label, len(match.Arms))
	bodyLabels := make([]Label, len(match.Arms))
	for i := range match.Arms {
		testLabels[i] = g.label(fmt.Sprintf("test_%d", i), id)
		bodyLabels[i] = g.label(fmt.Sprintf("case_%d", i), id)
	}
	if len(match.Arms) > 0 {
		g.write("br label %%%s", testLabels[0])
	} else {
		g.write("br label %%%s", elseLabel)
	}

	var phiEntries []matchPhiEntry
	for i, arm := range match.Arms {
		miss := elseLabel
		if i+1 < len(match.Arms) {
			miss = testLabels[i+1]
		}
		g.writeLabel(testLabels[i])
		g.genEnumArmBinding(arm.Body, arm.Binding, arm.Ref, arm.Guard != nil, discrReg, enumPtr, intIR)
		// Gather the tags this arm matches (one for a variant, every
		// variant of a subset for a bare subset pattern), then test whether the
		// value equals any of them.
		// An or-pattern matches if the value equals any tag of any of its patterns.
		var discrs []string
		for _, pat := range arm.Patterns {
			if refEnumID, variant, ok := g.env.EnumVariantRef(pat); ok {
				refEnum := base.Cast[types.EnumType](g.env.Type(refEnumID).Kind)
				discrs = append(discrs, refEnum.Variants[refEnum.VariantIndex(variant)].Tag.String())
			} else {
				patEnum := base.Cast[types.EnumType](g.typeOfNode(pat).Kind)
				for _, v := range patEnum.Variants {
					discrs = append(discrs, v.Tag.String())
				}
			}
		}
		matchReg := g.reg()
		g.write("%s = icmp eq %s %s, %s", matchReg, intIR, discrReg, discrs[0])
		for _, d := range discrs[1:] {
			eq := g.reg()
			g.write("%s = icmp eq %s %s, %s", eq, intIR, discrReg, d)
			combined := g.reg()
			g.write("%s = or i1 %s, %s", combined, matchReg, eq)
			matchReg = combined
		}
		if arm.Guard == nil {
			g.write("br i1 %s, label %%%s, label %%%s", matchReg, bodyLabels[i], miss)
		} else {
			guardLabel := g.label(fmt.Sprintf("guard_%d", i), id)
			g.write("br i1 %s, label %%%s, label %%%s", matchReg, guardLabel, miss)
			g.writeLabel(guardLabel)
			g.Gen(*arm.Guard)
			g.write("br i1 %s, label %%%s, label %%%s", g.lookupCode(*arm.Guard), bodyLabels[i], miss)
		}
		g.writeLabel(bodyLabels[i])
		g.Gen(arm.Body)
		if !g.breaksControlFlow(arm.Body) {
			g.write("br label %%%s", contLabel)
			phiEntries = append(phiEntries, matchPhiEntry{g.lookupCode(arm.Body), g.lastLabel})
		}
	}

	g.writeLabel(elseLabel)
	if match.Else != nil {
		g.genEnumArmBinding(match.Else.Body, match.Else.Binding, match.Else.Ref, false, discrReg, enumPtr, intIR)
		g.Gen(match.Else.Body)
		if !g.breaksControlFlow(match.Else.Body) {
			g.write("br label %%%s", contLabel)
			phiEntries = append(phiEntries, matchPhiEntry{g.lookupCode(match.Else.Body), g.lastLabel})
		}
	} else {
		g.write("unreachable")
	}
	g.emitMatchPhi(id, contLabel, phiEntries)
}

// genEnumArmBinding binds the matched value's tag (the enum value) to the
// arm's binding name.
func (g *IRFunGen) genEnumArmBinding(
	bodyID ast.NodeID, binding *ast.Name, ref, hasGuard bool, discrReg, enumPtr, intIR string,
) {
	body := base.Cast[ast.Block](g.ast.Node(bodyID).Kind)
	// An empty body never reads the binding, but a guard still can, so bind it
	// whenever the arm is guarded even if the body is empty.
	if binding == nil || (len(body.Exprs) == 0 && !hasGuard) {
		return
	}
	// A reference binding (`case Color.red &x`) aliases the matched value's
	// discriminant storage; bind the pointer, held in a slot like any ref var.
	if ref {
		slot := g.reg()
		g.writeAlloca(slot, "ptr")
		g.write("store ptr %s, ptr %s", enumPtr, slot)
		g.setSymbol(ast.BindingID(bodyID), binding.Name, slot, "ptr")
		return
	}
	allocReg := g.reg()
	g.writeAlloca(allocReg, intIR)
	g.write("store %s %s, ptr %s", intIR, discrReg, allocReg)
	g.setSymbol(ast.BindingID(bodyID), binding.Name, allocReg, intIR)
}

func (g *IRFunGen) genMatchArmBinding(match ast.Match, arm ast.MatchArm, payloadPtr string) {
	body := base.Cast[ast.Block](g.ast.Node(arm.Body).Kind)
	// An empty body never reads the binding, but a guard still can, so bind it
	// whenever the arm is guarded even if the body is empty.
	if arm.Binding == nil || (len(body.Exprs) == 0 && arm.Guard == nil) {
		return
	}
	// Use the binding's recorded type, not the pattern's: a `&x` binding is typed
	// `&Variant`. arm.Ref marks a projection into the (reference) matched value.
	bindID := ast.BindingID(arm.Body)
	bindTypeID, _ := g.env.BindingType(bindID)
	// An or-pattern binds the whole matched value (its bound type is the union),
	// not a single variant's payload, exactly as an else binding does.
	if g.derefIfRef(bindTypeID) == g.derefIfRef(g.typeIDOfNode(match.Expr)) {
		unionPtr := g.lookupCode(match.Expr)
		if _, ok := g.env.Type(bindTypeID).Kind.(types.RefType); ok {
			g.genMatchBinding(bindID, arm.Binding.Name, bindTypeID, unionPtr, arm.Ref)
			return
		}
		g.setSymbol(bindID, arm.Binding.Name, unionPtr, "ptr")
		return
	}
	g.genMatchBinding(bindID, arm.Binding.Name, bindTypeID, payloadPtr, arm.Ref)
}

func (g *IRFunGen) genMatchElseBinding(match ast.Match, payloadPtr string) {
	bindID := ast.BindingID(match.Else.Body)
	bindTypeID, _ := g.env.BindingType(bindID)
	// The else binding covers the whole matched value when its pointee type matches
	// the matched value's pointee type; a narrowed single-variant else binds the
	// payload instead. Unwrapping a leading ref handles `&U` matched values.
	if g.derefIfRef(bindTypeID) == g.derefIfRef(g.typeIDOfNode(match.Expr)) {
		unionPtr := g.lookupCode(match.Expr)
		if _, ok := g.env.Type(bindTypeID).Kind.(types.RefType); ok {
			g.genMatchBinding(bindID, match.Else.Binding.Name, bindTypeID, unionPtr, match.Else.Ref)
			return
		}
		g.setSymbol(bindID, match.Else.Binding.Name, unionPtr, "ptr")
		return
	}
	g.genMatchBinding(bindID, match.Else.Binding.Name, bindTypeID, payloadPtr, match.Else.Ref)
}

// derefIfRef returns the pointee type of a RefType, else the type itself.
func (g *IRFunGen) derefIfRef(typeID types.TypeID) types.TypeID {
	if ref, ok := g.env.Type(typeID).Kind.(types.RefType); ok {
		return ref.Type
	}
	return typeID
}

func (g *IRFunGen) genMatchBinding(
	bindID ast.BindingID, name string, typeID types.TypeID, ptr string, projection bool,
) {
	// A projection binding (`case Foo &x` on a reference matched value) binds the
	// payload pointer itself, never a copy: the ref value is the address `ptr`,
	// held in a fresh slot so genIdent loads it like any other ref variable. This
	// must be gated on the projection marker, not on the binding being a RefType:
	// a value binding whose variant type is itself a reference (e.g. matching
	// `?&mut Int`) holds the ref value in the payload and must be loaded below.
	if projection {
		slot := g.reg()
		g.writeAlloca(slot, "ptr")
		g.write("store ptr %s, ptr %s", ptr, slot)
		g.setSymbol(bindID, name, slot, "ptr")
		return
	}
	valReg := g.loadValue(ptr, typeID)
	irTyp := g.irType(typeID)
	// An allocator binding must hold the arena pointer directly, never a stack
	// slot: genIdent's allocator branch returns the symbol reg without a load.
	_, isAlloc := g.env.Type(typeID).Kind.(types.AllocatorType)
	if irTyp == "{}" || isAlloc || g.isAggregateType(typeID) {
		g.setSymbol(bindID, name, valReg, "ptr")
		return
	}
	allocReg := g.reg()
	g.writeAlloca(allocReg, irTyp)
	g.write("store %s %s, ptr %s", irTyp, valReg, allocReg)
	g.setSymbol(bindID, name, allocReg, irTyp)
}

func (g *IRFunGen) label(name string, id ast.NodeID) Label {
	return Label(fmt.Sprintf("%s_%s%s", name, id, g.labelSalt))
}

func (g *IRFunGen) writeLabel(label Label) {
	g.lastLabel = label
	i := g.indent
	g.indent = 0
	g.write("%s:", label)
	g.indent = i
}

func (g *IRFunGen) genAssign(id ast.NodeID, assign ast.Assign) {
	if assign.Op != nil {
		g.genCompoundAssign(id, assign)
		return
	}
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

// genCompoundAssign lowers `lhs op= rhs` to `lhs = lhs op rhs`. The place is
// evaluated exactly once, so a side-effecting index like `arr[next()] += 1`
// advances the iterator only once.
func (g *IRFunGen) genCompoundAssign(id ast.NodeID, assign ast.Assign) {
	ptrReg := g.genPlaceAddr(assign.LHS)
	typeID := g.typeIDOfNode(assign.LHS)
	cur := g.loadValue(ptrReg, typeID)
	g.Gen(assign.RHS)
	rhs := g.lookupCode(assign.RHS)
	var result string
	if _, isFloat := g.typeOfNode(assign.LHS).Kind.(types.FloatType); isFloat {
		result = g.emitFloatBinOp(*assign.Op, g.irType(typeID), cur, rhs)
	} else {
		intTyp, isInt := g.typeOfNode(assign.LHS).Kind.(types.IntType)
		signed := isInt && intTyp.Signed
		result = g.emitArithBitOp(id, *assign.Op, g.irType(typeID), cur, rhs, signed, assign.LHS)
	}
	g.storeValue(result, ptrReg, typeID)
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
	if strings.HasPrefix(op, "s") && g.opts.ArithmeticOverflowCheck {
		g.emitSignedDivOverflowCheck(id, irTyp, lhs, rhs)
	}
	g.write("%s = %s %s %s, %s", reg, op, irTyp, lhs, rhs)
}

// emitSignedDivOverflowCheck traps on `INT_MIN / -1` (and `INT_MIN % -1`), the
// one signed division that overflows: it is LLVM UB and SIGFPEs on x86.
func (g *IRFunGen) emitSignedDivOverflowCheck(id ast.NodeID, irTyp, lhs, rhs string) {
	bits, _ := strconv.Atoi(strings.TrimPrefix(irTyp, "i"))
	intMin := strconv.FormatInt(int64(-1)<<(bits-1), 10)
	span := g.ast.Node(id).Span
	locReg := g.addStrConst(span.String())
	panicLabel := g.label("divovf_panic", id)
	okLabel := g.label("divovf_ok", id)
	lhsMinReg := g.reg()
	rhsNegOneReg := g.reg()
	bothReg := g.reg()
	g.write("%s = icmp eq %s %s, %s", lhsMinReg, irTyp, lhs, intMin)
	g.write("%s = icmp eq %s %s, -1", rhsNegOneReg, irTyp, rhs)
	g.write("%s = and i1 %s, %s", bothReg, lhsMinReg, rhsNegOneReg)
	g.write("br i1 %s, label %%%s, label %%%s", bothReg, panicLabel, okLabel)
	g.writeLabel(panicLabel)
	g.write("call void @panic(ptr @str_integer_overflow, ptr %s)", locReg)
	g.write("unreachable")
	g.writeLabel(okLabel)
}

// emitCheckedShift emits a shift, range-checking the amount under
// ArithmeticOverflowCheck: shifting by >= the type's bit width is undefined.
func (g *IRFunGen) emitCheckedShift(id ast.NodeID, reg, irTyp, op, lhs, rhs string) {
	if !g.opts.ArithmeticOverflowCheck {
		g.write("%s = %s %s %s, %s", reg, op, irTyp, lhs, rhs)
		return
	}
	bits := strings.TrimPrefix(irTyp, "i")
	span := g.ast.Node(id).Span
	locReg := g.addStrConst(span.String())
	panicLabel := g.label("shift_panic", id)
	okLabel := g.label("shift_ok", id)
	oorReg := g.reg()
	// Unsigned compare catches a negative amount too (a huge unsigned value).
	g.write("%s = icmp uge %s %s, %s", oorReg, irTyp, rhs, bits)
	g.write("br i1 %s, label %%%s, label %%%s", oorReg, panicLabel, okLabel)
	g.writeLabel(panicLabel)
	g.write("call void @panic(ptr @str_shift_out_of_range, ptr %s)", locReg)
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
	case ast.UnaryOpNeg:
		if _, ok := g.typeOfNode(unary.Expr).Kind.(types.FloatType); ok {
			g.write("%s = fneg %s %s", reg, g.irTypeOfNode(unary.Expr), expr)
		} else {
			g.emitCheckedArithmeticOp(id, reg, g.irTypeOfNode(unary.Expr), "sub", "0", expr, true)
		}
	default:
		panic(base.Errorf("unknown unary operator: %s", unary.Op))
	}
	g.setCode(id, reg)
}

func (g *IRFunGen) genBinary(id ast.NodeID, binary ast.Binary) {
	if binary.Op == ast.BinaryOpAnd || binary.Op == ast.BinaryOpOr {
		g.genShortCircuit(id, binary)
		return
	}
	g.Gen(binary.LHS)
	g.Gen(binary.RHS)
	lhs := g.lookupCode(binary.LHS)
	rhs := g.lookupCode(binary.RHS)
	irTyp := g.irTypeOfNode(binary.LHS)
	if _, isFloat := g.typeOfNode(binary.LHS).Kind.(types.FloatType); isFloat {
		g.setCode(id, g.emitFloatBinOp(binary.Op, irTyp, lhs, rhs))
		return
	}
	intTyp, isInt := g.typeOfNode(binary.LHS).Kind.(types.IntType)
	signed := isInt && intTyp.Signed
	switch binary.Op { //nolint:exhaustive
	case ast.BinaryOpEq:
		reg := g.reg()
		g.write("%s = icmp eq %s %s, %s", reg, irTyp, lhs, rhs)
		g.setCode(id, reg)
	case ast.BinaryOpNeq:
		reg := g.reg()
		g.write("%s = icmp ne %s %s, %s", reg, irTyp, lhs, rhs)
		g.setCode(id, reg)
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
		reg := g.reg()
		g.write("%s = icmp %s %s %s, %s", reg, cmpOp, irTyp, lhs, rhs)
		g.setCode(id, reg)
	default:
		g.setCode(id, g.emitArithBitOp(id, binary.Op, irTyp, lhs, rhs, signed, binary.LHS))
	}
}

// emitFloatBinOp emits an IEEE-754 binary operation. `==` and the relational
// operators use ordered predicates (false when either operand is NaN); `!=`
// uses the unordered predicate so it is the exact negation of `==` (true when
// either operand is NaN, matching C/Rust/Go). Arithmetic never traps or checks
// overflow, unlike the integer path.
func (g *IRFunGen) emitFloatBinOp(op ast.BinaryOp, irTyp, lhs, rhs string) string {
	reg := g.reg()
	switch op { //nolint:exhaustive
	case ast.BinaryOpEq:
		g.write("%s = fcmp oeq %s %s, %s", reg, irTyp, lhs, rhs)
	case ast.BinaryOpNeq:
		g.write("%s = fcmp une %s %s, %s", reg, irTyp, lhs, rhs)
	case ast.BinaryOpLt:
		g.write("%s = fcmp olt %s %s, %s", reg, irTyp, lhs, rhs)
	case ast.BinaryOpLte:
		g.write("%s = fcmp ole %s %s, %s", reg, irTyp, lhs, rhs)
	case ast.BinaryOpGt:
		g.write("%s = fcmp ogt %s %s, %s", reg, irTyp, lhs, rhs)
	case ast.BinaryOpGte:
		g.write("%s = fcmp oge %s %s, %s", reg, irTyp, lhs, rhs)
	case ast.BinaryOpAdd:
		g.write("%s = fadd %s %s, %s", reg, irTyp, lhs, rhs)
	case ast.BinaryOpSub:
		g.write("%s = fsub %s %s, %s", reg, irTyp, lhs, rhs)
	case ast.BinaryOpMul:
		g.write("%s = fmul %s %s, %s", reg, irTyp, lhs, rhs)
	case ast.BinaryOpDiv:
		g.write("%s = fdiv %s %s, %s", reg, irTyp, lhs, rhs)
	default:
		panic(base.Errorf("not a valid float operator: %s", op))
	}
	return reg
}

// emitArithBitOp emits a single arithmetic or bitwise binary operation on two
// value registers and returns the result register. It applies the same
// overflow, divide-by-zero, and Rune-range checks as the surface `lhs op rhs`
// expression, so it is shared by genBinary and compound assignment
// (`lhs op= rhs`). runeCheckNode is the operand whose type decides whether a
// Rune-range check is needed.
func (g *IRFunGen) emitArithBitOp(
	id ast.NodeID, op ast.BinaryOp, irTyp, lhs, rhs string, signed bool, runeCheckNode ast.NodeID,
) string {
	reg := g.reg()
	switch op { //nolint:exhaustive
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
	case ast.BinaryOpBitAnd:
		g.write("%s = and %s %s, %s", reg, irTyp, lhs, rhs)
	case ast.BinaryOpBitOr:
		g.write("%s = or %s %s, %s", reg, irTyp, lhs, rhs)
	case ast.BinaryOpBitXor:
		g.write("%s = xor %s %s, %s", reg, irTyp, lhs, rhs)
	case ast.BinaryOpShl:
		g.emitCheckedShift(id, reg, irTyp, "shl", lhs, rhs)
	case ast.BinaryOpShr:
		shrOp := "ashr"
		if !signed {
			shrOp = "lshr"
		}
		g.emitCheckedShift(id, reg, irTyp, shrOp, lhs, rhs)
	default:
		panic(base.Errorf("not an arithmetic or bitwise operator: %s", op))
	}
	g.runeCheckIfNeeded(runeCheckNode, reg)
	return reg
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
		argTypeID := g.typeIDOfNode(g.calleeTypeArgs(call)[0])
		size := g.irTypeSize(g.env, argTypeID)
		g.setCode(id, "%d", size)
		return true
	case "ffi::alignof":
		argTypeID := g.typeIDOfNode(g.calleeTypeArgs(call)[0])
		align := g.irTypeAlign(g.env, argTypeID)
		g.setCode(id, "%d", align)
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
	case "ffi::Ptr.cast", "ffi::PtrMut.cast",
		"ffi::Ptr.cast_ptr", "ffi::PtrMut.cast_ptr", "ffi::PtrMut.as_ptr":
		// Raw pointers (Ptr, PtrMut) and references share one IR representation,
		// so reinterpreting between them is a pass-through of the receiver.
		receiver, ok := g.env.MethodCallReceiver(id)
		if !ok {
			panic(fmt.Sprintf("expected a method call receiver for %s", name))
		}
		g.Gen(receiver)
		g.setCode(id, g.lookupCode(receiver))
		return true
	case "ffi::ref_ptr", "ffi::ref_ptr_mut":
		// References and raw pointers share the same IR representation, so
		// these are pass-throughs.
		g.Gen(call.Args[0])
		g.setCode(id, g.lookupCode(call.Args[0]))
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
	case "enums::variants":
		g.genEnumVariants(id, call)
		return true
	case "enums::from_tag":
		g.genFromTag(id, call)
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

// callDefaultSet returns the set of a call's default-argument nodes. Default
// expressions live on the function declaration and are shared across every call
// site, so they are re-emitted per site rather than generated once.
func (g *IRFunGen) callDefaultSet(id ast.NodeID) map[ast.NodeID]bool {
	defaults, ok := g.env.CallDefaults(id)
	if !ok {
		return nil
	}
	set := make(map[ast.NodeID]bool, len(defaults))
	for _, d := range defaults {
		set[d] = true
	}
	return set
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
	defaults := g.callDefaultSet(id)
	argNodes := g.env.CallArgNodes(id)
	autoRefReceiver := false
	if _, ok := g.env.MethodCallReceiver(id); ok {
		_, autoRefReceiver = g.env.MethodReceiverAutoRef(id)
	}
	receiverAddr := ""
	for i, nodeID := range argNodes {
		// An auto-borrowed receiver passes the place's address (a `ptr`), not its
		// loaded value. Computed first to preserve left-to-right argument order.
		if i == 0 && autoRefReceiver {
			receiverAddr = g.genPlaceAddr(nodeID)
			continue
		}
		// Re-emit a default into the current basic block (rather than reuse a
		// prior site's IR) so it is not dominated by just one call site's branch.
		if defaults[nodeID] {
			g.regenSubtree(nodeID)
		} else {
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
	for i, nodeID := range argNodes {
		if hasArg {
			sb.WriteString(", ")
		}
		if i == 0 && autoRefReceiver {
			fmt.Fprintf(&sb, "ptr %s", receiverAddr)
			hasArg = true
			continue
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
	if g.tryGenEnumVariantRef(id) {
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

// tryGenEnumVariantRef emits the tag for a variant reference
// (`Color.red` or `mod.Color.red`), recorded during type checking.
func (g *IRFunGen) tryGenEnumVariantRef(id ast.NodeID) bool {
	enumTypeID, variant, ok := g.env.EnumVariantRef(id)
	if !ok {
		return false
	}
	enum := base.Cast[types.EnumType](g.env.Type(enumTypeID).Kind)
	g.setCode(id, enum.Variants[enum.VariantIndex(variant)].Tag.String())
	return true
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
		total += g.irTypeSize(g.env, capTypeID)
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

func (g *IRFunGen) genFloat(id ast.NodeID, float_ ast.Float) {
	bits := base.Cast[types.FloatType](g.typeOfNode(id).Kind).Bits
	g.setCode(id, llvmFloatConst(float_.Value, bits))
}

// llvmFloatConst renders a float constant as the 16-hex-digit IEEE-754 double
// bit pattern LLVM requires. For F32 the value is first rounded to single
// precision so the widened double pattern is exact for the `float` type.
func llvmFloatConst(value float64, bits int) string {
	if bits == 32 {
		value = float64(float32(value))
	}
	return fmt.Sprintf("0x%016x", math.Float64bits(value))
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
	fmt.Fprintf(&g.constGlobals, "%s.data = private constant [%d x i8] c\"%s\"\n", name, n, llvmEscape(s))
	fmt.Fprintf(&g.constGlobals, "%s = private constant {ptr, i64} { ptr %s.data, i64 %d }\n", name, name, n)
	return name
}

// llvmEscape escapes a Go string for use inside an LLVM `c"..."` byte literal.
// Bytes outside printable ASCII (or '\' / '"') are emitted as \HH.
func llvmEscape(s string) string {
	var sb strings.Builder
	for i := range len(s) {
		b := s[i]
		if b == '\\' || b == '"' || b < 0x20 || b > 0x7E {
			fmt.Fprintf(&sb, "\\%02X", b)
		} else {
			sb.WriteByte(b)
		}
	}
	return sb.String()
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
		return g.genTempAddr(nodeID)
	}
}

// genTempAddr materializes a temporary into a fresh scope-local slot and returns
// its address. An aggregate value register is the address of wherever the value
// lives, which for a block/if/match tail is an existing place, so it must be
// copied into the fresh slot rather than returned directly. Otherwise `&mut { b }`
// would alias `b` and let a mutation reach an immutable binding.
func (g *IRFunGen) genTempAddr(nodeID ast.NodeID) string {
	g.Gen(nodeID)
	valueReg := g.lookupCode(nodeID)
	typeID := g.typeIDOfNode(nodeID)
	reg := g.reg()
	g.writeAlloca(reg, g.irType(typeID))
	g.storeValue(valueReg, reg, typeID)
	return reg
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
	g.Gen(alloc.Expr)
	reg := g.lookupCode(alloc.Expr)
	g.setCode(id, reg)
	g.setSymbol(ast.BindingID(id), alloc.Name.Name, reg, "ptr")
}

// genArenaConstruction emits a fresh arena on the stack and registers it for
// destruction at the end of the current block scope.
func (g *IRFunGen) genArenaConstruction(id ast.NodeID) {
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
		case ast.TypeConstruction, ast.ArrayLiteral, ast.ArrayConstruction, ast.EmptySlice, ast.SubSlice, ast.Call:
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
	return isAggregateTypeEnv(g.env, typeID)
}

func isAggregateTypeEnv(env *types.TypeEnv, typeID types.TypeID) bool {
	switch kind := env.Type(typeID).Kind.(type) {
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
	case types.FloatType:
		if kind.Bits == 32 {
			return "float"
		}
		return "double"
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
	case types.EnumType:
		return irType(env, kind.Backing)
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
// our irTypeSize/irTypeAlign assumptions.
func validateDataLayout(dl string) {
	if !strings.Contains(dl, "i64:64") {
		panic(base.Errorf("unsupported target data layout: expected i64:64 (8-byte i64 alignment), got %q", dl))
	}
}

func (g *IRGen) irTypeAlign(env *types.TypeEnv, typeID types.TypeID) int64 {
	ptrSize := g.ptrSize()
	typ := env.Type(typeID)
	switch kind := typ.Kind.(type) {
	case types.IntType:
		return int64(kind.Bits+7) / 8
	case types.FloatType:
		return int64(kind.Bits) / 8
	case types.BoolType:
		return 1
	case types.VoidType:
		return 1
	case types.RefType, types.AllocatorType:
		return ptrSize
	case types.FunType:
		return ptrSize
	case types.StructType:
		if types.IsBuiltinPtrStruct(kind) {
			return ptrSize
		}
		var maxAlign int64 = 1
		for _, field := range kind.Fields {
			if a := g.irTypeAlign(env, field.Type); a > maxAlign {
				maxAlign = a
			}
		}
		return maxAlign
	case types.UnionType:
		return 8
	case types.EnumType:
		return g.irTypeAlign(env, kind.Backing)
	case types.ArrayType:
		return g.irTypeAlign(env, kind.Elem)
	case types.SliceType:
		return 8
	default:
		panic(base.Errorf("irTypeAlign: unknown type kind: %T", typ.Kind))
	}
}

func alignUp(size, align int64) int64 {
	return (size + align - 1) / align * align
}

func (g *IRGen) irTypeSize(env *types.TypeEnv, typeID types.TypeID) int64 {
	ptrSize := g.ptrSize()
	typ := env.Type(typeID)
	switch kind := typ.Kind.(type) {
	case types.IntType:
		return int64(kind.Bits+7) / 8
	case types.FloatType:
		return int64(kind.Bits) / 8
	case types.BoolType:
		return 1
	case types.VoidType:
		return 0
	case types.RefType, types.AllocatorType:
		return ptrSize
	case types.FunType:
		return 2 * ptrSize
	case types.StructType:
		if types.IsBuiltinPtrStruct(kind) {
			return ptrSize
		}
		var offset int64
		var maxAlign int64 = 1
		for _, field := range kind.Fields {
			fieldAlign := g.irTypeAlign(env, field.Type)
			offset = alignUp(offset, fieldAlign)
			offset += g.irTypeSize(env, field.Type)
			if fieldAlign > maxAlign {
				maxAlign = fieldAlign
			}
		}
		return alignUp(offset, maxAlign)
	case types.UnionType:
		payload := g.unionPayloadSize(env, kind)
		return alignUp(8+payload, 8)
	case types.EnumType:
		return g.irTypeSize(env, kind.Backing)
	case types.ArrayType:
		elemSize := g.irTypeSize(env, kind.Elem)
		return kind.Len * elemSize
	case types.SliceType:
		// {ptr, i64} padded up to i64 alignment.
		return alignUp(ptrSize, 8) + 8
	default:
		panic(base.Errorf("irTypeSize: unknown type kind: %T", typ.Kind))
	}
}

func (g *IRGen) unionPayloadSize(env *types.TypeEnv, union types.UnionType) int64 {
	var maxSize int64
	for _, variantID := range union.Variants {
		size := g.irTypeSize(env, variantID)
		if size > maxSize {
			maxSize = size
		}
	}
	return maxSize
}

func (g *IRGen) irScalarSize(irType string) int64 {
	switch irType {
	case "i1", "i8":
		return 1
	case "i16":
		return 2
	case "i32":
		return 4
	case "i64":
		return 8
	case "ptr":
		return g.ptrSize()
	default:
		panic(base.Errorf("unknown scalar IR type: %s", irType))
	}
}

// regenSubtree wipes the astCode cache for the given node and all its
// descendants, then re-runs Gen so its IR lands in the current basic block.
// Used for default-argument expressions whose AST nodes are shared across
// every call site.
func (g *IRFunGen) regenSubtree(id ast.NodeID) {
	var clear func(ast.NodeID) //nolint:predeclared
	clear = func(n ast.NodeID) {
		delete(g.astCode, n)
		g.ast.Walk(n, clear)
	}
	clear(id)
	g.Gen(id)
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
	ErrorTracing            bool
	Target                  Target
}

func (g *IRGen) genExternDecls(funs []types.FunWork) {
	if len(funs) == 0 {
		return
	}
	env := funs[0].Env
	// Symbols already declared or defined by builtins_{posix,wasm}.ll;
	// re-emitting a declare here would collide (LLVM 22 rejects dupes).
	// Keep this list in sync with the top-level symbols in those files.
	emitted := map[string]bool{
		"write":             true,
		"arena_debug_print": true,
		// Defined in builtins.ll for error-return traces (std/errors.met).
		"__errtrace_buf": true,
		// Float conversion: defined in builtins_posix.ll, declared (as harness
		// imports) in builtins_wasm.ll. The prelude `extern`s them, so skip the
		// would-be duplicate declare on every target.
		"__strtod":         true,
		"__snprintf_float": true,
	}
	if g.opts.Target.IsWasm() {
		emitted["fflush"] = true
		emitted["__wasmalloc_bump_get"] = true
		emitted["__wasmalloc_bump_set"] = true
		emitted["__wasm_memory_size_pages"] = true
		emitted["__wasm_memory_grow_pages"] = true
		emitted["runtime$wasmalloc.malloc"] = true
		emitted["runtime$wasmalloc.realloc"] = true
		emitted["runtime$wasmalloc.free"] = true
	} else {
		emitted["printf"] = true
		emitted["fflush"] = true
		emitted["strlen"] = true
		emitted["malloc"] = true
		emitted["realloc"] = true
		emitted["free"] = true
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
			// Aggregates are passed `ptr byval(T)` at the call site; the declare
			// must match or LLVM rejects the inconsistent function type.
			if isAggregateTypeEnv(env, paramTypeID) {
				fmt.Fprintf(&params, "ptr byval(%s)", irType(env, paramTypeID))
			} else {
				params.WriteString(irType(env, paramTypeID))
			}
		}
		g.write("declare %s @%s(%s)", retIR, name, params.String())
		return true
	})
	g.write("")
}

// enumTableIR is the IR type of an enum's associated-data table: one assoc
// struct per variant.
func enumTableIR(env *types.TypeEnv, assocTypeID types.TypeID, n int) string {
	return fmt.Sprintf("[%d x %s]", n, irType(env, assocTypeID))
}

func (g *IRGen) genEnumAssocStruct(ew types.TypeWork) {
	enum := base.Cast[types.EnumType](ew.Env.Type(ew.TypeID).Kind)
	assoc := base.Cast[types.StructType](ew.Env.Type(enum.AssociatedDataStruct).Kind)
	fields := make([]string, len(assoc.Fields))
	for i, f := range assoc.Fields {
		fields[i] = irType(ew.Env, f.Type)
	}
	g.write("%%%s = type { %s } ; %s\n", enum.AssociatedDataStruct, strings.Join(fields, ", "), assoc.Name)
}

func (g *IRGen) genModuleConsts(consts []types.ConstWork, enums []types.TypeWork) {
	for _, c := range consts {
		g.write("@%s = internal global %s zeroinitializer", irName(c.Name), irType(c.Env, c.TypeID))
	}
	for _, ew := range enums {
		enum := base.Cast[types.EnumType](ew.Env.Type(ew.TypeID).Kind)
		n := len(ew.Env.EnumFamilyVariants(ew.TypeID))
		g.write(
			"@enum.%s = internal global %s zeroinitializer",
			ew.TypeID,
			enumTableIR(ew.Env, enum.AssociatedDataStruct, n),
		)
	}
	if len(consts) == 0 && len(enums) == 0 {
		g.write("define internal void @__const_init() { ret void }")
		g.write("")
		return
	}
	env := consts[0].Env
	if len(consts) == 0 {
		env = enums[0].Env
	}
	f := g.newFunGen(env)
	f.constInit = true
	f.write("define internal void @__const_init() {")
	f.indent++
	entryAllocaInsertPos := f.sb.Len()
	// Enum associated-data tables come first: a module constant may read a
	// variant's associated data, and that read loads from the table.
	for _, ew := range enums {
		f.fillEnumTable(ew)
	}
	for _, c := range consts {
		varNode := base.Cast[ast.Var](f.ast.Node(c.NodeID).Kind)
		globalRef := fmt.Sprintf("@%s", irName(c.Name))
		f.Gen(varNode.Expr)
		f.storeValue(f.lookupCode(varNode.Expr), globalRef, c.TypeID)
		b, ok := c.Env.Lookup(c.NodeID, varNode.Name.Name, -1)
		if !ok {
			panic(base.Errorf("constant binding not found: %s", varNode.Name.Name))
		}
		g.symbols[b.ID] = Symbol{Name: varNode.Name.Name, Reg: globalRef, Type: irType(c.Env, c.TypeID)}
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

// fillEnumTable builds each variant's assoc struct (debug_name + associated
// values) into the enum's table, evaluating values like module-level constants.
func (g *IRFunGen) fillEnumTable(ew types.TypeWork) {
	enum := base.Cast[types.EnumType](g.env.Type(ew.TypeID).Kind)
	assoc := base.Cast[types.StructType](g.env.Type(enum.AssociatedDataStruct).Kind)
	assocIR := g.irType(enum.AssociatedDataStruct)
	variants := g.env.EnumFamilyVariants(ew.TypeID)
	tableIR := enumTableIR(g.env, enum.AssociatedDataStruct, len(variants))
	tableGlobal := fmt.Sprintf("@enum.%s", ew.TypeID)
	for ord, v := range variants {
		rowPtr := g.fieldPtr(tableIR, tableGlobal, ord)
		for fieldIdx, field := range assoc.Fields {
			var valReg string
			switch {
			case fieldIdx == 0:
				valReg = g.addStrConst(v.DebugName)
			case fieldIdx == 1:
				valReg = v.Tag.String()
			case v.AssocArgs[fieldIdx-2] == 0:
				valReg = g.genNoneValue(field.Type)
			default:
				g.Gen(v.AssocArgs[fieldIdx-2])
				valReg = g.lookupCode(v.AssocArgs[fieldIdx-2])
			}
			g.storeValue(valReg, g.fieldPtr(assocIR, rowPtr, fieldIdx), field.Type)
		}
	}
}

// genNoneValue builds a none Option value (the tag of the non-payload variant).
func (g *IRFunGen) genNoneValue(optionTypeID types.TypeID) string {
	union := base.Cast[types.UnionType](g.env.Type(optionTypeID).Kind)
	noneTag := 0
	for i, vID := range union.Variants {
		if vID != union.TypeArgs[0] {
			noneTag = i
			break
		}
	}
	unionIR := g.irType(optionTypeID)
	reg := g.reg()
	g.writeAlloca(reg, unionIR)
	g.write("store %s zeroinitializer, ptr %s", unionIR, reg)
	tagPtr := g.reg()
	g.write("%s = getelementptr %s, ptr %s, i32 0, i32 0", tagPtr, unionIR, reg)
	g.write("store i64 %d, ptr %s", noneTag, tagPtr)
	return reg
}

func GenIR( //nolint:funlen
	a *ast.AST,
	module ast.Module,
	funs []types.FunWork,
	structs []types.TypeWork,
	unions []types.TypeWork,
	enums []types.TypeWork,
	consts []types.ConstWork,
	exports []types.ExportWork,
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
	// Emit enum associated-data struct type definitions.
	for _, ew := range enums {
		g.genEnumAssocStruct(ew)
	}
	// Emit extern function declarations (FFI).
	g.genExternDecls(funs)
	// Emit module-level constants and enum associated-data tables + init function.
	g.genModuleConsts(consts, enums)
	// Emit all functions — each gets a fresh IRFunGen.
	for i := range funs {
		f := g.newFunGen(funs[i].Env)
		f.genFun(funs[i])
		g.sb.WriteString(f.sb.String())
		g.sb.WriteString(f.wrapperBuf.String())
	}
	// Emit C export wrappers.
	for _, exp := range exports {
		g.genCExport(exp)
	}
	// Emit global constants (strings, const-init arrays).
	g.write("; Global constants.")
	g.write("")
	g.sb.WriteString(g.constGlobals.String())
	switch opts.Target {
	case TargetWasm32:
		g.write(builtinsWasmIR)
		g.write(builtinsWasm32IR)
	case TargetWasm64:
		g.write(builtinsWasmIR)
		g.write(builtinsWasm64IR)
	case TargetNative:
		g.write(builtinsPosixIR)
	}
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

// genCExport emits an externally-visible C entry point that `alwaysinline`s
// the internal Metall function, keeping the target free to be specialized
// while giving C a stable unmangled symbol.
func (g *IRGen) genCExport(exp types.ExportWork) {
	funType := base.Cast[types.FunType](exp.Env.Type(exp.FunTypeID).Kind)
	retIRTyp := irReturnType(exp.Env, funType.Return)
	params := strings.Builder{}
	args := strings.Builder{}
	for i, paramTypeID := range funType.Params {
		if i > 0 {
			params.WriteString(", ")
			args.WriteString(", ")
		}
		paramIR := irType(exp.Env, paramTypeID)
		name := fmt.Sprintf("%%arg%d", i)
		fmt.Fprintf(&params, "%s%s %s", paramIR, cABIExtAttr(exp.Env, paramTypeID), name)
		fmt.Fprintf(&args, "%s %s", paramIR, name)
	}
	g.write("define%s %s @%s(%s) {",
		cABIExtAttr(exp.Env, funType.Return), retIRTyp, exp.CName, params.String())
	g.indent++
	callee := irName(exp.InternalIR)
	if retIRTyp == "void" {
		g.write("call void @%s(%s) alwaysinline", callee, args.String())
		g.write("ret void")
	} else {
		g.write("%%ret = call %s @%s(%s) alwaysinline", retIRTyp, callee, args.String())
		g.write("ret %s %%ret", retIRTyp)
	}
	g.indent--
	g.write("}\n")
}

// cABIExtAttr returns " signext" / " zeroext" / "" (with leading space so it
// concatenates cleanly into an LLVM parameter slot) for narrow-integer types
// that clang's C ABI extends at the call boundary.
func cABIExtAttr(env *types.TypeEnv, typeID types.TypeID) string {
	switch kind := env.Type(typeID).Kind.(type) {
	case types.BoolType:
		return " zeroext"
	case types.IntType:
		if kind.Bits >= 32 {
			return ""
		}
		if kind.Signed {
			return " signext"
		}
		return " zeroext"
	}
	return ""
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
