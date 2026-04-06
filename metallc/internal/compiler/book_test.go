//nolint:exhaustruct
package compiler

import (
	"testing"
)

func TestBookMD(t *testing.T) {
	t.Parallel()
	runE2ESuite(t, "../../../doc/book.md", e2eConfig{prefix: "book_", minimalPrelude: false, useEnvOptLevel: true})
}
