package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/compiler"
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
		opts, source := parseCommand("build")
		err := compiler.Compile(context.Background(), source, opts)
		if err != nil {
			fmt.Fprintln(os.Stderr, "failed to build:", err)
			os.Exit(1)
		}
	case "run":
		opts, source := parseCommand("run")
		exitCode, output, err := compiler.CompileAndRun(context.Background(), source, opts, false)
		if err != nil {
			fmt.Fprintln(os.Stderr, "failed to run:", err)
			os.Exit(1)
		}
		fmt.Print(output)
		os.Exit(exitCode)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", flag.Arg(0))
		flag.Usage()
		os.Exit(1)
	}
}

func parseCommand(command string) (compiler.CompileOpts, *base.Source) {
	var opts compiler.CompileOpts
	var includes includeFlags
	flags := flag.NewFlagSet(command, flag.ExitOnError)
	flags.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: metallc %s [flags] <file.met>\n\n", command)
		flags.PrintDefaults()
	}
	flags.StringVar(&opts.Output, "o", "", "output binary path (build default: ./<name>)")
	flags.Var(&includes, "I", "add include path (repeatable)")
	flags.BoolVar(&opts.PrintTiming, "timing", false, "print compilation timing")
	flags.BoolVar(&opts.KeepIR, "keep-ir", false, "keep intermediate .ll files next to the output")
	flags.Func("opt", "optimization mode: none, safe, fast", func(s string) error {
		level, err := compiler.ParseOptLevel(s)
		if err != nil {
			return base.WrapErrorf(err, "failed to parse -opt")
		}
		opts.OptLevel = level
		return nil
	})
	flags.BoolVar(&opts.AddressSanitizer, "asan", false, "enable AddressSanitizer")
	flags.BoolVar(&opts.PrintTypesDebug, "print-types-debug", false, "print type debug info to stderr")
	flags.BoolVar(&opts.PrintBindingsDebug, "print-bindings-debug", false, "print binding debug info to stderr")
	flags.BoolVar(&opts.DebugTypeCheck, "debug-typecheck", false, "enable verbose type checker debug output")
	flags.BoolVar(&opts.DebugLifetime, "print-lifetime-debug", false, "print lifetime analysis debug info")
	flags.BoolVar(&opts.DebugArenaAllocator, "arena-debug", false, "print arena allocations to stderr")
	flags.IntVar(&opts.ArenaStackBufSize, "arena-stack", 0, "arena inline stack buffer size")
	flags.IntVar(&opts.ArenaPageMinSize, "arena-min", 0, "arena min overflow page size")
	flags.IntVar(&opts.ArenaPageMaxSize, "arena-max", 0, "arena max overflow page size")
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
	opts.LLVMPasses = compiler.DefaultLLVMPasses
	src, err := os.ReadFile(fileName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to read file:", err)
		os.Exit(1)
	}
	moduleName := compiler.ModuleNameFromPath(fileName, includes)
	source := base.NewSource(fileName, moduleName, true, []rune(string(src)))
	return opts, source
}
