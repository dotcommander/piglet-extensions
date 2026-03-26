package webfetch

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
	"gopkg.in/yaml.v3"
)

// Config holds configuration for webfetch providers.
type Config struct {
	PerplexityAPIKey string       `yaml:"perplexity_api_key"`
	GeminiAPIKey     string       `yaml:"gemini_api_key"`
	GitHub           GitHubConfig `yaml:"github"`
}

// LoadConfig reads configuration from ~/.config/piglet/webfetch.yaml.
// If the file doesn't exist, it creates one with default values.
func LoadConfig() (*Config, error) {
	configDir, err := xdg.ConfigDir()
	if err != nil {
		return nil, fmt.Errorf("get config dir: %w", err)
	}

	configPath := filepath.Join(configDir, "webfetch.yaml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create default config file
			defaultConfig := &Config{
				GitHub: GitHubConfig{
					Enabled:        true,
					SkipLargeRepos: true,
				},
			}
			data, err := yaml.Marshal(defaultConfig)
			if err != nil {
				return nil, fmt.Errorf("marshal default config: %w", err)
			}
			if err := xdg.WriteFileAtomic(configPath, data); err != nil {
				return nil, fmt.Errorf("create default config: %w", err)
			}
			return defaultConfig, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// GitHub defaults are set in GitHubConfig struct tags / NewGitHubClient.
	// Don't override explicit false values from config.

	return &cfg, nil
}

// rateLimiter implements a simple rate limiter for API calls.
type rateLimiter struct {
	interval time.Duration
	mu       sync.Mutex
	lastCall time.Time
}

// newRateLimiter creates a rate limiter with the given minimum interval between calls.
func newRateLimiter(interval time.Duration) *rateLimiter {
	return &rateLimiter{interval: interval}
}

// Wait blocks until the minimum interval has passed since the last call.
func (r *rateLimiter) Wait() {
	r.mu.Lock()
	defer r.mu.Unlock()

	elapsed := time.Since(r.lastCall)
	if elapsed < r.interval {
		wait := r.interval - elapsed
		slog.Debug("rate limiter waiting", "wait", wait)
		time.Sleep(wait)
	}
	r.lastCall = time.Now()
}

// PerplexityRateLimitInterval is the minimum time between Perplexity API calls (10 req/min).
const PerplexityRateLimitInterval = 6 * time.Second
