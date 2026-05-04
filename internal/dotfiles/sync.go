package dotfiles

import (
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
	// DirSave copies from the live filesystem into the dotfiles repository.
	DirSave Direction = iota
	// DirApply copies from the dotfiles repository to the live filesystem.
	DirApply
)

// Options controls Sync behavior.
type Options struct {
	DryRun  bool
	Verbose bool
	Prune   bool
	Out     io.Writer
}

// Action records a single per-file effect of a Sync run for machine-readable
// output. Action is one of "copy", "prune", or "error".
type Action struct {
	Action  string `json:"action"`
	From    string `json:"from,omitempty"`
	To      string `json:"to,omitempty"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message,omitempty"`
}

// Result summarizes a Sync run.
type Result struct {
	Copied  int
	Removed int
	Errors  int
	Actions []Action
}

// Sync copies files for the given specs in the chosen direction. When Prune is
// set in DirApply mode, files in the destination tree that are not declared by
// the manifest are removed.
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
		src, dst := e.Local, e.Dotfile
		if dir == DirApply {
			src, dst = dst, src
		}
		if err := copyOne(src, dst, opts); err != nil {
			res.Errors++
			res.Actions = append(res.Actions, Action{Action: "error", From: src, To: dst, Message: err.Error()})
			fmt.Fprintf(opts.Out, "Error %s -> %s: %v\n", src, dst, err)
			continue
		}
		res.Copied++
		res.Actions = append(res.Actions, Action{Action: "copy", From: src, To: dst})
		if opts.Verbose {
			fmt.Fprintf(opts.Out, "OK %s -> %s\n", src, dst)
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

func copyOne(src, dst string, opts Options) error {
	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("source missing")
		}
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("source is a directory")
	}
	if opts.DryRun {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	// Replace any existing symlink with a real file copy.
	if li, err := os.Lstat(dst); err == nil && li.Mode()&fs.ModeSymlink != 0 {
		if rmErr := os.Remove(dst); rmErr != nil {
			return rmErr
		}
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chtimes(dst, info.ModTime(), info.ModTime())
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
			return removed, actions, err
		}
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if _, ok := expected[p]; ok {
				return nil
			}
			if opts.Verbose {
				fmt.Fprintf(opts.Out, "PRUNE %s\n", p)
			}
			if opts.DryRun {
				removed++
				actions = append(actions, Action{Action: "prune", Path: p})
				return nil
			}
			if rmErr := os.Remove(p); rmErr != nil {
				return rmErr
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
		destRoot := s.DotfileRoot
		if dir == DirApply {
			destRoot = s.LocalRoot
		}
		dirs = append(dirs, destRoot)
		entries, err := ExpandSource(s, dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if dir == DirSave {
				expected[e.Dotfile] = struct{}{}
			} else {
				expected[e.Local] = struct{}{}
			}
		}
	}
	return expected, dirs
}
