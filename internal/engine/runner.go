package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"rem/internal/remfile"
	"rem/internal/shellcfg"
)

type Runner struct {
	File     *remfile.File
	Jobs     int
	Stdout   io.Writer
	Stderr   io.Writer
	Colorize bool
}

type taskResult struct {
	name string
	err  error
}

type taskState struct {
	remaining int
	failedDep bool
	done      bool
}

func (r *Runner) Run(target string) error {
	if r.File == nil {
		return fmt.Errorf("runner has no loaded Remfile")
	}
	if r.Stdout == nil {
		r.Stdout = os.Stdout
	}
	if r.Stderr == nil {
		r.Stderr = os.Stderr
	}
	if target == "" {
		target = r.File.DefaultTarget()
	}
	target = r.File.ExpandString(target)
	if _, ok := r.File.Tasks[target]; !ok {
		return fmt.Errorf("target %q does not exist", target)
	}

	subset, err := r.collectSubset(target)
	if err != nil {
		return err
	}

	jobs := r.Jobs
	if jobs < 1 {
		jobs = runtime.NumCPU()
	}

	dependents := make(map[string][]string, len(subset))
	state := make(map[string]taskState, len(subset))
	for name := range subset {
		t := r.File.Tasks[name]
		rem := 0
		for _, dep := range r.File.ExpandList(t.Deps) {
			if subset[dep] {
				rem++
				dependents[dep] = append(dependents[dep], name)
			}
		}
		state[name] = taskState{remaining: rem}
	}

	ready := make([]string, 0, len(subset))
	for _, name := range r.File.Order {
		st, ok := state[name]
		if ok && st.remaining == 0 {
			ready = append(ready, name)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	taskCh := make(chan string)
	resultCh := make(chan taskResult, jobs)
	var wg sync.WaitGroup
	for i := 0; i < jobs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for name := range taskCh {
				resultCh <- taskResult{name: name, err: r.executeTask(ctx, name)}
			}
		}()
	}

	total := len(subset)
	completed := 0
	running := 0
	var firstErr error

	dispatch := func() {
		for running < jobs && len(ready) > 0 {
			name := ready[0]
			ready = ready[1:]

			st := state[name]
			if st.done {
				continue
			}

			if st.failedDep {
				st.done = true
				state[name] = st
				completed++
				if firstErr == nil {
					firstErr = fmt.Errorf("task %q blocked by failed dependency", name)
				}
				for _, dep := range dependents[name] {
					next := state[dep]
					next.remaining--
					next.failedDep = true
					state[dep] = next
					if next.remaining == 0 {
						ready = append(ready, dep)
					}
				}
				continue
			}

			running++
			taskCh <- name
		}
	}

	for completed < total {
		dispatch()
		if completed >= total {
			break
		}
		if running == 0 && len(ready) == 0 {
			break
		}

		res := <-resultCh
		running--

		st := state[res.name]
		if st.done {
			continue
		}
		st.done = true
		state[res.name] = st
		completed++

		if res.err != nil && firstErr == nil {
			firstErr = fmt.Errorf("task %q failed: %w", res.name, res.err)
		}

		for _, dep := range dependents[res.name] {
			next := state[dep]
			next.remaining--
			if res.err != nil {
				next.failedDep = true
			}
			state[dep] = next
			if next.remaining == 0 {
				ready = append(ready, dep)
			}
		}
	}

	close(taskCh)
	wg.Wait()

	return firstErr
}

func (r *Runner) executeTask(ctx context.Context, taskName string) error {
	task := r.File.Tasks[taskName]

	upToDate, reason, err := r.isUpToDate(task)
	if err != nil {
		return err
	}
	if upToDate {
		fmt.Fprintf(r.Stdout, "%s %s (%s)\n", r.paint("33", "[skip]"), taskName, reason)
		return nil
	}

	fmt.Fprintf(r.Stdout, "%s %s\n", r.paint("34", "[run]"), taskName)
	for _, rawCmd := range task.Cmds {
		rawCmd = r.File.ExpandString(rawCmd)
		cmdText := strings.TrimSpace(rawCmd)
		if cmdText == "" {
			continue
		}

		fmt.Fprintf(r.Stdout, "  %s %s\n", r.paint("2", "$"), cmdText)
		cmd := shellCommand(ctx, cmdText)
		cmd.Stdout = r.Stdout
		cmd.Stderr = r.Stderr
		cmd.Stdin = os.Stdin
		cmd.Env = os.Environ()
		taskDir := r.File.ExpandString(task.Dir)
		if taskDir != "" {
			if filepath.IsAbs(taskDir) {
				cmd.Dir = taskDir
			} else {
				cmd.Dir = filepath.Join(r.File.Dir, taskDir)
			}
		} else {
			cmd.Dir = r.File.Dir
		}
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) isUpToDate(t *remfile.Task) (bool, string, error) {
	outputs := r.File.ExpandList(t.Outputs)
	inputs := r.File.ExpandList(t.Inputs)

	if len(outputs) == 0 {
		return false, "no outputs", nil
	}

	oldestOutput := time.Time{}
	for _, out := range outputs {
		full := out
		if !filepath.IsAbs(full) {
			full = filepath.Join(r.File.Dir, out)
		}
		info, err := os.Stat(full)
		if err != nil {
			if os.IsNotExist(err) {
				return false, "missing output", nil
			}
			return false, "", err
		}
		if oldestOutput.IsZero() || info.ModTime().Before(oldestOutput) {
			oldestOutput = info.ModTime()
		}
	}

	if len(inputs) == 0 {
		return true, "outputs exist", nil
	}

	newestInput := time.Time{}
	for _, in := range inputs {
		paths, err := resolveInputPaths(r.File.Dir, in)
		if err != nil {
			return false, "", err
		}
		for _, p := range paths {
			info, err := os.Stat(p)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return false, "", err
			}
			if info.ModTime().After(newestInput) {
				newestInput = info.ModTime()
			}
		}
	}

	if newestInput.IsZero() {
		return true, "no matching inputs", nil
	}
	if newestInput.After(oldestOutput) {
		return false, "input newer than output", nil
	}
	return true, "outputs newer than inputs", nil
}

func resolveInputPaths(baseDir, value string) ([]string, error) {
	full := value
	if !filepath.IsAbs(full) {
		full = filepath.Join(baseDir, value)
	}

	if hasGlob(full) {
		matches, err := filepath.Glob(full)
		if err != nil {
			return nil, err
		}
		return matches, nil
	}
	return []string{full}, nil
}

func hasGlob(p string) bool {
	return strings.ContainsAny(p, "*?[")
}

func shellCommand(ctx context.Context, cmdText string) *exec.Cmd {
	bin, prefix, _ := shellcfg.ResolveTaskShell()
	args := append(prefix, cmdText)
	return exec.CommandContext(ctx, bin, args...)
}

func (r *Runner) collectSubset(target string) (map[string]bool, error) {
	subset := make(map[string]bool)
	vis := make(map[string]int)
	stack := make([]string, 0, 8)

	var dfs func(string) error
	dfs = func(name string) error {
		switch vis[name] {
		case 1:
			return fmt.Errorf("dependency cycle detected: %s -> %s", strings.Join(stack, " -> "), name)
		case 2:
			return nil
		}
		t, ok := r.File.Tasks[name]
		if !ok {
			return fmt.Errorf("undefined task %q", name)
		}

		vis[name] = 1
		stack = append(stack, name)
		subset[name] = true
		for _, dep := range r.File.ExpandList(t.Deps) {
			if err := dfs(dep); err != nil {
				return err
			}
		}
		stack = stack[:len(stack)-1]
		vis[name] = 2
		return nil
	}

	if err := dfs(target); err != nil {
		return nil, err
	}
	return subset, nil
}

func (r *Runner) paint(code string, value string) string {
	if !r.Colorize || runtime.GOOS == "windows" {
		return value
	}
	return "\x1b[" + code + "m" + value + "\x1b[0m"
}
