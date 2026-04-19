_host_arch := `uname -m`
host_arch := if _host_arch == "x86_64" { "amd64" } else if _host_arch == "aarch64" { "arm64" } else { _host_arch }

llvm_version := "22.1.3"
host_os := os()
host_arch_llvm := arch()
llvm_root := justfile_directory() + "/.llvm/" + llvm_version

precommit:
    just go-mod
    just fmt
    just lint
    just test safe
    just test safe wasm32 
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
        case "{{target}}:$(basename "$file")" in
            wasm*:fs_test.met|wasm*:os_test.met|wasm*:thread_test.met)
                echo ">>> $file (skipped on {{target}})"
                echo ""
                continue
                ;;
        esac
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

metallc *args:
    go run ./metallc/... {{args}}

linux-arm64 *args:
    METALL_ARCH=arm64 just -f justfile.linux {{args}}

linux-amd64 *args:
    METALL_ARCH=amd64 just -f justfile.linux {{args}}

install-llvm os=host_os arch=host_arch_llvm:
    #!/usr/bin/env bash
    set -euo pipefail
    # Keep compiler.go's LLVMVersion and this llvm_version in sync.
    if ! grep -q 'LLVMVersion = "{{llvm_version}}"' metallc/internal/compiler/compiler.go; then
        echo "error: LLVMVersion in metallc/internal/compiler/compiler.go" >&2
        echo "       does not match llvm_version={{llvm_version}} in justfile" >&2
        exit 1
    fi
    # Map `os`/`arch` args to Go's GOOS/GOARCH (local dir) and to the
    # upstream LLVM release naming (tarball).
    case "{{os}}" in
        macos|darwin)  goos="darwin";  release_os="macOS" ;;
        linux)         goos="linux";   release_os="Linux" ;;
        *)             echo "unsupported os: {{os}}" >&2; exit 1 ;;
    esac
    case "{{arch}}" in
        aarch64|arm64) goarch="arm64"; release_arch="ARM64" ;;
        x86_64|amd64)  goarch="amd64"; release_arch="X64" ;;
        *)             echo "unsupported arch: {{arch}}" >&2; exit 1 ;;
    esac
    release_name="LLVM-{{llvm_version}}-${release_os}-${release_arch}"
    dest="{{llvm_root}}/${goos}-${goarch}"
    if [ -x "$dest/bin/clang" ] && [ -x "$dest/bin/wasm-ld" ]; then
        echo "LLVM already installed at $dest"
    else
        tarball="${release_name}.tar.xz"
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
        rm -rf "$dest"
        mv "$(dirname "$dest")/${release_name}" "$dest"
    fi

go-mod:
    cd metallc && go mod tidy
    cd tools && go mod tidy
    go work sync
