package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yeshenlougu/codex/internal/provider"
)

// Session is a saved conversation.
type Session struct {
	ID        string             `json:"id"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
	Model     string             `json:"model"`
	Provider  string             `json:"provider"`
	Title     string             `json:"title"`
	Messages  []provider.Message `json:"messages"`
}

// Summary is a lightweight session listing entry.
type Summary struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Model     string    `json:"model"`
	Provider  string    `json:"provider"`
	Title     string    `json:"title"`
	MsgCount  int       `json:"msg_count"`
}

// Store manages session persistence as JSON files.
type Store struct {
	dir string
}

// NewStore creates a session store.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}
	return &Store{dir: dir}, nil
}

func (s *Store) path(id string) string {
	return filepath.Join(s.dir, id+".json")
}

// Save persists a session.
func (s *Store) Save(session *Session) error {
	session.UpdatedAt = time.Now()
	if session.CreatedAt.IsZero() {
		session.CreatedAt = session.UpdatedAt
	}
	if session.Title == "" {
		session.Title = extractTitle(session.Messages)
	}
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return os.WriteFile(s.path(session.ID), data, 0600)
}

// Load reads a session.
func (s *Store) Load(id string) (*Session, error) {
	data, err := os.ReadFile(s.path(id))
	if err != nil {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("parse session: %w", err)
	}
	return &sess, nil
}

// Delete removes a session.
func (s *Store) Delete(id string) error {
	return os.Remove(s.path(id))
}

// List returns summaries sorted by most recently updated.
func (s *Store) List(limit int) ([]Summary, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, nil
	}
	var summaries []Summary
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		sess, err := s.Load(id)
		if err != nil {
			continue
		}
		summaries = append(summaries, Summary{
			ID:        sess.ID,
			CreatedAt: sess.CreatedAt,
			UpdatedAt: sess.UpdatedAt,
			Model:     sess.Model,
			Provider:  sess.Provider,
			Title:     sess.Title,
			MsgCount:  len(sess.Messages),
		})
	}
	SortByTime(summaries)
	if limit > 0 && len(summaries) > limit {
		summaries = summaries[:limit]
	}
	return summaries, nil
}

func extractTitle(msgs []provider.Message) string {
	for _, m := range msgs {
		if m.Role == "user" {
			t := m.Content
			if len(t) > 60 {
				t = t[:60] + "..."
			}
			return t
		}
	}
	return "New Session"
}

// SortByTime sorts by UpdatedAt descending.
func SortByTime(summaries []Summary) {
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].UpdatedAt.After(summaries[j].UpdatedAt)
	})
}
