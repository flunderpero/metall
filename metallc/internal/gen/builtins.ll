; Platform-agnostic runtime. All printing goes through @__print_str,
; @__print_str_nl, @__print_{int,uint}{,_nl}; each target provides them
; in builtins_{posix,wasm}.ll along with declarations for @write,
; @fflush, and @llvm.trap.

; @__os_args_init is posix-only; on wasm the slice stays zero-init and
; os.args() returns empty.
@__os_args = internal global {ptr, i64} zeroinitializer

@__main_newline = private constant [1 x i8] c"\0A"
@__panic_sep = private constant [2 x i8] c": "
@__str_true.data = private constant [4 x i8] c"true"
@__str_false.data = private constant [5 x i8] c"false"

; Result<void> = {i64 tag, {%Str}}: tag != 0 => Err, print msg and exit 1.
define internal i32 @__main_check_result(ptr %result) alwaysinline {
    %tag_ptr = getelementptr {i64, %Str}, ptr %result, i32 0, i32 0
    %tag = load i64, ptr %tag_ptr
    %is_err = icmp ne i64 %tag, 0
    br i1 %is_err, label %err, label %ok
err:
    %msg_ptr = getelementptr {i64, %Str}, ptr %result, i32 0, i32 1, i32 0, i32 0
    %msg_data = load ptr, ptr %msg_ptr
    %len_ptr = getelementptr {i64, %Str}, ptr %result, i32 0, i32 1, i32 0, i32 1
    %msg_len = load i64, ptr %len_ptr
    call i64 @write(i32 2, ptr %msg_data, i64 %msg_len)
    call i64 @write(i32 2, ptr @__main_newline, i64 1)
    ret i32 1
ok:
    ret i32 0
}

define internal void @panic(ptr byval(%Str) %s, ptr byval(%Str) %loc) noreturn alwaysinline cold {
    %loc_data_field = getelementptr %Str, ptr %loc, i32 0, i32 0, i32 0
    %loc_data = load ptr, ptr %loc_data_field
    %loc_len_field = getelementptr %Str, ptr %loc, i32 0, i32 0, i32 1
    %loc_len = load i64, ptr %loc_len_field
    call void @__print_str(ptr %loc_data, i64 %loc_len)
    call void @__print_str(ptr @__panic_sep, i64 2)
    %s_data_field = getelementptr %Str, ptr %s, i32 0, i32 0, i32 0
    %s_data = load ptr, ptr %s_data_field
    %s_len_field = getelementptr %Str, ptr %s, i32 0, i32 0, i32 1
    %s_len = load i64, ptr %s_len_field
    call void @__print_str_nl(ptr %s_data, i64 %s_len)
    call i32 @fflush(ptr null)
    call void @llvm.trap()
    unreachable
}

define internal void @DebugIntern.print_str(ptr byval(%Str) %s) {
    %data_field = getelementptr %Str, ptr %s, i32 0, i32 0, i32 0
    %data = load ptr, ptr %data_field
    %len_field = getelementptr %Str, ptr %s, i32 0, i32 0, i32 1
    %len = load i64, ptr %len_field
    call void @__print_str_nl(ptr %data, i64 %len)
    ret void
}

define internal void @DebugIntern.print_int(i64 %n) {
    call void @__print_int_nl(i64 %n)
    ret void
}

define internal void @DebugIntern.print_uint(i64 %n) {
    call void @__print_uint_nl(i64 %n)
    ret void
}

define internal void @DebugIntern.print_bool(i1 %n) {
    br i1 %n, label %true, label %false
true:
    call void @__print_str_nl(ptr @__str_true.data, i64 4)
    ret void
false:
    call void @__print_str_nl(ptr @__str_false.data, i64 5)
    ret void
}

@__arena_dbg_open  = private constant [9 x i8] c"arena [0x"
@__arena_dbg_close = private constant [2 x i8] c"] "

; __print_hex_u64 writes %n as 16 zero-padded lowercase hex digits.
define internal void @__print_hex_u64(i64 %n) {
entry:
    %buf = alloca [16 x i8], align 1
    br label %loop
loop:
    %i = phi i64 [ 0, %entry ], [ %i_next, %loop ]
    %shift_idx = sub i64 15, %i
    %shift_bits = shl i64 %shift_idx, 2
    %shifted = lshr i64 %n, %shift_bits
    %nibble = and i64 %shifted, 15
    %is_letter = icmp uge i64 %nibble, 10
    ; '0' = 48, 'a' - 10 = 87.
    %offset = select i1 %is_letter, i64 87, i64 48
    %code = add i64 %nibble, %offset
    %ch = trunc i64 %code to i8
    %p = getelementptr inbounds i8, ptr %buf, i64 %i
    store i8 %ch, ptr %p, align 1
    %i_next = add i64 %i, 1
    %done = icmp eq i64 %i_next, 16
    br i1 %done, label %exit, label %loop
exit:
    call void @__print_str(ptr %buf, i64 16)
    ret void
}

; Prints "arena [0x<hex>] " then %fmt with "$1"/"$2" replaced by %arg1/%arg2.
define internal i32 @arena_debug_print(ptr %fmt, i64 %fmt_len, i64 %arena_ptr, i64 %arg1, i64 %arg2) {
entry:
    call void @__print_str(ptr @__arena_dbg_open, i64 9)
    call void @__print_hex_u64(i64 %arena_ptr)
    call void @__print_str(ptr @__arena_dbg_close, i64 2)
    br label %scan
scan:
    %i = phi i64 [ 0, %entry ], [ %i_adv, %advance ], [ %i_after_ph, %emit_arg ]
    %start = phi i64 [ 0, %entry ], [ %start, %advance ], [ %i_after_ph, %emit_arg ]
    %at_end = icmp uge i64 %i, %fmt_len
    br i1 %at_end, label %tail, label %check_dollar
check_dollar:
    %p = getelementptr inbounds i8, ptr %fmt, i64 %i
    %b = load i8, ptr %p, align 1
    %is_dollar = icmp eq i8 %b, 36 ; '$'
    br i1 %is_dollar, label %check_digit, label %advance
check_digit:
    %i_plus1 = add i64 %i, 1
    %past = icmp uge i64 %i_plus1, %fmt_len
    br i1 %past, label %advance, label %load_digit
load_digit:
    %pn = getelementptr inbounds i8, ptr %fmt, i64 %i_plus1
    %bn = load i8, ptr %pn, align 1
    %is_1 = icmp eq i8 %bn, 49 ; '1'
    %is_2 = icmp eq i8 %bn, 50 ; '2'
    %is_12 = or i1 %is_1, %is_2
    br i1 %is_12, label %emit_ph, label %advance
emit_ph:
    ; Flush the literal chunk [start, i) before emitting the placeholder.
    %chunk_len = sub i64 %i, %start
    %chunk_nonempty = icmp ne i64 %chunk_len, 0
    br i1 %chunk_nonempty, label %flush_lit, label %emit_arg
flush_lit:
    %chunk_ptr = getelementptr inbounds i8, ptr %fmt, i64 %start
    call void @__print_str(ptr %chunk_ptr, i64 %chunk_len)
    br label %emit_arg
emit_arg:
    %arg_val = select i1 %is_1, i64 %arg1, i64 %arg2
    call void @__print_uint(i64 %arg_val)
    %i_after_ph = add i64 %i_plus1, 1
    br label %scan
advance:
    %i_adv = add i64 %i, 1
    br label %scan
tail:
    %tail_len = sub i64 %fmt_len, %start
    %tail_nonempty = icmp ne i64 %tail_len, 0
    br i1 %tail_nonempty, label %flush_tail, label %done
flush_tail:
    %tail_ptr = getelementptr inbounds i8, ptr %fmt, i64 %start
    call void @__print_str(ptr %tail_ptr, i64 %tail_len)
    br label %done
done:
    ret i32 0
}

define internal void @__fill_cpy(ptr %dst, ptr %val, i64 %elem_size, i64 %count) {
entry:
    br label %loop
loop:
    %i = phi i64 [ 0, %entry ], [ %next_i, %body ]
    %curr_ptr = phi ptr [ %dst, %entry ], [ %next_ptr, %body ]
    %done = icmp sge i64 %i, %count
    br i1 %done, label %exit, label %body
body:
    call void @llvm.memcpy.p0.p0.i64(ptr %curr_ptr, ptr %val, i64 %elem_size, i1 false)
    %next_i = add i64 %i, 1
    %next_ptr = getelementptr i8, ptr %curr_ptr, i64 %elem_size
    br label %loop
exit:
    ret void
}

; >>> Signed widening conversions.

define internal i16 @"I8.to_i16"(i8 %v) alwaysinline {
    %r = sext i8 %v to i16
    ret i16 %r
}
define internal i32 @"I8.to_i32"(i8 %v) alwaysinline {
    %r = sext i8 %v to i32
    ret i32 %r
}
define internal i64 @"I8.to_int"(i8 %v) alwaysinline {
    %r = sext i8 %v to i64
    ret i64 %r
}
define internal i32 @"I16.to_i32"(i16 %v) alwaysinline {
    %r = sext i16 %v to i32
    ret i32 %r
}
define internal i64 @"I16.to_int"(i16 %v) alwaysinline {
    %r = sext i16 %v to i64
    ret i64 %r
}
define internal i64 @"I32.to_int"(i32 %v) alwaysinline {
    %r = sext i32 %v to i64
    ret i64 %r
}

; >>> Unsigned widening conversions.

define internal i16 @"U8.to_u16"(i8 %v) alwaysinline {
    %r = zext i8 %v to i16
    ret i16 %r
}
define internal i32 @"U8.to_u32"(i8 %v) alwaysinline {
    %r = zext i8 %v to i32
    ret i32 %r
}
define internal i64 @"U8.to_u64"(i8 %v) alwaysinline {
    %r = zext i8 %v to i64
    ret i64 %r
}
define internal i32 @"U16.to_u32"(i16 %v) alwaysinline {
    %r = zext i16 %v to i32
    ret i32 %r
}
define internal i64 @"U16.to_u64"(i16 %v) alwaysinline {
    %r = zext i16 %v to i64
    ret i64 %r
}
define internal i64 @"U32.to_u64"(i32 %v) alwaysinline {
    %r = zext i32 %v to i64
    ret i64 %r
}

; >>> Unsigned to signed widening conversions.

define internal i16 @"U8.to_i16"(i8 %v) alwaysinline {
    %r = zext i8 %v to i16
    ret i16 %r
}
define internal i32 @"U8.to_i32"(i8 %v) alwaysinline {
    %r = zext i8 %v to i32
    ret i32 %r
}
define internal i64 @"U8.to_int"(i8 %v) alwaysinline {
    %r = zext i8 %v to i64
    ret i64 %r
}
define internal i32 @"U16.to_i32"(i16 %v) alwaysinline {
    %r = zext i16 %v to i32
    ret i32 %r
}
define internal i64 @"U16.to_int"(i16 %v) alwaysinline {
    %r = zext i16 %v to i64
    ret i64 %r
}
define internal i64 @"U32.to_int"(i32 %v) alwaysinline {
    %r = zext i32 %v to i64
    ret i64 %r
}

; >>> Signed narrowing wrapping conversions (trunc).

define internal i8 @"I16.to_i8_wrapping"(i16 %v) alwaysinline {
    %r = trunc i16 %v to i8
    ret i8 %r
}
define internal i8 @"I32.to_i8_wrapping"(i32 %v) alwaysinline {
    %r = trunc i32 %v to i8
    ret i8 %r
}
define internal i8 @"Int.to_i8_wrapping"(i64 %v) alwaysinline {
    %r = trunc i64 %v to i8
    ret i8 %r
}
define internal i16 @"I32.to_i16_wrapping"(i32 %v) alwaysinline {
    %r = trunc i32 %v to i16
    ret i16 %r
}
define internal i16 @"Int.to_i16_wrapping"(i64 %v) alwaysinline {
    %r = trunc i64 %v to i16
    ret i16 %r
}
define internal i32 @"Int.to_i32_wrapping"(i64 %v) alwaysinline {
    %r = trunc i64 %v to i32
    ret i32 %r
}

; >>> Unsigned narrowing wrapping conversions (trunc).

define internal i8 @"U16.to_u8_wrapping"(i16 %v) alwaysinline {
    %r = trunc i16 %v to i8
    ret i8 %r
}
define internal i8 @"U32.to_u8_wrapping"(i32 %v) alwaysinline {
    %r = trunc i32 %v to i8
    ret i8 %r
}
define internal i8 @"U64.to_u8_wrapping"(i64 %v) alwaysinline {
    %r = trunc i64 %v to i8
    ret i8 %r
}
define internal i16 @"U32.to_u16_wrapping"(i32 %v) alwaysinline {
    %r = trunc i32 %v to i16
    ret i16 %r
}
define internal i16 @"U64.to_u16_wrapping"(i64 %v) alwaysinline {
    %r = trunc i64 %v to i16
    ret i16 %r
}
define internal i32 @"U64.to_u32_wrapping"(i64 %v) alwaysinline {
    %r = trunc i64 %v to i32
    ret i32 %r
}

; >>> Unsigned to signed narrowing/same-width wrapping conversions.

define internal i8 @"U8.to_i8_wrapping"(i8 %v) alwaysinline {
    ret i8 %v
}
define internal i16 @"U16.to_i16_wrapping"(i16 %v) alwaysinline {
    ret i16 %v
}
define internal i8 @"U16.to_i8_wrapping"(i16 %v) alwaysinline {
    %r = trunc i16 %v to i8
    ret i8 %r
}
define internal i32 @"U32.to_i32_wrapping"(i32 %v) alwaysinline {
    ret i32 %v
}
define internal i16 @"U32.to_i16_wrapping"(i32 %v) alwaysinline {
    %r = trunc i32 %v to i16
    ret i16 %r
}
define internal i8 @"U32.to_i8_wrapping"(i32 %v) alwaysinline {
    %r = trunc i32 %v to i8
    ret i8 %r
}
define internal i64 @"U64.to_int_wrapping"(i64 %v) alwaysinline {
    ret i64 %v
}
define internal i32 @"U64.to_i32_wrapping"(i64 %v) alwaysinline {
    %r = trunc i64 %v to i32
    ret i32 %r
}

; >>> Signed to unsigned wrapping conversions.

define internal i8 @"I8.to_u8_wrapping"(i8 %v) alwaysinline {
    ret i8 %v
}
define internal i16 @"I8.to_u16_wrapping"(i8 %v) alwaysinline {
    %r = sext i8 %v to i16
    ret i16 %r
}
define internal i32 @"I8.to_u32_wrapping"(i8 %v) alwaysinline {
    %r = sext i8 %v to i32
    ret i32 %r
}
define internal i64 @"I8.to_u64_wrapping"(i8 %v) alwaysinline {
    %r = sext i8 %v to i64
    ret i64 %r
}
define internal i8 @"I16.to_u8_wrapping"(i16 %v) alwaysinline {
    %r = trunc i16 %v to i8
    ret i8 %r
}
define internal i16 @"I16.to_u16_wrapping"(i16 %v) alwaysinline {
    ret i16 %v
}
define internal i32 @"I16.to_u32_wrapping"(i16 %v) alwaysinline {
    %r = sext i16 %v to i32
    ret i32 %r
}
define internal i64 @"I16.to_u64_wrapping"(i16 %v) alwaysinline {
    %r = sext i16 %v to i64
    ret i64 %r
}
define internal i8 @"I32.to_u8_wrapping"(i32 %v) alwaysinline {
    %r = trunc i32 %v to i8
    ret i8 %r
}
define internal i16 @"I32.to_u16_wrapping"(i32 %v) alwaysinline {
    %r = trunc i32 %v to i16
    ret i16 %r
}
define internal i32 @"I32.to_u32_wrapping"(i32 %v) alwaysinline {
    ret i32 %v
}
define internal i64 @"I32.to_u64_wrapping"(i32 %v) alwaysinline {
    %r = sext i32 %v to i64
    ret i64 %r
}
define internal i8 @"Int.to_u8_wrapping"(i64 %v) alwaysinline {
    %r = trunc i64 %v to i8
    ret i8 %r
}
define internal i16 @"Int.to_u16_wrapping"(i64 %v) alwaysinline {
    %r = trunc i64 %v to i16
    ret i16 %r
}
define internal i32 @"Int.to_u32_wrapping"(i64 %v) alwaysinline {
    %r = trunc i64 %v to i32
    ret i32 %r
}
define internal i64 @"Int.to_u64_wrapping"(i64 %v) alwaysinline {
    ret i64 %v
}

; >>> Rune builtins.

define internal i32 @"Rune.to_u32"(i32 %v) alwaysinline {
    ret i32 %v
}

@str_division_by_zero.data = private constant [16 x i8] c"division by zero"
@str_division_by_zero = private constant %Str { { ptr, i64 } { ptr @str_division_by_zero.data, i64 16 } }
@str_illegal_rune.data = private constant [12 x i8] c"illegal rune"
@str_illegal_rune = private constant %Str { { ptr, i64 } { ptr @str_illegal_rune.data, i64 12 } }
@str_integer_overflow.data = private constant [16 x i8] c"integer overflow"
@str_integer_overflow = private constant %Str { { ptr, i64 } { ptr @str_integer_overflow.data, i64 16 } }
@str_index_out_of_bounds.data = private constant [19 x i8] c"index out of bounds"
@str_index_out_of_bounds = private constant %Str { { ptr, i64 } { ptr @str_index_out_of_bounds.data, i64 19 } }
@str_slice_out_of_bounds.data = private constant [19 x i8] c"slice out of bounds"
@str_slice_out_of_bounds = private constant %Str { { ptr, i64 } { ptr @str_slice_out_of_bounds.data, i64 19 } }

; __fun_ptr_ctx_copy copies a closure's capture context to the arena.
; The context is prefixed by an i64 size (see emitClosureValue).
; Returns null if data_ptr is null (non-capturing function).
define internal ptr @__fun_ptr_ctx_copy(ptr %arena, ptr %data_ptr) alwaysinline {
    %is_null = icmp eq ptr %data_ptr, null
    br i1 %is_null, label %ret_null, label %do_copy
ret_null:
    ret ptr null
do_copy:
    %base = getelementptr i8, ptr %data_ptr, i64 -8
    %size = load i64, ptr %base
    %new_base = call ptr @runtime$arena.arena_alloc(ptr %arena, i64 %size)
    call void @llvm.memcpy.p0.p0.i64(ptr %new_base, ptr %base, i64 %size, i1 false)
    %new_ctx = getelementptr i8, ptr %new_base, i64 8
    ret ptr %new_ctx
}
