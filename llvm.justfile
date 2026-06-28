# LLVM toolchain. metallc links LLVM + LLD in-process, so the toolchain is a
# from-source static LLVM. Driven from the main justfile via `just llvm <recipe>`.

llvm_version := "22.1.3"

# Default: list recipes.
default:
    @just --justfile {{justfile()}} --list

# Build the static LLVM + LLD (all targets, so metallc can cross-compile to any
# of them; Release, no LTO, deps off, plus compiler-rt for the asan runtime),
# then regenerate the cgo flags. Slow on a cold run (~40 min); reruns just
# refresh the flags.
build arch="":
    #!/usr/bin/env bash
    set -euo pipefail
    cd "{{justfile_directory()}}"
    goos="$(go env GOOS)"; hostarch="$(go env GOARCH)"
    goarch="{{arch}}"; [ -n "$goarch" ] || goarch="$hostarch"
    prefix="$PWD/.build/llvm-static/${goos}-${goarch}"
    # The build (cmake cache) dir is per-platform; the source is shared.
    src="$PWD/.build/llvm-src"; obj="$PWD/.build/llvm-obj-${goos}-${goarch}"
    # Cross to a non-host macOS arch builds that slice via the universal SDK; its
    # x86_64 build tools (tblgen) run under Rosetta. (Linux cross goes via podman.)
    osxarch=""
    if [ "$goos" = darwin ] && [ "$goarch" != "$hostarch" ]; then
        case "$goarch" in
            amd64) osxarch="-DCMAKE_OSX_ARCHITECTURES=x86_64" ;;
            arm64) osxarch="-DCMAKE_OSX_ARCHITECTURES=arm64" ;;
        esac
    fi
    # Rebuild unless the install already has the full target set (RISCV stands
    # in for `all`), so `just llvm build` also converges an older partial build.
    if ! { [ -x "$prefix/bin/llvm-config" ] && "$prefix/bin/llvm-config" --targets-built 2>/dev/null | grep -qw RISCV; }; then
        for tool in cmake ninja; do
            command -v "$tool" >/dev/null 2>&1 || { echo "error: '$tool' is required for a cold LLVM build (macOS: brew install cmake ninja; Debian/Ubuntu: apt install cmake ninja-build)" >&2; exit 1; }
        done
        ver="{{llvm_version}}"
        url="https://github.com/llvm/llvm-project/releases/download/llvmorg-${ver}/llvm-project-${ver}.src.tar.xz"
        tarball="$PWD/.build/llvm-project-${ver}.src.tar.xz"
        mkdir -p "$PWD/.build"
        if [ ! -f "$tarball" ]; then
            echo ">>> downloading LLVM ${ver} source"
            curl -fL --retry 5 --retry-delay 2 --retry-all-errors --progress-bar "$url" -o "$tarball.part" && mv "$tarball.part" "$tarball"
        fi
        if [ ! -d "$src" ]; then
            echo ">>> extracting source"
            mkdir -p "$src" && tar -xJf "$tarball" -C "$src" --strip-components=1
        fi
        # Configure unconditionally (cheap when unchanged) so a target-set change
        # reaches an existing build dir. compiler-rt (built with the host
        # compiler) supplies the asan runtime that --sanitize=address links.
        echo ">>> configuring (Release, no LTO, all targets, lld + compiler-rt)"
        cmake -G Ninja -S "$src/llvm" -B "$obj" \
            -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX="$prefix" $osxarch \
            -DLLVM_ENABLE_PROJECTS="lld;compiler-rt" -DLLVM_TARGETS_TO_BUILD=all \
            -DCOMPILER_RT_BUILD_BUILTINS=ON -DCOMPILER_RT_BUILD_SANITIZERS=ON \
            -DCOMPILER_RT_BUILD_LIBFUZZER=OFF -DCOMPILER_RT_BUILD_XRAY=OFF \
            -DCOMPILER_RT_BUILD_PROFILE=OFF -DCOMPILER_RT_BUILD_MEMPROF=OFF \
            -DCOMPILER_RT_BUILD_ORC=OFF \
            -DLLVM_ENABLE_ASSERTIONS=OFF -DLLVM_ENABLE_LTO=OFF \
            -DLLVM_ENABLE_ZSTD=OFF -DLLVM_ENABLE_ZLIB=OFF -DLLVM_ENABLE_LIBXML2=OFF \
            -DLLVM_ENABLE_LIBEDIT=OFF -DLLVM_INCLUDE_TESTS=OFF \
            -DLLVM_INCLUDE_EXAMPLES=OFF -DLLVM_INCLUDE_BENCHMARKS=OFF -DLLVM_INCLUDE_DOCS=OFF
        # Cap parallelism by RAM: each LLVM TU needs ~1.5 GB, and a single
        # OOM-killed cc1plus fails the whole build (common in memory-limited
        # containers). Hosts with plenty of RAM still build at full width.
        if [ -r /proc/meminfo ]; then
            mem_gb=$(( $(awk '/MemTotal/{print $2}' /proc/meminfo) / 1024 / 1024 ))
            cpus="$(nproc)"
        else
            mem_gb=$(( $(sysctl -n hw.memsize) / 1024 / 1024 / 1024 ))
            cpus="$(sysctl -n hw.ncpu)"
        fi
        jobs=$(( mem_gb / 2 )); [ "$jobs" -lt 1 ] && jobs=1
        [ "$jobs" -gt "$cpus" ] && jobs="$cpus"
        echo ">>> building + installing LLVM with -j${jobs} (slow on first run)"
        ninja -C "$obj" -j "$jobs" install
    fi
    just -f "{{justfile()}}" genflags "$goarch"

# Generate metallc/internal/backend/cgoflags_<goos>_<goarch>.go (gitignored), the
# cgo directives that statically link the in-process LLVM + LLD into metallc.
genflags arch="":
    #!/usr/bin/env bash
    set -euo pipefail
    cd "{{justfile_directory()}}"
    goos="$(go env GOOS)"; goarch="{{arch}}"; [ -n "$goarch" ] || goarch="$(go env GOARCH)"
    lc="$PWD/.build/llvm-static/${goos}-${goarch}/bin/llvm-config"
    [ -x "$lc" ] || { echo "no llvm-config; run \`just llvm build\` first" >&2; exit 1; }
    inc="$("$lc" --includedir)"; lib="$("$lc" --libdir)"
    llvm_libs="$("$lc" --libs all-targets codegen irreader option lto)"
    sys_libs="$("$lc" --system-libs)"
    # lldELF references llvm::lto::DTLTO, in its own component llvm-config does
    # not pull in transitively.
    lld_libs="-llldMachO -llldWasm -llldELF -llldCommon -lLLVMDTLTO"
    out="metallc/internal/backend/cgoflags_${goos}_${goarch}.go"
    {
        echo "// Code generated by \`just llvm genflags\`. DO NOT EDIT."
        echo "package backend"
        echo ""
        echo "// #cgo CPPFLAGS: -I${inc}"
        echo "// #cgo CXXFLAGS: -std=c++17 -fno-exceptions -fno-rtti -D__STDC_CONSTANT_MACROS -D__STDC_FORMAT_MACROS -D__STDC_LIMIT_MACROS"
        echo "// #cgo LDFLAGS: -L${lib} ${lld_libs} ${llvm_libs} ${sys_libs}"
        echo 'import "C"'
    } > "$out"
    echo "wrote $out"
