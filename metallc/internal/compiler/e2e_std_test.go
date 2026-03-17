package compiler

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/flunderpero/metall/metallc/internal/base"
	mdtest "github.com/flunderpero/metall/metallc/internal/test"
)

func TestE2EStdMD(t *testing.T) {
	t.Parallel()
	if err := os.MkdirAll(".build", 0o700); err != nil {
		t.Fatal(err)
	}
	path := mdtest.File("e2e_std_test.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	modules := map[string]string{}
	for _, tc := range mdtest.Parse(string(content)) {
		for lang, body := range tc.Want {
			if tag, ok := strings.CutPrefix(lang, "module."); ok {
				modules[tag] = body
			}
		}
	}
	runner := &e2eStdRunner{modules: modules}
	mdtest.RunFile(t, path, runner)
}

type e2eStdRunner struct {
	modules map[string]string
}

func (r *e2eStdRunner) Run(t *testing.T, assert base.Assert, tc mdtest.TestCase) map[string]string {
	t.Helper()
	t.Parallel()
	results := map[string]string{}

	for lang, content := range tc.Want {
		if _, ok := strings.CutPrefix(lang, "module."); ok {
			results[lang] = content
		}
	}
	if tc.Input == "" {
		return results
	}

	projectDir := t.TempDir()
	for name, content := range r.modules {
		modPath := filepath.Join(projectDir, name+".met")
		assert.NoError(os.WriteFile(modPath, []byte(content), 0o600))
	}

	source := base.NewSource("test.met", "test", true, []rune(tc.Input))
	reg := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	outputPath := "./.build/" + reg.ReplaceAllString(tc.Name, "_")
	opts := CompileOpts{ //nolint:exhaustruct
		ProjectRoot:      projectDir,
		IncludePaths:     []string{"../../../lib"},
		Output:           outputPath,
		KeepIR:           true,
		LLVMPasses:       "verify," + DefaultLLVMPasses,
		AddressSanitizer: true,
		MinimalPrelude:   false,
	}
	exitCode, output, err := CompileAndRun(t.Context(), source, opts)
	assert.NoError(err)

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
