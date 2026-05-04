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

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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
	jsonOutput = false
	filterTool = ""
	filterFiles = nil
	forceInit = false
	resetCobraFlagState(rootCmd)
}

// resetCobraFlagState clears the Changed bit on every flag of cmd and its
// subcommands. Cobra evaluates flag groups (mutually-exclusive, etc.) against
// Changed, so without this prior runs would bleed into later ones.
func resetCobraFlagState(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
	for _, sub := range cmd.Commands() {
		resetCobraFlagState(sub)
	}
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
	assert.Contains(t, out, "Copied:")

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
	var resp struct {
		Entries []map[string]any `json:"entries"`
		Summary struct {
			Total    int `json:"total"`
			Unsynced int `json:"unsynced"`
		} `json:"summary"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	require.NotEmpty(t, resp.Entries)
	assert.Contains(t, resp.Entries[0], "tool")
	assert.Contains(t, resp.Entries[0], "state")
	assert.Equal(t, len(resp.Entries), resp.Summary.Total)
}

func TestCLI_ConfigPlainText(t *testing.T) {
	repo, home := stageRepoAndHome(t)

	out, err := runCLI(t, "--root", repo, "config")
	require.NoError(t, err)
	assert.Contains(t, out, "Root: "+repo)
	assert.Contains(t, out, "entries")
	assert.Contains(t, out, "git")
	assert.Contains(t, out, filepath.Join(home, ".gitconfig"))
	// Root header must precede the entries list.
	assert.True(t, strings.Index(out, "Root: ") < strings.Index(out, "git"),
		"root path must appear above the synced files list")
	// Root header is distinctively separated from the entries by a divider line.
	assert.Contains(t, out, strings.Repeat("-", 40))
	// Plain-text config no longer echoes the dotfile mirror path.
	assert.NotContains(t, out, "->")
	assert.NotContains(t, out, filepath.Join(repo, "config/git/.gitconfig"))
}

func TestCLI_ConfigJSON(t *testing.T) {
	repo, home := stageRepoAndHome(t)

	out, err := runCLI(t, "--root", repo, "config", "--json")
	require.NoError(t, err)
	var resp struct {
		Root    string `json:"root"`
		Entries []struct {
			Tool    string `json:"tool"`
			Local   string `json:"local"`
			Dotfile string `json:"dotfile"`
		} `json:"entries"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	assert.Equal(t, repo, resp.Root)
	require.NotEmpty(t, resp.Entries)
	for _, e := range resp.Entries {
		assert.NotEmpty(t, e.Tool)
		assert.True(t, strings.HasPrefix(e.Dotfile, repo), "dotfile under repo: %s", e.Dotfile)
		assert.True(t, strings.HasPrefix(e.Local, home), "local under home: %s", e.Local)
	}
}

func TestCLI_SaveDryRun(t *testing.T) {
	repo, home := stageRepoAndHome(t)

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

func TestCLI_SaveJSON(t *testing.T) {
	repo, home := stageRepoAndHome(t)

	_, err := runCLI(t, "--root", repo, "apply")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("changed\n"), 0o644))

	out, err := runCLI(t, "--root", repo, "save", "--json")
	require.NoError(t, err)
	var resp struct {
		Direction string `json:"direction"`
		DryRun    bool   `json:"dryRun"`
		Copied    int    `json:"copied"`
		Removed   int    `json:"removed"`
		Errors    int    `json:"errors"`
		Actions   []struct {
			Action  string `json:"action"`
			From    string `json:"from"`
			To      string `json:"to"`
			Path    string `json:"path"`
			Message string `json:"message"`
		} `json:"actions"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &resp), "JSON-only stdout: %s", out)
	assert.Equal(t, "save", resp.Direction)
	assert.False(t, resp.DryRun)
	assert.Equal(t, 0, resp.Errors)
	assert.NotZero(t, resp.Copied)
	require.NotEmpty(t, resp.Actions)
	assert.Equal(t, "copy", resp.Actions[0].Action)
	// JSON mode must not leak the plain-text header.
	assert.NotContains(t, out, "local environment ->")
}

func TestCLI_ApplyJSONDryRun(t *testing.T) {
	repo, _ := stageRepoAndHome(t)

	out, err := runCLI(t, "--root", repo, "apply", "--json", "-n", "-v")
	require.NoError(t, err)
	var resp struct {
		Direction string `json:"direction"`
		DryRun    bool   `json:"dryRun"`
		Actions   []map[string]any
	}
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	assert.Equal(t, "apply", resp.Direction)
	assert.True(t, resp.DryRun)
	// --verbose must be ignored in JSON mode (no "OK ..." lines).
	assert.NotContains(t, out, "\nOK ")
	assert.NotContains(t, out, "[DRY RUN]")
}

func TestCLI_SaveReportsOnlyChangedFiles(t *testing.T) {
	repo, home := stageRepoAndHome(t)

	// Seed the local home from the repo so the second save has nothing to write.
	_, err := runCLI(t, "--root", repo, "apply")
	require.NoError(t, err)

	// Modify a single file; save should report only it.
	require.NoError(t, os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("changed\n"), 0o644))

	out, err := runCLI(t, "--root", repo, "save")
	require.NoError(t, err)
	assert.Contains(t, out, "copy ")
	assert.Contains(t, out, ".gitconfig")
	// Untouched files must not appear in the default report.
	assert.NotContains(t, out, ".zshrc")
	assert.NotContains(t, out, "init.lua")
	assert.NotContains(t, out, "unchanged ")
	assert.Contains(t, out, "Copied: 1")
	assert.Contains(t, out, "unchanged: 4")
}

func TestCLI_SaveDryRunReportsChanges(t *testing.T) {
	repo, home := stageRepoAndHome(t)
	_, err := runCLI(t, "--root", repo, "apply")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("changed\n"), 0o644))

	out, err := runCLI(t, "--root", repo, "save", "-n")
	require.NoError(t, err)
	assert.Contains(t, out, "[DRY RUN]")
	assert.Contains(t, out, "copy ")
	assert.Contains(t, out, ".gitconfig")
	// Other files match — they must not show up in the dry-run report.
	assert.NotContains(t, out, ".zshrc")

	// Dry run must not write the change.
	got, err := os.ReadFile(filepath.Join(repo, "config/git/.gitconfig"))
	require.NoError(t, err)
	assert.NotEqual(t, "changed\n", string(got))
}

func TestCLI_SavePruneReportsDeletedFiles(t *testing.T) {
	repo, _ := stageRepoAndHome(t)
	_, err := runCLI(t, "--root", repo, "apply")
	require.NoError(t, err)

	stray := filepath.Join(repo, "config/nvim/leftover.lua")
	require.NoError(t, os.WriteFile(stray, []byte("stale"), 0o644))

	out, err := runCLI(t, "--root", repo, "save", "--prune")
	require.NoError(t, err)
	assert.Contains(t, out, "prune ")
	assert.Contains(t, out, stray)
	assert.Contains(t, out, "removed: 1")
}

func TestCLI_SaveJSONIncludesUnchanged(t *testing.T) {
	repo, _ := stageRepoAndHome(t)
	_, err := runCLI(t, "--root", repo, "apply")
	require.NoError(t, err)

	out, err := runCLI(t, "--root", repo, "save", "--json")
	require.NoError(t, err)
	var resp struct {
		Copied    int `json:"copied"`
		Unchanged int `json:"unchanged"`
		Errors    int `json:"errors"`
		Actions   []struct {
			Action string `json:"action"`
		} `json:"actions"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	assert.Equal(t, 0, resp.Copied)
	assert.NotZero(t, resp.Unchanged)
	require.NotEmpty(t, resp.Actions)
	for _, a := range resp.Actions {
		assert.Equal(t, "unchanged", a.Action)
	}
}

func TestCLI_StatusShowsToolName(t *testing.T) {
	repo, home := stageRepoAndHome(t)
	_, err := runCLI(t, "--root", repo, "apply")
	require.NoError(t, err)

	// Make one file out of sync so it surfaces in the report.
	require.NoError(t, os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("local edit\n"), 0o644))

	out, err := runCLI(t, "--root", repo, "status")
	require.NoError(t, err)
	// The report row should include the tool name alongside the path.
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, ".gitconfig") {
			assert.Contains(t, line, "git", "tool name should appear: %q", line)
			return
		}
	}
	t.Fatalf("no .gitconfig row in status output:\n%s", out)
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

func TestCLI_MissingManifestJSON(t *testing.T) {
	emptyRoot := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	out, err := runCLI(t, "--root", emptyRoot, "status", "--json")
	// In JSON mode the command writes the envelope and returns errSilent so
	// Execute can exit non-zero without further output.
	require.ErrorIs(t, err, errSilent)
	var env struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &env), "JSON envelope: %s", out)
	assert.NotEmpty(t, env.Error.Message)
}

func TestCLI_SaveWithToolFilter(t *testing.T) {
	repo, home := stageRepoAndHome(t)
	_, err := runCLI(t, "--root", repo, "apply")
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("git changed\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".zshrc"), []byte("shell changed\n"), 0o644))

	out, err := runCLI(t, "--root", repo, "save", "--tool", "git")
	require.NoError(t, err)
	assert.Contains(t, out, "[tool=git]")

	got, err := os.ReadFile(filepath.Join(repo, "config/git/.gitconfig"))
	require.NoError(t, err)
	assert.Equal(t, "git changed\n", string(got))

	// shell tool was excluded; repo copy must still match the original.
	got, err = os.ReadFile(filepath.Join(repo, "config/shell/.zshrc"))
	require.NoError(t, err)
	assert.NotContains(t, string(got), "shell changed")
}

func TestCLI_SaveWithToolAndFileFilter(t *testing.T) {
	repo, home := stageRepoAndHome(t)
	_, err := runCLI(t, "--root", repo, "apply")
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("only this\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".gitignore_global"), []byte("not synced\n"), 0o644))

	out, err := runCLI(t, "--root", repo, "save", "--tool", "git", "--file", filepath.Join(home, ".gitconfig"))
	require.NoError(t, err)
	assert.Contains(t, out, "[tool=git, files=1]")

	got, err := os.ReadFile(filepath.Join(repo, "config/git/.gitconfig"))
	require.NoError(t, err)
	assert.Equal(t, "only this\n", string(got))

	got, err = os.ReadFile(filepath.Join(repo, "config/git/.gitignore_global"))
	require.NoError(t, err)
	assert.NotContains(t, string(got), "not synced")
}

func TestCLI_SaveJSONWithFilter(t *testing.T) {
	repo, home := stageRepoAndHome(t)
	_, err := runCLI(t, "--root", repo, "apply")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("changed\n"), 0o644))

	out, err := runCLI(t, "--root", repo, "save", "--json", "--tool", "git", "--file", "~/.gitconfig")
	require.NoError(t, err)
	var resp struct {
		Direction string `json:"direction"`
		Filter    *struct {
			Tool  string   `json:"tool"`
			Files []string `json:"files"`
		} `json:"filter"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	require.NotNil(t, resp.Filter)
	assert.Equal(t, "git", resp.Filter.Tool)
	assert.Equal(t, []string{filepath.Join(home, ".gitconfig")}, resp.Filter.Files)
}

func TestCLI_SaveJSONNoFilter_OmitsFilterField(t *testing.T) {
	repo, _ := stageRepoAndHome(t)
	_, err := runCLI(t, "--root", repo, "apply")
	require.NoError(t, err)
	out, err := runCLI(t, "--root", repo, "save", "--json")
	require.NoError(t, err)
	assert.NotContains(t, out, `"filter"`)
}

func TestCLI_FileWithoutTool(t *testing.T) {
	repo, home := stageRepoAndHome(t)
	_, err := runCLI(t, "--root", repo, "save", "--file", filepath.Join(home, ".gitconfig"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--file requires --tool")
}

func TestCLI_UnknownTool(t *testing.T) {
	repo, _ := stageRepoAndHome(t)
	_, err := runCLI(t, "--root", repo, "save", "--tool", "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `tool "missing" not in manifest`)
}

func TestCLI_FileNotDeclared(t *testing.T) {
	repo, home := stageRepoAndHome(t)
	_, err := runCLI(t, "--root", repo, "save", "--tool", "git", "--file", filepath.Join(home, ".zshrc"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not declared by tool")
}

func TestCLI_FileAndPruneMutuallyExclusive(t *testing.T) {
	repo, home := stageRepoAndHome(t)
	_, err := runCLI(t, "--root", repo, "save", "--tool", "git", "--file", filepath.Join(home, ".gitconfig"), "--prune")
	require.Error(t, err)
	// Cobra phrases this as "if any flags in the group [file prune] are set
	// none of the others can be" — match the stable substring.
	assert.Contains(t, err.Error(), "[file prune]")
}

func TestCLI_PruneWithToolAllowed(t *testing.T) {
	repo, _ := stageRepoAndHome(t)
	_, err := runCLI(t, "--root", repo, "apply")
	require.NoError(t, err)
	_, err = runCLI(t, "--root", repo, "save", "--tool", "git", "--prune")
	require.NoError(t, err)
}

func TestCLI_StatusWithFilter(t *testing.T) {
	repo, _ := stageRepoAndHome(t)
	_, err := runCLI(t, "--root", repo, "apply")
	require.NoError(t, err)

	out, err := runCLI(t, "--root", repo, "status", "--tool", "git")
	require.NoError(t, err)
	assert.Contains(t, out, "files in sync")
}

func TestCLI_StatusJSONWithFilter(t *testing.T) {
	repo, _ := stageRepoAndHome(t)
	out, err := runCLI(t, "--root", repo, "status", "--json", "--tool", "git")
	require.NoError(t, err)
	var resp struct {
		Entries []struct {
			Tool string `json:"tool"`
		} `json:"entries"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	require.NotEmpty(t, resp.Entries)
	for _, e := range resp.Entries {
		assert.Equal(t, "git", e.Tool)
	}
}

func TestCLI_ConfigWithFilter(t *testing.T) {
	repo, _ := stageRepoAndHome(t)
	out, err := runCLI(t, "--root", repo, "config", "--tool", "git")
	require.NoError(t, err)
	assert.Contains(t, out, ".gitconfig")
	assert.NotContains(t, out, ".zshrc")
}

func TestCLI_ConfigJSONWithFilter(t *testing.T) {
	repo, _ := stageRepoAndHome(t)
	out, err := runCLI(t, "--root", repo, "config", "--json", "--tool", "git")
	require.NoError(t, err)
	var resp struct {
		Entries []struct {
			Tool string `json:"tool"`
		} `json:"entries"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &resp))
	require.NotEmpty(t, resp.Entries)
	for _, e := range resp.Entries {
		assert.Equal(t, "git", e.Tool)
	}
}

func TestCLI_FilterErrorJSON(t *testing.T) {
	repo, _ := stageRepoAndHome(t)
	out, err := runCLI(t, "--root", repo, "save", "--json", "--tool", "missing")
	require.ErrorIs(t, err, errSilent)
	var env struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &env))
	assert.Contains(t, env.Error.Message, `tool "missing" not in manifest`)
}
