package gen

import (
	"github.com/flunderpero/metall/metallc/internal/base"
)

type Target int

const (
	TargetNative Target = 0
	TargetWasm64 Target = 1
	TargetWasm32 Target = 2
)

func (t Target) String() string {
	switch t {
	case TargetNative:
		return "native"
	case TargetWasm64:
		return "wasm64"
	case TargetWasm32:
		return "wasm32"
	default:
		panic(base.Errorf("unknown Target: %d", t))
	}
}

func (t Target) IsWasm() bool {
	return t == TargetWasm64 || t == TargetWasm32
}

func (t Target) PointerSize() int64 {
	if t == TargetWasm32 {
		return 4
	}
	return 8
}

func ParseTarget(s string) (Target, error) {
	switch s {
	case "", "native":
		return TargetNative, nil
	case "wasm64":
		return TargetWasm64, nil
	case "wasm32":
		return TargetWasm32, nil
	default:
		return 0, base.Errorf("unknown target: %s (supported: native, wasm32, wasm64)", s)
	}
}
