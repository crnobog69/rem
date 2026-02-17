package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"rem/internal/remfile"
	"rem/internal/shellcfg"
)

type Severity int

const (
	SeverityOK Severity = iota
	SeverityWarn
	SeverityFail
)

type Check struct {
	Severity Severity
	Name     string
	Detail   string
}

type Report struct {
	Checks []Check
}

func (r Report) Counts() (ok int, warn int, fail int) {
	for _, c := range r.Checks {
		switch c.Severity {
		case SeverityOK:
			ok++
		case SeverityWarn:
			warn++
		case SeverityFail:
			fail++
		}
	}
	return ok, warn, fail
}

func Run(remVersion string, remfilePath string) Report {
	out := Report{Checks: make([]Check, 0, 8)}

	out.Checks = append(out.Checks, Check{
		Severity: SeverityOK,
		Name:     "runtime",
		Detail:   fmt.Sprintf("rem=%s go=%s os=%s arch=%s", remVersion, runtime.Version(), runtime.GOOS, runtime.GOARCH),
	})

	cwd, err := os.Getwd()
	if err != nil {
		out.Checks = append(out.Checks, Check{
			Severity: SeverityFail,
			Name:     "cwd",
			Detail:   err.Error(),
		})
	} else {
		out.Checks = append(out.Checks, Check{
			Severity: SeverityOK,
			Name:     "cwd",
			Detail:   cwd,
		})
	}

	out.Checks = append(out.Checks, checkToolVersion("go", "go", "version"))
	out.Checks = append(out.Checks, checkToolVersion("git", "git", "--version"))
	out.Checks = append(out.Checks, checkShell())

	out.Checks = append(out.Checks, checkRemfile(remfilePath))
	out.Checks = append(out.Checks, checkUpdateRepo())

	return out
}

func checkToolVersion(name string, cmd string, args ...string) Check {
	path, err := exec.LookPath(cmd)
	if err != nil {
		return Check{
			Severity: SeverityWarn,
			Name:     name,
			Detail:   fmt.Sprintf("%s not found in PATH", cmd),
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	c := exec.CommandContext(ctx, path, args...)
	raw, err := c.Output()
	if err != nil {
		return Check{
			Severity: SeverityWarn,
			Name:     name,
			Detail:   fmt.Sprintf("%s found at %s, version probe failed: %v", cmd, path, err),
		}
	}
	line := strings.TrimSpace(string(raw))
	if line == "" {
		line = "found"
	}
	return Check{
		Severity: SeverityOK,
		Name:     name,
		Detail:   line,
	}
}

func checkRemfile(path string) Check {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return Check{
				Severity: SeverityWarn,
				Name:     "remfile",
				Detail:   fmt.Sprintf("%s does not exist", absPath),
			}
		}
		return Check{
			Severity: SeverityFail,
			Name:     "remfile",
			Detail:   fmt.Sprintf("stat failed: %v", err),
		}
	}

	rf, err := remfile.Load(path)
	if err != nil {
		return Check{
			Severity: SeverityFail,
			Name:     "remfile",
			Detail:   fmt.Sprintf("parse failed: %v", err),
		}
	}
	return Check{
		Severity: SeverityOK,
		Name:     "remfile",
		Detail:   fmt.Sprintf("%s parsed: tasks=%d default=%s", absPath, len(rf.Order), rf.DefaultTarget()),
	}
}

func checkUpdateRepo() Check {
	repo := strings.TrimSpace(os.Getenv("REM_UPDATE_REPO"))
	if repo == "" {
		return Check{
			Severity: SeverityWarn,
			Name:     "update",
			Detail:   "REM_UPDATE_REPO is empty",
		}
	}
	if strings.Count(repo, "/") != 1 {
		return Check{
			Severity: SeverityWarn,
			Name:     "update",
			Detail:   fmt.Sprintf("REM_UPDATE_REPO=%q should look like owner/repo", repo),
		}
	}
	return Check{
		Severity: SeverityOK,
		Name:     "update",
		Detail:   fmt.Sprintf("REM_UPDATE_REPO=%s", repo),
	}
}

func checkShell() Check {
	user := shellcfg.UserShell()
	if user == "" {
		user = "(unknown)"
	}

	taskBin, prefix, detail := shellcfg.ResolveTaskShell()
	probeArgs := []string{"--version"}
	if runtime.GOOS == "windows" {
		probeArgs = []string{"/C", "ver"}
	}

	versionText := ""
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, taskBin, probeArgs...)
	raw, err := cmd.Output()
	if err == nil {
		line := strings.TrimSpace(string(raw))
		if line != "" {
			versionText = strings.Split(line, "\n")[0]
		}
	}

	msg := fmt.Sprintf("user=%s task=%s", user, detail)
	if versionText != "" {
		msg += " (" + versionText + ")"
	}

	severity := SeverityOK
	if user != "(unknown)" {
		userBase := filepath.Base(strings.ToLower(user))
		taskBase := filepath.Base(strings.ToLower(taskBin))
		if userBase != taskBase {
			severity = SeverityWarn
			msg += " [different shells]"
		}
	}

	if len(prefix) == 0 {
		severity = SeverityWarn
	}

	return Check{
		Severity: severity,
		Name:     "shell",
		Detail:   msg,
	}
}
