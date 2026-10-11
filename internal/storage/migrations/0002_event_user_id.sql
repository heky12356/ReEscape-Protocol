ALTER TABLE events ADD COLUMN user_id INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_events_user_cursor ON events(user_id, cursor);
