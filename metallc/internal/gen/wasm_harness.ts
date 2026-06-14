/**
 * Browser / Node harness for metallc's `--target wasm32|wasm64` output.
 *
 *     import { runMetall } from "./wasm_harness.ts"
 *     await runMetall("./hello.wasm")
 */

/** `write(fd, text)` is called for every chunk written to stdout (fd=1) or stderr (fd=2). */
export type WriteFn = ((fd: number, text: string) => void) & {
    /** Optional: flush any buffered bytes (called once on normal exit). */
    flush?: () => void
}

/** Shape of `env.*` imports expected by Metall-compiled modules. */
export type MetallImports = Record<string, (...args: any[]) => any>

export type RunOptions = {
    /** Override the default writer (`console.log`/`console.error` buffering lines). */
    write?: WriteFn
    /** Extra `env` imports (merged over the built-in ones). */
    imports?: MetallImports
}

export type Source = string | ArrayBuffer | ArrayBufferView

export type MetallInstance = {
    instance: WebAssembly.Instance
    write: WriteFn
}

/**
 * Compile and instantiate a Metall WASM module. The returned instance has the
 * built-in `env` imports wired up (plus any caller overrides) but `main` has
 * not been invoked. Use {@link runMetall} to also run `main`.
 */
export async function instantiateMetall(
    src: Source, options: RunOptions = {},
): Promise<MetallInstance> {
    let instance: WebAssembly.Instance
    const write = options.write ?? defaultWriteImpl()
    const {imports} = defaultImports(() => instance.exports.memory as WebAssembly.Memory, write)
    if (options.imports) {
        imports.env = {...imports.env, ...options.imports}
    }
    const module = src instanceof ArrayBuffer || ArrayBuffer.isView(src)
        ? await WebAssembly.compile(src)
        : await compileFromURL(String(src))
    const missing: string[] = []
    for (const imp of WebAssembly.Module.imports(module)) {
        if (imp.kind !== "function") {
            continue
        }
        const target = imports[imp.module]
        if (!target || !(imp.name in target)) {
            missing.push(`${imp.module}.${imp.name}`)
        }
    }
    if (missing.length > 0) {
        throw new Error(
            "Metall: unresolved extern imports: " + missing.join(", ") +
            "\nProvide implementations via the `imports` option.",
        )
    }
    instance = await WebAssembly.instantiate(module, imports)
    return {instance, write}
}

/**
 * Run a Metall WASM file. Returns the program's exit code.
 *
 * `src` may be a URL string, an `ArrayBuffer`, or an `ArrayBufferView`.
 */
export async function runMetall(src: Source, options: RunOptions = {}): Promise<number> {
    const {instance, write} = await instantiateMetall(src, options)
    const main = instance.exports.main
    if (typeof main !== "function") {
        return 0
    }
    try {
        // Clang wraps user main with an (argc, argv) shim. argv is a pointer,
        // so it's i32 on wasm32 (JS Number) and i64 on wasm64 (JS BigInt).
        // Try BigInt first; a TypeError means we're on wasm32, retry with Number.
        let code: unknown
        try {
            code = main(0, 0n)
        } catch (e) {
            if (!(e instanceof TypeError)) {
                throw e
            }
            code = main(0, 0)
        }
        return typeof code === "number" ? code : 0
    } catch (e) {
        // panic() -> llvm.trap() -> wasm unreachable -> RuntimeError.
        // The message is already on stderr; report non-zero and swallow.
        if (e instanceof WebAssembly.RuntimeError) {
            return 1
        }
        throw e
    } finally {
        write.flush?.()
    }
}

/**
 * Default writer used when the caller doesn't provide one. `console.log` /
 * `console.error` always append a newline, so we buffer per-fd and only emit
 * complete lines.
 */
function defaultWriteImpl(): WriteFn {
    const buffers: Record<number, string> = {1: "", 2: ""}
    const write: WriteFn = (fd, text) => {
        const log = fd === 2 ? console.error : console.log
        const combined = (buffers[fd] ?? "") + text
        const lines = combined.split("\n")
        buffers[fd] = lines.pop() ?? ""
        for (const line of lines) {
            log(line)
        }
    }
    write.flush = () => {
        for (const fd of [1, 2]) {
            if (buffers[fd]) {
                (fd === 2 ? console.error : console.log)(buffers[fd])
                buffers[fd] = ""
            }
        }
    }
    return write
}

/** Try streaming compilation and fall back to `fetch()` + `WebAssembly.compile()`. */
async function compileFromURL(url: string): Promise<WebAssembly.Module> {
    try {
        return await WebAssembly.compileStreaming(fetch(url))
    } catch {
        const res = await fetch(url)
        if (!res.ok) {
            throw new Error(`failed to fetch ${url}: ${res.status}`)
        }
        return await WebAssembly.compile(await res.arrayBuffer())
    }
}

function defaultImports(
    getMemory: () => WebAssembly.Memory,
    write: WriteFn,
): {imports: {env: MetallImports}} {
    const decoder = new TextDecoder()
    const view = () => new DataView(getMemory().buffer)
    const env: MetallImports = {
        write: (fd: number, ptr: number | bigint, len: number | bigint) => {
            const mem = getMemory()
            const bytes = new Uint8Array(mem.buffer, Number(ptr), Number(len))
            write(Number(fd), decoder.decode(bytes))
            return len
        },
        // Write seconds + nanoseconds into `struct timespec { i64 sec; i64 nsec }` at tsPtr.
        // POSIX clock ids:
        //   0 = CLOCK_REALTIME (wall-clock, relative to unix epoch)
        //   4 = CLOCK_MONOTONIC_RAW (monotonic, arbitrary origin)
        clock_gettime: (clockId: number, tsPtr: number | bigint) => {
            if (clockId !== 0 && clockId !== 4) {
                return 22 // EINVAL
            }
            const ms = clockId === 0
                ? performance.timeOrigin + performance.now()
                : performance.now()
            const ns = BigInt(Math.round(ms * 1e6))
            const v = view()
            v.setBigInt64(Number(tsPtr), ns / 1000000000n, true)
            v.setBigInt64(Number(tsPtr) + 8, ns % 1000000000n, true)
            return 0
        },
        // Block for the duration given by `struct timespec` at reqPtr.
        // Uses Atomics.wait on a throw-away SharedArrayBuffer (works in Node
        // and in Web Workers); falls back to a busy-wait on main-thread
        // browser contexts where Atomics.wait is forbidden.
        nanosleep: (reqPtr: number | bigint, _remPtr: number | bigint) => {
            const v = view()
            const sec = v.getBigInt64(Number(reqPtr), true)
            const nsec = v.getBigInt64(Number(reqPtr) + 8, true)
            const ms = Number(sec) * 1000 + Number(nsec) / 1e6
            blockingSleep(ms)
            return 0
        },
        // Thin snprintf/strtod equivalents; the prelude does the looping,
        // classification, and rendering. The slice argument arrives as a pointer
        // to a { ptr, i64 len } struct (byval); a bigint means a 64-bit pointer
        // (wasm64), a number means 32-bit (wasm32).
        __snprintf_float: (v: number, prec: number | bigint, mode: number, slicePtr: number | bigint): bigint => {
            const {ptr, len} = readSlice(view(), slicePtr)
            const enc = new TextEncoder().encode(snprintfFloat(v, Number(prec), mode))
            const m = Math.min(enc.length, len)
            new Uint8Array(getMemory().buffer, ptr, m).set(enc.subarray(0, m))
            return BigInt(enc.length)
        },
        __strtod: (slicePtr: number | bigint, consumedPtr: number | bigint): number => {
            const {ptr, len} = readSlice(view(), slicePtr)
            const s = decoder.decode(new Uint8Array(getMemory().buffer, ptr, len))
            const [v, consumed] = strtodLike(s)
            view().setBigInt64(Number(consumedPtr), BigInt(consumed), true)
            return v
        },
        // The wasm build is -nostdlib, so libc externs like `abs` become `env`
        // imports instead of resolving to libc. The extern-C e2e tests call it.
        abs: (n: number) => Math.abs(n),
    }
    return {imports: {env}}
}

/** Read a Metall slice ({ ptr, i64 len }) from the struct the byval call left in
 * linear memory. len is always at offset 8 (the i64 is 8-aligned on both wasm32
 * and wasm64); the pointer is 4 bytes on wasm32 (number arg) or 8 on wasm64. */
function readSlice(v: DataView, slicePtr: number | bigint): {ptr: number; len: number} {
    const sp = Number(slicePtr)
    const ptr = typeof slicePtr === "bigint" ? Number(v.getBigUint64(sp, true)) : v.getUint32(sp, true)
    return {ptr, len: Number(v.getBigUint64(sp + 8, true))}
}

/** snprintf("%.{prec}{mode}", v) with mode 101='e', 102='f', 103='g', matching
 * builtins_posix.ll. NaN/inf use the C spellings; the prelude special-cases the
 * stdlib spellings before this is reached. */
function snprintfFloat(v: number, prec: number, mode: number): string {
    if (Number.isNaN(v)) {
        return "nan"
    }
    if (!Number.isFinite(v)) {
        return v < 0 ? "-inf" : "inf"
    }
    const sign = v < 0 || Object.is(v, -0) ? "-" : ""
    const a = Math.abs(v)
    if (mode === 102) {
        return sign + a.toFixed(prec)
    }
    if (mode === 101) {
        return sign + toExpFixed(a, prec)
    }
    return sign + toG(a, prec)

    /** "%.{prec}e" of a non-negative value: at least two exponent digits, like C. */
    function toExpFixed(a: number, prec: number): string {
        const s = a.toExponential(prec)
        const e = s.indexOf("e")
        const exp = parseInt(s.slice(e + 1), 10)
        let ed = Math.abs(exp).toString()
        if (ed.length < 2) {
            ed = "0" + ed
        }
        return s.slice(0, e) + "e" + (exp < 0 ? "-" : "+") + ed
    }

    /** "%.{p}g" of a non-negative value: p significant digits, trailing zeros
     * removed, scientific when the exponent is < -4 or >= p. */
    function toG(a: number, p: number): string {
        if (a === 0) {
            return "0"
        }
        const es = a.toExponential(p - 1)
        const e = es.indexOf("e")
        const exp = parseInt(es.slice(e + 1), 10)
        let mant = es.slice(0, e).replace(".", "").replace(/0+$/, "")
        if (mant === "") {
            mant = "0"
        }
        if (exp < -4 || exp >= p) {
            const head = mant.length > 1 ? mant[0] + "." + mant.slice(1) : mant
            let ed = Math.abs(exp).toString()
            if (ed.length < 2) {
                ed = "0" + ed
            }
            return head + "e" + (exp < 0 ? "-" : "+") + ed
        }
        if (exp < 0) {
            return "0." + "0".repeat(-exp - 1) + mant
        }
        if (exp + 1 >= mant.length) {
            return mant + "0".repeat(exp + 1 - mant.length)
        }
        return mant.slice(0, exp + 1) + "." + mant.slice(exp + 1)
    }
}

/** Parse the longest float prefix (strtod-like), returning [value, bytes
 * consumed] so the prelude can detect trailing junk. */
function strtodLike(s: string): [number, number] {
    const m = s.match(/^[+-]?(infinity|inf|nan|(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?)/i)
    if (!m) {
        return [0, 0]
    }
    const tok = m[0]
    const body = tok.replace(/^[+-]/, "").toLowerCase()
    let v: number
    if (body === "inf" || body === "infinity") {
        v = tok[0] === "-" ? -Infinity : Infinity
    } else if (body === "nan") {
        v = NaN
    } else {
        v = Number(tok)
    }
    return [v, tok.length]
}

function blockingSleep(ms: number): void {
    if (ms <= 0) {
        return
    }
    // todo: Implement JSPI to make this also work on the main thread.
    try {
        Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, ms)
    } catch {
        const start = performance.now()
        while (performance.now() - start < ms) {
            // spin
        }
    }
}
