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

type Source = string | ArrayBuffer | ArrayBufferView

/**
 * Run a Metall WASM file. Returns the program's exit code.
 *
 * `src` may be a URL string, an `ArrayBuffer`, or an `ArrayBufferView`.
 */
export async function runMetall(src: Source, options: RunOptions = {}): Promise<number> {
    let instance: WebAssembly.Instance
    const write = options.write ?? defaultWriteImpl()
    const { imports } = defaultImports(() => instance.exports.memory as WebAssembly.Memory, write)
    if (options.imports) {
        imports.env = { ...imports.env, ...options.imports }
    }
    const module = src instanceof ArrayBuffer || ArrayBuffer.isView(src)
        ? await WebAssembly.compile(src)
        : await compileFromURL(String(src))
    // Find missing extern declarations.
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
            "\nProvide implementations via runMetall(uri, { imports: { ... } }).",
        )
    }
    instance = await WebAssembly.instantiate(module, imports)
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
    }
    return {imports: {env}}
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
