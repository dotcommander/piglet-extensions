package scaffold

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
	"github.com/dotcommander/piglet/sdk"
)

//go:embed defaults/manifest.yaml.tmpl defaults/index.ts.tmpl
var templates embed.FS

const Version = "0.1.0"

var validName = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// validateName checks that the extension name is a valid identifier
// and does not contain path traversal sequences.
func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if !validName.MatchString(name) {
		return fmt.Errorf("name must match [a-z][a-z0-9_-]+ (got %q)", name)
	}
	return nil
}

// renderTemplates loads embedded templates and substitutes {{NAME}} placeholders.
func renderTemplates(name string) (manifest, indexTS string, err error) {
	manifestTmpl, err := templates.ReadFile("defaults/manifest.yaml.tmpl")
	if err != nil {
		return "", "", fmt.Errorf("load manifest template: %w", err)
	}
	indexTmpl, err := templates.ReadFile("defaults/index.ts.tmpl")
	if err != nil {
		return "", "", fmt.Errorf("load index template: %w", err)
	}

	r := strings.NewReplacer("{{NAME}}", name)
	return r.Replace(string(manifestTmpl)), r.Replace(string(indexTmpl)), nil
}

// Register registers the scaffold extension's commands.
func Register(e *sdk.Extension) {
	e.RegisterCommand(sdk.CommandDef{
		Name:        "ext-init",
		Description: "Scaffold a new extension",
		Handler: func(ctx context.Context, args string) error {
			name := strings.TrimSpace(args)
			if err := validateName(name); err != nil {
				e.ShowMessage("Usage: /ext-init <name>\nExample: /ext-init my-tool\n\nError: " + err.Error())
				return nil
			}

			extDir, err := e.ExtensionsDir(ctx)
			if err != nil {
				return fmt.Errorf("extensions dir: %w", err)
			}

			dir := filepath.Join(extDir, name)

			if _, err := os.Stat(dir); err == nil {
				e.ShowMessage(fmt.Sprintf("Extension %q already exists at %s — remove it first or choose a different name.", name, dir))
				return nil
			}

			manifest, indexTS, err := renderTemplates(name)
			if err != nil {
				return err
			}

			if err := xdg.WriteFileAtomic(filepath.Join(dir, "manifest.yaml"), []byte(manifest)); err != nil {
				return fmt.Errorf("write manifest: %w", err)
			}
			if err := xdg.WriteFileAtomic(filepath.Join(dir, "index.ts"), []byte(indexTS)); err != nil {
				return fmt.Errorf("write index.ts: %w", err)
			}

			e.ShowMessage(fmt.Sprintf("Created extension at %s/\n\nFiles:\n  manifest.yaml — extension config\n  index.ts      — your code\n\nInstall SDK: cd %s && bun add @piglet/sdk\nRestart piglet to load.", dir, dir))
			return nil
		},
	})
}
