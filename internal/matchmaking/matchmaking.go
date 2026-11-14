// Package matchmaking picks four players from the queue and splits them
// into two teams. It is pure: no storage, no clock.
package matchmaking

import (
	"math"
	"math/bits"
	"sort"
	"time"

	"arrange-matches-bwd/internal/domain"
)

// Candidate is a waiting player eligible for a proposal.
type Candidate struct {
	ID              int64
	Gender          domain.Gender
	Level           domain.Level
	PriorityCount   int
	ConsecutiveRest int
	QueuedAt        time.Time
	// Teammates and opponents from the player's latest matches
	// (already limited to the configured lookback).
	RecentTeammates []int64
	RecentOpponents []int64
	// Group marks players who came together (0: none). A group enters
	// courts together: see GroupQueue and ProposeFor.
	Group int
	// MateHeld marks a player whose group mate is already in another
	// court's proposal: the group was split across two free courts and
	// this player should join the next proposal.
	MateHeld bool
}

// Lineup is a proposed match.
type Lineup struct {
	TeamA [2]int64
	TeamB [2]int64
	// Diff is the absolute difference of the team level sums.
	Diff     int
	Balanced bool
	// Repeats counts teammate and opponent pairings seen recently.
	Repeats int
	// GroupedTeams counts teams whose two players share a group.
	GroupedTeams int
}

// SortByPriority orders candidates by the fairness rule: fewer priority
// matches, then longer consecutive rest, then earlier queue time.
func SortByPriority(cands []Candidate) {
	sort.SliceStable(cands, func(i, j int) bool {
		a, b := &cands[i], &cands[j]
		if a.PriorityCount != b.PriorityCount {
			return a.PriorityCount < b.PriorityCount
		}
		if a.ConsecutiveRest != b.ConsecutiveRest {
			return a.ConsecutiveRest > b.ConsecutiveRest
		}
		if !a.QueuedAt.Equal(b.QueuedAt) {
			return a.QueuedAt.Before(b.QueuedAt)
		}
		return a.ID < b.ID
	})
}

const (
	// maxCandidates caps how many players (in priority order) compete for
	// one proposal, which bounds the search on a long queue.
	maxCandidates = 16
	// The lookahead plays lookaheadDepth further matches among the top
	// lookaheadCandidates players to see who would be left stranded.
	lookaheadCandidates = 8
	lookaheadDepth      = 2
	// noLineup scores a looked-ahead match that could not be formed.
	noLineup = 1 << 30
	// overdueRest is the sit-out streak that outranks mixing players up.
	overdueRest = 2
)

// Propose selects the best valid lineup from the queue.
//
// The candidate window starts with the players at the lowest priority count
// and widens one count at a time only while no valid lineup exists. Inside a
// window, lineups are ranked by, in order:
//  1. priority counts (as many players as possible at the lowest counts),
//  2. what the choice leaves behind: the remaining lowest-count players
//     should still split into valid lineups, and the next matches should
//     reach the lowest counts possible, so a pick does not strand players
//     in a group the gender or balance rules cannot seat together,
//  3. gender composition: 3-1 lineups only when no 4-0 or 2-2 ties,
//     unless the odd player has already been skipped once,
//  4. how hard the picked players are to seat (those with the fewest
//     valid lineups go first, while a lineup for them is available);
//     above groups, or 1-court sessions break the one-match gap,
//  5. players who came together: more same-group teams (a court where
//     group mates face each other counts only when none of them faced a
//     group mate recently, and ranks below no grouping otherwise), then
//     fewer same-group teams who just partnered, so partners rotate
//     inside the group,
//  6. players overdue for a match (sat out overdueRest matches or more),
//  7. mixing: fewer pairs who shared a court in their recent matches, so
//     a full court does not keep splitting into the same two foursomes,
//  8. consecutive rest (longer rest first),
//  9. team level difference (smaller first),
//  10. repeated teammates/opponents (fewer first),
//  11. queue order.
//
// Gender rules and the balance threshold are hard constraints. It returns
// domain.ErrNotEnoughPlayers or domain.ErrNoValidLineup when nothing fits.
func Propose(cands []Candidate, rules domain.MatchRules) (Lineup, error) {
	return ProposeFor(cands, rules, Courts{Open: 1})
}

// Courts describes the session around the court being proposed for.
type Courts struct {
	// Open is the number of courts in play this session.
	Open int
	// OtherFree counts the other courts that are empty and still
	// without a proposal.
	OtherFree int
}

// GroupsWait reports whether groups enter courts together and wait for
// each other. On a single court that would leave outsiders filling in
// for groups (the simulation shows frequent two-match gaps), so there a
// group is only preferred, never waited for.
func GroupsWait(openCourts int) bool {
	return openCourts >= 2
}

// ProposeFor is Propose for one court of a session.
//
// When GroupsWait, groups enter together: a lineup with some of a group's waiting players
// must have all of them, unless another court is free, in which case the
// group may send one pair here and the rest (marked MateHeld) to the next
// proposal, which takes them first when it can. A split is kept only if
// this proposal and the next one together play nobody further ahead than
// without the split. A group that no valid lineup can seat whole is not
// held to this, so nobody waits forever.
func ProposeFor(cands []Candidate, rules domain.MatchRules, courts Courts) (Lineup, error) {
	if len(cands) < 4 {
		return Lineup{}, domain.ErrNotEnoughPlayers
	}
	sorted := make([]Candidate, len(cands))
	copy(sorted, cands)
	SortByPriority(sorted)

	capped := sorted
	if len(capped) > maxCandidates {
		capped = capped[:maxCandidates]
	}
	split := GroupsWait(courts.Open) && courts.OtherFree > 0
	lineup, err := proposeGroups(sorted, capped, rules, courts, split)
	if err != nil || !split || !leavesMates(lineup, sorted) {
		return lineup, err
	}
	// Splitting a group binds the next court to its held mates, and the
	// players who can join them may already be ahead. Keep the split only
	// if both courts together play no one further ahead than without it.
	whole, err := proposeGroups(sorted, capped, rules, courts, false)
	if err != nil {
		return lineup, nil
	}
	if compareInts(jointCounts(whole, sorted, rules, courts), jointCounts(lineup, sorted, rules, courts)) < 0 {
		return whole, nil
	}
	return lineup, nil
}

func proposeGroups(sorted, capped []Candidate, rules domain.MatchRules, courts Courts, split bool) (Lineup, error) {
	rule := func(list []Candidate, requireHeld bool) *groupRule {
		if !GroupsWait(courts.Open) {
			return nil
		}
		g := newGroupRule(list, rules, split, requireHeld)
		if g.strands(list, rules) {
			return nil
		}
		return g
	}
	try := func(requireHeld bool) (Lineup, bool) {
		if lineup, ok := proposeFrom(capped, rules, true, rule(capped, requireHeld)); ok {
			return lineup, true
		}
		if len(sorted) > len(capped) {
			return proposeFrom(sorted, rules, false, rule(sorted, requireHeld))
		}
		return Lineup{}, false
	}
	for _, c := range sorted {
		if c.MateHeld {
			if lineup, ok := try(true); ok {
				return lineup, nil
			}
			break
		}
	}
	if lineup, ok := try(false); ok {
		return lineup, nil
	}
	return Lineup{}, domain.ErrNoValidLineup
}

func lineupIDs(l Lineup) [4]int64 {
	return [4]int64{l.TeamA[0], l.TeamA[1], l.TeamB[0], l.TeamB[1]}
}

// leavesMates reports whether the lineup takes part of a group and leaves
// a group mate among the candidates.
func leavesMates(l Lineup, cands []Candidate) bool {
	in, groups := lineupSets(l, cands)
	for _, c := range cands {
		if !in[c.ID] && c.Group != 0 && groups[c.Group] {
			return true
		}
	}
	return false
}

func lineupSets(l Lineup, cands []Candidate) (in map[int64]bool, groups map[int]bool) {
	in, groups = map[int64]bool{}, map[int]bool{}
	for _, id := range lineupIDs(l) {
		in[id] = true
	}
	for _, c := range cands {
		if in[c.ID] && c.Group != 0 {
			groups[c.Group] = true
		}
	}
	return in, groups
}

// jointCounts lists, lowest first, the priority counts of the lineup and
// of the proposal that would follow it on the next free court.
func jointCounts(l Lineup, cands []Candidate, rules domain.MatchRules, courts Courts) []int {
	in, groups := lineupSets(l, cands)
	var counts []int
	var rest []Candidate
	for _, c := range cands {
		if in[c.ID] {
			counts = append(counts, c.PriorityCount)
			continue
		}
		c.MateHeld = c.MateHeld || (c.Group != 0 && groups[c.Group])
		rest = append(rest, c)
	}
	next, err := ProposeFor(rest, rules, Courts{Open: courts.Open, OtherFree: courts.OtherFree - 1})
	if err != nil {
		counts = append(counts, math.MaxInt, math.MaxInt, math.MaxInt, math.MaxInt)
	} else {
		ids := lineupIDs(next)
		for _, c := range rest {
			for _, id := range ids {
				if c.ID == id {
					counts = append(counts, c.PriorityCount)
				}
			}
		}
	}
	sort.Ints(counts)
	return counts
}

// GroupQueue prepares the waiting players for a proposal. Players whose
// group mate is on court wait for them and are left out; players whose
// group mate is held by another court's proposal are marked MateHeld.
// playing and held list the group numbers with a member in each state.
func GroupQueue(waiting []Candidate, playing, held map[int]bool) []Candidate {
	out := make([]Candidate, 0, len(waiting))
	for _, c := range waiting {
		if c.Group != 0 && playing[c.Group] {
			continue
		}
		c.MateHeld = c.Group != 0 && held[c.Group]
		out = append(out, c)
	}
	return out
}

// groupRule keeps groups together in a proposal.
type groupRule struct {
	// avail counts each bound group's players among the candidates;
	// groups that cannot be seated whole are not bound.
	avail map[int]int
	// pending marks groups already split across courts: the rest go
	// together, no further split.
	pending map[int]bool
	split   bool
	// held lists the MateHeld players a lineup must contain.
	held []int64
}

func newGroupRule(cands []Candidate, rules domain.MatchRules, split, requireHeld bool) *groupRule {
	g := &groupRule{avail: map[int]int{}, pending: map[int]bool{}, split: split}
	members := map[int][]int{}
	for i, c := range cands {
		if c.Group == 0 {
			continue
		}
		members[c.Group] = append(members[c.Group], i)
		if c.MateHeld {
			g.pending[c.Group] = true
			if requireHeld {
				g.held = append(g.held, c.ID)
			}
		}
	}
	for grp, idx := range members {
		if seatableWhole(cands, idx, rules) {
			g.avail[grp] = len(idx)
		}
	}
	return g
}

// seatableWhole reports whether some valid, balanced lineup holds all the
// given players (at most four) together with other candidates.
func seatableWhole(cands []Candidate, idx []int, rules domain.MatchRules) bool {
	if len(idx) > 4 {
		return false
	}
	in := map[int]bool{}
	for _, i := range idx {
		in[i] = true
	}
	var others []int
	for i := range cands {
		if !in[i] {
			others = append(others, i)
		}
	}
	need := 4 - len(idx)
	var four [4]Candidate
	for k, i := range idx {
		four[k] = cands[i]
	}
	var fill func(start, k int) bool
	fill = func(start, k int) bool {
		if k == 4 {
			l, err := Arrange(four, rules)
			return err == nil && l.Balanced
		}
		for j := start; j < len(others); j++ {
			four[k] = cands[others[j]]
			if fill(j+1, k+1) {
				return true
			}
		}
		return false
	}
	return need <= len(others) && fill(0, len(idx))
}

// strands reports a lowest-count player whom keeping groups together
// leaves without any lineup, for example a lone woman when the other
// women came together. Groups then yield for this proposal.
func (g *groupRule) strands(sorted []Candidate, rules domain.MatchRules) bool {
	end := windowEnd(sorted, 0)
	for i := 0; i < end; i++ {
		if !seatableUnder(sorted, i, rules, g) && seatableUnder(sorted, i, rules, nil) {
			return true
		}
	}
	return false
}

func seatableUnder(cands []Candidate, i int, rules domain.MatchRules, g *groupRule) bool {
	n := len(cands)
	four := [4]Candidate{cands[i]}
	for a := 0; a < n; a++ {
		for b := a + 1; b < n; b++ {
			for c := b + 1; c < n; c++ {
				if a == i || b == i || c == i {
					continue
				}
				four[1], four[2], four[3] = cands[a], cands[b], cands[c]
				if !g.allows(&four) {
					continue
				}
				if l, err := Arrange(four, rules); err == nil && l.Balanced {
					return true
				}
			}
		}
	}
	return false
}

func (g *groupRule) allows(four *[4]Candidate) bool {
	if g == nil {
		return true
	}
	for _, id := range g.held {
		found := false
		for i := range four {
			if four[i].ID == id {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	var picked map[int]int
	for i := range four {
		if grp := four[i].Group; grp != 0 && g.avail[grp] > 0 {
			if picked == nil {
				picked = map[int]int{}
			}
			picked[grp]++
		}
	}
	for grp, n := range picked {
		if n == g.avail[grp] {
			continue
		}
		if g.pending[grp] || !g.split || n != 2 {
			return false
		}
	}
	return true
}

// proposeFrom widens the candidate window tier by tier until a lineup is
// found. Lookahead requires len(sorted) <= maxCandidates.
func proposeFrom(sorted []Candidate, rules domain.MatchRules, useLookahead bool, groups *groupRule) (Lineup, bool) {
	for end := windowEnd(sorted, 0); ; end = windowEnd(sorted, end) {
		if end >= 4 {
			if lineup, ok := bestInWindow(sorted, end, rules, useLookahead, groups); ok {
				return lineup, true
			}
		}
		if end == len(sorted) {
			return Lineup{}, false
		}
	}
}

// windowEnd extends the window past start to include every candidate
// sharing the next priority count.
func windowEnd(sorted []Candidate, start int) int {
	end := start
	for end < len(sorted) && sorted[end].PriorityCount == sorted[start].PriorityCount {
		end++
	}
	return end
}

type option struct {
	lineup Lineup
	order  [4]int
	counts [4]int
	rests  [4]int
	// stranded counts lowest-tier players this pick leaves without a
	// lineup among themselves.
	stranded  int
	future    []int
	oddGender bool
	// grouped scores players who came together; see groupScore.
	grouped int
	// groupStale counts same-group teams who partnered recently.
	groupStale int
	// overdue counts picked players who sat out overdueRest matches or more.
	overdue int
	// courtMates counts pairs among the four who shared a recent court.
	courtMates int
	// scarcity lists, hardest to seat first, how many valid lineups each
	// picked player could join.
	scarcity [4]int
}

func (o *option) better(b *option) bool {
	if o.stranded != b.stranded {
		return o.stranded < b.stranded
	}
	if c := compareInts(o.future, b.future); c != 0 {
		return c < 0
	}
	if o.oddGender != b.oddGender {
		return !o.oddGender
	}
	if c := compareInts(o.scarcity[:], b.scarcity[:]); c != 0 {
		return c < 0
	}
	if o.grouped != b.grouped {
		return o.grouped > b.grouped
	}
	if o.groupStale != b.groupStale {
		return o.groupStale < b.groupStale
	}
	if o.overdue != b.overdue {
		return o.overdue > b.overdue
	}
	if o.courtMates != b.courtMates {
		return o.courtMates < b.courtMates
	}
	for i := range o.rests {
		if o.rests[i] != b.rests[i] {
			return o.rests[i] > b.rests[i]
		}
	}
	if o.lineup.Diff != b.lineup.Diff {
		return o.lineup.Diff < b.lineup.Diff
	}
	if o.lineup.Repeats != b.lineup.Repeats {
		return o.lineup.Repeats < b.lineup.Repeats
	}
	return compareInts(o.order[:], b.order[:]) < 0
}

func compareInts(a, b []int) int {
	for i := range a {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

func bestInWindow(sorted []Candidate, end int, rules domain.MatchRules, useLookahead bool, groups *groupRule) (Lineup, bool) {
	w := sorted[:end]

	// Keep every valid lineup tied on the best priority counts.
	var ties []option
	n := len(w)
	for a := 0; a < n-3; a++ {
		for b := a + 1; b < n-2; b++ {
			for c := b + 1; c < n-1; c++ {
				for d := c + 1; d < n; d++ {
					o := option{order: [4]int{a, b, c, d}}
					for i, idx := range o.order {
						o.counts[i] = w[idx].PriorityCount
						o.rests[i] = w[idx].ConsecutiveRest
					}
					if len(ties) > 0 && compareInts(o.counts[:], ties[0].counts[:]) > 0 {
						continue
					}
					four := [4]Candidate{w[a], w[b], w[c], w[d]}
					if !groups.allows(&four) {
						continue
					}
					lineup, err := Arrange(four, rules)
					if err != nil || !lineup.Balanced {
						continue
					}
					// Only a valid lineup may drop the ties it beats.
					if len(ties) > 0 && compareInts(o.counts[:], ties[0].counts[:]) < 0 {
						ties = ties[:0]
					}
					o.lineup = lineup
					o.oddGender = oddOneOutFresh(four)
					o.grouped = groupScore(four, &lineup)
					o.groupStale = staleGroupTeams(four, &lineup)
					o.courtMates = courtMatePairs(four)
					for i := range four {
						if four[i].ConsecutiveRest >= overdueRest {
							o.overdue++
						}
					}
					ties = append(ties, o)
				}
			}
		}
	}
	if len(ties) == 0 {
		return Lineup{}, false
	}

	if useLookahead && len(ties) > 1 {
		la := newLookahead(sorted, rules)
		for i := range ties {
			ties[i].stranded = la.stranded(ties[i].order)
			ties[i].future = la.score(ties[i].order)
			for j, idx := range ties[i].order {
				ties[i].scarcity[j] = la.options[idx]
			}
			sort.Ints(ties[i].scarcity[:])
		}
	}
	best := &ties[0]
	for i := 1; i < len(ties); i++ {
		if ties[i].better(best) {
			best = &ties[i]
		}
	}
	return best.lineup, true
}

// lookahead scores a pick by simulating the matches that would follow it.
// It works on slots (lightweight views of the candidates) and a precomputed
// table of which foursomes form a valid, balanced lineup.
type lookahead struct {
	slots []slot
	valid [1 << maxCandidates]bool
	// options counts the valid lineups each candidate can join.
	options [maxCandidates]int
	// tier marks the candidates at the lowest priority count.
	tier uint16
	// covered memoizes cover(mask)+1; 0 means not computed yet.
	covered [1 << maxCandidates]int8
}

type slot struct {
	bit   uint16 // identifies the candidate in the valid table
	count int
	rest  int
	pos   int // queue position; later is larger
}

func (s slot) before(o slot) bool {
	if s.count != o.count {
		return s.count < o.count
	}
	if s.rest != o.rest {
		return s.rest > o.rest
	}
	return s.pos < o.pos
}

func newLookahead(sorted []Candidate, rules domain.MatchRules) *lookahead {
	la := &lookahead{slots: make([]slot, len(sorted))}
	for i := range sorted {
		la.slots[i] = slot{bit: 1 << i, count: sorted[i].PriorityCount, rest: sorted[i].ConsecutiveRest, pos: i}
		if sorted[i].PriorityCount == sorted[0].PriorityCount {
			la.tier |= 1 << i
		}
	}
	n := len(sorted)
	for a := 0; a < n-3; a++ {
		for b := a + 1; b < n-2; b++ {
			for c := b + 1; c < n-1; c++ {
				for d := c + 1; d < n; d++ {
					lineup, err := Arrange([4]Candidate{sorted[a], sorted[b], sorted[c], sorted[d]}, rules)
					if err == nil && lineup.Balanced {
						la.valid[1<<a|1<<b|1<<c|1<<d] = true
						la.options[a]++
						la.options[b]++
						la.options[c]++
						la.options[d]++
					}
				}
			}
		}
	}
	return la
}

// stranded counts the lowest-tier players who, once the picked ones are on
// court, cannot all be seated in lineups made only of that tier. Leftovers
// that are unavoidable (fewer than four remain) count the same for every
// pick, so only avoidable stranding separates the options.
func (la *lookahead) stranded(picked [4]int) int {
	rest := la.tier
	for _, i := range picked {
		rest &^= 1 << i
	}
	return bits.OnesCount16(rest) - la.cover(rest)
}

// cover returns the most players of mask that disjoint valid lineups
// within mask can seat.
func (la *lookahead) cover(mask uint16) int {
	n := bits.OnesCount16(mask)
	if n < 4 {
		return 0
	}
	if v := la.covered[mask]; v != 0 {
		return int(v) - 1
	}
	low := mask & -mask
	rest := mask ^ low
	// Either the lowest player stays out, or joins one of the lineups.
	best := la.cover(rest)
	limit := n / 4 * 4
search:
	for b := rest; b != 0 && best < limit; b &= b - 1 {
		bb := b & -b
		for c := b & (b - 1); c != 0; c &= c - 1 {
			cb := c & -c
			for d := c & (c - 1); d != 0; d &= d - 1 {
				lineup := low | bb | cb | d&-d
				if !la.valid[lineup] {
					continue
				}
				if got := 4 + la.cover(mask^lineup); got > best {
					best = got
					if best >= limit {
						break search
					}
				}
			}
		}
	}
	la.covered[mask] = int8(best + 1)
	return best
}

// score returns the priority counts of the next lookaheadDepth matches that
// remain possible after the picked players go on court; lower is better.
func (la *lookahead) score(picked [4]int) []int {
	score, _ := la.future(play(la.slots, picked, 1), lookaheadDepth)
	return score
}

// play returns the queue once the picked slots have played: they come back
// with one more match, behind everyone already waiting.
func play(slots []slot, picked [4]int, round int) []slot {
	next := make([]slot, 0, len(slots))
	for i, s := range slots {
		if i != picked[0] && i != picked[1] && i != picked[2] && i != picked[3] {
			next = append(next, s)
		}
	}
	for _, i := range picked {
		s := slots[i]
		s.count++
		s.rest = 0
		s.pos += round * maxCandidates
		j := len(next)
		next = append(next, s)
		for ; j > 0 && s.before(next[j-1]); j-- {
			next[j] = next[j-1]
		}
		next[j] = s
	}
	return next
}

// future scores the best chain of depth matches from the queue. perfect
// reports that no other chain could score lower, so callers can stop.
func (la *lookahead) future(slots []slot, depth int) (score []int, perfect bool) {
	if depth == 0 {
		return nil, true
	}
	top := slots
	if len(top) > lookaheadCandidates {
		top = top[:lookaheadCandidates]
	}
	var ties [][4]int
	for start, end := 0, 0; len(ties) == 0 && end < len(top); start = end {
		for end < len(top) && top[end].count == top[start].count {
			end++
		}
		// The last level only needs one lineup: ties score the same.
		ties = la.bestCounts(top[:end], depth == 1)
	}
	if len(ties) == 0 {
		score = make([]int, 4*depth)
		for i := range score {
			score[i] = noLineup
		}
		return score, false
	}
	for _, order := range ties {
		counts := []int{top[order[0]].count, top[order[1]].count, top[order[2]].count, top[order[3]].count}
		ideal := counts[0] == top[0].count && counts[1] == top[1].count && counts[2] == top[2].count && counts[3] == top[3].count
		rest, restPerfect := la.future(play(slots, order, lookaheadDepth-depth+2), depth-1)
		candidate := append(counts, rest...)
		if score == nil || compareInts(candidate, score) < 0 {
			score, perfect = candidate, ideal && restPerfect
		}
		if perfect {
			break
		}
	}
	return score, perfect
}

// bestCounts lists the valid lineups in the window that share the lowest
// priority counts. Slots are sorted by count, so combinations enumerate in
// non-decreasing count order and the search stops at the first worse one.
func (la *lookahead) bestCounts(w []slot, firstOnly bool) [][4]int {
	var (
		ties [][4]int
		best [4]int
	)
	n := len(w)
	for a := 0; a < n-3; a++ {
		for b := a + 1; b < n-2; b++ {
			for c := b + 1; c < n-1; c++ {
				for d := c + 1; d < n; d++ {
					counts := [4]int{w[a].count, w[b].count, w[c].count, w[d].count}
					if len(ties) > 0 && counts != best {
						return ties
					}
					if !la.valid[w[a].bit|w[b].bit|w[c].bit|w[d].bit] {
						continue
					}
					ties, best = append(ties, [4]int{a, b, c, d}), counts
					if firstOnly {
						return ties
					}
				}
			}
		}
	}
	return ties
}

// Arrange splits four given players into the best gender-valid teams:
// balanced first, then (among balanced splits) more same-group teams, then
// fewest repeats so partners rotate, then smallest level difference. The balance threshold is
// reported through Lineup.Balanced, not enforced, so the host can override
// it. It returns domain.ErrInvalidGenderLineup when no split is allowed.
func Arrange(four [4]Candidate, rules domain.MatchRules) (Lineup, error) {
	splits := [3][4]int{{0, 1, 2, 3}, {0, 2, 1, 3}, {0, 3, 1, 2}}
	var (
		best  Lineup
		found bool
	)
	for _, s := range splits {
		lineup, err := Evaluate([2]Candidate{four[s[0]], four[s[1]]}, [2]Candidate{four[s[2]], four[s[3]]}, rules)
		if err != nil {
			continue
		}
		if !found || splitBetter(&lineup, &best) {
			best, found = lineup, true
		}
	}
	if !found {
		return Lineup{}, domain.ErrInvalidGenderLineup
	}
	return best, nil
}

func splitBetter(l, b *Lineup) bool {
	if l.Balanced != b.Balanced {
		return l.Balanced
	}
	// Grouping never outweighs balance, so an unbalanced split is ranked by
	// level difference alone.
	if l.Balanced && l.GroupedTeams != b.GroupedTeams {
		return l.GroupedTeams > b.GroupedTeams
	}
	if l.Balanced && l.Repeats != b.Repeats {
		return l.Repeats < b.Repeats
	}
	if l.Diff != b.Diff {
		return l.Diff < b.Diff
	}
	return l.Repeats < b.Repeats
}

// Evaluate checks two given teams against the gender rules and measures
// their balance. It returns domain.ErrInvalidGenderLineup when the gender
// rules forbid the lineup; an unbalanced lineup is reported, not rejected.
func Evaluate(teamA, teamB [2]Candidate, rules domain.MatchRules) (Lineup, error) {
	a1, a2, b1, b2 := &teamA[0], &teamA[1], &teamB[0], &teamB[1]
	if !genderCompositionValid([4]Candidate{*a1, *a2, *b1, *b2}, rules) || !splitGenderValid(a1, a2, b1, b2) {
		return Lineup{}, domain.ErrInvalidGenderLineup
	}
	diff := a1.Level.Score() + a2.Level.Score() - b1.Level.Score() - b2.Level.Score()
	if diff < 0 {
		diff = -diff
	}
	grouped := 0
	if sameGroup(a1, a2) {
		grouped++
	}
	if sameGroup(b1, b2) {
		grouped++
	}
	return Lineup{
		TeamA:        [2]int64{a1.ID, a2.ID},
		TeamB:        [2]int64{b1.ID, b2.ID},
		Diff:         diff,
		Balanced:     diff <= rules.BalanceThreshold,
		Repeats:      countRepeats(a1, a2, b1, b2),
		GroupedTeams: grouped,
	}, nil
}

// genderCompositionValid applies the 3-1 rule: the three of one gender must
// out-level the odd one by the threshold on average. Compared as integers by
// multiplying both sides by 3.
func genderCompositionValid(four [4]Candidate, rules domain.MatchRules) bool {
	var men, menSum, womenSum int
	for i := range four {
		if four[i].Gender == domain.GenderMale {
			men++
			menSum += four[i].Level.Score()
		} else {
			womenSum += four[i].Level.Score()
		}
	}
	switch men {
	case 3:
		return menSum-3*womenSum >= 3*rules.GenderGapThreshold
	case 1:
		return womenSum-3*menSum >= 3*rules.GenderGapThreshold
	}
	return true
}

// splitGenderValid checks the pairing: with two men and two women each team
// must be mixed. Every 2-2 split of the other compositions already has the
// required shape.
func splitGenderValid(a1, a2, b1, b2 *Candidate) bool {
	menA := countMen(a1, a2)
	menB := countMen(b1, b2)
	if menA+menB == 2 {
		return menA == 1
	}
	return true
}

func countMen(ps ...*Candidate) int {
	n := 0
	for _, p := range ps {
		if p.Gender == domain.GenderMale {
			n++
		}
	}
	return n
}

func sameGroup(a, b *Candidate) bool {
	return a.Group != 0 && a.Group == b.Group
}

// groupScore ranks a lineup for players who came together: one point per
// same-group team. Group mates facing each other is the occasional
// whole-group match: it keeps its points only when none of the group pairs
// on court were opponents in their recent matches, and scores below an
// ungrouped lineup otherwise, so a group mostly teams up against outsiders.
func groupScore(four [4]Candidate, l *Lineup) int {
	byID := make(map[int64]*Candidate, 4)
	for i := range four {
		byID[four[i].ID] = &four[i]
	}
	facing := false
	for _, a := range l.TeamA {
		for _, b := range l.TeamB {
			if sameGroup(byID[a], byID[b]) {
				facing = true
			}
		}
	}
	if !facing {
		return l.GroupedTeams
	}
	for i := 0; i < 4; i++ {
		for j := i + 1; j < 4; j++ {
			a, b := &four[i], &four[j]
			if sameGroup(a, b) && (contains(a.RecentOpponents, b.ID) || contains(b.RecentOpponents, a.ID)) {
				return -1
			}
		}
	}
	return l.GroupedTeams
}

func staleGroupTeams(four [4]Candidate, l *Lineup) int {
	byID := make(map[int64]*Candidate, 4)
	for i := range four {
		byID[four[i].ID] = &four[i]
	}
	n := 0
	for _, team := range [2][2]int64{l.TeamA, l.TeamB} {
		a, b := byID[team[0]], byID[team[1]]
		if sameGroup(a, b) && (contains(a.RecentTeammates, b.ID) || contains(b.RecentTeammates, a.ID)) {
			n++
		}
	}
	return n
}

// countRepeats includes same-group pairs so partners rotate inside a group;
// Arrange still ranks same-group teams above repeats.
func countRepeats(a1, a2, b1, b2 *Candidate) int {
	n := 0
	for _, pair := range [2][2]*Candidate{{a1, a2}, {b1, b2}} {
		if contains(pair[0].RecentTeammates, pair[1].ID) || contains(pair[1].RecentTeammates, pair[0].ID) {
			n++
		}
	}
	for _, a := range [2]*Candidate{a1, a2} {
		for _, b := range [2]*Candidate{b1, b2} {
			if contains(a.RecentOpponents, b.ID) || contains(b.RecentOpponents, a.ID) {
				n++
			}
		}
	}
	return n
}

func contains(ids []int64, id int64) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

// oddOneOutFresh reports a 3-1 lineup worth avoiding. Once the odd player
// has been skipped, seating them wins: otherwise a lone woman (or man)
// can sit out repeatedly and break the one-match fairness gap.
func oddOneOutFresh(four [4]Candidate) bool {
	men := countMen(&four[0], &four[1], &four[2], &four[3])
	if men%2 == 0 {
		return false
	}
	odd := domain.GenderMale
	if men == 3 {
		odd = domain.GenderFemale
	}
	for i := range four {
		if four[i].Gender == odd {
			return four[i].ConsecutiveRest == 0
		}
	}
	return false
}

// courtMatePairs skips same-group pairs: they are meant to share courts.
func courtMatePairs(four [4]Candidate) int {
	n := 0
	for i := 0; i < 4; i++ {
		for j := i + 1; j < 4; j++ {
			a, b := &four[i], &four[j]
			if sameGroup(a, b) {
				continue
			}
			if contains(a.RecentTeammates, b.ID) || contains(a.RecentOpponents, b.ID) ||
				contains(b.RecentTeammates, a.ID) || contains(b.RecentOpponents, a.ID) {
				n++
			}
		}
	}
	return n
}
