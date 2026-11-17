package domain

import "testing"

func TestNextGroupPhase(t *testing.T) {
	cases := []struct {
		name      string
		phase     GroupPhase
		size      int
		onCourt   int
		teamed    bool
		splitDone bool
		want      GroupPhase
	}{
		{"pair teamed ends together", GroupTogether, 2, 2, true, false, GroupSplit},
		{"pair facing stays together", GroupTogether, 2, 2, false, false, GroupTogether},
		{"pair one on court stays together", GroupTogether, 2, 1, false, false, GroupTogether},
		{"pair split until both played", GroupSplit, 2, 1, false, false, GroupSplit},
		{"pair split done", GroupSplit, 2, 1, false, true, GroupTogether},
		{"four whole ends together", GroupTogether, 4, 4, true, false, GroupSplit},
		{"four pair stays together", GroupTogether, 4, 2, true, false, GroupTogether},
		{"four split done", GroupSplit, 4, 2, true, true, GroupTogether},
		{"four split not done", GroupSplit, 4, 2, true, false, GroupSplit},
		{"three has no rhythm", GroupTogether, 3, 3, true, true, GroupTogether},
		{"alone has no rhythm", GroupSplit, 1, 1, false, true, GroupSplit},
	}
	for _, c := range cases {
		if got := NextGroupPhase(c.phase, c.size, c.onCourt, c.teamed, c.splitDone); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}

func TestGroupHasRhythm(t *testing.T) {
	for size, want := range map[int]bool{1: false, 2: true, 3: false, 4: true} {
		if got := GroupHasRhythm(size); got != want {
			t.Errorf("size %d: got %v, want %v", size, got, want)
		}
	}
}
