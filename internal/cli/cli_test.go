package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runCLI executes the root cobra command with the given args, capturing stdout
// and returning any error. Persistent flags are reset between runs.
func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	resetFlags()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	return buf.String(), err
}

func resetFlags() {
	rootPath = ""
	manifestPath = ""
	dryRun = false
	verbose = false
	prune = false
	statusJSON = false
}

// findTestdataRepo locates testdata/sample-repo relative to the package.
func findTestdataRepo(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(file), "..", "..")
	src := filepath.Join(repoRoot, "testdata", "sample-repo")
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("sample-repo not found: %v", err)
	}
	return src
}

// stageRepoAndHome copies testdata/sample-repo into a fresh temp dir,
// creates a fake $HOME, and returns both paths. HOME is set via t.Setenv so
// resolveRoot/UserHomeDir picks it up.
func stageRepoAndHome(t *testing.T) (repo, home string) {
	t.Helper()
	src := findTestdataRepo(t)
	repo = t.TempDir()
	require.NoError(t, copyTree(src, repo))
	home = t.TempDir()
	t.Setenv("HOME", home)
	return repo, home
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	})
}

func TestCLI_ApplyAndStatus(t *testing.T) {
	repo, home := stageRepoAndHome(t)

	out, err := runCLI(t, "--root", repo, "apply")
	require.NoError(t, err, "apply failed: %s", out)
	assert.Contains(t, out, "dotfiles -> local environment")
	assert.Contains(t, out, "Files copied:")

	// All files should exist in the fake home now.
	for _, rel := range []string{".gitconfig", ".gitignore_global", ".zshrc", ".config/nvim/init.lua", ".config/nvim/lua/plugins.lua"} {
		_, err := os.Stat(filepath.Join(home, rel))
		assert.NoError(t, err, rel)
	}

	out, err = runCLI(t, "--root", repo, "status")
	require.NoError(t, err)
	assert.Contains(t, out, "files in sync")
}

func TestCLI_StatusJSON(t *testing.T) {
	repo, _ := stageRepoAndHome(t)

	out, err := runCLI(t, "--root", repo, "status", "--json")
	require.NoError(t, err)
	var entries []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &entries))
	require.NotEmpty(t, entries)
	assert.Contains(t, entries[0], "tool")
	assert.Contains(t, entries[0], "state")
}

func TestCLI_Config(t *testing.T) {
	repo, _ := stageRepoAndHome(t)

	out, err := runCLI(t, "--root", repo, "config")
	require.NoError(t, err)
	var mapping map[string]string
	require.NoError(t, json.Unmarshal([]byte(out), &mapping))
	require.NotEmpty(t, mapping)
	for dotfile, local := range mapping {
		assert.True(t, strings.HasPrefix(dotfile, repo), "dotfile path should be under repo: %s", dotfile)
		assert.NotEmpty(t, local)
	}
}

func TestCLI_SaveDryRun(t *testing.T) {
	repo, home := stageRepoAndHome(t)

	// populate the fake home from the mirror, then mutate one file locally
	_, err := runCLI(t, "--root", repo, "apply")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("local override\n"), 0o644))

	out, err := runCLI(t, "--root", repo, "save", "-n", "-v")
	require.NoError(t, err)
	assert.Contains(t, out, "[DRY RUN]")

	got, err := os.ReadFile(filepath.Join(repo, "config/git/.gitconfig"))
	require.NoError(t, err)
	assert.NotContains(t, string(got), "local override", "dry-run must not write")
}

func TestCLI_SaveAndApplyRoundtrip(t *testing.T) {
	repo, home := stageRepoAndHome(t)

	require.NoError(t, os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[user]\n  name = New\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".zshrc"), []byte("# updated\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".config/nvim/lua"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".config/nvim/init.lua"), []byte("-- new init\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".config/nvim/lua/plugins.lua"), []byte("-- new plugins\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".gitignore_global"), []byte(".DS_Store\n"), 0o644))

	_, err := runCLI(t, "--root", repo, "save")
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(repo, "config/git/.gitconfig"))
	require.NoError(t, err)
	assert.Equal(t, "[user]\n  name = New\n", string(got))

	require.NoError(t, os.RemoveAll(filepath.Join(home, ".config/nvim")))
	_, err = runCLI(t, "--root", repo, "apply")
	require.NoError(t, err)
	got, err = os.ReadFile(filepath.Join(home, ".config/nvim/init.lua"))
	require.NoError(t, err)
	assert.Equal(t, "-- new init\n", string(got))
}

func TestCLI_MissingManifest(t *testing.T) {
	emptyRoot := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	_, err := runCLI(t, "--root", emptyRoot, "status")
	require.Error(t, err)
}
