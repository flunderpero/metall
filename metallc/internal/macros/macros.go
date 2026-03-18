package macros

import (
	"fmt"
	"strings"

	"github.com/flunderpero/metall/metallc/internal/ast"
	"github.com/flunderpero/metall/metallc/internal/base"
)

type CompileAndRunFn func(source *base.Source, includePaths []string) (output string, err error)

func IsMacroModule(moduleName string) bool {
	parts := strings.Split(moduleName, "::")
	return strings.HasSuffix(parts[len(parts)-1], "_macro")
}

type MacroArg struct {
	Preamble string
	Expr     string
}

func GenerateWrapper(macroSource string, args []MacroArg) string {
	var sb strings.Builder
	sb.WriteString(macroSource)
	sb.WriteString("\nfun main() void {\n")
	sb.WriteString("    let @a = Arena()\n")
	sb.WriteString("    let sb = StrBuilder.new(1024, @a)\n")
	for _, arg := range args {
		sb.WriteString(arg.Preamble)
	}
	sb.WriteString("    apply(")
	for i, arg := range args {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(arg.Expr)
	}
	if len(args) > 0 {
		sb.WriteString(", ")
	}
	sb.WriteString("sb, @a)\n")
	sb.WriteString("    DebugIntern.print_str(sb.to_str())\n")
	sb.WriteString("}\n")
	return sb.String()
}

func RenderArg(kind ast.Kind, span base.Span) (string, *base.Diagnostic) {
	switch v := kind.(type) {
	case ast.Int:
		return v.Value.String(), nil
	case ast.String:
		return fmt.Sprintf("%q", v.Value), nil
	case ast.Bool:
		if v.Value {
			return "true", nil
		}
		return "false", nil
	default:
		return "", base.NewDiagnostic(span, "macro arguments must be compile-time constants")
	}
}
