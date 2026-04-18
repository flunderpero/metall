declare i64 @llvm.wasm.memory.size.i64(i32 immarg)
declare i64 @llvm.wasm.memory.grow.i64(i32 immarg, i64)

define internal i64 @__wasm_memory_size_pages() alwaysinline {
    %r = call i64 @llvm.wasm.memory.size.i64(i32 0)
    ret i64 %r
}

define internal i64 @__wasm_memory_grow_pages(i64 %delta) alwaysinline {
    %r = call i64 @llvm.wasm.memory.grow.i64(i32 0, i64 %delta)
    ret i64 %r
}
