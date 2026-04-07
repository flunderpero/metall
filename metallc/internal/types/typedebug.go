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
		env:      e,
		ids:      map[TypeID]string{},
		legends:  nil,
		counters: map[string]int{},
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
		env:          e,
		scopeIDs:     map[ast.ScopeID]string{},
		typeLabels:   &debugTypes{env: e, ids: map[TypeID]string{}, legends: nil, counters: map[string]int{}},
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
	env      *TypeEnv
	ids      map[TypeID]string
	legends  []debugTypesLegend
	counters map[string]int
}

func (d *debugTypes) walk(sb *strings.Builder, id ast.NodeID, indent int) {
	node := d.env.ast.Node(id)
	kindName := strings.TrimPrefix(fmt.Sprintf("%T", node.Kind), "ast.")
	prefix := strings.Repeat("  ", indent)
	typStr := d.typeLabel(id)
	fmt.Fprintf(sb, "%s%s: %s\n", prefix, kindName, typStr)
	d.env.ast.Walk(id, func(childID ast.NodeID) {
		d.walk(sb, childID, indent+1)
	})
}

func (d *debugTypes) typeLabel(nodeID ast.NodeID) string {
	cached := d.env.safeNodeType(nodeID)
	if cached == nil || cached.Type == nil {
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
	typ := d.env.reg.types[typeID]
	if typ == nil {
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
	// For instantiated generics, look up the origin template's AST name.
	if origin, ok := d.env.reg.genericOrigin[typeID]; ok {
		originTyp := d.env.reg.types[origin]
		if originTyp != nil && originTyp.Type.NodeID != 0 {
			return base.Cast[ast.Struct](d.env.ast.Node(originTyp.Type.NodeID).Kind).Name.Name
		}
	}
	// For non-generic structs, read the name from the AST node.
	if typ := d.env.reg.types[typeID]; typ != nil && typ.Type.NodeID != 0 {
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
	if origin, ok := d.env.reg.genericOrigin[typeID]; ok {
		originTyp := d.env.reg.types[origin]
		if originTyp != nil && originTyp.Type.NodeID != 0 {
			name = base.Cast[ast.Union](d.env.ast.Node(originTyp.Type.NodeID).Kind).Name.Name
		}
	} else if typ := d.env.reg.types[typeID]; typ != nil && typ.Type.NodeID != 0 {
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

func (d *debugTypes) shapeDetail(kind ShapeType) string {
	var sb strings.Builder
	sb.WriteString(kind.DeclName)
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
