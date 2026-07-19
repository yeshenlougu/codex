-- Migration 002: Skills FTS5 full-text search index
-- Replaces LIKE-based search with SQLite FTS5 for faster, ranked results.

-- Create virtual FTS5 table for skills
CREATE VIRTUAL TABLE IF NOT EXISTS skills_fts USING fts5(
    name,
    description,
    tags,
    content='skills',
    content_rowid='id'
);

-- Populate FTS index with existing skills data
INSERT INTO skills_fts(rowid, name, description, tags)
SELECT id, name, description, tags FROM skills WHERE enabled = 1;

-- Triggers to keep FTS in sync on INSERT
CREATE TRIGGER IF NOT EXISTS skills_ai AFTER INSERT ON skills BEGIN
    INSERT INTO skills_fts(rowid, name, description, tags)
    VALUES (new.id, new.name, new.description, new.tags);
END;

-- Trigger on UPDATE
CREATE TRIGGER IF NOT EXISTS skills_au AFTER UPDATE ON skills BEGIN
    UPDATE skills_fts SET name = new.name, description = new.description, tags = new.tags
    WHERE rowid = old.id;
END;

-- Trigger on DELETE
CREATE TRIGGER IF NOT EXISTS skills_ad AFTER DELETE ON skills BEGIN
    DELETE FROM skills_fts WHERE rowid = old.id;
END;
