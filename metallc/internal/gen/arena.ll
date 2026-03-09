; Hand-written arena allocator.
;
; Arena: stack-allocated, with an inline first page on the stack.
; Overflow pages are heap-allocated with doubling sizes.
;
; Type definitions (%struct.Arena, %struct.FirstPage, %struct.PageHeader)
; are emitted by the compiler in the IR header, before any function that
; uses alloca %struct.Arena.

define internal void @arena_create(ptr nocapture noundef %a) nounwind willreturn memory(argmem: write) {
  ${arena.on_create}
  ; a->page_size = page_min_size
  %ps = getelementptr inbounds %struct.Arena, ptr %a, i32 0, i32 0
  store i64 ${arena.page_min_size}, ptr %ps, align 8

  ; first = &a->first
  %first = getelementptr inbounds %struct.Arena, ptr %a, i32 0, i32 2

  ; a->current = first
  %cur = getelementptr inbounds %struct.Arena, ptr %a, i32 0, i32 1
  store ptr %first, ptr %cur, align 8

  ; first->header.next = null
  %next = getelementptr inbounds %struct.PageHeader, ptr %first, i32 0, i32 0
  store ptr null, ptr %next, align 8

  ; data = &first->data[0]
  %data = getelementptr inbounds %struct.FirstPage, ptr %first, i32 0, i32 1, i32 0

  ; first->header.cursor = data
  %cursor = getelementptr inbounds %struct.PageHeader, ptr %first, i32 0, i32 1
  store ptr %data, ptr %cursor, align 8

  ; first->header.end = data + stack_buf_size
  %end = getelementptr inbounds i8, ptr %data, i64 ${arena.stack_buf_size}
  %endp = getelementptr inbounds %struct.PageHeader, ptr %first, i32 0, i32 2
  store ptr %end, ptr %endp, align 8

  ret void
}

; --- arena_alloc(ptr %arena, i64 %size) -> ptr ---
define internal noalias ptr @arena_alloc(ptr %a, i64 %size) allockind("alloc") allocsize(1) "alloc-family"="arena" {
  ; p = a->current
  %cur = getelementptr inbounds %struct.Arena, ptr %a, i32 0, i32 1
  %p = load ptr, ptr %cur, align 8

  ; cursor = p->cursor
  %cursor_ptr = getelementptr inbounds %struct.PageHeader, ptr %p, i32 0, i32 1
  %cursor = load ptr, ptr %cursor_ptr, align 8

  ; new_cursor = cursor + size
  %new_cursor = getelementptr inbounds i8, ptr %cursor, i64 %size

  ; end = p->end
  %end_ptr = getelementptr inbounds %struct.PageHeader, ptr %p, i32 0, i32 2
  %end = load ptr, ptr %end_ptr, align 8

  ; if (new_cursor > end) goto slow
  %overflow = icmp ugt ptr %new_cursor, %end
  br i1 %overflow, label %slow, label %fast

fast:
  ${arena.on_alloc}
  store ptr %new_cursor, ptr %cursor_ptr, align 8
  ret ptr %cursor

slow:
  ; cap = a->page_size
  %cap_ptr = getelementptr inbounds %struct.Arena, ptr %a, i32 0, i32 0
  %cap = load i64, ptr %cap_ptr, align 8

  ; alloc_cap = max(size, cap)
  %size_gt = icmp ugt i64 %size, %cap
  %alloc_cap = select i1 %size_gt, i64 %size, i64 %cap

  ; malloc(page_header_size + alloc_cap)
  %total = add i64 ${arena.page_header_size}, %alloc_cap
  %mem = call ptr @malloc(i64 %total)
  %null_check = icmp eq ptr %mem, null
  br i1 %null_check, label %oom, label %init_page

oom:
  ret ptr null

init_page:
  ; np->next = null
  %np_next = getelementptr inbounds %struct.PageHeader, ptr %mem, i32 0, i32 0
  store ptr null, ptr %np_next, align 8

  ; np_data = mem + page_header_size  (just past the header)
  %np_data = getelementptr inbounds i8, ptr %mem, i64 ${arena.page_header_size}

  ; np->cursor = np_data
  %np_cursor = getelementptr inbounds %struct.PageHeader, ptr %mem, i32 0, i32 1
  store ptr %np_data, ptr %np_cursor, align 8

  ; np->end = np_data + alloc_cap
  %np_end = getelementptr inbounds i8, ptr %np_data, i64 %alloc_cap
  %np_endp = getelementptr inbounds %struct.PageHeader, ptr %mem, i32 0, i32 2
  store ptr %np_end, ptr %np_endp, align 8

  ; prev->next = np
  %p_next = getelementptr inbounds %struct.PageHeader, ptr %p, i32 0, i32 0
  store ptr %mem, ptr %p_next, align 8

  ; a->current = np
  store ptr %mem, ptr %cur, align 8

  ; Double page_size, clamped to page_max_size
  %ps = load i64, ptr %cap_ptr, align 8
  %doubled = mul i64 %ps, 2
  %clamped = call i64 @llvm.umin.i64(i64 %doubled, i64 ${arena.page_max_size})
  store i64 %clamped, ptr %cap_ptr, align 8

  ${arena.on_page_alloc}
  ${arena.on_alloc}

  ; Bump cursor and return
  %result = load ptr, ptr %np_cursor, align 8
  %bumped = getelementptr inbounds i8, ptr %result, i64 %size
  store ptr %bumped, ptr %np_cursor, align 8
  ret ptr %result
}

; --- arena_destroy(ptr %arena) ---
define internal void @arena_destroy(ptr allocptr nocapture %a) nosync nounwind willreturn memory(argmem: readwrite) {
  ${arena.on_destroy}
  ; p = a->first.header.next
  %first = getelementptr inbounds %struct.Arena, ptr %a, i32 0, i32 2
  %first_next = getelementptr inbounds %struct.PageHeader, ptr %first, i32 0, i32 0
  %p_init = load ptr, ptr %first_next, align 8
  br label %loop

loop:
  %p = phi ptr [ %p_init, %0 ], [ %next, %free_page ]
  %done = icmp eq ptr %p, null
  br i1 %done, label %exit, label %free_page

free_page:
  %next_ptr = getelementptr inbounds %struct.PageHeader, ptr %p, i32 0, i32 0
  %next = load ptr, ptr %next_ptr, align 8
  call void @free(ptr %p)
  br label %loop

exit:
  ret void
}

declare ptr @malloc(i64)
declare void @free(ptr)
declare i64 @llvm.umin.i64(i64, i64)
