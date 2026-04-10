package suggest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTurnDataEmptyToolResults(t *testing.T) {
	t.Parallel()

	var turn TurnData
	err := json.Unmarshal([]byte(`{"Assistant":"","ToolResults":[]}`), &turn)
	assert.NoError(t, err)
	assert.Empty(t, turn.ToolResults)
}

func TestTurnDataMissingToolResults(t *testing.T) {
	t.Parallel()

	var turn TurnData
	err := json.Unmarshal([]byte(`{"Assistant":"hello"}`), &turn)
	assert.NoError(t, err)
	assert.Empty(t, turn.ToolResults)
}
