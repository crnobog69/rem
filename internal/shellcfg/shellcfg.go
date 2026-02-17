package shellcfg

import (
	"os"
	"os/exec"
	"runtime"
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
	return ""
}

func ResolveTaskShell() (bin string, prefix []string, detail string) {
	if runtime.GOOS == "windows" {
		if v := strings.TrimSpace(os.Getenv("COMSPEC")); v != "" {
			return v, []string{"/C"}, v + " /C"
		}
		return "cmd", []string{"/C"}, "cmd /C"
	}

	if shell := strings.TrimSpace(os.Getenv("SHELL")); shell != "" {
		if p, err := exec.LookPath(shell); err == nil {
			return p, []string{"-c"}, p + " -c"
		}
	}
	if p, err := exec.LookPath("sh"); err == nil {
		return p, []string{"-c"}, p + " -c (fallback)"
	}
	return "/bin/sh", []string{"-c"}, "/bin/sh -c (fallback)"
}

