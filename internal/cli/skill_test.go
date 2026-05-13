package cli

import (
	"encoding/json"
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
	out, err := runCLI(t, "skill", "--help")
	require.NoError(t, err)
	assert.Contains(t, out, "JSON output shape (stdout mode):")
	assert.Contains(t, out, `"name"`)
	assert.Contains(t, out, `"description"`)
	assert.Contains(t, out, `"body"`)
}
