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

// group puts the candidates at the given indexes in group g with phase.
func group(cands []Candidate, g int, phase domain.GroupPhase, idx ...int) {
	for _, i := range idx {
		cands[i].Group, cands[i].GroupSize, cands[i].GroupPhase = g, len(idx), phase
	}
}

const (
	together = domain.GroupTogether
	apart    = domain.GroupSplit
)

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

func TestArrangeKeepsGroupOnOneTeamWithinBalance(t *testing.T) {
	// 1+2 vs 3+4 has diff 2 (allowed), 1+3 vs 2+4 has diff 0.
	four := [4]Candidate{cand(1, M, "TB"), cand(2, M, "TB-"), cand(3, M, "TB-"), cand(4, M, "TBY")}
	four[0].Group, four[1].Group = 1, 1
	l, err := Arrange(four, rules)
	if err != nil {
		t.Fatal(err)
	}
	if !teammates(l, 1, 2) || l.GroupedTeams != 1 || l.Diff != 2 {
		t.Fatalf("want group 1+2 together with diff 2, got %+v", l)
	}

	// Pairing the group would exceed the balance threshold: balance wins.
	lopsided := [4]Candidate{cand(1, M, "TB+"), cand(2, M, "TB+"), cand(3, M, "Y+"), cand(4, M, "Y+")}
	lopsided[0].Group, lopsided[1].Group = 1, 1
	l, err = Arrange(lopsided, rules)
	if err != nil {
		t.Fatal(err)
	}
	if teammates(l, 1, 2) || !l.Balanced {
		t.Fatalf("group pairing broke the balance: %+v", l)
	}
}

func TestArrangeKeepsGroupTeamEvenIfTheyJustPartnered(t *testing.T) {
	four := [4]Candidate{cand(1, M, "TB"), cand(2, M, "TB"), cand(3, M, "TB"), cand(4, M, "TB")}
	four[0].Group, four[1].Group = 1, 1
	four[0].RecentTeammates, four[1].RecentTeammates = []int64{2}, []int64{1}
	l, err := Arrange(four, rules)
	if err != nil {
		t.Fatal(err)
	}
	if !teammates(l, 1, 2) {
		t.Fatalf("group was split: %+v", l)
	}
}

func TestArrangeRotatesPartnersInsideWholeGroup(t *testing.T) {
	four := [4]Candidate{cand(1, F, "TB"), cand(2, M, "TB"), cand(3, F, "TB"), cand(4, M, "TB")}
	for i := range four {
		four[i].Group = 1
	}
	four[0].RecentTeammates, four[1].RecentTeammates = []int64{2}, []int64{1}
	l, err := Arrange(four, rules)
	if err != nil {
		t.Fatal(err)
	}
	if teammates(l, 1, 2) {
		t.Fatalf("want new partners inside the group, got %+v", l)
	}
}

// thiGroup is Thi's group (1 Thi F, 2 friend F, 3 and 4 friends M) plus
// four outsiders (5, 6 F and 7, 8 M), all equal.
func thiGroup() []Candidate {
	cands := []Candidate{
		cand(1, F, "TB"), cand(2, F, "TB"), cand(3, M, "TB"), cand(4, M, "TB"),
		cand(5, F, "TB"), cand(6, F, "TB"), cand(7, M, "TB"), cand(8, M, "TB"),
	}
	group(cands, 1, together, 0, 1, 2, 3)
	return cands
}

func TestProposeWholeGroupWhenDue(t *testing.T) {
	l, err := Propose(thiGroup(), rules)
	if err != nil {
		t.Fatal(err)
	}
	if !sameIDs(ids(l), []int64{1, 2, 3, 4}) {
		t.Fatalf("want the whole group together, got %+v", l)
	}
}

func TestProposeGroupPairFacesOutsidersAfterWholeGroupMatch(t *testing.T) {
	cands := thiGroup()
	group(cands, 1, apart, 0, 1, 2, 3)
	// The group just played 1+3 vs 2+4.
	opp := map[int64][]int64{1: {2, 4}, 3: {2, 4}, 2: {1, 3}, 4: {1, 3}}
	mate := map[int64][]int64{1: {3}, 3: {1}, 2: {4}, 4: {2}}
	for i := 0; i < 4; i++ {
		cands[i].PriorityCount = 1
		cands[i].RecentOpponents = opp[cands[i].ID]
		cands[i].RecentTeammates = mate[cands[i].ID]
	}
	for i := 4; i < 8; i++ {
		cands[i].PriorityCount = 1
		cands[i].ConsecutiveRest = 1
	}
	l, err := ProposeFor(cands, rules, Courts{Open: 2, OtherFree: 1})
	if err != nil {
		t.Fatal(err)
	}
	inGroup := 0
	for _, id := range ids(l) {
		if id <= 4 {
			inGroup++
		}
	}
	if inGroup != 2 {
		t.Fatalf("want one group pair against outsiders, got %+v", l)
	}
	var pair [2]int64
	switch {
	case l.TeamA[0] <= 4 && l.TeamA[1] <= 4:
		pair = l.TeamA
	case l.TeamB[0] <= 4 && l.TeamB[1] <= 4:
		pair = l.TeamB
	default:
		t.Fatalf("want the group pair on one team, got %+v", l)
	}
	if sameTeam(pair, 1, 3) || sameTeam(pair, 2, 4) {
		t.Fatalf("want a new partner inside the group, got %+v", l)
	}
}

func TestProposePrefersGroupOnSameCourt(t *testing.T) {
	var cands []Candidate
	for id := int64(1); id <= 8; id++ {
		cands = append(cands, cand(id, M, "TB"))
	}
	// 2 and 7 came together; 7 is late in the queue but ties on fairness.
	cands[1].Group, cands[6].Group = 1, 1
	l, err := Propose(cands, rules)
	if err != nil {
		t.Fatal(err)
	}
	if !teammates(l, 2, 7) {
		t.Fatalf("want 2 and 7 on one team, got %+v", l)
	}
}

func TestProposeGroupEntersTogether(t *testing.T) {
	var cands []Candidate
	for id := int64(1); id <= 6; id++ {
		cands = append(cands, cand(id, M, "TB"))
	}
	// 1 came with 5, who has one more match: they still go in together.
	cands[4].PriorityCount = 1
	group(cands, 1, together, 0, 4)
	l, err := ProposeFor(cands, rules, Courts{Open: 2})
	if err != nil {
		t.Fatal(err)
	}
	got := ids(l)
	if contains(got, 1) != contains(got, 5) {
		t.Fatalf("group split: %+v", l)
	}

	// With one court open the group is preferred, never waited for:
	// fairness keeps 5 out.
	l, err = Propose(cands, rules)
	if err != nil {
		t.Fatal(err)
	}
	if !sameIDs(ids(l), []int64{1, 2, 3, 4}) {
		t.Fatalf("one court: want the lowest counts, got %+v", l)
	}
}

func TestGroupQueueWaitsForMateOnCourt(t *testing.T) {
	waiting := []Candidate{cand(1, M, "TB"), cand(2, F, "TB"), cand(3, M, "TB")}
	group(waiting, 1, together, 0)
	group(waiting, 2, together, 1)
	waiting[0].GroupSize, waiting[1].GroupSize = 2, 2 // their mates are elsewhere
	q := GroupQueue(waiting, map[int]bool{1: true}, map[int]bool{2: true})
	if len(q) != 2 || q[0].ID != 2 || !q[0].MateHeld || q[1].MateHeld {
		t.Fatalf("got %+v", q)
	}

	// Apart, nobody waits or is held for a mate.
	waiting[0].GroupPhase, waiting[1].GroupPhase = apart, apart
	q = GroupQueue(waiting, map[int]bool{1: true}, map[int]bool{2: true})
	if len(q) != 3 || q[1].MateHeld {
		t.Fatalf("split phase: got %+v", q)
	}
}

// A group of four due to play together takes one court whole, even with
// another court free.
func TestFourTogetherPhasePlaysWhole(t *testing.T) {
	for _, free := range []int{0, 1} {
		l, err := ProposeFor(thiGroup(), rules, Courts{Open: 2, OtherFree: free})
		if err != nil {
			t.Fatal(err)
		}
		if n := countIn(ids(l), 1, 4); n != 4 {
			t.Fatalf("other free courts %d: want the whole group, got %+v", free, l)
		}
	}
}

// Split, a group of four never plays whole: one pair faces outsiders.
func TestFourSplitPhaseSendsOnePair(t *testing.T) {
	cands := thiGroup()
	group(cands, 1, apart, 0, 1, 2, 3)
	l, err := ProposeFor(cands, rules, Courts{Open: 2})
	if err != nil {
		t.Fatal(err)
	}
	if n := countIn(ids(l), 1, 4); n != 2 {
		t.Fatalf("want one pair of the group, got %+v", l)
	}
	pairTeamed := (l.TeamA[0] <= 4 && l.TeamA[1] <= 4) || (l.TeamB[0] <= 4 && l.TeamB[1] <= 4)
	if !pairTeamed {
		t.Fatalf("want the pair on one team, got %+v", l)
	}
}

func TestPairTogetherPhaseTeamsUp(t *testing.T) {
	var cands []Candidate
	for id := int64(1); id <= 8; id++ {
		cands = append(cands, cand(id, M, "TB"))
	}
	group(cands, 1, together, 1, 6)
	l, err := Propose(cands, rules)
	if err != nil {
		t.Fatal(err)
	}
	if !teammates(l, 2, 7) {
		t.Fatalf("want 2 and 7 on one team, got %+v", l)
	}
}

func TestPairSplitPhaseSeparates(t *testing.T) {
	four := [4]Candidate{cand(1, M, "TB"), cand(2, M, "TB"), cand(3, M, "TB"), cand(4, M, "TB")}
	group(four[:], 1, apart, 0, 1)
	l, err := Arrange(four, rules)
	if err != nil {
		t.Fatal(err)
	}
	if teammates(l, 1, 2) {
		t.Fatalf("a split pair must not team up, got %+v", l)
	}
}

func TestThreeGroupSendsOnePair(t *testing.T) {
	var cands []Candidate
	for id := int64(1); id <= 8; id++ {
		cands = append(cands, cand(id, M, "TB"))
	}
	group(cands, 1, together, 0, 1, 2)
	l, err := Propose(cands, rules)
	if err != nil {
		t.Fatal(err)
	}
	got := ids(l)
	if countIn(got, 1, 3) != 2 {
		t.Fatalf("want exactly two of the group, got %+v", l)
	}
	if !(teammates(l, 1, 2) || teammates(l, 1, 3) || teammates(l, 2, 3)) {
		t.Fatalf("want the two on one team, got %+v", l)
	}
}

func TestProposeFreesGroupThatCannotBeSeatedWhole(t *testing.T) {
	// Two men and a woman came together, the others are men: the group
	// needs a fourth woman (3-1 at equal levels is invalid), so the men
	// play with the others rather than everyone waiting.
	cands := []Candidate{cand(1, M, "Y"), cand(2, M, "Y"), cand(3, F, "Y"), cand(4, M, "Y"), cand(5, M, "Y")}
	group(cands, 1, together, 0, 1, 2)
	l, err := ProposeFor(cands, rules, Courts{Open: 2})
	if err != nil {
		t.Fatalf("group of three blocked every lineup: %v", err)
	}
	if !sameIDs(ids(l), []int64{1, 2, 4, 5}) {
		t.Fatalf("got %+v", l)
	}
}

func countIn(got []int64, lo, hi int64) int {
	n := 0
	for _, id := range got {
		if id >= lo && id <= hi {
			n++
		}
	}
	return n
}

func TestProposeAvoidsThreeOneWhenMixedLineupExists(t *testing.T) {
	// 3 men + woman 4 is a valid 3-1 lineup and the men have rested
	// longer, but 2 men + 2 women also fits.
	cands := []Candidate{
		cand(1, M, "TB"), cand(2, M, "TB"), cand(3, M, "TB"),
		cand(4, F, "TBY"), cand(5, F, "TBY"),
	}
	for i := 0; i < 3; i++ {
		cands[i].ConsecutiveRest = 1
	}
	l, err := Propose(cands, rules)
	if err != nil {
		t.Fatal(err)
	}
	got := ids(l)
	if !(contains(got, 4) && contains(got, 5)) {
		t.Fatalf("want a 2-2 lineup, got %+v", l)
	}

	// Once woman 4 has been skipped, rest wins again.
	cands[3].ConsecutiveRest = 1
	l, err = Propose(cands, rules)
	if err != nil {
		t.Fatal(err)
	}
	if !sameIDs(ids(l), []int64{1, 2, 3, 4}) {
		t.Fatalf("want the 3-1 lineup after woman 4 was skipped, got %+v", l)
	}

	// With only one woman waiting at the lowest count, the window widens
	// to woman 5 one match ahead rather than propose 3-1 (2026-11-16).
	cands[3].ConsecutiveRest = 0
	cands[4].PriorityCount = 1
	l, err = Propose(cands, rules)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(l); !(contains(got, 4) && contains(got, 5)) {
		t.Fatalf("want a 2-2 lineup with woman 5 from the next tier, got %+v", l)
	}
}

func threeMenOneWomanPlusLaterWoman() []Candidate {
	cands := []Candidate{
		cand(1, M, "TB"), cand(2, M, "TB"), cand(3, M, "TB"),
		cand(4, F, "TBY"), cand(5, F, "TBY"),
	}
	cands[4].PriorityCount = 1
	return cands
}

func TestProposeWidensWindowToAvoidThreeOne(t *testing.T) {
	l, err := Propose(threeMenOneWomanPlusLaterWoman(), rules)
	if err != nil {
		t.Fatal(err)
	}
	got := ids(l)
	if !(contains(got, 4) && contains(got, 5)) || countIn(got, 1, 3) != 2 {
		t.Fatalf("want two men with women 4 and 5, got %+v", l)
	}
}

// A woman who already sat out keeps her 3-1 rather than wait again: on one
// court, making her wait leaves sessions two matches apart.
func TestProposeLoneRestedWomanStillPlaysThreeOne(t *testing.T) {
	cands := threeMenOneWomanPlusLaterWoman()
	cands[3].ConsecutiveRest = 1
	l, err := Propose(cands, rules)
	if err != nil {
		t.Fatal(err)
	}
	if !sameIDs(ids(l), []int64{1, 2, 3, 4}) {
		t.Fatalf("want the 3-1 lineup for the rested woman, got %+v", l)
	}
}

func TestProposeNoWiderTierKeepsThreeOne(t *testing.T) {
	cands := threeMenOneWomanPlusLaterWoman()[:4]
	l, err := Propose(cands, rules)
	if err != nil {
		t.Fatal(err)
	}
	if !sameIDs(ids(l), []int64{1, 2, 3, 4}) {
		t.Fatalf("want the only lineup, got %+v", l)
	}
}

// A woman who can only join a 3-1 right now waits one turn instead of
// pushing out an all-men lineup (seen live: the other women were on the
// first court's proposal).
func TestProposeLoneWomanWaitsForMixedLineup(t *testing.T) {
	cands := []Candidate{
		cand(3, M, "TB"), cand(4, M, "TB-"), cand(5, M, "TB"), cand(6, F, "Y+"),
		cand(7, M, "TB"), cand(8, M, "TB"), cand(11, M, "TB"), cand(13, M, "TB-"),
	}
	l, err := Propose(cands, rules)
	if err != nil {
		t.Fatal(err)
	}
	if contains(ids(l), 6) {
		t.Fatalf("want an all-men lineup, got %+v", l)
	}
}

func TestProposeGroupBeatsConsecutiveRest(t *testing.T) {
	var cands []Candidate
	for id := int64(1); id <= 6; id++ {
		cands = append(cands, cand(id, M, "TB"))
	}
	// 1-4 rested longer, but 1 came with 5, who did not.
	for i := 0; i < 4; i++ {
		cands[i].ConsecutiveRest = 1
	}
	cands[0].Group, cands[4].Group = 1, 1
	l, err := Propose(cands, rules)
	if err != nil {
		t.Fatal(err)
	}
	if !teammates(l, 1, 5) {
		t.Fatalf("want 1 and 5 together, got %+v", l)
	}
}

// eightAfterTwoMatches is one court of eight equal players after 1-4 and
// then 5-8 played: everyone has one match, 1-4 rested once.
func eightAfterTwoMatches() []Candidate {
	var cands []Candidate
	for id := int64(1); id <= 8; id++ {
		c := cand(id, M, "TB")
		c.PriorityCount = 1
		base := int64(1)
		if id > 4 {
			base = 5
		}
		for o := base; o < base+4; o++ {
			if o != id {
				c.RecentOpponents = append(c.RecentOpponents, o)
			}
		}
		if id <= 4 {
			c.ConsecutiveRest = 1
		}
		cands = append(cands, c)
	}
	return cands
}

func TestProposeMixesFoursomesAboveRest(t *testing.T) {
	l, err := Propose(eightAfterTwoMatches(), rules)
	if err != nil {
		t.Fatal(err)
	}
	first := 0
	for _, id := range ids(l) {
		if id <= 4 {
			first++
		}
	}
	if first != 2 {
		t.Fatalf("want two from each foursome, got %+v", l)
	}
}

func TestProposeOverdueBeatsMixing(t *testing.T) {
	cands := eightAfterTwoMatches()
	for i := 0; i < 4; i++ {
		cands[i].ConsecutiveRest = overdueRest
	}
	l, err := Propose(cands, rules)
	if err != nil {
		t.Fatal(err)
	}
	if !sameIDs(ids(l), []int64{1, 2, 3, 4}) {
		t.Fatalf("players who sat out %d matches should play, got %+v", overdueRest, l)
	}
}

func TestArrangeRotatesPartnersWithinBalance(t *testing.T) {
	// 1+4 vs 2+3 is perfectly even but 1 and 4 just partnered; 1+2 vs 3+4
	// is off by 2, still within the threshold.
	four := [4]Candidate{cand(1, M, "TB"), cand(2, M, "TB-"), cand(3, M, "TB-"), cand(4, M, "TBY")}
	four[0].RecentTeammates = []int64{4}
	four[3].RecentTeammates = []int64{1}
	l, err := Arrange(four, rules)
	if err != nil {
		t.Fatal(err)
	}
	if teammates(l, 1, 4) || l.Repeats != 0 || !l.Balanced {
		t.Fatalf("want new partners within balance, got %+v", l)
	}
}
