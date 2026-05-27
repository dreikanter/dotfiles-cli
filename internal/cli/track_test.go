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

// makeManifestRepo creates a minimal repo with a dotfiles.json and returns the
// repo path and a fake HOME.
func makeManifestRepo(t *testing.T, manifest string) (repo, home string) {
	t.Helper()
	repo = t.TempDir()
	home = t.TempDir()
	t.Setenv("HOME", home)
	require.NoError(t, os.WriteFile(filepath.Join(repo, "dotfiles.json"), []byte(manifest), 0o644))
	return repo, home
}

func readManifestJSON(t *testing.T, repo string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repo, "dotfiles.json"))
	require.NoError(t, err)
	return string(b)
}

// --- track ---

func TestTrack_File(t *testing.T) {
	repo, home := makeManifestRepo(t, "{}\n")
	cfg := filepath.Join(home, ".gitconfig")
	require.NoError(t, os.WriteFile(cfg, []byte("[user]\n"), 0o644))

	out, err := runCLI(t, "--root", repo, "track", "--tool", "git", cfg)
	require.NoError(t, err)
	assert.Contains(t, out, "tracked git ~/.gitconfig")

	raw := readManifestJSON(t, repo)
	assert.Contains(t, raw, `"~/.gitconfig"`)
	assert.Contains(t, raw, `"git"`)
}

func TestTrack_Directory(t *testing.T) {
	repo, home := makeManifestRepo(t, "{}\n")
	dir := filepath.Join(home, ".config", "nvim")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	out, err := runCLI(t, "--root", repo, "track", "--tool", "nvim", dir)
	require.NoError(t, err)
	assert.Contains(t, out, "tracked nvim ~/.config/nvim/")

	raw := readManifestJSON(t, repo)
	assert.Contains(t, raw, `"~/.config/nvim/"`)
}

func TestTrack_AppendsToExistingTool(t *testing.T) {
	repo, home := makeManifestRepo(t, `{"git": ["~/.gitconfig"]}`+"\n")
	ignore := filepath.Join(home, ".gitignore_global")
	require.NoError(t, os.WriteFile(ignore, []byte(""), 0o644))

	_, err := runCLI(t, "--root", repo, "track", "--tool", "git", ignore)
	require.NoError(t, err)

	raw := readManifestJSON(t, repo)
	assert.Contains(t, raw, `"~/.gitconfig"`)
	assert.Contains(t, raw, `"~/.gitignore_global"`)
}

func TestTrack_PathNotExist(t *testing.T) {
	repo, home := makeManifestRepo(t, "{}\n")
	_, err := runCLI(t, "--root", repo, "track", "--tool", "git", filepath.Join(home, "missing"))
	require.Error(t, err)
	// manifest must not have changed
	assert.Equal(t, "{}\n", readManifestJSON(t, repo))
}

func TestTrack_AlreadyTracked(t *testing.T) {
	repo, home := makeManifestRepo(t, `{"git": ["~/.gitconfig"]}`+"\n")
	cfg := filepath.Join(home, ".gitconfig")
	require.NoError(t, os.WriteFile(cfg, []byte(""), 0o644))

	out, err := runCLI(t, "--root", repo, "track", "--tool", "git", cfg)
	require.NoError(t, err, "already-tracked should exit 0")
	assert.Contains(t, out, "already-tracked")
}

func TestTrack_AlreadyTrackedUnderDifferentTool(t *testing.T) {
	repo, home := makeManifestRepo(t, `{"git": ["~/.gitconfig"]}`+"\n")
	cfg := filepath.Join(home, ".gitconfig")
	require.NoError(t, os.WriteFile(cfg, []byte(""), 0o644))

	out, err := runCLI(t, "--root", repo, "track", "--tool", "other", cfg)
	require.NoError(t, err, "already-tracked under a different tool should still exit 0")
	assert.Contains(t, out, "already-tracked git")
}

func TestTrack_DryRun(t *testing.T) {
	repo, home := makeManifestRepo(t, "{}\n")
	cfg := filepath.Join(home, ".gitconfig")
	require.NoError(t, os.WriteFile(cfg, []byte(""), 0o644))

	out, err := runCLI(t, "--root", repo, "track", "--tool", "git", "--dry-run", cfg)
	require.NoError(t, err)
	assert.Contains(t, out, "tracked git")
	assert.Equal(t, "{}\n", readManifestJSON(t, repo), "dry-run must not write manifest")
}

func TestTrack_JSON(t *testing.T) {
	repo, home := makeManifestRepo(t, "{}\n")
	cfg := filepath.Join(home, ".gitconfig")
	require.NoError(t, os.WriteFile(cfg, []byte(""), 0o644))

	out, err := runCLI(t, "--root", repo, "track", "--tool", "git", "--json", cfg)
	require.NoError(t, err)

	var resp trackResponse
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	assert.Equal(t, "tracked", resp.Action)
	assert.Equal(t, "git", resp.Tool)
	assert.Equal(t, "~/.gitconfig", resp.Path)
}

// --- untrack ---

func TestUntrack_File(t *testing.T) {
	repo, home := makeManifestRepo(t, `{"git": ["~/.gitconfig"]}`+"\n")
	cfg := filepath.Join(home, ".gitconfig")

	out, err := runCLI(t, "--root", repo, "untrack", "--tool", "git", cfg)
	require.NoError(t, err)
	assert.Contains(t, out, "untracked git ~/.gitconfig")

	raw := readManifestJSON(t, repo)
	assert.NotContains(t, raw, "git")
	assert.NotContains(t, raw, ".gitconfig")
}

func TestUntrack_RemovesToolWhenEmpty(t *testing.T) {
	repo, home := makeManifestRepo(t, `{"git": ["~/.gitconfig"], "shell": ["~/.zshrc"]}`+"\n")

	_, err := runCLI(t, "--root", repo, "untrack", "--tool", "git", filepath.Join(home, ".gitconfig"))
	require.NoError(t, err)

	raw := readManifestJSON(t, repo)
	assert.NotContains(t, raw, `"git"`)
	assert.Contains(t, raw, `"shell"`)
}

func TestUntrack_KeepsOtherPathsInTool(t *testing.T) {
	repo, home := makeManifestRepo(t, `{"git": ["~/.gitconfig", "~/.gitignore_global"]}`+"\n")

	_, err := runCLI(t, "--root", repo, "untrack", "--tool", "git", filepath.Join(home, ".gitconfig"))
	require.NoError(t, err)

	raw := readManifestJSON(t, repo)
	assert.NotContains(t, raw, `"~/.gitconfig"`)
	assert.Contains(t, raw, `"~/.gitignore_global"`)
}

func TestUntrack_NotFound(t *testing.T) {
	repo, home := makeManifestRepo(t, `{"git": ["~/.gitconfig"]}`+"\n")

	_, err := runCLI(t, "--root", repo, "untrack", "--tool", "git", filepath.Join(home, ".zshrc"))
	require.Error(t, err)
}

func TestUntrack_WrongToolHint(t *testing.T) {
	repo, home := makeManifestRepo(t, `{"git": ["~/.gitconfig"]}`+"\n")

	_, err := runCLI(t, "--root", repo, "untrack", "--tool", "shell", filepath.Join(home, ".gitconfig"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git")
	assert.Contains(t, err.Error(), "shell")
}

func TestUntrack_DryRun(t *testing.T) {
	const manifest = `{"git": ["~/.gitconfig"]}` + "\n"
	repo, home := makeManifestRepo(t, manifest)

	out, err := runCLI(t, "--root", repo, "untrack", "--tool", "git", "--dry-run", filepath.Join(home, ".gitconfig"))
	require.NoError(t, err)
	assert.Contains(t, out, "untracked git")
	assert.Equal(t, manifest, readManifestJSON(t, repo), "dry-run must not write manifest")
}

func TestUntrack_Purge(t *testing.T) {
	repo, home := makeManifestRepo(t, `{"git": ["~/.gitconfig"]}`+"\n")

	// create a saved copy
	savedDir := filepath.Join(repo, "config", "git")
	savedFile := filepath.Join(savedDir, ".gitconfig")
	require.NoError(t, os.MkdirAll(savedDir, 0o755))
	require.NoError(t, os.WriteFile(savedFile, []byte("[user]\n"), 0o644))

	out, err := runCLI(t, "--root", repo, "untrack", "--tool", "git", "--purge", filepath.Join(home, ".gitconfig"))
	require.NoError(t, err)
	assert.Contains(t, out, "purged")
	assert.Contains(t, out, savedFile)

	_, statErr := os.Stat(savedFile)
	assert.True(t, os.IsNotExist(statErr), "saved file should have been deleted")
}

func TestUntrack_PurgeDryRun(t *testing.T) {
	repo, home := makeManifestRepo(t, `{"git": ["~/.gitconfig"]}`+"\n")

	savedDir := filepath.Join(repo, "config", "git")
	savedFile := filepath.Join(savedDir, ".gitconfig")
	require.NoError(t, os.MkdirAll(savedDir, 0o755))
	require.NoError(t, os.WriteFile(savedFile, []byte("[user]\n"), 0o644))

	out, err := runCLI(t, "--root", repo, "untrack", "--tool", "git", "--purge", "--dry-run", filepath.Join(home, ".gitconfig"))
	require.NoError(t, err)
	assert.Contains(t, out, "purged")

	_, statErr := os.Stat(savedFile)
	assert.NoError(t, statErr, "dry-run must not delete saved file")
}

func TestUntrack_JSON(t *testing.T) {
	repo, home := makeManifestRepo(t, `{"git": ["~/.gitconfig"]}`+"\n")

	out, err := runCLI(t, "--root", repo, "untrack", "--tool", "git", "--json", filepath.Join(home, ".gitconfig"))
	require.NoError(t, err)

	var resp untrackResponse
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	assert.Equal(t, "untracked", resp.Action)
	assert.Equal(t, "git", resp.Tool)
	assert.True(t, strings.HasSuffix(resp.Path, ".gitconfig"))
}
