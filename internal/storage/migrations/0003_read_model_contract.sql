ALTER TABLE sessions ADD COLUMN summary_json TEXT NOT NULL DEFAULT '{}';
ALTER TABLE deliveries ADD COLUMN session_id TEXT NOT NULL DEFAULT '';
ALTER TABLE deliveries ADD COLUMN turn_id TEXT NOT NULL DEFAULT '';
ALTER TABLE deliveries ADD COLUMN event_type TEXT NOT NULL DEFAULT '';
ALTER TABLE deliveries ADD COLUMN event_cursor INTEGER NOT NULL DEFAULT 0;
ALTER TABLE deliveries ADD COLUMN terminal INTEGER NOT NULL DEFAULT 0;

CREATE TABLE read_model_refreshes (
    kind TEXT PRIMARY KEY,
    refreshed_at TEXT NOT NULL,
    revision INTEGER NOT NULL DEFAULT 1
);

CREATE INDEX idx_sessions_page ON sessions(updated_at DESC, id DESC);
CREATE INDEX idx_intents_page ON intents(updated_at DESC, id DESC);
CREATE INDEX idx_open_loops_page ON open_loops(updated_at DESC, id DESC);
CREATE INDEX idx_deliveries_page ON deliveries(updated_at DESC, id DESC);
CREATE INDEX idx_deliveries_session_status ON deliveries(session_id, status);
