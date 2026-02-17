package shellcfg

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

func UserShell() string {
	if runtime.GOOS == "windows" {
		if v := strings.TrimSpace(os.Getenv("COMSPEC")); v != "" {
			return v
		}
		return "cmd.exe"
	}
	if v := strings.TrimSpace(os.Getenv("SHELL")); v != "" {
		return v
	}
	if v := parentShell(); v != "" {
		return v
	}
	return ""
}

func ResolveTaskShell() (bin string, prefix []string, detail string) {
	if runtime.GOOS == "windows" {
		if v := strings.TrimSpace(os.Getenv("COMSPEC")); v != "" {
			return v, []string{"/C"}, v + " /C"
		}
		return "cmd", []string{"/C"}, "cmd /C"
	}

	if shell := strings.TrimSpace(os.Getenv("REM_SHELL")); shell != "" {
		if p, err := exec.LookPath(shell); err == nil {
			return p, []string{"-c"}, p + " -c (REM_SHELL)"
		}
	}
	if shell := UserShell(); shell != "" {
		if p, err := exec.LookPath(shell); err == nil {
			return p, []string{"-c"}, p + " -c"
		}
	}
	if p, err := exec.LookPath("sh"); err == nil {
		return p, []string{"-c"}, p + " -c (fallback)"
	}
	return "/bin/sh", []string{"-c"}, "/bin/sh -c (fallback)"
}

func parentShell() string {
	if runtime.GOOS == "windows" {
		return ""
	}
	ppid := os.Getppid()
	if ppid <= 1 {
		return ""
	}
	out, err := exec.Command("ps", "-p", strconv.Itoa(ppid), "-o", "comm=").Output()
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(out))
	if line == "" {
		return ""
	}
	name := strings.TrimPrefix(strings.Split(line, "\n")[0], "-")
	if name == "" {
		return ""
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return name
}
