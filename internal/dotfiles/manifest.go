package dotfiles

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Manifest maps a tool name to a list of local paths to manage.
// Paths may use a leading "~" to refer to the user's home directory.
// A trailing "/" marks a path as a directory.
type Manifest map[string][]string

// LoadManifest reads and parses a JSON manifest from path.
func LoadManifest(path string) (Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
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
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// expand resolves a leading "~" and trims the directory marker suffix.
// Returns the cleaned absolute path.
func expand(p, home string) string {
	p = strings.TrimSuffix(p, "/*")
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
// the path as a directory via a trailing slash or "/*".
func hasDirMarker(original string) bool {
	return strings.HasSuffix(original, "/") || strings.HasSuffix(original, "/*")
}
