package dotfiles

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleSpecs(home string) []Spec {
	return []Spec{
		{Tool: "git", InstalledRoot: filepath.Join(home, ".gitconfig"), SavedRoot: "/repo/config/git/.gitconfig"},
		{Tool: "git", InstalledRoot: filepath.Join(home, ".gitignore_global"), SavedRoot: "/repo/config/git/.gitignore_global"},
		{Tool: "shell", InstalledRoot: filepath.Join(home, ".zshrc"), SavedRoot: "/repo/config/shell/.zshrc"},
		{Tool: "nvim", InstalledRoot: filepath.Join(home, ".config/nvim"), SavedRoot: "/repo/config/nvim", IsDir: true},
	}
}

func TestSelector_IsEmpty(t *testing.T) {
	assert.True(t, Selector{}.IsEmpty())
	assert.False(t, Selector{Tool: "git"}.IsEmpty())
	assert.False(t, Selector{Files: []string{"~/.gitconfig"}}.IsEmpty())
}

func TestSelector_Apply_NoFilter(t *testing.T) {
	home := "/home/alice"
	specs := sampleSpecs(home)
	got, err := Selector{}.Apply(specs, home)
	require.NoError(t, err)
	assert.Equal(t, specs, got)
}

func TestSelector_Apply_FileWithoutTool(t *testing.T) {
	home := "/home/alice"
	specs := sampleSpecs(home)
	_, err := Selector{Files: []string{"~/.gitconfig"}}.Apply(specs, home)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--file requires --tool")
}

func TestSelector_Apply_ToolNotInManifest(t *testing.T) {
	home := "/home/alice"
	specs := sampleSpecs(home)
	_, err := Selector{Tool: "missing"}.Apply(specs, home)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `tool "missing" not in manifest`)
}

func TestSelector_Apply_ToolOnly(t *testing.T) {
	home := "/home/alice"
	specs := sampleSpecs(home)
	got, err := Selector{Tool: "git"}.Apply(specs, home)
	require.NoError(t, err)
	require.Len(t, got, 2)
	for _, s := range got {
		assert.Equal(t, "git", s.Tool)
	}
}

func TestSelector_Apply_ToolAndFile(t *testing.T) {
	home := "/home/alice"
	specs := sampleSpecs(home)
	got, err := Selector{Tool: "git", Files: []string{"~/.gitconfig"}}.Apply(specs, home)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, filepath.Join(home, ".gitconfig"), got[0].InstalledRoot)
}

func TestSelector_Apply_FileTrailingSlashIgnored(t *testing.T) {
	home := "/home/alice"
	specs := sampleSpecs(home)
	got, err := Selector{Tool: "nvim", Files: []string{"~/.config/nvim/"}}.Apply(specs, home)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, filepath.Join(home, ".config/nvim"), got[0].InstalledRoot)
}

func TestSelector_Apply_FileNotDeclared(t *testing.T) {
	home := "/home/alice"
	specs := sampleSpecs(home)
	_, err := Selector{Tool: "git", Files: []string{"~/.zshrc"}}.Apply(specs, home)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not declared by tool")
	assert.Contains(t, err.Error(), `"git"`)
}

func TestSelector_Apply_FileMultiple(t *testing.T) {
	home := "/home/alice"
	specs := sampleSpecs(home)
	got, err := Selector{Tool: "git", Files: []string{"~/.gitconfig", "~/.gitignore_global"}}.Apply(specs, home)
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestSelector_Apply_FileDuplicatesDeduped(t *testing.T) {
	home := "/home/alice"
	specs := sampleSpecs(home)
	got, err := Selector{Tool: "git", Files: []string{"~/.gitconfig", "~/.gitconfig"}}.Apply(specs, home)
	require.NoError(t, err)
	assert.Len(t, got, 1)
}

func TestSelector_Apply_FileCWDRelative(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(cwd) }()
	require.NoError(t, os.Chdir(t.TempDir()))

	// Use Getwd's canonical form: on macOS t.TempDir() returns a path under
	// /var/folders/... which is a symlink to /private/var/folders/...; Getwd
	// reports the resolved form, which is what filepath.Abs joins against.
	dir, err := os.Getwd()
	require.NoError(t, err)

	specs := []Spec{{Tool: "x", InstalledRoot: filepath.Join(dir, "f"), SavedRoot: "/repo/config/x/f"}}
	got, err := Selector{Tool: "x", Files: []string{"f"}}.Apply(specs, "/nope")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, filepath.Join(dir, "f"), got[0].InstalledRoot)
}

func TestSelector_ResolvedFiles(t *testing.T) {
	home := "/home/alice"
	got := Selector{Tool: "git", Files: []string{"~/.gitconfig", "~/.gitconfig"}}.ResolvedFiles(home)
	assert.Equal(t, []string{filepath.Join(home, ".gitconfig")}, got)

	assert.Nil(t, Selector{}.ResolvedFiles(home))
}
