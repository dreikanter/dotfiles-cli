package dotfiles

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Direction selects the copy direction for Sync.
type Direction int

const (
	// DirSave copies live files into the dotfiles repository.
	DirSave Direction = iota
	// DirInstall copies repository files to their live paths.
	DirInstall
)

// Options controls Sync behavior.
type Options struct {
	DryRun  bool
	Verbose bool
	Prune   bool
	Out     io.Writer
}

// Action records a single per-file effect of a Sync run for machine-readable
// output. Action is one of "copy", "unchanged", "prune", or "error".
type Action struct {
	Action  string `json:"action"`
	From    string `json:"from,omitempty"`
	To      string `json:"to,omitempty"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message,omitempty"`
}

// Result summarizes a Sync run.
type Result struct {
	Copied    int
	Unchanged int
	Removed   int
	Errors    int
	Actions   []Action
}

// Sync copies files for the given specs in the chosen direction. When Prune is
// set, files in the destination tree that are not declared by the manifest are
// removed.
func Sync(specs []Spec, dir Direction, opts Options) (Result, error) {
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	entries, err := ExpandAllSource(specs, dir)
	if err != nil {
		return Result{}, err
	}
	res := Result{Actions: []Action{}}
	for _, e := range entries {
		src, dst := e.Live, e.Saved
		if dir == DirInstall {
			src, dst = dst, src
		}
		changed, err := copyOne(src, dst, opts)
		if err != nil {
			res.Errors++
			res.Actions = append(res.Actions, Action{Action: "error", From: src, To: dst, Message: err.Error()})
			fmt.Fprintf(opts.Out, "error %s -> %s: %v\n", src, dst, err)
			continue
		}
		if changed {
			res.Copied++
			res.Actions = append(res.Actions, Action{Action: "copy", From: src, To: dst})
			fmt.Fprintf(opts.Out, "copy %s -> %s\n", src, dst)
			continue
		}
		res.Unchanged++
		res.Actions = append(res.Actions, Action{Action: "unchanged", From: src, To: dst})
		if opts.Verbose {
			fmt.Fprintf(opts.Out, "unchanged %s\n", dst)
		}
	}
	if opts.Prune {
		removed, actions, err := prune(specs, dir, opts)
		if err != nil {
			return res, err
		}
		res.Removed = removed
		res.Actions = append(res.Actions, actions...)
	}
	return res, nil
}

// copyOne returns whether the destination required a write. Identical contents
// (after a content compare) are reported as unchanged and skipped, even in
// dry-run mode, so the caller can distinguish "would change" from "would
// no-op".
func copyOne(src, dst string, opts Options) (bool, error) {
	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return false, fmt.Errorf("source missing: %s", src)
		}
		return false, fmt.Errorf("stat %s: %w", src, err)
	}
	if info.IsDir() {
		return false, fmt.Errorf("source is a directory: %s", src)
	}
	same, err := sameContents(src, dst, info)
	if err != nil {
		return false, err
	}
	if same {
		return false, nil
	}
	if opts.DryRun {
		return true, nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return false, fmt.Errorf("mkdir %s: %w", filepath.Dir(dst), err)
	}
	// Replace any existing symlink with a real file copy.
	if li, err := os.Lstat(dst); err == nil && li.Mode()&fs.ModeSymlink != 0 {
		if rmErr := os.Remove(dst); rmErr != nil {
			return false, fmt.Errorf("remove existing symlink %s: %w", dst, rmErr)
		}
	}
	in, err := os.Open(src)
	if err != nil {
		return false, fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return false, fmt.Errorf("create %s: %w", dst, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return false, fmt.Errorf("copy %s -> %s: %w", src, dst, err)
	}
	if err := out.Close(); err != nil {
		return false, fmt.Errorf("close %s: %w", dst, err)
	}
	if err := os.Chtimes(dst, info.ModTime(), info.ModTime()); err != nil {
		return false, fmt.Errorf("set timestamps on %s: %w", dst, err)
	}
	return true, nil
}

// sameContents returns true when dst already exists as a regular file with the
// same byte contents as src. A missing destination, a symlink, a directory, or
// any size/byte mismatch counts as "not same".
func sameContents(src, dst string, srcInfo os.FileInfo) (bool, error) {
	li, err := os.Lstat(dst)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat %s: %w", dst, err)
	}
	if li.Mode()&fs.ModeSymlink != 0 || li.IsDir() {
		return false, nil
	}
	if li.Size() != srcInfo.Size() {
		return false, nil
	}
	a, err := os.ReadFile(src)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", src, err)
	}
	b, err := os.ReadFile(dst)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", dst, err)
	}
	return bytes.Equal(a, b), nil
}

// prune removes files under destination roots that are not declared by the
// manifest. Pruning only walks directory specs.
func prune(specs []Spec, dir Direction, opts Options) (int, []Action, error) {
	expected, dirs := destinationSet(specs, dir)
	removed := 0
	actions := []Action{}
	for _, root := range dirs {
		if _, err := os.Stat(root); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return removed, actions, fmt.Errorf("stat %s: %w", root, err)
		}
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return fmt.Errorf("walk %s: %w", p, err)
			}
			if d.IsDir() {
				return nil
			}
			if _, ok := expected[p]; ok {
				return nil
			}
			fmt.Fprintf(opts.Out, "prune %s\n", p)
			if opts.DryRun {
				removed++
				actions = append(actions, Action{Action: "prune", Path: p})
				return nil
			}
			if rmErr := os.Remove(p); rmErr != nil {
				return fmt.Errorf("remove %s: %w", p, rmErr)
			}
			removed++
			actions = append(actions, Action{Action: "prune", Path: p})
			return nil
		})
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return removed, actions, err
		}
	}
	return removed, actions, nil
}

// destinationSet returns (expected destination file paths, destination dir
// roots to walk). Files in the source tree are mapped to their destination
// counterparts; anything in the destination tree outside that set is stale.
// Pruning only walks directory specs.
func destinationSet(specs []Spec, dir Direction) (map[string]struct{}, []string) {
	expected := map[string]struct{}{}
	var dirs []string
	for _, s := range specs {
		if !s.IsDir {
			continue
		}
		destRoot := s.SavedRoot
		if dir == DirInstall {
			destRoot = s.LiveRoot
		}
		dirs = append(dirs, destRoot)
		entries, err := expandSource(s, dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if dir == DirSave {
				expected[e.Saved] = struct{}{}
			} else {
				expected[e.Live] = struct{}{}
			}
		}
	}
	return expected, dirs
}
