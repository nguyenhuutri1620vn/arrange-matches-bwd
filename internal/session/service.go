// Package session holds the usecases of a play session: check-in, courts,
// proposals, matches and the queue. Every state change runs in one
// transaction.
package session

import (
	"context"
	"errors"
	"time"

	"arrange-matches-bwd/internal/domain"
	"arrange-matches-bwd/internal/matchmaking"
	"arrange-matches-bwd/internal/storage"
)

type Service struct {
	store *storage.Store
	now   func() time.Time
}

// New builds the service. now is injectable for tests; nil means time.Now.
func New(store *storage.Store, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{store: store, now: func() time.Time { return now().UTC() }}
}

// MatchView is a match with its balance evaluated under the current rules.
type MatchView struct {
	domain.Match
	Diff     int
	Balanced bool
}

type CourtView struct {
	domain.Court
	Match *MatchView
}

type PlayerView struct {
	domain.SessionPlayer
	Fee int64
	// Held means the player is reserved by a proposal (DRAFT).
	Held bool
	// QueuePosition is the 1-based priority rank among waiting players,
	// 0 when not waiting.
	QueuePosition int
}

// State is the full picture of a session shown to the host.
type State struct {
	Session domain.Session
	Courts  []CourtView
	Players []PlayerView
	Fees    domain.SessionFees
}

type CreateParams struct {
	CourtCount int
	// Nil fees fall back to the app settings.
	FeeMale, FeeFemale, CourtFee *int64
}

// Create opens a new session. Only one session may be open at a time.
func (s *Service) Create(ctx context.Context, p CreateParams) (*State, error) {
	if !domain.ValidCourtCount(p.CourtCount) {
		return nil, domain.ErrInvalidCourtCount
	}
	var state *State
	err := s.store.InTx(ctx, func(q *storage.Queries) error {
		if _, err := q.GetOpenSession(ctx); err == nil {
			return domain.ErrSessionAlreadyOpen
		} else if !errors.Is(err, domain.ErrNotFound) {
			return err
		}
		settings, err := q.GetSettings(ctx)
		if err != nil {
			return err
		}
		now := s.now()
		sess := domain.Session{
			Date:       domain.VNDate(now),
			StartTime:  &now,
			Status:     domain.SessionOpen,
			CourtCount: p.CourtCount,
			FeeMale:    valueOr(p.FeeMale, settings.FeeMale),
			FeeFemale:  valueOr(p.FeeFemale, settings.FeeFemale),
			CourtFee:   valueOr(p.CourtFee, settings.CourtFee),
		}
		if sess.FeeMale < 0 || sess.FeeFemale < 0 || sess.CourtFee < 0 {
			return domain.ErrInvalidAmount
		}
		if sess.ID, err = q.InsertSession(ctx, sess); err != nil {
			return err
		}
		if err := q.EnsureCourts(ctx, sess.ID, sess.CourtCount); err != nil {
			return err
		}
		state, err = loadState(ctx, q, sess)
		return err
	})
	return state, err
}

func valueOr(v *int64, fallback int64) int64 {
	if v != nil {
		return *v
	}
	return fallback
}

// Current returns the open session, or nil when there is none.
func (s *Service) Current(ctx context.Context) (*State, error) {
	var state *State
	err := s.store.InTx(ctx, func(q *storage.Queries) error {
		sess, err := q.GetOpenSession(ctx)
		if errors.Is(err, domain.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		state, err = loadState(ctx, q, sess)
		return err
	})
	return state, err
}

// Get returns any session, open or ended.
func (s *Service) Get(ctx context.Context, sessionID int64) (*State, error) {
	var state *State
	err := s.store.InTx(ctx, func(q *storage.Queries) error {
		var err error
		state, err = stateOf(ctx, q, sessionID)
		return err
	})
	return state, err
}

func stateOf(ctx context.Context, q *storage.Queries, sessionID int64) (*State, error) {
	sess, err := q.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return loadState(ctx, q, sess)
}

func loadState(ctx context.Context, q *storage.Queries, sess domain.Session) (*State, error) {
	players, err := q.ListSessionPlayers(ctx, sess.ID)
	if err != nil {
		return nil, err
	}
	courts, err := q.ListCourts(ctx, sess.ID)
	if err != nil {
		return nil, err
	}
	active, err := q.ListMatches(ctx, sess.ID, domain.MatchDraft, domain.MatchOngoing)
	if err != nil {
		return nil, err
	}
	settings, err := q.GetSettings(ctx)
	if err != nil {
		return nil, err
	}

	byID := make(map[int64]*domain.SessionPlayer, len(players))
	for i := range players {
		byID[players[i].ID] = &players[i]
	}
	held := map[int64]bool{}
	matchByCourt := map[int64]*MatchView{}
	for _, m := range active {
		view := &MatchView{Match: m}
		if teamA, teamB, ok := teamsOf(m, byID, nil); ok {
			// A gender-invalid lineup (possible only if a player was edited
			// after the proposal) simply shows as unbalanced; confirming
			// re-checks the gender rule.
			if lineup, err := matchmaking.Evaluate(teamA, teamB, settings.Rules); err == nil {
				view.Diff, view.Balanced = lineup.Diff, lineup.Balanced
			}
		}
		if m.Status == domain.MatchDraft {
			for _, mp := range m.Players {
				held[mp.SessionPlayerID] = true
			}
		}
		// A court has at most one active match, so no overwrite happens.
		matchByCourt[m.CourtID] = view
	}

	state := &State{Session: sess, Fees: domain.SumSessionFees(&sess, players)}
	for _, c := range courts {
		if c.Index > sess.CourtCount {
			continue
		}
		state.Courts = append(state.Courts, CourtView{Court: c, Match: matchByCourt[c.ID]})
	}

	var queue []matchmaking.Candidate
	for i := range players {
		if players[i].Status == domain.PlayerWaiting {
			queue = append(queue, candidateOf(&players[i], nil))
		}
	}
	matchmaking.SortByPriority(queue)
	position := make(map[int64]int, len(queue))
	for i, c := range queue {
		position[c.ID] = i + 1
	}
	for i := range players {
		p := players[i]
		state.Players = append(state.Players, PlayerView{
			SessionPlayer: p,
			Fee:           p.Fee(&sess),
			Held:          held[p.ID],
			QueuePosition: position[p.ID],
		})
	}
	return state, nil
}

// openSession loads a session and requires it to be OPEN.
func openSession(ctx context.Context, q *storage.Queries, id int64) (domain.Session, error) {
	sess, err := q.GetSession(ctx, id)
	if err != nil {
		return sess, err
	}
	if sess.Status != domain.SessionOpen {
		return sess, domain.ErrSessionEnded
	}
	return sess, nil
}

// history holds each player's recent teammates and opponents.
type history map[int64]*recent

type recent struct {
	matches   int
	teammates []int64
	opponents []int64
}

// loadHistory collects, per player, the teammates and opponents of their
// latest `lookback` matches in this session.
func loadHistory(ctx context.Context, q *storage.Queries, sessionID int64, lookback int) (history, error) {
	h := history{}
	if lookback <= 0 {
		return h, nil
	}
	matches, err := q.ListMatches(ctx, sessionID, domain.MatchOngoing, domain.MatchFinished)
	if err != nil {
		return nil, err
	}
	for _, m := range matches {
		for _, me := range m.Players {
			r := h[me.SessionPlayerID]
			if r == nil {
				r = &recent{}
				h[me.SessionPlayerID] = r
			}
			if r.matches >= lookback {
				continue
			}
			r.matches++
			for _, other := range m.Players {
				switch {
				case other.SessionPlayerID == me.SessionPlayerID:
				case other.Team == me.Team:
					r.teammates = append(r.teammates, other.SessionPlayerID)
				default:
					r.opponents = append(r.opponents, other.SessionPlayerID)
				}
			}
		}
	}
	return h, nil
}

func candidateOf(p *domain.SessionPlayer, h history) matchmaking.Candidate {
	c := matchmaking.Candidate{
		ID:              p.ID,
		Gender:          p.Gender,
		Level:           p.Level,
		PriorityCount:   p.PriorityCount(),
		ConsecutiveRest: p.ConsecutiveRest,
		QueuedAt:        p.QueuedAt,
	}
	if r := h[p.ID]; r != nil {
		c.RecentTeammates, c.RecentOpponents = r.teammates, r.opponents
	}
	return c
}

// teamsOf turns a stored match into the two teams of candidates.
func teamsOf(m domain.Match, byID map[int64]*domain.SessionPlayer, h history) (teamA, teamB [2]matchmaking.Candidate, ok bool) {
	var a, b int
	for _, mp := range m.Players {
		p := byID[mp.SessionPlayerID]
		if p == nil {
			return teamA, teamB, false
		}
		switch {
		case mp.Team == domain.TeamA && a < 2:
			teamA[a] = candidateOf(p, h)
			a++
		case mp.Team == domain.TeamB && b < 2:
			teamB[b] = candidateOf(p, h)
			b++
		default:
			return teamA, teamB, false
		}
	}
	return teamA, teamB, a == 2 && b == 2
}

func matchPlayers(l matchmaking.Lineup) []domain.MatchPlayer {
	return []domain.MatchPlayer{
		{SessionPlayerID: l.TeamA[0], Team: domain.TeamA},
		{SessionPlayerID: l.TeamA[1], Team: domain.TeamA},
		{SessionPlayerID: l.TeamB[0], Team: domain.TeamB},
		{SessionPlayerID: l.TeamB[1], Team: domain.TeamB},
	}
}
