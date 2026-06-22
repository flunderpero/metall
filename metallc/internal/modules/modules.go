package modules

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/comptime"
	"github.com/flunderpero/metall/metallc/internal/token"
)

type ReadFileFn func(path string) ([]byte, error)

const wholeModuleSymbol = "*"

type ModuleResolution struct {
	AST     *ast.AST
	Imports map[string]Import
}

type Import struct {
	Module     ast.NodeID
	Symbol     string
	Pub        bool
	ImportNode ast.NodeID
}

func (i Import) IsModule() bool {
	return i.Symbol == wholeModuleSymbol
}

func importKey(modNode ast.NodeID, name string) string {
	return fmt.Sprintf("%d.%s", modNode, name)
}

// ImportFor returns the import bound as `name` in module `modNode`.
func (r *ModuleResolution) ImportFor(modNode ast.NodeID, name string) (Import, bool) {
	imp, ok := r.Imports[importKey(modNode, name)]
	return imp, ok
}

type moduleResolver struct {
	diagnostics      base.Diagnostics
	readFile         ReadFileFn
	projectRoot      string
	includePaths     []string
	ast              *ast.AST
	compTimeEnv      comptime.Env
	resolution       ModuleResolution
	modulesByPath    map[string]ast.NodeID // canonical file path -> module node (identity)
	resolving        map[string]bool       // canonical file path -> being resolved (cycle detection)
	namesByCanonical map[string]string     // canonical module name -> file path that claimed it
	resolvedModules  map[ast.NodeID]bool   // modules whose imports are already resolved
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
			Imports: make(map[string]Import),
		},
		modulesByPath:    make(map[string]ast.NodeID),
		resolving:        make(map[string]bool),
		namesByCanonical: make(map[string]string),
		resolvedModules:  make(map[ast.NodeID]bool),
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
	if m.resolvedModules[moduleID] {
		return
	}
	m.resolvedModules[moduleID] = true
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
		if _, exists := m.resolution.ImportFor(moduleID, name); exists {
			m.diag(importNode.Span, "import name `%s` already used", name)
			continue
		}
		// The path is a whole module if it resolves to a module file. Otherwise
		// the trailing segment is a symbol of the prefix module. (`pub use module`
		// is rejected later, in the type engine, as a normal diagnostic.)
		var moduleNodeID ast.NodeID
		symbol := wholeModuleSymbol
		if _, isModule := m.locateModuleFile(fqn); isModule {
			moduleNodeID, ok = m.resolveModule(fqn, importNode.Span)
		} else if len(imp.Segments) >= 2 &&
			m.isModulePath(strings.Join(imp.Segments[:len(imp.Segments)-1], "::")) {
			prefix := strings.Join(imp.Segments[:len(imp.Segments)-1], "::")
			moduleNodeID, ok = m.resolveModule(prefix, importNode.Span)
			symbol = imp.Segments[len(imp.Segments)-1]
		} else {
			// Neither the full path nor its prefix is a module: report not found.
			m.resolveModule(fqn, importNode.Span)
			continue
		}
		if !ok {
			continue
		}
		m.resolution.Imports[importKey(moduleID, name)] = Import{
			Module:     moduleNodeID,
			Symbol:     symbol,
			Pub:        imp.Pub,
			ImportNode: importNodeID,
		}
		m.resolveImports(moduleNodeID)
	}
}

func (m *moduleResolver) isModulePath(fqn string) bool {
	_, ok := m.locateModuleFile(fqn)
	return ok
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
// path: the path relative to the root that contains it, "::"-joined. It is
// independent of the import spelling, so a file has one name however it is
// reached.
//
// Include paths take priority over the project root and name non-local modules
// (the deepest include wins, keeping `std::errors` stable even when lib is also
// reachable from a wider `-I`, and yielding the shortest name). The project
// root is the naming root only for true local modules that no include path
// covers. This priority matters when the project root is itself nested below an
// include path (e.g. running `lib/std/ffi_test.met` directly with `-I lib`):
// `std::ffi` must keep its `std::` prefix, not collapse to `ffi`.
func CanonicalModuleName(path, projectRoot string, includePaths []string) string {
	cp := canonicalPath(path)
	best := ""
	for _, inc := range includePaths {
		if inc == "" {
			continue
		}
		rc := canonicalPath(inc)
		if strings.HasPrefix(cp, rc+string(filepath.Separator)) && len(rc) > len(best) {
			best = rc
		}
	}
	if best == "" && projectRoot != "" {
		rc := canonicalPath(projectRoot)
		if strings.HasPrefix(cp, rc+string(filepath.Separator)) {
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
	if path, ok := m.locateModuleFile(fqn); ok {
		return path, true
	}
	if strings.HasPrefix(fqn, "local::") {
		m.diag(span, "module not found: %s (project root: %s)", fqn, m.projectRoot)
	} else {
		m.diag(span, "module not found: %s (include paths: %s)", fqn, strings.Join(m.includePaths, ", "))
	}
	return "", false
}

// locateModuleFile resolves an fqn to a file path without emitting a
// diagnostic, so a `use a.b.Symbol` can probe `a::b` as a module before
// concluding the trailing segment is a symbol.
func (m *moduleResolver) locateModuleFile(fqn string) (string, bool) {
	local := strings.HasPrefix(fqn, "local::")
	rel := strings.ReplaceAll(strings.TrimPrefix(fqn, "local::"), "::", "/") + ".met"
	if local {
		path := filepath.Join(m.projectRoot, rel)
		if _, err := m.readFile(path); err == nil {
			return path, true
		}
		return "", false
	}
	for _, inc := range m.includePaths {
		path := filepath.Join(inc, rel)
		if _, err := m.readFile(path); err == nil {
			return path, true
		}
	}
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
