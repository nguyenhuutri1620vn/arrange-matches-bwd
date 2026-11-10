package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"arrange-matches-bwd/internal/domain"
	"arrange-matches-bwd/internal/storage"
)

type fixture struct {
	t     *testing.T
	ctx   context.Context
	store *storage.Store
	svc   *Service
	clock time.Time
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	store, err := storage.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	f := &fixture{t: t, ctx: ctx, store: store, clock: time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)}
	// Each call to the clock advances one second so queue order is stable.
	f.svc = New(store, func() time.Time {
		f.clock = f.clock.Add(time.Second)
		return f.clock
	})
	return f
}

func (f *fixture) player(name string, g domain.Gender, l domain.Level, regular bool) domain.Player {
	f.t.Helper()
	p, err := f.svc.CreatePlayer(f.ctx, domain.Player{Name: name, Gender: g, Level: l, IsRegular: regular})
	if err != nil {
		f.t.Fatal(err)
	}
	return p
}

// open starts a session and checks in n men of level TB.
func (f *fixture) open(courts, n int) *State {
	f.t.Helper()
	state, err := f.svc.Create(f.ctx, CreateParams{CourtCount: courts})
	if err != nil {
		f.t.Fatal(err)
	}
	for i := 0; i < n; i++ {
		p := f.player("P"+string(rune('A'+i)), domain.GenderMale, domain.LevelTB, false)
		if state, err = f.svc.CheckIn(f.ctx, state.Session.ID, p.ID); err != nil {
			f.t.Fatal(err)
		}
	}
	return state
}

func (f *fixture) must(state *State, err error) *State {
	f.t.Helper()
	if err != nil {
		f.t.Fatal(err)
	}
	return state
}

func countStatus(s *State, status domain.PlayerStatus) int {
	n := 0
	for _, p := range s.Players {
		if p.Status == status {
			n++
		}
	}
	return n
}

func findPlayer(s *State, id int64) PlayerView {
	for _, p := range s.Players {
		if p.ID == id {
			return p
		}
	}
	return PlayerView{}
}

func TestOnlyOneOpenSession(t *testing.T) {
	f := newFixture(t)
	f.open(1, 0)
	if _, err := f.svc.Create(f.ctx, CreateParams{CourtCount: 1}); !errors.Is(err, domain.ErrSessionAlreadyOpen) {
		t.Fatalf("want ErrSessionAlreadyOpen, got %v", err)
	}
	if _, err := f.svc.Create(f.ctx, CreateParams{CourtCount: 4}); !errors.Is(err, domain.ErrInvalidCourtCount) {
		t.Fatalf("want ErrInvalidCourtCount, got %v", err)
	}
}

func TestMatchLifecycle(t *testing.T) {
	f := newFixture(t)
	state := f.open(1, 6)
	court := state.Courts[0]

	// Propose: a DRAFT holds four players, who stay WAITING.
	state = f.must(f.svc.Propose(f.ctx, court.ID))
	draft := state.Courts[0].Match
	if draft == nil || draft.Status != domain.MatchDraft || len(draft.Players) != 4 {
		t.Fatalf("no draft after propose: %+v", draft)
	}
	if state.Courts[0].Status != domain.CourtEmpty || countStatus(state, domain.PlayerWaiting) != 6 {
		t.Fatal("draft must not change court or player status")
	}
	held := 0
	for _, p := range state.Players {
		if p.Held {
			held++
		}
	}
	if held != 4 {
		t.Fatalf("held = %d, want 4", held)
	}
	if _, err := f.svc.Propose(f.ctx, court.ID); !errors.Is(err, domain.ErrCourtNotEmpty) {
		t.Fatalf("second draft on the same court: %v", err)
	}

	// Confirm: match ONGOING, court IN_USE, four PLAYING, others rest +1.
	state = f.must(f.svc.Confirm(f.ctx, draft.ID, false))
	if m := state.Courts[0].Match; m == nil || m.Status != domain.MatchOngoing || m.StartedAt == nil {
		t.Fatalf("match not ongoing: %+v", m)
	}
	if state.Courts[0].Status != domain.CourtInUse {
		t.Fatal("court not IN_USE")
	}
	if countStatus(state, domain.PlayerPlaying) != 4 || countStatus(state, domain.PlayerWaiting) != 2 {
		t.Fatal("want 4 playing and 2 waiting")
	}
	for _, p := range state.Players {
		wantRest := 1
		if p.Status == domain.PlayerPlaying {
			wantRest = 0
		}
		if p.ConsecutiveRest != wantRest {
			t.Fatalf("%s rest = %d, want %d", p.Name, p.ConsecutiveRest, wantRest)
		}
	}
	if _, err := f.svc.Confirm(f.ctx, draft.ID, false); !errors.Is(err, domain.ErrMatchNotDraft) {
		t.Fatalf("confirming twice: %v", err)
	}

	// Finish: court EMPTY, four back to WAITING with one match each.
	matchID := draft.ID
	state = f.must(f.svc.Finish(f.ctx, matchID))
	if state.Courts[0].Status != domain.CourtEmpty {
		t.Fatal("court not EMPTY after finish")
	}
	if countStatus(state, domain.PlayerWaiting) != 6 || countStatus(state, domain.PlayerPlaying) != 0 {
		t.Fatal("players not back in the queue")
	}
	played := 0
	for _, mp := range draft.Players {
		if findPlayer(state, mp.SessionPlayerID).MatchesPlayed == 1 {
			played++
		}
	}
	if played != 4 {
		t.Fatalf("matches counted for %d players, want 4", played)
	}
	if _, err := f.svc.Finish(f.ctx, matchID); !errors.Is(err, domain.ErrMatchNotOngoing) {
		t.Fatalf("finishing twice: %v", err)
	}

	// Finishing prepares the next proposal, led by the two who sat out.
	next := state.Courts[0].Match
	if next == nil || next.Status != domain.MatchDraft {
		t.Fatalf("no follow-up proposal: %+v", next)
	}
	fresh := 0
	for _, mp := range next.Players {
		if findPlayer(state, mp.SessionPlayerID).MatchesPlayed == 0 {
			fresh++
		}
	}
	if fresh != 2 {
		t.Fatalf("follow-up proposal has %d players with no match, want both", fresh)
	}
}

func TestDraftHoldsPlayersAcrossCourts(t *testing.T) {
	f := newFixture(t)
	state := f.open(2, 8)
	state = f.must(f.svc.Propose(f.ctx, state.Courts[0].ID))
	state = f.must(f.svc.Propose(f.ctx, state.Courts[1].ID))
	seen := map[int64]bool{}
	for _, c := range state.Courts {
		for _, mp := range c.Match.Players {
			if seen[mp.SessionPlayerID] {
				t.Fatalf("player %d is in two proposals", mp.SessionPlayerID)
			}
			seen[mp.SessionPlayerID] = true
		}
	}

	// Dismissing frees the players and records nothing.
	state = f.must(f.svc.DismissDraft(f.ctx, state.Courts[1].Match.ID))
	if state.Courts[1].Match != nil {
		t.Fatal("draft still there after dismiss")
	}
	for _, p := range state.Players {
		if p.MatchesPlayed != 0 || p.ConsecutiveRest != 0 {
			t.Fatalf("dismiss changed counters of %s", p.Name)
		}
	}
}

func TestProposeNeedsFourFreePlayers(t *testing.T) {
	f := newFixture(t)
	state := f.open(2, 6)
	state = f.must(f.svc.Propose(f.ctx, state.Courts[0].ID))
	if _, err := f.svc.Propose(f.ctx, state.Courts[1].ID); !errors.Is(err, domain.ErrNotEnoughPlayers) {
		t.Fatalf("want ErrNotEnoughPlayers with 2 free players, got %v", err)
	}
}

func TestSwapChecksGenderAndWarnsOnLevel(t *testing.T) {
	f := newFixture(t)
	state := f.open(1, 4)
	sid := state.Session.ID
	woman := f.player("Lan", domain.GenderFemale, domain.LevelTB, false)
	weak := f.player("Khoa", domain.GenderMale, domain.LevelY, false)
	state = f.must(f.svc.CheckIn(f.ctx, sid, woman.ID))
	state = f.must(f.svc.CheckIn(f.ctx, sid, weak.ID))
	state = f.must(f.svc.Propose(f.ctx, state.Courts[0].ID))
	draft := state.Courts[0].Match
	out := draft.Players[0].SessionPlayerID

	var womanSP, weakSP int64
	for _, p := range state.Players {
		switch p.PlayerID {
		case woman.ID:
			womanSP = p.ID
		case weak.ID:
			weakSP = p.ID
		}
	}

	// 3 men + 1 woman of the same level: hard block, draft unchanged.
	if _, err := f.svc.SwapDraftPlayer(f.ctx, draft.ID, out, womanSP); !errors.Is(err, domain.ErrInvalidGenderLineup) {
		t.Fatalf("want ErrInvalidGenderLineup, got %v", err)
	}
	state = f.must(f.svc.Current(f.ctx))
	if got := state.Courts[0].Match.Players[0].SessionPlayerID; got != out {
		t.Fatal("blocked swap still changed the draft")
	}

	// TB,TB,TB + Y: allowed, flagged unbalanced (5+5 vs 5+1).
	state = f.must(f.svc.SwapDraftPlayer(f.ctx, draft.ID, out, weakSP))
	m := state.Courts[0].Match
	if m.Balanced || m.Diff != 4 {
		t.Fatalf("want unbalanced diff 4, got diff %d balanced %v", m.Diff, m.Balanced)
	}
	if _, err := f.svc.Confirm(f.ctx, draft.ID, false); !errors.Is(err, domain.ErrUnbalancedLineup) {
		t.Fatalf("want ErrUnbalancedLineup without force, got %v", err)
	}
	state = f.must(f.svc.Confirm(f.ctx, draft.ID, true))
	if state.Courts[0].Match.Status != domain.MatchOngoing {
		t.Fatal("forced confirm did not start the match")
	}
}

func TestSwapRejectsUnavailablePlayers(t *testing.T) {
	f := newFixture(t)
	state := f.open(2, 9)
	state = f.must(f.svc.Propose(f.ctx, state.Courts[0].ID))
	state = f.must(f.svc.Propose(f.ctx, state.Courts[1].ID))
	d1, d2 := state.Courts[0].Match, state.Courts[1].Match

	// A player held by the other court's proposal.
	if _, err := f.svc.SwapDraftPlayer(f.ctx, d1.ID, d1.Players[0].SessionPlayerID, d2.Players[0].SessionPlayerID); !errors.Is(err, domain.ErrPlayerHeld) {
		t.Fatalf("want ErrPlayerHeld, got %v", err)
	}
	// A resting player.
	var free int64
	for _, p := range state.Players {
		if !p.Held {
			free = p.ID
		}
	}
	f.must(f.svc.SetPlayerStatus(f.ctx, free, domain.PlayerResting))
	if _, err := f.svc.SwapDraftPlayer(f.ctx, d1.ID, d1.Players[0].SessionPlayerID, free); !errors.Is(err, domain.ErrPlayerNotWaiting) {
		t.Fatalf("want ErrPlayerNotWaiting, got %v", err)
	}
}

func TestRestingRules(t *testing.T) {
	f := newFixture(t)
	state := f.open(1, 5)
	state = f.must(f.svc.Propose(f.ctx, state.Courts[0].ID))
	draft := state.Courts[0].Match
	heldID := draft.Players[0].SessionPlayerID

	// Resting a held player drops the proposal.
	state = f.must(f.svc.SetPlayerStatus(f.ctx, heldID, domain.PlayerResting))
	if state.Courts[0].Match != nil {
		t.Fatal("proposal survived its player going to rest")
	}
	// With four left waiting, a new match starts; the resting player
	// accumulates no rest.
	state = f.must(f.svc.Propose(f.ctx, state.Courts[0].ID))
	state = f.must(f.svc.Confirm(f.ctx, state.Courts[0].Match.ID, false))
	if p := findPlayer(state, heldID); p.ConsecutiveRest != 0 || p.Status != domain.PlayerResting {
		t.Fatalf("resting player: %+v", p.SessionPlayer)
	}
	// A playing player cannot be set to rest by the host.
	playing := state.Courts[0].Match.Players[0].SessionPlayerID
	if _, err := f.svc.SetPlayerStatus(f.ctx, playing, domain.PlayerResting); !errors.Is(err, domain.ErrPlayerPlaying) {
		t.Fatalf("want ErrPlayerPlaying, got %v", err)
	}
	// Coming back re-queues at the end.
	before := findPlayer(state, heldID).QueuedAt
	state = f.must(f.svc.SetPlayerStatus(f.ctx, heldID, domain.PlayerWaiting))
	if p := findPlayer(state, heldID); p.Status != domain.PlayerWaiting || !p.QueuedAt.After(before) {
		t.Fatalf("resume did not re-queue: %+v", p.SessionPlayer)
	}
}

func TestLateCheckInGetsPriorityOffset(t *testing.T) {
	f := newFixture(t)
	state := f.open(1, 4)
	sid := state.Session.ID
	for i := 0; i < 2; i++ {
		state = f.must(f.svc.Propose(f.ctx, state.Courts[0].ID))
		state = f.must(f.svc.Confirm(f.ctx, state.Courts[0].Match.ID, false))
		state = f.must(f.svc.Finish(f.ctx, state.Courts[0].Match.ID))
		// Finish prepared the next proposal; drop it to loop cleanly.
		state = f.must(f.svc.DismissDraft(f.ctx, state.Courts[0].Match.ID))
	}
	late := f.player("Muộn", domain.GenderMale, domain.LevelTB, false)
	state = f.must(f.svc.CheckIn(f.ctx, sid, late.ID))
	for _, p := range state.Players {
		if p.PlayerID == late.ID {
			if p.PriorityOffset != 2 || p.MatchesPlayed != 0 {
				t.Fatalf("late player offset %d matches %d, want offset 2 matches 0", p.PriorityOffset, p.MatchesPlayed)
			}
		}
	}
	if _, err := f.svc.CheckIn(f.ctx, sid, late.ID); !errors.Is(err, domain.ErrAlreadyCheckedIn) {
		t.Fatalf("want ErrAlreadyCheckedIn, got %v", err)
	}
}

func TestRemovePlayerOnlyBeforeAnyMatch(t *testing.T) {
	f := newFixture(t)
	state := f.open(1, 5)
	state = f.must(f.svc.Propose(f.ctx, state.Courts[0].ID))
	var free, held int64
	for _, p := range state.Players {
		if p.Held {
			held = p.ID
		} else {
			free = p.ID
		}
	}
	if _, err := f.svc.RemovePlayer(f.ctx, held); !errors.Is(err, domain.ErrPlayerHasPlayed) {
		t.Fatalf("removed a player held by a proposal: %v", err)
	}
	state = f.must(f.svc.RemovePlayer(f.ctx, free))
	if len(state.Players) != 4 {
		t.Fatalf("players = %d, want 4", len(state.Players))
	}
}

func TestCourtCountChanges(t *testing.T) {
	f := newFixture(t)
	state := f.open(2, 8)
	sid := state.Session.ID
	state = f.must(f.svc.Propose(f.ctx, state.Courts[1].ID))

	// Court 2 has a proposal: cannot close it.
	if _, err := f.svc.SetCourtCount(f.ctx, sid, 1); !errors.Is(err, domain.ErrCourtBusy) {
		t.Fatalf("want ErrCourtBusy for a court with a draft, got %v", err)
	}
	state = f.must(f.svc.Confirm(f.ctx, state.Courts[1].Match.ID, false))
	if _, err := f.svc.SetCourtCount(f.ctx, sid, 1); !errors.Is(err, domain.ErrCourtBusy) {
		t.Fatalf("want ErrCourtBusy for a court in use, got %v", err)
	}
	state = f.must(f.svc.SetCourtCount(f.ctx, sid, 3))
	if len(state.Courts) != 3 {
		t.Fatalf("courts = %d, want 3", len(state.Courts))
	}
	// Court 3 is empty: closing it is fine.
	state = f.must(f.svc.SetCourtCount(f.ctx, sid, 2))
	if len(state.Courts) != 2 {
		t.Fatalf("courts = %d, want 2", len(state.Courts))
	}
}

func TestEndSession(t *testing.T) {
	f := newFixture(t)
	courtFee := int64(110000)
	state := f.must(f.svc.Create(f.ctx, CreateParams{CourtCount: 2, CourtFee: &courtFee}))
	sid := state.Session.ID
	add := func(name string, g domain.Gender, regular bool) {
		p := f.player(name, g, domain.LevelTB, regular)
		state = f.must(f.svc.CheckIn(f.ctx, sid, p.ID))
	}
	for _, n := range []string{"A", "B", "C", "D", "E", "F"} {
		add(n, domain.GenderMale, false)
	}
	add("Nữ1", domain.GenderFemale, false)
	add("Nữ2", domain.GenderFemale, false)
	add("Cố định", domain.GenderMale, true)

	// 6 casual men + 2 casual women; the regular owes nothing.
	if state.Fees.Expected != 6*70000+2*60000 {
		t.Fatalf("expected fees = %d", state.Fees.Expected)
	}
	paid := true
	for _, p := range state.Players[:3] {
		state = f.must(f.svc.SetPayment(f.ctx, p.ID, PaymentUpdate{IsPaid: &paid}))
	}

	// One running match and one proposal at the end.
	state = f.must(f.svc.Propose(f.ctx, state.Courts[0].ID))
	state = f.must(f.svc.Confirm(f.ctx, state.Courts[0].Match.ID, false))
	state = f.must(f.svc.Propose(f.ctx, state.Courts[1].ID))
	ongoing := state.Courts[0].Match

	state = f.must(f.svc.End(f.ctx, sid))
	if state.Session.Status != domain.SessionEnded || state.Session.EndTime == nil {
		t.Fatal("session not ended")
	}
	for _, c := range state.Courts {
		if c.Status != domain.CourtEmpty || c.Match != nil {
			t.Fatalf("court %d not cleared", c.Index)
		}
	}
	for _, mp := range ongoing.Players {
		if p := findPlayer(state, mp.SessionPlayerID); p.MatchesPlayed != 1 || p.Status != domain.PlayerWaiting {
			t.Fatalf("running match not counted for %s", p.Name)
		}
	}
	if drafts, _ := f.store.Q().ListMatches(f.ctx, sid, domain.MatchDraft); len(drafts) != 0 {
		t.Fatal("draft not deleted at session end")
	}

	txs, err := f.store.Q().ListTransactions(f.ctx, storage.TransactionFilter{SessionID: sid})
	if err != nil {
		t.Fatal(err)
	}
	var income, rent int64
	for _, tx := range txs {
		switch tx.Category {
		case domain.CatSessionFee:
			income = tx.Amount
		case domain.CatCourtRent:
			rent = tx.Amount
		}
	}
	if income != 3*70000 || rent != 2*courtFee || len(txs) != 2 {
		t.Fatalf("income %d rent %d (%d transactions)", income, rent, len(txs))
	}

	// Editing payment after the end keeps the income in sync.
	state = f.must(f.svc.SetPayment(f.ctx, state.Players[6].ID, PaymentUpdate{IsPaid: &paid}))
	discount := int64(50000)
	state = f.must(f.svc.SetPayment(f.ctx, state.Players[0].ID, PaymentUpdate{SetFeeOverride: true, FeeOverride: &discount}))
	tx, err := f.store.Q().GetAutoSessionFee(f.ctx, sid)
	if err != nil {
		t.Fatal(err)
	}
	if want := int64(50000 + 2*70000 + 60000); tx.Amount != want || state.Fees.Collected != want {
		t.Fatalf("synced income %d, state %d, want %d", tx.Amount, state.Fees.Collected, want)
	}

	// An ended session accepts no play changes, and a new one can open.
	if _, err := f.svc.Propose(f.ctx, state.Courts[0].ID); !errors.Is(err, domain.ErrSessionEnded) {
		t.Fatalf("want ErrSessionEnded, got %v", err)
	}
	if _, err := f.svc.Create(f.ctx, CreateParams{CourtCount: 1}); err != nil {
		t.Fatalf("new session after end: %v", err)
	}
}

func TestProposeAvoidsRecentTeammates(t *testing.T) {
	f := newFixture(t)
	state := f.open(1, 4)
	state = f.must(f.svc.Propose(f.ctx, state.Courts[0].ID))
	first := state.Courts[0].Match
	state = f.must(f.svc.Confirm(f.ctx, first.ID, false))
	state = f.must(f.svc.Finish(f.ctx, first.ID))
	second := state.Courts[0].Match
	if second == nil {
		t.Fatal("no follow-up proposal")
	}
	team := func(m *MatchView, id int64) domain.Team {
		for _, mp := range m.Players {
			if mp.SessionPlayerID == id {
				return mp.Team
			}
		}
		return ""
	}
	a, b := first.Players[0].SessionPlayerID, first.Players[1].SessionPlayerID
	if team(first, a) != team(first, b) {
		t.Fatal("test assumes the first two listed players are teammates")
	}
	if team(second, a) == team(second, b) {
		t.Fatal("same four players, equal levels: teammates should be reshuffled")
	}
}

func TestSeedIsIdempotent(t *testing.T) {
	f := newFixture(t)
	for i := 0; i < 2; i++ {
		if err := f.store.Seed(f.ctx); err != nil {
			t.Fatal(err)
		}
	}
	players, err := f.svc.ListPlayers(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(players) != 3 {
		t.Fatalf("players = %d, want 3", len(players))
	}
	txs, err := f.store.Q().ListTransactions(f.ctx, storage.TransactionFilter{Month: "2026-11"})
	if err != nil {
		t.Fatal(err)
	}
	if len(txs) != 6 {
		t.Fatalf("transactions = %d, want 6", len(txs))
	}
}
