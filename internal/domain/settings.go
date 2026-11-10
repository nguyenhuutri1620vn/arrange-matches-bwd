package domain

const (
	MinCourts = 1
	MaxCourts = 3
)

// MatchRules are the tunable thresholds of the matchmaking rules.
type MatchRules struct {
	// GenderGapThreshold: in a 3-1 lineup, the average level of the three
	// must exceed the level of the odd one by at least this much.
	GenderGapThreshold int
	// BalanceThreshold: max allowed difference of team level sums.
	BalanceThreshold int
	// RepeatLookback: how many of a player's latest matches count when
	// avoiding repeated teammates and opponents.
	RepeatLookback int
}

// Settings are the app-wide defaults; fees prefill a new session.
type Settings struct {
	FeeMale   int64
	FeeFemale int64
	CourtFee  int64
	Rules     MatchRules
}

// DefaultSettings is the single source of the default fees and thresholds.
func DefaultSettings() Settings {
	return Settings{
		FeeMale:   70000,
		FeeFemale: 60000,
		CourtFee:  0,
		Rules: MatchRules{
			GenderGapThreshold: 2,
			BalanceThreshold:   2,
			RepeatLookback:     1,
		},
	}
}

func (s Settings) Validate() error {
	if s.FeeMale < 0 || s.FeeFemale < 0 || s.CourtFee < 0 {
		return ErrInvalidAmount
	}
	r := s.Rules
	if r.GenderGapThreshold < 0 || r.BalanceThreshold < 0 || r.RepeatLookback < 0 {
		return ErrInvalidInput
	}
	return nil
}
