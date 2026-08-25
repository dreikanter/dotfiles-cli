package dotfiles

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Manifest maps a tool name to a list of local paths to manage.
// Paths may use a leading "~" to refer to the user's home directory.
// A trailing "/" marks a path as a directory; it is the sole signal for
// directory-ness. Without it the path is always treated as a single file,
// regardless of what exists on disk.
type Manifest map[string][]string

// LoadManifest reads and parses a JSON manifest from path.
func LoadManifest(path string) (Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	for tool, paths := range m {
		if tool == "" {
			return nil, fmt.Errorf("parse %s: empty tool name", path)
		}
		if len(paths) == 0 {
			return nil, fmt.Errorf("parse %s: tool %q has no paths", path, tool)
		}
	}
	return m, nil
}

// Tools returns the manifest's tool names in deterministic order.
func (m Manifest) Tools() []string {
	return slices.Sorted(maps.Keys(m))
}

// Expand resolves a manifest path to an absolute filesystem path. It expands
// a leading "~" and strips any trailing directory marker.
func Expand(p, home string) string {
	p = strings.TrimRight(p, "/")
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		p = filepath.Join(home, p[2:])
	}
	return filepath.Clean(p)
}

// hasDirMarker reports whether the original manifest entry explicitly marks
// the path as a directory via a trailing slash.
func hasDirMarker(original string) bool {
	return strings.HasSuffix(original, "/")
}

// ManifestPath converts abs to the storage form used in dotfiles.json.
// If abs is under home it is returned as "~/..." ; otherwise as-is.
func ManifestPath(abs, home string) string {
	rel, err := filepath.Rel(home, abs)
	if err == nil && !strings.HasPrefix(rel, "..") {
		return "~/" + rel
	}
	return abs
}

// SaveManifest writes m to path in the canonical dotfiles.json format:
// one tool per line with its paths as a compact JSON array, keys sorted.
func SaveManifest(path string, m Manifest) error {
	var buf bytes.Buffer
	buf.WriteString("{\n")
	tools := m.Tools()
	for i, tool := range tools {
		toolJSON, _ := json.Marshal(tool)
		buf.WriteString("  ")
		buf.Write(toolJSON)
		buf.WriteString(": [")
		for j, p := range m[tool] {
			if j > 0 {
				buf.WriteString(", ")
			}
			pJSON, _ := json.Marshal(p)
			buf.Write(pJSON)
		}
		buf.WriteByte(']')
		if i < len(tools)-1 {
			buf.WriteByte(',')
		}
		buf.WriteByte('\n')
	}
	buf.WriteString("}\n")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
