package session

import (
	"context"
	"errors"
	"time"

	"arrange-matches-bwd/internal/domain"
	"arrange-matches-bwd/internal/matchmaking"
	"arrange-matches-bwd/internal/storage"
)

// SetCourtCount opens or closes courts. Closing a court that is in use or
// holds a proposal is refused.
func (s *Service) SetCourtCount(ctx context.Context, sessionID int64, count int) (*State, error) {
	if !domain.ValidCourtCount(count) {
		return nil, domain.ErrInvalidCourtCount
	}
	return s.change(ctx, func(q *storage.Queries) (int64, error) {
		sess, err := openSession(ctx, q, sessionID)
		if err != nil {
			return 0, err
		}
		if count < sess.CourtCount {
			courts, err := q.ListCourts(ctx, sess.ID)
			if err != nil {
				return 0, err
			}
			drafts, err := q.ListMatches(ctx, sess.ID, domain.MatchDraft)
			if err != nil {
				return 0, err
			}
			hasDraft := map[int64]bool{}
			for _, d := range drafts {
				hasDraft[d.CourtID] = true
			}
			for _, c := range courts {
				if c.Index > count && (c.Status == domain.CourtInUse || hasDraft[c.ID]) {
					return 0, domain.ErrCourtBusy
				}
			}
		}
		if err := q.UpdateSessionCourtCount(ctx, sess.ID, count); err != nil {
			return 0, err
		}
		return sess.ID, q.EnsureCourts(ctx, sess.ID, count)
	})
}

// Propose creates a proposal (DRAFT) for an empty court. The app only
// proposes; the host confirms.
func (s *Service) Propose(ctx context.Context, courtID int64) (*State, error) {
	return s.change(ctx, func(q *storage.Queries) (int64, error) {
		court, err := q.GetCourt(ctx, courtID)
		if err != nil {
			return 0, err
		}
		sess, err := openSession(ctx, q, court.SessionID)
		if err != nil {
			return 0, err
		}
		return sess.ID, propose(ctx, q, sess, court)
	})
}

func propose(ctx context.Context, q *storage.Queries, sess domain.Session, court domain.Court) error {
	if court.Index > sess.CourtCount {
		return domain.ErrCourtClosed
	}
	if court.Status != domain.CourtEmpty {
		return domain.ErrCourtNotEmpty
	}
	drafts, err := q.ListMatches(ctx, sess.ID, domain.MatchDraft)
	if err != nil {
		return err
	}
	held := map[int64]bool{}
	for _, d := range drafts {
		if d.CourtID == court.ID {
			return domain.ErrCourtNotEmpty
		}
		for _, mp := range d.Players {
			held[mp.SessionPlayerID] = true
		}
	}
	settings, err := q.GetSettings(ctx)
	if err != nil {
		return err
	}
	players, err := q.ListSessionPlayers(ctx, sess.ID)
	if err != nil {
		return err
	}
	h, err := loadHistory(ctx, q, sess.ID, settings.Rules.RepeatLookback)
	if err != nil {
		return err
	}
	var queue []matchmaking.Candidate
	for i := range players {
		if players[i].Status == domain.PlayerWaiting && !held[players[i].ID] {
			queue = append(queue, candidateOf(&players[i], h))
		}
	}
	lineup, err := matchmaking.Propose(queue, settings.Rules)
	if err != nil {
		return err
	}
	_, err = q.InsertMatch(ctx, domain.Match{
		SessionID: sess.ID,
		CourtID:   court.ID,
		Status:    domain.MatchDraft,
		Players:   matchPlayers(lineup),
	})
	return err
}

// draftContext loads what the proposal operations share.
type draftContext struct {
	sess     domain.Session
	match    domain.Match
	players  []domain.SessionPlayer
	byID     map[int64]*domain.SessionPlayer
	settings domain.Settings
	history  history
}

func loadDraft(ctx context.Context, q *storage.Queries, matchID int64) (*draftContext, error) {
	m, err := q.GetMatch(ctx, matchID)
	if err != nil {
		return nil, err
	}
	if m.Status != domain.MatchDraft {
		return nil, domain.ErrMatchNotDraft
	}
	d := &draftContext{match: m}
	if d.sess, err = openSession(ctx, q, m.SessionID); err != nil {
		return nil, err
	}
	if d.players, err = q.ListSessionPlayers(ctx, m.SessionID); err != nil {
		return nil, err
	}
	d.byID = make(map[int64]*domain.SessionPlayer, len(d.players))
	for i := range d.players {
		d.byID[d.players[i].ID] = &d.players[i]
	}
	if d.settings, err = q.GetSettings(ctx); err != nil {
		return nil, err
	}
	d.history, err = loadHistory(ctx, q, m.SessionID, d.settings.Rules.RepeatLookback)
	return d, err
}

// SwapDraftPlayer replaces one player of a proposal with a waiting player
// and re-splits the teams. The gender rule is a hard block; a level
// imbalance is only reported on the proposal.
func (s *Service) SwapDraftPlayer(ctx context.Context, matchID, outID, inID int64) (*State, error) {
	return s.change(ctx, func(q *storage.Queries) (int64, error) {
		d, err := loadDraft(ctx, q, matchID)
		if err != nil {
			return 0, err
		}
		in := d.byID[inID]
		if in == nil {
			return 0, domain.ErrNotFound
		}
		if in.Status != domain.PlayerWaiting {
			return 0, domain.ErrPlayerNotWaiting
		}
		drafts, err := q.ListMatches(ctx, d.sess.ID, domain.MatchDraft)
		if err != nil {
			return 0, err
		}
		for _, other := range drafts {
			for _, mp := range other.Players {
				if mp.SessionPlayerID == inID {
					return 0, domain.ErrPlayerHeld
				}
			}
		}

		var (
			four  [4]matchmaking.Candidate
			n     int
			found bool
		)
		for _, mp := range d.match.Players {
			id := mp.SessionPlayerID
			if id == outID {
				id, found = inID, true
			}
			if p := d.byID[id]; p != nil && n < 4 {
				four[n] = candidateOf(p, d.history)
				n++
			}
		}
		if !found || n != 4 {
			return 0, domain.ErrPlayerNotInMatch
		}
		lineup, err := matchmaking.Arrange(four, d.settings.Rules)
		if err != nil {
			return 0, err
		}
		return d.sess.ID, q.ReplaceMatchPlayers(ctx, d.match.ID, matchPlayers(lineup))
	})
}

// Confirm starts a proposed match. An unbalanced lineup needs force (the
// host's "Vẫn xác nhận"); a gender-invalid lineup can never start.
func (s *Service) Confirm(ctx context.Context, matchID int64, force bool) (*State, error) {
	return s.change(ctx, func(q *storage.Queries) (int64, error) {
		d, err := loadDraft(ctx, q, matchID)
		if err != nil {
			return 0, err
		}
		court, err := q.GetCourt(ctx, d.match.CourtID)
		if err != nil {
			return 0, err
		}
		if court.Status != domain.CourtEmpty {
			return 0, domain.ErrCourtNotEmpty
		}
		if court.Index > d.sess.CourtCount {
			return 0, domain.ErrCourtClosed
		}
		teamA, teamB, ok := teamsOf(d.match, d.byID, d.history)
		if !ok {
			return 0, domain.ErrInvalidGenderLineup
		}
		lineup, err := matchmaking.Evaluate(teamA, teamB, d.settings.Rules)
		if err != nil {
			return 0, err
		}
		if !lineup.Balanced && !force {
			return 0, domain.ErrUnbalancedLineup
		}

		inMatch := map[int64]bool{}
		for _, mp := range d.match.Players {
			inMatch[mp.SessionPlayerID] = true
		}
		for i := range d.players {
			p := &d.players[i]
			before := *p
			if inMatch[p.ID] {
				if err := p.StartMatch(); err != nil {
					return 0, err
				}
			} else {
				p.SkipMatch()
			}
			if p.Status != before.Status || p.ConsecutiveRest != before.ConsecutiveRest {
				if err := q.UpdateSessionPlayer(ctx, *p); err != nil {
					return 0, err
				}
			}
		}
		now := s.now()
		d.match.Status, d.match.StartedAt = domain.MatchOngoing, &now
		if err := q.UpdateMatch(ctx, d.match); err != nil {
			return 0, err
		}
		court.Status, court.CurrentMatchID = domain.CourtInUse, &d.match.ID
		return d.sess.ID, q.UpdateCourt(ctx, court)
	})
}

// DismissDraft drops a proposal; its players go back to the free queue and
// nothing is recorded.
func (s *Service) DismissDraft(ctx context.Context, matchID int64) (*State, error) {
	return s.change(ctx, func(q *storage.Queries) (int64, error) {
		m, err := q.GetMatch(ctx, matchID)
		if err != nil {
			return 0, err
		}
		if m.Status != domain.MatchDraft {
			return 0, domain.ErrMatchNotDraft
		}
		return m.SessionID, q.DeleteMatch(ctx, m.ID)
	})
}

// Finish ends a running match: the court is freed, the four players return
// to the queue with one more match, and the next proposal for that court
// is prepared when the queue allows one.
func (s *Service) Finish(ctx context.Context, matchID int64) (*State, error) {
	return s.change(ctx, func(q *storage.Queries) (int64, error) {
		m, err := q.GetMatch(ctx, matchID)
		if err != nil {
			return 0, err
		}
		sess, err := openSession(ctx, q, m.SessionID)
		if err != nil {
			return 0, err
		}
		if err := finishMatch(ctx, q, m, s.now()); err != nil {
			return 0, err
		}
		court, err := q.GetCourt(ctx, m.CourtID)
		if err != nil {
			return 0, err
		}
		// No proposal is fine here: the host sees the reason when asking
		// for one explicitly.
		err = propose(ctx, q, sess, court)
		if errors.Is(err, domain.ErrNotEnoughPlayers) || errors.Is(err, domain.ErrNoValidLineup) || errors.Is(err, domain.ErrCourtClosed) {
			err = nil
		}
		return sess.ID, err
	})
}

func finishMatch(ctx context.Context, q *storage.Queries, m domain.Match, now time.Time) error {
	if m.Status != domain.MatchOngoing {
		return domain.ErrMatchNotOngoing
	}
	for _, mp := range m.Players {
		p, err := q.GetSessionPlayer(ctx, mp.SessionPlayerID)
		if err != nil {
			return err
		}
		if err := p.FinishMatch(now); err != nil {
			return err
		}
		if err := q.UpdateSessionPlayer(ctx, p); err != nil {
			return err
		}
	}
	m.Status, m.FinishedAt = domain.MatchFinished, &now
	if err := q.UpdateMatch(ctx, m); err != nil {
		return err
	}
	court, err := q.GetCourt(ctx, m.CourtID)
	if err != nil {
		return err
	}
	court.Status, court.CurrentMatchID = domain.CourtEmpty, nil
	return q.UpdateCourt(ctx, court)
}

// End closes the session: running matches finish and count, proposals are
// dropped, and the session-fee income and court-rent expense are recorded.
func (s *Service) End(ctx context.Context, sessionID int64) (*State, error) {
	return s.change(ctx, func(q *storage.Queries) (int64, error) {
		sess, err := openSession(ctx, q, sessionID)
		if err != nil {
			return 0, err
		}
		now := s.now()
		active, err := q.ListMatches(ctx, sess.ID, domain.MatchDraft, domain.MatchOngoing)
		if err != nil {
			return 0, err
		}
		for _, m := range active {
			if m.Status == domain.MatchDraft {
				err = q.DeleteMatch(ctx, m.ID)
			} else {
				err = finishMatch(ctx, q, m, now)
			}
			if err != nil {
				return 0, err
			}
		}
		if err := q.MarkSessionEnded(ctx, sess.ID, now); err != nil {
			return 0, err
		}

		players, err := q.ListSessionPlayers(ctx, sess.ID)
		if err != nil {
			return 0, err
		}
		fees := domain.SumSessionFees(&sess, players)
		if _, err := q.InsertTransaction(ctx, domain.Transaction{
			SessionID: &sess.ID,
			Type:      domain.TypeIncome,
			Category:  domain.CatSessionFee,
			Amount:    fees.Collected,
			Date:      sess.Date,
			Auto:      true,
		}); err != nil {
			return 0, err
		}
		if rent := sess.CourtFee * int64(sess.CourtCount); rent > 0 {
			if _, err := q.InsertTransaction(ctx, domain.Transaction{
				SessionID: &sess.ID,
				Type:      domain.TypeExpense,
				Category:  domain.CatCourtRent,
				Amount:    rent,
				Date:      sess.Date,
			}); err != nil {
				return 0, err
			}
		}
		return sess.ID, nil
	})
}

// syncSessionFee updates an ended session's fee income to the amount
// collected. If the host deleted that transaction it stays deleted.
func syncSessionFee(ctx context.Context, q *storage.Queries, sess domain.Session) error {
	tx, err := q.GetAutoSessionFee(ctx, sess.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	players, err := q.ListSessionPlayers(ctx, sess.ID)
	if err != nil {
		return err
	}
	tx.Amount = domain.SumSessionFees(&sess, players).Collected
	return q.UpdateTransaction(ctx, tx)
}
