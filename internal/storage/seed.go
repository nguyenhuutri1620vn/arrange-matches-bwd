package storage

import (
	"context"
	"time"

	"arrange-matches-bwd/internal/domain"
)

const (
	initialSeed       = "initial_2026_11"
	courtSetupSeed    = "initial_2026_11_court_setup"
	setupExpensesSeed = "initial_2026_11_setup_expenses"
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
	return s.InTx(ctx, func(q *Queries) error { return seedSetupExpenses(ctx, q) })
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
