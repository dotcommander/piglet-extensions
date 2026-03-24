package repomap

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	defaultMaxTokens      = 1024
	defaultMaxTokensNoCtx = 2048
	stalDebounce          = 30 * time.Second
)

// Config holds repomap configuration.
type Config struct {
	MaxTokens      int // token budget for output (default: 1024)
	MaxTokensNoCtx int // budget when no files in conversation (default: 2048)
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		MaxTokens:      defaultMaxTokens,
		MaxTokensNoCtx: defaultMaxTokensNoCtx,
	}
}

// Map holds the built repository map state.
type Map struct {
	root    string
	config  Config
	mu      sync.RWMutex
	ranked  []RankedFile
	output  string
	builtAt time.Time
	mtimes  map[string]time.Time // path → mtime at last build
}

// New creates a new Map for the given project root.
func New(root string, cfg Config) *Map {
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = defaultMaxTokens
	}
	if cfg.MaxTokensNoCtx == 0 {
		cfg.MaxTokensNoCtx = defaultMaxTokensNoCtx
	}
	return &Map{
		root:   root,
		config: cfg,
	}
}

// Build performs a full scan → parse → rank → format pipeline.
// Safe for concurrent use.
func (m *Map) Build(ctx context.Context) error {
	files, err := ScanFiles(ctx, m.root)
	if err != nil {
		return err
	}

	parsed, mtimes, err := m.parseFiles(ctx, files)
	if err != nil {
		return err
	}

	ranked := RankFiles(parsed)
	output := FormatMap(ranked, m.config.MaxTokens, false, false)

	m.mu.Lock()
	m.ranked = ranked
	m.output = output
	m.builtAt = time.Now()
	m.mtimes = mtimes
	m.mu.Unlock()

	return nil
}

// String returns the current formatted map output.
// Returns empty string if Build has not been called or produced no symbols.
func (m *Map) String() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.output
}

// StringVerbose returns the full verbose map output (all symbols, no summarization).
// Returns empty string if Build has not been called or produced no symbols.
func (m *Map) StringVerbose() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.ranked) == 0 {
		return ""
	}
	return FormatMap(m.ranked, 0, true, false) // verbose=true, detail=false
}

// StringDetail returns the full detailed map output with signatures and struct fields.
// Returns empty string if Build has not been called or produced no symbols.
func (m *Map) StringDetail() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.ranked) == 0 {
		return ""
	}
	return FormatMap(m.ranked, 0, true, true) // verbose=true, detail=true
}

// StringLines returns the source-line format showing actual code definitions.
// More concise than verbose mode, more useful than compact mode.
// Returns empty string if Build has not been called or produced no symbols.
func (m *Map) StringLines() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.ranked) == 0 {
		return ""
	}
	return FormatLines(m.ranked, m.config.MaxTokensNoCtx, m.root)
}

// BuiltAt returns the time of the last successful build, or zero time if never built.
func (m *Map) BuiltAt() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.builtAt
}

// Stale reports whether any tracked file has been modified since the last build.
// Uses file mtimes for fast checking.
// Also stale if Build has never been called.
// Debounced: returns false if last build was <30s ago.
func (m *Map) Stale() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.builtAt.IsZero() {
		return true
	}
	if time.Since(m.builtAt) < stalDebounce {
		return false
	}
	for path, recorded := range m.mtimes {
		info, err := os.Stat(path)
		if err != nil {
			return true
		}
		if info.ModTime().After(recorded) {
			return true
		}
	}
	return false
}

// parseFiles parses all discovered files in parallel and returns the symbols
// and a path→mtime map for stale checking.
// Uses ctags for non-Go files when available, falling back to regex.
func (m *Map) parseFiles(ctx context.Context, files []FileInfo) ([]*FileSymbols, map[string]time.Time, error) {
	// Collect mtimes for all files.
	mtimes := make(map[string]time.Time, len(files))
	for _, fi := range files {
		absPath := filepath.Join(m.root, fi.Path)
		info, err := os.Stat(absPath)
		if err != nil {
			continue
		}
		mtimes[absPath] = info.ModTime()
	}

	// Parse Go files in parallel with errgroup.
	type result struct {
		symbols *FileSymbols
	}
	goResults := make([]result, len(files))
	g, _ := errgroup.WithContext(ctx)
	g.SetLimit(runtime.NumCPU())

	for i, fi := range files {
		if fi.Language != "go" {
			continue
		}
		g.Go(func() error {
			absPath := filepath.Join(m.root, fi.Path)
			sym, err := ParseGoFile(absPath, m.root)
			if err != nil {
				return nil //nolint:nilerr // skip parse errors
			}
			goResults[i] = result{symbols: sym}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, nil, err
	}

	parsed := make([]*FileSymbols, 0, len(files))
	for _, r := range goResults {
		if r.symbols != nil {
			parsed = append(parsed, r.symbols)
		}
	}

	// Parse non-Go files: ctags batch or regex fallback.
	if CtagsAvailable() {
		ctagsParsed, err := ParseWithCtags(ctx, m.root, files)
		if err == nil {
			parsed = append(parsed, ctagsParsed...)
		} else {
			// ctags failed — fall back to regex for non-Go files.
			parsed = append(parsed, m.parseGenericFiles(files)...)
		}
	} else {
		parsed = append(parsed, m.parseGenericFiles(files)...)
	}

	return parsed, mtimes, nil
}

// parseGenericFiles parses non-Go files using regex patterns.
func (m *Map) parseGenericFiles(files []FileInfo) []*FileSymbols {
	var result []*FileSymbols
	for _, fi := range files {
		if fi.Language == "go" {
			continue
		}
		absPath := filepath.Join(m.root, fi.Path)
		sym, err := ParseGenericFile(absPath, m.root, fi.Language)
		if err != nil {
			continue
		}
		result = append(result, sym)
	}
	return result
}

