; >>> External functions.

declare i32 @putchar(i8)
declare i32 @puts(ptr)
declare i32 @printf(ptr, ...)
declare i32 @fflush(ptr)
declare i64 @write(i32, ptr, i64)

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
