package options

import (
	"testing"

	"github.com/IBM/go-sdk-core/v5/core"

	. "github.com/onsi/gomega"
)

func TestGetDebugMode(t *testing.T) {
	gs := NewWithT(t)

	Debug = nil
	gs.Expect(GetDebugMode()).To(Equal(false))

	Debug = core.BoolPtr(false)
	gs.Expect(GetDebugMode()).To(Equal(false))

	Debug = core.BoolPtr(true)
	gs.Expect(GetDebugMode()).To(Equal(true))
}
