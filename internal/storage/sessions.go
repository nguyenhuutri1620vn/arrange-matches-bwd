package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"arrange-matches-bwd/internal/domain"
)

const sessionColumns = `id, date, start_time, end_time, status, court_count, fee_male, fee_female, court_fee`

func scanSession(row interface{ Scan(...any) error }) (domain.Session, error) {
	var (
		s          domain.Session
		start, end sql.NullInt64
	)
	err := row.Scan(&s.ID, &s.Date, &start, &end, &s.Status, &s.CourtCount, &s.FeeMale, &s.FeeFemale, &s.CourtFee)
	s.StartTime, s.EndTime = timePtr(start), timePtr(end)
	if errors.Is(err, sql.ErrNoRows) {
		return s, domain.ErrNotFound
	}
	return s, err
}

func (q *Queries) InsertSession(ctx context.Context, s domain.Session) (int64, error) {
	res, err := q.db.ExecContext(ctx,
		`INSERT INTO sessions (date, start_time, end_time, status, court_count, fee_male, fee_female, court_fee) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		s.Date, nullMs(s.StartTime), nullMs(s.EndTime), s.Status, s.CourtCount, s.FeeMale, s.FeeFemale, s.CourtFee)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (q *Queries) GetSession(ctx context.Context, id int64) (domain.Session, error) {
	return scanSession(q.db.QueryRowContext(ctx, `SELECT `+sessionColumns+` FROM sessions WHERE id = ?`, id))
}

// GetOpenSession returns the single OPEN session or domain.ErrNotFound.
func (q *Queries) GetOpenSession(ctx context.Context) (domain.Session, error) {
	return scanSession(q.db.QueryRowContext(ctx, `SELECT `+sessionColumns+` FROM sessions WHERE status = ?`, domain.SessionOpen))
}

// ListSessionsInMonth returns sessions whose date falls in month (YYYY-MM).
func (q *Queries) ListSessionsInMonth(ctx context.Context, month string) ([]domain.Session, error) {
	rows, err := q.db.QueryContext(ctx, `SELECT `+sessionColumns+` FROM sessions WHERE substr(date, 1, 7) = ? ORDER BY date, id`, month)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Session
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (q *Queries) UpdateSessionCourtCount(ctx context.Context, id int64, count int) error {
	_, err := q.db.ExecContext(ctx, `UPDATE sessions SET court_count = ? WHERE id = ?`, count, id)
	return err
}

func (q *Queries) MarkSessionEnded(ctx context.Context, id int64, now time.Time) error {
	_, err := q.db.ExecContext(ctx, `UPDATE sessions SET status = ?, end_time = ? WHERE id = ?`, domain.SessionEnded, toMs(now), id)
	return err
}

func scanCourt(row interface{ Scan(...any) error }) (domain.Court, error) {
	var (
		c       domain.Court
		matchID sql.NullInt64
	)
	err := row.Scan(&c.ID, &c.SessionID, &c.Index, &c.Status, &matchID)
	c.CurrentMatchID = intPtr(matchID)
	if errors.Is(err, sql.ErrNoRows) {
		return c, domain.ErrNotFound
	}
	return c, err
}

// EnsureCourts creates the court rows 1..count that do not exist yet.
func (q *Queries) EnsureCourts(ctx context.Context, sessionID int64, count int) error {
	for idx := 1; idx <= count; idx++ {
		if _, err := q.db.ExecContext(ctx,
			`INSERT INTO courts (session_id, idx, status) VALUES (?, ?, ?) ON CONFLICT (session_id, idx) DO NOTHING`,
			sessionID, idx, domain.CourtEmpty); err != nil {
			return err
		}
	}
	return nil
}

// ListCourts returns every court row of the session, including closed ones
// (index above the session's court count), ordered by index.
func (q *Queries) ListCourts(ctx context.Context, sessionID int64) ([]domain.Court, error) {
	rows, err := q.db.QueryContext(ctx, `SELECT id, session_id, idx, status, current_match_id FROM courts WHERE session_id = ? ORDER BY idx`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Court
	for rows.Next() {
		c, err := scanCourt(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (q *Queries) GetCourt(ctx context.Context, id int64) (domain.Court, error) {
	return scanCourt(q.db.QueryRowContext(ctx, `SELECT id, session_id, idx, status, current_match_id FROM courts WHERE id = ?`, id))
}

func (q *Queries) UpdateCourt(ctx context.Context, c domain.Court) error {
	_, err := q.db.ExecContext(ctx, `UPDATE courts SET status = ?, current_match_id = ? WHERE id = ?`, c.Status, nullInt(c.CurrentMatchID), c.ID)
	return err
}

const sessionPlayerColumns = `sp.id, sp.session_id, sp.player_id, p.name, p.gender, p.level, p.is_regular,
	sp.status, sp.matches_played, sp.priority_offset, sp.consecutive_rest, sp.queued_at, sp.checked_in_at, sp.fee_override, sp.is_paid`

const sessionPlayerFrom = ` FROM session_players sp JOIN players p ON p.id = sp.player_id `

func scanSessionPlayer(row interface{ Scan(...any) error }) (domain.SessionPlayer, error) {
	var (
		p                 domain.SessionPlayer
		queued, checkedIn int64
		override          sql.NullInt64
	)
	err := row.Scan(&p.ID, &p.SessionID, &p.PlayerID, &p.Name, &p.Gender, &p.Level, &p.IsRegular,
		&p.Status, &p.MatchesPlayed, &p.PriorityOffset, &p.ConsecutiveRest, &queued, &checkedIn, &override, &p.IsPaid)
	p.QueuedAt, p.CheckedInAt = fromMs(queued), fromMs(checkedIn)
	p.FeeOverride = intPtr(override)
	if errors.Is(err, sql.ErrNoRows) {
		return p, domain.ErrNotFound
	}
	return p, err
}

func (q *Queries) ListSessionPlayers(ctx context.Context, sessionID int64) ([]domain.SessionPlayer, error) {
	rows, err := q.db.QueryContext(ctx, `SELECT `+sessionPlayerColumns+sessionPlayerFrom+`WHERE sp.session_id = ? ORDER BY sp.id`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.SessionPlayer
	for rows.Next() {
		p, err := scanSessionPlayer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (q *Queries) GetSessionPlayer(ctx context.Context, id int64) (domain.SessionPlayer, error) {
	return scanSessionPlayer(q.db.QueryRowContext(ctx, `SELECT `+sessionPlayerColumns+sessionPlayerFrom+`WHERE sp.id = ?`, id))
}

// SessionPlayerExists reports whether the player is already in the session.
func (q *Queries) SessionPlayerExists(ctx context.Context, sessionID, playerID int64) (bool, error) {
	var n int
	err := q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM session_players WHERE session_id = ? AND player_id = ?`, sessionID, playerID).Scan(&n)
	return n > 0, err
}

func (q *Queries) InsertSessionPlayer(ctx context.Context, p domain.SessionPlayer) (int64, error) {
	res, err := q.db.ExecContext(ctx,
		`INSERT INTO session_players (session_id, player_id, status, matches_played, priority_offset, consecutive_rest, queued_at, checked_in_at, fee_override, is_paid)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.SessionID, p.PlayerID, p.Status, p.MatchesPlayed, p.PriorityOffset, p.ConsecutiveRest,
		toMs(p.QueuedAt), toMs(p.CheckedInAt), nullInt(p.FeeOverride), boolInt(p.IsPaid))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateSessionPlayer saves the mutable in-session state of a player.
func (q *Queries) UpdateSessionPlayer(ctx context.Context, p domain.SessionPlayer) error {
	_, err := q.db.ExecContext(ctx,
		`UPDATE session_players SET status = ?, matches_played = ?, priority_offset = ?, consecutive_rest = ?, queued_at = ?, fee_override = ?, is_paid = ? WHERE id = ?`,
		p.Status, p.MatchesPlayed, p.PriorityOffset, p.ConsecutiveRest, toMs(p.QueuedAt), nullInt(p.FeeOverride), boolInt(p.IsPaid), p.ID)
	return err
}

func (q *Queries) DeleteSessionPlayer(ctx context.Context, id int64) error {
	_, err := q.db.ExecContext(ctx, `DELETE FROM session_players WHERE id = ?`, id)
	return err
}

// CountMatchesOfPlayer counts matches of any status the player appears in.
func (q *Queries) CountMatchesOfPlayer(ctx context.Context, sessionPlayerID int64) (int, error) {
	var n int
	err := q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM match_players WHERE session_player_id = ?`, sessionPlayerID).Scan(&n)
	return n, err
}

func (q *Queries) InsertMatch(ctx context.Context, m domain.Match) (int64, error) {
	res, err := q.db.ExecContext(ctx,
		`INSERT INTO matches (session_id, court_id, status, started_at, finished_at) VALUES (?, ?, ?, ?, ?)`,
		m.SessionID, m.CourtID, m.Status, nullMs(m.StartedAt), nullMs(m.FinishedAt))
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, q.ReplaceMatchPlayers(ctx, id, m.Players)
}

func (q *Queries) ReplaceMatchPlayers(ctx context.Context, matchID int64, players []domain.MatchPlayer) error {
	if _, err := q.db.ExecContext(ctx, `DELETE FROM match_players WHERE match_id = ?`, matchID); err != nil {
		return err
	}
	for _, p := range players {
		if _, err := q.db.ExecContext(ctx, `INSERT INTO match_players (match_id, session_player_id, team) VALUES (?, ?, ?)`, matchID, p.SessionPlayerID, p.Team); err != nil {
			return err
		}
	}
	return nil
}

func (q *Queries) UpdateMatch(ctx context.Context, m domain.Match) error {
	_, err := q.db.ExecContext(ctx, `UPDATE matches SET status = ?, started_at = ?, finished_at = ? WHERE id = ?`,
		m.Status, nullMs(m.StartedAt), nullMs(m.FinishedAt), m.ID)
	return err
}

// DeleteMatch removes a match and, by cascade, its players.
func (q *Queries) DeleteMatch(ctx context.Context, id int64) error {
	_, err := q.db.ExecContext(ctx, `DELETE FROM matches WHERE id = ?`, id)
	return err
}

func (q *Queries) GetMatch(ctx context.Context, id int64) (domain.Match, error) {
	matches, err := q.queryMatches(ctx, `WHERE m.id = ?`, id)
	if err != nil {
		return domain.Match{}, err
	}
	if len(matches) == 0 {
		return domain.Match{}, domain.ErrNotFound
	}
	return matches[0], nil
}

// ListMatches returns the session's matches with the given statuses, most
// recently started first (drafts, which have not started, come first).
func (q *Queries) ListMatches(ctx context.Context, sessionID int64, statuses ...domain.MatchStatus) ([]domain.Match, error) {
	args := []any{sessionID}
	for _, s := range statuses {
		args = append(args, s)
	}
	return q.queryMatches(ctx, `WHERE m.session_id = ? AND m.status IN (`+placeholders(len(statuses))+`)`, args...)
}

func (q *Queries) queryMatches(ctx context.Context, where string, args ...any) ([]domain.Match, error) {
	rows, err := q.db.QueryContext(ctx,
		`SELECT m.id, m.session_id, m.court_id, m.status, m.started_at, m.finished_at, mp.session_player_id, mp.team
		 FROM matches m LEFT JOIN match_players mp ON mp.match_id = m.id `+where+`
		 ORDER BY m.started_at IS NULL DESC, m.started_at DESC, m.id DESC, mp.team, mp.id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Match
	for rows.Next() {
		var (
			m                 domain.Match
			started, finished sql.NullInt64
			spID              sql.NullInt64
			team              sql.NullString
		)
		if err := rows.Scan(&m.ID, &m.SessionID, &m.CourtID, &m.Status, &started, &finished, &spID, &team); err != nil {
			return nil, err
		}
		if n := len(out); n == 0 || out[n-1].ID != m.ID {
			m.StartedAt, m.FinishedAt = timePtr(started), timePtr(finished)
			out = append(out, m)
		}
		if spID.Valid {
			last := &out[len(out)-1]
			last.Players = append(last.Players, domain.MatchPlayer{SessionPlayerID: spID.Int64, Team: domain.Team(team.String)})
		}
	}
	return out, rows.Err()
}
