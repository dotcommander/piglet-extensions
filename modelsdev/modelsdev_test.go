package modelsdev

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestRefreshFromURL_UpdatesExisting(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	pigletDir := filepath.Join(dir, "piglet")
	require.NoError(t, os.MkdirAll(pigletDir, 0o755))

	// Write a models.yaml with known values
	initial := modelsFile{Models: []modelEntry{
		{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", API: "openai", ContextWindow: 100000, MaxTokens: 10000},
		{ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet", Provider: "anthropic", API: "anthropic", ContextWindow: 180000, MaxTokens: 8000},
	}}
	data, err := yaml.Marshal(&initial)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(pigletDir, "models.yaml"), data, 0o644))

	// API returns updated values
	apiResp := apiResponse{
		"openai": providerData{ID: "openai", Models: map[string]modelData{
			"gpt-4o": {ID: "gpt-4o", Name: "GPT-4o", Limit: modelLimit{Context: 128000, Output: 16384}},
		}},
		"anthropic": providerData{ID: "anthropic", Models: map[string]modelData{
			"claude-sonnet-4-20250514": {ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4", Limit: modelLimit{Context: 200000, Output: 16000}},
		}},
	}
	srv := serveJSON(t, apiResp)

	updated, err := RefreshFromURL(context.Background(), srv.URL)
	require.NoError(t, err)
	assert.Equal(t, 2, updated)

	// Verify file was updated
	raw, err := os.ReadFile(filepath.Join(pigletDir, "models.yaml"))
	require.NoError(t, err)
	var result modelsFile
	require.NoError(t, yaml.Unmarshal(raw, &result))
	assert.Equal(t, 128000, result.Models[0].ContextWindow)
	assert.Equal(t, 16384, result.Models[0].MaxTokens)
	assert.Equal(t, "Claude Sonnet 4", result.Models[1].Name)
	assert.Equal(t, 200000, result.Models[1].ContextWindow)
}

func TestRefreshFromURL_NeverAddsNew(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	pigletDir := filepath.Join(dir, "piglet")
	require.NoError(t, os.MkdirAll(pigletDir, 0o755))

	initial := modelsFile{Models: []modelEntry{
		{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", API: "openai", ContextWindow: 100000, MaxTokens: 10000},
	}}
	data, err := yaml.Marshal(&initial)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(pigletDir, "models.yaml"), data, 0o644))

	apiResp := apiResponse{
		"openai": providerData{ID: "openai", Models: map[string]modelData{
			"gpt-4o": {ID: "gpt-4o", Name: "GPT-4o", Limit: modelLimit{Context: 128000, Output: 16384}},
			"gpt-5":  {ID: "gpt-5", Name: "GPT-5", Limit: modelLimit{Context: 500000, Output: 32768}},
		}},
	}
	srv := serveJSON(t, apiResp)

	updated, err := RefreshFromURL(context.Background(), srv.URL)
	require.NoError(t, err)
	assert.Equal(t, 1, updated) // only gpt-4o updated

	raw, err := os.ReadFile(filepath.Join(pigletDir, "models.yaml"))
	require.NoError(t, err)
	var result modelsFile
	require.NoError(t, yaml.Unmarshal(raw, &result))
	assert.Len(t, result.Models, 1) // gpt-5 NOT added
}

func TestRefreshFromURL_NoChangeNoWrite(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	pigletDir := filepath.Join(dir, "piglet")
	require.NoError(t, os.MkdirAll(pigletDir, 0o755))

	initial := modelsFile{Models: []modelEntry{
		{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", API: "openai", ContextWindow: 128000, MaxTokens: 16384},
	}}
	data, err := yaml.Marshal(&initial)
	require.NoError(t, err)
	modPath := filepath.Join(pigletDir, "models.yaml")
	require.NoError(t, os.WriteFile(modPath, data, 0o644))
	info, _ := os.Stat(modPath)
	origModTime := info.ModTime()

	apiResp := apiResponse{
		"openai": providerData{ID: "openai", Models: map[string]modelData{
			"gpt-4o": {ID: "gpt-4o", Name: "GPT-4o", Limit: modelLimit{Context: 128000, Output: 16384}},
		}},
	}
	srv := serveJSON(t, apiResp)

	updated, err := RefreshFromURL(context.Background(), srv.URL)
	require.NoError(t, err)
	assert.Equal(t, 0, updated)

	// File should not have been rewritten
	info2, _ := os.Stat(modPath)
	assert.Equal(t, origModTime, info2.ModTime())
}

func TestCacheStale_Missing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	assert.True(t, CacheStale())
}

func TestCacheStale_Fresh(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	pigletDir := filepath.Join(dir, "piglet")
	require.NoError(t, os.MkdirAll(pigletDir, 0o755))

	c := &cache{FetchedAt: time.Now(), Data: apiResponse{}}
	data, _ := json.Marshal(c)
	require.NoError(t, os.WriteFile(filepath.Join(pigletDir, cacheFile), data, 0o644))

	assert.False(t, CacheStale())
}

func TestCacheStale_Expired(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	pigletDir := filepath.Join(dir, "piglet")
	require.NoError(t, os.MkdirAll(pigletDir, 0o755))

	c := &cache{FetchedAt: time.Now().Add(-25 * time.Hour), Data: apiResponse{}}
	data, _ := json.Marshal(c)
	require.NoError(t, os.WriteFile(filepath.Join(pigletDir, cacheFile), data, 0o644))

	assert.True(t, CacheStale())
}

func serveJSON(t *testing.T, v any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(v)
	}))
	t.Cleanup(srv.Close)
	return srv
}
