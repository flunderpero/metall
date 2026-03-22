; >>> External functions.

declare i32 @putchar(i8)
declare i32 @puts(ptr)
declare i32 @printf(ptr, ...)
declare i32 @fflush(ptr)
declare ptr @fopen(ptr, ptr)
declare i32 @fclose(ptr)
declare ptr @popen(ptr, ptr)
declare i32 @pclose(ptr)
declare i64 @fwrite(ptr, i64, i64, ptr)
declare i64 @fread(ptr, i64, i64, ptr)
declare ptr @__error()
declare ptr @strerror(i32)
declare i64 @strlen(ptr)
declare i32 @memcmp(ptr, ptr, i64)
declare i64 @write(i32, ptr, i64)

; >>> Builtin functions.

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

define internal void @panic(ptr byval(%Str) %s, ptr byval(%Str) %loc) noreturn alwaysinline {
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

; >>> IO.

define internal i64 @LibCIntern.fopen(ptr byval(%CStr) %path, ptr byval(%CStr) %mode) {
    %path_data_field = getelementptr %CStr, ptr %path, i32 0, i32 0, i32 0
    %path_ptr = load ptr, ptr %path_data_field
    %mode_data_field = getelementptr %CStr, ptr %mode, i32 0, i32 0, i32 0
    %mode_ptr = load ptr, ptr %mode_data_field
    %fp = call ptr @fopen(ptr %path_ptr, ptr %mode_ptr)
    %fd = ptrtoint ptr %fp to i64
    ret i64 %fd
}

define internal i64 @LibCIntern.fwrite(i64 %fd, ptr byval({ptr, i64}) %data) {
    %data_ptr_field = getelementptr {ptr, i64}, ptr %data, i32 0, i32 0
    %data_ptr = load ptr, ptr %data_ptr_field
    %data_len_field = getelementptr {ptr, i64}, ptr %data, i32 0, i32 1
    %data_len = load i64, ptr %data_len_field
    %fp = inttoptr i64 %fd to ptr
    %written = call i64 @fwrite(ptr %data_ptr, i64 1, i64 %data_len, ptr %fp)
    ret i64 %written
}

define internal i64 @LibCIntern.fread(i64 %fd, ptr byval({ptr, i64}) %buf) {
    %buf_ptr_field = getelementptr {ptr, i64}, ptr %buf, i32 0, i32 0
    %buf_ptr = load ptr, ptr %buf_ptr_field
    %buf_len_field = getelementptr {ptr, i64}, ptr %buf, i32 0, i32 1
    %buf_len = load i64, ptr %buf_len_field
    %fp = inttoptr i64 %fd to ptr
    %read = call i64 @fread(ptr %buf_ptr, i64 1, i64 %buf_len, ptr %fp)
    ret i64 %read
}

define internal i32 @LibCIntern.fclose(i64 %fd) {
    %fp = inttoptr i64 %fd to ptr
    %result = call i32 @fclose(ptr %fp)
    ret i32 %result
}

define internal i64 @LibCIntern.popen(ptr byval(%CStr) %cmd, ptr byval(%CStr) %mode) {
    %cmd_data_field = getelementptr %CStr, ptr %cmd, i32 0, i32 0, i32 0
    %cmd_ptr = load ptr, ptr %cmd_data_field
    %mode_data_field = getelementptr %CStr, ptr %mode, i32 0, i32 0, i32 0
    %mode_ptr = load ptr, ptr %mode_data_field
    %fp = call ptr @popen(ptr %cmd_ptr, ptr %mode_ptr)
    %fd = ptrtoint ptr %fp to i64
    ret i64 %fd
}

define internal i32 @LibCIntern.pclose(i64 %fd) {
    %fp = inttoptr i64 %fd to ptr
    %result = call i32 @pclose(ptr %fp)
    ret i32 %result
}

define internal void @LibCIntern.strerror(ptr sret(%Str) %ret, i32 %errnum) {
    %cstr = call ptr @strerror(i32 %errnum)
    %len = call i64 @strlen(ptr %cstr)
    %data_field = getelementptr %Str, ptr %ret, i32 0, i32 0, i32 0
    store ptr %cstr, ptr %data_field
    %len_field = getelementptr %Str, ptr %ret, i32 0, i32 0, i32 1
    store i64 %len, ptr %len_field
    ret void
}

define internal i32 @LibCIntern.errno() {
    %errno_ptr = call ptr @__error()
    %errno_val = load i32, ptr %errno_ptr
    ret i32 %errno_val
}

define internal void @LibCIntern.reset_errno() {
    %errno_ptr = call ptr @__error()
    store i32 0, ptr %errno_ptr
    ret void
}

define internal i64 @"LibCIntern.write"(i32 %fd, ptr byval({ptr, i64}) %data) {
    %data_ptr_field = getelementptr {ptr, i64}, ptr %data, i32 0, i32 0
    %data_ptr = load ptr, ptr %data_ptr_field
    %data_len_field = getelementptr {ptr, i64}, ptr %data, i32 0, i32 1
    %data_len = load i64, ptr %data_len_field
    %written = call i64 @write(i32 %fd, ptr %data_ptr, i64 %data_len)
    ret i64 %written
}

; >>> String constants.

@str_division_by_zero.data = private constant [16 x i8] c"division by zero"
@str_division_by_zero = private constant %Str { { ptr, i64 } { ptr @str_division_by_zero.data, i64 16 } }
@str_illegal_rune.data = private constant [12 x i8] c"illegal rune"
@str_illegal_rune = private constant %Str { { ptr, i64 } { ptr @str_illegal_rune.data, i64 12 } }
@str_index_out_of_bounds.data = private constant [19 x i8] c"index out of bounds"
@str_index_out_of_bounds = private constant %Str { { ptr, i64 } { ptr @str_index_out_of_bounds.data, i64 19 } }
@str_slice_out_of_bounds.data = private constant [19 x i8] c"slice out of bounds"
@str_slice_out_of_bounds = private constant %Str { { ptr, i64 } { ptr @str_slice_out_of_bounds.data, i64 19 } }
