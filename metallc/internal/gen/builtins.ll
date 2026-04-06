; >>> External functions.

declare i32 @putchar(i8)
declare i32 @puts(ptr)
declare i32 @printf(ptr, ...)
declare i32 @fflush(ptr)
declare i64 @write(i32, ptr, i64)
declare i32 @dprintf(i32, ptr, ...)
declare i64 @strlen(ptr)
declare ptr @malloc(i64)

; >>> os::args global and init.
; __os_args stores the []Str slice: {ptr to array of %Str, i64 count}.
@__os_args = internal global {ptr, i64} zeroinitializer

; __os_args_init converts C argc/argv into a []Str and stores it in @__os_args.
; Each argv[i] (a null-terminated C string) becomes a %Str = { {ptr, i64} }.
define internal void @__os_args_init(i32 %argc, ptr %argv) {
entry:
    %n = sext i32 %argc to i64
    ; Allocate array of %Str (each is 16 bytes: {ptr, i64}).
    %bytes = mul i64 %n, 16
    %arr = call ptr @malloc(i64 %bytes)
    br label %loop
loop:
    %i = phi i64 [ 0, %entry ], [ %next_i, %body ]
    %done = icmp sge i64 %i, %n
    br i1 %done, label %exit, label %body
body:
    ; Load argv[i] (a char*).
    %argv_i_ptr = getelementptr ptr, ptr %argv, i64 %i
    %cstr = load ptr, ptr %argv_i_ptr
    ; Get string length.
    %len = call i64 @strlen(ptr %cstr)
    ; Store into arr[i] which is a %Str = { {ptr, i64} }.
    %str_ptr = getelementptr %Str, ptr %arr, i64 %i
    %data_field = getelementptr %Str, ptr %str_ptr, i32 0, i32 0, i32 0
    store ptr %cstr, ptr %data_field
    %len_field = getelementptr %Str, ptr %str_ptr, i32 0, i32 0, i32 1
    store i64 %len, ptr %len_field
    %next_i = add i64 %i, 1
    br label %loop
exit:
    ; Store the slice {ptr, len} into the global.
    %g_ptr = getelementptr {ptr, i64}, ptr @__os_args, i32 0, i32 0
    store ptr %arr, ptr %g_ptr
    %g_len = getelementptr {ptr, i64}, ptr @__os_args, i32 0, i32 1
    store i64 %n, ptr %g_len
    ret void
}

; >>> Builtin functions.

@__main_newline = private constant [1 x i8] c"\0A"

; __main_check_result inspects a Result<void> = {i64, {%Str}} union.
; If the tag (index 0) is non-zero, the result is an Err: print its msg to
; stderr and return 1. Otherwise return 0.
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
@__fmt_str_nl = private constant [6 x i8] c"%.*s\0A\00"
@__fmt_str = private constant [5 x i8] c"%.*s\00"
@__fmt_int_nl = private constant [6 x i8] c"%lld\0A\00"
@__fmt_uint_nl = private constant [6 x i8] c"%llu\0A\00"
@__str_true = private constant [5 x i8] c"true\00"
@__str_false = private constant [6 x i8] c"false\00"

define internal void @__print_str(ptr byval(%Str) %s) alwaysinline {
    %data_field = getelementptr %Str, ptr %s, i32 0, i32 0, i32 0
    %data = load ptr, ptr %data_field
    %len_field = getelementptr %Str, ptr %s, i32 0, i32 0, i32 1
    %len = load i64, ptr %len_field
    %len32 = trunc i64 %len to i32
    call i32 (ptr, ...) @printf(ptr @__fmt_str, i32 %len32, ptr %data)
    ret void
}

define internal void @panic(ptr byval(%Str) %s, ptr byval(%Str) %loc) noreturn alwaysinline cold {
    call void (ptr) @__print_str(ptr %loc)
    call void (i8) @putchar(i8 58)
    call void (i8) @putchar(i8 32)
    call void (ptr) @DebugIntern.print_str(ptr %s)
    call i32 @fflush(ptr null)
    call void @llvm.trap()
    unreachable
}

; Parameters: fmt (ptr), arena_ptr (i64), arg1 (i64), arg2 (i64)
define internal i32 @arena_debug_print(ptr %fmt, i64 %arena_ptr, i64 %arg1, i64 %arg2) alwaysinline {
    %r = call i32 (i32, ptr, ...) @dprintf(i32 2, ptr %fmt, i64 %arena_ptr, i64 %arg1, i64 %arg2)
    ret i32 %r
}

define internal void @DebugIntern.print_str(ptr byval(%Str) %s) {
    %data_field = getelementptr %Str, ptr %s, i32 0, i32 0, i32 0
    %data = load ptr, ptr %data_field
    %len_field = getelementptr %Str, ptr %s, i32 0, i32 0, i32 1
    %len = load i64, ptr %len_field
    %len32 = trunc i64 %len to i32
    call i32 (ptr, ...) @printf(ptr @__fmt_str_nl, i32 %len32, ptr %data)
    ret void
}

define internal void @DebugIntern.print_int(i64 %n) {
    call i32 (ptr, ...) @printf(ptr @__fmt_int_nl, i64 %n)
    ret void
}

define internal void @DebugIntern.print_uint(i64 %n) {
    call i32 (ptr, ...) @printf(ptr @__fmt_uint_nl, i64 %n)
    ret void
}

define internal void @DebugIntern.print_bool(i1 %n) {
	br i1 %n, label %true, label %false
	true:
	    call i32 @puts(ptr @__str_true)
	    ret void
	false:
		call i32 @puts(ptr @__str_false)
		ret void
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
