package internal

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/gen"
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

// LLVM optimization passes (https://llvm.org/docs/Passes.html):
//   - mem2reg: Promote alloca'd scalars to SSA registers.
//   - sroa: Scalar Replacement of Aggregates — decompose struct/array allocas into individual scalars.
//   - instcombine: Peephole optimizations — constant folding, strength reduction, dead code elimination.
//   - simplifycfg: Simplify the control flow graph — merge blocks, remove unreachable code.
const DefaultLLVMPasses = "mem2reg,sroa,instcombine,simplifycfg"

type CompileOpts struct {
	Listener         CompileListener
	Output           string
	KeepIR           bool
	LLVMPasses       string
	AddressSanitizer bool
}

func Compile(ctx context.Context, source *base.Source, opts CompileOpts) error { //nolint:funlen
	llvmHome := os.Getenv("LLVM_HOME")
	if llvmHome == "" {
		return base.Errorf("LLVM_HOME not set")
	}

	listener := opts.Listener
	tokens := token.Lex(source)
	if listener != nil && !listener.OnLex(tokens) {
		return ErrAbort
	}
	parser := ast.NewParser(tokens)
	fileID, _ := parser.ParseModule()
	if listener != nil && !listener.OnParse(parser.AST, fileID, parser.Diagnostics) {
		return ErrAbort
	}
	if len(parser.Diagnostics) > 0 {
		return parser.Diagnostics
	}
	engine := types.NewEngine(parser.AST)
	engine.Query(fileID)
	if listener != nil && !listener.OnTypeCheck(engine, engine.Diagnostics) {
		return ErrAbort
	}
	if len(engine.Diagnostics) > 0 {
		return engine.Diagnostics
	}
	lifetime := types.NewLifetimeAnalyzer(parser.AST, engine.Env())
	lifetime.Check(fileID)
	if listener != nil && !listener.OnLifetimeCheck(lifetime, lifetime.Diagnostics) {
		return ErrAbort
	}
	if len(lifetime.Diagnostics) > 0 {
		return lifetime.Diagnostics
	}
	module := base.Cast[ast.Module](engine.Node(fileID).Kind)
	funs, structs := engine.BuildWorkList(module)
	ir, err := gen.GenIR(parser.AST, module, funs, structs, gen.IROpts{AddressSanitizer: opts.AddressSanitizer})
	if err != nil {
		return err //nolint:wrapcheck
	}
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
	cmdline := []string{llvmHome + "/bin/opt", "-S", "-passes=" + opts.LLVMPasses, unopt_ll, "-o", opt_ll}
	if err := run_cmd(ctx, cmdline); err != nil {
		return base.WrapErrorf(err, "failed to generate optimized IR")
	}
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
	if opts.AddressSanitizer {
		cmdline = append(cmdline, "-fsanitize=address")
	}
	if err := run_cmd(ctx, cmdline); err != nil {
		return base.WrapErrorf(err, "failed to compile with clang")
	}
	if listener != nil && !listener.OnLink() {
		return ErrAbort
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

func ModuleNameFromPath(path string) string {
	name := strings.TrimSuffix(path, filepath.Ext(path))
	name = filepath.ToSlash(name)
	return strings.ReplaceAll(name, "/", ".")
}

func run_cmd(ctx context.Context, cmdline []string) error {
	cmd := exec.CommandContext(ctx, cmdline[0], cmdline[1:]...) //nolint:gosec
	out, err := cmd.CombinedOutput()
	if err != nil {
		return base.WrapErrorf(err, "command failed\n%s\n%s", strings.Join(cmdline, " "), string(out))
	}
	return nil
}
