package remfile

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type Task struct {
	Name    string
	Desc    string
	Deps    []string
	Inputs  []string
	Outputs []string
	Cmds    []string
	Dir     string
}

type File struct {
	Path     string
	Dir      string
	Vars     map[string]string
	RawVars  map[string]string
	VarOrder []string
	Default  string
	Order    []string
	Tasks    map[string]*Task
}

func Load(path string) (*File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rf, err := Parse(f)
	if err != nil {
		return nil, err
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	rf.Path = abs
	rf.Dir = filepath.Dir(abs)
	return rf, nil
}

func Parse(r io.Reader) (*File, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	text := strings.ReplaceAll(string(content), "\r\n", "\n")

	rf, err := parseTOML(text)
	if err != nil {
		return nil, fmt.Errorf("Remfile parse failed: %w", err)
	}
	return rf, nil
}

func parseTOML(text string) (*File, error) {
	rf := &File{
		Vars:    make(map[string]string),
		RawVars: make(map[string]string),
		Tasks:   make(map[string]*Task),
	}
	rawVars := make(map[string]string)

	const (
		sectionRoot = iota
		sectionVars
		sectionTask
	)
	section := sectionRoot
	currentTask := ""

	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); i++ {
		raw := lines[i]
		line := strings.TrimSpace(stripInlineComment(raw))
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "[") {
			name, ok := parseSection(line)
			if !ok {
				return nil, fmt.Errorf("line %d: invalid TOML section %q", i+1, line)
			}
			switch {
			case name == "vars":
				section = sectionVars
				currentTask = ""
			case strings.HasPrefix(name, "task."):
				taskName := strings.TrimSpace(strings.TrimPrefix(name, "task."))
				if !isTaskName(taskName) {
					return nil, fmt.Errorf("line %d: invalid task name %q", i+1, taskName)
				}
				if _, exists := rf.Tasks[taskName]; exists {
					return nil, fmt.Errorf("line %d: duplicate task section %q", i+1, taskName)
				}
				t := &Task{Name: taskName}
				rf.Tasks[taskName] = t
				rf.Order = append(rf.Order, taskName)
				section = sectionTask
				currentTask = taskName
			default:
				return nil, fmt.Errorf("line %d: unsupported section %q", i+1, name)
			}
			continue
		}

		key, val, ok := splitKV(line)
		if !ok {
			return nil, fmt.Errorf("line %d: invalid TOML key-value %q", i+1, line)
		}
		if strings.HasPrefix(strings.TrimSpace(val), "[") {
			balance := bracketDelta(val)
			for balance > 0 {
				i++
				if i >= len(lines) {
					return nil, fmt.Errorf("line %d: unterminated array value for key %q", i+1, key)
				}
				next := strings.TrimSpace(stripInlineComment(lines[i]))
				if next == "" {
					continue
				}
				val += " " + next
				balance += bracketDelta(next)
			}
		}

		switch section {
		case sectionRoot:
			if key != "default" {
				return nil, fmt.Errorf("line %d: unsupported top-level key %q", i+1, key)
			}
			parsed, err := parseTOMLStringValue(val)
			if err != nil {
				return nil, fmt.Errorf("line %d: default: %w", i+1, err)
			}
			rf.Default = parsed
		case sectionVars:
			if !isVarName(key) {
				return nil, fmt.Errorf("line %d: invalid variable name %q", i+1, key)
			}
			if _, exists := rawVars[key]; exists {
				return nil, fmt.Errorf("line %d: duplicate var %q", i+1, key)
			}
			parsed, err := parseTOMLStringValue(val)
			if err != nil {
				return nil, fmt.Errorf("line %d: var %q: %w", i+1, key, err)
			}
			rawVars[key] = parsed
			rf.RawVars[key] = parsed
			rf.VarOrder = append(rf.VarOrder, key)
		case sectionTask:
			t := rf.Tasks[currentTask]
			switch key {
			case "desc":
				parsed, err := parseTOMLStringValue(val)
				if err != nil {
					return nil, fmt.Errorf("line %d: task %q desc: %w", i+1, currentTask, err)
				}
				t.Desc = parsed
			case "dir":
				parsed, err := parseTOMLStringValue(val)
				if err != nil {
					return nil, fmt.Errorf("line %d: task %q dir: %w", i+1, currentTask, err)
				}
				t.Dir = parsed
			case "deps":
				items, err := parseTOMLListValue(val)
				if err != nil {
					return nil, fmt.Errorf("line %d: task %q deps: %w", i+1, currentTask, err)
				}
				t.Deps = append(t.Deps, items...)
			case "inputs":
				items, err := parseTOMLListValue(val)
				if err != nil {
					return nil, fmt.Errorf("line %d: task %q inputs: %w", i+1, currentTask, err)
				}
				t.Inputs = append(t.Inputs, items...)
			case "outputs":
				items, err := parseTOMLListValue(val)
				if err != nil {
					return nil, fmt.Errorf("line %d: task %q outputs: %w", i+1, currentTask, err)
				}
				t.Outputs = append(t.Outputs, items...)
			case "cmd":
				parsed, err := parseTOMLStringValue(val)
				if err != nil {
					return nil, fmt.Errorf("line %d: task %q cmd: %w", i+1, currentTask, err)
				}
				if parsed != "" {
					t.Cmds = append(t.Cmds, parsed)
				}
			case "cmds":
				items, err := parseTOMLStringArray(val)
				if err != nil {
					return nil, fmt.Errorf("line %d: task %q cmds: %w", i+1, currentTask, err)
				}
				t.Cmds = append(t.Cmds, items...)
			default:
				return nil, fmt.Errorf("line %d: unknown task field %q", i+1, key)
			}
		}
	}

	return finalizeFile(rf, rawVars)
}

func finalizeFile(rf *File, rawVars map[string]string) (*File, error) {
	resolvedVars, err := resolveVars(rawVars)
	if err != nil {
		return nil, err
	}
	rf.Vars = resolvedVars

	if len(rf.Tasks) == 0 {
		return nil, errors.New("Remfile has no tasks")
	}
	if rf.Default == "" {
		rf.Default = rf.Order[0]
	}
	defaultTask := rf.DefaultTarget()
	if _, ok := rf.Tasks[defaultTask]; !ok {
		return nil, fmt.Errorf("default task %q is not defined", defaultTask)
	}
	for _, name := range rf.Order {
		task := rf.Tasks[name]
		for _, dep := range task.Deps {
			depName := rf.ExpandString(dep)
			if _, ok := rf.Tasks[depName]; !ok {
				return nil, fmt.Errorf("task %q depends on undefined task %q", name, depName)
			}
		}
	}

	return rf, nil
}

func Format(rf *File) string {
	var b strings.Builder

	b.WriteString("default = ")
	b.WriteString(quoteTOML(rf.Default))
	b.WriteString("\n")

	writeVars := rf.VarOrder
	if len(writeVars) == 0 && len(rf.Vars) > 0 {
		writeVars = make([]string, 0, len(rf.Vars))
		for k := range rf.Vars {
			writeVars = append(writeVars, k)
		}
		sort.Strings(writeVars)
	}

	if len(writeVars) > 0 {
		b.WriteString("\n[vars]\n")
		for _, name := range writeVars {
			val := rf.RawVars[name]
			if val == "" {
				val = rf.Vars[name]
			}
			b.WriteString(name)
			b.WriteString(" = ")
			b.WriteString(quoteTOML(val))
			b.WriteString("\n")
		}
	}

	for _, name := range rf.Order {
		t := rf.Tasks[name]
		b.WriteString("\n[task.")
		b.WriteString(name)
		b.WriteString("]\n")

		if t.Desc != "" {
			b.WriteString("desc = ")
			b.WriteString(quoteTOML(t.Desc))
			b.WriteString("\n")
		}
		if len(t.Deps) > 0 {
			b.WriteString("deps = ")
			b.WriteString(formatTOMLArray(t.Deps))
			b.WriteString("\n")
		}
		if len(t.Inputs) > 0 {
			b.WriteString("inputs = ")
			b.WriteString(formatTOMLArray(t.Inputs))
			b.WriteString("\n")
		}
		if len(t.Outputs) > 0 {
			b.WriteString("outputs = ")
			b.WriteString(formatTOMLArray(t.Outputs))
			b.WriteString("\n")
		}
		if t.Dir != "" {
			b.WriteString("dir = ")
			b.WriteString(quoteTOML(t.Dir))
			b.WriteString("\n")
		}
		if len(t.Cmds) > 0 {
			b.WriteString("cmds = ")
			b.WriteString(formatTOMLArray(t.Cmds))
			b.WriteString("\n")
		}
	}

	out := b.String()
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return out
}

func (f *File) ExpandString(input string) string {
	return expandStringLoose(input, f.Vars)
}

func (f *File) ExpandList(values []string) []string {
	return expandListLoose(values, f.Vars)
}

func (f *File) DefaultTarget() string {
	return f.ExpandString(f.Default)
}

func (f *File) ApplyOverrides(overrides map[string]string) error {
	if len(overrides) == 0 {
		return nil
	}

	raw := make(map[string]string, len(f.RawVars)+len(overrides))
	for k, v := range f.RawVars {
		raw[k] = v
	}
	for k, v := range overrides {
		if _, exists := raw[k]; !exists {
			f.VarOrder = append(f.VarOrder, k)
		}
		raw[k] = v
	}

	resolved, err := resolveVars(raw)
	if err != nil {
		return err
	}
	f.RawVars = raw
	f.Vars = resolved
	return nil
}

func WriteStarter(path string, force bool) error {
	if _, err := os.Stat(path); err == nil && !force {
		return fmt.Errorf("%s already exists (use --force)", path)
	}

	const starter = `default = "build"

[vars]
APP_NAME = "rem"
VERSION = "dev"
PROD_LDFLAGS = "-s -w -X main.version=${VERSION}"
RELEASE_VERSION = "${VERSION}"

[task.gen]
desc = "Generate files"
cmds = ["go generate ./..."]

[task.build]
desc = "Build rem binary"
deps = ["gen"]
inputs = ["cmd/rem/main.go", "internal/*/*.go", "go.mod"]
outputs = ["bin/${APP_NAME}"]
cmds = [
  "mkdir -p bin",
  "go build -ldflags \"-X main.version=${VERSION}\" -o bin/${APP_NAME} ./cmd/rem",
]

[task.test]
desc = "Run tests"
cmds = ["go test ./..."]

[task.production]
desc = "Build production binary"
deps = ["test"]
outputs = ["bin/${APP_NAME}-prod"]
cmds = [
  "mkdir -p bin",
  "go build -trimpath -ldflags \"${PROD_LDFLAGS}\" -o bin/${APP_NAME}-prod ./cmd/rem",
]

[task.release-assets]
desc = "Build cross-platform release artifacts"
deps = ["test"]
cmds = ["./scripts/release.sh --version ${RELEASE_VERSION}"]

[task.release]
desc = "Production + release artifacts"
deps = ["production", "release-assets"]
`

	const starterDocEN = `# REM

Project notes for rem.

## Quick Start

1. Run:
   - rem list
   - rem build
2. Inspect graph:
   - rem graph
3. Run a specific target:
   - rem run test

## Remfile Syntax (TOML)

` + "```toml" + `
default = "build"

[vars]
APP_NAME = "myapp"
VERSION = "${VERSION:-dev}"

[task.build]
desc = "Build binary"
inputs = ["cmd/myapp/main.go", "go.mod"]
outputs = ["bin/${APP_NAME}"]
cmds = [
  "mkdir -p bin",
  "go build -ldflags \"-X main.version=${VERSION}\" -o bin/${APP_NAME} ./cmd/myapp",
]

[task.test]
deps = ["build"]
cmds = ["go test ./..."]
` + "```" + `

## Commands

- rem init
- rem doctor
- rem list -D VERSION=v0.1.0
- rem graph -D APP_NAME=rem
- rem build [target]
- rem run <target>
- rem run -D VERSION=v1.0.0 production
- rem run -D RELEASE_VERSION=v1.0.0 release
- rem format
- rem format --check

## Notes

- Variable expansion: ${VAR} and ${VAR:-fallback}
- Tasks without outputs behave like phony targets
- rem format writes canonical TOML and may rewrite layout/comments
- rem doctor checks basic environment and Remfile health
- Task shell follows $SHELL; set REM_SHELL=/path/to/shell to force shell
`

	const starterDocSRCyrl = `# REM

Белешке за rem пројекат.

## Брзи старт

1. Покрени:
   - rem list
   - rem build
2. Погледај граф:
   - rem graph
3. Покрени конкретан target:
   - rem run test

## Remfile синтакса (TOML)

` + "```toml" + `
default = "build"

[vars]
APP_NAME = "myapp"
VERSION = "${VERSION:-dev}"

[task.build]
desc = "Компилација бинарног фајла"
inputs = ["cmd/myapp/main.go", "go.mod"]
outputs = ["bin/${APP_NAME}"]
cmds = [
  "mkdir -p bin",
  "go build -ldflags \"-X main.version=${VERSION}\" -o bin/${APP_NAME} ./cmd/myapp",
]

[task.test]
deps = ["build"]
cmds = ["go test ./..."]
` + "```" + `

## Команде

- rem init
- rem doctor
- rem list -D VERSION=v0.1.0
- rem graph -D APP_NAME=rem
- rem build [target]
- rem run <target>
- rem run -D VERSION=v1.0.0 production
- rem run -D RELEASE_VERSION=v1.0.0 release
- rem format
- rem format --check

## Напомене

- Експанзија променљивих: ${VAR} и ${VAR:-fallback}
- Task без outputs се понаша као phony target
- rem format пише канонски TOML и може да промени распоред/коментаре
- rem doctor проверава основно окружење и здравље Remfile-а
- Task shell прати $SHELL; постави REM_SHELL=/path/to/shell за форсирање shell-а
`

	if err := os.WriteFile(path, []byte(starter), 0o644); err != nil {
		return err
	}

	baseDir := filepath.Dir(path)
	if err := writeDocFile(filepath.Join(baseDir, "REM.md"), starterDocEN, force); err != nil {
		return err
	}
	if err := writeDocFile(filepath.Join(baseDir, "REM.sr-Cyrl.md"), starterDocSRCyrl, force); err != nil {
		return err
	}
	return nil
}

func writeDocFile(path string, content string, force bool) error {
	if _, err := os.Stat(path); err == nil && !force {
		return nil
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func stripInlineComment(s string) string {
	inSingle := false
	inDouble := false
	escaped := false

	for i := 0; i < len(s); i++ {
		c := s[i]
		if inDouble {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inDouble = false
			}
			continue
		}
		if inSingle {
			if c == '\'' {
				inSingle = false
			}
			continue
		}

		if c == '"' {
			inDouble = true
			continue
		}
		if c == '\'' {
			inSingle = true
			continue
		}
		if c == '#' {
			return s[:i]
		}
	}
	return s
}

func parseSection(line string) (string, bool) {
	if !strings.HasPrefix(line, "[") || !strings.HasSuffix(line, "]") {
		return "", false
	}
	inner := strings.TrimSpace(line[1 : len(line)-1])
	if inner == "" {
		return "", false
	}
	return inner, true
}

func parseTOMLStringValue(v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", fmt.Errorf("empty value")
	}
	if strings.HasPrefix(v, "[") {
		return "", fmt.Errorf("expected string value, got array")
	}

	if strings.HasPrefix(v, "\"") {
		u, err := strconv.Unquote(v)
		if err != nil {
			return "", err
		}
		return u, nil
	}
	if strings.HasPrefix(v, "'") {
		if len(v) < 2 || !strings.HasSuffix(v, "'") {
			return "", fmt.Errorf("unterminated single-quoted string")
		}
		return v[1 : len(v)-1], nil
	}

	return v, nil
}

func parseTOMLListValue(v string) ([]string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, nil
	}
	if strings.HasPrefix(v, "[") {
		return parseTOMLStringArray(v)
	}
	s, err := parseTOMLStringValue(v)
	if err != nil {
		return nil, err
	}
	return splitList(s), nil
}

func parseTOMLStringArray(v string) ([]string, error) {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "[") || !strings.HasSuffix(v, "]") {
		return nil, fmt.Errorf("expected array syntax [..]")
	}
	inner := strings.TrimSpace(v[1 : len(v)-1])
	if inner == "" {
		return nil, nil
	}

	items, err := splitArrayItems(inner)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		s, err := parseTOMLStringValue(item)
		if err != nil {
			return nil, err
		}
		if s != "" {
			out = append(out, s)
		}
	}
	return out, nil
}

func splitArrayItems(inner string) ([]string, error) {
	items := make([]string, 0, 4)
	start := 0
	inSingle := false
	inDouble := false
	escaped := false

	for i := 0; i < len(inner); i++ {
		c := inner[i]
		if inDouble {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inDouble = false
			}
			continue
		}
		if inSingle {
			if c == '\'' {
				inSingle = false
			}
			continue
		}

		switch c {
		case '"':
			inDouble = true
		case '\'':
			inSingle = true
		case ',':
			part := strings.TrimSpace(inner[start:i])
			if part == "" {
				return nil, fmt.Errorf("empty array item")
			}
			items = append(items, part)
			start = i + 1
		}
	}

	if inSingle || inDouble {
		return nil, fmt.Errorf("unterminated quoted string in array")
	}
	part := strings.TrimSpace(inner[start:])
	if part == "" {
		if strings.HasSuffix(strings.TrimSpace(inner), ",") {
			return items, nil
		}
		return nil, fmt.Errorf("empty array item")
	}
	items = append(items, part)
	return items, nil
}

func bracketDelta(s string) int {
	delta := 0
	inSingle := false
	inDouble := false
	escaped := false

	for i := 0; i < len(s); i++ {
		c := s[i]
		if inDouble {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inDouble = false
			}
			continue
		}
		if inSingle {
			if c == '\'' {
				inSingle = false
			}
			continue
		}

		switch c {
		case '"':
			inDouble = true
		case '\'':
			inSingle = true
		case '[':
			delta++
		case ']':
			delta--
		}
	}
	return delta
}

func quoteTOML(v string) string {
	return strconv.Quote(v)
}

func formatTOMLArray(items []string) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		parts = append(parts, quoteTOML(it))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func splitKV(line string) (string, string, bool) {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), true
}

func splitList(v string) []string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "[")
	v = strings.TrimSuffix(v, "]")
	fields := strings.FieldsFunc(v, func(r rune) bool {
		return unicode.IsSpace(r) || r == ','
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = trimQuotes(strings.TrimSpace(f))
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

func trimQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return s
	}
	if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
		return s[1 : len(s)-1]
	}
	return s
}

func isVarName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if i == 0 {
			if !(r == '_' || unicode.IsLetter(r)) {
				return false
			}
			continue
		}
		if !(r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return false
		}
	}
	return true
}

func isTaskName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if !(r == '_' || r == '-' || r == '.' || unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return false
		}
	}
	return true
}

func resolveVars(raw map[string]string) (map[string]string, error) {
	resolved := make(map[string]string, len(raw))
	visit := make(map[string]int, len(raw))
	stack := make([]string, 0, 8)

	var resolveOne func(string) (string, error)
	var expandStrict func(input string, current string) (string, error)

	resolveOne = func(name string) (string, error) {
		if v, ok := resolved[name]; ok {
			return v, nil
		}
		switch visit[name] {
		case 1:
			return "", fmt.Errorf("variable cycle detected: %s -> %s", strings.Join(stack, " -> "), name)
		case 2:
			return resolved[name], nil
		}

		rawVal, ok := raw[name]
		if !ok {
			return "", fmt.Errorf("undefined variable %q", name)
		}

		visit[name] = 1
		stack = append(stack, name)
		out, err := expandStrict(rawVal, name)
		if err != nil {
			return "", fmt.Errorf("var %q: %w", name, err)
		}
		stack = stack[:len(stack)-1]
		visit[name] = 2
		resolved[name] = out
		return out, nil
	}

	expandStrict = func(input string, current string) (string, error) {
		return expandTemplate(input, true, func(expr string) (string, bool, error) {
			refName, fallback, hasFallback := parseVarExpr(expr)
			if !isVarName(refName) {
				return "", false, nil
			}
			if refName == current {
				if envVal, ok := os.LookupEnv(refName); ok {
					return envVal, true, nil
				}
				if hasFallback {
					v, err := expandStrict(fallback, current)
					if err != nil {
						return "", false, err
					}
					return v, true, nil
				}
				return "", false, fmt.Errorf("self reference without fallback in ${%s}", refName)
			}
			if _, ok := raw[refName]; ok {
				v, err := resolveOne(refName)
				if err != nil {
					return "", false, err
				}
				return v, true, nil
			}
			if envVal, ok := os.LookupEnv(refName); ok {
				return envVal, true, nil
			}
			if hasFallback {
				v, err := expandStrict(fallback, current)
				if err != nil {
					return "", false, err
				}
				return v, true, nil
			}
			return "", false, nil
		})
	}

	for name := range raw {
		if _, err := resolveOne(name); err != nil {
			return nil, err
		}
	}
	return resolved, nil
}

func expandListLoose(values []string, vars map[string]string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		exp := strings.TrimSpace(expandStringLoose(v, vars))
		if exp != "" {
			out = append(out, exp)
		}
	}
	return out
}

func expandStringLoose(input string, vars map[string]string) string {
	out, _ := expandTemplate(input, false, func(expr string) (string, bool, error) {
		name, fallback, hasFallback := parseVarExpr(expr)
		if !isVarName(name) {
			return "", false, nil
		}
		if val, ok := vars[name]; ok {
			return val, true, nil
		}
		if envVal, ok := os.LookupEnv(name); ok {
			return envVal, true, nil
		}
		if hasFallback {
			return expandStringLoose(fallback, vars), true, nil
		}
		return "", false, nil
	})
	return out
}

func parseVarExpr(expr string) (name string, fallback string, hasFallback bool) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return "", "", false
	}
	parts := strings.SplitN(expr, ":-", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), true
	}
	return strings.TrimSpace(expr), "", false
}

func expandTemplate(input string, strict bool, resolver func(expr string) (string, bool, error)) (string, error) {
	if input == "" {
		return "", nil
	}

	var b strings.Builder
	for i := 0; i < len(input); {
		if i+1 < len(input) && input[i] == '$' && input[i+1] == '{' {
			end := strings.IndexByte(input[i+2:], '}')
			if end < 0 {
				if strict {
					return "", fmt.Errorf("unterminated variable expression")
				}
				b.WriteString(input[i:])
				break
			}
			end += i + 2
			expr := input[i+2 : end]
			val, ok, err := resolver(expr)
			if err != nil {
				return "", err
			}
			if ok {
				b.WriteString(val)
			} else {
				token := input[i : end+1]
				if strict {
					return "", fmt.Errorf("unable to resolve %q", token)
				}
				b.WriteString(token)
			}
			i = end + 1
			continue
		}
		b.WriteByte(input[i])
		i++
	}
	return b.String(), nil
}
