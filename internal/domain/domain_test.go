package domain

import (
	"errors"
	"testing"
	"time"
)

func TestLevelScores(t *testing.T) {
	want := map[Level]int{"Y": 1, "Y+": 2, "TBY": 3, "TB-": 4, "TB": 5, "TB+": 6}
	for l, score := range want {
		if l.Score() != score || !l.Valid() {
			t.Errorf("level %s: score %d valid %v, want %d", l, l.Score(), l.Valid(), score)
		}
	}
	if Level("X").Valid() {
		t.Error("unknown level reported valid")
	}
}

func TestSessionPlayerLifecycle(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	p := SessionPlayer{Status: PlayerWaiting, ConsecutiveRest: 0}

	p.SkipMatch()
	p.SkipMatch()
	if p.ConsecutiveRest != 2 {
		t.Fatalf("rest = %d after two skipped matches, want 2", p.ConsecutiveRest)
	}

	if err := p.StartMatch(); err != nil {
		t.Fatal(err)
	}
	if p.Status != PlayerPlaying || p.ConsecutiveRest != 0 {
		t.Fatalf("after StartMatch: %+v", p)
	}
	p.SkipMatch()
	if p.ConsecutiveRest != 0 {
		t.Fatal("rest increased while playing")
	}
	if err := p.StartMatch(); !errors.Is(err, ErrPlayerNotWaiting) {
		t.Fatalf("playing player started another match: %v", err)
	}
	if err := p.Rest(); !errors.Is(err, ErrPlayerPlaying) {
		t.Fatalf("playing player was allowed to rest: %v", err)
	}

	if err := p.FinishMatch(now); err != nil {
		t.Fatal(err)
	}
	if p.Status != PlayerWaiting || p.MatchesPlayed != 1 || !p.QueuedAt.Equal(now) {
		t.Fatalf("after FinishMatch: %+v", p)
	}

	if err := p.Rest(); err != nil {
		t.Fatal(err)
	}
	p.SkipMatch()
	if p.ConsecutiveRest != 0 {
		t.Fatal("rest increased while RESTING")
	}
	later := now.Add(time.Minute)
	if err := p.Resume(later); err != nil {
		t.Fatal(err)
	}
	if p.Status != PlayerWaiting || !p.QueuedAt.Equal(later) {
		t.Fatalf("after Resume: %+v", p)
	}
}

func TestCheckInOffset(t *testing.T) {
	if got := CheckInOffset(nil); got != 0 {
		t.Fatalf("empty session offset = %d, want 0", got)
	}
	present := []SessionPlayer{
		{Status: PlayerWaiting, MatchesPlayed: 3},
		{Status: PlayerPlaying, MatchesPlayed: 1, PriorityOffset: 1},
		{Status: PlayerResting, MatchesPlayed: 0},
	}
	if got := CheckInOffset(present); got != 2 {
		t.Fatalf("offset = %d, want 2 (resting players are not present)", got)
	}
}

func TestSessionPlayerFee(t *testing.T) {
	s := &Session{FeeMale: 70000, FeeFemale: 60000}
	free := int64(0)
	discount := int64(50000)
	tests := []struct {
		name string
		p    SessionPlayer
		want int64
	}{
		{"casual man", SessionPlayer{Gender: GenderMale}, 70000},
		{"casual woman", SessionPlayer{Gender: GenderFemale}, 60000},
		{"regular pays nothing per session", SessionPlayer{Gender: GenderMale, IsRegular: true}, 0},
		{"override free", SessionPlayer{Gender: GenderMale, FeeOverride: &free}, 0},
		{"override discount", SessionPlayer{Gender: GenderFemale, FeeOverride: &discount}, 50000},
		{"override on regular", SessionPlayer{Gender: GenderMale, IsRegular: true, FeeOverride: &discount}, 50000},
	}
	for _, tc := range tests {
		if got := tc.p.Fee(s); got != tc.want {
			t.Errorf("%s: fee %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestSumSessionFees(t *testing.T) {
	s := &Session{FeeMale: 70000, FeeFemale: 60000}
	players := []SessionPlayer{
		{Gender: GenderMale, IsPaid: true},
		{Gender: GenderMale},
		{Gender: GenderFemale, IsPaid: true},
		{Gender: GenderFemale, IsRegular: true, IsPaid: true},
	}
	f := SumSessionFees(s, players)
	if f.Expected != 200000 || f.Collected != 130000 || f.Outstanding() != 70000 {
		t.Fatalf("fees = %+v", f)
	}
}

func TestTransactionValidate(t *testing.T) {
	ok := Transaction{Type: TypeIncome, Category: CatDrinks, Amount: 10000, Date: "2026-11-10"}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	wrongType := Transaction{Type: TypeExpense, Category: CatDrinks, Amount: 1, Date: "2026-11-10"}
	if err := wrongType.Validate(); !errors.Is(err, ErrInvalidCategory) {
		t.Fatalf("want ErrInvalidCategory, got %v", err)
	}
	badDate := Transaction{Type: TypeIncome, Category: CatDrinks, Amount: 1, Date: "2026-13-01"}
	if err := badDate.Validate(); !errors.Is(err, ErrInvalidDate) {
		t.Fatalf("want ErrInvalidDate, got %v", err)
	}
}

func TestVNDate(t *testing.T) {
	// 18:30 UTC on Oct 31 is already Nov 1 in Vietnam.
	at := time.Date(2026, 10, 31, 18, 30, 0, 0, time.UTC)
	if got := VNDate(at); got != "2026-11-01" {
		t.Fatalf("VNDate = %s, want 2026-11-01", got)
	}
}
