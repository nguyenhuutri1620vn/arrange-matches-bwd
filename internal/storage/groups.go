package storage

import (
	"context"
	"database/sql"
	"errors"

	"arrange-matches-bwd/internal/domain"
)

const sessionGroupColumns = `id, session_id, no, phase, phase_started_at, created_at`

func scanSessionGroup(row interface{ Scan(...any) error }) (domain.SessionGroup, error) {
	var (
		g                domain.SessionGroup
		phaseAt, created int64
	)
	err := row.Scan(&g.ID, &g.SessionID, &g.No, &g.Phase, &phaseAt, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return g, domain.ErrNotFound
	}
	g.PhaseStartedAt, g.CreatedAt = fromMs(phaseAt), fromMs(created)
	return g, err
}

// InsertSessionGroup adds a group numbered with the smallest number not in
// use in the session, and returns it.
func (q *Queries) InsertSessionGroup(ctx context.Context, g domain.SessionGroup) (domain.SessionGroup, error) {
	existing, err := q.ListSessionGroups(ctx, g.SessionID)
	if err != nil {
		return g, err
	}
	used := map[int]bool{}
	for _, e := range existing {
		used[e.No] = true
	}
	for g.No = 1; used[g.No]; g.No++ {
	}
	res, err := q.db.ExecContext(ctx,
		`INSERT INTO session_groups (session_id, no, phase, phase_started_at, created_at) VALUES (?, ?, ?, ?, ?)`,
		g.SessionID, g.No, g.Phase, toMs(g.PhaseStartedAt), toMs(g.CreatedAt))
	if err != nil {
		return g, err
	}
	g.ID, err = res.LastInsertId()
	return g, err
}

func (q *Queries) GetSessionGroup(ctx context.Context, id int64) (domain.SessionGroup, error) {
	return scanSessionGroup(q.db.QueryRowContext(ctx, `SELECT `+sessionGroupColumns+` FROM session_groups WHERE id = ?`, id))
}

func (q *Queries) ListSessionGroups(ctx context.Context, sessionID int64) ([]domain.SessionGroup, error) {
	rows, err := q.db.QueryContext(ctx, `SELECT `+sessionGroupColumns+` FROM session_groups WHERE session_id = ? ORDER BY no`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.SessionGroup
	for rows.Next() {
		g, err := scanSessionGroup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// UpdateSessionGroupPhase saves the group's rhythm.
func (q *Queries) UpdateSessionGroupPhase(ctx context.Context, g domain.SessionGroup) error {
	_, err := q.db.ExecContext(ctx, `UPDATE session_groups SET phase = ?, phase_started_at = ? WHERE id = ?`,
		g.Phase, toMs(g.PhaseStartedAt), g.ID)
	return err
}

// DeleteEmptySessionGroups removes the session's groups nobody is in.
func (q *Queries) DeleteEmptySessionGroups(ctx context.Context, sessionID int64) error {
	_, err := q.db.ExecContext(ctx,
		`DELETE FROM session_groups WHERE session_id = ?
		   AND NOT EXISTS (SELECT 1 FROM session_players sp WHERE sp.group_id = session_groups.id)`, sessionID)
	return err
}
