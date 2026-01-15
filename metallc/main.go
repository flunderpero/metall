package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: metallc <file.met>")
		os.Exit(1)
	}
	fmt.Println("Hello, Metall!")
}
