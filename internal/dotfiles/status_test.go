package dotfiles

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatus_AllStates(t *testing.T) {
	home := t.TempDir()
	repo := t.TempDir()
	r := Resolver{RepoRoot: repo, Home: home}

	// in sync
	require.NoError(t, os.WriteFile(filepath.Join(home, ".synced"), []byte("same"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(repo, "config/a"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "config/a/.synced"), []byte("same"), 0o644))

	// local missing (only dotfile exists)
	require.NoError(t, os.MkdirAll(filepath.Join(repo, "config/b"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "config/b/.localmiss"), []byte("x"), 0o644))

	// dotfile missing (only local exists)
	require.NoError(t, os.WriteFile(filepath.Join(home, ".dotmiss"), []byte("x"), 0o644))

	// local newer
	localNewer := filepath.Join(home, ".localnewer")
	repoOlder := filepath.Join(repo, "config/d/.localnewer")
	require.NoError(t, os.MkdirAll(filepath.Dir(repoOlder), 0o755))
	require.NoError(t, os.WriteFile(localNewer, []byte("new"), 0o644))
	require.NoError(t, os.WriteFile(repoOlder, []byte("old"), 0o644))
	old := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(repoOlder, old, old))

	// dotfile newer
	localOlder := filepath.Join(home, ".dotnewer")
	repoNewer := filepath.Join(repo, "config/e/.dotnewer")
	require.NoError(t, os.MkdirAll(filepath.Dir(repoNewer), 0o755))
	require.NoError(t, os.WriteFile(localOlder, []byte("old"), 0o644))
	require.NoError(t, os.WriteFile(repoNewer, []byte("new"), 0o644))
	require.NoError(t, os.Chtimes(localOlder, old, old))

	// neither exists
	specs := r.Resolve(Manifest{
		"a": {"~/.synced"},
		"b": {"~/.localmiss"},
		"c": {"~/.dotmiss"},
		"d": {"~/.localnewer"},
		"e": {"~/.dotnewer"},
		"f": {"~/.absent"},
	})

	entries, err := Status(specs)
	require.NoError(t, err)
	byTool := map[string]State{}
	for _, e := range entries {
		byTool[e.Tool] = e.State
	}
	assert.Equal(t, StateInSync, byTool["a"])
	assert.Equal(t, StateLocalMissing, byTool["b"])
	assert.Equal(t, StateDotfileMissing, byTool["c"])
	assert.Equal(t, StateLocalChanges, byTool["d"])
	assert.Equal(t, StateDotfileChanges, byTool["e"])
	assert.Equal(t, StateNeitherExists, byTool["f"])
}

func TestConfigJSON(t *testing.T) {
	home := t.TempDir()
	repo := t.TempDir()
	r := Resolver{RepoRoot: repo, Home: home}
	specs := r.Resolve(Manifest{"git": {"~/.gitconfig"}})

	b, err := ConfigJSON(specs)
	require.NoError(t, err)
	var m map[string]string
	require.NoError(t, json.Unmarshal(b, &m))

	expectedDotfile := filepath.Join(repo, "config/git/.gitconfig")
	expectedLocal := filepath.Join(home, ".gitconfig")
	assert.Equal(t, expectedLocal, m[expectedDotfile])
}
