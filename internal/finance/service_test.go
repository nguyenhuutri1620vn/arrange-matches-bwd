package finance

import (
	"context"
	"errors"
	"testing"

	"arrange-matches-bwd/internal/domain"
	"arrange-matches-bwd/internal/session"
	"arrange-matches-bwd/internal/storage"
)

func newServices(t *testing.T) (context.Context, *storage.Store, *Service) {
	t.Helper()
	ctx := context.Background()
	store, err := storage.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return ctx, store, New(store, nil)
}

func TestSeedReportNovember2026(t *testing.T) {
	ctx, store, svc := newServices(t)
	if err := store.Seed(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.Seed(ctx); err != nil {
		t.Fatal(err)
	}

	r, err := svc.Month(ctx, "2026-11")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.RegularPayments) != 3 {
		t.Fatalf("regular payments = %d, want 3 (Huyền's 0 included)", len(r.RegularPayments))
	}
	if r.RegularIncome != 1333000+666000 {
		t.Fatalf("regular income = %d", r.RegularIncome)
	}
	if r.CasualIncome != 315000+790000 {
		t.Fatalf("casual income = %d", r.CasualIncome)
	}
	if len(r.Sessions) != 2 || r.SessionCount != 2 {
		t.Fatalf("sessions = %d, want 2", len(r.Sessions))
	}
	if r.Expense != 220000 {
		t.Fatalf("expense = %d, want 220000", r.Expense)
	}
	if r.Sessions[0].Expense != 0 || r.Sessions[1].Expense != 220000 || r.Sessions[0].PlayerCount != 0 {
		t.Fatalf("session rows: %+v", r.Sessions)
	}
	assertMonthAddsUp(t, r)

	sum, err := svc.Summary(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.Months) != 1 || sum.Months[0].Totals != r.Totals || sum.Totals != r.Totals {
		t.Fatalf("summary %+v does not match month %+v", sum, r.Totals)
	}
}

// assertMonthAddsUp checks the drill-down invariants of a month report.
func assertMonthAddsUp(t *testing.T, r MonthReport) {
	t.Helper()
	if r.RegularIncome+r.CasualIncome+r.OtherIncome != r.Income {
		t.Fatalf("regular %d + casual %d + other %d != income %d", r.RegularIncome, r.CasualIncome, r.OtherIncome, r.Income)
	}
	children := r.UnlinkedTotals
	for _, s := range r.Sessions {
		children.Income += s.Totals().Income
		children.Expense += s.Totals().Expense
	}
	if children != r.Totals {
		t.Fatalf("sessions + unlinked = %+v, month = %+v", children, r.Totals)
	}
	var byCategory Totals
	for _, c := range r.Categories {
		if c.Type == domain.TypeIncome {
			byCategory.Income += c.Amount
		} else {
			byCategory.Expense += c.Amount
		}
	}
	if byCategory != r.Totals {
		t.Fatalf("categories = %+v, month = %+v", byCategory, r.Totals)
	}
}

func TestMonthReportIgnoresDeletedAndUsesVietnamMonth(t *testing.T) {
	ctx, _, svc := newServices(t)
	keep, err := svc.Create(ctx, domain.Transaction{Type: domain.TypeIncome, Category: domain.CatDrinks, Amount: 50000, Date: "2026-11-30"})
	if err != nil {
		t.Fatal(err)
	}
	gone, err := svc.Create(ctx, domain.Transaction{Type: domain.TypeExpense, Category: domain.CatShuttles, Amount: 300000, Date: "2026-11-15"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, domain.Transaction{Type: domain.TypeIncome, Category: domain.CatDrinks, Amount: 99000, Date: "2026-12-01"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, gone.ID); err != nil {
		t.Fatal(err)
	}

	r, err := svc.Month(ctx, "2026-11")
	if err != nil {
		t.Fatal(err)
	}
	if r.Income != 50000 || r.Expense != 0 || r.OtherIncome != 50000 {
		t.Fatalf("november totals %+v other %d", r.Totals, r.OtherIncome)
	}
	if len(r.Unlinked) != 1 || r.Unlinked[0].ID != keep.ID {
		t.Fatalf("unlinked = %+v", r.Unlinked)
	}
	assertMonthAddsUp(t, r)

	sum, err := svc.Summary(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Income != 149000 || sum.Expense != 0 || len(sum.Months) != 2 {
		t.Fatalf("summary = %+v", sum)
	}
	if err := svc.Delete(ctx, gone.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleting twice: %v", err)
	}
}

func TestMonthReportWithLiveSession(t *testing.T) {
	ctx, store, svc := newServices(t)
	sessions := session.New(store, nil)
	courtFee := int64(100000)
	state, err := sessions.Create(ctx, session.CreateParams{CourtCount: 1, CourtFee: &courtFee})
	if err != nil {
		t.Fatal(err)
	}
	sid := state.Session.ID
	month := state.Session.Date[:7]
	paid := true
	people := []domain.Player{
		{Name: "Nam", Gender: domain.GenderMale, Level: domain.LevelTB},
		{Name: "Nữ", Gender: domain.GenderFemale, Level: domain.LevelTB},
		{Name: "Cố định", Gender: domain.GenderMale, Level: domain.LevelTB, IsRegular: true},
	}
	for _, p := range people {
		p, err := sessions.CreatePlayer(ctx, p)
		if err != nil {
			t.Fatal(err)
		}
		if state, err = sessions.CheckIn(ctx, sid, p.ID); err != nil {
			t.Fatal(err)
		}
	}
	if state, err = sessions.SetPayment(ctx, state.Players[0].ID, session.PaymentUpdate{IsPaid: &paid}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, domain.Transaction{SessionID: &sid, Type: domain.TypeIncome, Category: domain.CatDrinks, Amount: 20000, Date: state.Session.Date}); err != nil {
		t.Fatal(err)
	}
	regularID := state.Players[2].PlayerID
	if _, err := svc.Create(ctx, domain.Transaction{PlayerID: &regularID, Type: domain.TypeIncome, Category: domain.CatMonthlyFee, Amount: 500000, Date: state.Session.Date}); err != nil {
		t.Fatal(err)
	}
	casualID := state.Players[0].PlayerID
	if _, err := svc.Create(ctx, domain.Transaction{PlayerID: &casualID, Type: domain.TypeIncome, Category: domain.CatMonthlyFee, Amount: 1, Date: state.Session.Date}); !errors.Is(err, domain.ErrNotRegularPlayer) {
		t.Fatalf("monthly fee for a casual player: %v", err)
	}
	if _, err = sessions.End(ctx, sid); err != nil {
		t.Fatal(err)
	}

	r, err := svc.Month(ctx, month)
	if err != nil {
		t.Fatal(err)
	}
	assertMonthAddsUp(t, r)
	if len(r.Sessions) != 1 {
		t.Fatalf("sessions = %d", len(r.Sessions))
	}
	row := r.Sessions[0]
	if row.PlayerCount != 3 || row.Male != 2 || row.Female != 1 || row.Casual != 2 || row.Regular != 1 {
		t.Fatalf("head counts: %+v", row)
	}
	if row.Fees.Expected != 130000 || row.Fees.Collected != 70000 || row.Fees.Outstanding() != 60000 {
		t.Fatalf("fees: %+v", row.Fees)
	}
	if row.FeeIncome != 70000 || row.OtherIncome != 20000 || row.Expense != 100000 {
		t.Fatalf("money: %+v", row)
	}
	if r.RegularIncome != 500000 || r.CasualIncome != 70000 || r.OtherIncome != 20000 {
		t.Fatalf("split: regular %d casual %d other %d", r.RegularIncome, r.CasualIncome, r.OtherIncome)
	}

	// The auto session fee follows the paid flags and is not editable.
	txs, err := svc.SessionTransactions(ctx, sid)
	if err != nil {
		t.Fatal(err)
	}
	for _, tx := range txs {
		if tx.Auto {
			tx.Amount = 1
			if _, err := svc.Update(ctx, tx.Transaction); !errors.Is(err, domain.ErrTransactionIsAuto) {
				t.Fatalf("editing auto transaction: %v", err)
			}
		}
	}
}
