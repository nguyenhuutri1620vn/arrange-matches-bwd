package domain

// Level is a player's skill tier. The stored value is the display code.
type Level string

const (
	LevelY       Level = "Y"
	LevelYPlus   Level = "Y+"
	LevelTBY     Level = "TBY"
	LevelTBMinus Level = "TB-"
	LevelTB      Level = "TB"
	LevelTBPlus  Level = "TB+"
)

var levelScores = map[Level]int{
	LevelY:       1,
	LevelYPlus:   2,
	LevelTBY:     3,
	LevelTBMinus: 4,
	LevelTB:      5,
	LevelTBPlus:  6,
}

// Levels returns all levels from weakest to strongest.
func Levels() []Level {
	return []Level{LevelY, LevelYPlus, LevelTBY, LevelTBMinus, LevelTB, LevelTBPlus}
}

// Score is the numeric strength used for balancing. Invalid levels score 0.
func (l Level) Score() int { return levelScores[l] }

func (l Level) Valid() bool {
	_, ok := levelScores[l]
	return ok
}

type Gender string

const (
	GenderMale   Gender = "M"
	GenderFemale Gender = "F"
)

func (g Gender) Valid() bool { return g == GenderMale || g == GenderFemale }
