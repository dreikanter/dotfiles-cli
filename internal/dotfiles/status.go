package dotfiles

import (
	"bytes"
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
		out = append(out, StatusEntry{
			Tool:  e.Tool,
			Live:  e.Live,
			Saved: e.Saved,
			State: compare(e.Live, e.Saved),
		})
	}
	return out, nil
}

func compare(live, saved string) State {
	li, lerr := os.Stat(live)
	di, derr := os.Stat(saved)
	switch {
	case os.IsNotExist(lerr) && os.IsNotExist(derr):
		return StateNeitherExists
	case os.IsNotExist(lerr):
		return StateLiveMissing
	case os.IsNotExist(derr):
		return StateSavedMissing
	case lerr != nil || derr != nil:
		return StateNeitherExists
	}
	if li.IsDir() || di.IsDir() {
		return StateInSync // directories are not compared as units
	}
	lb, err := os.ReadFile(live)
	if err != nil {
		return StateLiveMissing
	}
	db, err := os.ReadFile(saved)
	if err != nil {
		return StateSavedMissing
	}
	if bytes.Equal(lb, db) {
		return StateInSync
	}
	if li.ModTime().After(di.ModTime()) {
		return StateLiveChanges
	}
	return StateSavedChanges
}
