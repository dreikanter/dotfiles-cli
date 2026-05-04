package dotfiles

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolver_SingleFile(t *testing.T) {
	home := t.TempDir()
	repo := t.TempDir()
	r := Resolver{RepoRoot: repo, Home: home}

	specs := r.Resolve(Manifest{"git": {"~/.gitconfig"}})
	require.Len(t, specs, 1)
	assert.Equal(t, "git", specs[0].Tool)
	assert.Equal(t, filepath.Join(home, ".gitconfig"), specs[0].LiveRoot)
	assert.Equal(t, filepath.Join(repo, "config/git/.gitconfig"), specs[0].SavedRoot)
	assert.False(t, specs[0].IsDir)
}

func TestResolver_MultipleFilesShareCommonRoot(t *testing.T) {
	home := t.TempDir()
	repo := t.TempDir()
	r := Resolver{RepoRoot: repo, Home: home}

	specs := r.Resolve(Manifest{"shell": {"~/.zshrc", "~/.zprofile"}})
	require.Len(t, specs, 2)
	assert.Equal(t, filepath.Join(repo, "config/shell/.zshrc"), specs[0].SavedRoot)
	assert.Equal(t, filepath.Join(repo, "config/shell/.zprofile"), specs[1].SavedRoot)
}

func TestResolver_DirectoryByTrailingSlash(t *testing.T) {
	home := t.TempDir()
	repo := t.TempDir()
	r := Resolver{RepoRoot: repo, Home: home}

	specs := r.Resolve(Manifest{"nvim": {"~/.config/nvim/"}})
	require.Len(t, specs, 1)
	assert.True(t, specs[0].IsDir)
	assert.Equal(t, filepath.Join(home, ".config/nvim"), specs[0].LiveRoot)
	assert.Equal(t, filepath.Join(repo, "config/nvim"), specs[0].SavedRoot)
}

func TestResolver_DirectoryDetectedOnDisk(t *testing.T) {
	home := t.TempDir()
	repo := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".config/nvim"), 0o755))
	r := Resolver{RepoRoot: repo, Home: home}

	specs := r.Resolve(Manifest{"nvim": {"~/.config/nvim"}})
	require.Len(t, specs, 1)
	assert.True(t, specs[0].IsDir, "directory on disk should be classified as dir without trailing slash")
}

func TestResolver_MultiTool_DeterministicOrder(t *testing.T) {
	home := t.TempDir()
	repo := t.TempDir()
	r := Resolver{RepoRoot: repo, Home: home}

	specs := r.Resolve(Manifest{
		"zzz": {"~/.zfoo"},
		"aaa": {"~/.afoo"},
	})
	require.Len(t, specs, 2)
	assert.Equal(t, "aaa", specs[0].Tool)
	assert.Equal(t, "zzz", specs[1].Tool)
}

func TestExpand_File(t *testing.T) {
	es, err := ExpandUnion(Spec{Tool: "git", LiveRoot: "/x/.gitconfig", SavedRoot: "/r/config/git/.gitconfig"})
	require.NoError(t, err)
	require.Len(t, es, 1)
	assert.Equal(t, "/x/.gitconfig", es[0].Live)
	assert.Equal(t, "/r/config/git/.gitconfig", es[0].Saved)
}

func TestExpand_DirUnion(t *testing.T) {
	home := t.TempDir()
	repo := t.TempDir()
	liveRoot := filepath.Join(home, ".config/nvim")
	savedRoot := filepath.Join(repo, "config/nvim")

	require.NoError(t, os.MkdirAll(filepath.Join(liveRoot, "lua"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(liveRoot, "init.lua"), []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(liveRoot, "lua/plugins.lua"), []byte("y"), 0o644))

	require.NoError(t, os.MkdirAll(savedRoot, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(savedRoot, "extra.lua"), []byte("z"), 0o644))

	es, err := ExpandUnion(Spec{Tool: "nvim", LiveRoot: liveRoot, SavedRoot: savedRoot, IsDir: true})
	require.NoError(t, err)
	require.Len(t, es, 3)
	rels := []string{}
	for _, e := range es {
		rel, _ := filepath.Rel(liveRoot, e.Live)
		rels = append(rels, rel)
	}
	assert.Equal(t, []string{"extra.lua", "init.lua", "lua/plugins.lua"}, rels)
}

func TestCommonDirPrefix(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{[]string{"/home/a"}, "/home/a"},
		{[]string{"/home/a", "/home/a"}, "/home/a"},
		{[]string{"/home/a/x", "/home/a/y"}, "/home/a"},
		{[]string{"/home/a/x", "/home/b/y"}, "/home"},
		{[]string{"/x", "/y"}, "/"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, commonDirPrefix(c.in), c.in)
	}
}
