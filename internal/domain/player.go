package domain

import (
	"strings"
	"time"
)

// Player is an entry of the address book shared across sessions.
type Player struct {
	ID         int64
	Name       string
	Gender     Gender
	Level      Level
	IsRegular  bool
	MonthlyFee int64
	Note       string
	DeletedAt  *time.Time
}

func (p *Player) Validate() error {
	p.Name = strings.TrimSpace(p.Name)
	switch {
	case p.Name == "":
		return ErrInvalidName
	case !p.Gender.Valid():
		return ErrInvalidGender
	case !p.Level.Valid():
		return ErrInvalidLevel
	case p.MonthlyFee < 0:
		return ErrInvalidAmount
	}
	return nil
}

type PlayerStatus string

const (
	PlayerWaiting PlayerStatus = "WAITING"
	PlayerPlaying PlayerStatus = "PLAYING"
	PlayerResting PlayerStatus = "RESTING"
)

// SessionPlayer is a player's state inside one session. Name, Gender, Level
// and IsRegular are read from the address book.
type SessionPlayer struct {
	ID        int64
	SessionID int64
	PlayerID  int64
	Name      string
	Gender    Gender
	Level     Level
	IsRegular bool

	Status          PlayerStatus
	MatchesPlayed   int
	PriorityOffset  int
	ConsecutiveRest int
	QueuedAt        time.Time
	CheckedInAt     time.Time
	FeeOverride     *int64
	IsPaid          bool
}

// PriorityCount is the match count used for fairness. MatchesPlayed stays
// the real count; the offset keeps late arrivals from jumping the queue.
func (p *SessionPlayer) PriorityCount() int { return p.MatchesPlayed + p.PriorityOffset }

// CheckInOffset is the priority offset for someone joining mid-session:
// the lowest priority count among players present (not resting).
func CheckInOffset(present []SessionPlayer) int {
	offset, found := 0, false
	for i := range present {
		if present[i].Status == PlayerResting {
			continue
		}
		if c := present[i].PriorityCount(); !found || c < offset {
			offset, found = c, true
		}
	}
	return offset
}

// StartMatch moves a waiting player onto a court.
func (p *SessionPlayer) StartMatch() error {
	if p.Status != PlayerWaiting {
		return ErrPlayerNotWaiting
	}
	p.Status = PlayerPlaying
	p.ConsecutiveRest = 0
	return nil
}

// SkipMatch records that a match started without this player. Only players
// waiting in the queue accumulate rest.
func (p *SessionPlayer) SkipMatch() {
	if p.Status == PlayerWaiting {
		p.ConsecutiveRest++
	}
}

// FinishMatch returns a playing player to the queue and counts the match.
func (p *SessionPlayer) FinishMatch(now time.Time) error {
	if p.Status != PlayerPlaying {
		return ErrInvalidTransition
	}
	p.Status = PlayerWaiting
	p.MatchesPlayed++
	p.QueuedAt = now
	return nil
}

// Rest takes a waiting player out of the queue (also used for leaving early).
func (p *SessionPlayer) Rest() error {
	switch p.Status {
	case PlayerPlaying:
		return ErrPlayerPlaying
	case PlayerResting:
		return ErrInvalidTransition
	}
	p.Status = PlayerResting
	return nil
}

// Resume puts a resting player back at the end of the queue.
func (p *SessionPlayer) Resume(now time.Time) error {
	if p.Status != PlayerResting {
		return ErrInvalidTransition
	}
	p.Status = PlayerWaiting
	p.QueuedAt = now
	return nil
}

// Fee is what this player owes for the session: a manual override wins,
// regular players pay nothing per session, casual players pay by gender.
func (p *SessionPlayer) Fee(s *Session) int64 {
	if p.FeeOverride != nil {
		return *p.FeeOverride
	}
	if p.IsRegular {
		return 0
	}
	if p.Gender == GenderFemale {
		return s.FeeFemale
	}
	return s.FeeMale
}
