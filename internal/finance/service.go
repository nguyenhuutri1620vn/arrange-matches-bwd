// Package finance records income and expenses and builds the drill-down
// revenue report: total → month → session → player.
package finance

import (
	"context"
	"sort"
	"strings"
	"time"

	"arrange-matches-bwd/internal/domain"
	"arrange-matches-bwd/internal/storage"
)

type Service struct {
	store *storage.Store
	now   func() time.Time
}

// New builds the service. now is injectable for tests; nil means time.Now.
func New(store *storage.Store, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{store: store, now: func() time.Time { return now().UTC() }}
}

// TransactionView is a transaction with the linked player's name.
type TransactionView struct {
	domain.Transaction
	PlayerName string
}

// Create records a transaction. A monthly fee must name a regular player.
func (s *Service) Create(ctx context.Context, t domain.Transaction) (TransactionView, error) {
	t.Note = strings.TrimSpace(t.Note)
	t.Auto = false
	if err := t.Validate(); err != nil {
		return TransactionView{}, err
	}
	var view TransactionView
	err := s.store.InTx(ctx, func(q *storage.Queries) error {
		if err := checkLinks(ctx, q, t); err != nil {
			return err
		}
		id, err := q.InsertTransaction(ctx, t)
		if err != nil {
			return err
		}
		t.ID = id
		view, err = viewOf(ctx, q, t)
		return err
	})
	return view, err
}

// Update edits a transaction. The auto-synced session fee is derived from
// the players' paid flags and cannot be edited directly.
func (s *Service) Update(ctx context.Context, t domain.Transaction) (TransactionView, error) {
	t.Note = strings.TrimSpace(t.Note)
	if err := t.Validate(); err != nil {
		return TransactionView{}, err
	}
	var view TransactionView
	err := s.store.InTx(ctx, func(q *storage.Queries) error {
		current, err := q.GetTransaction(ctx, t.ID)
		if err != nil {
			return err
		}
		if current.Auto {
			return domain.ErrTransactionIsAuto
		}
		if err := checkLinks(ctx, q, t); err != nil {
			return err
		}
		if err := q.UpdateTransaction(ctx, t); err != nil {
			return err
		}
		view, err = viewOf(ctx, q, t)
		return err
	})
	return view, err
}

// Delete soft-deletes a transaction; reports stop counting it.
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.store.InTx(ctx, func(q *storage.Queries) error {
		if _, err := q.GetTransaction(ctx, id); err != nil {
			return err
		}
		return q.SoftDeleteTransaction(ctx, id, s.now())
	})
}

// List returns the month's transactions (YYYY-MM), newest first.
func (s *Service) List(ctx context.Context, month string) ([]TransactionView, error) {
	if !validMonth(month) {
		return nil, domain.ErrInvalidDate
	}
	var views []TransactionView
	err := s.store.InTx(ctx, func(q *storage.Queries) error {
		txs, err := q.ListTransactions(ctx, storage.TransactionFilter{Month: month})
		if err != nil {
			return err
		}
		views, err = viewsOf(ctx, q, txs)
		return err
	})
	return views, err
}

func validMonth(month string) bool {
	_, err := time.Parse("2006-01", month)
	return err == nil
}

func checkLinks(ctx context.Context, q *storage.Queries, t domain.Transaction) error {
	if t.SessionID != nil {
		if _, err := q.GetSession(ctx, *t.SessionID); err != nil {
			return err
		}
	}
	if t.PlayerID != nil {
		p, err := q.GetPlayer(ctx, *t.PlayerID)
		if err != nil {
			return err
		}
		if t.Category == domain.CatMonthlyFee && !p.IsRegular {
			return domain.ErrNotRegularPlayer
		}
	} else if t.Category == domain.CatMonthlyFee {
		return domain.ErrNotRegularPlayer
	}
	return nil
}

func viewOf(ctx context.Context, q *storage.Queries, t domain.Transaction) (TransactionView, error) {
	views, err := viewsOf(ctx, q, []domain.Transaction{t})
	if err != nil {
		return TransactionView{}, err
	}
	return views[0], nil
}

func viewsOf(ctx context.Context, q *storage.Queries, txs []domain.Transaction) ([]TransactionView, error) {
	names := map[int64]string{}
	views := make([]TransactionView, len(txs))
	for i, t := range txs {
		views[i].Transaction = t
		if t.PlayerID == nil {
			continue
		}
		name, ok := names[*t.PlayerID]
		if !ok {
			// Deleted players still show by name in past records.
			p, err := q.GetPlayer(ctx, *t.PlayerID)
			if err != nil {
				return nil, err
			}
			name = p.Name
			names[*t.PlayerID] = name
		}
		views[i].PlayerName = name
	}
	return views, nil
}

// Totals are income and expense sums; profit is derived.
type Totals struct {
	Income  int64
	Expense int64
}

func (t Totals) Profit() int64 { return t.Income - t.Expense }

func (t *Totals) add(tx domain.Transaction) {
	if tx.Type == domain.TypeIncome {
		t.Income += tx.Amount
	} else {
		t.Expense += tx.Amount
	}
}

type MonthSummary struct {
	Month string
	Totals
	SessionCount int
}

// Summary is the top of the drill-down: all-time totals and each month.
type Summary struct {
	Totals
	Months []MonthSummary
}

// Summary sums every non-deleted transaction, grouped by the month of its
// date (Vietnam calendar). The all-time totals equal the sum of the months.
func (s *Service) Summary(ctx context.Context) (Summary, error) {
	var out Summary
	err := s.store.InTx(ctx, func(q *storage.Queries) error {
		months, err := q.ListMonthTotals(ctx)
		if err != nil {
			return err
		}
		sessions, err := q.CountSessionsByMonth(ctx)
		if err != nil {
			return err
		}
		seen := map[string]bool{}
		for _, m := range months {
			seen[m.Month] = true
			out.Months = append(out.Months, MonthSummary{
				Month:        m.Month,
				Totals:       Totals{Income: m.Income, Expense: m.Expense},
				SessionCount: sessions[m.Month],
			})
			out.Income += m.Income
			out.Expense += m.Expense
		}
		// Months that have sessions but no transactions yet.
		for month, n := range sessions {
			if !seen[month] {
				out.Months = append(out.Months, MonthSummary{Month: month, SessionCount: n})
			}
		}
		sort.Slice(out.Months, func(i, j int) bool { return out.Months[i].Month > out.Months[j].Month })
		return nil
	})
	return out, err
}

type CategoryTotal struct {
	Category domain.Category
	Type     domain.TransactionType
	Amount   int64
}

// RegularPayment is what one regular player paid as monthly fee.
type RegularPayment struct {
	PlayerID int64
	Name     string
	Amount   int64
}

// SessionRow is one session inside a month report. The money columns only
// count transactions dated in that month, so rows always add up to the
// month totals.
type SessionRow struct {
	Session                                    domain.Session
	PlayerCount, Male, Female, Casual, Regular int
	// Fees come from the players' fees and paid flags.
	Fees domain.SessionFees
	// FeeIncome is the recorded "Phí buổi" income.
	FeeIncome   int64
	OtherIncome int64
	Expense     int64
}

func (r SessionRow) Totals() Totals {
	return Totals{Income: r.FeeIncome + r.OtherIncome, Expense: r.Expense}
}

// MonthReport is the month level of the drill-down.
//
//	Income  = RegularIncome + CasualIncome + OtherIncome
//	Totals  = sum of Sessions + UnlinkedTotals
type MonthReport struct {
	Month string
	Totals
	SessionCount int
	AvgIncome    int64
	AvgProfit    int64
	Categories   []CategoryTotal

	RegularIncome   int64
	RegularPayments []RegularPayment
	CasualIncome    int64
	OtherIncome     int64

	Sessions       []SessionRow
	Unlinked       []TransactionView
	UnlinkedTotals Totals
}

// Month builds the report of one month (YYYY-MM).
func (s *Service) Month(ctx context.Context, month string) (MonthReport, error) {
	if !validMonth(month) {
		return MonthReport{}, domain.ErrInvalidDate
	}
	report := MonthReport{Month: month}
	err := s.store.InTx(ctx, func(q *storage.Queries) error {
		txs, err := q.ListTransactions(ctx, storage.TransactionFilter{Month: month})
		if err != nil {
			return err
		}
		views, err := viewsOf(ctx, q, txs)
		if err != nil {
			return err
		}
		sessions, err := q.ListSessionsInMonth(ctx, month)
		if err != nil {
			return err
		}
		report.SessionCount = len(sessions)

		rows := map[int64]*SessionRow{}
		var order []int64
		addRow := func(sess domain.Session) error {
			players, err := q.ListSessionPlayers(ctx, sess.ID)
			if err != nil {
				return err
			}
			row := &SessionRow{Session: sess, PlayerCount: len(players), Fees: domain.SumSessionFees(&sess, players)}
			for _, p := range players {
				if p.Gender == domain.GenderMale {
					row.Male++
				} else {
					row.Female++
				}
				if p.IsRegular {
					row.Regular++
				} else {
					row.Casual++
				}
			}
			rows[sess.ID] = row
			order = append(order, sess.ID)
			return nil
		}
		for _, sess := range sessions {
			if err := addRow(sess); err != nil {
				return err
			}
		}

		categories := map[domain.Category]int64{}
		regular := map[int64]*RegularPayment{}
		for _, v := range views {
			tx := v.Transaction
			report.Totals.add(tx)
			categories[tx.Category] += tx.Amount

			switch {
			case tx.Category == domain.CatMonthlyFee:
				report.RegularIncome += tx.Amount
				if tx.PlayerID != nil {
					rp := regular[*tx.PlayerID]
					if rp == nil {
						rp = &RegularPayment{PlayerID: *tx.PlayerID, Name: v.PlayerName}
						regular[*tx.PlayerID] = rp
					}
					rp.Amount += tx.Amount
				}
			case tx.Category == domain.CatSessionFee:
				report.CasualIncome += tx.Amount
			case tx.Type == domain.TypeIncome:
				report.OtherIncome += tx.Amount
			}

			if tx.SessionID == nil {
				report.Unlinked = append(report.Unlinked, v)
				report.UnlinkedTotals.add(tx)
				continue
			}
			row := rows[*tx.SessionID]
			if row == nil {
				// A transaction dated this month for a session of
				// another month still needs a row to keep sums equal.
				sess, err := q.GetSession(ctx, *tx.SessionID)
				if err != nil {
					return err
				}
				if err := addRow(sess); err != nil {
					return err
				}
				row = rows[sess.ID]
			}
			switch {
			case tx.Type == domain.TypeExpense:
				row.Expense += tx.Amount
			case tx.Category == domain.CatSessionFee:
				row.FeeIncome += tx.Amount
			default:
				row.OtherIncome += tx.Amount
			}
		}

		for _, id := range order {
			report.Sessions = append(report.Sessions, *rows[id])
		}
		sort.SliceStable(report.Sessions, func(i, j int) bool {
			return report.Sessions[i].Session.Date < report.Sessions[j].Session.Date
		})
		for cat, amount := range categories {
			typ, _ := cat.Type()
			report.Categories = append(report.Categories, CategoryTotal{Category: cat, Type: typ, Amount: amount})
		}
		sort.Slice(report.Categories, func(i, j int) bool {
			a, b := report.Categories[i], report.Categories[j]
			if a.Type != b.Type {
				return a.Type == domain.TypeIncome
			}
			return a.Category < b.Category
		})
		for _, rp := range regular {
			report.RegularPayments = append(report.RegularPayments, *rp)
		}
		sort.Slice(report.RegularPayments, func(i, j int) bool {
			return report.RegularPayments[i].PlayerID < report.RegularPayments[j].PlayerID
		})
		if report.SessionCount > 0 {
			report.AvgIncome = report.Income / int64(report.SessionCount)
			report.AvgProfit = report.Profit() / int64(report.SessionCount)
		}
		return nil
	})
	return report, err
}

// SessionTransactions returns the transactions linked to a session.
func (s *Service) SessionTransactions(ctx context.Context, sessionID int64) ([]TransactionView, error) {
	var views []TransactionView
	err := s.store.InTx(ctx, func(q *storage.Queries) error {
		txs, err := q.ListTransactions(ctx, storage.TransactionFilter{SessionID: sessionID})
		if err != nil {
			return err
		}
		views, err = viewsOf(ctx, q, txs)
		return err
	})
	return views, err
}
