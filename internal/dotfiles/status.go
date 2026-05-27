package dotfiles

import (
	"bytes"
	"fmt"
	"os"
)

// State enumerates the possible sync states for a single Entry. The string
// form is the JSON-friendly hyphenated token; use Display for plain text.
type State string

const (
	StateInSync        State = "in-sync"
	StateLiveMissing   State = "live-missing"
	StateSavedMissing  State = "saved-missing"
	StateLiveChanges   State = "live-changes"
	StateSavedChanges  State = "saved-changes"
	StateNeitherExists State = "neither-exists"
	StateError         State = "error"
)

// Display returns the human-readable form of the state, suitable for
// plain-text output. The phrases read as descriptions, not actions, so they
// do not collide with verbs like "saved" in the save command's output.
func (s State) Display() string {
	switch s {
	case StateInSync:
		return "in sync"
	case StateLiveMissing:
		return "not on disk"
	case StateSavedMissing:
		return "not in repo"
	case StateLiveChanges:
		return "live newer"
	case StateSavedChanges:
		return "saved newer"
	case StateNeitherExists:
		return "both missing"
	case StateError:
		return "error"
	}
	return string(s)
}

// StatusEntry is the comparison result for one Entry.
type StatusEntry struct {
	Tool  string `json:"tool"`
	Live  string `json:"live"`
	Saved string `json:"saved"`
	State State  `json:"state"`
	Error string `json:"error,omitempty"`
}

// Status compares each entry's live and saved files and returns one
// StatusEntry per entry, in the order produced by ExpandAll.
func Status(specs []Spec) ([]StatusEntry, error) {
	entries, err := ExpandAllUnion(specs)
	if err != nil {
		return nil, err
	}
	out := make([]StatusEntry, 0, len(entries))
	for _, e := range entries {
		state, errMsg := compare(e.Live, e.Saved)
		out = append(out, StatusEntry{
			Tool:  e.Tool,
			Live:  e.Live,
			Saved: e.Saved,
			State: state,
			Error: errMsg,
		})
	}
	return out, nil
}

func compare(live, saved string) (State, string) {
	li, lerr := os.Stat(live)
	di, derr := os.Stat(saved)
	// A legitimate directory spec expands to one entry per file inside the
	// tree, so compare only ever sees files. A directory on either side means
	// a single-file entry resolved to a directory on disk — i.e. the manifest
	// is missing the trailing-slash marker. Surface it before the
	// missing-side logic so it never masquerades as live/saved-missing.
	if lerr == nil && li.IsDir() {
		return StateError, fmt.Sprintf("%s: expected a file but found a directory (missing trailing slash in manifest?)", live)
	}
	if derr == nil && di.IsDir() {
		return StateError, fmt.Sprintf("%s: expected a file but found a directory (missing trailing slash in manifest?)", saved)
	}
	switch {
	case os.IsNotExist(lerr) && os.IsNotExist(derr):
		return StateNeitherExists, ""
	case lerr != nil && !os.IsNotExist(lerr):
		return StateError, fmt.Sprintf("stat %s: %s", live, lerr)
	case derr != nil && !os.IsNotExist(derr):
		return StateError, fmt.Sprintf("stat %s: %s", saved, derr)
	case os.IsNotExist(lerr):
		return StateLiveMissing, ""
	case os.IsNotExist(derr):
		return StateSavedMissing, ""
	}
	lb, err := os.ReadFile(live)
	if err != nil {
		return StateError, fmt.Sprintf("read %s: %s", live, err)
	}
	db, err := os.ReadFile(saved)
	if err != nil {
		return StateError, fmt.Sprintf("read %s: %s", saved, err)
	}
	if bytes.Equal(lb, db) {
		return StateInSync, ""
	}
	if li.ModTime().After(di.ModTime()) {
		return StateLiveChanges, ""
	}
	return StateSavedChanges, ""
}
