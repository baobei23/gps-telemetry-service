CREATE TABLE IF NOT EXISTS devices (
    id VARCHAR(32) PRIMARY KEY,
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_devices_last_seen
    ON devices (last_seen_at DESC);

CREATE TABLE IF NOT EXISTS positions (
    id BIGSERIAL PRIMARY KEY,
    device_id VARCHAR(32) NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    device_time TIMESTAMPTZ NOT NULL,
    server_time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    lat DOUBLE PRECISION NOT NULL,
    lon DOUBLE PRECISION NOT NULL,
    valid SMALLINT NOT NULL DEFAULT 1,
    speed DOUBLE PRECISION,
    bearing DOUBLE PRECISION,
    altitude DOUBLE PRECISION,
    accuracy DOUBLE PRECISION,
    hdop DOUBLE PRECISION,
    batt DOUBLE PRECISION,
    charge BOOLEAN,
    CONSTRAINT uq_positions_device_time UNIQUE (device_id, device_time)
);
