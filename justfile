_host_arch := `uname -m`
host_arch := if _host_arch == "x86_64" { "amd64" } else if _host_arch == "aarch64" { "arm64" } else { _host_arch }

llvm_version := "22.1.3"
_llvm_os := if os() == "macos" { "macOS" } else if os() == "linux" { "Linux" } else { error("unsupported OS: " + os()) }
_llvm_arch := if arch() == "aarch64" { "ARM64" } else if arch() == "x86_64" { "X64" } else { error("unsupported arch: " + arch()) }
_llvm_name := "LLVM-" + llvm_version + "-" + _llvm_os + "-" + _llvm_arch
llvm_dir := justfile_directory() + "/.llvm/" + llvm_version

precommit:
    just go-mod
    just fmt
    just lint
    just test
    just test-go safe
    just test-go safe wasm32
    just test-go safe wasm64
    just test-lib fast
    just examples
    just examples wasm32
    just examples wasm64

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

# Run all tests.
#   opt: none, safe, fast - see `CompilerOpts`
test opt="none" target="native": (test-go opt target) (test-lib opt target)

# Run Go tests.
#   opt:    none, safe, fast - run the E2E tests with this opt-level, see `CompilerOpts`
#   target: native, wasm32, wasm64 - run the E2E tests for this target
test-go opt="none" target="native":
    {{ if opt != "" { "METALL_E2E_TEST_OPTLEVEL=" + opt } else { "" } }} {{ if target != "" { "METALL_E2E_TEST_TARGET=" + target } else { "" } }} go test ./metallc/... -count 1

# Run lib tests.
#   opt: none, safe, fast - see `CompilerOpts`
test-lib opt="none" target="native":
    #!/usr/bin/env bash
    set -uo pipefail

    failed=0
    for file in lib/*/*_test.met; do
        echo ">>> $file"
        if go run ./metallc/... run --opt {{opt}} --target {{target}} "$file" 2>&1 | tee /dev/stderr | grep -q "FAILED"; then
            failed=1
        fi
        if [ "${PIPESTATUS[0]}" -ne 0 ]; then
            failed=1
        fi
        echo ""
    done
    if [ $failed -ne 0 ]; then
        echo "At least one test FAILED"
        exit 1
    else
        echo "All tests passed"
    fi

benchmarks *args:
    just -f benchmarks/justfile {{args}}

# Run all examples/*.met (except *_macro.met).
#   target: native, wasm32, wasm64 - run the examples for this target
examples target="native":
    #!/usr/bin/env bash
    set -euo pipefail

    for file in examples/*.met; do
        if [[ "$file" == *_macro.met ]]; then continue; fi
        echo ">>> $file ({{target}})"
        go run ./metallc/... run --target {{target}} "$file"
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

# Download a prebuilt LLVM (clang + lld / wasm-ld) for the current
# platform into ./.llvm/<version>.
install-llvm:
    #!/usr/bin/env bash
    set -euo pipefail
    # Keep compiler.go's LLVMVersion and this llvm_version in sync.
    if ! grep -q 'LLVMVersion = "{{llvm_version}}"' metallc/internal/compiler/compiler.go; then
        echo "error: LLVMVersion in metallc/internal/compiler/compiler.go" >&2
        echo "       does not match llvm_version={{llvm_version}} in justfile" >&2
        exit 1
    fi
    dest="{{llvm_dir}}"
    if [ -x "$dest/bin/clang" ] && [ -x "$dest/bin/wasm-ld" ]; then
        echo "LLVM already installed at $dest"
    else
        tarball="{{_llvm_name}}.tar.xz"
        url="https://github.com/llvm/llvm-project/releases/download/llvmorg-{{llvm_version}}/$tarball"
        mkdir -p "$(dirname "$dest")"
        archive="$(dirname "$dest")/$tarball"
        if [ ! -f "$archive" ]; then
            echo "Downloading $url"
            curl -fL --progress-bar "$url" -o "$archive.part"
            mv "$archive.part" "$archive"
        fi
        echo "Extracting $archive"
        tar -xJf "$archive" -C "$(dirname "$dest")"
        rm -f "$archive"
        # Flatten the platform-specific dir so the path stays just
        # `.llvm/<version>/` regardless of host.
        rm -rf "$dest"
        mv "$(dirname "$dest")/{{_llvm_name}}" "$dest"
    fi

# Print the LLVM path `install-llvm` provisions (whether or not it exists yet).
llvm-path:
    @echo {{llvm_dir}}

go-mod:
    cd metallc && go mod tidy
    cd tools && go mod tidy
    go work sync
