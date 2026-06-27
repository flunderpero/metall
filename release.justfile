# Release mechanism, modeled on ../cling-sync's `build.sh release`.
#
# A metall release is a self-contained bundle: the `metallc` binary (which
# statically links LLVM + LLD, so it shells out to no toolchain) plus the `lib/`
# stdlib it finds next to itself. The only runtime dependency is the host's
# system C library / SDK, which native linking fundamentally requires.
#
# For now this builds for the current OS/arch only.
#
#   just -f release.justfile build    build + verify the bundle into ./dist
#   just -f release.justfile check    verify HEAD has a green CI build
#   just -f release.justfile tag      tag HEAD with the next patch version
#   just -f release.justfile upload   push the tag and publish ./dist
#   just -f release.justfile all      check, tag, build, upload

set positional-arguments

root := justfile_directory()

# Default: list recipes.
default:
    @just --justfile {{justfile()}} --list

# Build the self-contained release bundle for the current OS/arch into ./dist,
# then verify it compiles and runs a program using only the bundled binary+lib.
build:
    #!/usr/bin/env bash
    set -euo pipefail
    cd "{{root}}"

    just llvm build

    version="$(just -f release.justfile _version)"
    os="$(uname -s | tr '[:upper:]' '[:lower:]')"
    case "$(uname -m)" in
        arm64|aarch64) arch="arm64" ;;
        x86_64|amd64)  arch="amd64" ;;
        *) echo "unsupported arch: $(uname -m)" >&2; exit 1 ;;
    esac
    name="metall-$version-$os-$arch"
    dist="dist/$name"
    echo ">>> Building $name"
    rm -rf "$dist" "dist/$name.tar.gz"
    mkdir -p "$dist"

    # Stamp the version at build time (never committed): the release tag, the git
    # short sha (+ -dirty if the tree is not clean), and the actual linked LLVM
    # version straight from llvm-config.
    sha="$(git rev-parse --short HEAD)$([ -n "$(git status --porcelain)" ] && echo -dirty)"
    llvmver="$(.build/llvm-static/$os-$arch/bin/llvm-config --version)"
    CGO_ENABLED=1 go build \
        -ldflags "-s -w -X main.version=$version -X main.commit=$sha -X main.llvmVersion=$llvmver" \
        -o "$dist/metallc" ./metallc
    cp -R lib "$dist/lib"
    cp LICENSE "$dist/LICENSE"

    # Bundle compiler-rt (the asan runtime) so a released metallc can link
    # --sanitize=address; resourceDir() finds it at <exe-dir>/lib/clang/<major>.
    cp -R ".build/llvm-static/$os-$arch/lib/clang" "$dist/lib/clang"

    # Verify: compile + run an example from a scratch dir (no repo `lib` on the
    # cwd), so the binary must resolve the stdlib from its own <exe-dir>/lib and
    # link in-process. This is the real self-containment check.
    echo ">>> Verifying the bundle is self-contained"
    # Drop any resource-dir override so the verify can only resolve the bundled
    # compiler-rt, not a dev shell's external .build tree.
    unset METALL_RESOURCE_DIR
    work="$(mktemp -d)"
    trap 'rm -rf "$work"' EXIT
    cp examples/hello.met "$work/hello.met"
    out="$(cd "$work" && "{{root}}/$dist/metallc" run hello.met)"
    echo "    metallc run hello.met -> $out"
    [ -n "$out" ] || { echo "FAIL: empty output from bundled metallc" >&2; exit 1; }

    # asan links the bundled compiler-rt, so verify it works from the bundle too.
    asout="$(cd "$work" && "{{root}}/$dist/metallc" run --sanitize address hello.met)"
    echo "    metallc run --sanitize address hello.met -> $asout"
    [ -n "$asout" ] || { echo "FAIL: --sanitize=address produced no output" >&2; exit 1; }

    tar -C dist -czf "dist/$name.tar.gz" "$name"
    size="$(du -sh "$dist/metallc" | cut -f1)"
    echo ">>> Built dist/$name.tar.gz (metallc: $size)"

# Print the release version: the latest vMAJOR.MINOR.PATCH tag, or v0.0.0-dev.
_version:
    #!/usr/bin/env bash
    set -euo pipefail
    latest="$(git for-each-ref --sort=-v:refname --format='%(refname:short)' 'refs/tags/*' \
        | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -n1 || true)"
    echo "${latest:-v0.0.0-dev}"

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

# Tag HEAD with the next patch version (latest + 1), or v0.0.1 if there are none.
tag:
    #!/usr/bin/env bash
    set -euo pipefail
    cd "{{root}}"
    [ -z "$(git status --porcelain)" ] || { echo "Working tree is not clean." >&2; exit 1; }
    latest="$(git for-each-ref --sort=-v:refname --format='%(refname:short)' 'refs/tags/*' \
        | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -n1 || true)"
    if [ -z "$latest" ]; then
        new="v0.0.1"
    else
        ver="${latest#v}"
        new="v${ver%%.*}.$(echo "$ver" | cut -d. -f2).$(( ${ver##*.} + 1 ))"
    fi
    echo ">>> Tagging $new (previous: ${latest:-none})"
    git tag "$new"

# Push the current release tag and publish ./dist as a GitHub release.
upload:
    #!/usr/bin/env bash
    set -euo pipefail
    cd "{{root}}"
    command -v gh >/dev/null 2>&1 || { echo "gh (GitHub CLI) is not installed" >&2; exit 1; }
    version="$(just -f release.justfile _version)"
    [ "$version" != "v0.0.0-dev" ] || { echo "No release tag found. Run \`tag\` first." >&2; exit 1; }
    echo ">>> Pushing git tag $version"
    git push origin "$version"
    prev="$(git for-each-ref --sort=-v:refname --format='%(refname:short)' 'refs/tags/*' \
        | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | sed -n '2p' || true)"
    git log --format='- %s (%h)' "${prev:+$prev..}$version" \
        | gh release create "$version" dist/*.tar.gz --title "$version" --notes-file -
    echo ">>> Released $version"

# check, tag, build, upload in order.
all:
    just -f release.justfile check
    just -f release.justfile tag
    just -f release.justfile build
    just -f release.justfile upload
