package types

import (
	"testing"

	mdtest "github.com/flunderpero/metall/metallc/internal/test"
)

func TestMatchMD(t *testing.T) {
	mdtest.RunFile(t, mdtest.File("match_test.md"), mdtest.RunFunc(runEngineTest))
}
