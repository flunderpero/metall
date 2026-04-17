package gen

import (
	_ "embed"
)

//go:embed wasm_harness.js
var wasmHarnessJS string

func WasmHarnessJS() string {
	return wasmHarnessJS
}
