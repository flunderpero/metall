# Fetch, build, and stage a static libSDL3.a + headers under vendor/sdl3/.

sdl_version := "3.4.10"
sdl_sha256 := "0dc11d980ba17250200718fa4e28011da293f27ed92f92203afffe396811f307"
sdl_url := "https://github.com/libsdl-org/SDL/archive/refs/tags/release-" + sdl_version + ".tar.gz"

osx_arch := if os() == "macos" { "-DCMAKE_OSX_ARCHITECTURES=arm64" } else { "" }

work := ".build/sdl"
tarball := work / ("sdl3-" + sdl_version + ".tar.gz")
src := work / ("SDL-release-" + sdl_version)
build := src / "build"
vendor := "vendor/sdl3"

default:
    #!/usr/bin/env bash
    set -euo pipefail
    if [[ "$(cat "{{vendor}}/VERSION" 2>/dev/null || true)" == "{{sdl_version}}" ]]; then
        echo "SDL {{sdl_version}} already vendored"
    else
        just -f "{{justfile()}}" vendor
    fi

fetch:
    #!/usr/bin/env bash
    set -euo pipefail
    mkdir -p "{{work}}"
    [[ -f "{{tarball}}" ]] || curl -fL --retry 3 -o "{{tarball}}" "{{sdl_url}}"
    echo "{{sdl_sha256}}  {{tarball}}" | shasum -a 256 -c -

extract: fetch
    rm -rf "{{src}}"
    tar -xzf "{{tarball}}" -C "{{work}}"

build: extract
    cmake -S "{{src}}" -B "{{build}}" -DCMAKE_BUILD_TYPE=Release \
        {{osx_arch}} \
        -DSDL_STATIC=ON -DSDL_SHARED=OFF -DSDL_TESTS=OFF -DSDL_EXAMPLES=OFF
    cmake --build "{{build}}" --parallel

vendor: build
    mkdir -p "{{vendor}}/lib" "{{vendor}}/include"
    cp "{{build}}/libSDL3.a" "{{vendor}}/lib/"
    cp -R "{{src}}/include/SDL3" "{{vendor}}/include/"
    echo "{{sdl_version}}" > "{{vendor}}/VERSION"

clean:
    rm -rf "{{work}}" "{{vendor}}"
