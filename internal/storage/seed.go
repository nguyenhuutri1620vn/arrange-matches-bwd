package storage

import (
	"context"
	"time"

	"arrange-matches-bwd/internal/domain"
)

const initialSeed = "initial_2026_11"

// Seed loads the host's historical data (docs/raw_requirement.md section
// 12). It runs once per database; calling it again changes nothing.
func (s *Store) Seed(ctx context.Context) error {
	return s.InTx(ctx, func(q *Queries) error {
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
	})
}
