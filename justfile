precommit:
    just go-mod
    just fmt
    just lint
    just test
    just examples

lint:
    go tool golangci-lint run ./metallc/...
    just lint-fixme
    just lint-no-excluded-tests

# We never want to commit anything with a "fixme" comment.
lint-fixme:
    ! git --no-pager grep -i -n --untracked "fixme" -- :^.golangci.yaml :^.spell :^justfile

lint-no-excluded-tests:
    ! git --no-pager grep -i -n --untracked "!only" -- :^justfile

fmt:
    go tool golangci-lint fmt ./metallc/...

test:
    go test ./metallc/...

examples:
    #!/usr/bin/env bash
    set -euo pipefail

    for file in examples/*.met; do
        echo ">>> $file"
        go run ./metallc/... run "$file"
    done

# Compile the C runtime to LLVM IR. We use -O0 so the output is clean and
# readable; the existing `opt` pipeline in the compiler will optimize it.
# The sed pipeline strips module-level metadata (target triple, attributes,
# loop metadata, etc.) so the .ll can be embedded into the generated IR.
compile-runtime:
    #!/usr/bin/env bash
    set -euo pipefail
    $LLVM_HOME/bin/clang -x c -S -emit-llvm -O0 -fno-strict-aliasing \
        -Xclang -disable-llvm-passes \
        -o - metallc/internal/gen/arena.c_ \
    | sed -E \
        -e '/^; ModuleID/d' \
        -e '/^source_filename/d' \
        -e '/^target datalayout/d' \
        -e '/^target triple/d' \
        -e '/^attributes #/d' \
        -e '/^!/d' \
        -e '/^; Function Attrs:/d' \
        -e 's/ #[0-9]+//g' \
        -e 's/, !llvm\.loop ![0-9]+//g' \
        -e 's/^define (internal )?/define internal /' \
    > metallc/internal/gen/arena.ll

go-mod:
    cd metallc && go mod tidy
    cd tools && go mod tidy
    go work sync
