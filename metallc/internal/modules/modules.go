package modules

import (
	"path/filepath"
	"strings"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/token"
)

type ReadFileFn func(path string) ([]byte, error)

type ModuleResolution struct {
	AST     *ast.AST
	Imports map[ast.NodeID]map[string]ast.NodeID
}

type moduleResolver struct {
	diagnostics  base.Diagnostics
	readFile     ReadFileFn
	projectRoot  string
	includePaths []string
	ast          *ast.AST
	resolution   ModuleResolution
	modulesByFQN map[string]ast.NodeID
	resolving    map[string]bool
}

func ResolveModules(
	a *ast.AST, projectRoot string, includePaths []string, readFile ReadFileFn,
) (*ModuleResolution, base.Diagnostics) {
	m := moduleResolver{
		diagnostics:  base.Diagnostics{},
		readFile:     readFile,
		projectRoot:  projectRoot,
		includePaths: includePaths,
		ast:          a,
		resolution: ModuleResolution{
			AST:     a,
			Imports: make(map[ast.NodeID]map[string]ast.NodeID),
		},
		modulesByFQN: make(map[string]ast.NodeID),
		resolving:    make(map[string]bool),
	}
	for _, root := range a.Roots {
		node := a.Node(root)
		mod := base.Cast[ast.Module](node.Kind)
		m.modulesByFQN[mod.Name] = root
	}
	for _, root := range a.Roots {
		m.resolveImports(root)
	}
	if len(m.diagnostics) > 0 {
		return nil, m.diagnostics
	}
	return &m.resolution, nil
}

func (m *moduleResolver) resolveImports(moduleID ast.NodeID) {
	mod := base.Cast[ast.Module](m.ast.Node(moduleID).Kind)
	if _, ok := m.resolution.Imports[moduleID]; ok {
		return
	}
	m.resolving[mod.Name] = true
	defer delete(m.resolving, mod.Name)
	importMap := make(map[string]ast.NodeID)
	m.resolution.Imports[moduleID] = importMap
	seen := make(map[string]bool)
	for _, importNodeID := range mod.Imports {
		importNode := m.ast.Node(importNodeID)
		imp := base.Cast[ast.Import](importNode.Kind)
		fqn := strings.Join(imp.Segments, "::")
		if seen[fqn] {
			m.diag(importNode.Span, "duplicate import: %s", fqn)
			continue
		}
		seen[fqn] = true
		name := m.importName(imp)
		if _, exists := importMap[name]; exists {
			m.diag(importNode.Span, "import name `%s` already used", name)
			continue
		}
		if m.resolving[fqn] {
			m.diag(importNode.Span, "circular import: %s", fqn)
			continue
		}
		moduleNodeID, ok := m.resolveModule(fqn, importNode.Span)
		if !ok {
			continue
		}
		importMap[name] = moduleNodeID
		m.resolveImports(moduleNodeID)
	}
}

func (m *moduleResolver) resolveModule(fqn string, span base.Span) (ast.NodeID, bool) {
	if id, ok := m.modulesByFQN[fqn]; ok {
		return id, true
	}
	path, ok := m.findModuleFile(fqn, span)
	if !ok {
		return 0, false
	}
	content, err := m.readFile(path)
	if err != nil {
		m.diag(span, "failed to read module %s: %s", fqn, err)
		return 0, false
	}
	source := base.NewSource(path, fqn, false, []rune(string(content)))
	tokens := token.Lex(source)
	parser := ast.NewParser(tokens, m.ast)
	moduleID, _ := parser.ParseModule()
	if len(parser.Diagnostics) > 0 {
		m.diagnostics = append(m.diagnostics, parser.Diagnostics...)
		return 0, false
	}
	m.modulesByFQN[fqn] = moduleID
	return moduleID, true
}

func (m *moduleResolver) findModuleFile(fqn string, span base.Span) (string, bool) {
	local := strings.HasPrefix(fqn, "local::")
	rel := strings.ReplaceAll(strings.TrimPrefix(fqn, "local::"), "::", "/") + ".met"
	if local {
		path := filepath.Join(m.projectRoot, rel)
		if _, err := m.readFile(path); err == nil {
			return path, true
		}
		m.diag(span, "module not found: %s (project root: %s)", fqn, m.projectRoot)
		return "", false
	}
	for _, inc := range m.includePaths {
		path := filepath.Join(inc, rel)
		if _, err := m.readFile(path); err == nil {
			return path, true
		}
	}
	m.diag(span, "module not found: %s (include paths: %s)", fqn, strings.Join(m.includePaths, ", "))
	return "", false
}

func (m *moduleResolver) importName(imp ast.Import) string {
	if imp.Alias != nil {
		return imp.Alias.Name
	}
	return imp.Segments[len(imp.Segments)-1]
}

func (m *moduleResolver) diag(span base.Span, msg string, args ...any) {
	m.diagnostics = append(m.diagnostics, *base.NewDiagnostic(span, msg, args...))
}
