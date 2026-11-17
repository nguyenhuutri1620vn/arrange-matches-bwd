package session

import (
	"context"
	"errors"

	"arrange-matches-bwd/internal/domain"
	"arrange-matches-bwd/internal/storage"
)

func (s *Service) Settings(ctx context.Context) (domain.Settings, error) {
	return s.store.Q().GetSettings(ctx)
}

func (s *Service) UpdateSettings(ctx context.Context, settings domain.Settings) error {
	if err := settings.Validate(); err != nil {
		return err
	}
	return s.store.InTx(ctx, func(q *storage.Queries) error {
		return q.SaveSettings(ctx, settings)
	})
}

// ListPlayers returns the address book.
func (s *Service) ListPlayers(ctx context.Context) ([]domain.Player, error) {
	return s.store.Q().ListPlayers(ctx)
}

func (s *Service) CreatePlayer(ctx context.Context, p domain.Player) (domain.Player, error) {
	if err := p.Validate(); err != nil {
		return p, err
	}
	id, err := s.store.Q().InsertPlayer(ctx, p)
	p.ID = id
	return p, err
}

func (s *Service) UpdatePlayer(ctx context.Context, p domain.Player) (domain.Player, error) {
	if err := p.Validate(); err != nil {
		return p, err
	}
	err := s.store.InTx(ctx, func(q *storage.Queries) error {
		current, err := q.GetPlayer(ctx, p.ID)
		if err != nil {
			return err
		}
		if current.DeletedAt != nil {
			return domain.ErrPlayerDeleted
		}
		return q.UpdatePlayer(ctx, p)
	})
	return p, err
}

// DeletePlayer soft-deletes a player; past sessions keep showing them.
func (s *Service) DeletePlayer(ctx context.Context, id int64) error {
	return s.store.InTx(ctx, func(q *storage.Queries) error {
		if _, err := q.GetPlayer(ctx, id); err != nil {
			return err
		}
		return q.SoftDeletePlayer(ctx, id, s.now())
	})
}

// CheckIn adds a player from the address book to the session's queue.
// Someone joining mid-session starts at the lowest priority count present.
func (s *Service) CheckIn(ctx context.Context, sessionID, playerID int64, group GroupChoice) (*State, error) {
	return s.change(ctx, func(q *storage.Queries) (int64, error) {
		sess, err := openSession(ctx, q, sessionID)
		if err != nil {
			return 0, err
		}
		player, err := q.GetPlayer(ctx, playerID)
		if err != nil {
			return 0, err
		}
		if player.DeletedAt != nil {
			return 0, domain.ErrPlayerDeleted
		}
		if exists, err := q.SessionPlayerExists(ctx, sess.ID, playerID); err != nil {
			return 0, err
		} else if exists {
			return 0, domain.ErrAlreadyCheckedIn
		}
		present, err := q.ListSessionPlayers(ctx, sess.ID)
		if err != nil {
			return 0, err
		}
		now := s.now()
		groupID, err := s.joinGroup(ctx, q, sess.ID, 0, group)
		if err != nil {
			return 0, err
		}
		_, err = q.InsertSessionPlayer(ctx, domain.SessionPlayer{
			SessionID:      sess.ID,
			PlayerID:       playerID,
			Status:         domain.PlayerWaiting,
			PriorityOffset: domain.CheckInOffset(present),
			QueuedAt:       now,
			CheckedInAt:    now,
			Group:          groupID,
		})
		return sess.ID, err
	})
}

// GroupChoice picks a group for a player: an existing group by ID, a new
// group, or none (zero value).
type GroupChoice struct {
	ID  int64
	New bool
}

// joinGroup resolves the choice to a group ID in the session, creating a
// new group when asked. current is the player's group, which always has
// room for them.
func (s *Service) joinGroup(ctx context.Context, q *storage.Queries, sessionID int64, current int, c GroupChoice) (int, error) {
	if c.ID < 0 || (c.New && c.ID != 0) {
		return 0, domain.ErrInvalidGroup
	}
	if c.New {
		now := s.now()
		g, err := q.InsertSessionGroup(ctx, domain.SessionGroup{
			SessionID: sessionID, Phase: domain.GroupTogether, PhaseStartedAt: now, CreatedAt: now,
		})
		return int(g.ID), err
	}
	if c.ID == 0 || int(c.ID) == current {
		return int(c.ID), nil
	}
	g, err := q.GetSessionGroup(ctx, c.ID)
	if errors.Is(err, domain.ErrNotFound) || (err == nil && g.SessionID != sessionID) {
		return 0, domain.ErrInvalidGroup
	}
	if err != nil {
		return 0, err
	}
	players, err := q.ListSessionPlayers(ctx, sessionID)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, o := range players {
		if o.Group == int(c.ID) {
			n++
		}
	}
	if n >= domain.MaxGroupSize {
		return 0, domain.ErrGroupFull
	}
	return int(c.ID), nil
}

// RemovePlayer takes a player out of the session. Only allowed before they
// have appeared in any match or proposal.
func (s *Service) RemovePlayer(ctx context.Context, sessionPlayerID int64) (*State, error) {
	return s.change(ctx, func(q *storage.Queries) (int64, error) {
		p, err := q.GetSessionPlayer(ctx, sessionPlayerID)
		if err != nil {
			return 0, err
		}
		if _, err := openSession(ctx, q, p.SessionID); err != nil {
			return 0, err
		}
		if n, err := q.CountMatchesOfPlayer(ctx, p.ID); err != nil {
			return 0, err
		} else if n > 0 || p.MatchesPlayed > 0 || p.Status == domain.PlayerPlaying {
			return 0, domain.ErrPlayerHasPlayed
		}
		if err := q.DeleteSessionPlayer(ctx, p.ID); err != nil {
			return 0, err
		}
		return p.SessionID, q.DeleteEmptySessionGroups(ctx, p.SessionID)
	})
}

// SetPlayerStatus moves a player between the queue and resting. Resting a
// player who is held by a proposal drops that proposal.
func (s *Service) SetPlayerStatus(ctx context.Context, sessionPlayerID int64, status domain.PlayerStatus) (*State, error) {
	return s.change(ctx, func(q *storage.Queries) (int64, error) {
		p, err := q.GetSessionPlayer(ctx, sessionPlayerID)
		if err != nil {
			return 0, err
		}
		if _, err := openSession(ctx, q, p.SessionID); err != nil {
			return 0, err
		}
		switch status {
		case domain.PlayerResting:
			if err := p.Rest(); err != nil {
				return 0, err
			}
			drafts, err := q.ListMatches(ctx, p.SessionID, domain.MatchDraft)
			if err != nil {
				return 0, err
			}
			for _, d := range drafts {
				for _, mp := range d.Players {
					if mp.SessionPlayerID != p.ID {
						continue
					}
					if err := q.DeleteMatch(ctx, d.ID); err != nil {
						return 0, err
					}
				}
			}
		case domain.PlayerWaiting:
			if err := p.Resume(s.now()); err != nil {
				return 0, err
			}
		default:
			return 0, domain.ErrInvalidTransition
		}
		return p.SessionID, q.UpdateSessionPlayer(ctx, p)
	})
}

// SetPlayerGroup marks who came together; group 0 clears it. Existing
// proposals are kept: the grouping applies from the next one.
func (s *Service) SetPlayerGroup(ctx context.Context, sessionPlayerID int64, group GroupChoice) (*State, error) {
	return s.change(ctx, func(q *storage.Queries) (int64, error) {
		p, err := q.GetSessionPlayer(ctx, sessionPlayerID)
		if err != nil {
			return 0, err
		}
		if _, err := openSession(ctx, q, p.SessionID); err != nil {
			return 0, err
		}
		if p.Group, err = s.joinGroup(ctx, q, p.SessionID, p.Group, group); err != nil {
			return 0, err
		}
		if err := q.UpdateSessionPlayer(ctx, p); err != nil {
			return 0, err
		}
		return p.SessionID, q.DeleteEmptySessionGroups(ctx, p.SessionID)
	})
}

// PaymentUpdate changes a player's paid flag and/or fee override.
type PaymentUpdate struct {
	IsPaid *bool
	// SetFeeOverride applies FeeOverride; a nil FeeOverride clears it.
	SetFeeOverride bool
	FeeOverride    *int64
}

// SetPayment is allowed on ended sessions too; there it also keeps the
// session-fee income in sync with what has been collected.
func (s *Service) SetPayment(ctx context.Context, sessionPlayerID int64, u PaymentUpdate) (*State, error) {
	return s.change(ctx, func(q *storage.Queries) (int64, error) {
		p, err := q.GetSessionPlayer(ctx, sessionPlayerID)
		if err != nil {
			return 0, err
		}
		if u.IsPaid != nil {
			p.IsPaid = *u.IsPaid
		}
		if u.SetFeeOverride {
			if u.FeeOverride != nil && *u.FeeOverride < 0 {
				return 0, domain.ErrInvalidAmount
			}
			p.FeeOverride = u.FeeOverride
		}
		if err := q.UpdateSessionPlayer(ctx, p); err != nil {
			return 0, err
		}
		sess, err := q.GetSession(ctx, p.SessionID)
		if err != nil {
			return 0, err
		}
		if sess.Status == domain.SessionEnded {
			if err := syncSessionFee(ctx, q, sess); err != nil {
				return 0, err
			}
		}
		return p.SessionID, nil
	})
}

// change runs a state change in a transaction and returns the session's
// new state. fn returns the id of the session it touched.
func (s *Service) change(ctx context.Context, fn func(q *storage.Queries) (int64, error)) (*State, error) {
	var state *State
	err := s.store.InTx(ctx, func(q *storage.Queries) error {
		sessionID, err := fn(q)
		if err != nil {
			return err
		}
		state, err = stateOf(ctx, q, sessionID)
		return err
	})
	return state, err
}
