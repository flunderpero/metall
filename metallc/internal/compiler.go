package internal

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type CompileListener interface {
	OnLex(tokens []Token) bool
	OnParse(file *File, diagnostics Diagnostics) bool
	OnTypeCheck(typeEnv *TypeEnv, diagnostics Diagnostics) bool
	OnIRGen(ir string) bool
}

var ErrAbort = Errorf("aborted by listener")

type CompileOpts struct {
	Listener CompileListener
	Output   string
	KeepIR   bool
}

func Compile(ctx context.Context, source *Source, opts CompileOpts) error { //nolint:funlen
	listener := opts.Listener
	tokens := Lex(source)
	if listener != nil && !listener.OnLex(tokens) {
		return ErrAbort
	}
	parser := NewParser(tokens)
	file, _ := parser.ParseFile()
	if listener != nil && !listener.OnParse(&file, parser.Diagnostics) {
		return ErrAbort
	}
	if len(parser.Diagnostics) > 0 {
		return parser.Diagnostics
	}
	tc := NewTypeChecker()
	tc.VisitFile(&file)
	if listener != nil && !listener.OnTypeCheck(&tc.Env, tc.Diagnostics) {
		return ErrAbort
	}
	if len(tc.Diagnostics) > 0 {
		return tc.Diagnostics
	}
	ir, err := GenIR(&file, &tc.Env)
	if err != nil {
		return err
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

	base := filepath.Base(output)
	unopt_ll := filepath.Join(artifact_dir, base+".ll")
	opt_ll := filepath.Join(artifact_dir, base+".opt.ll")
	bc := filepath.Join(artifact_dir, base+".bc")

	// Write unoptimized IR (.ll)
	err = os.WriteFile(unopt_ll, []byte(ir), 0o600)
	if err != nil {
		return WrapErrorf(err, "failed to write IR file")
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

func CompileAndRun(ctx context.Context, source *Source, opts CompileOpts) (exitCode int, output string, err error) {
	runOpts := opts
	if opts.Output == "" {
		runOpts.Output = "/tmp/metallc"
	}
	defer func() {
		if opts.Output == "" {
			os.Remove(runOpts.Output) //nolint:errcheck,gosec
		}
	}()
	err = Compile(ctx, source, opts)
	if err != nil {
		return 0, "", err
	}
	cmd := exec.CommandContext(ctx, runOpts.Output) //nolint:gosec
	cmdOutput, err := cmd.CombinedOutput()
	if err != nil {
		return 0, "", WrapErrorf(err, "run failed\n%s", string(cmdOutput))
	}
	return cmd.ProcessState.ExitCode(), string(cmdOutput), nil
}

func run_cmd(ctx context.Context, cmdline []string) error {
	cmd := exec.CommandContext(ctx, cmdline[0], cmdline[1:]...) //nolint:gosec
	out, err := cmd.CombinedOutput()
	if err != nil {
		return WrapErrorf(err, "command failed\n%s\n%s", strings.Join(cmdline, " "), string(out))
	}
	return nil
}
