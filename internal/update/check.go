package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type latestRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

func CheckLatest(currentVersion, defaultRepo string) (string, error) {
	if currentVersion == "" || currentVersion == "dev" {
		return "", nil
	}
	if os.Getenv("REM_NO_UPDATE_CHECK") == "1" {
		return "", nil
	}

	repo := os.Getenv("REM_UPDATE_REPO")
	if repo == "" {
		repo = defaultRepo
	}
	if repo == "" || !strings.Contains(repo, "/") {
		return "", nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "rem-cli")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil
	}

	var latest latestRelease
	if err := json.NewDecoder(resp.Body).Decode(&latest); err != nil {
		return "", err
	}
	if latest.TagName == "" {
		return "", nil
	}

	if !isNewer(latest.TagName, currentVersion) {
		return "", nil
	}

	url := latest.HTMLURL
	if url == "" {
		url = "https://github.com/" + repo + "/releases/latest"
	}
	return fmt.Sprintf("New rem version available: %s (current %s)\nUpdate: %s", latest.TagName, currentVersion, url), nil
}

func isNewer(latest, current string) bool {
	la := parseSemver(latest)
	cu := parseSemver(current)
	if len(la) == 0 || len(cu) == 0 {
		return normalize(latest) != normalize(current)
	}

	for i := 0; i < 3; i++ {
		if la[i] > cu[i] {
			return true
		}
		if la[i] < cu[i] {
			return false
		}
	}
	return false
}

func parseSemver(v string) []int {
	v = normalize(v)
	parts := strings.Split(v, ".")
	if len(parts) == 0 {
		return nil
	}

	out := []int{0, 0, 0}
	for i := 0; i < 3 && i < len(parts); i++ {
		p := parts[i]
		if p == "" {
			return nil
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		out[i] = n
	}
	return out
}

func normalize(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}
