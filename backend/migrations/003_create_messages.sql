CREATE TABLE IF NOT EXISTS messages (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id    UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id),
    username   VARCHAR(64) NOT NULL,
    body       TEXT NOT NULL CHECK (char_length(body) <= 4096),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_messages_room_created ON messages(room_id, created_at DESC);
