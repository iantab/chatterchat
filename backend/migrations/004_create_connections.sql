CREATE TABLE IF NOT EXISTS connections (
    connection_id VARCHAR(128) PRIMARY KEY,
    user_id       UUID NOT NULL REFERENCES users(id),
    username      VARCHAR(64) NOT NULL,
    room_id       UUID REFERENCES rooms(id),
    connected_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_connections_room_id ON connections(room_id);
