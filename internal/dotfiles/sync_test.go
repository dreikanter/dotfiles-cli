package dotfiles

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	_, repo, specs := scenario(t)

	res, err := Sync(specs, DirSave, Options{})
	require.NoError(t, err)
	assert.Equal(t, 0, res.Errors)
	assert.Equal(t, 4, res.Copied)

	mustEqualFile(t, filepath.Join(repo, "config/git/.gitconfig"), "[user]\n\tname = Alice\n")
	mustEqualFile(t, filepath.Join(repo, "config/shell/.zshrc"), "export A=1\n")
	mustEqualFile(t, filepath.Join(repo, "config/nvim/init.lua"), "init\n")
	mustEqualFile(t, filepath.Join(repo, "config/nvim/lua/plugins.lua"), "plugins\n")
}

func TestSync_Install(t *testing.T) {
	home, repo, specs := scenario(t)

	// populate the saved copy from installed first
	_, err := Sync(specs, DirSave, Options{})
	require.NoError(t, err)

	// wipe installed copies and edit the saved copy, then install pulls
	// everything back
	require.NoError(t, os.RemoveAll(filepath.Join(home, ".gitconfig")))
	require.NoError(t, os.RemoveAll(filepath.Join(home, ".config/nvim")))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "config/git/.gitconfig"), []byte("changed\n"), 0o644))

	res, err := Sync(specs, DirInstall, Options{})
	require.NoError(t, err)
	assert.Equal(t, 0, res.Errors)
	mustEqualFile(t, filepath.Join(home, ".gitconfig"), "changed\n")
	mustEqualFile(t, filepath.Join(home, ".config/nvim/init.lua"), "init\n")
}

// olderNewer stamps two paths with mod times an hour apart so the newer-wins
// guard has an unambiguous ordering regardless of filesystem timestamp
// granularity.
func olderNewer(t *testing.T, older, newer string) {
	t.Helper()
	past := time.Now().Add(-time.Hour)
	now := time.Now()
	require.NoError(t, os.Chtimes(older, past, past))
	require.NoError(t, os.Chtimes(newer, now, now))
}

func TestSync_SaveSkipsNewerSaved(t *testing.T) {
	home, repo, specs := scenario(t)
	_, err := Sync(specs, DirSave, Options{})
	require.NoError(t, err)

	live := filepath.Join(home, ".gitconfig")
	saved := filepath.Join(repo, "config/git/.gitconfig")
	// The saved copy diverges and is newer than the live file.
	require.NoError(t, os.WriteFile(saved, []byte("[user]\n\tname = Repo\n"), 0o644))
	olderNewer(t, live, saved)

	var buf bytes.Buffer
	res, err := Sync(specs, DirSave, Options{Out: &buf})
	require.NoError(t, err)
	assert.Equal(t, 0, res.Copied, "a newer saved copy must not be overwritten by an older live file")
	assert.Equal(t, 1, res.Skipped)
	assert.Equal(t, 3, res.Unchanged)
	assert.Contains(t, buf.String(), "skip ")
	assert.Contains(t, buf.String(), "saved is newer")
	// The newer saved copy is preserved verbatim.
	mustEqualFile(t, saved, "[user]\n\tname = Repo\n")
}

func TestSync_InstallSkipsNewerLive(t *testing.T) {
	home, repo, specs := scenario(t)
	_, err := Sync(specs, DirSave, Options{})
	require.NoError(t, err)

	live := filepath.Join(home, ".gitconfig")
	saved := filepath.Join(repo, "config/git/.gitconfig")
	// The live file diverges and is newer than the saved copy.
	require.NoError(t, os.WriteFile(live, []byte("[user]\n\tname = Local\n"), 0o644))
	olderNewer(t, saved, live)

	var buf bytes.Buffer
	res, err := Sync(specs, DirInstall, Options{Out: &buf})
	require.NoError(t, err)
	assert.Equal(t, 0, res.Copied, "a newer live file must not be overwritten by an older saved copy")
	assert.Equal(t, 1, res.Skipped)
	assert.Contains(t, buf.String(), "live is newer")
	// The newer live file is preserved verbatim.
	mustEqualFile(t, live, "[user]\n\tname = Local\n")
}

func TestSync_SaveCopiesObsoleteSavedAndSkipsNewer(t *testing.T) {
	home, repo, specs := scenario(t)
	_, err := Sync(specs, DirSave, Options{})
	require.NoError(t, err)

	// .gitconfig: live is newer -> obsolete saved copy gets overwritten.
	liveCfg := filepath.Join(home, ".gitconfig")
	savedCfg := filepath.Join(repo, "config/git/.gitconfig")
	require.NoError(t, os.WriteFile(liveCfg, []byte("[user]\n\tname = Local\n"), 0o644))
	olderNewer(t, savedCfg, liveCfg)

	// .zshrc: saved is newer -> preserved.
	liveRc := filepath.Join(home, ".zshrc")
	savedRc := filepath.Join(repo, "config/shell/.zshrc")
	require.NoError(t, os.WriteFile(savedRc, []byte("export A=2\n"), 0o644))
	olderNewer(t, liveRc, savedRc)

	res, err := Sync(specs, DirSave, Options{})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Copied)
	assert.Equal(t, 1, res.Skipped)
	assert.Equal(t, 2, res.Unchanged)
	mustEqualFile(t, savedCfg, "[user]\n\tname = Local\n") // obsolete copy refreshed
	mustEqualFile(t, savedRc, "export A=2\n")              // newer copy preserved
}

func TestSync_SkipReportedInDryRun(t *testing.T) {
	home, repo, specs := scenario(t)
	_, err := Sync(specs, DirSave, Options{})
	require.NoError(t, err)

	saved := filepath.Join(repo, "config/git/.gitconfig")
	require.NoError(t, os.WriteFile(saved, []byte("[user]\n\tname = Repo\n"), 0o644))
	olderNewer(t, filepath.Join(home, ".gitconfig"), saved)

	res, err := Sync(specs, DirSave, Options{DryRun: true})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Skipped)
	assert.Equal(t, 0, res.Copied)
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

	// First sync writes everything; output should report "copy" for each file.
	var first bytes.Buffer
	res, err := Sync(specs, DirSave, Options{Verbose: true, Out: &first})
	require.NoError(t, err)
	assert.Equal(t, 4, res.Copied)
	assert.Equal(t, 0, res.Unchanged)
	assert.Contains(t, first.String(), "copy ")
	assert.NotContains(t, first.String(), "unchanged ")

	// Second sync finds identical files; verbose should surface them as "unchanged".
	var second bytes.Buffer
	res, err = Sync(specs, DirSave, Options{Verbose: true, Out: &second})
	require.NoError(t, err)
	assert.Equal(t, 0, res.Copied)
	assert.Equal(t, 4, res.Unchanged)
	assert.Contains(t, second.String(), "unchanged ")
	assert.NotContains(t, second.String(), "copy ")
}

func TestSync_DefaultOmitsUnchanged(t *testing.T) {
	_, _, specs := scenario(t)
	_, err := Sync(specs, DirSave, Options{})
	require.NoError(t, err)

	// All files match the mirror; default (non-verbose) output should be empty.
	var buf bytes.Buffer
	res, err := Sync(specs, DirSave, Options{Out: &buf})
	require.NoError(t, err)
	assert.Equal(t, 0, res.Copied)
	assert.Equal(t, 4, res.Unchanged)
	assert.Empty(t, buf.String())
}

func TestSync_DefaultPrintsChanges(t *testing.T) {
	home, _, specs := scenario(t)
	_, err := Sync(specs, DirSave, Options{})
	require.NoError(t, err)

	// Mutate one file so it is the only change; default output lists only that one.
	require.NoError(t, os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[user]\n\tname = Bob\n"), 0o644))

	var buf bytes.Buffer
	res, err := Sync(specs, DirSave, Options{Out: &buf})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Copied)
	assert.Equal(t, 3, res.Unchanged)
	assert.Contains(t, buf.String(), "copy ")
	assert.Contains(t, buf.String(), ".gitconfig")
	assert.NotContains(t, buf.String(), ".zshrc")
}

func TestSync_DryRunReportsChanges(t *testing.T) {
	home, repo, specs := scenario(t)
	_, err := Sync(specs, DirSave, Options{})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[user]\n\tname = Bob\n"), 0o644))

	var buf bytes.Buffer
	res, err := Sync(specs, DirSave, Options{DryRun: true, Out: &buf})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Copied)
	assert.Equal(t, 3, res.Unchanged)
	assert.Contains(t, buf.String(), "copy ")

	// Dry run must not write the change.
	got, err := os.ReadFile(filepath.Join(repo, "config/git/.gitconfig"))
	require.NoError(t, err)
	assert.Equal(t, "[user]\n\tname = Alice\n", string(got))
}

func TestSync_ReplacesSymlink(t *testing.T) {
	_, repo, specs := scenario(t)
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
	_, repo, specs := scenario(t)
	_, err := Sync(specs, DirSave, Options{})
	require.NoError(t, err)

	// add a stray file inside the saved repository copy
	stray := filepath.Join(repo, "config/nvim/leftover.lua")
	require.NoError(t, os.WriteFile(stray, []byte("stale"), 0o644))

	var buf bytes.Buffer
	res, err := Sync(specs, DirSave, Options{Prune: true, Out: &buf})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Removed)
	assert.Contains(t, buf.String(), "prune ")
	assert.Contains(t, buf.String(), stray)
	_, err = os.Stat(stray)
	assert.True(t, os.IsNotExist(err))
}

func TestSync_PruneInstall(t *testing.T) {
	home, _, specs := scenario(t)
	_, err := Sync(specs, DirSave, Options{})
	require.NoError(t, err)

	// add a stray file in the installed nvim dir
	stray := filepath.Join(home, ".config/nvim/leftover.lua")
	require.NoError(t, os.WriteFile(stray, []byte("stale"), 0o644))

	res, err := Sync(specs, DirInstall, Options{Prune: true})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, res.Removed, 1)
	_, err = os.Stat(stray)
	assert.True(t, os.IsNotExist(err))
}

func TestSync_DirMarkerOnFileErrors(t *testing.T) {
	home := t.TempDir()
	repo := t.TempDir()
	// Manifest marks the entry as a directory, but the live path is a file.
	require.NoError(t, os.WriteFile(filepath.Join(home, ".zshrc"), []byte("x"), 0o644))
	r := Resolver{RepoRoot: repo, Home: home}
	specs := r.Resolve(Manifest{"shell": {"~/.zshrc/"}})

	_, err := Sync(specs, DirSave, Options{})
	require.Error(t, err, "a directory-marked path that is actually a file must fail loudly")
	assert.Contains(t, err.Error(), "declared as a directory")
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
