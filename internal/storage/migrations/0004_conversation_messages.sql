CREATE TABLE conversation_messages (
    session_id TEXT NOT NULL,
    sequence INTEGER NOT NULL,
    role TEXT NOT NULL,
    recorded_at TEXT,
    payload_json TEXT NOT NULL,
    PRIMARY KEY (session_id, sequence)
);
CREATE TABLE conversation_revisions (
    session_id TEXT PRIMARY KEY,
    content_hash TEXT NOT NULL,
    revision INTEGER NOT NULL,
    refreshed_at TEXT NOT NULL
);
