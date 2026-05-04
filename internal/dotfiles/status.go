package dotfiles

import (
	"bytes"
	"encoding/json"
	"os"
)

// State enumerates the possible sync states for a single Entry.
type State string

const (
	StateInSync         State = "in sync"
	StateLocalMissing   State = "local copy missing"
	StateDotfileMissing State = "dotfile missing"
	StateLocalChanges   State = "local changes"
	StateDotfileChanges State = "dotfile changes"
	StateNeitherExists  State = "neither exists"
)

// StatusEntry is the comparison result for one Entry.
type StatusEntry struct {
	Tool    string `json:"tool"`
	Local   string `json:"local"`
	Dotfile string `json:"dotfile"`
	State   State  `json:"state"`
}

// Status compares each entry's local and dotfile files and returns one
// StatusEntry per entry, in the order produced by ExpandAll.
func Status(specs []Spec) ([]StatusEntry, error) {
	entries, err := ExpandAllUnion(specs)
	if err != nil {
		return nil, err
	}
	out := make([]StatusEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, StatusEntry{
			Tool:    e.Tool,
			Local:   e.Local,
			Dotfile: e.Dotfile,
			State:   compare(e.Local, e.Dotfile),
		})
	}
	return out, nil
}

func compare(local, dotfile string) State {
	li, lerr := os.Stat(local)
	di, derr := os.Stat(dotfile)
	switch {
	case os.IsNotExist(lerr) && os.IsNotExist(derr):
		return StateNeitherExists
	case os.IsNotExist(lerr):
		return StateLocalMissing
	case os.IsNotExist(derr):
		return StateDotfileMissing
	case lerr != nil || derr != nil:
		return StateNeitherExists
	}
	if li.IsDir() || di.IsDir() {
		return StateInSync // directories are not compared as units
	}
	lb, err := os.ReadFile(local)
	if err != nil {
		return StateLocalMissing
	}
	db, err := os.ReadFile(dotfile)
	if err != nil {
		return StateDotfileMissing
	}
	if bytes.Equal(lb, db) {
		return StateInSync
	}
	if li.ModTime().After(di.ModTime()) {
		return StateLocalChanges
	}
	return StateDotfileChanges
}

// ConfigJSON returns a deterministic JSON object mapping dotfile path to local
// path for every entry produced by the manifest.
func ConfigJSON(specs []Spec) ([]byte, error) {
	entries, err := ExpandAllUnion(specs)
	if err != nil {
		return nil, err
	}
	type pair struct {
		Dotfile string
		Local   string
	}
	pairs := make([]pair, 0, len(entries))
	for _, e := range entries {
		pairs = append(pairs, pair{e.Dotfile, e.Local})
	}
	// Use an ordered marshal: write keys in the order of entries for stable output.
	// json.Marshal of a map sorts keys alphabetically; that's good enough for an
	// agent-facing format and keeps the output deterministic.
	m := make(map[string]string, len(pairs))
	for _, p := range pairs {
		m[p.Dotfile] = p.Local
	}
	return json.MarshalIndent(m, "", "  ")
}
