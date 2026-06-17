package modules

import (
	"path/filepath"
	"strings"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/comptime"
	"github.com/flunderpero/metall/metallc/internal/token"
)

type ReadFileFn func(path string) ([]byte, error)

type ModuleResolution struct {
	AST     *ast.AST
	Imports map[ast.NodeID]map[string]ast.NodeID
}

type moduleResolver struct {
	diagnostics   base.Diagnostics
	readFile      ReadFileFn
	projectRoot   string
	includePaths  []string
	ast           *ast.AST
	compTimeEnv   comptime.Env
	resolution       ModuleResolution
	modulesByPath    map[string]ast.NodeID // canonical file path -> module node (identity)
	resolving        map[string]bool       // canonical file path -> being resolved (cycle detection)
	namesByCanonical map[string]string     // canonical module name -> file path that claimed it
}

func ResolveModules(
	a *ast.AST, projectRoot string, includePaths []string,
	compTimeEnv comptime.Env, readFile ReadFileFn,
) (*ModuleResolution, base.Diagnostics) {
	m := moduleResolver{
		diagnostics:  base.Diagnostics{},
		readFile:     readFile,
		projectRoot:  projectRoot,
		includePaths: includePaths,
		ast:          a,
		compTimeEnv:  compTimeEnv,
		resolution: ModuleResolution{
			AST:     a,
			Imports: make(map[ast.NodeID]map[string]ast.NodeID),
		},
		modulesByPath:    make(map[string]ast.NodeID),
		resolving:        make(map[string]bool),
		namesByCanonical: make(map[string]string),
	}
	// Seed the identity maps with the already-parsed roots (the entry, parsed
	// by the caller before resolution). Without this, an import that resolves
	// back to one of these files would re-parse it as a separate module.
	for _, root := range a.Roots {
		mod := base.Cast[ast.Module](a.Node(root).Kind)
		m.modulesByPath[canonicalPath(mod.FileName)] = root
		m.namesByCanonical[mod.Name] = mod.FileName
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
	if _, ok := m.resolution.Imports[moduleID]; ok {
		return
	}
	// Rewrite `#if` blocks before scanning Decls: a `use` inside an
	// unresolved CompIf is invisible to this loop and would leave nested
	// modules (and their own CompIfs) unloaded/unresolved, which later
	// trips the type checker.
	if diags := comptime.ResolveModule(m.ast, moduleID, m.compTimeEnv); len(diags) > 0 {
		m.diagnostics = append(m.diagnostics, diags...)
		return
	}
	mod := base.Cast[ast.Module](m.ast.Node(moduleID).Kind)
	selfPath := canonicalPath(mod.FileName)
	m.resolving[selfPath] = true
	defer delete(m.resolving, selfPath)
	importMap := make(map[string]ast.NodeID)
	m.resolution.Imports[moduleID] = importMap
	seen := make(map[string]bool)
	for _, importNodeID := range mod.Decls {
		importNode := m.ast.Node(importNodeID)
		imp, ok := importNode.Kind.(ast.Import)
		if !ok {
			continue
		}
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
		moduleNodeID, ok := m.resolveModule(fqn, importNode.Span)
		if !ok {
			continue
		}
		importMap[name] = moduleNodeID
		m.resolveImports(moduleNodeID)
	}
}

func (m *moduleResolver) resolveModule(fqn string, span base.Span) (ast.NodeID, bool) {
	path, ok := m.findModuleFile(fqn, span)
	if !ok {
		return 0, false
	}
	// Identity is the canonical file path, so the same file reached via
	// different import spellings (e.g. `local.a` and `pkg.b.a`) is one module.
	cp := canonicalPath(path)
	if m.resolving[cp] {
		m.diag(span, "circular import: %s", fqn)
		return 0, false
	}
	if id, ok := m.modulesByPath[cp]; ok {
		return id, true
	}
	content, err := m.readFile(path)
	if err != nil {
		m.diag(span, "failed to read module %s: %s", fqn, err)
		return 0, false
	}
	name := CanonicalModuleName(path, m.projectRoot, m.includePaths)
	// Two different files that produce the same canonical name (same relative
	// path under different roots) would share symbol-table keys and mangled
	// symbols, silently clobbering each other. Reject it.
	if prev, ok := m.namesByCanonical[name]; ok && canonicalPath(prev) != cp {
		m.diag(span, "ambiguous module name %q: %s and %s resolve to it from different roots", name, prev, path)
		return 0, false
	}
	source := base.NewSource(path, name, false, []rune(string(content)))
	tokens := token.Lex(source)
	parser := ast.NewParser(tokens, m.ast)
	moduleID, _ := parser.ParseModule()
	if len(parser.Diagnostics) > 0 {
		m.diagnostics = append(m.diagnostics, parser.Diagnostics...)
		return 0, false
	}
	m.modulesByPath[cp] = moduleID
	m.namesByCanonical[name] = path
	return moduleID, true
}

// canonicalPath normalizes a path for identity comparison. It does not resolve
// symlinks (so it works with injected/virtual file systems in tests), which is
// enough to fold `lib/x`, `./lib/x`, and `a/../lib/x` onto one identity.
func canonicalPath(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(p)
}

// CanonicalModuleName derives a module's stable logical name from its file
// path: the path relative to the deepest (most specific) root that contains it,
// "::"-joined. Roots are the project root plus the include paths. Deepest-root
// keeps `std::errors` stable even when lib is also reachable from a wider `-I`,
// and yields the shortest name. It is independent of the import spelling, so a
// file has one name however it is reached.
func CanonicalModuleName(path, projectRoot string, includePaths []string) string {
	cp := canonicalPath(path)
	best := ""
	for _, root := range append([]string{projectRoot}, includePaths...) {
		if root == "" {
			continue
		}
		rc := canonicalPath(root)
		prefix := rc + string(filepath.Separator)
		if strings.HasPrefix(cp, prefix) && len(rc) > len(best) {
			best = rc
		}
	}
	rel := filepath.Base(cp)
	if best != "" {
		if r, err := filepath.Rel(best, cp); err == nil {
			rel = r
		}
	}
	rel = strings.TrimSuffix(filepath.ToSlash(rel), ".met")
	return strings.ReplaceAll(rel, "/", "::")
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
