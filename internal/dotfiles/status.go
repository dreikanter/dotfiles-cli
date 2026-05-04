package dotfiles

import (
	"bytes"
	"fmt"
	"os"
	"strings"
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
// plain-text output.
func (s State) Display() string {
	return strings.ReplaceAll(string(s), "-", " ")
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
	if li.IsDir() || di.IsDir() {
		return StateInSync, "" // directories are not compared as units
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
