-- Codex Go: Initial database schema
-- Migration version: 1

CREATE TABLE IF NOT EXISTS schema_version (
    version     INTEGER PRIMARY KEY,
    applied_at  INTEGER NOT NULL
);

-- Provider
CREATE TABLE IF NOT EXISTS providers (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    icon        TEXT DEFAULT '',
    icon_color  TEXT DEFAULT '',
    category    TEXT DEFAULT 'third_party',
    notes       TEXT DEFAULT '',
    in_failover_queue INTEGER DEFAULT 1,
    is_current  INTEGER DEFAULT 0,
    api_format            TEXT DEFAULT '',
    cost_multiplier       TEXT DEFAULT '',
    limit_daily_usd       TEXT DEFAULT '',
    limit_monthly_usd     TEXT DEFAULT '',
    is_full_url           INTEGER DEFAULT 0,
    endpoint_auto_select  INTEGER DEFAULT 0,
    prompt_cache_key      TEXT DEFAULT '',
    max_output_tokens     INTEGER DEFAULT 0,
    custom_user_agent     TEXT DEFAULT '',
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);

-- Backend endpoints
CREATE TABLE IF NOT EXISTS backends (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    provider_id TEXT NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    label       TEXT NOT NULL,
    api_key     TEXT NOT NULL,
    base_url    TEXT NOT NULL,
    weight      INTEGER DEFAULT 1,
    headers     TEXT DEFAULT '{}',
    health_status TEXT DEFAULT 'unknown',
    last_probe_at INTEGER DEFAULT 0,
    fail_count  INTEGER DEFAULT 0,
    created_at  INTEGER NOT NULL,
    UNIQUE(provider_id, label)
);

-- Backend supported models
CREATE TABLE IF NOT EXISTS backend_models (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    backend_id  INTEGER NOT NULL REFERENCES backends(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    type        TEXT DEFAULT '',
    context_length INTEGER DEFAULT 0,
    UNIQUE(backend_id, name)
);

-- Provider presets
CREATE TABLE IF NOT EXISTS provider_presets (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,
    category    TEXT NOT NULL,
    icon        TEXT DEFAULT '',
    icon_color  TEXT DEFAULT '',
    website_url TEXT DEFAULT '',
    api_key_url TEXT DEFAULT '',
    sort_order  INTEGER DEFAULT 0
);

-- Agents
CREATE TABLE IF NOT EXISTS agents (
    name            TEXT PRIMARY KEY,
    display_name    TEXT NOT NULL,
    provider        TEXT NOT NULL DEFAULT '',
    model           TEXT NOT NULL DEFAULT '',
    system_prompt   TEXT DEFAULT '',
    max_turns       INTEGER DEFAULT 50,
    reasoning_effort TEXT DEFAULT 'medium',
    tools_mode      TEXT DEFAULT 'all',
    tools_list      TEXT DEFAULT '[]',
    mcp_mode        TEXT DEFAULT 'all',
    mcp_list        TEXT DEFAULT '[]',
    skills_mode     TEXT DEFAULT 'all',
    skills_list     TEXT DEFAULT '[]',
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL
);

-- Agent persistent memory
CREATE TABLE IF NOT EXISTS agent_memory (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_name  TEXT NOT NULL REFERENCES agents(name) ON DELETE CASCADE,
    key         TEXT NOT NULL,
    value       TEXT NOT NULL,
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL,
    UNIQUE(agent_name, key)
);

-- Sessions
CREATE TABLE IF NOT EXISTS sessions (
    id          TEXT PRIMARY KEY,
    title       TEXT DEFAULT '',
    agent_name  TEXT NOT NULL DEFAULT 'default',
    provider    TEXT DEFAULT '',
    model       TEXT DEFAULT '',
    msg_count   INTEGER DEFAULT 0,
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);

-- Messages
CREATE TABLE IF NOT EXISTS messages (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id  TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    role        TEXT NOT NULL,
    content     TEXT NOT NULL,
    tool_calls  TEXT DEFAULT '[]',
    token_count INTEGER DEFAULT 0,
    created_at  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id, id);

-- Global tools registry
CREATE TABLE IF NOT EXISTS tools (
    name        TEXT PRIMARY KEY,
    description TEXT DEFAULT '',
    category    TEXT DEFAULT 'optional',
    risk        TEXT DEFAULT 'low',
    approval_required INTEGER DEFAULT 0,
    enabled     INTEGER DEFAULT 1,
    sort_order  INTEGER DEFAULT 0
);

-- Global MCP servers
CREATE TABLE IF NOT EXISTS mcp_servers (
    name        TEXT PRIMARY KEY,
    description TEXT DEFAULT '',
    command     TEXT NOT NULL,
    args        TEXT DEFAULT '[]',
    env         TEXT DEFAULT '{}',
    enabled     INTEGER DEFAULT 1,
    sort_order  INTEGER DEFAULT 0
);

-- Spec/Plan tasks
CREATE TABLE IF NOT EXISTS tasks (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    spec_file   TEXT DEFAULT '',
    plan_file   TEXT DEFAULT '',
    title       TEXT NOT NULL,
    description TEXT DEFAULT '',
    status      TEXT DEFAULT 'pending',
    depends_on  TEXT DEFAULT '[]',
    agent_name  TEXT DEFAULT '',
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);

-- Scheduled jobs
CREATE TABLE IF NOT EXISTS scheduled_jobs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL,
    cron_expr   TEXT NOT NULL,
    prompt      TEXT NOT NULL,
    agent_name  TEXT NOT NULL,
    enabled     INTEGER DEFAULT 1,
    last_run    INTEGER DEFAULT 0,
    next_run    INTEGER DEFAULT 0,
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);

-- Job execution logs
CREATE TABLE IF NOT EXISTS job_logs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id      INTEGER NOT NULL REFERENCES scheduled_jobs(id) ON DELETE CASCADE,
    status      TEXT NOT NULL,
    output      TEXT DEFAULT '',
    error       TEXT DEFAULT '',
    duration_ms INTEGER DEFAULT 0,
    created_at  INTEGER NOT NULL
);

-- API usage logs
CREATE TABLE IF NOT EXISTS usage_logs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    provider_id TEXT NOT NULL,
    backend_id  INTEGER DEFAULT 0,
    model       TEXT DEFAULT '',
    input_tokens  INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    cost_est      REAL DEFAULT 0.0,
    created_at INTEGER NOT NULL
);

-- Daily usage aggregation
CREATE TABLE IF NOT EXISTS usage_daily (
    date        TEXT NOT NULL,
    provider_id TEXT NOT NULL,
    model       TEXT NOT NULL,
    input_tokens  INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    request_count INTEGER DEFAULT 0,
    cost_est      REAL DEFAULT 0.0,
    PRIMARY KEY (date, provider_id, model)
);

-- Skills index
CREATE TABLE IF NOT EXISTS skills (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,
    description TEXT DEFAULT '',
    tags        TEXT DEFAULT '[]',
    file_path   TEXT NOT NULL,
    source      TEXT DEFAULT '',
    enabled     INTEGER DEFAULT 1,
    created_at  INTEGER NOT NULL
);

-- Plugins
CREATE TABLE IF NOT EXISTS plugins (
    id          TEXT PRIMARY KEY,
    version     TEXT DEFAULT '',
    description TEXT DEFAULT '',
    source_url  TEXT DEFAULT '',
    status      TEXT DEFAULT 'stopped',
    pid         INTEGER DEFAULT 0,
    installed_at INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);
