package dotfiles

import (
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

	// installed missing (only saved copy exists)
	require.NoError(t, os.MkdirAll(filepath.Join(repo, "config/b"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "config/b/.instmiss"), []byte("x"), 0o644))

	// saved missing (only installed copy exists)
	require.NoError(t, os.WriteFile(filepath.Join(home, ".savedmiss"), []byte("x"), 0o644))

	// installed newer
	installedNewer := filepath.Join(home, ".instnewer")
	savedOlder := filepath.Join(repo, "config/d/.instnewer")
	require.NoError(t, os.MkdirAll(filepath.Dir(savedOlder), 0o755))
	require.NoError(t, os.WriteFile(installedNewer, []byte("new"), 0o644))
	require.NoError(t, os.WriteFile(savedOlder, []byte("old"), 0o644))
	old := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(savedOlder, old, old))

	// saved newer
	installedOlder := filepath.Join(home, ".savednewer")
	savedNewer := filepath.Join(repo, "config/e/.savednewer")
	require.NoError(t, os.MkdirAll(filepath.Dir(savedNewer), 0o755))
	require.NoError(t, os.WriteFile(installedOlder, []byte("old"), 0o644))
	require.NoError(t, os.WriteFile(savedNewer, []byte("new"), 0o644))
	require.NoError(t, os.Chtimes(installedOlder, old, old))

	// neither exists
	specs := r.Resolve(Manifest{
		"a": {"~/.synced"},
		"b": {"~/.instmiss"},
		"c": {"~/.savedmiss"},
		"d": {"~/.instnewer"},
		"e": {"~/.savednewer"},
		"f": {"~/.absent"},
	})

	entries, err := Status(specs)
	require.NoError(t, err)
	byTool := map[string]State{}
	for _, e := range entries {
		byTool[e.Tool] = e.State
	}
	assert.Equal(t, StateInSync, byTool["a"])
	assert.Equal(t, StateInstalledMissing, byTool["b"])
	assert.Equal(t, StateSavedMissing, byTool["c"])
	assert.Equal(t, StateInstalledChanges, byTool["d"])
	assert.Equal(t, StateSavedChanges, byTool["e"])
	assert.Equal(t, StateNeitherExists, byTool["f"])
}
