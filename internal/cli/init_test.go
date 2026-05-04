package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stageInitHome returns a fresh empty home directory and sets $HOME so
// resolveRoot picks it up. The caller passes its own --root.
func stageInitHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
}

func TestCLI_Init_CreatesArtifacts(t *testing.T) {
	requireGit(t)
	stageInitHome(t)
	root := filepath.Join(t.TempDir(), "new-repo")

	out, err := runCLI(t, "--root", root, "init")
	require.NoError(t, err, out)
	assert.Contains(t, out, "Initialized")

	manifest, err := os.ReadFile(filepath.Join(root, "dotfiles.json"))
	require.NoError(t, err)
	assert.Equal(t, "{}\n", string(manifest))

	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	require.NoError(t, err)
	assert.Contains(t, string(readme), "dotfiles-cli")
	assert.Contains(t, string(readme), "dotfiles --help")

	st, err := os.Stat(filepath.Join(root, ".git"))
	require.NoError(t, err)
	assert.True(t, st.IsDir())
}

func TestCLI_Init_RefusesIfManifestExists(t *testing.T) {
	requireGit(t)
	stageInitHome(t)
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "dotfiles.json"), []byte(`{"git":["~/.gitconfig"]}`), 0o644))

	_, err := runCLI(t, "--root", root, "init")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	got, _ := os.ReadFile(filepath.Join(root, "dotfiles.json"))
	assert.Contains(t, string(got), ".gitconfig", "must not overwrite without --force")
}

func TestCLI_Init_RefusesIfReadmeExists(t *testing.T) {
	requireGit(t)
	stageInitHome(t)
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("keep me"), 0o644))

	_, err := runCLI(t, "--root", root, "init")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	got, _ := os.ReadFile(filepath.Join(root, "README.md"))
	assert.Equal(t, "keep me", string(got))
}

func TestCLI_Init_ForceOverwrites(t *testing.T) {
	requireGit(t)
	stageInitHome(t)
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "dotfiles.json"), []byte(`{"git":["~/.gitconfig"]}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("old"), 0o644))

	_, err := runCLI(t, "--root", root, "init", "--force")
	require.NoError(t, err)

	manifest, _ := os.ReadFile(filepath.Join(root, "dotfiles.json"))
	assert.Equal(t, "{}\n", string(manifest))
	readme, _ := os.ReadFile(filepath.Join(root, "README.md"))
	assert.Contains(t, string(readme), "dotfiles --help")
}

func TestCLI_Init_ExistingGitLeftAlone(t *testing.T) {
	requireGit(t)
	stageInitHome(t)
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0o755))
	marker := filepath.Join(gitDir, "marker")
	require.NoError(t, os.WriteFile(marker, []byte("x"), 0o644))

	_, err := runCLI(t, "--root", root, "init")
	require.NoError(t, err)

	_, err = os.Stat(marker)
	assert.NoError(t, err, "existing .git must be left untouched")
}

func TestCLI_Init_DryRun(t *testing.T) {
	requireGit(t)
	stageInitHome(t)
	root := filepath.Join(t.TempDir(), "would-be-repo")

	out, err := runCLI(t, "--root", root, "init", "-n", "-v")
	require.NoError(t, err)
	assert.Contains(t, out, "[DRY RUN]")

	_, err = os.Stat(root)
	assert.True(t, os.IsNotExist(err), "dry-run must not create the directory")
}

func TestCLI_Init_JSON(t *testing.T) {
	requireGit(t)
	stageInitHome(t)
	root := filepath.Join(t.TempDir(), "json-repo")

	out, err := runCLI(t, "--root", root, "init", "--json")
	require.NoError(t, err)

	var resp struct {
		Root    string `json:"root"`
		DryRun  bool   `json:"dryRun"`
		Force   bool   `json:"force"`
		Actions []struct {
			Action  string `json:"action"`
			Path    string `json:"path"`
			Message string `json:"message"`
		} `json:"actions"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &resp), out)
	assert.Equal(t, root, resp.Root)
	assert.False(t, resp.DryRun)

	seen := map[string]string{}
	for _, a := range resp.Actions {
		seen[filepath.Base(a.Path)] = a.Action
	}
	assert.Equal(t, "create", seen["dotfiles.json"])
	assert.Equal(t, "create", seen["README.md"])
	assert.Equal(t, "git-init", seen[".git"])
}

func TestCLI_Init_JSONErrorEnvelope(t *testing.T) {
	requireGit(t)
	stageInitHome(t)
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "dotfiles.json"), []byte("{}"), 0o644))

	out, err := runCLI(t, "--root", root, "init", "--json")
	require.ErrorIs(t, err, errSilent)
	var env struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &env), out)
	assert.Contains(t, env.Error.Message, "already exists")
}
