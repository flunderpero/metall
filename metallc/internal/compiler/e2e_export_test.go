//nolint:exhaustruct
package compiler

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/flunderpero/metall/metallc/internal/base"
	"github.com/flunderpero/metall/metallc/internal/gen"
	mdtest "github.com/flunderpero/metall/metallc/internal/test"
)

func TestE2EExportMD(t *testing.T) {
	t.Parallel()
	if err := os.MkdirAll(".build", 0o700); err != nil {
		t.Fatal(err)
	}
	path := mdtest.File("e2e_export.md")
	runner := &e2eExportRunner{}
	mdtest.RunFile(t, path, runner)
}

type e2eExportRunner struct{}

func (r *e2eExportRunner) Run(t *testing.T, assert base.Assert, tc mdtest.TestCase) map[string]string {
	t.Helper()
	t.Parallel()
	results := map[string]string{}

	cSource, hasC := tc.Want["c"]
	if hasC {
		results["c"] = cSource
	}
	if tc.Input == "" {
		return results
	}
	target := exportTarget(t)
	reg := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	baseName := "export_" + reg.ReplaceAllString(tc.FullName(), "_")
	outDir := t.TempDir()
	objPath := filepath.Join(outDir, baseName+".o")
	source := base.NewSource(baseName+".met", baseName, true, []rune(tc.Input))
	opts := CompileOpts{
		ProjectRoot:    outDir,
		IncludePaths:   []string{"../../../lib"},
		Output:         objPath,
		OptLevel:       OptLevelNone,
		LLVMPasses:     "verify," + DefaultLLVMPasses,
		EmitObject:     true,
		EmitHeaderFile: true,
		Target:         target,
	}
	if _, wantErr := tc.Want["error"]; wantErr {
		err := Compile(t.Context(), source, opts)
		if err != nil {
			results["error"] = err.Error()
		}
		return results
	}
	err := Compile(t.Context(), source, opts)
	assert.NoError(err, "metall compile failed")

	if !hasC {
		t.Fatalf("export test %q has no ```c block", tc.FullName())
	}
	headerPath := filepath.Join(outDir, baseName+".h")
	cPath := filepath.Join(outDir, baseName+".c")
	assert.NoError(os.WriteFile(cPath, []byte(cSource), 0o600))

	binPath := filepath.Join(outDir, baseName)
	runExportLink(t, assert, headerPath, cPath, objPath, binPath, outDir)
	runOut, exitCode := runExportBinary(t, binPath)

	if _, ok := tc.Want["header"]; ok {
		headerBytes, err := os.ReadFile(headerPath)
		assert.NoError(err, "read generated header")
		results["header"] = string(headerBytes)
	}
	if _, ok := tc.Want["output"]; ok {
		assert.Equal(0, exitCode, "exit code")
		results["output"] = string(runOut)
	}
	if _, ok := tc.Want["panic"]; ok {
		assert.NotEqual(0, exitCode, "expected non-zero exit code")
		results["panic"] = string(runOut)
	}
	return results
}

// exportTarget resolves the compile target from METALL_E2E_TEST_TARGET and
// skips the test under wasm since object-file exports are native-only.
func exportTarget(t *testing.T) gen.Target {
	t.Helper()
	target := gen.TargetNative
	s := os.Getenv("METALL_E2E_TEST_TARGET")
	if s == "" {
		return target
	}
	parsed, err := gen.ParseTarget(s)
	if err != nil {
		t.Fatalf("unknown target: %s", s)
	}
	if parsed.IsWasm() {
		t.Skip("skipped: C export tests are native-only")
	}
	return parsed
}

func runExportLink(
	t *testing.T, assert base.Assert, headerPath, cPath, objPath, binPath, outDir string,
) {
	t.Helper()
	llvmHome, err := findLLVMHome()
	assert.NoError(err, "find LLVM home")
	clang := filepath.Join(llvmHome, "bin", "clang")
	cmdline := []string{clang, "-I", outDir, "-include", headerPath, "-o", binPath, cPath, objPath}
	if runtime.GOOS == "darwin" {
		sdk, err := exec.CommandContext(t.Context(), "xcrun", "--show-sdk-path").Output()
		assert.NoError(err, "xcrun --show-sdk-path")
		cmdline = append(cmdline, "-isysroot", strings.TrimSpace(string(sdk)))
	}
	cmd := exec.CommandContext(t.Context(), cmdline[0], cmdline[1:]...) //nolint:gosec
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("clang link failed: %v\n%s", err, string(out))
	}
}

func runExportBinary(t *testing.T, binPath string) ([]byte, int) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), binPath)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return out, 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return out, exitErr.ExitCode()
	}
	t.Fatalf("binary run failed: %v\n%s", err, string(out))
	return nil, 0
}
