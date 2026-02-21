package internal

import (
	"context"
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
}

var ErrAbort = base.Errorf("aborted by listener")

type CompileOpts struct {
	Listener CompileListener
	Output   string
	KeepIR   bool
}

func Compile(ctx context.Context, source *base.Source, opts CompileOpts) error { //nolint:funlen
	listener := opts.Listener
	tokens := token.Lex(source)
	if listener != nil && !listener.OnLex(tokens) {
		return ErrAbort
	}
	parser := ast.NewParser(tokens)
	fileID, _ := parser.ParseFile()
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
	lifetime := types.NewLifetimeAnalyzer(engine)
	lifetime.Check(fileID)
	if listener != nil && !listener.OnLifetimeCheck(lifetime, lifetime.Diagnostics) {
		return ErrAbort
	}
	if len(lifetime.Diagnostics) > 0 {
		return lifetime.Diagnostics
	}
	ir, err := gen.GenIR(fileID, engine)
	if err != nil {
		return err //nolint:wrapcheck
	}
	if listener != nil && !listener.OnIRGen(ir) {
		return ErrAbort
	}
	output := opts.Output
	if output == "" {
		output = source.Name[0 : len(source.Name)-len(filepath.Ext(source.Name))]
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

	// Default passes.
	// `mem2reg` is required because we lazily use `alloca` a lot.
	passes := "mem2reg,instcombine,simplifycfg"
	// passes := "module(function(mem2reg,instcombine,simplifycfg),cgscc(inline),function(instcombine,simplifycfg))"

	// Produce optimized textual IR (.opt.ll)
	cmdline := []string{"opt", "-S", "-passes=" + passes, unopt_ll, "-o", opt_ll}
	if err := run_cmd(ctx, cmdline); err != nil {
		return err
	}

	// Produce optimized bitcode (.bc)
	cmdline = []string{"opt", "-passes=" + passes, unopt_ll, "-o", bc}
	if err := run_cmd(ctx, cmdline); err != nil {
		return err
	}

	// Compile from optimized bitcode.
	cmdline = []string{"clang", bc, "-o", output}
	if err := run_cmd(ctx, cmdline); err != nil {
		return err
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
	cmdOutput, err := cmd.CombinedOutput()
	if err != nil {
		return 0, "", base.WrapErrorf(err, "run failed\n%s", string(cmdOutput))
	}
	return cmd.ProcessState.ExitCode(), string(cmdOutput), nil
}

func run_cmd(ctx context.Context, cmdline []string) error {
	cmd := exec.CommandContext(ctx, cmdline[0], cmdline[1:]...) //nolint:gosec
	out, err := cmd.CombinedOutput()
	if err != nil {
		return base.WrapErrorf(err, "command failed\n%s\n%s", strings.Join(cmdline, " "), string(out))
	}
	return nil
}
