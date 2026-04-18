declare i32 @llvm.wasm.memory.size.i32(i32 immarg)
declare i32 @llvm.wasm.memory.grow.i32(i32 immarg, i32)

define internal i64 @__wasm_memory_size_pages() alwaysinline {
    %r = call i32 @llvm.wasm.memory.size.i32(i32 0)
    %z = zext i32 %r to i64
    ret i64 %z
}

define internal i64 @__wasm_memory_grow_pages(i64 %delta) alwaysinline {
    %d32 = trunc i64 %delta to i32
    %r = call i32 @llvm.wasm.memory.grow.i32(i32 0, i32 %d32)
    %z = zext i32 %r to i64
    ret i64 %z
}
