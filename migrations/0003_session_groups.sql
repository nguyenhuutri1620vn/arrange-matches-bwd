-- Groups of players who came together to a session, with their rhythm:
-- a match together, then matches apart, then together again.
CREATE TABLE session_groups (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id       INTEGER NOT NULL REFERENCES sessions (id),
    no               INTEGER NOT NULL,
    phase            TEXT    NOT NULL DEFAULT 'TOGETHER' CHECK (phase IN ('TOGETHER', 'SPLIT')),
    phase_started_at INTEGER NOT NULL,
    created_at       INTEGER NOT NULL,
    UNIQUE (session_id, no)
);

INSERT INTO session_groups (session_id, no, phase, phase_started_at, created_at)
SELECT session_id, group_no, 'TOGETHER', MIN(checked_in_at), MIN(checked_in_at)
FROM session_players
WHERE group_no IS NOT NULL
GROUP BY session_id, group_no;

ALTER TABLE session_players ADD COLUMN group_id INTEGER REFERENCES session_groups (id);

UPDATE session_players
SET group_id = (
    SELECT g.id FROM session_groups g
    WHERE g.session_id = session_players.session_id AND g.no = session_players.group_no
)
WHERE group_no IS NOT NULL;

ALTER TABLE session_players DROP COLUMN group_no;
