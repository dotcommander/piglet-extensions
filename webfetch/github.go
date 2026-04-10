package webfetch

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// GitHubConfig configures GitHub clone behavior.
type GitHubConfig struct {
	Enabled        bool `yaml:"enabled"`
	SkipLargeRepos bool `yaml:"skip_large_repos"`
}

// GitHubResult holds the result of fetching a GitHub repo.
type GitHubResult struct {
	LocalPath string   `json:"local_path"`
	README    string   `json:"readme"`
	Tree      []string `json:"tree"`
	UsedAPI   bool     `json:"used_api"`
}

// githubURL represents a parsed GitHub URL.
type githubURL struct {
	Owner    string
	Repo     string
	Branch   string // empty means default branch
	Path     string // path within repo (empty means root)
	Commit   string // full commit SHA (for commit URLs)
	IsCommit bool   // true if URL points to a specific commit
}

// parseGitHubURL parses various GitHub URL formats.
// Supported formats:
//   - https://github.com/owner/repo
//   - https://github.com/owner/repo/tree/branch
//   - https://github.com/owner/repo/tree/branch/path
//   - https://github.com/owner/repo/commit/sha
func parseGitHubURL(rawURL string) (*githubURL, bool) {
	rawURL = strings.TrimSpace(rawURL)

	re := regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/]+)(?:/(tree/([^/]+)(?:/(.*))?|commit/([a-f0-9]{40}))?)/?$`)
	matches := re.FindStringSubmatch(rawURL)
	if matches == nil {
		return nil, false
	}

	result := &githubURL{
		Owner: matches[1],
		Repo:  strings.TrimSuffix(matches[2], ".git"),
	}

	if matches[6] != "" {
		result.Commit = matches[6]
		result.IsCommit = true
		return result, true
	}

	result.Branch = matches[4]
	result.Path = matches[5]

	return result, true
}

// GitHubClient handles GitHub repo cloning and API fallback.
type GitHubClient struct {
	config     *GitHubConfig
	httpClient *http.Client
}

// NewGitHubClient creates a new GitHub client.
func NewGitHubClient(cfg *GitHubConfig) *GitHubClient {
	if cfg == nil {
		cfg = &GitHubConfig{Enabled: true, SkipLargeRepos: true}
	}
	return &GitHubClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: fetchTimeout,
		},
	}
}

// Fetch retrieves content from a GitHub URL.
// Returns nil if the URL is not a GitHub URL (caller should try other providers).
func (g *GitHubClient) Fetch(ctx context.Context, rawURL string) (*GitHubResult, error) {
	parsed, ok := parseGitHubURL(rawURL)
	if !ok {
		return nil, nil
	}

	if parsed.IsCommit {
		return g.fetchCommit(ctx, parsed)
	}

	g.cleanupOldClones()

	if g.config.SkipLargeRepos {
		tooLarge, err := g.checkRepoSize(ctx, parsed)
		if err != nil {
			slog.Debug("failed to check repo size, falling back to API", "error", err)
			return g.fetchViaAPI(ctx, parsed)
		}
		if tooLarge {
			slog.Debug("repo too large, using API", "owner", parsed.Owner, "repo", parsed.Repo)
			return g.fetchViaAPI(ctx, parsed)
		}
	}

	result, err := g.clone(ctx, parsed)
	if err != nil {
		slog.Debug("clone failed, falling back to API", "error", err)
		return g.fetchViaAPI(ctx, parsed)
	}

	return result, nil
}

// clone performs a shallow clone and builds the result.
func (g *GitHubClient) clone(ctx context.Context, parsed *githubURL) (*GitHubResult, error) {
	localPath := filepath.Join(os.TempDir(), fmt.Sprintf("piglet-gh-%s-%s", parsed.Owner, parsed.Repo))

	if _, err := os.Stat(localPath); err == nil {
		slog.Debug("using existing clone", "path", localPath)
	} else {
		cloneURL := fmt.Sprintf("https://github.com/%s/%s.git", parsed.Owner, parsed.Repo)

		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", cloneURL, localPath)
		if parsed.Branch != "" {
			cmd.Args = append(cmd.Args, "--branch", parsed.Branch)
		}

		output, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("git clone failed: %w: %s", err, string(output))
		}
	}

	result := &GitHubResult{
		LocalPath: localPath,
		UsedAPI:   false,
	}

	readmePath := filepath.Join(localPath, parsed.Path, "README.md")
	if data, err := os.ReadFile(readmePath); err == nil {
		result.README = string(data)
	}

	tree, err := g.buildTree(localPath, parsed.Path)
	if err != nil {
		slog.Debug("failed to build tree", "error", err)
	}
	result.Tree = tree

	return result, nil
}

const maxTreeEntries = 10000

// buildTree creates a file tree listing (capped at maxTreeEntries).
func (g *GitHubClient) buildTree(localPath, subPath string) ([]string, error) {
	root := filepath.Join(localPath, subPath)
	var tree []string

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if strings.Contains(path, "/.git/") || filepath.Base(path) == ".git" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		if d.IsDir() {
			tree = append(tree, relPath+"/")
		} else {
			tree = append(tree, relPath)
		}

		if len(tree) >= maxTreeEntries {
			return filepath.SkipAll
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return tree, nil
}

// cleanupOldClones removes clone directories older than 1 hour.
func (g *GitHubClient) cleanupOldClones() {
	entries, err := os.ReadDir(os.TempDir())
	if err != nil {
		return
	}

	cutoff := time.Now().Add(-1 * time.Hour)

	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "piglet-gh-") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			path := filepath.Join(os.TempDir(), entry.Name())
			if err := os.RemoveAll(path); err != nil {
				slog.Debug("failed to cleanup old clone", "path", path, "error", err)
			} else {
				slog.Debug("cleaned up old clone", "path", path)
			}
		}
	}
}

// FormatGitHubResult renders a GitHubResult as markdown.
func FormatGitHubResult(result *GitHubResult) string {
	var b strings.Builder

	if result.UsedAPI {
		b.WriteString("> **Note:** Used GitHub API (repo too large or commit URL)\n\n")
	}

	if result.README != "" {
		b.WriteString("## README\n\n")
		b.WriteString(result.README)
		b.WriteString("\n\n")
	}

	if len(result.Tree) > 0 {
		b.WriteString("## File Tree\n\n```\n")
		for _, path := range result.Tree {
			b.WriteString(path)
			b.WriteString("\n")
		}
		b.WriteString("```\n")
	}

	if result.LocalPath != "" {
		fmt.Fprintf(&b, "\n*Cloned to: %s*\n", result.LocalPath)
	}

	return b.String()
}
