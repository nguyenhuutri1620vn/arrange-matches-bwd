-- Timestamps are UTC, stored as Unix milliseconds.
-- Dates (sessions.date, transactions.date) are Vietnam-local YYYY-MM-DD.
-- Money is integer VND.

CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value INTEGER NOT NULL
);

CREATE TABLE players (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT    NOT NULL,
    gender      TEXT    NOT NULL CHECK (gender IN ('M', 'F')),
    level       TEXT    NOT NULL CHECK (level IN ('Y', 'Y+', 'TBY', 'TB-', 'TB', 'TB+')),
    is_regular  INTEGER NOT NULL DEFAULT 0,
    monthly_fee INTEGER NOT NULL DEFAULT 0,
    note        TEXT    NOT NULL DEFAULT '',
    deleted_at  INTEGER
);

CREATE TABLE sessions (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    date        TEXT    NOT NULL,
    start_time  INTEGER,
    end_time    INTEGER,
    status      TEXT    NOT NULL CHECK (status IN ('OPEN', 'ENDED')),
    court_count INTEGER NOT NULL CHECK (court_count BETWEEN 1 AND 3),
    fee_male    INTEGER NOT NULL,
    fee_female  INTEGER NOT NULL,
    court_fee   INTEGER NOT NULL
);
CREATE UNIQUE INDEX sessions_one_open ON sessions (status) WHERE status = 'OPEN';
CREATE INDEX sessions_date ON sessions (date);

CREATE TABLE session_players (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id       INTEGER NOT NULL REFERENCES sessions (id),
    player_id        INTEGER NOT NULL REFERENCES players (id),
    status           TEXT    NOT NULL CHECK (status IN ('WAITING', 'PLAYING', 'RESTING')),
    matches_played   INTEGER NOT NULL DEFAULT 0,
    priority_offset  INTEGER NOT NULL DEFAULT 0,
    consecutive_rest INTEGER NOT NULL DEFAULT 0,
    queued_at        INTEGER NOT NULL,
    checked_in_at    INTEGER NOT NULL,
    fee_override     INTEGER,
    is_paid          INTEGER NOT NULL DEFAULT 0,
    UNIQUE (session_id, player_id)
);

-- A court row with idx above sessions.court_count is closed.
CREATE TABLE courts (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id       INTEGER NOT NULL REFERENCES sessions (id),
    idx              INTEGER NOT NULL CHECK (idx BETWEEN 1 AND 3),
    status           TEXT    NOT NULL CHECK (status IN ('EMPTY', 'IN_USE')),
    current_match_id INTEGER REFERENCES matches (id),
    UNIQUE (session_id, idx)
);

CREATE TABLE matches (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id  INTEGER NOT NULL REFERENCES sessions (id),
    court_id    INTEGER NOT NULL REFERENCES courts (id),
    status      TEXT    NOT NULL CHECK (status IN ('DRAFT', 'ONGOING', 'FINISHED')),
    started_at  INTEGER,
    finished_at INTEGER
);
CREATE INDEX matches_session ON matches (session_id, status);
-- A court holds at most one proposal and at most one running match.
CREATE UNIQUE INDEX matches_one_active_per_court ON matches (court_id, status) WHERE status IN ('DRAFT', 'ONGOING');

CREATE TABLE match_players (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    match_id          INTEGER NOT NULL REFERENCES matches (id) ON DELETE CASCADE,
    session_player_id INTEGER NOT NULL REFERENCES session_players (id),
    team              TEXT    NOT NULL CHECK (team IN ('A', 'B')),
    UNIQUE (match_id, session_player_id)
);
CREATE INDEX match_players_player ON match_players (session_player_id);

CREATE TABLE transactions (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id INTEGER REFERENCES sessions (id),
    player_id  INTEGER REFERENCES players (id),
    type       TEXT    NOT NULL CHECK (type IN ('income', 'expense')),
    category   TEXT    NOT NULL,
    amount     INTEGER NOT NULL CHECK (amount >= 0),
    note       TEXT    NOT NULL DEFAULT '',
    date       TEXT    NOT NULL,
    -- 1 for the session-fee income kept in sync with the players' paid flags
    auto       INTEGER NOT NULL DEFAULT 0,
    deleted_at INTEGER
);
CREATE INDEX transactions_date ON transactions (date);
CREATE INDEX transactions_session ON transactions (session_id);

-- Marks one-off data loads (seed) so they never run twice.
CREATE TABLE data_loads (
    name       TEXT PRIMARY KEY,
    applied_at INTEGER NOT NULL
);
