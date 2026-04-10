package safety

import (
	"testing"

	sdk "github.com/dotcommander/piglet/sdk"
	"github.com/stretchr/testify/assert"
)

func TestRegister(t *testing.T) {
	t.Parallel()

	t.Run("normal registration", func(t *testing.T) {
		t.Parallel()
		called := false
		Register(nil, "test", func(_ *sdk.Extension) { called = true })
		assert.True(t, called, "registration function should be called")
	})

	t.Run("panic is recovered", func(t *testing.T) {
		t.Parallel()
		Register(nil, "panicking", func(_ *sdk.Extension) {
			panic("boom")
		})
		// Should not crash — recoverPanic swallows the panic
	})
}

func TestRegisterWithVersion(t *testing.T) {
	t.Parallel()

	t.Run("normal registration", func(t *testing.T) {
		t.Parallel()
		called := false
		RegisterWithVersion(nil, "test", "1.0.0", func(_ *sdk.Extension, _ string) { called = true })
		assert.True(t, called, "registration function should be called")
	})

	t.Run("panic is recovered", func(t *testing.T) {
		t.Parallel()
		RegisterWithVersion(nil, "panicking", "1.0.0", func(_ *sdk.Extension, _ string) {
			panic("boom")
		})
		// Should not crash
	})
}
