//nolint:exhaustruct
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

func TestE2EMD(t *testing.T) {
	t.Parallel()
	runE2ESuite(t, "e2e_test.md", e2eConfig{prefix: "", minimalPrelude: true, useEnvOptLevel: true})
}

func TestE2EUnoptimizedMD(t *testing.T) {
	t.Parallel()
	runE2ESuite(t, "e2e_unoptimized_test.md", e2eConfig{prefix: "unopt_", minimalPrelude: true, llvmPasses: "verify"})
}

func TestE2EStdMD(t *testing.T) {
	t.Parallel()
	runE2ESuite(t, "e2e_std_test.md", e2eConfig{minimalPrelude: false, useEnvOptLevel: true})
}

type e2eConfig struct {
	prefix         string
	minimalPrelude bool
	llvmPasses     string
	useEnvOptLevel bool
}

func runE2ESuite(t *testing.T, filename string, cfg e2eConfig) {
	t.Helper()
	if err := os.MkdirAll(".build", 0o700); err != nil {
		t.Fatal(err)
	}
	path := mdtest.File(filename)

	// Parse modules from test file.
	var modules map[string]string
	if !cfg.minimalPrelude {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		modules = map[string]string{}
		for _, tc := range mdtest.Parse(string(content)) {
			for lang, body := range tc.Want {
				if tag, ok := strings.CutPrefix(lang, "module."); ok {
					modules[tag] = body
				}
			}
		}
	}

	runner := &e2eRunner{cfg: cfg, modules: modules}
	mdtest.RunFile(t, path, runner)
}

type e2eRunner struct {
	cfg     e2eConfig
	modules map[string]string
}

func (r *e2eRunner) Run(t *testing.T, assert base.Assert, tc mdtest.TestCase) map[string]string {
	t.Helper()
	t.Parallel()
	results := map[string]string{}

	// Pass through module blocks.
	for lang, content := range tc.Want {
		if _, ok := strings.CutPrefix(lang, "module."); ok {
			results[lang] = content
		}
	}
	if tc.Input == "" {
		return results
	}

	// Determine project directory.
	projectDir := "."
	if len(r.modules) > 0 {
		projectDir = t.TempDir()
		for name, content := range r.modules {
			modPath := filepath.Join(projectDir, name+".met")
			assert.NoError(os.WriteFile(modPath, []byte(content), 0o600))
		}
	}

	source := base.NewSource("test.met", "test", true, []rune(tc.Input))
	reg := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	outputPath := "./.build/" + r.cfg.prefix + reg.ReplaceAllString(tc.Name, "_")

	// Determine optimization level and LLVM passes.
	optLevel := OptLevelNone
	if r.cfg.useEnvOptLevel {
		if s := os.Getenv("METALL_E2E_TEST_OPTLEVEL"); s != "" {
			var err error
			optLevel, err = ParseOptLevel(s)
			assert.NoError(err, "METALL_E2E_TEST_OPTLEVEL is invalid")
		}
		t.Logf("optlevel: %s\n", optLevel)
	}
	llvmPasses := r.cfg.llvmPasses
	if llvmPasses == "" {
		llvmPasses = "verify," + DefaultLLVMPasses
	}

	opts := CompileOpts{ //nolint:exhaustruct
		ProjectRoot:      projectDir,
		IncludePaths:     []string{"../../../lib"},
		Output:           outputPath,
		OptLevel:         optLevel,
		KeepIR:           true,
		LLVMPasses:       llvmPasses,
		AddressSanitizer: true,
		MinimalPrelude:   r.cfg.minimalPrelude,
	}
	exitCode, output, err := CompileAndRun(t.Context(), source, opts, true)
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
