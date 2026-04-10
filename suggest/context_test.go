package suggest

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGatherContext_NoGitRepo(t *testing.T) {
	t.Parallel()

	// Use a temp dir with no .git — should return "no-repo".
	ctx := GatherContext(t.TempDir(), TurnData{})
	assert.Equal(t, "no-repo", ctx.GitStatus)
	assert.Empty(t, ctx.ModifiedFiles)
	assert.Empty(t, ctx.LastTool)
	assert.False(t, ctx.LastError)
}

func TestGatherContext_CleanRepo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	gitInit(t, dir)

	ctx := GatherContext(dir, TurnData{})
	assert.Equal(t, "clean", ctx.GitStatus)
	assert.Empty(t, ctx.ModifiedFiles)
}

func TestGatherContext_DirtyRepo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	gitInit(t, dir)

	// Create an untracked file to make the repo dirty.
	writeFile(t, dir, "example.txt", "hello")

	ctx := GatherContext(dir, TurnData{})
	assert.Equal(t, "dirty", ctx.GitStatus)
	assert.Contains(t, ctx.ModifiedFiles, "example.txt")
}

func TestGatherContext_ExtractsLastTool(t *testing.T) {
	t.Parallel()

	turn := TurnData{
		ToolResults: []ToolResult{
			{ToolName: "Read", IsError: false},
			{ToolName: "Bash", IsError: true},
		},
	}

	ctx := GatherContext(t.TempDir(), turn)
	assert.Equal(t, "Bash", ctx.LastTool)
	assert.True(t, ctx.LastError)
}

func TestGatherContext_EmptyToolResults(t *testing.T) {
	t.Parallel()

	ctx := GatherContext(t.TempDir(), TurnData{})
	assert.Empty(t, ctx.LastTool)
	assert.False(t, ctx.LastError)
}

// gitInit creates a git repo in dir with a clean initial commit.
func gitInit(t *testing.T, dir string) {
	t.Helper()
	run(t, dir, "git", "init")
	run(t, dir, "git", "config", "user.email", "test@test.com")
	run(t, dir, "git", "config", "user.name", "Test")
	writeFile(t, dir, ".gitkeep", "")
	run(t, dir, "git", "add", ".gitkeep")
	run(t, dir, "git", "commit", "-m", "init")
}

func run(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("run %s %v: %v", name, args, err)
	}
}

func writeFile(t *testing.T, dir, name string, content ...string) {
	t.Helper()
	path := dir + "/" + name
	data := ""
	if len(content) > 0 {
		data = content[0]
	}
	if err := exec.Command("sh", "-c", "printf '%s' '"+data+"' > '"+path+"'").Run(); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
