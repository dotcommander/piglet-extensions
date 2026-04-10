package skill

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
	sdk "github.com/dotcommander/piglet/sdk"
)

const Version = "0.2.0"

// Register wires all skill capabilities into the extension.
func Register(e *sdk.Extension) {
	store := new(*Store)

	e.OnInit(func(x *sdk.Extension) {
		base, err := xdg.ConfigDir()
		if err != nil {
			return
		}
		s := NewStore(filepath.Join(base, "skills"))
		if len(s.List()) == 0 {
			return
		}
		*store = s

		x.RegisterPromptSection(sdk.PromptSectionDef{
			Title:   "Skills",
			Content: "Available skills (call skill_load to use):\n" + FormatList(s.List(), FormatOpts{Indent: "- ", Separator: ": "}),
			Order:   25,
		})
	})

	e.RegisterTool(toolList(store))
	e.RegisterTool(toolLoad(store))
	e.RegisterTool(toolStatus(store))
	e.RegisterCommand(skillCommand(store, e))
	e.RegisterMessageHook(skillTrigger(store))
}

func toolList(store **Store) sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "skill_list",
		Description: "List available skills with descriptions and trigger keywords.",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Execute: func(_ context.Context, _ map[string]any) (*sdk.ToolResult, error) {
			s := *store
			if s == nil {
				return sdk.TextResult("No skills available."), nil
			}
			skills := s.List()
			if len(skills) == 0 {
				return sdk.TextResult("No skills available."), nil
			}
			return sdk.TextResult(FormatList(skills, FormatOpts{Indent: "- ", Separator: ": ", Triggers: true})), nil
		},
	}
}

func toolLoad(store **Store) sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "skill_load",
		Description: "Load a skill's full methodology and instructions by name.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string", "description": "Skill name"},
			},
			"required": []string{"name"},
		},
		Execute: func(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
			s := *store
			if s == nil {
				return sdk.ErrorResult("no skills available"), nil
			}
			name, _ := args["name"].(string)
			if name == "" {
				return sdk.ErrorResult("name is required"), nil
			}
			content, err := s.Load(name)
			if err != nil {
				return sdk.ErrorResult(fmt.Sprintf("skill %q not found", name)), nil
			}
			return sdk.TextResult(content), nil
		},
	}
}

func toolStatus(store **Store) sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "skill_status",
		Description: "Show skill extension status: version, skill count, and skills directory path.",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Execute: func(_ context.Context, _ map[string]any) (*sdk.ToolResult, error) {
			s := *store
			if s == nil {
				return sdk.TextResult(fmt.Sprintf("skill v%s\nNo skills loaded.", Version)), nil
			}
			skills := s.List()
			return sdk.TextResult(fmt.Sprintf("skill v%s\nSkills: %d loaded\nDirectory: %s", Version, len(skills), s.Dir())), nil
		},
	}
}

func skillCommand(store **Store, e *sdk.Extension) sdk.CommandDef {
	return sdk.CommandDef{
		Name:        "skill",
		Description: "List or load a skill",
		Handler: func(_ context.Context, args string) error {
			s := *store
			if s == nil {
				e.ShowMessage("No skills available.")
				return nil
			}
			arg := strings.TrimSpace(args)
			if arg == "" || arg == "list" {
				skills := s.List()
				if len(skills) == 0 {
					e.ShowMessage("No skills found in " + s.Dir())
					return nil
				}
				e.ShowMessage(FormatList(skills, FormatOpts{Prefix: "Available skills:\n", Indent: "  ", Separator: " — "}))
				return nil
			}
			content, err := s.Load(arg)
			if err != nil {
				e.ShowMessage(fmt.Sprintf("Skill %q not found. Run /skill list to see available skills.", arg))
				return nil
			}
			e.ShowMessage(fmt.Sprintf("# Skill: %s\n\n%s", arg, content))
			return nil
		},
	}
}

func skillTrigger(store **Store) sdk.MessageHookDef {
	return sdk.MessageHookDef{
		Name:     "skill-trigger",
		Priority: 500,
		OnMessage: func(_ context.Context, msg string) (string, error) {
			s := *store
			if s == nil {
				return "", nil
			}
			matches := s.Match(msg)
			if len(matches) == 0 {
				return "", nil
			}
			content, err := s.Load(matches[0].Name)
			if err != nil {
				return "", nil
			}
			return "# Skill: " + matches[0].Name + "\n\n" + content, nil
		},
	}
}
