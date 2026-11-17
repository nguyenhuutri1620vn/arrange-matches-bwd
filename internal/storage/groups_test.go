package storage

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"arrange-matches-bwd/internal/domain"
	"arrange-matches-bwd/migrations"
)

// A database from before 0003 keeps its groups: each (session, group_no)
// becomes one session group shared by its members.
func TestMigrationMovesGroupNumbersToSessionGroups(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE schema_migrations (name TEXT PRIMARY KEY, applied_at INTEGER NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"0001_init.sql", "0002_session_player_group.sql"} {
		script, err := migrations.FS.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(script)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if _, err := db.Exec(`INSERT INTO schema_migrations VALUES (?, 0)`, name); err != nil {
			t.Fatal(err)
		}
	}
	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	mustExec(`INSERT INTO sessions (id, date, status, court_count, fee_male, fee_female, court_fee) VALUES (1, '2026-11-14', 'OPEN', 2, 70000, 60000, 0)`)
	for i := 1; i <= 5; i++ {
		mustExec(`INSERT INTO players (id, name, gender, level) VALUES (?, ?, 'M', 'TB')`, i, "p")
	}
	// Players 1-2 in group 1, 3-4 in group 2, 5 alone.
	for i, g := range []any{1, 1, 2, 2, nil} {
		mustExec(`INSERT INTO session_players (session_id, player_id, status, queued_at, checked_in_at, group_no) VALUES (1, ?, 'WAITING', ?, ?, ?)`,
			i+1, 1000+i, 1000+i, g)
	}
	db.Close()

	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	q := store.Q()
	groups, err := q.ListSessionGroups(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 || groups[0].No != 1 || groups[1].No != 2 || groups[0].Phase != domain.GroupTogether {
		t.Fatalf("groups = %+v", groups)
	}
	players, err := q.ListSessionPlayers(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := []int{int(groups[0].ID), int(groups[0].ID), int(groups[1].ID), int(groups[1].ID), 0}
	for i, p := range players {
		if p.Group != want[i] {
			t.Errorf("player %d group = %d, want %d", p.PlayerID, p.Group, want[i])
		}
	}
}

func TestInsertSessionGroupTakesSmallestFreeNumber(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	q := store.Q()
	if _, err := q.db.ExecContext(ctx, `INSERT INTO sessions (id, date, status, court_count, fee_male, fee_female, court_fee) VALUES (1, '2026-11-17', 'OPEN', 1, 0, 0, 0)`); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	add := func() domain.SessionGroup {
		g, err := q.InsertSessionGroup(ctx, domain.SessionGroup{SessionID: 1, Phase: domain.GroupTogether, PhaseStartedAt: now, CreatedAt: now})
		if err != nil {
			t.Fatal(err)
		}
		return g
	}
	g1, g2, g3 := add(), add(), add()
	if g1.No != 1 || g2.No != 2 || g3.No != 3 {
		t.Fatalf("numbers %d %d %d", g1.No, g2.No, g3.No)
	}
	// All three are empty, so deleting empty groups frees every number.
	if err := q.DeleteEmptySessionGroups(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if g := add(); g.No != 1 {
		t.Fatalf("after delete got number %d, want 1", g.No)
	}
	g := add()
	g.Phase, g.PhaseStartedAt = domain.GroupSplit, now.Add(time.Hour)
	if err := q.UpdateSessionGroupPhase(ctx, g); err != nil {
		t.Fatal(err)
	}
	got, err := q.GetSessionGroup(ctx, g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Phase != domain.GroupSplit || !got.PhaseStartedAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("got %+v", got)
	}
}
