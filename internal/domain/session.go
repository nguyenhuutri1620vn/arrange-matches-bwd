package domain

import "time"

type SessionStatus string

const (
	SessionOpen  SessionStatus = "OPEN"
	SessionEnded SessionStatus = "ENDED"
)

// Session is one play session. Date is the Vietnam-local calendar day
// (YYYY-MM-DD); StartTime and EndTime are UTC instants.
type Session struct {
	ID         int64
	Date       string
	StartTime  *time.Time
	EndTime    *time.Time
	Status     SessionStatus
	CourtCount int
	FeeMale    int64
	FeeFemale  int64
	CourtFee   int64
}

func ValidCourtCount(n int) bool { return n >= MinCourts && n <= MaxCourts }

type CourtStatus string

const (
	CourtEmpty CourtStatus = "EMPTY"
	CourtInUse CourtStatus = "IN_USE"
)

type Court struct {
	ID             int64
	SessionID      int64
	Index          int
	Status         CourtStatus
	CurrentMatchID *int64
}

type MatchStatus string

const (
	MatchDraft    MatchStatus = "DRAFT"
	MatchOngoing  MatchStatus = "ONGOING"
	MatchFinished MatchStatus = "FINISHED"
)

type Team string

const (
	TeamA Team = "A"
	TeamB Team = "B"
)

type MatchPlayer struct {
	SessionPlayerID int64
	Team            Team
}

type Match struct {
	ID         int64
	SessionID  int64
	CourtID    int64
	Status     MatchStatus
	StartedAt  *time.Time
	FinishedAt *time.Time
	Players    []MatchPlayer
}

// SessionFees sums the fees of a session's players.
type SessionFees struct {
	Expected  int64
	Collected int64
}

func (f SessionFees) Outstanding() int64 { return f.Expected - f.Collected }

func SumSessionFees(s *Session, players []SessionPlayer) SessionFees {
	var f SessionFees
	for i := range players {
		fee := players[i].Fee(s)
		f.Expected += fee
		if players[i].IsPaid {
			f.Collected += fee
		}
	}
	return f
}
