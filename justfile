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
    ! git --no-pager grep -i -n --untracked "fixme" -- :^.golangci.yaml :^.spell :^justfile :^AGENTS.md

lint-no-excluded-tests:
    ! git --no-pager grep -i -n --untracked "!only" -- :^justfile :^AGENTS.md :^metallc/internal/test/

fmt:
    go tool golangci-lint fmt ./metallc/...

test:
    go test ./metallc/... -count 1

examples:
    #!/usr/bin/env bash
    set -euo pipefail

    for file in examples/*.met; do
        if [[ "$file" != *_macro.met ]]; then
            echo ">>> $file"
            go run ./metallc/... run "$file"
        fi
    done

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
    mkdir -p .build/tests
    rm -f .build/tests/*
    for pkg in $(go list ./metallc/...); do
        name=$(echo "$pkg" | tr '/' '-')
        CGO_ENABLED=0 GOOS=linux GOARCH="{{arch}}" go test -c -o ".build/tests/${name}" "$pkg" 2>/dev/null || true
    done

    echo ">>> Running tests in podman ($platform)"
    podman run --rm --platform "$platform" \
        -v "$PWD/.build/tests:/tests:ro" \
        "$image" \
        sh -c 'for t in /tests/*; do echo "=== ${t##*/}"; "$t" -test.count=1 || exit 1; done'

go-mod:
    cd metallc && go mod tidy
    cd tools && go mod tidy
    go work sync
