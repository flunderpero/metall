package compiler

import (
	"os"
	"regexp"
	"testing"

	"github.com/flunderpero/metall/metallc/internal/base"
	mdtest "github.com/flunderpero/metall/metallc/internal/test"
)

func TestE2EMD(t *testing.T) {
	t.Parallel()
	if err := os.MkdirAll(".build", 0o700); err != nil {
		t.Fatal(err)
	}
	mdtest.RunFile(t, mdtest.File("e2e_test.md"), mdtest.RunFunc(runE2ETest))
}

func runE2ETest(t *testing.T, assert base.Assert, tc mdtest.TestCase) map[string]string {
	t.Helper()
	t.Parallel()
	source := base.NewSource("test.met", "test", true, []rune(tc.Input))
	reg := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	outputPath := "./.build/" + reg.ReplaceAllString(tc.Name, "_")
	opts := CompileOpts{ //nolint:exhaustruct
		ProjectRoot:      ".",
		Output:           outputPath,
		KeepIR:           true,
		LLVMPasses:       "verify," + DefaultLLVMPasses,
		AddressSanitizer: true,
		MinimalPrelude:   true,
	}
	exitCode, output, err := CompileAndRun(t.Context(), source, opts)
	assert.NoError(err)

	results := map[string]string{}

	if _, ok := tc.Want["output"]; ok {
		assert.Equal(0, exitCode, "exit code")
		results["output"] = output
	}

	if _, ok := tc.Want["panic"]; ok {
		assert.NotEqual(0, exitCode, "expected non-zero exit code (trap)")
		results["panic"] = output
	}

	return results
}
