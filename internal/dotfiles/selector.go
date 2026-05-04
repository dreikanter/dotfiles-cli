package dotfiles

import (
	"fmt"
	"path/filepath"
)

// Selector narrows save/install/status/config to a specific tool and
// optionally to specific files within that tool. The zero value selects
// everything.
//
// Tool is singular (not repeatable). Files require a Tool and are matched by
// exact, byte-for-byte equality against each Spec's LiveRoot after the same
// expansion the manifest uses (~ → home, strip trailing /, Clean) plus
// filepath.Abs to resolve CWD-relative input. No glob support, no symlink
// resolution, no sub-file matching inside directory entries.
type Selector struct {
	Tool  string
	Files []string
}

// IsEmpty reports whether no filter is set.
func (s Selector) IsEmpty() bool {
	return s.Tool == "" && len(s.Files) == 0
}

// Apply filters specs according to the selector and validates the filter
// against the manifest. Validation runs before any IO; all failures return a
// non-nil error so callers can exit non-zero.
//
// home is used to expand "~" prefixes in Files entries.
func (s Selector) Apply(specs []Spec, home string) ([]Spec, error) {
	if len(s.Files) > 0 && s.Tool == "" {
		return nil, fmt.Errorf("--file requires --tool")
	}
	if s.Tool == "" {
		return specs, nil
	}

	toolSpecs := make([]Spec, 0, len(specs))
	for _, sp := range specs {
		if sp.Tool == s.Tool {
			toolSpecs = append(toolSpecs, sp)
		}
	}
	if len(toolSpecs) == 0 {
		return nil, fmt.Errorf("tool %q not in manifest", s.Tool)
	}
	if len(s.Files) == 0 {
		return toolSpecs, nil
	}

	byLive := make(map[string]Spec, len(toolSpecs))
	for _, sp := range toolSpecs {
		byLive[sp.LiveRoot] = sp
	}

	out := make([]Spec, 0, len(s.Files))
	seen := make(map[string]bool, len(s.Files))
	for _, f := range s.Files {
		abs, err := resolveFileFilter(f, home)
		if err != nil {
			return nil, fmt.Errorf("resolve --file %q: %w", f, err)
		}
		sp, ok := byLive[abs]
		if !ok {
			return nil, fmt.Errorf("file %q not declared by tool %q", abs, s.Tool)
		}
		if seen[abs] {
			continue
		}
		seen[abs] = true
		out = append(out, sp)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("filter resolved to zero entries")
	}
	return out, nil
}

// ResolvedFiles returns the absolute paths --file resolved to, in input order
// with duplicates removed. Used to populate machine-readable output. Errors
// from Apply are not re-checked; callers should call Apply first.
func (s Selector) ResolvedFiles(home string) []string {
	if len(s.Files) == 0 {
		return nil
	}
	out := make([]string, 0, len(s.Files))
	seen := make(map[string]bool, len(s.Files))
	for _, f := range s.Files {
		abs, err := resolveFileFilter(f, home)
		if err != nil {
			continue
		}
		if seen[abs] {
			continue
		}
		seen[abs] = true
		out = append(out, abs)
	}
	return out
}

func resolveFileFilter(p, home string) (string, error) {
	return filepath.Abs(expand(p, home))
}
