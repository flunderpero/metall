package test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/flunderpero/metall/metallc/internal/base"
)

var update = flag.Bool("update", false, "update markdown test expectations") //nolint:gochecknoglobals

// File returns the absolute path to a file relative to the caller's source file.
func File(name string) string {
	_, callerFile, _, ok := runtime.Caller(1)
	if !ok {
		panic("mdtest.File: failed to determine caller")
	}
	return filepath.Join(filepath.Dir(callerFile), name)
}

type TestCase struct {
	Name     string
	Sections []string
	Tags     []string
	Input    string
	Want     map[string]string
	Only     bool

	inputLine int
	wantLines map[string]int
}

func (tc TestCase) FullName() string {
	parts := make([]string, 0, len(tc.Sections)+1)
	parts = append(parts, tc.Sections...)
	parts = append(parts, tc.Name)
	return strings.Join(parts, "/")
}

type Runner interface {
	Run(t *testing.T, assert base.Assert, tc TestCase) map[string]string
}

type RunFunc func(t *testing.T, assert base.Assert, tc TestCase) map[string]string

func (f RunFunc) Run(t *testing.T, assert base.Assert, tc TestCase) map[string]string {
	t.Helper()
	return f(t, assert, tc)
}

// RunFile parses a markdown file and runs all test cases using the given runner.
// Pass -update to update mismatched expectations in the markdown file.
// Runners may call t.Parallel() — results are collected safely and the
// --update rewrite happens after all subtests complete.
func RunFile(t *testing.T, path string, runner Runner) { //nolint:funlen
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read test file %s: %v", path, err)
	}
	cases := Parse(string(content))
	if len(cases) == 0 {
		t.Fatalf("no test cases found in %s", path)
	}
	hasOnly := false
	for i := range cases {
		if cases[i].Only {
			hasOnly = true
			break
		}
	}

	// updateResults collects actual outputs per test case index + lang
	// so we can rewrite the file after all (possibly parallel) subtests finish.
	type updateEntry struct {
		caseIdx int
		lang    string
		actual  string
	}
	var mu sync.Mutex
	var updates []updateEntry

	for i, tc := range cases {
		if hasOnly && !tc.Only {
			continue
		}
		caseIdx := i
		t.Run(tc.FullName(), func(t *testing.T) {
			assert := base.NewAssert(t)
			got := runner.Run(t, assert, tc)
			for lang, want := range tc.Want {
				actual, ok := got[lang]
				if !ok {
					t.Errorf("runner did not return result for %q", lang)
					continue
				}
				want = strings.TrimSpace(want)
				actual = strings.TrimSpace(actual)
				if want != actual {
					if *update {
						mu.Lock()
						updates = append(updates, updateEntry{caseIdx, lang, actual})
						mu.Unlock()
					} else {
						wantStr, actualStr := base.Diff(want, actual)
						line := tc.wantLines[lang]
						t.Errorf("mismatch for %q (line %d):\nwant:\n%s\n\ngot:\n%s", lang, line, wantStr, actualStr)
					}
				}
			}
		})
	}

	// Apply updates after all subtests (including parallel ones) complete.
	t.Cleanup(func() {
		if len(updates) == 0 {
			return
		}
		// Sort by wantLine so we process from top to bottom with correct offsets.
		sortKey := func(e updateEntry) int {
			return cases[e.caseIdx].wantLines[e.lang]
		}
		for i := 1; i < len(updates); i++ {
			for j := i; j > 0 && sortKey(updates[j]) < sortKey(updates[j-1]); j-- {
				updates[j], updates[j-1] = updates[j-1], updates[j]
			}
		}
		lines := strings.Split(string(content), "\n")
		lineOffset := 0
		for _, entry := range updates {
			startLine := cases[entry.caseIdx].wantLines[entry.lang] + lineOffset
			delta := updateExpectation(t, &lines, startLine, entry.lang, entry.actual)
			lineOffset += delta
		}
		updated := strings.Join(lines, "\n")
		if err := os.WriteFile(path, []byte(updated), 0o600); err != nil {
			t.Errorf("failed to update test file %s: %v", path, err)
			return
		}
		t.Log("updated expectations in", path)
	})
}

// updateExpectation replaces the content of a fenced code block.
// startLine is 1-indexed, pointing to the opening fence line (adjusted for prior edits).
// Returns the number of lines added (positive) or removed (negative).
func updateExpectation(t *testing.T, lines *[]string, startLine int, lang, actual string) int {
	t.Helper()
	// startLine is 1-indexed, pointing to the opening fence line.
	// Find the closing fence.
	idx := startLine - 1 // convert to 0-indexed
	closingIdx := -1
	for i := idx + 1; i < len(*lines); i++ {
		if strings.TrimSpace((*lines)[i]) == "```" {
			closingIdx = i
			break
		}
	}
	if closingIdx == -1 {
		t.Fatalf("could not find closing fence for %q block at line %d", lang, startLine)
	}
	oldContentLines := closingIdx - idx - 1
	// Replace everything between the opening and closing fences.
	var newLines []string
	if actual != "" {
		newLines = strings.Split(actual, "\n")
	}
	replacement := make([]string, 0, len(*lines))
	replacement = append(replacement, (*lines)[:idx+1]...)
	replacement = append(replacement, newLines...)
	replacement = append(replacement, (*lines)[closingIdx:]...)
	*lines = replacement
	t.Logf("updated %q expectation at line %d", lang, startLine)
	return len(newLines) - oldContentLines
}

// Parse extracts test cases from markdown content.
func Parse(content string) []TestCase { //nolint:funlen
	lines := strings.Split(content, "\n")
	var cases []TestCase
	var sections []string
	sectionOnly := false
	// Number within the current innermost section.
	sectionNum := 0
	// Pending test name from a **bold** line.
	pendingName := ""

	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Track section headings.
		if strings.HasPrefix(trimmed, "#") {
			level := 0
			for _, ch := range trimmed {
				if ch == '#' {
					level++
				} else {
					break
				}
			}
			title := strings.TrimSpace(trimmed[level:])
			only := false
			if strings.HasPrefix(title, "!only ") {
				only = true
				title = strings.TrimPrefix(title, "!only ")
			}
			// Adjust sections slice to match heading depth.
			// Level 1 = index 0, level 2 = index 1, etc.
			if level-1 < len(sections) {
				sections = sections[:level-1]
			}
			sections = append(sections, title)
			sectionOnly = only
			sectionNum = 0
			pendingName = ""
			i++
			continue
		}

		// Track **bold** test names.
		if strings.HasPrefix(trimmed, "**") && strings.HasSuffix(trimmed, "**") && len(trimmed) > 4 {
			pendingName = trimmed[2 : len(trimmed)-2]
			i++
			continue
		}

		// Look for ```metall fences.
		if strings.HasPrefix(trimmed, "```metall") {
			sectionNum++
			only := false
			var tags []string
			for _, word := range strings.Fields(trimmed)[1:] { // skip "```metall"
				if word == "!only" {
					only = true
				} else {
					tags = append(tags, word)
				}
			}
			inputStartLine := i + 1 // 1-indexed line of the fence

			// Collect the metall source.
			i++
			var inputLines []string
			for i < len(lines) {
				if strings.TrimSpace(lines[i]) == "```" {
					break
				}
				inputLines = append(inputLines, lines[i])
				i++
			}
			i++ // skip closing ```

			input := strings.Join(inputLines, "\n")

			// Build the test name.
			var name string
			if pendingName != "" {
				name = fmt.Sprintf("%d %s", sectionNum, pendingName)
			} else {
				name = fmt.Sprintf("%d", sectionNum)
			}

			// Collect all subsequent expectation blocks.
			want := map[string]string{}
			wantLines := map[string]int{}
			for i < len(lines) {
				nextTrimmed := strings.TrimSpace(lines[i])
				// Stop at the next heading, bold name, or metall block.
				if strings.HasPrefix(nextTrimmed, "#") {
					break
				}
				if strings.HasPrefix(nextTrimmed, "**") &&
					strings.HasSuffix(nextTrimmed, "**") && len(nextTrimmed) > 4 {
					break
				}
				if strings.HasPrefix(nextTrimmed, "```metall") {
					break
				}
				// Parse an expectation code block.
				if strings.HasPrefix(nextTrimmed, "```") && nextTrimmed != "```" {
					lang := strings.TrimPrefix(nextTrimmed, "```")
					lang = strings.TrimSpace(lang)
					wantLines[lang] = i + 1 // 1-indexed
					i++
					var expectLines []string
					for i < len(lines) {
						if strings.TrimSpace(lines[i]) == "```" {
							break
						}
						expectLines = append(expectLines, lines[i])
						i++
					}
					i++ // skip closing ```
					want[lang] = strings.Join(expectLines, "\n")
					continue
				}
				// Skip non-code-block lines (prose, blank lines).
				i++
			}

			tc := TestCase{
				Name:      name,
				Sections:  make([]string, len(sections)),
				Tags:      tags,
				Input:     input,
				Want:      want,
				Only:      only || sectionOnly,
				inputLine: inputStartLine,
				wantLines: wantLines,
			}
			copy(tc.Sections, sections)
			cases = append(cases, tc)
			pendingName = ""
			continue
		}

		// Skip other lines.
		i++
	}
	return cases
}
