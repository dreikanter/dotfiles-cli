package dotfiles

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scenario builds a fake home + repo with a manifest matching the live testdata
// shape. It returns the home dir, repo dir, and resolved specs.
func scenario(t *testing.T) (home, repo string, specs []Spec) {
	t.Helper()
	home = t.TempDir()
	repo = t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[user]\n\tname = Alice\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".zshrc"), []byte("export A=1\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".config/nvim/lua"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".config/nvim/init.lua"), []byte("init\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".config/nvim/lua/plugins.lua"), []byte("plugins\n"), 0o644))

	r := Resolver{RepoRoot: repo, Home: home}
	specs = r.Resolve(Manifest{
		"git":   {"~/.gitconfig"},
		"shell": {"~/.zshrc"},
		"nvim":  {"~/.config/nvim/"},
	})
	return
}

func TestSync_Save(t *testing.T) {
	home, repo, specs := scenario(t)
	_ = home

	res, err := Sync(specs, DirSave, Options{})
	require.NoError(t, err)
	assert.Equal(t, 0, res.Errors)
	assert.Equal(t, 4, res.Copied)

	mustEqualFile(t, filepath.Join(repo, "config/git/.gitconfig"), "[user]\n\tname = Alice\n")
	mustEqualFile(t, filepath.Join(repo, "config/shell/.zshrc"), "export A=1\n")
	mustEqualFile(t, filepath.Join(repo, "config/nvim/init.lua"), "init\n")
	mustEqualFile(t, filepath.Join(repo, "config/nvim/lua/plugins.lua"), "plugins\n")
}

func TestSync_Apply(t *testing.T) {
	home, repo, specs := scenario(t)

	// populate the dotfile mirror from local first
	_, err := Sync(specs, DirSave, Options{})
	require.NoError(t, err)

	// wipe locals and edit the mirror, then apply pulls everything back
	require.NoError(t, os.RemoveAll(filepath.Join(home, ".gitconfig")))
	require.NoError(t, os.RemoveAll(filepath.Join(home, ".config/nvim")))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "config/git/.gitconfig"), []byte("changed\n"), 0o644))

	res, err := Sync(specs, DirApply, Options{})
	require.NoError(t, err)
	assert.Equal(t, 0, res.Errors)
	mustEqualFile(t, filepath.Join(home, ".gitconfig"), "changed\n")
	mustEqualFile(t, filepath.Join(home, ".config/nvim/init.lua"), "init\n")
}

func TestSync_DryRun(t *testing.T) {
	_, repo, specs := scenario(t)
	res, err := Sync(specs, DirSave, Options{DryRun: true})
	require.NoError(t, err)
	assert.Equal(t, 4, res.Copied)
	_, err = os.Stat(filepath.Join(repo, "config/git/.gitconfig"))
	assert.True(t, os.IsNotExist(err), "dry-run should not write files")
}

func TestSync_Verbose(t *testing.T) {
	_, _, specs := scenario(t)
	var buf bytes.Buffer
	_, err := Sync(specs, DirSave, Options{Verbose: true, Out: &buf})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "OK ")
}

func TestSync_ReplacesSymlink(t *testing.T) {
	home, repo, specs := scenario(t)
	_ = home
	target := filepath.Join(repo, "config/git/.gitconfig")
	require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755))
	require.NoError(t, os.Symlink("/nonexistent", target))

	_, err := Sync(specs, DirSave, Options{})
	require.NoError(t, err)
	li, err := os.Lstat(target)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0), li.Mode()&os.ModeSymlink, "symlink should have been replaced with a regular file")
}

func TestSync_Prune(t *testing.T) {
	home, repo, specs := scenario(t)
	_ = home
	_, err := Sync(specs, DirSave, Options{})
	require.NoError(t, err)

	// add a stray file inside the dotfiles mirror
	stray := filepath.Join(repo, "config/nvim/leftover.lua")
	require.NoError(t, os.WriteFile(stray, []byte("stale"), 0o644))

	res, err := Sync(specs, DirSave, Options{Prune: true})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, res.Removed, 1)
	_, err = os.Stat(stray)
	assert.True(t, os.IsNotExist(err))
}

func TestSync_PruneApply(t *testing.T) {
	home, _, specs := scenario(t)
	_, err := Sync(specs, DirSave, Options{})
	require.NoError(t, err)

	// add a stray file in the local nvim dir
	stray := filepath.Join(home, ".config/nvim/leftover.lua")
	require.NoError(t, os.WriteFile(stray, []byte("stale"), 0o644))

	res, err := Sync(specs, DirApply, Options{Prune: true})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, res.Removed, 1)
	_, err = os.Stat(stray)
	assert.True(t, os.IsNotExist(err))
}

func TestSync_MissingSourceCountsAsError(t *testing.T) {
	home := t.TempDir()
	repo := t.TempDir()
	r := Resolver{RepoRoot: repo, Home: home}
	specs := r.Resolve(Manifest{"git": {"~/.gitconfig"}})

	res, err := Sync(specs, DirSave, Options{})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Errors)
	assert.Equal(t, 0, res.Copied)
}

func mustEqualFile(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	require.NoError(t, err, "read %s", path)
	assert.Equal(t, want, string(got), path)
}
