package remfile

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseTOMLBasic(t *testing.T) {
	content := `
default = "build"

[task.gen]
cmds = ["go generate ./..."]

[task.build]
deps = ["gen"]
inputs = ["a.go", "b.go"]
outputs = ["bin/rem"]
cmds = ["go build ./..."]
`
	dir := t.TempDir()
	path := filepath.Join(dir, "Remfile")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)), 0o644); err != nil {
		t.Fatal(err)
	}

	rf, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if rf.Default != "build" {
		t.Fatalf("default = %q, want build", rf.Default)
	}
	if len(rf.Tasks["build"].Deps) != 1 || rf.Tasks["build"].Deps[0] != "gen" {
		t.Fatalf("unexpected deps: %#v", rf.Tasks["build"].Deps)
	}
	if len(rf.Tasks["build"].Cmds) != 1 {
		t.Fatalf("unexpected cmds: %#v", rf.Tasks["build"].Cmds)
	}
}

func TestParseVarsExpansion(t *testing.T) {
	t.Setenv("VERSION", "v9.9.9")

	content := `
default = "build"

[vars]
APP_NAME = "gitcrn"
VERSION = "${VERSION:-dev}"

[task.build]
outputs = ["dist/${APP_NAME}"]
cmds = ["echo ${VERSION}"]
`
	dir := t.TempDir()
	path := filepath.Join(dir, "Remfile")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)), 0o644); err != nil {
		t.Fatal(err)
	}

	rf, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if got := rf.Tasks["build"].Outputs[0]; got != "dist/${APP_NAME}" {
		t.Fatalf("raw outputs[0] = %q, want dist/${APP_NAME}", got)
	}
	if got := rf.ExpandList(rf.Tasks["build"].Outputs)[0]; got != "dist/gitcrn" {
		t.Fatalf("expanded outputs[0] = %q, want dist/gitcrn", got)
	}
	if got := rf.ExpandString(rf.Tasks["build"].Cmds[0]); got != "echo v9.9.9" {
		t.Fatalf("expanded cmds[0] = %q, want echo v9.9.9", got)
	}
}

func TestFormatRoundTripTOML(t *testing.T) {
	content := `
default = "build"

[vars]
APP_NAME = "gitcrn"

[task.build]
cmds = ["echo ${APP_NAME}"]
`
	rf, err := Parse(bytes.NewBufferString(strings.TrimSpace(content)))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	formatted := Format(rf)
	if !strings.Contains(formatted, "[task.build]") {
		t.Fatalf("formatted output missing task section:\n%s", formatted)
	}

	rf2, err := Parse(bytes.NewBufferString(formatted))
	if err != nil {
		t.Fatalf("Parse(formatted) error: %v", err)
	}
	if rf2.Default != "build" {
		t.Fatalf("default = %q, want build", rf2.Default)
	}
}

func TestParseTOMLMultilineArray(t *testing.T) {
	content := `
default = "build"

[task.build]
cmds = [
  "echo one",
  "echo two",
]
`
	rf, err := Parse(bytes.NewBufferString(strings.TrimSpace(content)))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if len(rf.Tasks["build"].Cmds) != 2 {
		t.Fatalf("cmds len = %d, want 2", len(rf.Tasks["build"].Cmds))
	}
}

func TestParseLegacyRejected(t *testing.T) {
	legacy := `
default build

task build:
    cmd = echo hi
`
	_, err := Parse(bytes.NewBufferString(strings.TrimSpace(legacy)))
	if err == nil {
		t.Fatalf("expected legacy syntax to fail, got nil")
	}
}

func TestApplyOverridesRecomputeDependentVars(t *testing.T) {
	content := `
default = "build"

[vars]
VERSION = "dev"
LDFLAGS = "-X main.version=${VERSION}"

[task.build]
cmds = ["echo ${LDFLAGS}"]
`
	rf, err := Parse(bytes.NewBufferString(strings.TrimSpace(content)))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if err := rf.ApplyOverrides(map[string]string{"VERSION": "v1.2.3"}); err != nil {
		t.Fatalf("ApplyOverrides() error: %v", err)
	}
	got := rf.ExpandString(rf.Tasks["build"].Cmds[0])
	if got != "echo -X main.version=v1.2.3" {
		t.Fatalf("expanded cmd = %q", got)
	}
}
