package compiler

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/backend"
	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/comptime"
	"github.com/flunderpero/metall/metallc/internal/gen"
	"github.com/flunderpero/metall/metallc/internal/macros"
	"github.com/flunderpero/metall/metallc/internal/modules"
	"github.com/flunderpero/metall/metallc/internal/token"
	"github.com/flunderpero/metall/metallc/internal/types"
)

type CompileListener interface {
	OnLex(tokens []token.Token) bool
	OnParse(a *ast.AST, fileID ast.NodeID, diagnostics base.Diagnostics) bool
	OnCompTime(a *ast.AST, diagnostics base.Diagnostics) bool
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
//   - sroa: Scalar Replacement of Aggregates, decomposes struct/array allocas into individual scalars.
//   - early-cse: Eliminate redundant loads. A slice header lives in memory, so repeated indexing
//     reloads its ptr/len; this collapses them, giving instcombine cleaner input.
//   - instcombine: Peephole optimizations: constant folding, strength reduction, dead code elimination.
//   - simplifycfg: Simplify the control flow graph by merging blocks and removing unreachable code.
//
// no-verify-fixpoint matches what the standard -O1/-O2/-O3 pipelines do. instcombine
// legitimately leaves trivially-dead code (e.g. allocas that only become dead once it
// constant-folds their loads) for a later DCE pass; the fixpoint verifier, a debug aid
// that bare `instcombine` enables but the -O pipelines do not, flags that as an error.
const DefaultLLVMPasses = "mem2reg,sroa,early-cse,instcombine<no-verify-fixpoint>,simplifycfg"

type CompileOpts struct {
	ProjectRoot  string
	IncludePaths []string
	Tags         []string
	LinkFlags    []string
	Listener     CompileListener
	PrintTiming  bool
	Output       string
	KeepIR       bool
	LLVMPasses   string
	OptLevel     OptLevel
	// TargetCPU, when set (e.g. "native", "apple-m1"), targets that CPU for codegen.
	// Empty targets a portable baseline so the binary runs on any CPU of the arch.
	TargetCPU string
	// TargetArch, when set ("x86_64"/"aarch64"), cross-compiles the native target
	// to that architecture instead of the host's. Empty means the host arch.
	TargetArch          string
	Sanitizers          []gen.Sanitizer
	DebugArenaAllocator bool
	ArenaStackBufSize   int
	ArenaPageMinSize    int
	ArenaPageMaxSize    int
	ErrorTracing        bool
	MinimalPrelude      bool
	Target              gen.Target
	PrintTypesDebug     bool
	PrintBindingsDebug  bool
	DebugTypeCheck      bool
	DebugLifetime       bool
	EmitObject          bool
	EmitHeaderFile      bool
	EmitTypeScript      bool
}

// NewCompileOptsWithDefaults returns CompileOpts with the runtime defaults set,
// including error tracing on.
func NewCompileOptsWithDefaults() CompileOpts {
	return CompileOpts{ErrorTracing: true}.withArenaDefaults() //nolint:exhaustruct
}

// withArenaDefaults fills any unset (zero) arena size with its default, for
// callers that build a partial CompileOpts.
func (o CompileOpts) withArenaDefaults() CompileOpts {
	if o.ArenaStackBufSize == 0 {
		o.ArenaStackBufSize = 1024
	}
	if o.ArenaPageMinSize == 0 {
		o.ArenaPageMinSize = 4096
	}
	if o.ArenaPageMaxSize == 0 {
		o.ArenaPageMaxSize = 65536
	}
	return o
}

func Compile(ctx context.Context, source *base.Source, opts CompileOpts) error { //nolint:funlen
	opts = opts.withArenaDefaults()
	if opts.EmitHeaderFile && opts.Target.IsWasm() {
		return base.Errorf("--emit-header-file is only valid for the native target")
	}
	if opts.EmitTypeScript && !opts.Target.IsWasm() {
		return base.Errorf("--emit-typescript is only valid for wasm targets")
	}
	if opts.TargetCPU != "" && opts.Target.IsWasm() {
		return base.Errorf("--cpu is only valid for the native target")
	}
	if opts.TargetArch != "" && opts.Target.IsWasm() {
		return base.Errorf("--arch is only valid for the native target")
	}
	if slices.Contains(opts.Sanitizers, gen.SanitizerAddress) && opts.Target.IsWasm() {
		return base.Errorf("the address sanitizer is not supported for wasm targets")
	}
	targetTriple, err := targetTriple(opts.Target, opts.TargetArch)
	if err != nil {
		return err
	}
	targetDataLayout, err := backend.DataLayout(targetTriple)
	if err != nil {
		return base.WrapErrorf(err, "failed to query target data layout")
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
	parseDiagnostics := parser.Diagnostics
	timingListener.OnParse(parser.AST, fileID, parseDiagnostics)
	if listener != nil && !listener.OnParse(parser.AST, fileID, parseDiagnostics) {
		return ErrAbort
	}
	if len(parseDiagnostics) > 0 {
		return parseDiagnostics
	}
	runtimeModuleIDs, postParseDiags := addRuntimeModules(parser.AST, opts)
	if len(postParseDiags) > 0 {
		return postParseDiags
	}
	compTimeEnv := buildCompTimeEnv(targetTriple, opts.Tags, opts.ErrorTracing)
	moduleResolution, moduleDiags := resolveModules(parser.AST, opts, compTimeEnv)
	timingListener.OnCompTime(parser.AST, moduleDiags)
	if listener != nil && !listener.OnCompTime(parser.AST, moduleDiags) {
		return ErrAbort
	}
	if len(moduleDiags) > 0 {
		return moduleDiags
	}
	preludeAST, _ := ast.PreludeAST(opts.MinimalPrelude)
	engine := types.NewEngine(parser.AST, preludeAST, moduleResolution, newMacroExpander(ctx, opts))
	if opts.DebugTypeCheck {
		engine.SetDebug(base.NewStdoutDebug("types"))
	}
	engine.Query(fileID)
	for _, id := range runtimeModuleIDs {
		engine.Query(id)
	}
	engine.AssignEnumTags()
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
		engine.AST(), module, engine.Funs(), engine.Structs(), engine.Unions(), engine.Enums(),
		engine.Consts(), engine.Exports(),
		gen.IROpts{
			TargetDataLayout:        targetDataLayout,
			TargetTriple:            targetTriple,
			ArithmeticOverflowCheck: opts.OptLevel != OptLevelFast,
			Sanitizers:              opts.Sanitizers,
			ArenaDebug:              opts.DebugArenaAllocator,
			ArenaStackBufSize:       opts.ArenaStackBufSize,
			ArenaPageMinSize:        opts.ArenaPageMinSize,
			ArenaPageMaxSize:        opts.ArenaPageMaxSize,
			ErrorTracing:            opts.ErrorTracing,
			Target:                  opts.Target,
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
		artifact_dir, err = os.MkdirTemp("", "metallc-artifacts-*")
		if err != nil {
			return base.WrapErrorf(err, "failed to create temp dir for artifacts")
		}
		defer func() {
			_ = os.RemoveAll(artifact_dir)
		}()
	}

	filebase := filepath.Base(output)

	if opts.KeepIR {
		unoptLL := filepath.Join(artifact_dir, filebase+".ll")
		if err := os.WriteFile(unoptLL, []byte(ir), 0o600); err != nil {
			return base.WrapErrorf(err, "failed to write IR file")
		}
	}

	// At -O none the passes are a bare function pipeline; at -O3 the full
	// `default<O3>` module pipeline. asan is a module pass that runs last, so a
	// function pipeline must be wrapped before appending it.
	passes := opts.LLVMPasses
	moduleLevel := false
	codegen := backend.CodeGenNone
	if opts.OptLevel != OptLevelNone {
		passes = "default<O3>"
		moduleLevel = true
		codegen = backend.CodeGenAggressive
	}
	if slices.Contains(opts.Sanitizers, gen.SanitizerAddress) {
		switch {
		case passes == "":
			passes = "asan"
		case moduleLevel:
			passes += ",asan"
		default:
			passes = "function(" + passes + "),asan"
		}
	}
	objectPath := output
	if !opts.EmitObject {
		objectPath = filepath.Join(artifact_dir, filebase+".o")
	}
	if err := backend.EmitObject(
		[]byte(ir), targetTriple, opts.TargetCPU, passes, codegen, objectPath,
	); err != nil {
		return base.WrapErrorf(err, "failed to compile to object")
	}
	timingListener.OnOptimizeIR()
	if listener != nil && !listener.OnOptimizeIR() {
		return ErrAbort
	}

	exportNames := make([]string, 0, len(engine.Exports()))
	for _, exp := range engine.Exports() {
		exportNames = append(exportNames, exp.CName)
	}
	if !opts.EmitObject {
		if err := linkExecutable(ctx, opts, targetTriple, objectPath, output, exportNames); err != nil {
			return err
		}
	}
	outBase := strings.TrimSuffix(output, filepath.Ext(output))
	if opts.EmitHeaderFile {
		header := gen.GenCHeader(engine.AST(), module, engine.Exports())
		if err := os.WriteFile(outBase+".h", []byte(header), 0o600); err != nil {
			return base.WrapErrorf(err, "failed to write C header")
		}
	}
	if opts.EmitTypeScript {
		ts := gen.GenTypeScript(engine.AST(), module, engine.Exports())
		if err := os.WriteFile(outBase+".ts", []byte(ts), 0o600); err != nil {
			return base.WrapErrorf(err, "failed to write TypeScript bindings")
		}
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
	captureStdoutStderr bool,
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
	var cmd *exec.Cmd
	if opts.Target.IsWasm() {
		cmdline, cleanup, hErr := wasmRunCommand(runOpts.Output)
		if hErr != nil {
			return 0, "", hErr
		}
		defer cleanup()
		cmd = exec.CommandContext(ctx, cmdline[0], cmdline[1:]...) //nolint:gosec
	} else {
		cmd = exec.CommandContext(ctx, runOpts.Output) //nolint:gosec
	}
	if slices.Contains(opts.Sanitizers, gen.SanitizerAddress) {
		cmd.Env = append(os.Environ(), "ASAN_OPTIONS=detect_stack_use_after_return=1")
	}
	var cmdOutput []byte
	if captureStdoutStderr {
		cmdOutput, err = cmd.CombinedOutput()
	} else {
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		err = cmd.Run()
	}
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

func (l *TimingListener) OnCompTime(_ *ast.AST, _ base.Diagnostics) bool {
	l.record("comptime")
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

// targetTriple is the single source of truth for what to build: the wasm
// triples, or the host OS with the chosen architecture for native. arch is the
// --arch value ("" = host, "x86_64", "aarch64").
func targetTriple(target gen.Target, arch string) (string, error) {
	switch target { //nolint:exhaustive
	case gen.TargetWasm32:
		return "wasm32-unknown-unknown", nil
	case gen.TargetWasm64:
		return "wasm64-unknown-unknown", nil
	}
	switch arch {
	case "":
		arch = hostArch()
	case "x86_64", "aarch64":
	default:
		return "", base.Errorf("unsupported --arch %q (supported: x86_64, aarch64)", arch)
	}
	switch runtime.GOOS {
	case "darwin":
		if arch == "aarch64" {
			arch = "arm64"
		}
		return arch + "-apple-macosx11.0.0", nil
	case "linux":
		return arch + "-unknown-linux-gnu", nil
	default:
		return backend.DefaultTriple(), nil
	}
}

// hostArch is the host architecture in LLVM triple naming.
func hostArch() string {
	if runtime.GOARCH == "amd64" {
		return "x86_64"
	}
	return "aarch64"
}

func linkExecutable(
	ctx context.Context, opts CompileOpts, triple, objectPath, output string, exportNames []string,
) error {
	if opts.Target.IsWasm() {
		return linkWasm(opts, objectPath, output, exportNames)
	}
	switch runtime.GOOS {
	case "darwin":
		return linkMachO(ctx, opts, triple, objectPath, output)
	case "linux":
		return linkELF(ctx, opts, triple, objectPath, output)
	default:
		return base.Errorf("in-process linking for %s is not implemented (darwin, linux, wasm)", runtime.GOOS)
	}
}

// linkELF builds the ld.lld command line clang's driver produces for a Linux
// PIE executable and runs it in-process. The crt objects, gcc support libs, and
// search paths come from the system toolchain (queried via `cc`), the
// irreducible dependency for native Linux linking, just like the macOS SDK.
func linkELF(ctx context.Context, opts CompileOpts, triple, objectPath, output string) error {
	// The crt objects and libc come from the host toolchain (via `cc`), so a
	// non-host arch would need a target sysroot we do not have.
	arch, _, _ := strings.Cut(triple, "-")
	if arch != hostArch() {
		return base.Errorf(
			"cross-architecture linking (%s on a %s host) needs a target sysroot",
			arch,
			hostArch(),
		)
	}
	crt := func(name string) string {
		out, err := exec.CommandContext(ctx, "cc", "-print-file-name="+name).Output() //nolint:gosec
		if err != nil {
			return name
		}
		return strings.TrimSpace(string(out))
	}
	scrt1, crti, crtn := crt("Scrt1.o"), crt("crti.o"), crt("crtn.o")
	crtbegin, crtend := crt("crtbeginS.o"), crt("crtendS.o")

	var dynLinker, emulation string
	switch arch {
	case "aarch64":
		dynLinker, emulation = "/lib/ld-linux-aarch64.so.1", "aarch64linux"
	case "x86_64":
		dynLinker, emulation = "/lib64/ld-linux-x86-64.so.2", "elf_x86_64"
	default:
		return base.Errorf("unsupported linux arch: %s", arch)
	}

	// -L both the gcc dir (crtbegin, libgcc) and the libc multiarch dir (crti,
	// libc/libm); lld's default search does not include Debian's triple dirs.
	args := []string{
		"ld.lld", "-m", emulation, "--eh-frame-hdr", "-pie",
		"-dynamic-linker", dynLinker,
		"-o", output,
		scrt1, crti, crtbegin,
		"-L", filepath.Dir(crtbegin),
		"-L", filepath.Dir(crti),
		objectPath,
	}
	if slices.Contains(opts.Sanitizers, gen.SanitizerAddress) {
		rd, err := resourceDir()
		if err != nil {
			return err
		}
		// compiler-rt's per-triple runtime dir. The asan runtime is whole-archived
		// so its interceptors survive --gc-sections; asan_static carries the parts
		// the instrumented code calls directly. The runtime is C++, hence -lstdc++.
		rtDir := filepath.Join(rd, "lib", arch+"-unknown-linux-gnu")
		args = append(args,
			"--whole-archive", filepath.Join(rtDir, "libclang_rt.asan.a"), "--no-whole-archive",
			filepath.Join(rtDir, "libclang_rt.asan_static.a"),
			"--export-dynamic",
			"-lstdc++", "-lpthread", "-lrt", "-ldl", "-lresolv",
		)
	}
	args = append(args, opts.LinkFlags...)
	args = append(args, "-lgcc", "--as-needed", "-lgcc_s", "--no-as-needed", "-lc", "-lm", crtend, crtn)
	if err := backend.LinkELF(args); err != nil {
		return base.WrapErrorf(err, "failed to link executable")
	}
	return nil
}

// linkMachO builds the ld64.lld command line clang's driver would have produced
// for a macOS executable and runs it in-process. macOS forbids static linking
// of libSystem, so it is always dynamic, reached via the SDK's -syslibroot.
func linkMachO(ctx context.Context, opts CompileOpts, triple, objectPath, output string) error {
	sdk, sdkVersion, err := macOSSDK(ctx)
	if err != nil {
		return err
	}
	arch, _, _ := strings.Cut(triple, "-")
	args := []string{
		"ld64.lld",
		"-arch", arch,
		"-platform_version", "macos", macOSDeploymentTarget(triple), sdkVersion,
		"-syslibroot", sdk,
		"-lSystem",
		"-o", output, objectPath,
	}
	if slices.Contains(opts.Sanitizers, gen.SanitizerAddress) {
		rd, err := resourceDir()
		if err != nil {
			return err
		}
		// The instrumented object references __asan_* from the compiler-rt
		// runtime dylib; -rpath lets the linked binary find it at run time.
		rtDir := filepath.Join(rd, "lib", "darwin")
		args = append(args,
			filepath.Join(rtDir, "libclang_rt.asan_osx_dynamic.dylib"),
			"-rpath", rtDir,
		)
	}
	args = append(args, opts.LinkFlags...)
	if err := backend.LinkMachO(args); err != nil {
		return base.WrapErrorf(err, "failed to link executable")
	}
	return nil
}

// linkWasm builds the wasm-ld command line and runs it in-process. The flags
// match the old `clang --target=wasm32 -nostdlib -Wl,...` link: no entry point,
// the runtime exports, and a 1 MiB shadow stack (wasm-ld's 64 KiB default
// overflows on modest recursion). User -link flags come last so an explicit
// stack-size overrides the default.
func linkWasm(opts CompileOpts, objectPath, output string, exportNames []string) error {
	args := []string{ //nolint:prealloc
		"wasm-ld",
		"--no-entry",
		"--export=main",
		"--export=memory",
		"--allow-undefined",
		"-z", fmt.Sprintf("stack-size=%d", wasmShadowStackSize),
	}
	for _, name := range exportNames {
		args = append(args, "--export="+name)
	}
	args = append(args, opts.LinkFlags...)
	args = append(args, "-o", output, objectPath)
	if err := backend.LinkWasm(args); err != nil {
		return base.WrapErrorf(err, "failed to link wasm module")
	}
	return nil
}

// resourceDir locates the clang resource dir holding the compiler-rt sanitizer
// runtimes. Those are not linked into metallc, so they are found on disk: an
// explicit override, next to the binary in a release, or the static LLVM build
// tree during development. Only called when a sanitizer needs them.
func resourceDir() (string, error) {
	if d := os.Getenv("METALL_RESOURCE_DIR"); d != "" {
		return d, nil
	}
	// The resource dir is the single lib/clang/<major> under a search root.
	find := func(root string) string {
		matches, _ := filepath.Glob(filepath.Join(root, "lib", "clang", "*"))
		if len(matches) == 1 {
			return matches[0]
		}
		return ""
	}
	if exe, err := os.Executable(); err == nil {
		if d := find(filepath.Dir(exe)); d != "" {
			return d, nil
		}
	}
	platform := runtime.GOOS + "-" + runtime.GOARCH
	cwd, err := os.Getwd()
	if err != nil {
		return "", base.WrapErrorf(err, "get cwd")
	}
	for dir := cwd; ; {
		if d := find(filepath.Join(dir, ".build", "llvm-static", platform)); d != "" {
			return d, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", base.Errorf("compiler-rt resource dir not found; set METALL_RESOURCE_DIR")
		}
		dir = parent
	}
}

// macOSDeploymentTarget extracts the deployment version from a macOS triple
// (e.g. "arm64-apple-macosx26.0.0" -> "26.0"), so the linked binary's minimum
// matches the object the compiler emitted and lld does not warn. Falls back to
// a broad baseline when the triple carries no version.
func macOSDeploymentTarget(triple string) string {
	_, version, ok := strings.Cut(triple, "macosx")
	if !ok || version == "" {
		return "11.0"
	}
	parts := strings.SplitN(version, ".", 3)
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return parts[0]
}

// macOSSDK returns the active SDK path and its version, used for -syslibroot
// and -platform_version. These come from the host toolchain, the irreducible
// dependency for native macOS linking.
func macOSSDK(ctx context.Context) (path, version string, err error) {
	out, err := exec.CommandContext(ctx, "xcrun", "--show-sdk-path").Output()
	if err != nil {
		return "", "", base.WrapErrorf(err, "failed to locate macOS SDK via xcrun")
	}
	path = strings.TrimSpace(string(out))
	verOut, err := exec.CommandContext( //nolint:gosec // fixed argv, path from xcrun
		ctx, "plutil", "-extract", "Version", "raw", filepath.Join(path, "SDKSettings.plist"),
	).Output()
	if err != nil {
		return "", "", base.WrapErrorf(err, "failed to read SDK version")
	}
	return path, strings.TrimSpace(string(verOut)), nil
}

func addRuntimeModules(a *ast.AST, opts CompileOpts) ([]ast.NodeID, base.Diagnostics) {
	files := []struct{ rel, module string }{
		{filepath.Join("runtime", "arena.met"), "runtime::arena"},
	}
	if opts.ErrorTracing {
		files = append(files, struct{ rel, module string }{
			filepath.Join("std", "errors.met"), "std::errors",
		})
	}
	if opts.Target.IsWasm() {
		// Provides malloc/realloc/free that arena.met aliases via extern.
		files = append(files, struct{ rel, module string }{
			filepath.Join("runtime", "wasmalloc.met"), "runtime::wasmalloc",
		})
	}
	var ids []ast.NodeID
	for _, f := range files {
		id, diags := loadRuntimeFile(a, opts, f.rel, f.module)
		if len(diags) > 0 {
			return nil, diags
		}
		if id != 0 {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func loadRuntimeFile(
	a *ast.AST, opts CompileOpts, relPath, moduleName string,
) (ast.NodeID, base.Diagnostics) {
	for _, includePath := range opts.IncludePaths {
		path := filepath.Join(includePath, relPath)
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		source := base.NewSource(path, moduleName, false, []rune(string(content)))
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
	a *ast.AST, opts CompileOpts, compTimeEnv comptime.Env,
) (*modules.ModuleResolution, base.Diagnostics) {
	res, diags := modules.ResolveModules(
		a, opts.ProjectRoot, opts.IncludePaths, compTimeEnv, os.ReadFile,
	)
	if len(diags) > 0 {
		return nil, diags
	}
	return res, nil
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
		_, output, err := CompileAndRun(ctx, source, macroOpts, true)
		if err != nil {
			return "", base.WrapErrorf(err, "macro compilation failed")
		}
		return output, nil
	}
}

func buildCompTimeEnv(targetTriple string, tags []string, errorTracing bool) comptime.Env {
	// Parse target triple: <arch>-<vendor>-<os> or <arch>-<vendor>-<os>-<env>
	parts := strings.SplitN(targetTriple, "-", 4)
	arch := ""
	osName := ""
	if len(parts) >= 1 {
		arch = parts[0]
	}
	if len(parts) >= 3 {
		osName = parts[2]
	}

	env := comptime.Env{
		"os": {
			"darwin":  osName == "darwin" || strings.HasPrefix(osName, "macosx"),
			"linux":   osName == "linux",
			"windows": osName == "windows" || strings.HasPrefix(osName, "win32"),
			"wasm":    osName == "wasi" || osName == "emscripten" || strings.HasPrefix(arch, "wasm"),
		},
		"arch": {
			"aarch64": arch == "aarch64" || arch == "arm64",
			"x86_64":  arch == "x86_64",
			"wasm32":  arch == "wasm32",
			"wasm64":  arch == "wasm64",
		},
		"endian": {
			"little": true, // All currently supported targets are little-endian.
			"big":    false,
		},
		"flags": {
			"errtrace": errorTracing,
		},
		"tag": {},
	}
	for _, t := range tags {
		env["tag"][t] = true
	}
	return env
}

// wasmRunCommand writes the embedded JS harness to a temp file and
// returns the node invocation that dynamically imports it and runs the
// wasm module at wasmPath. The cleanup func deletes the temp dir.
func wasmRunCommand(wasmPath string) ([]string, func(), error) {
	absWasm, err := filepath.Abs(wasmPath)
	if err != nil {
		return nil, func() {}, base.WrapErrorf(err, "abs wasm path")
	}
	tmpDir, err := os.MkdirTemp("", "metallc-wasm-run-*")
	if err != nil {
		return nil, func() {}, base.WrapErrorf(err, "temp dir for wasm harness")
	}
	cleanup := func() { _ = os.RemoveAll(tmpDir) }
	harnessPath := filepath.Join(tmpDir, "wasm_harness.ts")
	if err := os.WriteFile(harnessPath, []byte(gen.WasmHarnessTS()), 0o600); err != nil {
		cleanup()
		return nil, func() {}, base.WrapErrorf(err, "write wasm harness")
	}
	harnessURL := "file://" + harnessPath
	// Provide an improved `write` function when run with `node`. (The default impl has to
	// work around console.log always adding newlines.)
	runOpts := `{ write: (fd, text) => (fd === 2 ? process.stderr : process.stdout).write(text) }`
	if os.Getenv("METALL_WASM_RUN_USE_DEFAULT_WRITE") != "" {
		runOpts = `{}`
	}
	script := fmt.Sprintf(
		`
			import("node:fs").then(fs =>
				import(%q).then(m =>
					m.runMetall(fs.readFileSync(%q), %s)
				)
			).then(c => {
				if (c !== 0) {
					process.exit(c)
				}
			})
		`,
		harnessURL, absWasm, runOpts,
	)
	return []string{"node", "--input-type=module", "-e", script}, cleanup, nil
}

// wasmShadowStackSize is the wasm linear-memory stack reservation. wasm-ld
// defaults to 64 KiB, which a modestly deep recursion silently overflows; 1 MiB
// is the common wasm stack size (matching Rust's wasm32 default).
const wasmShadowStackSize = 1 << 20
