// Package update provides release update checking for the desktop app.
// This is used by the Electron app to check for new releases on GitHub.
package update

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Release represents a GitHub release.
type Release struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	PublishedAt time.Time `json:"published_at"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	Assets      []Asset   `json:"assets"`
}

// Asset is a downloadable file in a release.
type Asset struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	DownloadURL string `json:"browser_download_url"`
}

// LatestRelease fetches the latest release from a GitHub repo.
func LatestRelease(owner, repo string) (*Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "codex-go-updater")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("parse release: %w", err)
	}

	return &release, nil
}

// CheckUpdate compares current version against latest release.
func CheckUpdate(owner, repo, currentVersion string) (*Release, bool, error) {
	latest, err := LatestRelease(owner, repo)
	if err != nil {
		return nil, false, err
	}

	// Simple version comparison
	hasUpdate := strings.TrimPrefix(latest.TagName, "v") != strings.TrimPrefix(currentVersion, "v")

	return latest, hasUpdate, nil
}
