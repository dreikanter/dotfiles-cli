package dotfiles

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dotfiles.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
		"git": ["~/.gitconfig", "~/.gitignore_global"],
		"nvim": ["~/.config/nvim/"]
	}`), 0o644))

	m, err := LoadManifest(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"~/.gitconfig", "~/.gitignore_global"}, m["git"])
	assert.Equal(t, []string{"~/.config/nvim/"}, m["nvim"])
	assert.Equal(t, []string{"git", "nvim"}, m.Tools())
}

func TestLoadManifest_NotFound(t *testing.T) {
	_, err := LoadManifest(filepath.Join(t.TempDir(), "missing.json"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, fs.ErrNotExist))
}

func TestLoadManifest_BadJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dotfiles.json")
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0o644))
	_, err := LoadManifest(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse")
}

func TestLoadManifest_RejectsEmptyTool(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dotfiles.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"git": []}`), 0o644))
	_, err := LoadManifest(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no paths")
}

func TestSaveManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dotfiles.json")
	m := Manifest{
		"git":  {"~/.gitconfig", "~/.gitignore_global"},
		"nvim": {"~/.config/nvim/"},
	}
	require.NoError(t, SaveManifest(path, m))

	// reload and verify round-trip
	got, err := LoadManifest(path)
	require.NoError(t, err)
	assert.Equal(t, m["git"], got["git"])
	assert.Equal(t, m["nvim"], got["nvim"])

	// keys are sorted, compact array format
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	s := string(raw)
	assert.True(t, strings.Index(s, `"git"`) < strings.Index(s, `"nvim"`), "keys must be sorted")
	assert.Contains(t, s, `["~/.gitconfig", "~/.gitignore_global"]`)
}

func TestManifestPath(t *testing.T) {
	home := "/home/alice"
	assert.Equal(t, "~/.gitconfig", ManifestPath("/home/alice/.gitconfig", home))
	assert.Equal(t, "~/.config/nvim", ManifestPath("/home/alice/.config/nvim", home))
	assert.Equal(t, "/etc/hosts", ManifestPath("/etc/hosts", home))
}

func TestExpand(t *testing.T) {
	home := "/home/alice"
	cases := []struct {
		in, want string
	}{
		{"~/.gitconfig", "/home/alice/.gitconfig"},
		{"~/.config/nvim/", "/home/alice/.config/nvim"},
		{"~", "/home/alice"},
		{"/etc/hosts", "/etc/hosts"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, expand(c.in, home), c.in)
	}
}

func TestHasDirMarker(t *testing.T) {
	assert.True(t, hasDirMarker("~/.config/nvim/"))
	assert.False(t, hasDirMarker("~/.gitconfig"))
}
