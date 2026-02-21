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
    go test ./metallc/...

examples:
    #!/usr/bin/env bash
    set -euo pipefail

    for file in examples/*.met; do
        echo ">>> $file"
        go run ./metallc/... run "$file"
    done

go-mod:
    cd metallc && go mod tidy
    cd tools && go mod tidy
    go work sync
