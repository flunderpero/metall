package gen

import (
	_ "embed"
)

//go:embed wasm_harness.ts
var wasmHarnessTS string

// WasmHarnessTS returns the embedded TypeScript harness. Node ≥ 23 strips
// type annotations automatically when importing the file; the same source
// doubles as the runtime harness (for `metallc run --target wasm*`) and the
// file emitted by `--emit-typescript`.
func WasmHarnessTS() string {
	return wasmHarnessTS
}
