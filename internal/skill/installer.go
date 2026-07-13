// Package skill provides skill installation from GitHub repos.
package skill

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/yeshenlougu/codex/internal/store"
)

// Installer manages skill installation from GitHub repositories.
type Installer struct {
	store     *store.SkillStore
	skillsDir string
}

// NewInstaller creates an installer with the given store and target directory.
func NewInstaller(store *store.SkillStore, skillsDir string) *Installer {
	os.MkdirAll(skillsDir, 0700)
	return &Installer{store: store, skillsDir: skillsDir}
}

// DiscoverResult is a skill found in a GitHub repo.
type DiscoverResult struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Directory   string `json:"directory"`
	RepoOwner   string `json:"repo_owner"`
	RepoName    string `json:"repo_name"`
	RepoBranch  string `json:"repo_branch"`
	ReadmeURL   string `json:"readme_url"`
	Installed   bool   `json:"installed"`
}

// Discover scans all configured repos for installable skills via GitHub API.
func (inst *Installer) Discover() ([]DiscoverResult, error) {
	repos := inst.store.Repos()
	var all []DiscoverResult

	for _, repo := range repos {
		if !repo.Enabled {
			continue
		}
		results, err := inst.scanRepo(repo)
		if err != nil {
			log.Printf("[skill] scan repo %s/%s: %v", repo.Owner, repo.Name, err)
			continue
		}
		all = append(all, results...)
	}
	return all, nil
}

// scanRepo uses the GitHub API to list top-level directories (potential skills).
func (inst *Installer) scanRepo(repo store.SkillRepo) ([]DiscoverResult, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents?ref=%s", repo.Owner, repo.Name, repo.Branch)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("github api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api status %d", resp.StatusCode)
	}

	var entries []struct {
		Name string `json:"name"`
		Type string `json:"type"`
		Path string `json:"path"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	var results []DiscoverResult
	for _, e := range entries {
		if e.Type != "dir" {
			continue
		}
		// Check if this directory has a SKILL.md
		hasSkill, name, desc := inst.probeSkillDir(repo, e.Path)
		if !hasSkill {
			continue
		}
		key := fmt.Sprintf("%s/%s:%s", repo.Owner, repo.Name, e.Name)
		results = append(results, DiscoverResult{
			Key:         key,
			Name:        name,
			Description: desc,
			Directory:   e.Name,
			RepoOwner:   repo.Owner,
			RepoName:    repo.Name,
			RepoBranch:  repo.Branch,
			ReadmeURL:   fmt.Sprintf("https://github.com/%s/%s/blob/%s/%s/SKILL.md", repo.Owner, repo.Name, repo.Branch, e.Path),
			Installed:   inst.store.SkillExistsByKey(repo.Owner, repo.Name, e.Name),
		})
	}
	return results, nil
}

// probeSkillDir checks if a directory has a SKILL.md and extracts metadata.
func (inst *Installer) probeSkillDir(repo store.SkillRepo, dirPath string) (bool, string, string) {
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s/SKILL.md", repo.Owner, repo.Name, repo.Branch, dirPath)
	resp, err := http.Get(url)
	if err != nil {
		return false, "", ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, "", ""
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	content := string(body)
	name := dirPath
	desc := ""

	// Try to parse YAML frontmatter
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content[3:], "---", 2)
		if len(parts) >= 1 {
			for _, line := range strings.Split(parts[0], "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "name:") {
					name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
				}
				if strings.HasPrefix(line, "description:") {
					desc = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
				}
			}
		}
	}

	return true, name, desc
}

// Install clones a skill from GitHub and saves it locally.
func (inst *Installer) Install(owner, repoName, branch, directory string) (*store.InstalledSkill, error) {
	key := fmt.Sprintf("%s/%s:%s", owner, repoName, directory)
	if inst.store.SkillExistsByKey(owner, repoName, directory) {
		return nil, fmt.Errorf("skill %s already installed", key)
	}

	// Clone into temp dir
	tmpDir, err := os.MkdirTemp("", "codex-skill-")
	if err != nil {
		return nil, fmt.Errorf("temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	cloneURL := fmt.Sprintf("https://github.com/%s/%s.git", owner, repoName)
	cmd := exec.Command("git", "clone", "--depth", "1", "--filter=blob:none", "-b", branch, cloneURL, tmpDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git clone: %v — %s", err, string(output))
	}

	// Locate the skill directory
	srcDir := filepath.Join(tmpDir, directory)
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("skill directory %s not found in repo", directory)
	}

	// Parse SKILL.md for metadata
	name := directory
	desc := ""
	skillMDPath := filepath.Join(srcDir, "SKILL.md")
	if data, err := os.ReadFile(skillMDPath); err == nil {
		content := string(data)
		if strings.HasPrefix(content, "---") {
			parts := strings.SplitN(content[3:], "---", 2)
			if len(parts) >= 1 {
				for _, line := range strings.Split(parts[0], "\n") {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "name:") {
						name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
					}
					if strings.HasPrefix(line, "description:") {
						desc = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
					}
				}
			}
		}
	}

	// Copy to skills dir
	destDir := filepath.Join(inst.skillsDir, directory)
	os.RemoveAll(destDir) // remove old if exists
	if err := copyDir(srcDir, destDir); err != nil {
		return nil, fmt.Errorf("copy: %w", err)
	}

	// Compute hash of SKILL.md
	hash := ""
	if data, err := os.ReadFile(filepath.Join(destDir, "SKILL.md")); err == nil {
		h := sha256.Sum256(data)
		hash = fmt.Sprintf("%x", h)
	}

	// Save to store
	sk := &store.InstalledSkill{
		ID:          uuid.New().String(),
		Name:        name,
		Description: desc,
		Directory:   directory,
		RepoOwner:   owner,
		RepoName:    repoName,
		RepoBranch:  branch,
		ReadmeURL:   fmt.Sprintf("https://github.com/%s/%s/blob/%s/%s/SKILL.md", owner, repoName, branch, directory),
		ContentHash: hash,
		Enabled:     true,
	}

	if err := inst.store.AddSkill(sk); err != nil {
		return nil, fmt.Errorf("save: %w", err)
	}

	// Reload skill registry to make it available to agents
	// (this is handled by the caller / api layer)

	log.Printf("[skill] installed: %s (%s/%s -> %s)", sk.Name, owner, repoName, destDir)
	return sk, nil
}

// Uninstall removes a skill and creates a backup.
func (inst *Installer) Uninstall(id string) (string, error) {
	sk, _ := inst.store.GetSkill(id)
	if sk == nil {
		return "", fmt.Errorf("skill %s not found", id)
	}

	skillDir := filepath.Join(inst.skillsDir, sk.Directory)
	backupDir := filepath.Join(filepath.Dir(inst.skillsDir), "skill-backups")
	os.MkdirAll(backupDir, 0700)

	// Create backup archive
	backupPath := filepath.Join(backupDir, fmt.Sprintf("%s-%s.tar.gz", sk.Directory, sk.ID[:8]))
	if _, err := os.Stat(skillDir); err == nil {
		cmd := exec.Command("tar", "-czf", backupPath, "-C", inst.skillsDir, sk.Directory)
		cmd.Run() // best-effort backup
	}

	// Remove skill directory
	os.RemoveAll(skillDir)

	// Remove from store
	if err := inst.store.RemoveSkill(id); err != nil {
		return backupPath, fmt.Errorf("store: %w", err)
	}

	log.Printf("[skill] uninstalled: %s (backup: %s)", sk.Name, backupPath)
	return backupPath, nil
}

// CheckUpdates compares local content hash with remote HEAD for each installed skill.
func (inst *Installer) CheckUpdates() ([]store.InstalledSkill, error) {
	skills := inst.store.Skills()
	var outdated []store.InstalledSkill
	for _, sk := range skills {
		remoteHash, err := getRemoteHash(sk.RepoOwner, sk.RepoName, sk.RepoBranch, sk.Directory)
		if err != nil {
			log.Printf("[skill] update check %s: %v", sk.Name, err)
			continue
		}
		if remoteHash != "" && remoteHash != sk.ContentHash {
			outdated = append(outdated, sk)
		}
	}
	return outdated, nil
}

func getRemoteHash(owner, repo, branch, directory string) (string, error) {
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s/SKILL.md", owner, repo, branch, directory)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	h := sha256.Sum256(body)
	return fmt.Sprintf("%x", h), nil
}

// copyDir recursively copies a directory.
func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0700); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			srcFile, err := os.Open(srcPath)
			if err != nil {
				return err
			}
			dstFile, err := os.Create(dstPath)
			if err != nil {
				srcFile.Close()
				return err
			}
			io.Copy(dstFile, srcFile)
			srcFile.Close()
			dstFile.Close()
		}
	}
	return nil
}
