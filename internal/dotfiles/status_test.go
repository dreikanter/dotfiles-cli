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

	// live missing (only saved copy exists)
	require.NoError(t, os.MkdirAll(filepath.Join(repo, "config/b"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "config/b/.livemiss"), []byte("x"), 0o644))

	// saved missing (only live copy exists)
	require.NoError(t, os.WriteFile(filepath.Join(home, ".savedmiss"), []byte("x"), 0o644))

	// live newer
	liveNewer := filepath.Join(home, ".livenewer")
	savedOlder := filepath.Join(repo, "config/d/.livenewer")
	require.NoError(t, os.MkdirAll(filepath.Dir(savedOlder), 0o755))
	require.NoError(t, os.WriteFile(liveNewer, []byte("new"), 0o644))
	require.NoError(t, os.WriteFile(savedOlder, []byte("old"), 0o644))
	old := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(savedOlder, old, old))

	// saved newer
	liveOlder := filepath.Join(home, ".savednewer")
	savedNewer := filepath.Join(repo, "config/e/.savednewer")
	require.NoError(t, os.MkdirAll(filepath.Dir(savedNewer), 0o755))
	require.NoError(t, os.WriteFile(liveOlder, []byte("old"), 0o644))
	require.NoError(t, os.WriteFile(savedNewer, []byte("new"), 0o644))
	require.NoError(t, os.Chtimes(liveOlder, old, old))

	// neither exists
	specs := r.Resolve(Manifest{
		"a": {"~/.synced"},
		"b": {"~/.livemiss"},
		"c": {"~/.savedmiss"},
		"d": {"~/.livenewer"},
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
	assert.Equal(t, StateLiveMissing, byTool["b"])
	assert.Equal(t, StateSavedMissing, byTool["c"])
	assert.Equal(t, StateLiveChanges, byTool["d"])
	assert.Equal(t, StateSavedChanges, byTool["e"])
	assert.Equal(t, StateNeitherExists, byTool["f"])
}

func TestStatus_FileEntryResolvingToDirectoryErrors(t *testing.T) {
	home := t.TempDir()
	repo := t.TempDir()
	// No trailing slash, so the entry is a file spec, but the live path is a
	// directory on disk. This must surface as an error, not a false in-sync.
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".config/nvim"), 0o755))
	r := Resolver{RepoRoot: repo, Home: home}
	specs := r.Resolve(Manifest{"nvim": {"~/.config/nvim"}})

	entries, err := Status(specs)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, StateError, entries[0].State)
	assert.Contains(t, entries[0].Error, "expected a file but found a directory")
}

func TestState_Display(t *testing.T) {
	cases := map[State]string{
		StateInSync:        "in sync",
		StateLiveMissing:   "not on disk",
		StateSavedMissing:  "not in repo",
		StateLiveChanges:   "live newer",
		StateSavedChanges:  "saved newer",
		StateNeitherExists: "both missing",
		StateError:         "error",
	}
	for s, want := range cases {
		assert.Equal(t, want, s.Display())
	}
}

func TestStatus_UnreadableLiveSurfacesError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission bits do not block root")
	}
	home := t.TempDir()
	repo := t.TempDir()
	r := Resolver{RepoRoot: repo, Home: home}

	// Live file inside an unreadable directory: stat fails with EACCES.
	unreadable := filepath.Join(home, "locked")
	require.NoError(t, os.MkdirAll(unreadable, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(unreadable, ".secret"), []byte("x"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(repo, "config/g"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "config/g/.secret"), []byte("x"), 0o644))
	require.NoError(t, os.Chmod(unreadable, 0o000))
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o755) })

	specs := r.Resolve(Manifest{"g": {"~/locked/.secret"}})
	entries, err := Status(specs)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, StateError, entries[0].State)
	assert.NotEmpty(t, entries[0].Error)
}
