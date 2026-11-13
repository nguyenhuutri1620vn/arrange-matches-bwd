package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"arrange-matches-bwd/internal/domain"
)

const (
	initialSeed         = "initial_2026_11"
	courtSetupSeed      = "initial_2026_11_court_setup"
	setupExpensesSeed   = "initial_2026_11_setup_expenses"
	recountSeed         = "recount_2026_11_13"
	session20261111Seed = "recount_2026_11_13_session_2026_11_11"
)

// Seed loads the host's historical data (docs/raw_requirement.md section
// 12). Each load runs once per database; calling it again changes nothing.
func (s *Store) Seed(ctx context.Context) error {
	if err := s.InTx(ctx, func(q *Queries) error { return seedInitial(ctx, q) }); err != nil {
		return err
	}
	if err := s.InTx(ctx, func(q *Queries) error { return seedCourtSetup(ctx, q) }); err != nil {
		return err
	}
	if err := s.InTx(ctx, func(q *Queries) error { return seedSetupExpenses(ctx, q) }); err != nil {
		return err
	}
	if err := s.InTx(ctx, func(q *Queries) error { return seedRecount(ctx, q) }); err != nil {
		return err
	}
	return s.InTx(ctx, func(q *Queries) error { return seedSession20261111(ctx, q) })
}

// seedRecount applies the host's recount of 2026-11-13: the 2.030.000
// capital split 2/3 and 1/3 (rounded to thousands), the duplicate setup
// expenses removed, and one more shuttle tube.
func seedRecount(ctx context.Context, q *Queries) error {
	if done, err := q.DataLoadApplied(ctx, recountSeed); err != nil || done {
		return err
	}
	now := time.Now()

	for _, c := range []struct {
		name string
		fee  int64
	}{
		{"Trí", 1353000},
		{"Trang", 677000},
	} {
		if _, err := q.db.ExecContext(ctx,
			`UPDATE players SET monthly_fee = ? WHERE name = ? AND is_regular = 1`, c.fee, c.name); err != nil {
			return err
		}
		if _, err := q.db.ExecContext(ctx,
			`UPDATE transactions SET amount = ?
			 WHERE category = 'monthly_fee' AND date = '2026-11-01' AND deleted_at IS NULL
			   AND player_id = (SELECT id FROM players WHERE name = ? AND is_regular = 1)`, c.fee, c.name); err != nil {
			return err
		}
	}

	// The regulars' money was spent on the court and shuttles, which are
	// already recorded; these two only counted it twice.
	if _, err := q.db.ExecContext(ctx,
		`UPDATE transactions SET deleted_at = ?
		 WHERE category = 'other_expense' AND date = '2026-11-01' AND deleted_at IS NULL
		   AND note IN ('Chi phí ban đầu của sân (Trí)', 'Chi phí ban đầu của sân (Trang)')`, toMs(now)); err != nil {
		return err
	}

	if _, err := q.InsertTransaction(ctx, domain.Transaction{
		Type:     domain.TypeExpense,
		Category: domain.CatShuttles,
		Amount:   315000,
		Note:     "Mua thêm 1 ống cầu",
		Date:     "2026-11-12",
	}); err != nil {
		return err
	}

	return q.MarkDataLoadApplied(ctx, recountSeed, now)
}

// seedSession20261111 records the real 2026-11-11 session: five casual
// players, none paid, no court rent. A session already held that day was
// a trial run, so it is rewritten in place; its matches and players go,
// and its money rows are kept but corrected (rent soft-deleted).
func seedSession20261111(ctx context.Context, q *Queries) error {
	if done, err := q.DataLoadApplied(ctx, session20261111Seed); err != nil || done {
		return err
	}
	const date = "2026-11-11"
	now := time.Now()
	var sessionID int64
	err := q.db.QueryRowContext(ctx, `SELECT id FROM sessions WHERE date = ? ORDER BY id LIMIT 1`, date).Scan(&sessionID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		settings, err := q.GetSettings(ctx)
		if err != nil {
			return err
		}
		if sessionID, err = q.InsertSession(ctx, domain.Session{
			Date:       date,
			Status:     domain.SessionEnded,
			CourtCount: 2,
			FeeMale:    settings.FeeMale,
			FeeFemale:  settings.FeeFemale,
		}); err != nil {
			return err
		}
	case err != nil:
		return err
	default:
		for _, stmt := range []string{
			`UPDATE courts SET current_match_id = NULL WHERE session_id = ?`,
			`DELETE FROM match_players WHERE match_id IN (SELECT id FROM matches WHERE session_id = ?)`,
			`DELETE FROM matches WHERE session_id = ?`,
			`DELETE FROM courts WHERE session_id = ?`,
			`DELETE FROM session_players WHERE session_id = ?`,
		} {
			if _, err := q.db.ExecContext(ctx, stmt, sessionID); err != nil {
				return err
			}
		}
		if _, err := q.db.ExecContext(ctx,
			`UPDATE sessions SET status = ?, court_fee = 0 WHERE id = ?`, domain.SessionEnded, sessionID); err != nil {
			return err
		}
		if _, err := q.db.ExecContext(ctx,
			`UPDATE transactions SET deleted_at = ? WHERE session_id = ? AND category = 'court_rent' AND deleted_at IS NULL`,
			toMs(now), sessionID); err != nil {
			return err
		}
	}

	casuals := []domain.Player{
		{Name: "VL nam 1", Gender: domain.GenderMale},
		{Name: "VL nam 2", Gender: domain.GenderMale},
		{Name: "VL nam 3", Gender: domain.GenderMale},
		{Name: "VL nam 4", Gender: domain.GenderMale},
		{Name: "VL nữ 1", Gender: domain.GenderFemale},
	}
	for _, p := range casuals {
		p.Level = domain.LevelTBY
		p.Note = "Tên tạm, nhập lại khi thống kê 13/11"
		playerID, err := q.InsertPlayer(ctx, p)
		if err != nil {
			return err
		}
		if _, err := q.InsertSessionPlayer(ctx, domain.SessionPlayer{
			SessionID:   sessionID,
			PlayerID:    playerID,
			Status:      domain.PlayerWaiting,
			QueuedAt:    now,
			CheckedInAt: now,
		}); err != nil {
			return err
		}
	}

	// The auto "Phí buổi" follows the paid flags; none are paid yet.
	fee, err := q.GetAutoSessionFee(ctx, sessionID)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		_, err = q.InsertTransaction(ctx, domain.Transaction{
			SessionID: &sessionID,
			Type:      domain.TypeIncome,
			Category:  domain.CatSessionFee,
			Date:      date,
			Auto:      true,
		})
	case err == nil:
		fee.Amount = 0
		err = q.UpdateTransaction(ctx, fee)
	}
	if err != nil {
		return err
	}
	return q.MarkDataLoadApplied(ctx, session20261111Seed, now)
}

// seedSetupExpenses records the court and shuttle purchases paid at setup.
func seedSetupExpenses(ctx context.Context, q *Queries) error {
	if done, err := q.DataLoadApplied(ctx, setupExpensesSeed); err != nil || done {
		return err
	}
	for _, e := range []struct {
		category domain.Category
		note     string
		amount   int64
	}{
		{domain.CatCourtRent, "Chi phí ban đầu của sân (tiền sân)", 1400000},
		{domain.CatShuttles, "Chi phí ban đầu của sân (tiền cầu)", 630000},
	} {
		if _, err := q.InsertTransaction(ctx, domain.Transaction{
			Type:     domain.TypeExpense,
			Category: e.category,
			Amount:   e.amount,
			Note:     e.note,
			Date:     "2026-11-01",
		}); err != nil {
			return err
		}
	}
	return q.MarkDataLoadApplied(ctx, setupExpensesSeed, time.Now())
}

func seedInitial(ctx context.Context, q *Queries) error {
	if done, err := q.DataLoadApplied(ctx, initialSeed); err != nil || done {
		return err
	}
	settings, err := q.GetSettings(ctx)
	if err != nil {
		return err
	}

	regulars := []domain.Player{
		{Name: "Trí", Gender: domain.GenderMale, Level: domain.LevelTBMinus, IsRegular: true, MonthlyFee: 1333000},
		{Name: "Trang", Gender: domain.GenderFemale, Level: domain.LevelTB, IsRegular: true, MonthlyFee: 666000},
		{Name: "Huyền", Gender: domain.GenderFemale, Level: domain.LevelY, IsRegular: true, MonthlyFee: 0},
	}
	for _, p := range regulars {
		id, err := q.InsertPlayer(ctx, p)
		if err != nil {
			return err
		}
		if _, err := q.InsertTransaction(ctx, domain.Transaction{
			PlayerID: &id,
			Type:     domain.TypeIncome,
			Category: domain.CatMonthlyFee,
			Amount:   p.MonthlyFee,
			Note:     "Phí tháng 11/2026",
			Date:     "2026-11-01",
		}); err != nil {
			return err
		}
	}

	// Past sessions carry only totals: no players, no matches, and no
	// court rent derived from court_fee.
	pastSessions := []struct {
		date      string
		courts    int
		income    int64
		courtRent int64
	}{
		{"2026-11-04", 1, 315000, 0},
		{"2026-11-09", 2, 790000, 220000},
	}
	for _, ps := range pastSessions {
		id, err := q.InsertSession(ctx, domain.Session{
			Date:       ps.date,
			Status:     domain.SessionEnded,
			CourtCount: ps.courts,
			FeeMale:    settings.FeeMale,
			FeeFemale:  settings.FeeFemale,
		})
		if err != nil {
			return err
		}
		if _, err := q.InsertTransaction(ctx, domain.Transaction{
			SessionID: &id, Type: domain.TypeIncome, Category: domain.CatSessionFee, Amount: ps.income, Date: ps.date,
		}); err != nil {
			return err
		}
		if ps.courtRent > 0 {
			if _, err := q.InsertTransaction(ctx, domain.Transaction{
				SessionID: &id, Type: domain.TypeExpense, Category: domain.CatCourtRent, Amount: ps.courtRent, Date: ps.date,
			}); err != nil {
				return err
			}
		}
	}
	return q.MarkDataLoadApplied(ctx, initialSeed, time.Now())
}

// seedCourtSetup records the two regulars' payments as spent on the court's
// initial setup, and corrects Trang's monthly fee to 667.000. It runs after
// seedInitial, so a database that already holds the first load gets the fix
// too, and a fresh one ends in the same state.
func seedCourtSetup(ctx context.Context, q *Queries) error {
	if done, err := q.DataLoadApplied(ctx, courtSetupSeed); err != nil || done {
		return err
	}
	const trangFee int64 = 667000
	if _, err := q.db.ExecContext(ctx,
		`UPDATE players SET monthly_fee = ? WHERE name = 'Trang' AND is_regular = 1`, trangFee); err != nil {
		return err
	}
	if _, err := q.db.ExecContext(ctx,
		`UPDATE transactions SET amount = ?
		 WHERE category = 'monthly_fee' AND date = '2026-11-01' AND deleted_at IS NULL
		   AND player_id = (SELECT id FROM players WHERE name = 'Trang' AND is_regular = 1)`, trangFee); err != nil {
		return err
	}
	for _, e := range []struct {
		name   string
		amount int64
	}{
		{"Trí", 1333000},
		{"Trang", trangFee},
	} {
		if _, err := q.InsertTransaction(ctx, domain.Transaction{
			Type:     domain.TypeExpense,
			Category: domain.CatOtherExpense,
			Amount:   e.amount,
			Note:     "Chi phí ban đầu của sân (" + e.name + ")",
			Date:     "2026-11-01",
		}); err != nil {
			return err
		}
	}
	return q.MarkDataLoadApplied(ctx, courtSetupSeed, time.Now())
}
