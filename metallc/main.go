package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/flunderpero/metall/metallc/internal"
	"github.com/flunderpero/metall/metallc/internal/base"
)

func main() {
	flag.Usage = func() {
		fmt.Println("Usage: metallc <build|run> ...")
		fmt.Println("  build <file.met>: build the program")
		fmt.Println("  run <file.met>:   run the program")
		flag.PrintDefaults()
	}
	flag.Parse()
	if len(flag.Args()) == 0 {
		flag.Usage()
		os.Exit(1)
	}
	switch flag.Arg(0) {
	case "build":
		fmt.Fprintln(os.Stderr, "build is not implemented yet")
		os.Exit(1)
	case "run":
		var opts internal.CompileOpts
		flags := flag.NewFlagSet("run", flag.ExitOnError)
		flags.BoolVar(&opts.DebugArenaAllocator, "arena-debug", false, "print arena allocations to stderr")
		flags.IntVar(&opts.ArenaStackBufSize, "arena-stack", 0, "arena inline stack buffer size (default 32)")
		flags.IntVar(&opts.ArenaPageMinSize, "arena-min", 0, "arena min overflow page size (default 256)")
		flags.IntVar(&opts.ArenaPageMaxSize, "arena-max", 0, "arena max overflow page size (default 65536)")
		if err := flags.Parse(flag.Args()[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "failed to parse flags:", err)
			os.Exit(1)
		}
		if len(flags.Args()) != 1 {
			fmt.Fprintln(os.Stderr, "run requires a file to run")
			flag.Usage()
			os.Exit(1)
		}
		fileName := flags.Arg(0)
		src, err := os.ReadFile(fileName)
		if err != nil {
			fmt.Fprintln(os.Stderr, "failed to read file:", err)
			os.Exit(1)
		}
		moduleName := internal.ModuleNameFromPath(fileName)
		source := base.NewSource(fileName, moduleName, true, []rune(string(src)))
		opts.KeepIR = true
		opts.AddressSanitizer = true
		opts.LLVMPasses = internal.DefaultLLVMPasses
		exitCode, output, err := internal.CompileAndRun(context.Background(), source, opts)
		if err != nil {
			fmt.Fprintln(os.Stderr, "failed to run:", err)
			os.Exit(1)
		}
		fmt.Println(output)
		os.Exit(exitCode)
	default:
		fmt.Fprintln(os.Stderr, "unknown command:", flag.Arg(0))
		flag.Usage()
		os.Exit(1)
	}
}
