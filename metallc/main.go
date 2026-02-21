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
		if len(flag.Args()) < 2 {
			fmt.Fprintln(os.Stderr, "run requires a file to run")
			flag.Usage()
			os.Exit(1)
		}
		src, err := os.ReadFile(flag.Arg(1))
		if err != nil {
			fmt.Fprintln(os.Stderr, "failed to read file:", err)
			os.Exit(1)
		}
		source := base.NewSource(flag.Arg(1), []rune(string(src)))
		exitCode, output, err := internal.CompileAndRun(
			context.Background(),
			source,
			internal.CompileOpts{Listener: nil, Output: "", KeepIR: true},
		)
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
	fmt.Println("Hello, Metall!")
}
