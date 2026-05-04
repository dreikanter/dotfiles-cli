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
// LiveRoot is the absolute path on the live filesystem where each tool reads
// the file; SavedRoot is the matching path inside the dotfiles repository.
type Spec struct {
	Tool      string
	LiveRoot  string
	SavedRoot string
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
func (r *Resolver) Resolve(m Manifest) []Spec {
	var out []Spec
	for _, tool := range m.Tools() {
		out = append(out, r.resolveTool(tool, m[tool])...)
	}
	return out
}

func (r *Resolver) resolveTool(tool string, paths []string) []Spec {
	type item struct {
		original string
		resolved string
		isDir    bool
	}
	items := make([]item, 0, len(paths))
	for _, p := range paths {
		resolved := expand(p, r.Home)
		isDir := hasDirMarker(p) || pathIsDir(resolved)
		items = append(items, item{p, resolved, isDir})
	}
	containers := make([]string, 0, len(items))
	for _, it := range items {
		if it.isDir {
			containers = append(containers, it.resolved)
		} else {
			containers = append(containers, filepath.Dir(it.resolved))
		}
	}
	prefix := commonDirPrefix(containers)
	specs := make([]Spec, 0, len(items))
	for _, it := range items {
		rel, err := filepath.Rel(prefix, it.resolved)
		if err != nil || rel == "" || strings.HasPrefix(rel, "..") {
			rel = filepath.Base(it.resolved)
		}
		specs = append(specs, Spec{
			Tool:      tool,
			LiveRoot:  it.resolved,
			SavedRoot: filepath.Join(r.RepoRoot, "config", tool, rel),
			IsDir:     it.isDir,
		})
	}
	return specs
}

// ExpandUnion turns a Spec into entries by taking the union of files on the
// live and saved sides. It is used for status reporting where files that
// exist on only one side still need to surface.
func ExpandUnion(s Spec) ([]Entry, error) {
	if !s.IsDir {
		return []Entry{{Tool: s.Tool, Live: s.LiveRoot, Saved: s.SavedRoot}}, nil
	}
	rels := map[string]struct{}{}
	if err := walkFiles(s.LiveRoot, rels); err != nil {
		return nil, err
	}
	if err := walkFiles(s.SavedRoot, rels); err != nil {
		return nil, err
	}
	return relsToEntries(s, rels), nil
}

// ExpandSource produces entries from files present on the source side of the
// given direction: the live tree for DirSave, the saved tree for DirInstall.
// Use this for copy/prune planning where only real source files matter.
func ExpandSource(s Spec, dir Direction) ([]Entry, error) {
	if !s.IsDir {
		return []Entry{{Tool: s.Tool, Live: s.LiveRoot, Saved: s.SavedRoot}}, nil
	}
	root := s.LiveRoot
	if dir == DirInstall {
		root = s.SavedRoot
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
			Live:  filepath.Join(s.LiveRoot, rel),
			Saved: filepath.Join(s.SavedRoot, rel),
		})
	}
	return out
}

// ExpandAllUnion concatenates ExpandUnion across specs.
func ExpandAllUnion(specs []Spec) ([]Entry, error) {
	var out []Entry
	for _, s := range specs {
		es, err := ExpandUnion(s)
		if err != nil {
			return nil, err
		}
		out = append(out, es...)
	}
	return out, nil
}

// ExpandAllSource concatenates ExpandSource across specs for the direction.
func ExpandAllSource(specs []Spec, dir Direction) ([]Entry, error) {
	var out []Entry
	for _, s := range specs {
		es, err := ExpandSource(s, dir)
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
		return nil
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

func pathIsDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
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
