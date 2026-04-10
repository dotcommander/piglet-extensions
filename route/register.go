package route

import (
	"context"
	"fmt"
	"sync"
	"time"

	sdk "github.com/dotcommander/piglet/sdk"
)

const Version = "0.2.0"

// state holds mutable state shared across handlers.
type state struct {
	mu       sync.RWMutex
	scorer   *Scorer
	reg      *Registry
	config   Config
	feedback *FeedbackStore
	learned  *LearnedTriggers
	cwd      string
	fbDir    string
	ready    bool
}

// Register wires the route extension into a shared SDK extension.
func Register(e *sdk.Extension) {
	s := &state{}

	e.OnInitAppend(func(x *sdk.Extension) {
		s.cwd = x.CWD()
		s.config = LoadConfig()

		intents := LoadIntents()
		domains := LoadDomains()

		ic := NewIntentClassifier(intents)
		de := NewDomainExtractor(domains)
		s.scorer = NewScorer(s.config, ic, de)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		reg, err := BuildRegistry(ctx, x)
		if err != nil {
			x.Log("warn", fmt.Sprintf("[route] registry build failed: %v", err))
			reg = nil
		}

		fbDir, _ := feedbackDir()
		fb := NewFeedbackStore(fbDir)
		learned := fb.LoadLearned()

		if reg != nil {
			mergeLearnedIntoRegistry(reg, learned)
		}

		s.mu.Lock()
		s.reg = reg
		s.feedback = fb
		s.learned = learned
		s.fbDir = fbDir
		s.ready = true
		s.mu.Unlock()
	})

	registerTools(e, s)
	registerCommand(e, s)
	registerHook(e, s)
}
