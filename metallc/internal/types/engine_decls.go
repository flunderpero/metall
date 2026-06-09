package types

import (
	"math/big"
	"strings"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
)

func (e *Engine) checkStructCreateAndBind(node *ast.Node, structNode ast.Struct) (TypeID, TypeStatus) {
	name := e.declMangledName(node.ID, structNode.Name.Name)
	typeID := e.env.newType(
		StructType{Name: name, Fields: []StructField{}, TypeArgs: nil},
		node.ID,
		node.Span,
		TypeInProgress,
	)
	if !e.bind(node.ID, structNode.Name.Name, false, typeID, structNode.Name.Span, -1) {
		return typeID, TypeFailed
	}
	return typeID, TypeInProgress
}

func (e *Engine) checkUnionCreateAndBind(node *ast.Node, unionNode ast.Union) (TypeID, TypeStatus) {
	name := e.declMangledName(node.ID, unionNode.Name.Name)
	typeID := e.env.newType(UnionType{name, nil, nil}, node.ID, node.Span, TypeInProgress)
	if !e.bind(node.ID, unionNode.Name.Name, false, typeID, unionNode.Name.Span, -1) {
		return typeID, TypeFailed
	}
	return typeID, TypeInProgress
}

func (e *Engine) checkShapeCreateAndBind(node *ast.Node, shapeNode ast.Shape) (TypeID, TypeStatus) {
	name := e.declMangledName(node.ID, shapeNode.Name.Name)
	typeID := e.env.newType(
		ShapeType{Name: name, DeclName: shapeNode.Name.Name, Fields: nil, TypeArgs: nil},
		node.ID,
		node.Span,
		TypeInProgress,
	)
	if !e.bind(node.ID, shapeNode.Name.Name, false, typeID, shapeNode.Name.Span, -1) {
		return typeID, TypeFailed
	}
	return typeID, TypeInProgress
}

// checkStructCompleteType walks fields with type params in scope. resolveGenerics
// binds the type params into the keyed child env for this decl; we re-enter the
// same child env so its enterChildEnvFor(node.ID) and ours land on the same env.
func (e *Engine) checkStructCompleteType(
	node *ast.Node, structNode ast.Struct, structType StructType,
) (TypeStatus, StructType) {
	if _, status, _ := e.resolveGenerics(node.ID, nil); status.Failed() {
		return status, structType
	}
	defer e.enterChildEnvFor(node.ID)()
	fields := make([]StructField, len(structNode.Fields))
	for i, fieldNodeID := range structNode.Fields {
		fieldTypeID, status := e.Query(fieldNodeID)
		if status.Failed() {
			return TypeDepFailed, structType
		}
		fieldNode := base.Cast[ast.StructField](e.ast.Node(fieldNodeID).Kind)
		fields[i] = StructField{Name: fieldNode.Name.Name, Type: fieldTypeID, Pub: fieldNode.Pub}
	}
	structType.Fields = fields
	return TypeOK, structType
}

func (e *Engine) checkUnionCompleteType(
	node *ast.Node, unionNode ast.Union, unionType UnionType,
) (TypeStatus, UnionType) {
	if _, status, _ := e.resolveGenerics(node.ID, nil); status.Failed() {
		return status, unionType
	}
	defer e.enterChildEnvFor(node.ID)()
	variants := make([]TypeID, len(unionNode.Variants))
	for i, variantNodeID := range unionNode.Variants {
		variantTypeID, status := e.Query(variantNodeID)
		if status.Failed() {
			return TypeDepFailed, unionType
		}
		if unionNode.Pub {
			variantDeclNode := e.env.DeclNode(variantTypeID)
			if variantDeclNode != 0 {
				_, isTypeParam := e.ast.Node(variantDeclNode).Kind.(ast.TypeParam)
				if !isTypeParam && !e.declIsPub(variantDeclNode) {
					e.diag(e.ast.Node(variantNodeID).Span,
						"public union %s contains non-public variant type %s",
						unionNode.Name.Name, e.env.TypeDisplay(variantTypeID))
					return TypeFailed, unionType
				}
			}
		}
		variants[i] = variantTypeID
	}
	unionType.Variants = variants
	return TypeOK, unionType
}

func (e *Engine) checkEnumCreateAndBind(node *ast.Node, enumNode ast.Enum) (TypeID, TypeStatus) {
	name := e.declMangledName(node.ID, enumNode.Name.Name)
	typeID := e.env.newType(
		EnumType{
			Name: name, Backing: InvalidTypeID, Params: nil, Variants: nil,
			Root: InvalidTypeID, IsOpen: enumNode.Open, AssociatedDataStruct: InvalidTypeID,
		},
		node.ID,
		node.Span,
		TypeInProgress,
	)
	if !e.bind(node.ID, enumNode.Name.Name, false, typeID, enumNode.Name.Span, -1) {
		return typeID, TypeFailed
	}
	return typeID, TypeInProgress
}

// enumDeclIsSubset reports whether an enum's backing names another (open) enum
// rather than an integer type. Used to complete roots before subsets.
func (e *Engine) enumDeclIsSubset(enumNode ast.Enum) bool {
	backingTypeID, status := e.Query(enumNode.Backing)
	if status.Failed() {
		return false
	}
	_, isEnum := e.env.Type(backingTypeID).Kind.(EnumType)
	return isEnum
}

func (e *Engine) checkEnumCompleteType(
	node *ast.Node, enumNode ast.Enum, enumType EnumType,
) (TypeStatus, EnumType) {
	// The backing names either an integer type (standalone/open root) or the
	// open root this is a closed subset of.
	backingTypeID, status := e.Query(enumNode.Backing)
	if status.Failed() {
		return TypeDepFailed, enumType
	}
	switch k := e.env.Type(backingTypeID).Kind.(type) {
	case IntType:
		enumType.Backing = backingTypeID
		params, status := e.resolveEnumParams(enumNode)
		if status.Failed() {
			return status, enumType
		}
		enumType.Params = params
		enumType.AssociatedDataStruct = e.synthesizeAssocStruct(enumType.Name, enumType.Backing, params, node.Span)
	case EnumType:
		if !k.IsOpen {
			e.diag(e.ast.Node(enumNode.Backing).Span,
				"enum %s: %s is not an open enum and cannot be subsetted",
				enumNode.Name.Name, e.env.TypeDisplay(backingTypeID))
			return TypeFailed, enumType
		}
		if len(enumNode.Params) > 0 {
			e.diag(node.Span, "subset enum %s cannot declare its own associated-data params",
				enumNode.Name.Name)
			return TypeFailed, enumType
		}
		enumType.Root = backingTypeID
		enumType.Backing = k.Backing
		enumType.Params = k.Params
		enumType.AssociatedDataStruct = k.AssociatedDataStruct
	default:
		e.diag(e.ast.Node(enumNode.Backing).Span,
			"enum %s must be backed by an integer type or an open enum, got %s",
			enumNode.Name.Name, e.env.TypeDisplay(backingTypeID))
		return TypeFailed, enumType
	}
	if enumType.Root != InvalidTypeID && enumType.IsOpen {
		e.diag(node.Span, "enum %s: a subset of an open enum must have variants", enumNode.Name.Name)
		return TypeFailed, enumType
	}
	if !enumType.IsOpen {
		variants, status := e.resolveEnumVariants(node, enumNode, enumType)
		if status.Failed() {
			return status, enumType
		}
		enumType.Variants = variants
		e.bindEnumVariants(node, enumNode, enumType)
	}
	return TypeOK, enumType
}

// resolveEnumAssocArgs matches a variant's positional arguments to the params.
// An omitted optional field defaults to none. An omitted required field is an
// error. Returns one node per field (0 means none) for the backend to emit.
func (e *Engine) resolveEnumAssocArgs(v ast.EnumVariant, params []StructField) ([]ast.NodeID, TypeStatus) {
	if len(params) == 0 {
		if len(v.Args) > 0 {
			e.diag(v.Name.Span, "enum variant %s has associated data but the enum declares no params",
				v.Name.Name)
			return nil, TypeFailed
		}
		return nil, TypeOK
	}
	paramNames := make([]string, len(params))
	for i, field := range params {
		paramNames[i] = field.Name
	}
	slots, ok := matchArgs(e.ast, paramNames, "associated field", v.Args, v.ArgNames, v.Name.Span, e.diag)
	if !ok {
		return nil, TypeFailed
	}
	args := make([]ast.NodeID, len(params))
	for i, field := range params {
		argNodeID := slots[i]
		if argNodeID == 0 {
			if !e.isOptionType(field.Type) {
				e.diag(v.Name.Span, "enum variant %s: missing associated value for %s", v.Name.Name, field.Name)
				return nil, TypeFailed
			}
			continue
		}
		argType, status := e.queryWithHint(argNodeID, &field.Type)
		if status.Failed() {
			return nil, TypeDepFailed
		}
		if !e.isAssignableTo(argType, field.Type) {
			e.diag(e.ast.Node(argNodeID).Span, "associated value type mismatch for %s: expected %s, got %s",
				field.Name, e.env.TypeDisplay(field.Type), e.env.TypeDisplay(argType))
			return nil, TypeFailed
		}
		// Associated values are emitted as compile-time constants, so they follow
		// the same restriction as module-level constants: no function calls.
		if callNodeID, ok := ast.FindNode[ast.Call](e.ast, argNodeID); ok {
			e.diag(e.ast.Node(callNodeID).Span, "enum associated values cannot contain function calls")
			return nil, TypeFailed
		}
		// Reading a member off an enum value lowers to a runtime table load whose
		// fill order is unspecified, so it would silently read a zero table.
		if memberID, ok := e.assocValueReadsEnumMember(argNodeID); ok {
			e.diag(e.ast.Node(memberID).Span, "enum associated values cannot read enum fields")
			return nil, TypeFailed
		}
		args[i] = argNodeID
	}
	return args, TypeOK
}

func (e *Engine) isOptionType(typeID TypeID) bool {
	u, ok := e.env.Type(typeID).Kind.(UnionType)
	return ok && (u.Name == "Option" || strings.HasPrefix(u.Name, "Option."))
}

// assocValueReadsEnumMember finds a field access reading a member (debug_name or
// an associated field) off an enum value. A bare variant reference lowers to a
// constant tag and is fine; only a read off a value loads the table.
func (e *Engine) assocValueReadsEnumMember(nodeID ast.NodeID) (ast.NodeID, bool) {
	var found ast.NodeID
	var walk func(id ast.NodeID)
	walk = func(id ast.NodeID) {
		if found != 0 {
			return
		}
		if fa, ok := e.ast.Node(id).Kind.(ast.FieldAccess); ok && !e.isTypeReference(fa.Target) {
			if ct, ok := e.env.cachedNodeType(fa.Target); ok && ct.Type != nil {
				if _, isEnum := ct.Type.Kind.(EnumType); isEnum {
					found = id
					return
				}
			}
		}
		e.ast.Walk(id, walk)
	}
	walk(nodeID)
	return found, found != 0
}

// bindEnumVariants binds each variant as a member name (e.g. `Color.red`) so it
// resolves through the same dotted-name convention as methods. The binding's
// decl is the variant node so a variant value is not mistaken for a type.
func (e *Engine) bindEnumVariants(node *ast.Node, enumNode ast.Enum, enumType EnumType) {
	ct, ok := e.env.cachedNodeType(node.ID)
	if !ok {
		return
	}
	scope := e.scopeGraph.NodeScope(node.ID)
	for _, variantNodeID := range enumNode.Variants {
		v := base.Cast[ast.EnumVariant](e.ast.Node(variantNodeID).Kind)
		e.env.bindInScope(scope, variantNodeID, enumType.Name+"."+v.Name.Name, ct.Type.ID)
	}
}

// synthesizeAssocStruct builds the per-variant struct { debug_name, tag, params... }
// so associated-data access reuses the struct field-access path. `tag` is the
// variant's backing integer.
func (e *Engine) synthesizeAssocStruct(name string, backing TypeID, params []StructField, span base.Span) TypeID {
	fields := make([]StructField, 0, len(params)+2)
	fields = append(fields, StructField{Name: "debug_name", Type: e.strTyp, Pub: true})
	fields = append(fields, StructField{Name: "tag", Type: backing, Pub: true})
	fields = append(fields, params...)
	return e.env.newType(StructType{Name: name + "$assoc", Fields: fields, TypeArgs: nil}, 0, span, TypeOK)
}

func (e *Engine) resolveEnumParams(enumNode ast.Enum) ([]StructField, TypeStatus) {
	if len(enumNode.Params) == 0 {
		return nil, TypeOK
	}
	fields := make([]StructField, len(enumNode.Params))
	for i, paramNodeID := range enumNode.Params {
		paramNode := base.Cast[ast.FunParam](e.ast.Node(paramNodeID).Kind)
		if paramNode.Name.Name == "debug_name" || paramNode.Name.Name == "tag" {
			e.diag(paramNode.Name.Span, "%s is a reserved associated-data field name", paramNode.Name.Name)
			return nil, TypeFailed
		}
		if paramNode.Default != nil {
			e.diag(paramNode.Name.Span,
				"associated-data field defaults are not supported; use ?T for an optional field")
			return nil, TypeFailed
		}
		typeID, status := e.Query(paramNodeID)
		if status.Failed() {
			return nil, TypeDepFailed
		}
		fields[i] = StructField{Name: paramNode.Name.Name, Type: typeID, Pub: true}
	}
	return fields, TypeOK
}

//nolint:funlen
func (e *Engine) resolveEnumVariants(
	node *ast.Node, enumNode ast.Enum, enumType EnumType,
) ([]EnumVariantInfo, TypeStatus) {
	backing := base.Cast[IntType](e.env.Type(enumType.Backing).Kind)
	isSubset := enumType.Root != InvalidTypeID
	explicitCount := 0
	seen := map[string]bool{}
	seenValue := map[string]bool{}
	variants := make([]EnumVariantInfo, len(enumNode.Variants))
	for i, variantNodeID := range enumNode.Variants {
		v := base.Cast[ast.EnumVariant](e.ast.Node(variantNodeID).Kind)
		if v.Name.Name == "debug_name" {
			e.diag(v.Name.Span, "debug_name is a reserved enum member name")
			return nil, TypeFailed
		}
		if seen[v.Name.Name] {
			e.diag(v.Name.Span, "duplicate enum variant: %s", v.Name.Name)
			return nil, TypeFailed
		}
		seen[v.Name.Name] = true
		info := EnumVariantInfo{
			Name:      v.Name.Name,
			DebugName: enumVariantDebugName(enumType, isSubset, v.Name.Name),
			Tag:       nil,
			AssocArgs: nil,
		}
		if v.Tag != nil {
			if isSubset {
				e.diag(e.ast.Node(*v.Tag).Span,
					"subset enum variant %s cannot have an explicit tag", v.Name.Name)
				return nil, TypeFailed
			}
			value := base.Cast[ast.Int](e.ast.Node(*v.Tag).Kind).Value
			if value.Cmp(backing.Min) < 0 || value.Cmp(backing.Max) > 0 {
				e.diag(e.ast.Node(*v.Tag).Span,
					"tag %s does not fit backing type %s", value, backing.Name)
				return nil, TypeFailed
			}
			if seenValue[value.String()] {
				e.diag(e.ast.Node(*v.Tag).Span, "duplicate enum tag: %s", value)
				return nil, TypeFailed
			}
			seenValue[value.String()] = true
			info.Tag = value
			explicitCount++
		}
		assoc, status := e.resolveEnumAssocArgs(v, enumType.Params)
		if status.Failed() {
			return nil, status
		}
		info.AssocArgs = assoc
		variants[i] = info
	}
	if explicitCount > 0 && explicitCount < len(variants) {
		e.diag(node.Span, "enum %s: tags must be all-or-none", enumNode.Name.Name)
		return nil, TypeFailed
	}
	// Standalone closed enums get a self-contained 0..n numbering. Subset
	// tags come from the whole-program pool (AssignEnumTags).
	if !isSubset && explicitCount == 0 {
		if big.NewInt(int64(len(variants)-1)).Cmp(backing.Max) > 0 {
			e.diag(node.Span, "enum %s has %d variants, exceeding backing type %s",
				enumNode.Name.Name, len(variants), backing.Name)
			return nil, TypeFailed
		}
		for i := range variants {
			variants[i].Tag = big.NewInt(int64(i))
		}
	}
	return variants, TypeOK
}

//nolint:funlen
func (e *Engine) checkFunCreateAndBind(node *ast.Node, fun ast.FunDecl) (TypeID, TypeStatus) {
	if !e.checkMethodFieldCollision(node.ID, fun) {
		return InvalidTypeID, TypeFailed
	}
	if _, status, _ := e.resolveGenerics(node.ID, nil); status.Failed() {
		return InvalidTypeID, status
	}
	if fun.ReturnType == ast.InferredType {
		e.diag(node.Span, "cannot infer return type of function literal")
		return InvalidTypeID, TypeFailed
	}
	retTypeID, status := e.Query(fun.ReturnType)
	if status.Failed() {
		return InvalidTypeID, TypeDepFailed
	}
	if _, ok := e.env.Type(retTypeID).Kind.(AllocatorType); ok {
		e.diag(e.ast.Node(fun.ReturnType).Span, "cannot return an allocator from a function")
		return InvalidTypeID, TypeFailed
	}
	paramTypeIDs := make([]TypeID, len(fun.Params))
	noescapeParams := make([]bool, len(fun.Params))
	for i, paramNodeID := range fun.Params {
		paramNode := base.Cast[ast.FunParam](e.ast.Node(paramNodeID).Kind)
		if paramNode.Type == ast.InferredType {
			e.diag(paramNode.Name.Span, "cannot infer type of parameter '%s'", paramNode.Name.Name)
			return InvalidTypeID, TypeFailed
		}
		paramTypeID, status := e.Query(paramNodeID)
		if status.Failed() {
			return InvalidTypeID, TypeDepFailed
		}
		if !e.checkAllocatorNaming(paramNode.Name, paramTypeID) {
			return InvalidTypeID, TypeFailed
		}
		paramTypeIDs[i] = paramTypeID
		if paramNode.Noescape {
			noescapeParams[i] = true
		}
	}
	isSync := e.isFunDeclSync(node, paramTypeIDs, retTypeID)
	funTyp := FunType{
		Params:         paramTypeIDs,
		Return:         retTypeID,
		Macro:          false,
		Sync:           isSync,
		NoescapeParams: noescapeParams,
		NoescapeReturn: fun.NoescapeReturn,
	}
	funTypeID := e.env.buildFunType(funTyp, node.ID, node.Span)
	bindName := fun.Name.Name
	if structName, methodName, ok := strings.Cut(fun.Name.Name, "."); ok {
		resolved, ok := e.resolveMethodBindName(node.ID, structName, methodName, fun.Name.Span)
		if !ok {
			return InvalidTypeID, TypeFailed
		}
		bindName = resolved
	}
	if !e.bind(node.ID, bindName, false, funTypeID, fun.Name.Span, -1) {
		return InvalidTypeID, TypeFailed
	}
	if fun.Extern {
		e.env.setNamedFunRef(node.ID, fun.ExternName)
	} else {
		e.env.setNamedFunRef(node.ID, e.declMangledName(node.ID, fun.Name.Name))
	}
	return funTypeID, TypeOK
}

func (e *Engine) checkMethodFieldCollision(nodeID ast.NodeID, fun ast.FunDecl) bool {
	typeName, methodName, hasDot := strings.Cut(fun.Name.Name, ".")
	if !hasDot {
		return true
	}
	binding, ok := e.lookup(nodeID, typeName, -1)
	if !ok {
		return true
	}
	conflict := false
	switch decl := e.ast.Node(binding.Decl).Kind.(type) {
	case ast.Struct:
		for _, fieldNodeID := range decl.Fields {
			if base.Cast[ast.StructField](e.ast.Node(fieldNodeID).Kind).Name.Name == methodName {
				conflict = true
			}
		}
	case ast.Enum:
		// Variants read their generated debug_name and associated-data fields
		// through the same dotted path as methods, so a method may not shadow one.
		conflict = methodName == "debug_name"
		for _, paramNodeID := range decl.Params {
			if base.Cast[ast.FunParam](e.ast.Node(paramNodeID).Kind).Name.Name == methodName {
				conflict = true
			}
		}
	}
	if conflict {
		e.diag(fun.Name.Span, "method name conflicts with field: %s.%s", typeName, methodName)
		return false
	}
	return true
}
