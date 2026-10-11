CREATE TABLE IF NOT EXISTS snapshots (
    name TEXT PRIMARY KEY,
    data BLOB NOT NULL,
    backup BLOB,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL,
    payload_json TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_user_updated ON sessions(user_id, updated_at);

CREATE TABLE IF NOT EXISTS intents (
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL DEFAULT 0,
    session_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    payload_json TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_intents_session_status ON intents(session_id, status);
CREATE INDEX IF NOT EXISTS idx_intents_user_updated ON intents(user_id, updated_at);

CREATE TABLE IF NOT EXISTS open_loops (
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL DEFAULT 0,
    session_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    payload_json TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_open_loops_session_status ON open_loops(session_id, status);
CREATE INDEX IF NOT EXISTS idx_open_loops_user_updated ON open_loops(user_id, updated_at);

CREATE TABLE IF NOT EXISTS deliveries (
    id TEXT PRIMARY KEY,
    intent_id TEXT NOT NULL DEFAULT '',
    user_id INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    payload_json TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_deliveries_intent_updated ON deliveries(intent_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_deliveries_user_status ON deliveries(user_id, status);

CREATE TABLE IF NOT EXISTS events (
    cursor INTEGER PRIMARY KEY AUTOINCREMENT,
    id TEXT NOT NULL UNIQUE,
    type TEXT NOT NULL,
    created_at TEXT NOT NULL,
    session_id TEXT NOT NULL DEFAULT '',
    turn_id TEXT NOT NULL DEFAULT '',
    intent_id TEXT NOT NULL DEFAULT '',
    delivery_id TEXT NOT NULL DEFAULT '',
    payload_json TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_events_created_cursor ON events(created_at, cursor);
CREATE INDEX IF NOT EXISTS idx_events_session_cursor ON events(session_id, cursor);
CREATE INDEX IF NOT EXISTS idx_events_intent_cursor ON events(intent_id, cursor);
