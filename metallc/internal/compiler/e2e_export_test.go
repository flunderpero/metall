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
	tsSource, hasTS := tc.Want["ts"]
	if hasTS {
		results["ts"] = tsSource
	}
	if tc.Input == "" {
		return results
	}
	target := exportTarget(t)

	reg := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	baseName := "export_" + reg.ReplaceAllString(tc.FullName(), "_")
	outDir := t.TempDir()

	if target.IsWasm() {
		runWasmExport(t, assert, tc, tsSource, outDir, baseName, target, results)
	} else {
		runNativeExport(t, assert, tc, cSource, hasC, outDir, baseName, results)
	}
	return results
}

func runNativeExport(
	t *testing.T, assert base.Assert, tc mdtest.TestCase, cSource string, hasC bool,
	outDir, baseName string, results map[string]string,
) {
	t.Helper()
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
		Target:         gen.TargetNative,
	}
	if _, wantErr := tc.Want["error"]; wantErr {
		err := Compile(t.Context(), source, opts)
		if err != nil {
			results["error"] = err.Error()
		}
		return
	}
	assert.NoError(Compile(t.Context(), source, opts), "metall compile failed")
	if !hasC {
		t.Skipf("skipped: no ```c``` section found")
	}
	headerPath := filepath.Join(outDir, baseName+".h")
	cPath := filepath.Join(outDir, baseName+".c")
	assert.NoError(os.WriteFile(cPath, []byte(cSource), 0o600))
	binPath := filepath.Join(outDir, baseName)
	runExportLink(t, assert, headerPath, cPath, objPath, binPath, outDir)
	out, exitCode := runExportBinary(t, binPath)
	if _, ok := tc.Want["header"]; ok {
		headerBytes, err := os.ReadFile(headerPath)
		assert.NoError(err, "read generated header")
		results["header"] = string(headerBytes)
	}
	recordRunResult(tc, results, exitCode, out)
}

func runWasmExport(
	t *testing.T, assert base.Assert, tc mdtest.TestCase, tsSource string,
	outDir, baseName string, target gen.Target, results map[string]string,
) {
	t.Helper()
	wasmPath := filepath.Join(outDir, "metall.wasm")
	source := base.NewSource(baseName+".met", baseName, true, []rune(tc.Input))
	opts := CompileOpts{
		ProjectRoot:    outDir,
		IncludePaths:   []string{"../../../lib"},
		Output:         wasmPath,
		OptLevel:       OptLevelNone,
		LLVMPasses:     "verify," + DefaultLLVMPasses,
		EmitTypeScript: true,
		Target:         target,
	}
	if _, wantErr := tc.Want["error"]; wantErr {
		t.Skip("skipped: error cases only run on native")
	}
	assert.NoError(Compile(t.Context(), source, opts), "metall wasm compile failed")
	if tsSource == "" {
		t.Skipf("skipped: no ```ts``` section found")
	}
	runnerPath := filepath.Join(outDir, "runner.mts")
	assert.NoError(os.WriteFile(runnerPath, []byte(tsSource), 0o600))
	out, exitCode := runNodeJS(t, runnerPath, outDir)
	recordRunResult(tc, results, exitCode, out)
}

func recordRunResult(tc mdtest.TestCase, results map[string]string, exitCode int, out []byte) {
	if _, ok := tc.Want["output"]; ok && exitCode == 0 {
		results["output"] = string(out)
	}
	if _, ok := tc.Want["panic"]; ok && exitCode != 0 {
		results["panic"] = string(out)
	}
}

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

func runNodeJS(t *testing.T, runnerPath, outDir string) ([]byte, int) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "node", runnerPath)
	cmd.Dir = outDir
	out, err := cmd.CombinedOutput()
	if err == nil {
		return out, 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return out, exitErr.ExitCode()
	}
	t.Fatalf("node run failed: %v\n%s", err, string(out))
	return nil, 0
}
