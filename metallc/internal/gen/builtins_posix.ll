; POSIX runtime: print primitives via libc dprintf (write to an explicit fd),
; plus @__os_args_init to marshal argv into @__os_args.

declare i32 @dprintf(i32, ptr, ...)
declare i32 @fflush(ptr)
declare i64 @write(i32, ptr, i64)
declare i64 @strlen(ptr)
declare ptr @malloc(i64)
declare ptr @realloc(ptr, i64)
declare void @free(ptr)

@__posix_fmt_str = private constant [5 x i8] c"%.*s\00"
@__posix_fmt_str_nl = private constant [6 x i8] c"%.*s\0A\00"
@__posix_fmt_int = private constant [5 x i8] c"%lld\00"
@__posix_fmt_int_nl = private constant [6 x i8] c"%lld\0A\00"
@__posix_fmt_uint = private constant [5 x i8] c"%llu\00"
@__posix_fmt_uint_nl = private constant [6 x i8] c"%llu\0A\00"

define internal void @__print_str(i32 %fd, i1 %nl, ptr %data, i64 %len) alwaysinline {
    %len32 = trunc i64 %len to i32
    %fmt = select i1 %nl, ptr @__posix_fmt_str_nl, ptr @__posix_fmt_str
    call i32 (i32, ptr, ...) @dprintf(i32 %fd, ptr %fmt, i32 %len32, ptr %data)
    ret void
}

define internal void @__print_int(i32 %fd, i1 %nl, i64 %n) alwaysinline {
    %fmt = select i1 %nl, ptr @__posix_fmt_int_nl, ptr @__posix_fmt_int
    call i32 (i32, ptr, ...) @dprintf(i32 %fd, ptr %fmt, i64 %n)
    ret void
}

define internal void @__print_uint(i32 %fd, i1 %nl, i64 %n) alwaysinline {
    %fmt = select i1 %nl, ptr @__posix_fmt_uint_nl, ptr @__posix_fmt_uint
    call i32 (i32, ptr, ...) @dprintf(i32 %fd, ptr %fmt, i64 %n)
    ret void
}

; Converts argc/argv into a []Str and stores it in @__os_args.
define internal void @__os_args_init(i32 %argc, ptr %argv) {
entry:
    %n = sext i32 %argc to i64
    %bytes = mul i64 %n, 16
    %arr = call ptr @malloc(i64 %bytes)
    br label %loop
loop:
    %i = phi i64 [ 0, %entry ], [ %next_i, %body ]
    %done = icmp sge i64 %i, %n
    br i1 %done, label %exit, label %body
body:
    %argv_i_ptr = getelementptr ptr, ptr %argv, i64 %i
    %cstr = load ptr, ptr %argv_i_ptr
    %len = call i64 @strlen(ptr %cstr)
    %str_ptr = getelementptr %Str, ptr %arr, i64 %i
    %data_field = getelementptr %Str, ptr %str_ptr, i32 0, i32 0, i32 0
    store ptr %cstr, ptr %data_field
    %len_field = getelementptr %Str, ptr %str_ptr, i32 0, i32 0, i32 1
    store i64 %len, ptr %len_field
    %next_i = add i64 %i, 1
    br label %loop
exit:
    %g_ptr = getelementptr {ptr, i64}, ptr @__os_args, i32 0, i32 0
    store ptr %arr, ptr %g_ptr
    %g_len = getelementptr {ptr, i64}, ptr @__os_args, i32 0, i32 1
    store i64 %n, ptr %g_len
    ret void
}
