package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkill_StdoutMarkdown(t *testing.T) {
	out, err := runCLI(t, "skill")
	require.NoError(t, err)

	require.True(t, strings.HasPrefix(out, "---\nname: dotfiles\ndescription: Use when "),
		"expected frontmatter prefix, got: %s", out[:min(200, len(out))])
	require.Contains(t, out, "\n---\n\n")
	require.True(t, strings.HasSuffix(out, "\n"), "skill output must end in newline")

	assert.Contains(t, out, "## Global flags")
	assert.Contains(t, out, "## Commands")
	assert.Contains(t, out, "## Manifest")
	assert.Contains(t, out, "## JSON output and errors")
	assert.Contains(t, out, `{ "error": { "message": "..." } }`)
}

func TestSkill_StdoutDeterministic(t *testing.T) {
	a, err := runCLI(t, "skill")
	require.NoError(t, err)
	b, err := runCLI(t, "skill")
	require.NoError(t, err)
	assert.Equal(t, a, b, "skill output must be byte-identical across invocations")
}

func TestSkill_JSONShape(t *testing.T) {
	out, err := runCLI(t, "skill", "--json")
	require.NoError(t, err)

	var got struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Body        string `json:"body"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &got))

	assert.Equal(t, "dotfiles", got.Name)
	assert.True(t, strings.HasPrefix(got.Description, "Use when "),
		"description must start with 'Use when ': %q", got.Description)
	assert.NotEmpty(t, got.Body)
	assert.True(t, strings.HasSuffix(got.Body, "\n"), "body must end in newline")
}

func TestSkill_CommandsAlphabetized(t *testing.T) {
	out, err := runCLI(t, "skill")
	require.NoError(t, err)

	want := []string{"config", "init", "install", "save", "skill", "status"}
	lastIdx := -1
	for _, name := range want {
		marker := "| `" + name + "` |"
		idx := strings.Index(out, marker)
		require.GreaterOrEqual(t, idx, 0, "expected %q in commands table", name)
		assert.Greater(t, idx, lastIdx, "command %q out of alphabetic order", name)
		lastIdx = idx
	}
}

func TestSkill_HelpAndCompletionExcluded(t *testing.T) {
	out, err := runCLI(t, "skill")
	require.NoError(t, err)
	assert.NotContains(t, out, "| `help` |", "help command should not appear in the commands table")
	assert.NotContains(t, out, "| `completion` |", "completion command should not appear in the commands table")
}

func TestSkill_GlobalFlagsListed(t *testing.T) {
	out, err := runCLI(t, "skill")
	require.NoError(t, err)
	for _, flag := range []string{"--config", "--json", "--root"} {
		assert.Contains(t, out, "| `"+flag+"` |", "expected persistent flag %q in global flags table", flag)
	}
}

func TestSkill_HelpEmbedsJSONShape(t *testing.T) {
	// Inspect skillCmd.Long directly. Invoking `runCLI(t, "skill", "--help")`
	// would leak Cobra's help-flag state into later in-process tests.
	long := skillCmd.Long
	assert.Contains(t, long, "JSON output shape (stdout mode):")
	assert.Contains(t, long, "JSON output shape (install mode")
	assert.Contains(t, long, `"name"`)
	assert.Contains(t, long, `"description"`)
	assert.Contains(t, long, `"body"`)
	assert.Contains(t, long, `"actions"`)
	assert.Contains(t, long, "claude")
}

// installSandbox creates a temp HOME with .claude/skills/ pre-created and
// returns the absolute path the claude agent install would target.
func installSandbox(t *testing.T) (home, skillPath string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude", "skills"), 0o755))
	skillPath = filepath.Join(home, ".claude", "skills", "dotfiles", "SKILL.md")
	return home, skillPath
}

type installJSONResp struct {
	Actions []struct {
		Agent  string `json:"agent"`
		Path   string `json:"path"`
		Action string `json:"action"`
		Error  string `json:"error,omitempty"`
	} `json:"actions"`
}

func TestSkill_InstallCreate(t *testing.T) {
	_, skillPath := installSandbox(t)

	out, err := runCLI(t, "skill", "--install", "--agent=claude", "--json")
	require.NoError(t, err)

	var resp installJSONResp
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	require.Len(t, resp.Actions, 1)
	assert.Equal(t, "claude", resp.Actions[0].Agent)
	assert.Equal(t, "create", resp.Actions[0].Action)
	assert.Equal(t, skillPath, resp.Actions[0].Path)
	assert.Empty(t, resp.Actions[0].Error)

	got, err := os.ReadFile(skillPath)
	require.NoError(t, err)

	stdout, err := runCLI(t, "skill")
	require.NoError(t, err)
	assert.Equal(t, stdout, string(got), "installed file must equal `dotfiles skill` stdout")
}

func TestSkill_InstallSkipWhenIdentical(t *testing.T) {
	_, skillPath := installSandbox(t)

	// First run creates the file.
	_, err := runCLI(t, "skill", "--install", "--agent=claude")
	require.NoError(t, err)
	beforeStat, err := os.Stat(skillPath)
	require.NoError(t, err)
	beforeMtime := beforeStat.ModTime()

	// Second run with identical content: action=skip, exit 0, no write.
	out, err := runCLI(t, "skill", "--install", "--agent=claude", "--json")
	require.NoError(t, err)

	var resp installJSONResp
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	require.Len(t, resp.Actions, 1)
	assert.Equal(t, "skip", resp.Actions[0].Action)

	afterStat, err := os.Stat(skillPath)
	require.NoError(t, err)
	assert.Equal(t, beforeMtime, afterStat.ModTime(), "skip must not touch the file")
}

func TestSkill_InstallConflictWithoutForce(t *testing.T) {
	_, skillPath := installSandbox(t)

	_, err := runCLI(t, "skill", "--install", "--agent=claude")
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(skillPath, []byte("MUTATED\n"), 0o644))

	out, err := runCLI(t, "skill", "--install", "--agent=claude", "--json")
	require.Error(t, err, "conflict must yield non-zero exit")

	var resp installJSONResp
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	require.Len(t, resp.Actions, 1)
	assert.Equal(t, "conflict", resp.Actions[0].Action)

	// File is unchanged.
	got, err := os.ReadFile(skillPath)
	require.NoError(t, err)
	assert.Equal(t, "MUTATED\n", string(got), "conflict must not write")
}

func TestSkill_InstallOverwriteWithForce(t *testing.T) {
	_, skillPath := installSandbox(t)

	require.NoError(t, os.MkdirAll(filepath.Dir(skillPath), 0o755))
	require.NoError(t, os.WriteFile(skillPath, []byte("STALE\n"), 0o644))

	out, err := runCLI(t, "skill", "--install", "--agent=claude", "--force", "--json")
	require.NoError(t, err)

	var resp installJSONResp
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	require.Len(t, resp.Actions, 1)
	assert.Equal(t, "overwrite", resp.Actions[0].Action)

	got, err := os.ReadFile(skillPath)
	require.NoError(t, err)
	stdout, err := runCLI(t, "skill")
	require.NoError(t, err)
	assert.Equal(t, stdout, string(got))
}

func TestSkill_InstallDryRunDoesNotWrite(t *testing.T) {
	_, skillPath := installSandbox(t)

	out, err := runCLI(t, "skill", "--install", "--agent=claude", "--dry-run", "--json")
	require.NoError(t, err)

	var resp installJSONResp
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	require.Len(t, resp.Actions, 1)
	assert.Equal(t, "create", resp.Actions[0].Action, "dry-run must report the same action it would execute")

	_, statErr := os.Stat(skillPath)
	assert.True(t, os.IsNotExist(statErr), "dry-run must not write the file")
}

func TestSkill_InstallUnknownAgent(t *testing.T) {
	installSandbox(t)

	out, err := runCLI(t, "skill", "--install", "--agent=nopepants", "--json")
	require.Error(t, err)

	var env struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &env))
	assert.Contains(t, env.Error.Message, "unknown agent")
	assert.Contains(t, env.Error.Message, "claude")
}

func TestSkill_InstallAutoDetectNoAgents(t *testing.T) {
	// Fresh HOME without any agent skills directory present.
	home := t.TempDir()
	t.Setenv("HOME", home)

	out, err := runCLI(t, "skill", "--install", "--json")
	require.Error(t, err)

	var env struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &env))
	assert.Contains(t, env.Error.Message, "no supported agents detected")
	assert.Contains(t, env.Error.Message, "--agent")
	assert.Contains(t, env.Error.Message, "claude")
}

func TestSkill_InstallAutoDetectSingleAgent(t *testing.T) {
	_, skillPath := installSandbox(t)

	out, err := runCLI(t, "skill", "--install", "--json")
	require.NoError(t, err)

	var resp installJSONResp
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	require.Len(t, resp.Actions, 1)
	assert.Equal(t, "claude", resp.Actions[0].Agent)
	assert.Equal(t, "create", resp.Actions[0].Action)
	require.FileExists(t, skillPath)
}

func TestSkill_InstallAutoDetectMultiAgent(t *testing.T) {
	home, _ := installSandbox(t)
	fakeSkillsDir := filepath.Join(home, ".fake", "skills")
	fakePath := filepath.Join(fakeSkillsDir, "dotfiles", "SKILL.md")
	require.NoError(t, os.MkdirAll(fakeSkillsDir, 0o755))

	orig := agents
	t.Cleanup(func() { agents = orig })
	agents = append(append([]agentTarget{}, orig...), agentTarget{
		Name:    "fake",
		PathFor: func() (string, error) { return fakePath, nil },
		Detect: func() (bool, error) {
			_, err := os.Stat(fakeSkillsDir)
			if err != nil {
				return false, nil
			}
			return true, nil
		},
	})

	out, err := runCLI(t, "skill", "--install", "--json")
	require.NoError(t, err)

	var resp installJSONResp
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	require.Len(t, resp.Actions, 2)
	names := []string{resp.Actions[0].Agent, resp.Actions[1].Agent}
	assert.ElementsMatch(t, []string{"claude", "fake"}, names)

	require.FileExists(t, fakePath)
}

func TestSkill_ForceRequiresInstall(t *testing.T) {
	out, err := runCLI(t, "skill", "--force", "--json")
	require.Error(t, err)

	var env struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &env))
	assert.Contains(t, env.Error.Message, "require --install")
}

func TestSkill_InstallSkillsDirMissing(t *testing.T) {
	// Sandbox HOME without creating .claude/skills/.
	home := t.TempDir()
	t.Setenv("HOME", home)

	out, err := runCLI(t, "skill", "--install", "--agent=claude", "--json")
	require.Error(t, err, "missing skills dir must yield non-zero exit")

	var resp installJSONResp
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	require.Len(t, resp.Actions, 1)
	assert.Contains(t, resp.Actions[0].Error, "skills directory missing")
}
