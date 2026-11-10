package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"arrange-matches-bwd/internal/domain"
)

const transactionColumns = `id, session_id, player_id, type, category, amount, note, date, auto, deleted_at`

func scanTransaction(row interface{ Scan(...any) error }) (domain.Transaction, error) {
	var (
		t                            domain.Transaction
		sessionID, playerID, deleted sql.NullInt64
	)
	err := row.Scan(&t.ID, &sessionID, &playerID, &t.Type, &t.Category, &t.Amount, &t.Note, &t.Date, &t.Auto, &deleted)
	t.SessionID, t.PlayerID, t.DeletedAt = intPtr(sessionID), intPtr(playerID), timePtr(deleted)
	if errors.Is(err, sql.ErrNoRows) {
		return t, domain.ErrNotFound
	}
	return t, err
}

func (q *Queries) InsertTransaction(ctx context.Context, t domain.Transaction) (int64, error) {
	res, err := q.db.ExecContext(ctx,
		`INSERT INTO transactions (session_id, player_id, type, category, amount, note, date, auto) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		nullInt(t.SessionID), nullInt(t.PlayerID), t.Type, t.Category, t.Amount, t.Note, t.Date, boolInt(t.Auto))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetTransaction returns a transaction that has not been soft-deleted.
func (q *Queries) GetTransaction(ctx context.Context, id int64) (domain.Transaction, error) {
	return scanTransaction(q.db.QueryRowContext(ctx, `SELECT `+transactionColumns+` FROM transactions WHERE id = ? AND deleted_at IS NULL`, id))
}

func (q *Queries) UpdateTransaction(ctx context.Context, t domain.Transaction) error {
	_, err := q.db.ExecContext(ctx,
		`UPDATE transactions SET session_id = ?, player_id = ?, type = ?, category = ?, amount = ?, note = ?, date = ? WHERE id = ? AND deleted_at IS NULL`,
		nullInt(t.SessionID), nullInt(t.PlayerID), t.Type, t.Category, t.Amount, t.Note, t.Date, t.ID)
	return err
}

func (q *Queries) SoftDeleteTransaction(ctx context.Context, id int64, now time.Time) error {
	_, err := q.db.ExecContext(ctx, `UPDATE transactions SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL`, toMs(now), id)
	return err
}

// GetAutoSessionFee returns the session's auto-synced fee income, or
// domain.ErrNotFound when it does not exist (or the host deleted it).
func (q *Queries) GetAutoSessionFee(ctx context.Context, sessionID int64) (domain.Transaction, error) {
	return scanTransaction(q.db.QueryRowContext(ctx,
		`SELECT `+transactionColumns+` FROM transactions WHERE session_id = ? AND category = ? AND auto = 1 AND deleted_at IS NULL`,
		sessionID, domain.CatSessionFee))
}

// TransactionFilter narrows ListTransactions; zero fields are ignored.
type TransactionFilter struct {
	Month     string // YYYY-MM by transaction date
	SessionID int64
}

// ListTransactions returns transactions that are not soft-deleted.
func (q *Queries) ListTransactions(ctx context.Context, f TransactionFilter) ([]domain.Transaction, error) {
	query := `SELECT ` + transactionColumns + ` FROM transactions WHERE deleted_at IS NULL`
	var args []any
	if f.Month != "" {
		query += ` AND substr(date, 1, 7) = ?`
		args = append(args, f.Month)
	}
	if f.SessionID != 0 {
		query += ` AND session_id = ?`
		args = append(args, f.SessionID)
	}
	rows, err := q.db.QueryContext(ctx, query+` ORDER BY date DESC, id DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Transaction
	for rows.Next() {
		t, err := scanTransaction(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// MonthTotals are the sums of one month's transactions.
type MonthTotals struct {
	Month   string
	Income  int64
	Expense int64
}

// ListMonthTotals sums non-deleted transactions per month (by transaction
// date), newest month first.
func (q *Queries) ListMonthTotals(ctx context.Context) ([]MonthTotals, error) {
	rows, err := q.db.QueryContext(ctx,
		`SELECT substr(date, 1, 7) AS month,
		        COALESCE(SUM(CASE WHEN type = 'income' THEN amount END), 0),
		        COALESCE(SUM(CASE WHEN type = 'expense' THEN amount END), 0)
		 FROM transactions WHERE deleted_at IS NULL GROUP BY month ORDER BY month DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MonthTotals
	for rows.Next() {
		var m MonthTotals
		if err := rows.Scan(&m.Month, &m.Income, &m.Expense); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// CountSessionsByMonth counts sessions per month of their date.
func (q *Queries) CountSessionsByMonth(ctx context.Context) (map[string]int, error) {
	rows, err := q.db.QueryContext(ctx, `SELECT substr(date, 1, 7), COUNT(*) FROM sessions GROUP BY 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var (
			month string
			n     int
		)
		if err := rows.Scan(&month, &n); err != nil {
			return nil, err
		}
		out[month] = n
	}
	return out, rows.Err()
}

// DataLoadApplied reports whether a one-off data load already ran.
func (q *Queries) DataLoadApplied(ctx context.Context, name string) (bool, error) {
	var n int
	err := q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM data_loads WHERE name = ?`, name).Scan(&n)
	return n > 0, err
}

func (q *Queries) MarkDataLoadApplied(ctx context.Context, name string, now time.Time) error {
	_, err := q.db.ExecContext(ctx, `INSERT INTO data_loads (name, applied_at) VALUES (?, ?)`, name, toMs(now))
	return err
}
