// Package matchmaking picks four players from the queue and splits them
// into two teams. It is pure: no storage, no clock.
package matchmaking

import (
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
//  3. how hard the picked players are to seat (those with the fewest
//     valid lineups go first, while a lineup for them is available),
//  4. consecutive rest (longer rest first),
//  5. team level difference (smaller first),
//  6. repeated teammates/opponents (fewer first),
//  7. queue order.
//
// Gender rules and the balance threshold are hard constraints. It returns
// domain.ErrNotEnoughPlayers or domain.ErrNoValidLineup when nothing fits.
func Propose(cands []Candidate, rules domain.MatchRules) (Lineup, error) {
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
	if lineup, ok := proposeFrom(capped, rules, true); ok {
		return lineup, nil
	}
	if len(sorted) > len(capped) {
		if lineup, ok := proposeFrom(sorted, rules, false); ok {
			return lineup, nil
		}
	}
	return Lineup{}, domain.ErrNoValidLineup
}

// proposeFrom widens the candidate window tier by tier until a lineup is
// found. Lookahead requires len(sorted) <= maxCandidates.
func proposeFrom(sorted []Candidate, rules domain.MatchRules, useLookahead bool) (Lineup, bool) {
	for end := windowEnd(sorted, 0); ; end = windowEnd(sorted, end) {
		if end >= 4 {
			if lineup, ok := bestInWindow(sorted, end, rules, useLookahead); ok {
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
	stranded int
	future   []int
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
	if c := compareInts(o.scarcity[:], b.scarcity[:]); c != 0 {
		return c < 0
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

func bestInWindow(sorted []Candidate, end int, rules domain.MatchRules, useLookahead bool) (Lineup, bool) {
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
					if len(ties) > 0 {
						if cmp := compareInts(o.counts[:], ties[0].counts[:]); cmp > 0 {
							continue
						} else if cmp < 0 {
							ties = ties[:0]
						}
					}
					lineup, err := Arrange([4]Candidate{w[a], w[b], w[c], w[d]}, rules)
					if err != nil || !lineup.Balanced {
						continue
					}
					o.lineup = lineup
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
// smallest level difference, then fewest repeats. The balance threshold is
// reported through Lineup.Balanced, not enforced, so the host can override
// it. It returns domain.ErrInvalidGenderLineup when no split is allowed.
func Arrange(four [4]Candidate, rules domain.MatchRules) (Lineup, error) {
	if !genderCompositionValid(four, rules) {
		return Lineup{}, domain.ErrInvalidGenderLineup
	}
	splits := [3][4]int{{0, 1, 2, 3}, {0, 2, 1, 3}, {0, 3, 1, 2}}
	var (
		best  Lineup
		found bool
	)
	for _, s := range splits {
		a1, a2, b1, b2 := &four[s[0]], &four[s[1]], &four[s[2]], &four[s[3]]
		if !splitGenderValid(a1, a2, b1, b2) {
			continue
		}
		diff := a1.Level.Score() + a2.Level.Score() - b1.Level.Score() - b2.Level.Score()
		if diff < 0 {
			diff = -diff
		}
		repeats := countRepeats(a1, a2, b1, b2)
		if !found || diff < best.Diff || (diff == best.Diff && repeats < best.Repeats) {
			best = Lineup{
				TeamA:    [2]int64{a1.ID, a2.ID},
				TeamB:    [2]int64{b1.ID, b2.ID},
				Diff:     diff,
				Balanced: diff <= rules.BalanceThreshold,
				Repeats:  repeats,
			}
			found = true
		}
	}
	if !found {
		return Lineup{}, domain.ErrInvalidGenderLineup
	}
	return best, nil
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
