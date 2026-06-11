package types

import (
	"fmt"
	"sort"
	"strings"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
)

// DebugTypes returns a string showing every AST node and its resolved type.
// Non-builtin types get sequential labels (struct01, fun01, shape01, etc.).
// After the AST tree, a legend lists what each label means.
func (e *TypeEnv) DebugTypes(rootID ast.NodeID) string {
	d := &debugTypes{
		env:                 e,
		ids:                 map[TypeID]string{},
		legends:             nil,
		counters:            map[string]int{},
		constraintTypes:     collectConstraintTypeNodes(e.ast, rootID),
		ownerTypeParamNodes: collectOwnerTypeParamNodes(e.ast, rootID),
	}
	var tree strings.Builder
	d.walk(&tree, rootID, 0)
	if len(d.legends) == 0 {
		return strings.TrimRight(tree.String(), "\n")
	}
	// Find max label width for alignment.
	maxLen := 0
	for _, l := range d.legends {
		if len(l.label) > maxLen {
			maxLen = len(l.label)
		}
	}
	var legend strings.Builder
	for _, l := range d.legends {
		fmt.Fprintf(&legend, "%-*s = %s\n", maxLen, l.label, l.detail)
	}
	return strings.TrimRight(tree.String(), "\n") + "\n---\n" + strings.TrimRight(legend.String(), "\n")
}

// DebugBindings returns a string showing every AST node and the scope it lives in.
// Scopes get sequential labels (scope01, scope02, etc.). After the AST tree,
// a legend lists each scope's bindings with their types and mutability.
// Only bindings whose declaration node is within the tested subtree are shown.
func (e *TypeEnv) DebugBindings(rootID ast.NodeID) string {
	// First, collect all node IDs in the subtree so we can filter bindings.
	subtreeNodes := map[ast.NodeID]bool{}
	e.collectNodes(rootID, subtreeNodes)

	db := &debugBindings{
		env:      e,
		scopeIDs: map[ast.ScopeID]string{},
		typeLabels: &debugTypes{
			env:                 e,
			ids:                 map[TypeID]string{},
			legends:             nil,
			counters:            map[string]int{},
			constraintTypes:     map[ast.NodeID]bool{},
			ownerTypeParamNodes: map[ast.NodeID]bool{},
		},
		counter:      0,
		scopeLegends: nil,
		subtreeNodes: subtreeNodes,
	}
	var tree strings.Builder
	db.walk(&tree, rootID, 0)
	if len(db.scopeLegends) == 0 {
		return strings.TrimRight(tree.String(), "\n")
	}
	var legend strings.Builder
	for i, l := range db.scopeLegends {
		if i > 0 {
			legend.WriteString("\n")
		}
		legend.WriteString(l)
	}
	var sb strings.Builder
	sb.WriteString(strings.TrimRight(tree.String(), "\n"))
	sb.WriteString("\n---\n")
	sb.WriteString(legend.String())
	// Append type legends if any were generated.
	if len(db.typeLabels.legends) > 0 {
		maxLen := 0
		for _, l := range db.typeLabels.legends {
			if len(l.label) > maxLen {
				maxLen = len(l.label)
			}
		}
		for _, l := range db.typeLabels.legends {
			fmt.Fprintf(&sb, "\n%-*s = %s", maxLen, l.label, l.detail)
		}
	}
	return sb.String()
}

func (e *TypeEnv) collectNodes(id ast.NodeID, nodes map[ast.NodeID]bool) {
	nodes[id] = true
	e.ast.Walk(id, func(childID ast.NodeID) {
		e.collectNodes(childID, nodes)
	})
}

func (e *TypeEnv) safeNodeType(id ast.NodeID) *cachedType {
	cached, ok := e.nodes[id]
	if ok {
		return cached
	}
	if e.parent != nil {
		return e.parent.safeNodeType(id)
	}
	return nil
}

// --- debugBindings ---

type debugBindings struct {
	env          *TypeEnv
	scopeIDs     map[ast.ScopeID]string
	typeLabels   *debugTypes
	counter      int
	scopeLegends []string
	subtreeNodes map[ast.NodeID]bool
}

func (db *debugBindings) walk(sb *strings.Builder, id ast.NodeID, indent int) {
	node := db.env.ast.Node(id)
	kindName := strings.TrimPrefix(fmt.Sprintf("%T", node.Kind), "ast.")
	prefix := strings.Repeat("  ", indent)
	scope := db.env.scopeGraph.NodeScope(id)
	label := db.scopeLabel(scope)
	fmt.Fprintf(sb, "%s%s: %s\n", prefix, kindName, label)
	switch kind := node.Kind.(type) {
	case ast.SimpleType:
		// Type args are part of the reference itself; suppress them so the
		// binding tree mirrors what the template pipeline produces.
		_ = kind
		return
	case ast.Ident:
		// Same reasoning as SimpleType: hide TypeArgs on identifier refs.
		_ = kind
		return
	case ast.FieldAccess:
		// Walk only the receiver; TypeArgs are absorbed into the call's
		// resolved type.
		db.walk(sb, kind.Target, indent+1)
		return
	}
	db.env.ast.Walk(id, func(childID ast.NodeID) {
		db.walk(sb, childID, indent+1)
	})
}

func (db *debugBindings) scopeLabel(scope *ast.Scope) string {
	if label, ok := db.scopeIDs[scope.ID]; ok {
		return label
	}
	db.counter++
	label := fmt.Sprintf("scope%02d", db.counter)
	db.scopeIDs[scope.ID] = label
	db.scopeLegends = append(db.scopeLegends, db.scopeDetail(label, scope))
	return label
}

func (db *debugBindings) scopeDetail(label string, scope *ast.Scope) string {
	// Filter to only bindings declared within the tested subtree.
	var bindings []*ast.Binding
	for _, ab := range scope.Bindings {
		if db.subtreeNodes[ab.Decl] {
			bindings = append(bindings, ab)
		}
	}
	sort.Slice(bindings, func(i, j int) bool {
		return bindings[i].Name < bindings[j].Name
	})
	var sb strings.Builder
	sb.WriteString(label + ":")
	if len(bindings) == 0 {
		return sb.String()
	}
	for _, ab := range bindings {
		b, ok := db.env.bindings[ab.ID]
		if !ok {
			sb.WriteString(fmt.Sprintf("\n  %s: ?", ab.Name))
			continue
		}
		if _, isTypeParam := db.env.ast.Node(ab.Decl).Kind.(ast.TypeParam); isTypeParam {
			sb.WriteString(fmt.Sprintf("\n  %s: ?", ab.Name))
			continue
		}
		typeLabel := db.typeLabels.typeLabelForID(b.TypeID)
		if b.Mut {
			sb.WriteString(fmt.Sprintf("\n  %s: %s (mut)", ab.Name, typeLabel))
		} else {
			sb.WriteString(fmt.Sprintf("\n  %s: %s", ab.Name, typeLabel))
		}
	}
	return sb.String()
}

// --- debugTypes ---

type debugTypesLegend struct {
	label  string
	detail string
}

type debugTypes struct {
	env                 *TypeEnv
	ids                 map[TypeID]string
	legends             []debugTypesLegend
	counters            map[string]int
	constraintTypes     map[ast.NodeID]bool
	ownerTypeParamNodes map[ast.NodeID]bool
}

func (d *debugTypes) walk(sb *strings.Builder, id ast.NodeID, indent int) {
	d.walkNode(sb, id, indent, false)
}

//nolint:funlen
func (d *debugTypes) walkNode(sb *strings.Builder, id ast.NodeID, indent int, hidden bool) {
	node := d.env.ast.Node(id)
	kindName := strings.TrimPrefix(fmt.Sprintf("%T", node.Kind), "ast.")
	prefix := strings.Repeat("  ", indent)
	typStr := "?"
	if !hidden {
		typStr = d.typeLabel(id)
	}
	fmt.Fprintf(sb, "%s%s: %s\n", prefix, kindName, typStr)
	switch kind := node.Kind.(type) {
	case ast.Struct:
		for _, childID := range kind.TypeParams {
			d.walkNode(sb, childID, indent+1, true)
		}
		for _, childID := range kind.Fields {
			d.walkNode(sb, childID, indent+1, true)
		}
	case ast.Shape:
		hideChildren := len(kind.TypeParams) > 0
		for _, childID := range kind.TypeParams {
			d.walkNode(sb, childID, indent+1, hideChildren)
		}
		for _, childID := range kind.Funs {
			d.walkNode(sb, childID, indent+1, hideChildren)
		}
	case ast.Union:
		for _, childID := range kind.TypeParams {
			d.walkNode(sb, childID, indent+1, true)
		}
		for _, childID := range kind.Variants {
			d.walkNode(sb, childID, indent+1, true)
		}
	case ast.Ident:
		// Show type-arg children only when they reference type parameters
		// (mirrors the SimpleType case below).
		for _, typeArgID := range kind.TypeArgs {
			argCached := d.env.safeNodeType(typeArgID)
			if argCached == nil || argCached.Type == nil {
				continue
			}
			if _, isParam := argCached.Type.Kind.(TypeParamType); !isParam {
				continue
			}
			d.walkNode(sb, typeArgID, indent+1, hidden)
		}
	case ast.FieldAccess:
		// Always walk the receiver. Walk type arguments only when the receiver
		// is a type/module reference (`Arena.method<T>`) - those are static
		// calls where the type argument is part of the visible AST.
		// For value receivers (`obj.method<T>`), the type args have been
		// absorbed into the materialized signature and are suppressed.
		d.walkNode(sb, kind.Target, indent+1, hidden)
		if d.fieldAccessReceiverIsType(kind.Target) {
			for _, typeArg := range kind.TypeArgs {
				d.walkNode(sb, typeArg, indent+1, hidden)
			}
		}
	case ast.Call:
		d.walkNode(sb, kind.Callee, indent+1, hidden)
		for _, argID := range kind.Args {
			d.walkNode(sb, argID, indent+1, hidden)
		}
		if defaults, ok := d.env.CallDefaults(id); ok {
			for _, defaultID := range defaults {
				d.walkNode(sb, defaultID, indent+1, hidden)
			}
		}
	case ast.Fun:
		tps := funTypeParamsOf(d.env, d.env.ast, id)
		d.walkFunTypeParams(sb, kind.Name.Name, tps, kind.Params, kind.ReturnType, indent+1, hidden)
		for _, paramID := range kind.Params {
			d.walkNode(sb, paramID, indent+1, hidden)
		}
		if kind.ReturnType != 0 {
			d.walkNode(sb, kind.ReturnType, indent+1, hidden)
		}
		if kind.Block != 0 {
			d.walkNode(sb, kind.Block, indent+1, hidden)
		}
	case ast.FunDecl:
		tps := funTypeParamsOf(d.env, d.env.ast, id)
		d.walkFunTypeParams(sb, kind.Name.Name, tps, kind.Params, kind.ReturnType, indent+1, hidden)
		for _, paramID := range kind.Params {
			d.walkNode(sb, paramID, indent+1, hidden)
		}
		if kind.ReturnType != 0 {
			d.walkNode(sb, kind.ReturnType, indent+1, hidden)
		}
	case ast.SimpleType:
		// In hidden contexts (inside a Struct/Union/Shape declaration walk) we
		// walk the AST TypeArgs as-is so the structural shape of the
		// declaration shows up; the labels are all "?" anyway.
		// In non-hidden contexts the AST TypeArgs are absorbed into the
		// resolved type, so we walk only the type-parameter args (omitting
		// concrete instantiations to match the template-pipeline output).
		if hidden || d.simpleTypeUnderConstraint(id) {
			for _, typeArg := range kind.TypeArgs {
				d.walkNode(sb, typeArg, indent+1, hidden)
			}
			return
		}
		cached := d.env.safeNodeType(id)
		if cached == nil || cached.Type == nil {
			return
		}
		typeArgs := typeArgsOf(cached.Type.Kind)
		for _, argTypeID := range typeArgs {
			argTyp, ok := d.env.cachedTypeInfo(argTypeID)
			if !ok || argTyp.Type == nil {
				continue
			}
			if _, isParam := argTyp.Type.Kind.(TypeParamType); !isParam {
				continue
			}
			label := d.typeLabelForID(argTypeID)
			prefix := strings.Repeat("  ", indent+1)
			fmt.Fprintf(sb, "%sSimpleType: %s\n", prefix, label)
		}
	default:
		_ = kind
		d.env.ast.Walk(id, func(childID ast.NodeID) {
			d.walkNode(sb, childID, indent+1, hidden)
		})
	}
}

func collectOwnerTypeParamNodes(a *ast.AST, rootID ast.NodeID) map[ast.NodeID]bool {
	out := map[ast.NodeID]bool{}
	var visit func(id ast.NodeID)
	visit = func(id ast.NodeID) {
		node := a.Node(id)
		switch kind := node.Kind.(type) {
		case ast.Struct:
			for _, p := range kind.TypeParams {
				out[p] = true
			}
		case ast.Union:
			for _, p := range kind.TypeParams {
				out[p] = true
			}
		case ast.Shape:
			for _, p := range kind.TypeParams {
				out[p] = true
			}
		}
		a.Walk(id, visit)
	}
	visit(rootID)
	return out
}

func collectConstraintTypeNodes(a *ast.AST, rootID ast.NodeID) map[ast.NodeID]bool {
	out := map[ast.NodeID]bool{}
	var visit func(id ast.NodeID)
	visit = func(id ast.NodeID) {
		node := a.Node(id)
		if tp, ok := node.Kind.(ast.TypeParam); ok && tp.Constraint != nil {
			out[*tp.Constraint] = true
		}
		a.Walk(id, visit)
	}
	visit(rootID)
	return out
}

func (d *debugTypes) simpleTypeUnderConstraint(nodeID ast.NodeID) bool {
	return d.constraintTypes[nodeID]
}

// walkFunTypeParams walks a function's type parameters. When the function has
// method-local params alongside owner-inherited ones (implicit type params
// added by the engine for methods on generic owners), the implicit owner
// params are hidden unless the method's signature references the owner type
// in a bare form (e.g. `f Foo` rather than `f Foo<U>`).
func (d *debugTypes) walkFunTypeParams(
	sb *strings.Builder,
	funName string,
	typeParams []ast.NodeID,
	params []ast.NodeID,
	returnTypeID ast.NodeID,
	indent int,
	hidden bool,
) {
	hasMethodLocal := false
	for _, paramID := range typeParams {
		if !d.ownerTypeParamNodes[paramID] {
			hasMethodLocal = true
			break
		}
	}
	ownerName, _, hasDot := strings.Cut(funName, ".")
	bareOwnerRef := false
	if hasDot {
		bareOwnerRef = d.signatureHasBareOwnerRef(ownerName, params, returnTypeID)
	}
	for _, paramID := range typeParams {
		if hasMethodLocal && d.ownerTypeParamNodes[paramID] && !bareOwnerRef {
			continue
		}
		d.walkNode(sb, paramID, indent, hidden)
	}
}

func (d *debugTypes) signatureHasBareOwnerRef(
	ownerName string, params []ast.NodeID, returnTypeID ast.NodeID,
) bool {
	check := func(typeID ast.NodeID) bool {
		if typeID == 0 {
			return false
		}
		st, ok := d.env.ast.Node(typeID).Kind.(ast.SimpleType)
		if !ok {
			return false
		}
		return st.Name.Name == ownerName && len(st.TypeArgs) == 0
	}
	for _, paramID := range params {
		p, ok := d.env.ast.Node(paramID).Kind.(ast.FunParam)
		if !ok {
			continue
		}
		if check(p.Type) {
			return true
		}
		if rt, ok := d.env.ast.Node(p.Type).Kind.(ast.RefType); ok {
			if check(rt.Type) {
				return true
			}
		}
	}
	return check(returnTypeID)
}

func (d *debugTypes) fieldAccessReceiverIsType(targetID ast.NodeID) bool {
	target, ok := d.env.ast.Node(targetID).Kind.(ast.Ident)
	if ok {
		binding, _, ok := d.env.scopeGraph.NodeScope(targetID).Lookup(target.Name, -1)
		if !ok {
			return false
		}
		switch d.env.ast.Node(binding.Decl).Kind.(type) {
		case ast.Struct, ast.Union, ast.Shape:
			return true
		}
		if b, ok := d.env.bindings[binding.ID]; ok {
			switch d.env.Type(b.TypeID).Kind.(type) {
			case ModuleType, AllocatorType:
				return true
			}
		}
	}
	if cached := d.env.safeNodeType(targetID); cached != nil && cached.Type != nil {
		switch d.env.Type(cached.Type.ID).Kind.(type) {
		case AllocatorType, ModuleType:
			return true
		}
	}
	return false
}

func typeArgsOf(kind TypeKind) []TypeID {
	switch k := kind.(type) {
	case StructType:
		return k.TypeArgs
	case UnionType:
		return k.TypeArgs
	case ShapeType:
		return k.TypeArgs
	default:
		return nil
	}
}

func (d *debugTypes) typeLabel(nodeID ast.NodeID) string {
	cached := d.env.safeNodeType(nodeID)
	if cached == nil || cached.Type == nil {
		if typeParamID, ok := d.env.TypeParamForNode(nodeID); ok {
			return d.typeLabelForID(typeParamID)
		}
		return "?"
	}
	return d.typeLabelForID(cached.Type.ID)
}

func (d *debugTypes) typeLabelForID(typeID TypeID) string { //nolint:funlen
	if typeID == InvalidTypeID {
		return "?"
	}
	// Check if already assigned.
	if label, ok := d.ids[typeID]; ok {
		return label
	}
	typ, ok := d.env.cachedTypeInfo(typeID)
	if !ok {
		return "?"
	}
	switch kind := typ.Type.Kind.(type) {
	case VoidType:
		return "void"
	case NeverType:
		return "never"
	case BoolType:
		return "Bool"
	case IntType:
		return kind.Name
	case TypeParamType:
		if typ.Type.NodeID != 0 {
			tp := base.Cast[ast.TypeParam](d.env.ast.Node(typ.Type.NodeID).Kind)
			return tp.Name.Name
		}
		return "?"
	case AllocatorType:
		return fmt.Sprint(kind.Impl)
	case StructType:
		if d.isBuiltinStruct(kind.Name) {
			return d.builtinStructLabel(kind)
		}
		label, isNew := d.reserveLabel("struct", typeID)
		if isNew {
			d.legends = append(d.legends, debugTypesLegend{label, d.structDetailWithID(kind, typeID)})
		}
		return label
	case FunType:
		label, isNew := d.reserveLabel("fun", typeID)
		if isNew {
			d.legends = append(d.legends, debugTypesLegend{label, d.funDetail(kind)})
		}
		return label
	case RefType:
		inner := d.typeLabelForID(kind.Type)
		if kind.Mut {
			return "&mut " + inner
		}
		return "&" + inner
	case ArrayType:
		return fmt.Sprintf("[%d]%s", kind.Len, d.typeLabelForID(kind.Elem))
	case SliceType:
		elem := d.typeLabelForID(kind.Elem)
		if kind.Mut {
			return "[]mut " + elem
		}
		return "[]" + elem
	case UnionType:
		label, isNew := d.reserveLabel("union", typeID)
		if isNew {
			d.legends = append(d.legends, debugTypesLegend{label, d.unionDetail(kind, typeID)})
		}
		return label
	case EnumType:
		label, isNew := d.reserveLabel("enum", typeID)
		if isNew {
			d.legends = append(d.legends, debugTypesLegend{label, d.enumDetail(kind)})
		}
		return label
	case ShapeType:
		label, isNew := d.reserveLabel("shape", typeID)
		if isNew {
			d.legends = append(d.legends, debugTypesLegend{label, d.shapeDetail(kind)})
		}
		return label
	case ModuleType:
		return kind.Name
	default:
		return "?"
	}
}

// reserveLabel pre-registers a label for a type ID so that recursive
// references can find it. Returns the label and false if the type was
// already registered (caller should skip detail computation).
func (d *debugTypes) reserveLabel(prefix string, typeID TypeID) (string, bool) {
	if label, ok := d.ids[typeID]; ok {
		return label, false
	}
	d.counters[prefix]++
	label := fmt.Sprintf("%s%02d", prefix, d.counters[prefix])
	d.ids[typeID] = label
	return label, true
}

func (d *debugTypes) isBuiltinStruct(name string) bool {
	return name == "Str"
}

func (d *debugTypes) builtinStructLabel(kind StructType) string {
	return kind.Name
}

func (d *debugTypes) structDeclName(kind StructType, typeID TypeID) string {
	// For instantiated Generics, look up the origin template's AST name.
	if origin, ok := d.env.GenericOrigin(typeID); ok {
		if originTyp, ok := d.env.cachedTypeInfo(origin); ok && originTyp.Type.NodeID != 0 {
			return base.Cast[ast.Struct](d.env.ast.Node(originTyp.Type.NodeID).Kind).Name.Name
		}
	}
	// For non-generic structs, read the name from the AST node.
	if typ, ok := d.env.cachedTypeInfo(typeID); ok && typ.Type.NodeID != 0 {
		if s, ok := d.env.ast.Node(typ.Type.NodeID).Kind.(ast.Struct); ok {
			return s.Name.Name
		}
	}
	return kind.Name
}

func (d *debugTypes) structDetailWithID(kind StructType, typeID TypeID) string {
	var sb strings.Builder
	sb.WriteString(d.structDeclName(kind, typeID))
	if len(kind.TypeArgs) > 0 {
		sb.WriteString("<")
		for i, ta := range kind.TypeArgs {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(d.typeLabelForID(ta))
		}
		sb.WriteString(">")
	}
	sb.WriteString(" { ")
	for i, f := range kind.Fields {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(f.Name)
		sb.WriteString(" ")
		sb.WriteString(d.typeLabelForID(f.Type))
	}
	sb.WriteString(" }")
	return sb.String()
}

func (d *debugTypes) funDetail(kind FunType) string {
	var sb strings.Builder
	if kind.Unsafe {
		sb.WriteString("unsafe ")
	}
	if kind.Sync {
		sb.WriteString("sync ")
	}
	sb.WriteString("fun(")
	for i, p := range kind.Params {
		if i > 0 {
			sb.WriteString(", ")
		}
		if kind.IsNoescape(i) {
			sb.WriteString("noescape ")
		}
		sb.WriteString(d.typeLabelForID(p))
	}
	sb.WriteString(") ")
	sb.WriteString(d.typeLabelForID(kind.Return))
	return sb.String()
}

func (d *debugTypes) unionDetail(kind UnionType, typeID TypeID) string {
	var sb strings.Builder
	name := kind.Name
	if origin, ok := d.env.GenericOrigin(typeID); ok {
		if originTyp, ok := d.env.cachedTypeInfo(origin); ok && originTyp.Type.NodeID != 0 {
			name = base.Cast[ast.Union](d.env.ast.Node(originTyp.Type.NodeID).Kind).Name.Name
		}
	} else if typ, ok := d.env.cachedTypeInfo(typeID); ok && typ.Type.NodeID != 0 {
		if u, ok := d.env.ast.Node(typ.Type.NodeID).Kind.(ast.Union); ok {
			name = u.Name.Name
		}
	}
	sb.WriteString(name)
	if len(kind.TypeArgs) > 0 {
		sb.WriteString("<")
		for i, ta := range kind.TypeArgs {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(d.typeLabelForID(ta))
		}
		sb.WriteString(">")
	}
	sb.WriteString(" = ")
	for i, v := range kind.Variants {
		if i > 0 {
			sb.WriteString(" | ")
		}
		sb.WriteString(d.typeLabelForID(v))
	}
	return sb.String()
}

func (d *debugTypes) enumDetail(kind EnumType) string {
	var sb strings.Builder
	sb.WriteString(kind.Name)
	if len(kind.Variants) > 0 {
		sb.WriteString(" = ")
		for i, v := range kind.Variants {
			if i > 0 {
				sb.WriteString(" | ")
			}
			sb.WriteString(v.Name)
		}
	}
	return sb.String()
}

func (d *debugTypes) shapeDetail(kind ShapeType) string {
	var sb strings.Builder
	sb.WriteString(kind.DeclName)
	sb.WriteString(" {  }")
	return sb.String()
}
