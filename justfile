_host_arch := `uname -m`
host_arch := if _host_arch == "x86_64" { "amd64" } else if _host_arch == "aarch64" { "arm64" } else { _host_arch }

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
    go test ./metallc/... -count 1

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
        -e 's/^(define internal) (ptr @arena_create\()/\1 noalias \2/' \
        -e 's/(@arena_create\(i64 noundef %[0-9]+\)) \{/\1 allockind("alloc") "alloc-family"="arena" allocsize(0) {/' \
        -e 's/^(define internal) (ptr @arena_alloc\()/\1 noalias \2/' \
        -e 's/(@arena_alloc\(ptr noundef %[0-9]+, i64 noundef %[0-9]+\)) \{/\1 allockind("alloc") allocsize(1) {/' \
        -e 's/(@arena_destroy\()(ptr noundef %[0-9]+\)) \{/\1ptr allocptr nocapture noundef %0) allockind("free") "alloc-family"="arena" {/' \
    > metallc/internal/gen/arena.ll

test-linux arch=host_arch:
    #!/usr/bin/env bash
    set -euo pipefail
    platform="linux/{{arch}}"
    image="metall-test-linux-{{arch}}"
    if ! podman image exists "$image"; then
        echo ">>> Building $image image ($platform)"
        podman build --platform "$platform" -t "$image" -f - <<'DOCKERFILE'
    FROM docker.io/library/ubuntu:noble
    RUN apt-get update \
        && apt-get install -y --no-install-recommends wget ca-certificates \
        && wget -qO- https://apt.llvm.org/llvm-snapshot.gpg.key > /etc/apt/trusted.gpg.d/apt.llvm.org.asc \
        && echo "deb http://apt.llvm.org/noble/ llvm-toolchain-noble-21 main" > /etc/apt/sources.list.d/llvm.list \
        && apt-get update \
        && apt-get install -y --no-install-recommends clang-21 llvm-21 libclang-rt-21-dev \
        && ln -s /usr/bin/clang-21 /usr/bin/clang \
        && ln -s /usr/bin/opt-21 /usr/bin/opt \
        && apt-get clean && rm -rf /var/lib/apt/lists/*
    ENV LLVM_HOME=/usr
    DOCKERFILE
    fi

    echo ">>> Cross-compiling tests for $platform"
    CGO_ENABLED=0 GOOS=linux GOARCH="{{arch}}" go test -c -o .build/metallc-test ./metallc/internal/

    echo ">>> Running tests in podman ($platform)"
    podman run --rm --platform "$platform" \
        -v "$PWD/.build/metallc-test:/metallc-test:ro" \
        "$image" \
        /metallc-test -test.count=1

go-mod:
    cd metallc && go mod tidy
    cd tools && go mod tidy
    go work sync
