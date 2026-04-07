package safety

import (
	"log/slog"

	"github.com/dotcommander/piglet/sdk"
)

// Register wraps an extension's Register function with panic recovery.
// If the registration panics, the pack continues without that extension's capabilities.
func Register(e *sdk.Extension, name string, fn func(e *sdk.Extension)) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("extension register panicked", "name", name, "panic", r)
		}
	}()
	fn(e)
}

// RegisterWithVersion is like Register but for extensions that accept a version string.
func RegisterWithVersion(e *sdk.Extension, name, version string, fn func(e *sdk.Extension, version string)) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("extension register panicked", "name", name, "panic", r)
		}
	}()
	fn(e, version)
}
