package skill

import (
	"bytes"
	"cmp"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// Skill describes a loaded skill file.
type Skill struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Triggers    []string `yaml:"triggers"`
	Path        string   `yaml:"-"`
	body        string   // cached body (after frontmatter), populated at scan time
}

// Store manages skill files in a directory.
type Store struct {
	dir    string
	skills []Skill
}

// FormatOpts controls how FormatList renders a skill listing.
type FormatOpts struct {
	Prefix    string // header line before the list (e.g. "Available skills:\n")
	Indent    string // per-item prefix (e.g. "- " or "  ")
	Separator string // between name and description (e.g. ": " or " — ")
	Triggers  bool   // include trigger keywords
}

// FormatList formats skills as a human-readable listing.
func FormatList(skills []Skill, opts FormatOpts) string {
	var b strings.Builder
	if opts.Prefix != "" {
		b.WriteString(opts.Prefix)
	}
	for _, sk := range skills {
		b.WriteString(opts.Indent)
		b.WriteString(sk.Name)
		if sk.Description != "" {
			b.WriteString(opts.Separator)
			b.WriteString(sk.Description)
		}
		if opts.Triggers && len(sk.Triggers) > 0 {
			b.WriteString(" (triggers: ")
			b.WriteString(strings.Join(sk.Triggers, ", "))
			b.WriteByte(')')
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// NewStore scans dir for .md files with YAML frontmatter and caches metadata.
func NewStore(dir string) *Store {
	s := &Store{dir: dir}
	s.scan()
	return s
}

// Dir returns the skills directory path.
func (s *Store) Dir() string { return s.dir }

// List returns all loaded skills.
func (s *Store) List() []Skill {
	return s.skills
}

// Load returns the cached body of a skill by name (no frontmatter).
func (s *Store) Load(name string) (string, error) {
	for _, sk := range s.skills {
		if strings.EqualFold(sk.Name, name) {
			return sk.body, nil
		}
	}
	return "", os.ErrNotExist
}

// Match returns skills whose trigger keywords appear in text.
// Sorted by longest matching trigger first (most specific).
func (s *Store) Match(text string) []Skill {
	lower := strings.ToLower(text)
	type hit struct {
		skill      Skill
		triggerLen int
	}
	var hits []hit
	for _, sk := range s.skills {
		for _, t := range sk.Triggers {
			if strings.Contains(lower, strings.ToLower(t)) {
				hits = append(hits, hit{skill: sk, triggerLen: len(t)})
				break // one match per skill is enough
			}
		}
	}
	// Sort by trigger length descending (most specific first)
	slices.SortFunc(hits, func(a, b hit) int { return cmp.Compare(b.triggerLen, a.triggerLen) })
	result := make([]Skill, len(hits))
	for i, h := range hits {
		result[i] = h.skill
	}
	return result
}

func (s *Store) scan() {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(s.dir, e.Name())
		sk, body, err := parseSkillFile(path)
		if err != nil {
			continue
		}
		sk.body = body
		s.skills = append(s.skills, sk)
	}
}

var frontmatterSep = []byte("---")

// parseSkillFile reads a single .md file, extracts YAML frontmatter metadata,
// and returns the skill descriptor and its body content in one pass.
func parseSkillFile(path string) (Skill, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Skill{}, "", err
	}

	trimmed := bytes.TrimSpace(data)
	fallbackName := strings.TrimSuffix(filepath.Base(path), ".md")

	if !bytes.HasPrefix(trimmed, frontmatterSep) {
		return Skill{Name: fallbackName, Path: path}, string(trimmed), nil
	}

	rest := trimmed[len(frontmatterSep):]
	end := bytes.Index(rest, frontmatterSep)
	if end < 0 {
		return Skill{Name: fallbackName, Path: path}, string(trimmed), nil
	}

	var sk Skill
	if err := yaml.Unmarshal(rest[:end], &sk); err != nil {
		return Skill{}, "", err
	}
	if sk.Name == "" {
		sk.Name = fallbackName
	}
	sk.Path = path
	body := strings.TrimSpace(string(rest[end+len(frontmatterSep):]))
	return sk, body, nil
}
