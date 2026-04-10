package sessiontools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldEnhance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		facts []memoryFact
		cfg   Config
		want  bool
	}{
		{
			name: "llm mode always true",
			cfg:  Config{SummaryMode: SummaryModeLLM},
			want: true,
		},
		{
			name: "template mode always false",
			cfg:  Config{SummaryMode: SummaryModeTemplate},
			want: false,
		},
		{
			name: "auto mode zero facts",
			cfg:  Config{SummaryMode: SummaryModeAuto},
			want: false,
		},
		{
			name: "auto mode 21 facts triggers enhance",
			facts: func() []memoryFact {
				facts := make([]memoryFact, 21)
				for i := range facts {
					facts[i] = memoryFact{Key: "ctx:goal", Value: "x"}
				}
				return facts
			}(),
			cfg:  Config{SummaryMode: SummaryModeAuto},
			want: true,
		},
		{
			name: "auto mode 5 facts no errors",
			facts: []memoryFact{
				{Key: "ctx:goal:1", Value: "a"},
				{Key: "ctx:goal:2", Value: "b"},
				{Key: "ctx:goal:3", Value: "c"},
				{Key: "ctx:goal:4", Value: "d"},
				{Key: "ctx:goal:5", Value: "e"},
			},
			cfg:  Config{SummaryMode: SummaryModeAuto},
			want: false,
		},
		{
			name: "auto mode 5 facts 3 errors triggers enhance",
			facts: []memoryFact{
				{Key: "ctx:goal:1", Value: "a"},
				{Key: "ctx:error:1", Value: "e1"},
				{Key: "ctx:error:2", Value: "e2"},
				{Key: "ctx:error:3", Value: "e3"},
				{Key: "ctx:goal:5", Value: "e"},
			},
			cfg:  Config{SummaryMode: SummaryModeAuto},
			want: true,
		},
		{
			name: "auto mode 5 facts 2 errors no enhance",
			facts: []memoryFact{
				{Key: "ctx:goal:1", Value: "a"},
				{Key: "ctx:error:1", Value: "e1"},
				{Key: "ctx:error:2", Value: "e2"},
				{Key: "ctx:goal:4", Value: "d"},
				{Key: "ctx:goal:5", Value: "e"},
			},
			cfg:  Config{SummaryMode: SummaryModeAuto},
			want: false,
		},
		{
			name: "auto mode 20 facts exactly no enhance",
			facts: func() []memoryFact {
				return make([]memoryFact, 20)
			}(),
			cfg:  Config{SummaryMode: SummaryModeAuto},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := shouldEnhance(tt.facts, tt.cfg)
			assert.Equal(t, tt.want, got)
		})
	}
}
