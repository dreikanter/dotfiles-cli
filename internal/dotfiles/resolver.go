package dotfiles

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Spec describes one path declared in the manifest after resolution.
// LivePath is the absolute path on the live filesystem where each tool reads
// the file; SavedPath is the matching path inside the dotfiles repository.
type Spec struct {
	Tool      string
	LivePath  string
	SavedPath string
	IsDir     bool
}

// Entry is a concrete file pair: a single source/destination to copy or compare.
type Entry struct {
	Tool  string
	Live  string
	Saved string
}

// Resolver turns a Manifest into Specs grounded against a repository root and
// a home directory.
type Resolver struct {
	RepoRoot string
	Home     string
}

// Resolve produces one Spec per manifest entry. Specs are returned in tool
// order, then in declaration order within a tool.
func (r Resolver) Resolve(m Manifest) []Spec {
	var out []Spec
	for _, tool := range m.Tools() {
		out = append(out, r.resolveTool(tool, m[tool])...)
	}
	return out
}

func (r Resolver) resolveTool(tool string, paths []string) []Spec {
	// A trailing slash is the sole signal for directory-ness: resolution is a
	// pure function of the manifest, never of what currently exists on disk.
	specs := make([]Spec, len(paths))
	anchors := make([]string, len(paths))
	for i, p := range paths {
		live := expand(p, r.Home)
		isDir := hasDirMarker(p)
		specs[i] = Spec{Tool: tool, LivePath: live, IsDir: isDir}
		// A directory anchors the common-prefix search at itself; a file
		// anchors at its parent so siblings share a root.
		if isDir {
			anchors[i] = live
		} else {
			anchors[i] = filepath.Dir(live)
		}
	}
	prefix := commonDirPrefix(anchors)
	for i := range specs {
		rel, err := filepath.Rel(prefix, specs[i].LivePath)
		if err != nil || rel == "" || strings.HasPrefix(rel, "..") {
			rel = filepath.Base(specs[i].LivePath)
		}
		specs[i].SavedPath = filepath.Join(r.RepoRoot, "config", tool, rel)
	}
	return specs
}

// expandUnion turns a Spec into entries by taking the union of files on the
// live and saved sides. It is used for status reporting where files that
// exist on only one side still need to surface.
func expandUnion(s Spec) ([]Entry, error) {
	if !s.IsDir {
		return []Entry{{Tool: s.Tool, Live: s.LivePath, Saved: s.SavedPath}}, nil
	}
	rels := map[string]struct{}{}
	if err := walkFiles(s.LivePath, rels); err != nil {
		return nil, err
	}
	if err := walkFiles(s.SavedPath, rels); err != nil {
		return nil, err
	}
	return relsToEntries(s, rels), nil
}

// expandSource produces entries from files present on the source side of the
// given direction: the live tree for DirSave, the saved tree for DirInstall.
// Use this for copy/prune planning where only real source files matter.
func expandSource(s Spec, dir Direction) ([]Entry, error) {
	if !s.IsDir {
		return []Entry{{Tool: s.Tool, Live: s.LivePath, Saved: s.SavedPath}}, nil
	}
	root := s.LivePath
	if dir == DirInstall {
		root = s.SavedPath
	}
	rels := map[string]struct{}{}
	if err := walkFiles(root, rels); err != nil {
		return nil, err
	}
	return relsToEntries(s, rels), nil
}

func relsToEntries(s Spec, rels map[string]struct{}) []Entry {
	keys := make([]string, 0, len(rels))
	for k := range rels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]Entry, 0, len(keys))
	for _, rel := range keys {
		out = append(out, Entry{
			Tool:  s.Tool,
			Live:  filepath.Join(s.LivePath, rel),
			Saved: filepath.Join(s.SavedPath, rel),
		})
	}
	return out
}

// ExpandAllUnion concatenates expandUnion across specs.
func ExpandAllUnion(specs []Spec) ([]Entry, error) {
	var out []Entry
	for _, s := range specs {
		es, err := expandUnion(s)
		if err != nil {
			return nil, err
		}
		out = append(out, es...)
	}
	return out, nil
}

// ExpandAllSource concatenates expandSource across specs for the direction.
func ExpandAllSource(specs []Spec, dir Direction) ([]Entry, error) {
	var out []Entry
	for _, s := range specs {
		es, err := expandSource(s, dir)
		if err != nil {
			return nil, err
		}
		out = append(out, es...)
	}
	return out, nil
}

func walkFiles(root string, set map[string]struct{}) error {
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat %s: %w", root, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s: declared as a directory (trailing slash) but is a file", root)
	}
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk %s: %w", p, err)
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return fmt.Errorf("relative path %s under %s: %w", p, root, err)
		}
		set[rel] = struct{}{}
		return nil
	})
}

// commonDirPrefix returns the longest directory shared by all input paths.
// All inputs are expected to be cleaned absolute paths to directories.
func commonDirPrefix(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	if len(paths) == 1 {
		return paths[0]
	}
	prefix := paths[0]
	for _, p := range paths[1:] {
		prefix = sharedDir(prefix, p)
	}
	return prefix
}

func sharedDir(a, b string) string {
	aParts := splitPath(a)
	bParts := splitPath(b)
	n := len(aParts)
	if len(bParts) < n {
		n = len(bParts)
	}
	i := 0
	for i < n && aParts[i] == bParts[i] {
		i++
	}
	if i == 0 {
		return string(filepath.Separator)
	}
	return filepath.Join(aParts[:i]...)
}

func splitPath(p string) []string {
	p = filepath.Clean(p)
	var parts []string
	if filepath.IsAbs(p) {
		parts = append(parts, string(filepath.Separator))
	}
	for _, seg := range strings.Split(p, string(filepath.Separator)) {
		if seg != "" {
			parts = append(parts, seg)
		}
	}
	return parts
}
