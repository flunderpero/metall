precommit:
    just llvm build
    just go-mod
    just fmt
    just lint
    just test safe
    just test safe wasm32
    just examples
    just benchmarks compile-check

lint:
    go tool golangci-lint run ./metallc/...
    just lint-fixme
    just lint-no-excluded-tests

# We never want to commit anything with a "fixme" comment.
lint-fixme:
    ! git --no-pager grep -i -n --untracked "fixme" -- :^.golangci.yaml :^.spell :^justfile :^AGENTS.md ':(exclude,glob)**/vendor/**'

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
    out=$(mktemp)
    trap 'rm -f "$out"' EXIT
    for file in lib/*/*_test.met; do
        case "{{target}}:$(basename "$file")" in
            wasm*:fs_test.met|wasm*:os_test.met|wasm*:thread_test.met)
                echo ">>> $file (skipped on {{target}})"
                echo ""
                continue
                ;;
        esac
        echo ">>> $file"
        # Capture to a file, don't pipe into `grep -q FAILED`: `grep -q` exits on
        # the first match and SIGPIPEs the producer, so under `pipefail` the
        # pipeline status hid the match and a printed FAILED could still report
        # "All tests passed".
        go run ./metallc/... run --opt {{opt}} --target {{target}} "$file" 2>&1 | tee "$out"
        if [ "${PIPESTATUS[0]}" -ne 0 ] || grep -q "FAILED" "$out"; then
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
#   opt:    none, safe, fast - see `CompilerOpts`
#   target: native, wasm32, wasm64 - run the examples for this target
examples opt="none" target="native":
    #!/usr/bin/env bash
    set -euo pipefail

    # Export-flavored examples have a runner paired with the .met:
    #   <name>.c   -> native: compile to .o + .h, link against the C runner.
    #   <name>.mts -> wasm:   compile to .wasm + .ts, run the TS runner via node.
    # All artifacts (.o, .h, .wasm, .ts) land next to the source in examples/.
    for file in examples/*.met; do
        if [[ "$file" == *_macro.met ]]; then continue; fi
        base="${file%.met}"
        c_file="$base.c"
        mts_file="$base.mts"
        has_runner=false
        if [ "{{target}}" = "native" ] && [ -f "$c_file" ]; then
            echo ">>> $file + $c_file ({{target}})"
            obj="$base.o"
            header="$base.h"
            bin="$(mktemp -t "$(basename "$base").XXXXXX")"
            trap 'rm -f "$bin"' EXIT
            go run ./metallc/... build --opt {{opt}} -c -emit-header-file -o "$obj" "$file"
            # `export` is the user-side workflow: link the emitted .o with the
            # system C compiler, exactly as a user of the library would.
            cc_args=(-I "$(dirname "$header")" -o "$bin" "$c_file" "$obj")
            if [ "$(uname -s)" = "Darwin" ]; then
                cc_args+=(-isysroot "$(xcrun --show-sdk-path)")
            fi
            cc "${cc_args[@]}"
            "$bin"
            rm -f "$bin"
            trap - EXIT
            has_runner=true
        elif [ "{{target}}" != "native" ] && [ -f "$mts_file" ]; then
            echo ">>> $file + $mts_file ({{target}})"
            wasm="$base.wasm"
            go run ./metallc/... build --opt {{opt}} --target {{target}} --emit-typescript -o "$wasm" "$file"
            (cd "$(dirname "$mts_file")" && node "$(basename "$mts_file")")
            has_runner=true
        fi
        if $has_runner; then
            continue
        fi
        if [ -f "$c_file" ] || [ -f "$mts_file" ]; then
            echo ">>> $file ({{target}}) (skipped: no runner for target)"
            continue
        fi
        echo ">>> $file ({{target}})"
        go run ./metallc/... run --opt {{opt}} --target {{target}} "$file"
    done

metallc *args:
    go run ./metallc/... {{args}}

linux-arm64 *args:
    METALL_ARCH=arm64 just -f justfile.linux {{args}}

linux-amd64 *args:
    METALL_ARCH=amd64 just -f justfile.linux {{args}}


# Run an LLVM toolchain recipe (build, genflags) from llvm.justfile.
llvm *args:
    just -f llvm.justfile {{args}}

go-mod:
    cd metallc && go mod tidy
    cd tools && go mod tidy
    go work sync
