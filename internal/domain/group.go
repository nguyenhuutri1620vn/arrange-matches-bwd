package domain

import "time"

// GroupPhase is where a group of players who came together stands in its
// rhythm: a match together, then matches apart, then together again.
type GroupPhase string

const (
	GroupTogether GroupPhase = "TOGETHER"
	GroupSplit    GroupPhase = "SPLIT"
)

func (p GroupPhase) Valid() bool { return p == GroupTogether || p == GroupSplit }

// SessionGroup is a group of players who came together to one session.
type SessionGroup struct {
	ID        int64
	SessionID int64
	// No is the number the host sees ("Nhóm 1").
	No             int
	Phase          GroupPhase
	PhaseStartedAt time.Time
	CreatedAt      time.Time
}

// GroupHasRhythm reports whether a group of this size alternates between
// playing together and apart. A group of three instead rotates which pair
// teams up, and a lone member is no group at all.
func GroupHasRhythm(size int) bool { return size == 2 || size >= 4 }

// GroupTogetherCount is how many members play a group's together match: a
// pair, or four of a larger group, since four fill a court. A group of
// five leaves one member to another court.
func GroupTogetherCount(size int) int { return min(size, 4) }

// NextGroupPhase is the phase after a match with some of the group's
// members is confirmed. onCourt counts the members in that match, teamed
// tells whether two of them share a team, and splitDone whether every
// member has started a match since the phase began, this one included.
//
// A pair is together when they team up; a group of four or five when four
// of its members share the court. The split phase lasts until everyone has played apart.
func NextGroupPhase(phase GroupPhase, size, onCourt int, teamed, splitDone bool) GroupPhase {
	if !GroupHasRhythm(size) {
		return phase
	}
	switch phase {
	case GroupTogether:
		if (size == 2 && onCourt == 2 && teamed) || (size >= 4 && onCourt == 4) {
			return GroupSplit
		}
	case GroupSplit:
		if splitDone {
			return GroupTogether
		}
	}
	return phase
}
