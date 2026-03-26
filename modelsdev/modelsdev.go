package modelsdev

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
	"gopkg.in/yaml.v3"
)

const (
	DefaultAPIURL = "https://models.dev/api.json"
	cacheFile     = ".models-cache.json"
	cacheMaxAge   = 24 * time.Hour
)

type apiResponse map[string]providerData

type providerData struct {
	ID     string               `json:"id"`
	Models map[string]modelData `json:"models"`
}

type modelData struct {
	ID    string     `json:"id"`
	Name  string     `json:"name"`
	Limit modelLimit `json:"limit"`
}

type modelLimit struct {
	Context int `json:"context"`
	Output  int `json:"output"`
}

// models.yaml types (mirrors piglet's provider/registry.go format).

type modelsFile struct {
	Models []modelEntry `yaml:"models"`
}

type modelEntry struct {
	ID            string `yaml:"id"`
	Name          string `yaml:"name"`
	Provider      string `yaml:"provider"`
	API           string `yaml:"api"`
	BaseURL       string `yaml:"baseUrl,omitempty"`
	ContextWindow int    `yaml:"contextWindow"`
	MaxTokens     int    `yaml:"maxTokens"`
}

// cache is the on-disk format for the API response cache.
type cache struct {
	FetchedAt time.Time   `json:"fetched_at"`
	Data      apiResponse `json:"data"`
}

// CacheStale returns true if the cache is missing or older than 24h.
func CacheStale() bool {
	c := readCache()
	if c == nil {
		return true
	}
	return time.Since(c.FetchedAt) > cacheMaxAge
}

// Refresh fetches the models.dev API, merges updates into the existing
// models.yaml, and writes back. Returns the number of updated models.
func Refresh(ctx context.Context) (int, error) {
	return RefreshFromURL(ctx, DefaultAPIURL)
}

// RefreshFromURL is like Refresh but fetches from the given URL (for testing).
func RefreshFromURL(ctx context.Context, url string) (int, error) {
	data, err := fetch(ctx, url)
	if err != nil {
		return 0, fmt.Errorf("fetch models.dev: %w", err)
	}

	// Cache write failure is non-fatal; next CacheStale check will refetch.
	_ = writeCache(&cache{FetchedAt: time.Now(), Data: data})

	// Read existing models.yaml
	modPath, err := modelsPath()
	if err != nil {
		return 0, fmt.Errorf("models path: %w", err)
	}

	raw, err := os.ReadFile(modPath)
	if err != nil {
		return 0, fmt.Errorf("read models.yaml: %w", err)
	}

	var file modelsFile
	if err := yaml.Unmarshal(raw, &file); err != nil {
		return 0, fmt.Errorf("parse models.yaml: %w", err)
	}

	// Index API data by provider/id
	apiIndex := indexAPI(data)

	// Merge: update existing models, never add new ones
	updated := 0
	for i := range file.Models {
		m := &file.Models[i]
		key := m.Provider + "/" + m.ID
		api, ok := apiIndex[key]
		if !ok {
			continue
		}

		changed := false
		if api.Limit.Context > 0 && api.Limit.Context != m.ContextWindow {
			m.ContextWindow = api.Limit.Context
			changed = true
		}
		if api.Limit.Output > 0 && api.Limit.Output != m.MaxTokens {
			m.MaxTokens = api.Limit.Output
			changed = true
		}
		if api.Name != "" && api.Name != m.Name {
			m.Name = api.Name
			changed = true
		}
		if changed {
			updated++
		}
	}

	if updated == 0 {
		return 0, nil
	}

	// Write updated models.yaml atomically
	out, err := yaml.Marshal(&file)
	if err != nil {
		return 0, fmt.Errorf("marshal models.yaml: %w", err)
	}
	if err := xdg.WriteFileAtomic(modPath, out); err != nil {
		return 0, fmt.Errorf("write models.yaml: %w", err)
	}

	return updated, nil
}

func fetch(ctx context.Context, url string) (apiResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "piglet")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var result apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}
	return result, nil
}

func indexAPI(data apiResponse) map[string]modelData {
	index := make(map[string]modelData)
	for provName, prov := range data {
		for _, md := range prov.Models {
			index[provName+"/"+md.ID] = md
		}
	}
	return index
}

func modelsPath() (string, error) {
	dir, err := xdg.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "models.yaml"), nil
}

func cachePath() (string, error) {
	dir, err := xdg.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, cacheFile), nil
}

func readCache() *cache {
	path, err := cachePath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var c cache
	if err := json.Unmarshal(data, &c); err != nil {
		return nil
	}
	return &c
}

func writeCache(c *cache) error {
	path, err := cachePath()
	if err != nil {
		return err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal cache: %w", err)
	}
	return xdg.WriteFileAtomic(path, data)
}
