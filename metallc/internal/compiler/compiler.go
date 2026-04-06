package compiler

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/gen"
	"github.com/flunderpero/metall/metallc/internal/macros"
	"github.com/flunderpero/metall/metallc/internal/modules"
	"github.com/flunderpero/metall/metallc/internal/token"
	"github.com/flunderpero/metall/metallc/internal/types"
)

type CompileListener interface {
	OnLex(tokens []token.Token) bool
	OnParse(a *ast.AST, fileID ast.NodeID, diagnostics base.Diagnostics) bool
	OnTypeCheck(engine *types.Engine, diagnostics base.Diagnostics) bool
	OnLifetimeCheck(lifetime *types.LifetimeCheck, diagnostics base.Diagnostics) bool
	OnIRGen(ir string) bool
	OnOptimizeIR() bool
	OnLink() bool
	OnRun(exitCode int, output string) bool
}

var ErrAbort = base.Errorf("aborted by listener")

type OptLevel int

const (
	OptLevelNone OptLevel = 0
	OptLevelSafe OptLevel = 1
	OptLevelFast OptLevel = 2
)

func (o OptLevel) String() string {
	switch o {
	case OptLevelNone:
		return "none"
	case OptLevelSafe:
		return "safe"
	case OptLevelFast:
		return "fast"
	default:
		panic(fmt.Sprintf("unknown OptLevel: %d", o))
	}
}

func ParseOptLevel(s string) (OptLevel, error) {
	switch s {
	case "none":
		return OptLevelNone, nil
	case "safe":
		return OptLevelSafe, nil
	case "fast":
		return OptLevelFast, nil
	default:
		return 0, base.Errorf("unknown opt-level: %s", s)
	}
}

// LLVM optimization passes (https://llvm.org/docs/Passes.html):
//   - mem2reg: Promote alloca'd scalars to SSA registers.
//   - sroa: Scalar Replacement of Aggregates — decompose struct/array allocas into individual scalars.
//   - instcombine: Peephole optimizations — constant folding, strength reduction, dead code elimination.
//   - simplifycfg: Simplify the control flow graph — merge blocks, remove unreachable code.
const DefaultLLVMPasses = "mem2reg,sroa,instcombine,simplifycfg"

type CompileOpts struct {
	ProjectRoot         string
	IncludePaths        []string
	Listener            CompileListener
	PrintTiming         bool
	Output              string
	KeepIR              bool
	LLVMPasses          string
	OptLevel            OptLevel
	AddressSanitizer    bool
	DebugArenaAllocator bool
	ArenaStackBufSize   int
	ArenaPageMinSize    int
	ArenaPageMaxSize    int
	MinimalPrelude      bool
	PrintTypesDebug     bool
	PrintBindingsDebug  bool
	DebugTypeCheck      bool
	DebugLifetime       bool
}

func (o CompileOpts) WithDefaults() CompileOpts {
	if o.ArenaStackBufSize == 0 {
		o.ArenaStackBufSize = 32
	}
	if o.ArenaPageMinSize == 0 {
		o.ArenaPageMinSize = 256
	}
	if o.ArenaPageMaxSize == 0 {
		o.ArenaPageMaxSize = 65536
	}
	return o
}

func Compile(ctx context.Context, source *base.Source, opts CompileOpts) error { //nolint:funlen
	opts = opts.WithDefaults()
	llvmHome := os.Getenv("LLVM_HOME")
	if llvmHome == "" {
		return base.Errorf("LLVM_HOME not set")
	}
	targetDataLayout, targetTriple, err := queryTargetInfo(ctx, llvmHome)
	if err != nil {
		return err
	}

	listener := opts.Listener
	timingListener := NewTimingListener(0)
	tokens := token.Lex(source)
	timingListener.OnLex(tokens)
	if listener != nil && !listener.OnLex(tokens) {
		return ErrAbort
	}
	parser := ast.NewParser(tokens, ast.NewAST(1))
	fileID, _ := parser.ParseModule()
	var moduleResolution *modules.ModuleResolution
	var runtimeModuleID ast.NodeID
	parseDiagnostics := parser.Diagnostics
	if len(parseDiagnostics) == 0 {
		var diags base.Diagnostics
		runtimeModuleID, diags = addRuntimeModule(parser.AST, opts)
		if len(diags) > 0 {
			parseDiagnostics = append(parseDiagnostics, diags...)
		} else {
			var moduleDiags base.Diagnostics
			moduleResolution, moduleDiags = resolveModules(parser.AST, opts)
			parseDiagnostics = append(parseDiagnostics, moduleDiags...)
		}
	}
	timingListener.OnParse(parser.AST, fileID, parseDiagnostics)
	if listener != nil && !listener.OnParse(parser.AST, fileID, parseDiagnostics) {
		return ErrAbort
	}
	if len(parseDiagnostics) > 0 {
		return parseDiagnostics
	}
	preludeAST, _ := ast.PreludeAST(opts.MinimalPrelude)
	engine := types.NewEngine(parser.AST, preludeAST, moduleResolution, newMacroExpander(ctx, opts))
	if opts.DebugTypeCheck {
		engine.SetDebug(base.NewStdoutDebug("types"))
	}
	engine.Query(fileID)
	engine.Query(runtimeModuleID)
	timingListener.OnTypeCheck(engine, engine.Diagnostics())
	if listener != nil && !listener.OnTypeCheck(engine, engine.Diagnostics()) {
		return ErrAbort
	}
	if len(engine.Diagnostics()) > 0 {
		return engine.Diagnostics()
	}
	if opts.PrintTypesDebug {
		fmt.Fprintln(os.Stderr, engine.Env().DebugTypes(fileID))
	}
	if opts.PrintBindingsDebug {
		fmt.Fprintln(os.Stderr, engine.Env().DebugBindings(fileID))
	}
	lifetime := types.NewLifetimeAnalyzer(engine.AST(), engine.ScopeGraph(), engine.Env(), engine.Funs())
	if opts.DebugLifetime {
		lifetime.Debug = base.NewStdoutDebug("lifetime")
	}
	lifetime.Check(fileID)
	lifetime.VerifyShapeContracts()
	timingListener.OnLifetimeCheck(lifetime, lifetime.Diagnostics)
	if listener != nil && !listener.OnLifetimeCheck(lifetime, lifetime.Diagnostics) {
		return ErrAbort
	}
	if len(lifetime.Diagnostics) > 0 {
		return lifetime.Diagnostics
	}
	module := base.Cast[ast.Module](engine.AST().Node(fileID).Kind)
	ir, err := gen.GenIR(
		engine.AST(), module, engine.Funs(), engine.Structs(), engine.Unions(), engine.Consts(),
		gen.IROpts{
			TargetDataLayout:        targetDataLayout,
			TargetTriple:            targetTriple,
			ArithmeticOverflowCheck: opts.OptLevel != OptLevelFast,
			AddressSanitizer:        opts.AddressSanitizer,
			ArenaDebug:              opts.DebugArenaAllocator,
			ArenaStackBufSize:       opts.ArenaStackBufSize,
			ArenaPageMinSize:        opts.ArenaPageMinSize,
			ArenaPageMaxSize:        opts.ArenaPageMaxSize,
		},
	)
	if err != nil {
		return err //nolint:wrapcheck
	}
	timingListener.OnIRGen(ir)
	if listener != nil && !listener.OnIRGen(ir) {
		return ErrAbort
	}
	output := opts.Output
	if output == "" {
		output = source.FileName[0 : len(source.FileName)-len(filepath.Ext(source.FileName))]
	}

	artifact_dir := filepath.Dir(output)
	if !opts.KeepIR {
		artifact_dir = os.TempDir()
		defer func() {
			_ = os.RemoveAll(artifact_dir)
		}()
	}

	filebase := filepath.Base(output)
	unopt_ll := filepath.Join(artifact_dir, filebase+".ll")
	opt_ll := filepath.Join(artifact_dir, filebase+".opt.ll")
	bc := filepath.Join(artifact_dir, filebase+".bc")

	// Write unoptimized IR (.ll)
	err = os.WriteFile(unopt_ll, []byte(ir), 0o600)
	if err != nil {
		return base.WrapErrorf(err, "failed to write IR file")
	}

	// Produce optimized textual IR (.opt.ll)
	cmdline := []string{llvmHome + "/bin/opt", "-S"}
	if opts.OptLevel != OptLevelNone {
		cmdline = append(cmdline, "-O3")
	} else if opts.LLVMPasses != "" {
		cmdline = append(cmdline, "-passes="+opts.LLVMPasses)
	}
	cmdline = append(cmdline, unopt_ll, "-o", opt_ll)
	if err := run_cmd(ctx, cmdline); err != nil {
		return base.WrapErrorf(err, "failed to generate optimized IR")
	}
	timingListener.OnOptimizeIR()
	if listener != nil && !listener.OnOptimizeIR() {
		return ErrAbort
	}

	// Produce optimized bitcode (.bc)
	cmdline = []string{llvmHome + "/bin/opt", opt_ll, "-o", bc}
	if err := run_cmd(ctx, cmdline); err != nil {
		return base.WrapErrorf(err, "failed to generate bitcode")
	}

	// Compile from optimized bitcode.
	cmdline = []string{llvmHome + "/bin/clang", bc, "-v", "-o", output}
	if opts.OptLevel != OptLevelNone {
		cmdline = append(cmdline, "-O3")
	}
	if opts.AddressSanitizer {
		cmdline = append(cmdline, "-fsanitize=address")
	}
	if err := run_cmd(ctx, cmdline); err != nil {
		return base.WrapErrorf(err, "failed to compile with clang")
	}
	timingListener.OnLink()
	if listener != nil && !listener.OnLink() {
		return ErrAbort
	}
	if opts.PrintTiming {
		fmt.Println(timingListener.String())
	}
	return nil
}

func CompileAndRun(
	ctx context.Context,
	source *base.Source,
	opts CompileOpts,
) (exitCode int, output string, err error) {
	runOpts := opts
	if runOpts.Output == "" {
		runOpts.Output = "/tmp/metallc"
	}
	defer func() {
		if opts.Output == "" {
			os.Remove(runOpts.Output) //nolint:errcheck,gosec
		}
	}()
	err = Compile(ctx, source, runOpts)
	if err != nil {
		return 0, "", err
	}
	cmd := exec.CommandContext(ctx, runOpts.Output) //nolint:gosec
	if opts.AddressSanitizer {
		cmd.Env = append(os.Environ(), "ASAN_OPTIONS=detect_stack_use_after_return=1")
	}
	cmdOutput, err := cmd.CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if listener := runOpts.Listener; listener != nil {
				listener.OnRun(exitErr.ExitCode(), string(cmdOutput))
			}
			return exitErr.ExitCode(), string(cmdOutput), nil
		}
		return 0, "", base.WrapErrorf(err, "run failed\n%s", string(cmdOutput))
	}
	exitCode = cmd.ProcessState.ExitCode()
	output = string(cmdOutput)
	if listener := runOpts.Listener; listener != nil {
		listener.OnRun(exitCode, output)
	}
	return exitCode, output, nil
}

// TimingListener records the wall-clock duration of each compiler phase.
type TimingListener struct {
	Threshold time.Duration
	start     time.Time
	last      time.Time
	steps     []step
}

type step struct {
	name     string
	duration time.Duration
}

func NewTimingListener(threshold time.Duration) *TimingListener {
	return &TimingListener{start: time.Now(), last: time.Now(), steps: nil, Threshold: threshold}
}

func (l *TimingListener) OnLex(_ []token.Token) bool {
	l.record("lex")
	return true
}

func (l *TimingListener) OnParse(_ *ast.AST, _ ast.NodeID, _ base.Diagnostics) bool {
	l.record("parse")
	return true
}

func (l *TimingListener) OnTypeCheck(_ *types.Engine, _ base.Diagnostics) bool {
	l.record("typecheck")
	return true
}

func (l *TimingListener) OnLifetimeCheck(_ *types.LifetimeCheck, _ base.Diagnostics) bool {
	l.record("lifetime")
	return true
}

func (l *TimingListener) OnIRGen(_ string) bool {
	l.record("irgen")
	return true
}

func (l *TimingListener) OnOptimizeIR() bool {
	l.record("optimize")
	return true
}

func (l *TimingListener) OnLink() bool {
	l.record("link")
	return true
}

func (l *TimingListener) OnRun(_ int, _ string) bool {
	l.record("run")
	return true
}

// Log prints every step that took longer than 10ms.
func (l *TimingListener) String() string {
	var sb strings.Builder
	overall := l.last.Sub(l.start)
	fmt.Fprintf(&sb, "compilation time : %s\n", overall.Round(time.Millisecond))
	for _, s := range l.steps {
		if s.duration >= l.Threshold {
			fmt.Fprintf(&sb, "  %-15s: %s\n", s.name, s.duration.Round(time.Millisecond))
		}
	}
	return sb.String()
}

// Total returns a display string of all step durations.
func (l *TimingListener) Total() string {
	var total time.Duration
	parts := make([]string, 0, len(l.steps))
	for _, s := range l.steps {
		total += s.duration
		parts = append(parts, fmt.Sprintf("%s=%s", s.name, s.duration.Round(time.Millisecond)))
	}
	return fmt.Sprintf("total=%s (%s)", total.Round(time.Millisecond), strings.Join(parts, ", "))
}

func (l *TimingListener) record(name string) {
	now := time.Now()
	l.steps = append(l.steps, step{name, now.Sub(l.last)})
	l.last = now
}

func queryTargetInfo(ctx context.Context, llvmHome string) (dataLayout, triple string, err error) {
	clang := filepath.Join(llvmHome, "bin", "clang")
	cmd := exec.CommandContext(ctx, clang, "-xc", "-S", "-emit-llvm", "-o", "-", "-")
	cmd.Stdin = strings.NewReader("")
	out, err := cmd.Output()
	if err != nil {
		return "", "", base.WrapErrorf(err, "failed to query target info from clang")
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		line = strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(line, "target datalayout = "); ok {
			dataLayout = strings.Trim(rest, `"`)
		}
		if rest, ok := strings.CutPrefix(line, "target triple = "); ok {
			triple = strings.Trim(rest, `"`)
		}
	}
	if dataLayout == "" || triple == "" {
		return "", "", base.Errorf("could not extract target info from clang output")
	}
	return dataLayout, triple, nil
}

func addRuntimeModule(a *ast.AST, opts CompileOpts) (ast.NodeID, base.Diagnostics) {
	for _, includePath := range opts.IncludePaths {
		path := filepath.Join(includePath, "runtime", "arena.met")
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		source := base.NewSource(path, "runtime::arena", false, []rune(string(content)))
		tokens := token.Lex(source)
		runtimeParser := ast.NewParser(tokens, a)
		moduleID, _ := runtimeParser.ParseModule()
		if len(runtimeParser.Diagnostics) > 0 {
			return 0, runtimeParser.Diagnostics
		}
		return moduleID, nil
	}
	return 0, nil
}

func resolveModules(
	a *ast.AST, opts CompileOpts,
) (*modules.ModuleResolution, base.Diagnostics) {
	res, diags := modules.ResolveModules(a, opts.ProjectRoot, opts.IncludePaths, os.ReadFile)
	if len(diags) > 0 {
		return nil, diags
	}
	return res, nil
}

func ModuleNameFromPath(path string, includePaths []string) string {
	name := strings.TrimSuffix(path, filepath.Ext(path))
	name = filepath.ToSlash(name)
	for _, inc := range includePaths {
		inc = filepath.ToSlash(inc)
		if stripped, ok := strings.CutPrefix(name, inc+"/"); ok {
			name = stripped
			break
		}
	}
	return strings.ReplaceAll(name, "/", "::")
}

func newMacroExpander(ctx context.Context, opts CompileOpts) types.MacroExpander {
	return func(macroSource string, funName string, args []macros.MacroArg) (string, error) {
		wrapperSource := macros.GenerateWrapper(macroSource, funName, args)
		source := base.NewSource("__macro__.met", "__macro__", true, []rune(wrapperSource))
		tmpDir, err := os.MkdirTemp("", "metallc-macro-*")
		if err != nil {
			return "", base.WrapErrorf(err, "failed to create temp dir for macro")
		}
		defer os.RemoveAll(tmpDir) //nolint:errcheck
		macroOpts := CompileOpts{  //nolint:exhaustruct
			IncludePaths:   opts.IncludePaths,
			Output:         filepath.Join(tmpDir, "macro"),
			KeepIR:         true,
			MinimalPrelude: false,
		}
		_, output, err := CompileAndRun(ctx, source, macroOpts)
		if err != nil {
			return "", base.WrapErrorf(err, "macro compilation failed")
		}
		return output, nil
	}
}

func run_cmd(ctx context.Context, cmdline []string) error {
	cmd := exec.CommandContext(ctx, cmdline[0], cmdline[1:]...) //nolint:gosec
	out, err := cmd.CombinedOutput()
	if err != nil {
		return base.WrapErrorf(err, "command failed\n%s\n%s", strings.Join(cmdline, " "), string(out))
	}
	return nil
}
