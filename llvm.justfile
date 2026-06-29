# LLVM toolchain. metallc links LLVM + LLD in-process, so it is a from-source
# static LLVM, built per (goos, goarch) into .build/.

llvm_version := "22.1.3"
targets := "X86;AArch64;WebAssembly"
# ubsan (incl. the standalone runtime --sanitize=alignment links) builds
# unconditionally with the sanitizers; naming it here double-adds it.
sanitizers := "asan"
projects := "lld;compiler-rt"
flags := "-DCMAKE_BUILD_TYPE=Release -DLLVM_ENABLE_ASSERTIONS=OFF -DLLVM_ENABLE_LTO=OFF -DLLVM_ENABLE_ZSTD=OFF -DLLVM_ENABLE_ZLIB=OFF -DLLVM_ENABLE_LIBXML2=OFF -DLLVM_ENABLE_LIBEDIT=OFF -DLLVM_INCLUDE_TESTS=OFF -DLLVM_INCLUDE_EXAMPLES=OFF -DLLVM_INCLUDE_BENCHMARKS=OFF -DLLVM_INCLUDE_DOCS=OFF -DCOMPILER_RT_BUILD_BUILTINS=ON -DCOMPILER_RT_BUILD_SANITIZERS=ON -DCOMPILER_RT_BUILD_LIBFUZZER=OFF -DCOMPILER_RT_BUILD_XRAY=OFF -DCOMPILER_RT_BUILD_PROFILE=OFF -DCOMPILER_RT_BUILD_MEMPROF=OFF -DCOMPILER_RT_BUILD_ORC=OFF"

# Default: list recipes.
default:
    @just --justfile {{justfile()}} --list

# The build identity for one slice (os, arch, config), one line. CI keys LLVM
# caches on this; the build stamps it and skips when it matches.
# METALL_LLVM_BUILD_TAG, if set, is appended: a cache buster CI fills with the
# toolchain/image identity (which this string cannot otherwise see).
build-info arch="":
    #!/usr/bin/env bash
    set -euo pipefail
    goos="$(go env GOOS)"
    goarch="{{arch}}"; [ -n "$goarch" ] || goarch="$(go env GOARCH)"
    echo "goos=$goos goarch=$goarch llvm={{llvm_version}} projects={{projects}} targets={{targets}} sanitizers={{sanitizers}} flags={{flags}}${METALL_LLVM_BUILD_TAG:+ tag=$METALL_LLVM_BUILD_TAG}"

# Build the static LLVM + LLD into .build/ and regenerate the cgo flags. Skips the
# build when the installed config already matches `build-info`.
build arch="":
    #!/usr/bin/env bash
    set -euo pipefail
    cd "{{justfile_directory()}}"
    goos="$(go env GOOS)"; hostarch="$(go env GOARCH)"
    goarch="{{arch}}"; [ -n "$goarch" ] || goarch="$hostarch"
    prefix="$PWD/.build/llvm-static/${goos}-${goarch}"
    src="$PWD/.build/llvm-src"; obj="$PWD/.build/llvm-obj-${goos}-${goarch}"
    # macOS targets pin the arch (a no-op when native, x86_64 cross runs build
    # tools under Rosetta); linux cross goes via podman.
    osxarch=""
    if [ "$goos" = darwin ]; then case "$goarch" in amd64) osxarch=x86_64 ;; arm64) osxarch=arm64 ;; esac; fi
    info="$(just -f "{{justfile()}}" build-info "$goarch")"
    if [ "$(cat "$prefix/.build-info" 2>/dev/null || true)" = "$info" ]; then
        echo ">>> LLVM already built: $info"
    else
        ver="{{llvm_version}}"
        tarball="$PWD/.build/llvm-project-${ver}.src.tar.xz"
        mkdir -p "$PWD/.build"
        # Shared source, re-extracted only when the version changes. Skip the test
        # trees: never built, and their symlink fixtures fail on a bind mount.
        if [ "$(cat "$src/.version" 2>/dev/null || true)" != "$ver" ]; then
            if [ ! -f "$tarball" ]; then
                echo ">>> Downloading LLVM ${ver}"
                curl -fL --retry 5 --retry-delay 2 --retry-all-errors --progress-bar \
                    "https://github.com/llvm/llvm-project/releases/download/llvmorg-${ver}/llvm-project-${ver}.src.tar.xz" \
                    -o "$tarball.part" && mv "$tarball.part" "$tarball"
            fi
            echo ">>> Extracting LLVM ${ver}"
            rm -rf "$src"; mkdir -p "$src"
            tar -xJf "$tarball" -C "$src" --strip-components=1 --exclude='*/test/*'
            echo "$ver" > "$src/.version"
        fi
        rm -rf "$obj" "$prefix"
        echo ">>> Configuring LLVM (${goos}/${goarch})"
        cmake -G Ninja -S "$src/llvm" -B "$obj" \
            -DCMAKE_INSTALL_PREFIX="$prefix" ${osxarch:+-DCMAKE_OSX_ARCHITECTURES=$osxarch} \
            -DLLVM_ENABLE_PROJECTS="{{projects}}" \
            -DLLVM_TARGETS_TO_BUILD="{{targets}}" \
            -DCOMPILER_RT_SANITIZERS_TO_BUILD="{{sanitizers}}" \
            {{flags}}
        # Cap ninja by RAM where /proc/meminfo exists (memory-limited containers):
        # each LLVM TU needs ~2 GB and one OOM-killed cc1plus fails the build.
        echo ">>> Building LLVM (${goos}/${goarch})"
        if [ -r /proc/meminfo ]; then
            jobs="$(nproc)"
            mem_jobs=$(( $(awk '/MemTotal/{print $2}' /proc/meminfo) / 1024 / 1024 / 2 ))
            [ "$mem_jobs" -lt "$jobs" ] && jobs="$mem_jobs"
            [ "$jobs" -ge 1 ] || jobs=1
            ninja -C "$obj" -j "$jobs" install
        else
            ninja -C "$obj" install
        fi
        printf '%s' "$info" > "$prefix/.build-info"
    fi
    just -f "{{justfile()}}" genflags "$goarch"

# Generate metallc/internal/backend/cgoflags_<goos>_<goarch>.go (gitignored): the
# cgo directives that statically link LLVM + LLD into metallc.
genflags arch="":
    #!/usr/bin/env bash
    set -euo pipefail
    cd "{{justfile_directory()}}"
    goos="$(go env GOOS)"; goarch="{{arch}}"; [ -n "$goarch" ] || goarch="$(go env GOARCH)"
    prefix="$PWD/.build/llvm-static/${goos}-${goarch}"
    lc="$prefix/bin/llvm-config"
    [ -x "$lc" ] || { echo "no llvm-config; run \`just llvm build\` first" >&2; exit 1; }
    inc="$("$lc" --includedir)"; lib="$("$lc" --libdir)"
    llvm_libs="$("$lc" --libs all-targets codegen irreader option lto)"
    sys_libs="$("$lc" --system-libs)"
    # lldELF references llvm::lto::DTLTO, which llvm-config does not pull in.
    lld_libs="-llldMachO -llldWasm -llldELF -llldCommon -lLLVMDTLTO"
    # Embed build-info so Go relinks when the LLVM config changes; it does not hash
    # cgo C archives by content.
    info="$(cat "$prefix/.build-info" 2>/dev/null || true)"
    out="metallc/internal/backend/cgoflags_${goos}_${goarch}.go"
    {
        echo "// Code generated by \`just llvm genflags\`. DO NOT EDIT."
        echo "// build: ${info}"
        echo "package backend"
        echo ""
        echo "// #cgo CPPFLAGS: -I${inc}"
        echo "// #cgo CXXFLAGS: -std=c++17 -fno-exceptions -fno-rtti -D__STDC_CONSTANT_MACROS -D__STDC_FORMAT_MACROS -D__STDC_LIMIT_MACROS"
        echo "// #cgo LDFLAGS: -L${lib} ${lld_libs} ${llvm_libs} ${sys_libs}"
        echo 'import "C"'
    } > "$out"
    echo "Wrote $out"
