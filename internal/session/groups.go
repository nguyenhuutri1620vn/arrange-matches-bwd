package session

import (
	"context"
	"time"

	"arrange-matches-bwd/internal/domain"
	"arrange-matches-bwd/internal/storage"
)

// groupFacts is what matchmaking needs to know about a player's group.
type groupFacts struct {
	size  int
	phase domain.GroupPhase
}

// groupFactsOf sizes each group by its members still in play: someone who
// went home early is RESTING and no longer part of the group's rhythm.
func groupFactsOf(groups []domain.SessionGroup, players []domain.SessionPlayer) map[int]groupFacts {
	facts := make(map[int]groupFacts, len(groups))
	for _, g := range groups {
		facts[int(g.ID)] = groupFacts{phase: g.Phase}
	}
	for i := range players {
		if players[i].Status == domain.PlayerResting {
			continue
		}
		if f, ok := facts[players[i].Group]; ok {
			f.size++
			facts[players[i].Group] = f
		}
	}
	return facts
}

// advanceGroups moves the rhythm of every group with a member in the match
// just confirmed. It runs inside the confirming transaction, after the
// match was saved as ONGOING.
func advanceGroups(ctx context.Context, q *storage.Queries, sessionID int64, match domain.Match, players []domain.SessionPlayer, now time.Time) error {
	team := map[int64]domain.Team{}
	for _, mp := range match.Players {
		team[mp.SessionPlayerID] = mp.Team
	}
	members := map[int][]int64{}
	for i := range players {
		if g := players[i].Group; g != 0 && players[i].Status != domain.PlayerResting {
			members[g] = append(members[g], players[i].ID)
		}
	}
	groups, err := q.ListSessionGroups(ctx, sessionID)
	if err != nil {
		return err
	}
	var started []domain.Match
	for _, g := range groups {
		ids := members[int(g.ID)]
		onCourt, perTeam := 0, map[domain.Team]int{}
		for _, id := range ids {
			if t, ok := team[id]; ok {
				onCourt++
				perTeam[t]++
			}
		}
		if onCourt == 0 {
			continue
		}
		teamed := perTeam[domain.TeamA] == 2 || perTeam[domain.TeamB] == 2
		splitDone := false
		if g.Phase == domain.GroupSplit {
			if started == nil {
				if started, err = q.ListMatches(ctx, sessionID, domain.MatchOngoing, domain.MatchFinished); err != nil {
					return err
				}
			}
			splitDone = everyonePlayedSince(ids, started, g.PhaseStartedAt)
		}
		next := domain.NextGroupPhase(g.Phase, len(ids), onCourt, teamed, splitDone)
		if next == g.Phase {
			continue
		}
		g.Phase, g.PhaseStartedAt = next, now
		if err := q.UpdateSessionGroupPhase(ctx, g); err != nil {
			return err
		}
	}
	return nil
}

func everyonePlayedSince(ids []int64, matches []domain.Match, since time.Time) bool {
	played := map[int64]bool{}
	for _, m := range matches {
		// The match that began the phase started at `since` and is not
		// part of it.
		if m.StartedAt == nil || !m.StartedAt.After(since) {
			continue
		}
		for _, mp := range m.Players {
			played[mp.SessionPlayerID] = true
		}
	}
	for _, id := range ids {
		if !played[id] {
			return false
		}
	}
	return true
}
