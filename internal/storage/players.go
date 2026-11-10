package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"arrange-matches-bwd/internal/domain"
)

var settingKeys = struct {
	feeMale, feeFemale, courtFee, genderGap, balance, repeatLookback string
}{"fee_male", "fee_female", "court_fee", "gender_gap_threshold", "balance_threshold", "repeat_lookback"}

// GetSettings returns the stored settings; missing keys keep their defaults.
func (q *Queries) GetSettings(ctx context.Context) (domain.Settings, error) {
	s := domain.DefaultSettings()
	rows, err := q.db.QueryContext(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return s, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			key   string
			value int64
		)
		if err := rows.Scan(&key, &value); err != nil {
			return s, err
		}
		switch key {
		case settingKeys.feeMale:
			s.FeeMale = value
		case settingKeys.feeFemale:
			s.FeeFemale = value
		case settingKeys.courtFee:
			s.CourtFee = value
		case settingKeys.genderGap:
			s.Rules.GenderGapThreshold = int(value)
		case settingKeys.balance:
			s.Rules.BalanceThreshold = int(value)
		case settingKeys.repeatLookback:
			s.Rules.RepeatLookback = int(value)
		}
	}
	return s, rows.Err()
}

func (q *Queries) SaveSettings(ctx context.Context, s domain.Settings) error {
	values := map[string]int64{
		settingKeys.feeMale:        s.FeeMale,
		settingKeys.feeFemale:      s.FeeFemale,
		settingKeys.courtFee:       s.CourtFee,
		settingKeys.genderGap:      int64(s.Rules.GenderGapThreshold),
		settingKeys.balance:        int64(s.Rules.BalanceThreshold),
		settingKeys.repeatLookback: int64(s.Rules.RepeatLookback),
	}
	for key, value := range values {
		if _, err := q.db.ExecContext(ctx,
			`INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value`,
			key, value); err != nil {
			return err
		}
	}
	return nil
}

const playerColumns = `id, name, gender, level, is_regular, monthly_fee, note, deleted_at`

func scanPlayer(row interface{ Scan(...any) error }) (domain.Player, error) {
	var (
		p       domain.Player
		deleted sql.NullInt64
	)
	err := row.Scan(&p.ID, &p.Name, &p.Gender, &p.Level, &p.IsRegular, &p.MonthlyFee, &p.Note, &deleted)
	p.DeletedAt = timePtr(deleted)
	return p, err
}

// ListPlayers returns the address book without deleted players.
func (q *Queries) ListPlayers(ctx context.Context) ([]domain.Player, error) {
	rows, err := q.db.QueryContext(ctx, `SELECT `+playerColumns+` FROM players WHERE deleted_at IS NULL ORDER BY is_regular DESC, name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Player
	for rows.Next() {
		p, err := scanPlayer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetPlayer returns a player, including soft-deleted ones.
func (q *Queries) GetPlayer(ctx context.Context, id int64) (domain.Player, error) {
	p, err := scanPlayer(q.db.QueryRowContext(ctx, `SELECT `+playerColumns+` FROM players WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return p, domain.ErrNotFound
	}
	return p, err
}

func (q *Queries) InsertPlayer(ctx context.Context, p domain.Player) (int64, error) {
	res, err := q.db.ExecContext(ctx,
		`INSERT INTO players (name, gender, level, is_regular, monthly_fee, note) VALUES (?, ?, ?, ?, ?, ?)`,
		p.Name, p.Gender, p.Level, boolInt(p.IsRegular), p.MonthlyFee, p.Note)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (q *Queries) UpdatePlayer(ctx context.Context, p domain.Player) error {
	_, err := q.db.ExecContext(ctx,
		`UPDATE players SET name = ?, gender = ?, level = ?, is_regular = ?, monthly_fee = ?, note = ? WHERE id = ?`,
		p.Name, p.Gender, p.Level, boolInt(p.IsRegular), p.MonthlyFee, p.Note, p.ID)
	return err
}

func (q *Queries) SoftDeletePlayer(ctx context.Context, id int64, now time.Time) error {
	_, err := q.db.ExecContext(ctx, `UPDATE players SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL`, toMs(now), id)
	return err
}

// FindPlayerByName returns the active player with this exact name.
func (q *Queries) FindPlayerByName(ctx context.Context, name string) (domain.Player, error) {
	p, err := scanPlayer(q.db.QueryRowContext(ctx, `SELECT `+playerColumns+` FROM players WHERE name = ? AND deleted_at IS NULL ORDER BY id LIMIT 1`, name))
	if errors.Is(err, sql.ErrNoRows) {
		return p, domain.ErrNotFound
	}
	return p, err
}
