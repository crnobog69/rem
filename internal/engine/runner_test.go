package engine

import (
	"io"
	"testing"

	"rem/internal/remfile"
)

func TestCycleDetection(t *testing.T) {
	rf := &remfile.File{
		Default: "a",
		Order:   []string{"a", "b"},
		Tasks: map[string]*remfile.Task{
			"a": {Name: "a", Deps: []string{"b"}},
			"b": {Name: "b", Deps: []string{"a"}},
		},
	}

	r := &Runner{
		File:   rf,
		Jobs:   1,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}

	if err := r.Run("a"); err == nil {
		t.Fatalf("expected cycle error, got nil")
	}
}
