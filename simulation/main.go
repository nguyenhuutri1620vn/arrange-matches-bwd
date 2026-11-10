// Command simulation plays whole sessions in memory to check match-count
// fairness of the matchmaking algorithm.
//
//	go run ./simulation -courts 2 -players 16 -runs 1000
//	go run ./simulation -matrix -runs 1000
package main

import (
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"sync"
	"time"

	"arrange-matches-bwd/internal/domain"
	"arrange-matches-bwd/internal/matchmaking"
)

type options struct {
	courts, players, runs  int
	matchMinutes, duration int
	seed                   int64
}

type result struct {
	runs, failed int
	stuckRuns    int // runs where some court had no valid lineup at least once
	unseatable   int // runs with a player no valid lineup can include
	excused      int // spread > 1 but the rules make a fair session impossible
	firstFailure string
}

func main() {
	var (
		opt    options
		file   string
		matrix bool
	)
	flag.IntVar(&opt.courts, "courts", 2, "number of courts (1-3)")
	flag.IntVar(&opt.players, "players", 16, "number of players in the session")
	flag.IntVar(&opt.runs, "runs", 1000, "number of simulated sessions")
	flag.IntVar(&opt.matchMinutes, "match-minutes", 15, "length of one match")
	flag.IntVar(&opt.duration, "duration", 120, "session length in minutes")
	flag.Int64Var(&opt.seed, "seed", 1, "random seed")
	flag.StringVar(&file, "file", "simulation/players.csv", "CSV of players: name,gender,level")
	flag.BoolVar(&matrix, "matrix", false, "run 1-3 courts with 7 to 8 players per court")
	verbose := flag.Bool("v", false, "print every failing run")
	flag.Parse()

	pool, err := loadPlayers(file)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load players:", err)
		os.Exit(2)
	}

	var cases []options
	if matrix {
		for courts := 1; courts <= 3; courts++ {
			for n := 7 * courts; n <= 8*courts; n++ {
				o := opt
				o.courts, o.players = courts, n
				cases = append(cases, o)
			}
		}
	} else {
		cases = []options{opt}
	}

	failed := false
	for _, o := range cases {
		if o.players > len(pool) {
			fmt.Fprintf(os.Stderr, "need %d players but %s has only %d\n", o.players, file, len(pool))
			os.Exit(2)
		}
		res := simulate(o, pool, *verbose)
		status := "PASS"
		if res.failed > 0 {
			status, failed = "FAIL", true
		}
		fmt.Printf("%s courts=%d players=%-2d runs=%d  unfair: %d  unfair but impossible under the rules: %d  with unseatable player: %d  with idle court: %d\n",
			status, o.courts, o.players, res.runs, res.failed, res.excused, res.unseatable, res.stuckRuns)
		if res.firstFailure != "" {
			fmt.Println("  first failure:", res.firstFailure)
		}
	}
	if failed {
		os.Exit(1)
	}
}

type poolPlayer struct {
	name   string
	gender domain.Gender
	level  domain.Level
}

func loadPlayers(path string) ([]poolPlayer, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}
	var out []poolPlayer
	for i, row := range rows {
		if i == 0 || len(row) < 3 {
			continue
		}
		p := poolPlayer{name: row[0], gender: domain.Gender(row[1]), level: domain.Level(row[2])}
		if !p.gender.Valid() || !p.level.Valid() {
			return nil, fmt.Errorf("line %d: invalid gender or level", i+1)
		}
		out = append(out, p)
	}
	return out, nil
}

func simulate(opt options, pool []poolPlayer, verbose bool) result {
	rng := rand.New(rand.NewSource(opt.seed))
	picks := make([][]int, opt.runs)
	for i := range picks {
		picks[i] = rng.Perm(len(pool))[:opt.players]
	}

	// Runs are independent; play them in parallel and merge in order.
	type outcome struct {
		spread, idle, excluded int
		fairPossible           bool
		desc                   string
	}
	outcomes := make([]outcome, opt.runs)
	var wg sync.WaitGroup
	next := make(chan int)
	for w := 0; w < runtime.NumCPU(); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				o := &outcomes[i]
				o.spread, o.idle, o.excluded, o.fairPossible, o.desc = runSession(opt, pool, picks[i])
			}
		}()
	}
	for i := range picks {
		next <- i
	}
	close(next)
	wg.Wait()

	res := result{runs: opt.runs}
	for _, o := range outcomes {
		if o.excluded > 0 {
			res.unseatable++
		}
		if o.idle > 0 {
			res.stuckRuns++
		}
		if o.spread > 1 && !o.fairPossible {
			res.excused++
		} else if o.spread > 1 {
			res.failed++
			if res.firstFailure == "" {
				res.firstFailure = o.desc
			}
			if verbose {
				fmt.Println("  fail:", o.desc)
			}
		}
	}
	return res
}

type simPlayer struct {
	domain.SessionPlayer
	name string
	// latest matches first
	teammates [][]int64
	opponents [][]int64
}

// runSession plays one session and returns the spread between the most and
// least matches played, how many court-rounds stayed idle because no valid
// lineup existed, and how many players were left out of the spread because
// the gender and balance rules allow no lineup that includes them.
func runSession(opt options, pool []poolPlayer, picked []int) (spread, idle, excluded int, fairPossible bool, desc string) {
	rules := domain.DefaultSettings().Rules
	start := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	players := make([]*simPlayer, len(picked))
	for i, idx := range picked {
		players[i] = &simPlayer{name: pool[idx].name, SessionPlayer: domain.SessionPlayer{
			ID: int64(i + 1), Gender: pool[idx].gender, Level: pool[idx].level,
			Status: domain.PlayerWaiting, QueuedAt: start.Add(time.Duration(i) * time.Second),
		}}
	}
	byID := func(id int64) *simPlayer { return players[id-1] }

	rounds := opt.duration / opt.matchMinutes
	for round := 0; round < rounds; round++ {
		now := start.Add(time.Duration(round*opt.matchMinutes) * time.Minute)

		// Propose a draft per court; drafted players are held.
		held := map[int64]bool{}
		var drafts []matchmaking.Lineup
		for court := 0; court < opt.courts; court++ {
			var queue []matchmaking.Candidate
			for _, p := range players {
				if p.Status == domain.PlayerWaiting && !held[p.ID] {
					queue = append(queue, candidate(p, rules))
				}
			}
			lineup, err := matchmaking.Propose(queue, rules)
			if err != nil {
				if errors.Is(err, domain.ErrNoValidLineup) {
					idle++
				}
				continue
			}
			for _, id := range lineupIDs(lineup) {
				held[id] = true
			}
			drafts = append(drafts, lineup)
		}

		// Confirm each draft in court order.
		for _, lineup := range drafts {
			in := map[int64]bool{}
			for _, id := range lineupIDs(lineup) {
				in[id] = true
			}
			for _, p := range players {
				if in[p.ID] {
					if err := p.StartMatch(); err != nil {
						panic(fmt.Sprintf("player %d put in two matches: %v", p.ID, err))
					}
				} else {
					p.SkipMatch()
				}
			}
		}

		// All matches of the round finish together.
		end := now.Add(time.Duration(opt.matchMinutes) * time.Minute)
		for _, lineup := range drafts {
			teams := [2][2]int64{lineup.TeamA, lineup.TeamB}
			for t, team := range teams {
				for i, id := range team {
					p := byID(id)
					if err := p.FinishMatch(end); err != nil {
						panic(err)
					}
					p.teammates = prepend(p.teammates, []int64{team[1-i]}, rules.RepeatLookback)
					p.opponents = prepend(p.opponents, teams[1-t][:], rules.RepeatLookback)
				}
			}
		}
	}

	min, max := -1, 0
	var seated []*simPlayer
	for _, p := range players {
		if !seatable(p, players, rules) {
			excluded++
			continue
		}
		seated = append(seated, p)
		if min < 0 || p.MatchesPlayed < min {
			min = p.MatchesPlayed
		}
		if p.MatchesPlayed > max {
			max = p.MatchesPlayed
		}
	}
	fairPossible = true
	if max-min > 1 {
		fairPossible = fairSchedulePossible(seated, rules, rounds*opt.courts)
		for _, p := range players {
			desc += fmt.Sprintf("%s(%s,%s)=%d ", p.name, p.Gender, p.Level, p.MatchesPlayed)
		}
	}
	return max - min, idle, excluded, fairPossible, desc
}

// fairSchedulePossible checks a necessary condition for a fair session:
// some choice of `matches` valid lineups gives every player a match count
// within one of the others. It ignores which lineups can share a round, so
// "false" proves no algorithm could pass, while "true" may be optimistic.
// The search is budgeted; running out counts as possible.
func fairSchedulePossible(players []*simPlayer, rules domain.MatchRules, matches int) bool {
	n := len(players)
	if n < 4 {
		return false
	}
	floor := 4 * matches / n
	ceil := (4*matches + n - 1) / n
	cands := make([]matchmaking.Candidate, n)
	for i, p := range players {
		cands[i] = matchmaking.Candidate{ID: p.ID, Gender: p.Gender, Level: p.Level}
	}
	var lineups [][4]int
	for a := 0; a < n; a++ {
		for b := a + 1; b < n; b++ {
			for c := b + 1; c < n; c++ {
				for d := c + 1; d < n; d++ {
					l, err := matchmaking.Arrange([4]matchmaking.Candidate{cands[a], cands[b], cands[c], cands[d]}, rules)
					if err == nil && l.Balanced {
						lineups = append(lineups, [4]int{a, b, c, d})
					}
				}
			}
		}
	}
	counts := make([]int, n)
	budget := 2_000_000
	var dfs func(left int) bool
	dfs = func(left int) bool {
		if budget--; budget < 0 {
			return true
		}
		// Every remaining match must include the player furthest behind.
		neediest, need := 0, 0
		for i, c := range counts {
			if c < counts[neediest] {
				neediest = i
			}
			if c < floor {
				need += floor - c
			}
		}
		if left == 0 {
			return need == 0
		}
		if need > 4*left {
			return false
		}
		for _, l := range lineups {
			if l[0] != neediest && l[1] != neediest && l[2] != neediest && l[3] != neediest {
				continue
			}
			if counts[l[0]] >= ceil || counts[l[1]] >= ceil || counts[l[2]] >= ceil || counts[l[3]] >= ceil {
				continue
			}
			for _, i := range l {
				counts[i]++
			}
			ok := dfs(left - 1)
			for _, i := range l {
				counts[i]--
			}
			if ok {
				return true
			}
		}
		return false
	}
	return dfs(matches)
}

// seatable reports whether any valid, balanced lineup of the session's
// players includes p. A player who fails this (for example the only woman
// among men of similar level) can never play under the rules, so no
// algorithm could keep them within one match of the others.
func seatable(p *simPlayer, players []*simPlayer, rules domain.MatchRules) bool {
	var others []matchmaking.Candidate
	for _, o := range players {
		if o != p {
			others = append(others, matchmaking.Candidate{ID: o.ID, Gender: o.Gender, Level: o.Level})
		}
	}
	me := matchmaking.Candidate{ID: p.ID, Gender: p.Gender, Level: p.Level}
	for a := 0; a < len(others); a++ {
		for b := a + 1; b < len(others); b++ {
			for c := b + 1; c < len(others); c++ {
				l, err := matchmaking.Arrange([4]matchmaking.Candidate{me, others[a], others[b], others[c]}, rules)
				if err == nil && l.Balanced {
					return true
				}
			}
		}
	}
	return false
}

func candidate(p *simPlayer, rules domain.MatchRules) matchmaking.Candidate {
	c := matchmaking.Candidate{
		ID: p.ID, Gender: p.Gender, Level: p.Level,
		PriorityCount: p.PriorityCount(), ConsecutiveRest: p.ConsecutiveRest, QueuedAt: p.QueuedAt,
	}
	for _, ids := range p.teammates {
		c.RecentTeammates = append(c.RecentTeammates, ids...)
	}
	for _, ids := range p.opponents {
		c.RecentOpponents = append(c.RecentOpponents, ids...)
	}
	return c
}

func lineupIDs(l matchmaking.Lineup) []int64 {
	return []int64{l.TeamA[0], l.TeamA[1], l.TeamB[0], l.TeamB[1]}
}

func prepend(history [][]int64, entry []int64, keep int) [][]int64 {
	history = append([][]int64{append([]int64(nil), entry...)}, history...)
	if len(history) > keep {
		history = history[:keep]
	}
	return history
}
