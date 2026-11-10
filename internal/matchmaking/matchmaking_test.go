package matchmaking

import (
	"errors"
	"sort"
	"testing"
	"time"

	"arrange-matches-bwd/internal/domain"
)

var (
	rules = domain.DefaultSettings().Rules
	t0    = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
)

const (
	M = domain.GenderMale
	F = domain.GenderFemale
)

// cand builds a candidate queued id seconds after t0.
func cand(id int64, g domain.Gender, l domain.Level) Candidate {
	return Candidate{ID: id, Gender: g, Level: l, QueuedAt: t0.Add(time.Duration(id) * time.Second)}
}

func ids(l Lineup) []int64 {
	out := []int64{l.TeamA[0], l.TeamA[1], l.TeamB[0], l.TeamB[1]}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func sameIDs(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sameTeam(team [2]int64, x, y int64) bool {
	return (team[0] == x && team[1] == y) || (team[0] == y && team[1] == x)
}

func teammates(l Lineup, x, y int64) bool {
	return sameTeam(l.TeamA, x, y) || sameTeam(l.TeamB, x, y)
}

func TestArrangeGenderRules(t *testing.T) {
	tests := []struct {
		name    string
		four    [4]Candidate
		wantErr bool
		// pair that must be on the same team, when set
		mixed [2]int64
	}{
		{name: "4 men", four: [4]Candidate{cand(1, M, "TB"), cand(2, M, "TB"), cand(3, M, "TB"), cand(4, M, "TB")}},
		{name: "4 women", four: [4]Candidate{cand(1, F, "Y"), cand(2, F, "Y+"), cand(3, F, "Y"), cand(4, F, "Y+")}},
		{name: "3 men 1 woman, gap exactly 2", four: [4]Candidate{cand(1, M, "TB"), cand(2, M, "TB"), cand(3, M, "TB"), cand(4, F, "TBY")}},
		{name: "3 men 1 woman, average gap 5/3 is below 2", four: [4]Candidate{cand(1, M, "TB"), cand(2, M, "TB"), cand(3, M, "TB-"), cand(4, F, "TBY")}, wantErr: true},
		{name: "3 men 1 woman, same level", four: [4]Candidate{cand(1, M, "TB"), cand(2, M, "TB"), cand(3, M, "TB"), cand(4, F, "TB")}, wantErr: true},
		{name: "3 women 1 man, gap 2", four: [4]Candidate{cand(1, F, "TB"), cand(2, F, "TB"), cand(3, F, "TB"), cand(4, M, "TBY")}},
		{name: "3 women 1 man, man stronger", four: [4]Candidate{cand(1, F, "TBY"), cand(2, F, "TBY"), cand(3, F, "TBY"), cand(4, M, "TB")}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Arrange(tc.four, rules)
			if tc.wantErr {
				if !errors.Is(err, domain.ErrInvalidGenderLineup) {
					t.Fatalf("want ErrInvalidGenderLineup, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestArrangeTwoMenTwoWomenAlwaysMixedPairs(t *testing.T) {
	// The most level-balanced split would be man+man vs woman+woman
	// (6+1 vs 6+1), but only mixed pairs are allowed.
	four := [4]Candidate{cand(1, M, "TB+"), cand(2, M, "Y"), cand(3, F, "TB+"), cand(4, F, "Y")}
	l, err := Arrange(four, rules)
	if err != nil {
		t.Fatal(err)
	}
	if teammates(l, 1, 2) || teammates(l, 3, 4) {
		t.Fatalf("same-gender pair in 2M2F lineup: %+v", l)
	}
	if l.Diff != 0 {
		t.Fatalf("want diff 0 (TB+ man with Y woman on each side), got %d", l.Diff)
	}
}

func TestArrangePicksSmallestDiffAndReportsBalance(t *testing.T) {
	four := [4]Candidate{cand(1, M, "TB+"), cand(2, M, "TB"), cand(3, M, "TBY"), cand(4, M, "Y+")}
	l, err := Arrange(four, rules)
	if err != nil {
		t.Fatal(err)
	}
	// 6+2 vs 5+3
	if l.Diff != 0 || !l.Balanced || !teammates(l, 1, 4) {
		t.Fatalf("want 1+4 vs 2+3 with diff 0, got %+v", l)
	}

	lopsided := [4]Candidate{cand(1, M, "TB+"), cand(2, M, "TB+"), cand(3, M, "TB+"), cand(4, M, "Y")}
	l, err = Arrange(lopsided, rules)
	if err != nil {
		t.Fatal(err)
	}
	if l.Diff != 5 || l.Balanced {
		t.Fatalf("want diff 5 and unbalanced, got %+v", l)
	}
}

func TestArrangeAvoidsRecentTeammateWhenLevelsTie(t *testing.T) {
	four := [4]Candidate{cand(1, M, "TB"), cand(2, M, "TB"), cand(3, M, "TB"), cand(4, M, "TB")}
	four[0].RecentTeammates = []int64{2}
	four[0].RecentOpponents = []int64{3, 4}
	l, err := Arrange(four, rules)
	if err != nil {
		t.Fatal(err)
	}
	if teammates(l, 1, 2) {
		t.Fatalf("1 and 2 were paired again: %+v", l)
	}
}

func TestProposeErrors(t *testing.T) {
	if _, err := Propose([]Candidate{cand(1, M, "TB"), cand(2, M, "TB"), cand(3, M, "TB")}, rules); !errors.Is(err, domain.ErrNotEnoughPlayers) {
		t.Fatalf("want ErrNotEnoughPlayers, got %v", err)
	}
	// 3 men and 1 woman of equal level: gender rule can never pass.
	queue := []Candidate{cand(1, M, "TB"), cand(2, M, "TB"), cand(3, M, "TB"), cand(4, F, "TB")}
	if _, err := Propose(queue, rules); !errors.Is(err, domain.ErrNoValidLineup) {
		t.Fatalf("want ErrNoValidLineup, got %v", err)
	}
	// Valid genders but no split within the balance threshold.
	queue = []Candidate{cand(1, M, "TB+"), cand(2, M, "TB+"), cand(3, M, "TB+"), cand(4, M, "Y")}
	if _, err := Propose(queue, rules); !errors.Is(err, domain.ErrNoValidLineup) {
		t.Fatalf("want ErrNoValidLineup for unbalanced queue, got %v", err)
	}
}

func TestProposePriorityOrder(t *testing.T) {
	t.Run("fewer priority matches first", func(t *testing.T) {
		var queue []Candidate
		for i := int64(1); i <= 8; i++ {
			c := cand(i, M, "TB")
			if i <= 4 {
				c.PriorityCount = 1
			}
			queue = append(queue, c)
		}
		l, err := Propose(queue, rules)
		if err != nil {
			t.Fatal(err)
		}
		if got := ids(l); !sameIDs(got, []int64{5, 6, 7, 8}) {
			t.Fatalf("want players 5-8, got %v", got)
		}
	})
	t.Run("longer rest breaks count ties", func(t *testing.T) {
		var queue []Candidate
		for i := int64(1); i <= 8; i++ {
			c := cand(i, M, "TB")
			if i > 4 {
				c.ConsecutiveRest = 2
			}
			queue = append(queue, c)
		}
		l, err := Propose(queue, rules)
		if err != nil {
			t.Fatal(err)
		}
		if got := ids(l); !sameIDs(got, []int64{5, 6, 7, 8}) {
			t.Fatalf("want players 5-8, got %v", got)
		}
	})
	t.Run("earlier queue time breaks remaining ties", func(t *testing.T) {
		var queue []Candidate
		for i := int64(8); i >= 1; i-- {
			queue = append(queue, cand(i, M, "TB"))
		}
		l, err := Propose(queue, rules)
		if err != nil {
			t.Fatal(err)
		}
		if got := ids(l); !sameIDs(got, []int64{1, 2, 3, 4}) {
			t.Fatalf("want players 1-4, got %v", got)
		}
	})
}

func TestProposeWindowExpansionKeepsMostPlayersAtMin(t *testing.T) {
	// Three players at the minimum count cannot fill a court alone. The
	// window widens to min+1 and must keep all three.
	queue := []Candidate{cand(1, M, "TB"), cand(2, M, "TB"), cand(3, M, "TB")}
	for i := int64(4); i <= 8; i++ {
		c := cand(i, M, "TB")
		c.PriorityCount = 1
		queue = append(queue, c)
	}
	l, err := Propose(queue, rules)
	if err != nil {
		t.Fatal(err)
	}
	got := ids(l)
	if !sameIDs(got[:3], []int64{1, 2, 3}) {
		t.Fatalf("want players 1,2,3 plus one more, got %v", got)
	}
}

func TestProposeGenderRuleBeatsFairness(t *testing.T) {
	// The four lowest-count players are 3 men + 1 woman of equal level,
	// which is not allowed. The woman must wait even though she is due.
	queue := []Candidate{cand(1, M, "TB"), cand(2, M, "TB"), cand(3, M, "TB"), cand(4, F, "TB")}
	for i := int64(5); i <= 6; i++ {
		c := cand(i, M, "TB")
		c.PriorityCount = 1
		queue = append(queue, c)
	}
	l, err := Propose(queue, rules)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range ids(l) {
		if id == 4 {
			t.Fatalf("woman placed in an invalid 3-1 lineup: %+v", l)
		}
	}
	if got := ids(l); !sameIDs(got[:3], []int64{1, 2, 3}) {
		t.Fatalf("want the three men at min count kept, got %v", got)
	}
}

func TestProposeNeverReturnsInvalidLineup(t *testing.T) {
	levels := domain.Levels()
	byID := map[int64]Candidate{}
	var queue []Candidate
	for i := int64(1); i <= 14; i++ {
		g := M
		if i%3 == 0 {
			g = F
		}
		c := cand(i, g, levels[int(i)%len(levels)])
		c.PriorityCount = int(i) % 3
		byID[i] = c
		queue = append(queue, c)
	}
	l, err := Propose(queue, rules)
	if err != nil {
		t.Fatal(err)
	}
	four := [4]Candidate{byID[l.TeamA[0]], byID[l.TeamA[1]], byID[l.TeamB[0]], byID[l.TeamB[1]]}
	if !genderCompositionValid(four, rules) || !splitGenderValid(&four[0], &four[1], &four[2], &four[3]) {
		t.Fatalf("gender-invalid lineup returned: %+v", l)
	}
	if l.Diff > rules.BalanceThreshold {
		t.Fatalf("unbalanced lineup returned: %+v", l)
	}
}

func TestProposeUnder200msWith30Players(t *testing.T) {
	levels := domain.Levels()
	var queue []Candidate
	for i := int64(1); i <= 30; i++ {
		g := M
		if i%2 == 0 {
			g = F
		}
		queue = append(queue, cand(i, g, levels[int(i)%len(levels)]))
	}
	start := time.Now()
	if _, err := Propose(queue, rules); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 200*time.Millisecond {
		t.Fatalf("Propose took %v, want under 200ms", d)
	}
}
