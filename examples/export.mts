// Wasm driver for examples/export.met - paired with export.c on the native
// side. Run via `just examples wasm32` / `wasm64`, or by hand with:
//   just metallc build --target wasm32 --emit-typescript -o examples/export.wasm examples/export.met
//   (cd examples && node export.mts)
import * as fs from "node:fs"
import {loadMetall} from "./export.ts"

const api = await loadMetall(fs.readFileSync("./export.wasm"))
console.log(`metall_add(3, 4)   = ${api.metall_add(3n, 4n)}`)
console.log(`metall_fib(10)     = ${api.metall_fib(10n)}`)
console.log(`metall_is_even(7)  = ${api.metall_is_even(7n)}`)
console.log(`metall_is_even(8)  = ${api.metall_is_even(8n)}`)
