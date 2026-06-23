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
	Skipped   int
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
		outcome, err := copyOne(src, dst, opts)
		if err != nil {
			res.Errors++
			res.Actions = append(res.Actions, Action{Action: "error", From: src, To: dst, Message: err.Error()})
			fmt.Fprintf(opts.Out, "error %s -> %s: %v\n", src, dst, err)
			continue
		}
		switch outcome {
		case outcomeCopied:
			res.Copied++
			res.Actions = append(res.Actions, Action{Action: "copy", From: src, To: dst})
			fmt.Fprintf(opts.Out, "copy %s -> %s\n", src, dst)
		case outcomeSkipped:
			res.Skipped++
			msg := preservedSide(dir) + " is newer"
			res.Actions = append(res.Actions, Action{Action: "skip", From: src, To: dst, Message: msg})
			fmt.Fprintf(opts.Out, "skip %s (%s)\n", dst, msg)
		default: // outcomeUnchanged
			res.Unchanged++
			res.Actions = append(res.Actions, Action{Action: "unchanged", From: src, To: dst})
			if opts.Verbose {
				fmt.Fprintf(opts.Out, "unchanged %s\n", dst)
			}
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

// copyOutcome is the per-file result reported by copyOne.
type copyOutcome int

const (
	// outcomeError is the zero value, returned only alongside a non-nil error.
	outcomeError copyOutcome = iota
	// outcomeCopied means dst was written (or, in dry-run, would be).
	outcomeCopied
	// outcomeUnchanged means dst already matched src byte for byte.
	outcomeUnchanged
	// outcomeSkipped means dst was a regular file newer than src and was left
	// intact, so an obsolete source never clobbers a newer destination.
	outcomeSkipped
)

// preservedSide names the side kept intact when a newer destination is skipped:
// the "saved" repo copy when saving, the "live" file when installing.
func preservedSide(dir Direction) string {
	if dir == DirInstall {
		return "live"
	}
	return "saved"
}

// copyOne classifies how src and dst relate and, unless dry-run, makes dst
// match src. Identical contents are reported as unchanged. A destination that
// is a regular file newer than the source is preserved (outcomeSkipped) so only
// an obsolete copy is ever overwritten. Every classification is decided even in
// dry-run mode, so the caller can distinguish "would change" from "would
// no-op".
func copyOne(src, dst string, opts Options) (copyOutcome, error) {
	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return outcomeError, fmt.Errorf("source missing: %s", src)
		}
		return outcomeError, fmt.Errorf("stat %s: %w", src, err)
	}
	if info.IsDir() {
		return outcomeError, fmt.Errorf("source is a directory: %s", src)
	}
	dstInfo, derr := os.Lstat(dst)
	if derr != nil && !os.IsNotExist(derr) {
		return outcomeError, fmt.Errorf("stat %s: %w", dst, derr)
	}
	// The newer-wins guard applies only between two regular files. A missing
	// destination, a symlink, or a directory is always (re)written; in
	// particular a stale symlink is replaced with a real copy below.
	if derr == nil && dstInfo.Mode().IsRegular() {
		same, err := sameContents(src, dst, info, dstInfo)
		if err != nil {
			return outcomeError, err
		}
		if same {
			return outcomeUnchanged, nil
		}
		if dstInfo.ModTime().After(info.ModTime()) {
			return outcomeSkipped, nil
		}
	}
	if opts.DryRun {
		return outcomeCopied, nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return outcomeError, fmt.Errorf("mkdir %s: %w", filepath.Dir(dst), err)
	}
	// Replace any existing symlink with a real file copy.
	if derr == nil && dstInfo.Mode()&fs.ModeSymlink != 0 {
		if rmErr := os.Remove(dst); rmErr != nil {
			return outcomeError, fmt.Errorf("remove existing symlink %s: %w", dst, rmErr)
		}
	}
	in, err := os.Open(src)
	if err != nil {
		return outcomeError, fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return outcomeError, fmt.Errorf("create %s: %w", dst, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return outcomeError, fmt.Errorf("copy %s -> %s: %w", src, dst, err)
	}
	if err := out.Close(); err != nil {
		return outcomeError, fmt.Errorf("close %s: %w", dst, err)
	}
	if err := os.Chtimes(dst, info.ModTime(), info.ModTime()); err != nil {
		return outcomeError, fmt.Errorf("set timestamps on %s: %w", dst, err)
	}
	return outcomeCopied, nil
}

// sameContents reports whether two regular files hold identical bytes. The
// caller supplies both FileInfos and guarantees dst is a regular file; size is
// compared before any read.
func sameContents(src, dst string, srcInfo, dstInfo os.FileInfo) (bool, error) {
	if srcInfo.Size() != dstInfo.Size() {
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
		destRoot := s.SavedPath
		if dir == DirInstall {
			destRoot = s.LivePath
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
