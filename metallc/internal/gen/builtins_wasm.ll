; wasm64 runtime. Print primitives are built on @write (resolved to an
; env import by the JS harness); int formatting runs inline so the
; harness stays printf-free. No @__os_args_init: leaving @__os_args
; zero-init makes os.args() return an empty slice, which is what wasm
; programs see (no host argv).

declare i64 @write(i32, ptr, i64)

; Stub so the shared @panic in builtins.ll can call @fflush unconditionally.
define i32 @fflush(ptr %_unused) {
    ret i32 0
}

@__wasm_newline = private constant [1 x i8] c"\0A"
@__wasm_minus   = private constant [1 x i8] c"-"

; Bump allocator state for runtime/wasmalloc.met: `@__heap_base` is
; supplied by wasm-ld after the static data segment; the Metall-side
; allocator reads/writes `@__wasmalloc_bump` through the get/set shim.
@__heap_base = external hidden global i8
@__wasmalloc_bump = internal global ptr null, align 8

define ptr @__wasmalloc_bump_get() {
    %r = load ptr, ptr @__wasmalloc_bump, align 8
    ret ptr %r
}

define void @__wasmalloc_bump_set(ptr %p) {
    store ptr %p, ptr @__wasmalloc_bump, align 8
    ret void
}

define internal void @__print_str(ptr %data, i64 %len) alwaysinline {
    call i64 @write(i32 1, ptr %data, i64 %len)
    ret void
}

define internal void @__print_str_nl(ptr %data, i64 %len) alwaysinline {
    call i64 @write(i32 1, ptr %data, i64 %len)
    call i64 @write(i32 1, ptr @__wasm_newline, i64 1)
    ret void
}

; Writes %n in decimal (no newline), digits built back-to-front in a
; 24-byte stack buffer (big enough for u64.max).
define internal void @__print_uint(i64 %n) {
entry:
    %buf = alloca [24 x i8], align 1
    %end = getelementptr inbounds [24 x i8], ptr %buf, i64 0, i64 24
    br label %loop
loop:
    %p    = phi ptr [ %end, %entry ], [ %p_next, %loop ]
    %v    = phi i64 [ %n,   %entry ], [ %v_next, %loop ]
    %p_next = getelementptr inbounds i8, ptr %p, i64 -1
    %digit = urem i64 %v, 10
    %digit8 = trunc i64 %digit to i8
    %ascii = add i8 %digit8, 48
    store i8 %ascii, ptr %p_next, align 1
    %v_next = udiv i64 %v, 10
    %done = icmp eq i64 %v_next, 0
    br i1 %done, label %write_out, label %loop
write_out:
    %start_int = ptrtoint ptr %p_next to i64
    %end_int = ptrtoint ptr %end to i64
    %len = sub i64 %end_int, %start_int
    call i64 @write(i32 1, ptr %p_next, i64 %len)
    ret void
}

define internal void @__print_uint_nl(i64 %n) {
    call void @__print_uint(i64 %n)
    call i64 @write(i32 1, ptr @__wasm_newline, i64 1)
    ret void
}

define internal void @__print_int(i64 %n) {
entry:
    %is_neg = icmp slt i64 %n, 0
    br i1 %is_neg, label %neg, label %pos
neg:
    call i64 @write(i32 1, ptr @__wasm_minus, i64 1)
    %n_abs = sub i64 0, %n
    call void @__print_uint(i64 %n_abs)
    ret void
pos:
    call void @__print_uint(i64 %n)
    ret void
}

define internal void @__print_int_nl(i64 %n) {
    call void @__print_int(i64 %n)
    call i64 @write(i32 1, ptr @__wasm_newline, i64 1)
    ret void
}

; compiler-rt i128 multiply (https://maskray.me/blog/2021-04-03-compiler-rt-multiplication-helpers),
; emitted by llvm.{s,u}mul.with.overflow.i64 on wasm. Schoolbook 32-bit
; decomposition of aLo*bLo plus aHi*bLo + aLo*bHi for the high half.
;
; todo: drop this once V8 ships wasm's wide-arithmetic proposal; then
; `-mattr=+wide-arithmetic` lets LLVM emit i64.mul_wide_{s,u} directly.
define void @__multi3(ptr %sret, i64 %aLo, i64 %aHi, i64 %bLo, i64 %bHi) {
    %a0 = and i64 %aLo, 4294967295
    %a1 = lshr i64 %aLo, 32
    %b0 = and i64 %bLo, 4294967295
    %b1 = lshr i64 %bLo, 32

    %p00 = mul i64 %a0, %b0
    %p01 = mul i64 %a0, %b1
    %p10 = mul i64 %a1, %b0
    %p11 = mul i64 %a1, %b1

    %p00_lo = and i64 %p00, 4294967295
    %p00_hi = lshr i64 %p00, 32
    %p01_lo = and i64 %p01, 4294967295
    %p01_hi = lshr i64 %p01, 32
    %p10_lo = and i64 %p10, 4294967295
    %p10_hi = lshr i64 %p10, 32

    %col32_a = add i64 %p00_hi, %p01_lo
    %col32   = add i64 %col32_a, %p10_lo
    %col32_lo = and i64 %col32, 4294967295
    %col32_carry = lshr i64 %col32, 32

    %col32_shl = shl i64 %col32_lo, 32
    %lolo = or i64 %p00_lo, %col32_shl

    %h_a = add i64 %p01_hi, %p10_hi
    %h_b = add i64 %h_a, %p11
    %lohi = add i64 %h_b, %col32_carry

    %cross_a = mul i64 %aHi, %bLo
    %cross_b = mul i64 %aLo, %bHi
    %cross = add i64 %cross_a, %cross_b
    %hi = add i64 %lohi, %cross

    store i64 %lolo, ptr %sret, align 8
    %sret_hi = getelementptr inbounds i8, ptr %sret, i64 8
    store i64 %hi, ptr %sret_hi, align 8
    ret void
}
