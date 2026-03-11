; >>> External functions.

declare i32 @puts(ptr)
declare i32 @printf(ptr, ...)

; >>> Builtin functions.

@fmt_str = private constant [6 x i8] c"%.*s\0A\00"
define internal void @print_str(ptr byval(%Str) %s) {
    %data_field = getelementptr %Str, ptr %s, i32 0, i32 0, i32 0
    %data = load ptr, ptr %data_field
    %len_field = getelementptr %Str, ptr %s, i32 0, i32 0, i32 1
    %len = load i64, ptr %len_field
    %len32 = trunc i64 %len to i32
    call i32 (ptr, ...) @printf(ptr @fmt_str, i32 %len32, ptr %data)
    ret void
}

@fmt_int = private constant [6 x i8] c"%lld\0A\00"
define internal void @print_int(i64 %n) {
    call i32 (ptr, ...) @printf(ptr @fmt_int, i64 %n)
    ret void
}

@fmt_uint = private constant [6 x i8] c"%llu\0A\00"
define internal void @print_uint(i64 %n) {
    call i32 (ptr, ...) @printf(ptr @fmt_uint, i64 %n)
    ret void
}

@str_true = private constant [5 x i8] c"true\00"
@str_false = private constant [6 x i8] c"false\00"
define internal void @print_bool(i1 %n) {
	br i1 %n, label %true, label %false
	true:
	    call i32 @puts(ptr @str_true)
	    ret void
	false:
		call i32 @puts(ptr @str_false)
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

; >>> Signed narrowing clamped conversions.

define internal i8 @"I16.to_i8_clamped"(i16 %v) alwaysinline {
    %lo = icmp slt i16 %v, -128
    %v1 = select i1 %lo, i16 -128, i16 %v
    %hi = icmp sgt i16 %v1, 127
    %v2 = select i1 %hi, i16 127, i16 %v1
    %r = trunc i16 %v2 to i8
    ret i8 %r
}
define internal i8 @"I32.to_i8_clamped"(i32 %v) alwaysinline {
    %lo = icmp slt i32 %v, -128
    %v1 = select i1 %lo, i32 -128, i32 %v
    %hi = icmp sgt i32 %v1, 127
    %v2 = select i1 %hi, i32 127, i32 %v1
    %r = trunc i32 %v2 to i8
    ret i8 %r
}
define internal i8 @"Int.to_i8_clamped"(i64 %v) alwaysinline {
    %lo = icmp slt i64 %v, -128
    %v1 = select i1 %lo, i64 -128, i64 %v
    %hi = icmp sgt i64 %v1, 127
    %v2 = select i1 %hi, i64 127, i64 %v1
    %r = trunc i64 %v2 to i8
    ret i8 %r
}
define internal i16 @"I32.to_i16_clamped"(i32 %v) alwaysinline {
    %lo = icmp slt i32 %v, -32768
    %v1 = select i1 %lo, i32 -32768, i32 %v
    %hi = icmp sgt i32 %v1, 32767
    %v2 = select i1 %hi, i32 32767, i32 %v1
    %r = trunc i32 %v2 to i16
    ret i16 %r
}
define internal i16 @"Int.to_i16_clamped"(i64 %v) alwaysinline {
    %lo = icmp slt i64 %v, -32768
    %v1 = select i1 %lo, i64 -32768, i64 %v
    %hi = icmp sgt i64 %v1, 32767
    %v2 = select i1 %hi, i64 32767, i64 %v1
    %r = trunc i64 %v2 to i16
    ret i16 %r
}
define internal i32 @"Int.to_i32_clamped"(i64 %v) alwaysinline {
    %lo = icmp slt i64 %v, -2147483648
    %v1 = select i1 %lo, i64 -2147483648, i64 %v
    %hi = icmp sgt i64 %v1, 2147483647
    %v2 = select i1 %hi, i64 2147483647, i64 %v1
    %r = trunc i64 %v2 to i32
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

; >>> Unsigned narrowing clamped conversions.

define internal i8 @"U16.to_u8_clamped"(i16 %v) alwaysinline {
    %hi = icmp ugt i16 %v, 255
    %v1 = select i1 %hi, i16 255, i16 %v
    %r = trunc i16 %v1 to i8
    ret i8 %r
}
define internal i8 @"U32.to_u8_clamped"(i32 %v) alwaysinline {
    %hi = icmp ugt i32 %v, 255
    %v1 = select i1 %hi, i32 255, i32 %v
    %r = trunc i32 %v1 to i8
    ret i8 %r
}
define internal i8 @"U64.to_u8_clamped"(i64 %v) alwaysinline {
    %hi = icmp ugt i64 %v, 255
    %v1 = select i1 %hi, i64 255, i64 %v
    %r = trunc i64 %v1 to i8
    ret i8 %r
}
define internal i16 @"U32.to_u16_clamped"(i32 %v) alwaysinline {
    %hi = icmp ugt i32 %v, 65535
    %v1 = select i1 %hi, i32 65535, i32 %v
    %r = trunc i32 %v1 to i16
    ret i16 %r
}
define internal i16 @"U64.to_u16_clamped"(i64 %v) alwaysinline {
    %hi = icmp ugt i64 %v, 65535
    %v1 = select i1 %hi, i64 65535, i64 %v
    %r = trunc i64 %v1 to i16
    ret i16 %r
}
define internal i32 @"U64.to_u32_clamped"(i64 %v) alwaysinline {
    %hi = icmp ugt i64 %v, 4294967295
    %v1 = select i1 %hi, i64 4294967295, i64 %v
    %r = trunc i64 %v1 to i32
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

; >>> Unsigned to signed narrowing/same-width clamped conversions.

define internal i8 @"U8.to_i8_clamped"(i8 %v) alwaysinline {
    %hi = icmp ugt i8 %v, 127
    %r = select i1 %hi, i8 127, i8 %v
    ret i8 %r
}
define internal i16 @"U16.to_i16_clamped"(i16 %v) alwaysinline {
    %hi = icmp ugt i16 %v, 32767
    %r = select i1 %hi, i16 32767, i16 %v
    ret i16 %r
}
define internal i8 @"U16.to_i8_clamped"(i16 %v) alwaysinline {
    %hi = icmp ugt i16 %v, 127
    %v1 = select i1 %hi, i16 127, i16 %v
    %r = trunc i16 %v1 to i8
    ret i8 %r
}
define internal i32 @"U32.to_i32_clamped"(i32 %v) alwaysinline {
    %hi = icmp ugt i32 %v, 2147483647
    %r = select i1 %hi, i32 2147483647, i32 %v
    ret i32 %r
}
define internal i16 @"U32.to_i16_clamped"(i32 %v) alwaysinline {
    %hi = icmp ugt i32 %v, 32767
    %v1 = select i1 %hi, i32 32767, i32 %v
    %r = trunc i32 %v1 to i16
    ret i16 %r
}
define internal i8 @"U32.to_i8_clamped"(i32 %v) alwaysinline {
    %hi = icmp ugt i32 %v, 127
    %v1 = select i1 %hi, i32 127, i32 %v
    %r = trunc i32 %v1 to i8
    ret i8 %r
}
define internal i64 @"U64.to_int_clamped"(i64 %v) alwaysinline {
    %hi = icmp ugt i64 %v, 9223372036854775807
    %r = select i1 %hi, i64 9223372036854775807, i64 %v
    ret i64 %r
}
define internal i32 @"U64.to_i32_clamped"(i64 %v) alwaysinline {
    %hi = icmp ugt i64 %v, 2147483647
    %v1 = select i1 %hi, i64 2147483647, i64 %v
    %r = trunc i64 %v1 to i32
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

; >>> Signed to unsigned clamped conversions (clamp negative to 0, then clamp upper bound).

define internal i8 @"I8.to_u8_clamped"(i8 %v) alwaysinline {
    %neg = icmp slt i8 %v, 0
    %r = select i1 %neg, i8 0, i8 %v
    ret i8 %r
}
define internal i16 @"I8.to_u16_clamped"(i8 %v) alwaysinline {
    %neg = icmp slt i8 %v, 0
    %v1 = select i1 %neg, i8 0, i8 %v
    %r = zext i8 %v1 to i16
    ret i16 %r
}
define internal i32 @"I8.to_u32_clamped"(i8 %v) alwaysinline {
    %neg = icmp slt i8 %v, 0
    %v1 = select i1 %neg, i8 0, i8 %v
    %r = zext i8 %v1 to i32
    ret i32 %r
}
define internal i64 @"I8.to_u64_clamped"(i8 %v) alwaysinline {
    %neg = icmp slt i8 %v, 0
    %v1 = select i1 %neg, i8 0, i8 %v
    %r = zext i8 %v1 to i64
    ret i64 %r
}
define internal i8 @"I16.to_u8_clamped"(i16 %v) alwaysinline {
    %neg = icmp slt i16 %v, 0
    %v1 = select i1 %neg, i16 0, i16 %v
    %hi = icmp sgt i16 %v1, 255
    %v2 = select i1 %hi, i16 255, i16 %v1
    %r = trunc i16 %v2 to i8
    ret i8 %r
}
define internal i16 @"I16.to_u16_clamped"(i16 %v) alwaysinline {
    %neg = icmp slt i16 %v, 0
    %r = select i1 %neg, i16 0, i16 %v
    ret i16 %r
}
define internal i32 @"I16.to_u32_clamped"(i16 %v) alwaysinline {
    %neg = icmp slt i16 %v, 0
    %v1 = select i1 %neg, i16 0, i16 %v
    %r = zext i16 %v1 to i32
    ret i32 %r
}
define internal i64 @"I16.to_u64_clamped"(i16 %v) alwaysinline {
    %neg = icmp slt i16 %v, 0
    %v1 = select i1 %neg, i16 0, i16 %v
    %r = zext i16 %v1 to i64
    ret i64 %r
}
define internal i8 @"I32.to_u8_clamped"(i32 %v) alwaysinline {
    %neg = icmp slt i32 %v, 0
    %v1 = select i1 %neg, i32 0, i32 %v
    %hi = icmp sgt i32 %v1, 255
    %v2 = select i1 %hi, i32 255, i32 %v1
    %r = trunc i32 %v2 to i8
    ret i8 %r
}
define internal i16 @"I32.to_u16_clamped"(i32 %v) alwaysinline {
    %neg = icmp slt i32 %v, 0
    %v1 = select i1 %neg, i32 0, i32 %v
    %hi = icmp sgt i32 %v1, 65535
    %v2 = select i1 %hi, i32 65535, i32 %v1
    %r = trunc i32 %v2 to i16
    ret i16 %r
}
define internal i32 @"I32.to_u32_clamped"(i32 %v) alwaysinline {
    %neg = icmp slt i32 %v, 0
    %r = select i1 %neg, i32 0, i32 %v
    ret i32 %r
}
define internal i64 @"I32.to_u64_clamped"(i32 %v) alwaysinline {
    %neg = icmp slt i32 %v, 0
    %v1 = select i1 %neg, i32 0, i32 %v
    %r = zext i32 %v1 to i64
    ret i64 %r
}
define internal i8 @"Int.to_u8_clamped"(i64 %v) alwaysinline {
    %neg = icmp slt i64 %v, 0
    %v1 = select i1 %neg, i64 0, i64 %v
    %hi = icmp sgt i64 %v1, 255
    %v2 = select i1 %hi, i64 255, i64 %v1
    %r = trunc i64 %v2 to i8
    ret i8 %r
}
define internal i16 @"Int.to_u16_clamped"(i64 %v) alwaysinline {
    %neg = icmp slt i64 %v, 0
    %v1 = select i1 %neg, i64 0, i64 %v
    %hi = icmp sgt i64 %v1, 65535
    %v2 = select i1 %hi, i64 65535, i64 %v1
    %r = trunc i64 %v2 to i16
    ret i16 %r
}
define internal i32 @"Int.to_u32_clamped"(i64 %v) alwaysinline {
    %neg = icmp slt i64 %v, 0
    %v1 = select i1 %neg, i64 0, i64 %v
    %hi = icmp sgt i64 %v1, 4294967295
    %v2 = select i1 %hi, i64 4294967295, i64 %v1
    %r = trunc i64 %v2 to i32
    ret i32 %r
}
define internal i64 @"Int.to_u64_clamped"(i64 %v) alwaysinline {
    %neg = icmp slt i64 %v, 0
    %r = select i1 %neg, i64 0, i64 %v
    ret i64 %r
}

; >>> Rune builtins.

define internal void @__rune_check(i32 %v) alwaysinline {
    %above_max = icmp ugt i32 %v, 1114111
    br i1 %above_max, label %panic, label %check_surrogate
check_surrogate:
    %above_d7ff = icmp ugt i32 %v, 55295
    %below_e000 = icmp ult i32 %v, 57344
    %in_surrogate = and i1 %above_d7ff, %below_e000
    br i1 %in_surrogate, label %panic, label %ok
panic:
    call void @llvm.trap()
    unreachable
ok:
    ret void
}

define internal i32 @"Rune.to_u32"(i32 %v) alwaysinline {
    ret i32 %v
}
