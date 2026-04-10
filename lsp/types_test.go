package lsp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSymbolKindString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		kind SymbolKind
		want string
	}{
		{SymbolKindFile, "file"},
		{SymbolKindFunction, "function"},
		{SymbolKindVariable, "variable"},
		{SymbolKindClass, "class"},
		{SymbolKindInterface, "interface"},
		{SymbolKindStruct, "struct"},
		{SymbolKindMethod, "method"},
		{SymbolKindConstant, "constant"},
		{SymbolKindEnum, "enum"},
		{SymbolKindModule, "module"},
		{SymbolKindTypeParameter, "type parameter"},
		{SymbolKind(99), "unknown"},
		{SymbolKind(0), "unknown"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, tc.kind.String())
		})
	}
}
