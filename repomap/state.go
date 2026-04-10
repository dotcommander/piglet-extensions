package repomap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	sdk "github.com/dotcommander/piglet/sdk"
)

// repomapState holds mutable state for the repomap extension lifecycle.
type repomapState struct {
	rm      *Map
	ext     *sdk.Extension
	built   bool
	builtMu sync.RWMutex
}

func newRepomapState() *repomapState {
	return &repomapState{}
}

func (s *repomapState) setBuilt(v bool) {
	s.builtMu.Lock()
	s.built = v
	s.builtMu.Unlock()
}

func (s *repomapState) isBuilt() bool {
	s.builtMu.RLock()
	defer s.builtMu.RUnlock()
	return s.built
}

// registerPrompt registers the Repository Map prompt section with the given content.
func (s *repomapState) registerPrompt(ext *sdk.Extension, content string) {
	ext.RegisterPromptSection(sdk.PromptSectionDef{
		Title:   "Repository Map",
		Content: content,
		Order:   95,
	})
}

// buildInBackground runs a full scan in a goroutine, updating the user via notifications.
func (s *repomapState) buildInBackground() {
	s.ext.Notify("Scanning repository...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	if err := s.rm.Build(ctx); err != nil {
		if errors.Is(err, ErrNotCodeProject) {
			s.ext.Log("debug", "skipping repomap: no source files found")
		} else {
			s.ext.Notify("Scan failed")
			s.ext.Log("warn", "repomap background build failed: "+err.Error())
		}
		return
	}

	elapsed := time.Since(start).Round(time.Millisecond)
	out := s.rm.StringLines()
	if out == "" {
		s.ext.Notify("No source files found")
		s.ext.Log("warn", "repomap produced empty output")
		s.setBuilt(true)
		return
	}

	s.setBuilt(true)
	s.ext.Notify("Map ready")
	s.ext.Log("info", "repomap built in "+elapsed.String())
}

// registerOnInit sets up the OnInit handler for cache-aware startup.
func (s *repomapState) registerOnInit(e *sdk.Extension) {
	e.OnInit(func(x *sdk.Extension) {
		start := time.Now()
		x.Log("debug", "[repomap] OnInit start")

		s.ext = x

		cachedInv := LoadInventory(repomapCacheDir())
		if cachedInv != nil {
			x.Log("debug", fmt.Sprintf("[repomap] inventory cache found: %d files", len(cachedInv.Files)))
		}
		cfg := loadRepomapConfig()
		s.rm = New(x.CWD(), cfg)

		cd := repomapCacheDir()
		s.rm.SetCacheDir(cd)

		// Try disk cache first — instant startup
		if s.rm.LoadCache(cd) {
			x.Log("debug", fmt.Sprintf("[repomap] cache hit (%s)", time.Since(start)))
			s.setBuilt(true)
			s.registerPrompt(x, s.rm.StringLines())
			go func() {
				if !s.rm.Stale() {
					return
				}
				buildCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if err := s.rm.Build(buildCtx); err != nil {
					if !errors.Is(err, ErrNotCodeProject) {
						x.Log("warn", "repomap background rebuild: "+err.Error())
					}
				}
			}()
			x.Log("debug", fmt.Sprintf("[repomap] OnInit complete (%s)", time.Since(start)))
			return
		}

		x.Log("debug", fmt.Sprintf("[repomap] cache miss — quick build start (%s)", time.Since(start)))

		quickCtx, quickCancel := context.WithTimeout(context.Background(), 5*time.Second)
		buildErr := s.rm.Build(quickCtx)
		quickCancel()
		if buildErr == nil {
			x.Log("debug", fmt.Sprintf("[repomap] quick build done (%s)", time.Since(start)))
			s.setBuilt(true)
			s.registerPrompt(x, s.rm.StringLines())
			x.Log("debug", fmt.Sprintf("[repomap] OnInit complete (%s)", time.Since(start)))
			return
		}
		if errors.Is(buildErr, ErrNotCodeProject) {
			x.Log("debug", "skipping repomap: no source files found")
			x.Log("debug", fmt.Sprintf("[repomap] OnInit complete — not a code project (%s)", time.Since(start)))
			return
		}

		x.Log("debug", fmt.Sprintf("[repomap] quick build timed out — continuing in background (%s)", time.Since(start)))

		s.registerPrompt(x, "")
		x.Log("debug", fmt.Sprintf("[repomap] OnInit complete (%s)", time.Since(start)))
		go s.buildInBackground()
	})
}

// registerEventHandler registers the turn_end handler for stale detection.
func (s *repomapState) registerEventHandler(e *sdk.Extension) {
	e.RegisterEventHandler(sdk.EventHandlerDef{
		Name:     "repomap-stale-check",
		Priority: 50,
		Events:   []string{"turn_end"},
		Handle: func(ctx context.Context, eventType string, data json.RawMessage) *sdk.Action {
			if s.rm == nil {
				return nil
			}

			if turnModifiedCode(data) {
				s.rm.Dirty()
			}

			if !s.rm.Stale() {
				return nil
			}
			go func() {
				buildCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if err := s.rm.Build(buildCtx); err != nil {
					if !errors.Is(err, ErrNotCodeProject) {
						s.ext.Log("warn", "repomap rebuild failed: "+err.Error())
					}
				}
			}()
			return nil
		},
	})
}
