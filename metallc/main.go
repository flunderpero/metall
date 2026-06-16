package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/compiler"
	"github.com/flunderpero/metall/metallc/internal/gen"
)

type includeFlags []string

func (f *includeFlags) String() string { return fmt.Sprintf("%v", *f) }
func (f *includeFlags) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `metallc - the Metall compiler

Usage:
  metallc build [flags] <file.met>    Compile only
  metallc run   [flags] <file.met>    Compile and run

`)
	}
	flag.Parse()
	if len(flag.Args()) == 0 {
		flag.Usage()
		os.Exit(1)
	}
	switch flag.Arg(0) {
	case "build":
		opts, source, errorLimit := parseCommand("build")
		err := compiler.Compile(context.Background(), source, opts)
		if err != nil {
			reportError("failed to build:", err, errorLimit)
		}
	case "run":
		opts, source, errorLimit := parseCommand("run")
		exitCode, output, err := compiler.CompileAndRun(context.Background(), source, opts, false)
		if err != nil {
			reportError("failed to run:", err, errorLimit)
		}
		fmt.Print(output)
		os.Exit(exitCode)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", flag.Arg(0))
		flag.Usage()
		os.Exit(1)
	}
}

// reportError prints err to stderr and exits. Compiler diagnostics are
// truncated to errorLimit entries (errorLimit <= 0 prints all of them),
// followed by a summary of how many were hidden.
func reportError(prefix string, err error, errorLimit int) {
	var diags base.Diagnostics
	if !errors.As(err, &diags) {
		fmt.Fprintln(os.Stderr, prefix, err)
		os.Exit(1)
	}
	hidden := 0
	if errorLimit > 0 && len(diags) > errorLimit {
		hidden = len(diags) - errorLimit
		diags = diags[:errorLimit]
	}
	fmt.Fprintln(os.Stderr, prefix, diags.String())
	if hidden > 0 {
		plural := ""
		if hidden > 1 {
			plural = "s"
		}
		fmt.Fprintf(os.Stderr, "... %d more message%s (run with '--error-limit n' to see more)\n", hidden, plural)
	}
	os.Exit(1)
}

type tagFlags []string

func (f *tagFlags) String() string { return fmt.Sprintf("%v", *f) }
func (f *tagFlags) Set(value string) error {
	*f = append(*f, value)
	return nil
}

// linkFlags collects extra linker arguments passed straight through to clang
// at link time. Each value is split on whitespace (LDFLAGS-style), so a single
// `-link "-Lvendor/lib -lfoo -framework Cocoa"` expands to separate clang args.
type linkFlags []string

func (f *linkFlags) String() string { return fmt.Sprintf("%v", *f) }
func (f *linkFlags) Set(value string) error {
	*f = append(*f, strings.Fields(value)...)
	return nil
}

// sanitizeFlags collects the --sanitize selections. The flag is repeatable and
// each occurrence names one sanitizer; we validate against the closed set here
// rather than forwarding unknown names to clang/LLVM. "all" enables every
// implemented sanitizer.
type sanitizeFlags []gen.Sanitizer

func (f *sanitizeFlags) String() string { return fmt.Sprintf("%v", *f) }
func (f *sanitizeFlags) Set(value string) error {
	switch value {
	case "all":
		*f = append(*f, gen.AllSanitizers()...)
	case string(gen.SanitizerAddress):
		*f = append(*f, gen.SanitizerAddress)
	case string(gen.SanitizerAlignment):
		*f = append(*f, gen.SanitizerAlignment)
	default:
		return fmt.Errorf("unknown sanitizer %q (valid: all, address, alignment)", value)
	}
	return nil
}

func parseCommand(command string) (compiler.CompileOpts, *base.Source, int) { //nolint:funlen
	opts := compiler.NewCompileOptsWithDefaults()
	var includes includeFlags
	var tags tagFlags
	var link linkFlags
	var noErrtrace bool
	var errorLimit int
	flags := flag.NewFlagSet(command, flag.ExitOnError)
	flags.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: metallc %s [flags] <file.met>\n\n", command)
		flags.PrintDefaults()
	}
	flags.StringVar(&opts.Output, "o", "", "output binary path (build default: ./<name>)")
	flags.Var(&includes, "I", "add include path (repeatable)")
	flags.Var(&tags, "tag", "add compile-time tag (repeatable)")
	flags.Var(&link, "link", "extra linker flags passed to clang, LDFLAGS-style (repeatable, whitespace-split)")
	flags.BoolVar(&opts.PrintTiming, "timing", false, "print compilation timing")
	flags.BoolVar(&opts.KeepIR, "keep-ir", false, "keep intermediate .ll files next to the output")
	flags.Func("target", "compile target: native (default), wasm32, wasm64", func(s string) error {
		t, err := gen.ParseTarget(s)
		if err != nil {
			return base.WrapErrorf(err, "failed to parse -target")
		}
		opts.Target = t
		return nil
	})
	flags.Func("opt", "optimization mode: none, safe, fast", func(s string) error {
		level, err := compiler.ParseOptLevel(s)
		if err != nil {
			return base.WrapErrorf(err, "failed to parse -opt")
		}
		opts.OptLevel = level
		return nil
	})
	var sanitize sanitizeFlags
	flags.Var(
		&sanitize,
		"sanitize",
		"enable a runtime sanitizer (repeatable): all, address, alignment",
	)
	flags.BoolVar(
		&opts.MinimalPrelude,
		"minimal-prelude",
		false,
		"skip the stdlib prelude and load only the built-in types",
	)
	flags.BoolVar(&opts.EmitObject, "c", false, "emit a relocatable object file (.o) instead of linking an executable")
	flags.BoolVar(
		&opts.EmitHeaderFile,
		"emit-header-file",
		false,
		"write a C header (.h) declaring every `export`d function next to the output (native only)",
	)
	flags.BoolVar(
		&opts.EmitTypeScript,
		"emit-typescript",
		false,
		"write a TypeScript module (.ts) with the embedded wasm harness and typed wrappers for every `export` (wasm only)",
	)
	flags.BoolVar(&opts.PrintTypesDebug, "print-types-debug", false, "print type debug info to stderr")
	flags.BoolVar(&opts.PrintBindingsDebug, "print-bindings-debug", false, "print binding debug info to stderr")
	flags.BoolVar(&opts.DebugTypeCheck, "debug-typecheck", false, "enable verbose type checker debug output")
	flags.BoolVar(&opts.DebugLifetime, "print-lifetime-debug", false, "print lifetime analysis debug info")
	flags.BoolVar(&opts.DebugArenaAllocator, "arena-debug", false, "print arena allocations to stderr")
	flags.IntVar(&opts.ArenaStackBufSize, "arena-stack", 0, "arena inline stack buffer size")
	flags.IntVar(&opts.ArenaPageMinSize, "arena-min", 0, "arena min overflow page size")
	flags.IntVar(&opts.ArenaPageMaxSize, "arena-max", 0, "arena max overflow page size")
	flags.BoolVar(&noErrtrace, "no-errtrace", false, "disable automatic error return traces (on by default)")
	flags.IntVar(&errorLimit, "error-limit", 1, "maximum number of diagnostics to print, 0 for no limit")
	if err := flags.Parse(flag.Args()[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "failed to parse flags:", err)
		os.Exit(1)
	}
	if len(flags.Args()) != 1 {
		fmt.Fprintf(os.Stderr, "%s requires a file\n\n", command)
		flags.Usage()
		os.Exit(1)
	}
	if len(includes) == 0 {
		includes = []string{"lib"}
	}
	fileName := flags.Arg(0)
	if opts.Output == "" && command == "build" {
		baseName := filepath.Base(fileName)
		opts.Output = baseName[:len(baseName)-len(filepath.Ext(baseName))]
	}
	opts.ProjectRoot = filepath.Dir(fileName)
	opts.IncludePaths = includes
	opts.Tags = tags
	opts.LinkFlags = link
	opts.Sanitizers = sanitize
	opts.ErrorTracing = !noErrtrace
	opts.LLVMPasses = compiler.DefaultLLVMPasses
	src, err := os.ReadFile(fileName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to read file:", err)
		os.Exit(1)
	}
	moduleName := compiler.ModuleNameFromPath(fileName, includes)
	source := base.NewSource(fileName, moduleName, true, []rune(string(src)))
	return opts, source, errorLimit
}
