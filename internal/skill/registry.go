// Package skill provides skill loading and management.
package skill

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Skill represents a loaded skill from a SKILL.md file.
type Skill struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Category    string   `yaml:"category"`
	Tags        []string `yaml:"tags"`
	Content     string   // full markdown body
	Path        string   // file path
}

// Registry holds all loaded skills.
type Registry struct {
	skills map[string]*Skill
	dirs   []string
}

// NewRegistry creates an empty skill registry.
func NewRegistry() *Registry {
	return &Registry{skills: make(map[string]*Skill)}
}

// AddDir adds a directory to scan for skills.
func (r *Registry) AddDir(dir string) {
	r.dirs = append(r.dirs, dir)
}

// LoadAll scans all configured directories and loads skills.
func (r *Registry) LoadAll() error {
	for _, dir := range r.dirs {
		if err := r.loadDir(dir); err != nil {
			return fmt.Errorf("load skills from %s: %w", dir, err)
		}
	}
	return nil
}

func (r *Registry) loadDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillDir := filepath.Join(dir, entry.Name())
		skillFile := filepath.Join(skillDir, "SKILL.md")
		if _, err := os.Stat(skillFile); os.IsNotExist(err) {
			// Check for lowercase
			skillFile = filepath.Join(skillDir, "skill.md")
			if _, err := os.Stat(skillFile); os.IsNotExist(err) {
				continue
			}
		}
		skill, err := parseSkillFile(skillFile)
		if err != nil {
			continue // skip malformed skills
		}
		skill.Path = skillDir
		r.skills[skill.Name] = skill
	}
	return nil
}

// yamlFrontmatterRegex matches YAML between --- delimiters.
var yamlFrontmatterRegex = regexp.MustCompile(`^---\s*\n([\s\S]*?)\n---`)

func parseSkillFile(path string) (*Skill, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var raw strings.Builder
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	for scanner.Scan() {
		raw.WriteString(scanner.Text() + "\n")
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	text := raw.String()
	skill := &Skill{Name: filepath.Base(filepath.Dir(path))}

	if m := yamlFrontmatterRegex.FindStringSubmatch(text); m != nil {
		parseYAMLFrontmatter(m[1], skill)
		// Everything after the closing --- is the body
		idx := strings.Index(text, "\n---")
		if idx2 := strings.Index(text[idx+4:], "\n---"); idx2 != -1 {
			// Skip trailing ---
		}
		after := text[strings.LastIndex(text, "---")+3:]
		skill.Content = strings.TrimSpace(after)
	} else {
		skill.Content = strings.TrimSpace(text)
	}

	// Use first heading as name if frontmatter didn't provide one
	if skill.Name != "" && skill.Description == "" {
		skill.Description = skill.Content
	}

	return skill, nil
}

func parseYAMLFrontmatter(yamlStr string, skill *Skill) {
	for _, line := range strings.Split(yamlStr, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch strings.ToLower(key) {
		case "name":
			skill.Name = stripQuotes(val)
		case "description":
			skill.Description = stripQuotes(val)
		case "category":
			skill.Category = stripQuotes(val)
		case "tags":
			// Tags can be YAML list or comma-separated
			val = stripQuotes(val)
			if strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]") {
				val = strings.Trim(val, "[]")
				for _, t := range strings.Split(val, ",") {
					skill.Tags = append(skill.Tags, strings.TrimSpace(stripQuotes(t)))
				}
			} else {
				skill.Tags = append(skill.Tags, val)
			}
		}
	}
}

func stripQuotes(s string) string {
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1]
	}
	return s
}

// Get returns a skill by name.
func (r *Registry) Get(name string) (*Skill, bool) {
	skill, ok := r.skills[name]
	return skill, ok
}

// List returns all loaded skill names.
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.skills))
	for name := range r.skills {
		names = append(names, name)
	}
	return names
}

// All returns all skills.
func (r *Registry) All() map[string]*Skill {
	return r.skills
}

// SystemPrompt returns a system prompt fragment listing available skills.
func (r *Registry) SystemPrompt() string {
	if len(r.skills) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n## Available Skills\n\n")
	sb.WriteString("You have access to the following skills. Use `/skill-name` to invoke a specific skill, ")
	sb.WriteString("or mention a skill's purpose and the system will load its instructions.\n\n")
	for name, skill := range r.skills {
		desc := skill.Description
		if len(desc) > 100 {
			desc = desc[:100] + "..."
		}
		sb.WriteString(fmt.Sprintf("- **%s**", name))
		if skill.Category != "" {
			sb.WriteString(fmt.Sprintf(" [%s]", skill.Category))
		}
		if desc != "" {
			sb.WriteString(fmt.Sprintf(": %s", desc))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
