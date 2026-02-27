
%struct.Arena = type { ptr, i64 }
%struct.Page = type { ptr, i64, i64, [0 x i8] }

define internal noalias ptr @arena_create(i64 noundef %0) allockind("alloc") "alloc-family"="arena" allocsize(0) {
  %2 = alloca ptr, align 8
  %3 = alloca i64, align 8
  %4 = alloca ptr, align 8
  %5 = alloca ptr, align 8
  store i64 %0, ptr %3, align 8
  %6 = load i64, ptr %3, align 8
  %7 = icmp eq i64 %6, 0
  br i1 %7, label %8, label %9

8:                                                ; preds = %1
  store i64 4096, ptr %3, align 8
  br label %9

9:                                                ; preds = %8, %1
  %10 = call ptr @malloc(i64 noundef 16)
  store ptr %10, ptr %4, align 8
  %11 = load ptr, ptr %4, align 8
  %12 = icmp ne ptr %11, null
  br i1 %12, label %14, label %13

13:                                               ; preds = %9
  store ptr null, ptr %2, align 8
  br label %29

14:                                               ; preds = %9
  %15 = load i64, ptr %3, align 8
  %16 = call ptr @page_create(i64 noundef %15, ptr noundef null)
  store ptr %16, ptr %5, align 8
  %17 = load ptr, ptr %5, align 8
  %18 = icmp ne ptr %17, null
  br i1 %18, label %21, label %19

19:                                               ; preds = %14
  %20 = load ptr, ptr %4, align 8
  call void @free(ptr noundef %20)
  store ptr null, ptr %2, align 8
  br label %29

21:                                               ; preds = %14
  %22 = load ptr, ptr %5, align 8
  %23 = load ptr, ptr %4, align 8
  %24 = getelementptr inbounds nuw %struct.Arena, ptr %23, i32 0, i32 0
  store ptr %22, ptr %24, align 8
  %25 = load i64, ptr %3, align 8
  %26 = load ptr, ptr %4, align 8
  %27 = getelementptr inbounds nuw %struct.Arena, ptr %26, i32 0, i32 1
  store i64 %25, ptr %27, align 8
  %28 = load ptr, ptr %4, align 8
  store ptr %28, ptr %2, align 8
  br label %29

29:                                               ; preds = %21, %19, %13
  %30 = load ptr, ptr %2, align 8
  ret ptr %30
}

declare ptr @malloc(i64 noundef)

define internal ptr @page_create(i64 noundef %0, ptr noundef %1) {
  %3 = alloca ptr, align 8
  %4 = alloca i64, align 8
  %5 = alloca ptr, align 8
  %6 = alloca ptr, align 8
  store i64 %0, ptr %4, align 8
  store ptr %1, ptr %5, align 8
  %7 = load i64, ptr %4, align 8
  %8 = add i64 24, %7
  %9 = call ptr @malloc(i64 noundef %8)
  store ptr %9, ptr %6, align 8
  %10 = load ptr, ptr %6, align 8
  %11 = icmp ne ptr %10, null
  br i1 %11, label %13, label %12

12:                                               ; preds = %2
  store ptr null, ptr %3, align 8
  br label %23

13:                                               ; preds = %2
  %14 = load ptr, ptr %5, align 8
  %15 = load ptr, ptr %6, align 8
  %16 = getelementptr inbounds nuw %struct.Page, ptr %15, i32 0, i32 0
  store ptr %14, ptr %16, align 8
  %17 = load i64, ptr %4, align 8
  %18 = load ptr, ptr %6, align 8
  %19 = getelementptr inbounds nuw %struct.Page, ptr %18, i32 0, i32 1
  store i64 %17, ptr %19, align 8
  %20 = load ptr, ptr %6, align 8
  %21 = getelementptr inbounds nuw %struct.Page, ptr %20, i32 0, i32 2
  store i64 0, ptr %21, align 8
  %22 = load ptr, ptr %6, align 8
  store ptr %22, ptr %3, align 8
  br label %23

23:                                               ; preds = %13, %12
  %24 = load ptr, ptr %3, align 8
  ret ptr %24
}

declare void @free(ptr noundef)

define internal noalias ptr @arena_alloc(ptr noundef %0, i64 noundef %1) allockind("alloc") allocsize(1) {
  %3 = alloca ptr, align 8
  %4 = alloca ptr, align 8
  %5 = alloca i64, align 8
  %6 = alloca ptr, align 8
  %7 = alloca i64, align 8
  %8 = alloca ptr, align 8
  %9 = alloca ptr, align 8
  store ptr %0, ptr %4, align 8
  store i64 %1, ptr %5, align 8
  %10 = load ptr, ptr %4, align 8
  %11 = getelementptr inbounds nuw %struct.Arena, ptr %10, i32 0, i32 0
  %12 = load ptr, ptr %11, align 8
  store ptr %12, ptr %6, align 8
  %13 = load ptr, ptr %6, align 8
  %14 = getelementptr inbounds nuw %struct.Page, ptr %13, i32 0, i32 2
  %15 = load i64, ptr %14, align 8
  %16 = load i64, ptr %5, align 8
  %17 = add i64 %15, %16
  %18 = load ptr, ptr %6, align 8
  %19 = getelementptr inbounds nuw %struct.Page, ptr %18, i32 0, i32 1
  %20 = load i64, ptr %19, align 8
  %21 = icmp ugt i64 %17, %20
  br i1 %21, label %22, label %43

22:                                               ; preds = %2
  %23 = load ptr, ptr %4, align 8
  %24 = getelementptr inbounds nuw %struct.Arena, ptr %23, i32 0, i32 1
  %25 = load i64, ptr %24, align 8
  store i64 %25, ptr %7, align 8
  %26 = load i64, ptr %5, align 8
  %27 = load i64, ptr %7, align 8
  %28 = icmp ugt i64 %26, %27
  br i1 %28, label %29, label %31

29:                                               ; preds = %22
  %30 = load i64, ptr %5, align 8
  store i64 %30, ptr %7, align 8
  br label %31

31:                                               ; preds = %29, %22
  %32 = load i64, ptr %7, align 8
  %33 = load ptr, ptr %6, align 8
  %34 = call ptr @page_create(i64 noundef %32, ptr noundef %33)
  store ptr %34, ptr %8, align 8
  %35 = load ptr, ptr %8, align 8
  %36 = icmp ne ptr %35, null
  br i1 %36, label %38, label %37

37:                                               ; preds = %31
  store ptr null, ptr %3, align 8
  br label %57

38:                                               ; preds = %31
  %39 = load ptr, ptr %8, align 8
  %40 = load ptr, ptr %4, align 8
  %41 = getelementptr inbounds nuw %struct.Arena, ptr %40, i32 0, i32 0
  store ptr %39, ptr %41, align 8
  %42 = load ptr, ptr %8, align 8
  store ptr %42, ptr %6, align 8
  br label %43

43:                                               ; preds = %38, %2
  %44 = load ptr, ptr %6, align 8
  %45 = getelementptr inbounds nuw %struct.Page, ptr %44, i32 0, i32 3
  %46 = getelementptr inbounds [0 x i8], ptr %45, i64 0, i64 0
  %47 = load ptr, ptr %6, align 8
  %48 = getelementptr inbounds nuw %struct.Page, ptr %47, i32 0, i32 2
  %49 = load i64, ptr %48, align 8
  %50 = getelementptr inbounds nuw i8, ptr %46, i64 %49
  store ptr %50, ptr %9, align 8
  %51 = load i64, ptr %5, align 8
  %52 = load ptr, ptr %6, align 8
  %53 = getelementptr inbounds nuw %struct.Page, ptr %52, i32 0, i32 2
  %54 = load i64, ptr %53, align 8
  %55 = add i64 %54, %51
  store i64 %55, ptr %53, align 8
  %56 = load ptr, ptr %9, align 8
  store ptr %56, ptr %3, align 8
  br label %57

57:                                               ; preds = %43, %37
  %58 = load ptr, ptr %3, align 8
  ret ptr %58
}

define internal void @arena_destroy(ptr allocptr nocapture noundef %0) allockind("free") "alloc-family"="arena" {
  %2 = alloca ptr, align 8
  %3 = alloca ptr, align 8
  %4 = alloca ptr, align 8
  store ptr %0, ptr %2, align 8
  %5 = load ptr, ptr %2, align 8
  %6 = getelementptr inbounds nuw %struct.Arena, ptr %5, i32 0, i32 0
  %7 = load ptr, ptr %6, align 8
  store ptr %7, ptr %3, align 8
  br label %8

8:                                                ; preds = %11, %1
  %9 = load ptr, ptr %3, align 8
  %10 = icmp ne ptr %9, null
  br i1 %10, label %11, label %17

11:                                               ; preds = %8
  %12 = load ptr, ptr %3, align 8
  %13 = getelementptr inbounds nuw %struct.Page, ptr %12, i32 0, i32 0
  %14 = load ptr, ptr %13, align 8
  store ptr %14, ptr %4, align 8
  %15 = load ptr, ptr %3, align 8
  call void @free(ptr noundef %15)
  %16 = load ptr, ptr %4, align 8
  store ptr %16, ptr %3, align 8
  br label %8

17:                                               ; preds = %8
  %18 = load ptr, ptr %2, align 8
  call void @free(ptr noundef %18)
  ret void
}



