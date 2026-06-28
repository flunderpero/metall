# Release mechanism, modeled on ../cling-sync's `build.sh release`.
#
# A metall release is a self-contained bundle: the `metallc` binary (which
# statically links LLVM + LLD, so it shells out to no toolchain) plus the `lib/`
# stdlib it finds next to itself. The only runtime dependency is the host's
# system C library / SDK, which native linking fundamentally requires.
#
# `build` produces all four bundles (darwin/linux x arm64/x86_64): the macOS
# slices on this Apple Silicon host (x86_64 cross-compiled, run via Rosetta to
# verify), the linux slices via podman.
#
#   just release build    build + verify the bundle into ./dist
#   just release check    verify HEAD has a green CI build
#   just release tag      tag HEAD with the next patch version
#   just release upload   push the tag and publish ./dist
#   just release all      check, tag, build, upload

set positional-arguments

root := justfile_directory()

# Default: list recipes.
default:
    @just --justfile {{justfile()}} --list

# Build and verify all four release bundles (darwin/linux x arm64/x86_64) into ./dist.
build:
    #!/usr/bin/env bash
    set -euo pipefail
    cd "{{root}}"
    [ -z "$(git status --porcelain)" ] || { echo "working tree is not clean; release from a clean checkout" >&2; exit 1; }
    version="$(just -f release.justfile _version)"
    sha="$(git rev-parse --short HEAD)"

    echo ">>> macOS: arm64 (native) + x86_64 (cross) on this host"
    just llvm build
    just -f release.justfile _bundle darwin arm64 "$version" "$sha"
    just llvm build amd64
    just -f release.justfile _bundle darwin amd64 "$version" "$sha"

    echo ">>> linux: arm64 + x86_64 via podman"
    just linux-arm64 run llvm build
    just linux-arm64 run release _bundle linux arm64 "$version" "$sha"
    just linux-amd64 run llvm build
    just linux-amd64 run release _bundle linux amd64 "$version" "$sha"

    echo ">>> Built all release bundles:"
    ls -1 dist/*.tar.gz

# Build, verify, and tar one platform's bundle into ./dist. The build recipe
# drives this per target (darwin on the host, linux via podman); version and sha
# are passed in so this runs git-free, e.g. inside a container.
_bundle goos goarch version sha:
    #!/usr/bin/env bash
    set -euo pipefail
    cd "{{root}}"
    name="metall-{{version}}-{{goos}}-{{goarch}}"
    dist="dist/$name"
    echo ">>> Building $name"
    rm -rf "$dist" "dist/$name.tar.gz"
    mkdir -p "$dist"
    llvmver="$(.build/llvm-static/{{goos}}-{{goarch}}/bin/llvm-config --version)"
    CGO_ENABLED=1 GOOS={{goos}} GOARCH={{goarch}} go build -buildvcs=false \
        -ldflags "-s -w -X main.version={{version}} -X main.commit={{sha}} -X main.llvmVersion=$llvmver" \
        -o "$dist/metallc" ./metallc
    cp -R lib "$dist/lib"
    cp LICENSE "$dist/LICENSE"
    # Bundle compiler-rt (asan runtime + builtins) so the binary can link
    # --sanitize=address; resourceDir() finds it at <exe-dir>/lib/clang/<major>.
    cp -R ".build/llvm-static/{{goos}}-{{goarch}}/lib/clang" "$dist/lib/clang"

    # Verify self-containment from a scratch dir: the binary resolves the stdlib
    # from its own <exe-dir>/lib and links in-process. Every target runs where it
    # is built, natively, in its container, or via Rosetta (x86_64 on arm).
    echo ">>> Verifying $name"
    unset METALL_RESOURCE_DIR
    work="$(mktemp -d)"
    trap 'rm -rf "$work"' EXIT
    cp examples/hello.met "$work/hello.met"
    out="$(cd "$work" && "{{root}}/$dist/metallc" run hello.met)"
    [ -n "$out" ] || { echo "FAIL: empty output from $name" >&2; exit 1; }
    asout="$(cd "$work" && "{{root}}/$dist/metallc" run --sanitize address hello.met)"
    [ -n "$asout" ] || { echo "FAIL: --sanitize=address produced no output from $name" >&2; exit 1; }
    echo "    hello -> $out ; asan ok"

    tar -C dist -czf "dist/$name.tar.gz" "$name"
    echo ">>> Built dist/$name.tar.gz ($(du -sh "$dist/metallc" | cut -f1))"

# Print the latest release version (the highest vMAJOR.MINOR.PATCH tag).
_version:
    #!/usr/bin/env bash
    set -euo pipefail
    latest="$(git for-each-ref --sort=-v:refname --format='%(refname:short)' 'refs/tags/*' \
        | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -n1 || true)"
    [ -n "$latest" ] || { echo "no release tag found; make the first release by hand" >&2; exit 1; }
    echo "$latest"

# Verify the current HEAD has a green CI build on GitHub.
check:
    #!/usr/bin/env bash
    set -euo pipefail
    cd "{{root}}"
    command -v gh >/dev/null 2>&1 || { echo "gh (GitHub CLI) is not installed" >&2; exit 1; }
    sha="$(git rev-parse HEAD)"
    echo ">>> Checking CI status for $sha"
    state="$(gh api "repos/{owner}/{repo}/commits/$sha/check-runs" --jq '
        if (.check_runs | length) == 0 then "none"
        elif any(.check_runs[]; .status != "completed") then "pending"
        elif all(.check_runs[]; .conclusion == "success" or .conclusion == "skipped") then "success"
        else "failure"
        end')" || { echo "Could not query CI status for HEAD. Is it pushed?" >&2; exit 1; }
    case "$state" in
        success) echo "    CI is green" ;;
        none)    echo "No CI build found for HEAD. Push it and wait for CI." >&2; exit 1 ;;
        pending) echo "CI is still running for HEAD." >&2; exit 1 ;;
        *)       echo "CI is not green for HEAD ($state)." >&2; exit 1 ;;
    esac

# Tag HEAD with the next patch version (the first release is tagged by hand).
tag:
    #!/usr/bin/env bash
    set -euo pipefail
    cd "{{root}}"
    [ -z "$(git status --porcelain)" ] || { echo "working tree is not clean" >&2; exit 1; }
    latest="$(git for-each-ref --sort=-v:refname --format='%(refname:short)' 'refs/tags/*' \
        | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -n1 || true)"
    [ -n "$latest" ] || { echo "no release tag found; make the first release by hand" >&2; exit 1; }
    ver="${latest#v}"
    new="v${ver%%.*}.$(echo "$ver" | cut -d. -f2).$(( ${ver##*.} + 1 ))"
    echo ">>> Tagging $new (previous: $latest)"
    git tag "$new"

# Push the current release tag and publish ./dist as a GitHub release.
upload:
    #!/usr/bin/env bash
    set -euo pipefail
    cd "{{root}}"
    command -v gh >/dev/null 2>&1 || { echo "gh (GitHub CLI) is not installed" >&2; exit 1; }
    version="$(just -f release.justfile _version)"
    prev="$(git for-each-ref --sort=-v:refname --format='%(refname:short)' 'refs/tags/*' \
        | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | sed -n '2p' || true)"
    [ -n "$prev" ] || { echo "no previous tag; publish the first release by hand" >&2; exit 1; }
    echo ">>> Pushing git tag $version"
    git push origin "$version"
    git log --format='- %s (%h)' "$prev..$version" \
        | gh release create "$version" dist/*.tar.gz --title "$version" --notes-file -
    echo ">>> Released $version"

# check, tag, build, upload in order.
all:
    just -f release.justfile check
    just -f release.justfile tag
    just -f release.justfile build
    just -f release.justfile upload
