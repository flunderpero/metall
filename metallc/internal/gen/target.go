package gen

import (
	"github.com/flunderpero/metall/metallc/internal/base"
)

type Target int

const (
	TargetNative Target = 0
	TargetWasm64 Target = 1
)

func (t Target) String() string {
	switch t {
	case TargetNative:
		return "native"
	case TargetWasm64:
		return "wasm64"
	default:
		panic(base.Errorf("unknown Target: %d", t))
	}
}

func (t Target) IsWasm() bool {
	return t == TargetWasm64
}

func ParseTarget(s string) (Target, error) {
	switch s {
	case "", "native":
		return TargetNative, nil
	case "wasm64":
		return TargetWasm64, nil
	default:
		return 0, base.Errorf("unknown target: %s (supported: native, wasm64)", s)
	}
}
