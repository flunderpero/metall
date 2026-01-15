precommit:
    just go-mod
    just fmt
    just lint
    just test

lint:
    go tool golangci-lint run ./metallc/...
    just lint-fixme

# We never want to commit anything with a "fixme" comment.
lint-fixme:
    #!/usr/bin/env sh
    ! git --no-pager grep -i -n --untracked "fixme" -- :^.golangci.yaml :^.spell :^justfile

fmt:
    go tool golangci-lint fmt ./metallc/...

test:
    go test ./metallc/...

go-mod:
    cd metallc && go mod tidy
    cd tools && go mod tidy
    go work sync
