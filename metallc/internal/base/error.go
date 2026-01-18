package base

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
)

type WrappedError struct {
	Msg      string
	Err      error
	Location string
}

func (w *WrappedError) Error() string {
	return w.internalError("Error", "")
}

func (w *WrappedError) Unwrap() error {
	return w.Err
}

func (w *WrappedError) Is(target error) bool {
	return errors.Is(w.Err, target)
}

func (w *WrappedError) internalError(prefix string, indent string) string {
	var sb strings.Builder
	// sb.WriteString(indent)
	sb.WriteString(prefix)
	sb.WriteString(" at ")
	sb.WriteString(w.Location)
	sb.WriteString(": ")
	sb.WriteString(w.Msg)
	if wrapped, ok := w.Err.(*WrappedError); ok { //nolint:errorlint
		indent += "  "
		sb.WriteString(wrapped.internalError("\n"+indent+"Cause", indent))
	} else if w.Err != nil {
		sb.WriteString("\n" + indent + "Cause: ")
		sb.WriteString(w.Err.Error())
	}
	return sb.String()
}

func WrapErrorf(err error, msg string, msgArgs ...any) *WrappedError {
	return internalWrapErrorf(err, msg, msgArgs...)
}

func internalWrapErrorf(err error, msg string, msgArgs ...any) *WrappedError {
	location := Location(3)
	return &WrappedError{
		Msg:      fmt.Sprintf(msg, msgArgs...),
		Err:      err,
		Location: location,
	}
}

func Errorf(msg string, msgArgs ...any) *WrappedError {
	return internalWrapErrorf(nil, msg, msgArgs...)
}

func Location(skip int) string {
	pc := make([]uintptr, skip+1)
	runtime.Callers(skip+1, pc)
	frames := runtime.CallersFrames(pc)
	frame, ok := frames.Next()
	if ok {
		file := strings.ReplaceAll(frame.File, "github.com/flunderpero/metall/metallc/internal/", "")
		return fmt.Sprintf("%s:%d", file, frame.Line)
	}
	return ""
}
