package compiler

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
	mdtest "github.com/flunderpero/metall/metallc/internal/test"
	"github.com/flunderpero/metall/metallc/internal/token"
	"github.com/flunderpero/metall/metallc/internal/types"
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
	timing := newTimingListener()
	opts := CompileOpts{ //nolint:exhaustruct
		ProjectRoot:      ".",
		Listener:         timing,
		Output:           outputPath,
		KeepIR:           true,
		LLVMPasses:       "verify," + DefaultLLVMPasses,
		AddressSanitizer: true,
		MinimalPrelude:   true,
	}
	exitCode, output, err := CompileAndRun(t.Context(), source, opts)
	timing.Log(t)
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

// timingListener records the wall-clock duration of each compiler phase.
type timingListener struct {
	last  time.Time
	steps []step
}

type step struct {
	name     string
	duration time.Duration
}

func newTimingListener() *timingListener {
	return &timingListener{last: time.Now(), steps: nil}
}

func (l *timingListener) OnLex(_ []token.Token) bool {
	l.record("lex")
	return true
}

func (l *timingListener) OnParse(_ *ast.AST, _ ast.NodeID, _ base.Diagnostics) bool {
	l.record("parse")
	return true
}

func (l *timingListener) OnTypeCheck(_ *types.Engine, _ base.Diagnostics) bool {
	l.record("typecheck")
	return true
}

func (l *timingListener) OnLifetimeCheck(_ *types.LifetimeCheck, _ base.Diagnostics) bool {
	l.record("lifetime")
	return true
}

func (l *timingListener) OnIRGen(_ string) bool {
	l.record("irgen")
	return true
}

func (l *timingListener) OnOptimizeIR() bool {
	l.record("optimize")
	return true
}

func (l *timingListener) OnLink() bool {
	l.record("link")
	return true
}

func (l *timingListener) OnRun(_ int, _ string) bool {
	l.record("run")
	return true
}

// Log prints every step that took longer than 10ms.
func (l *timingListener) Log(t *testing.T) {
	t.Helper()
	for _, s := range l.steps {
		if s.duration >= 10*time.Millisecond {
			t.Logf("%-12s %s", s.name, s.duration.Round(time.Millisecond))
		}
	}
}

// Total returns a display string of all step durations.
func (l *timingListener) Total() string {
	var total time.Duration
	parts := make([]string, 0, len(l.steps))
	for _, s := range l.steps {
		total += s.duration
		parts = append(parts, fmt.Sprintf("%s=%s", s.name, s.duration.Round(time.Millisecond)))
	}
	return fmt.Sprintf("total=%s (%s)", total.Round(time.Millisecond), strings.Join(parts, ", "))
}

func (l *timingListener) record(name string) {
	now := time.Now()
	l.steps = append(l.steps, step{name, now.Sub(l.last)})
	l.last = now
}
