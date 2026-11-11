package domain

import (
	"reflect"
	"testing"
)

func TestVideoID(t *testing.T) {
	cases := map[string]string{
		"https://www.youtube.com/watch?v=AAA":       "AAA",
		"https://youtube.com/watch?v=BBB&t=30s":     "BBB",
		"https://youtu.be/CCC":                      "CCC",
		"https://music.youtube.com/watch?v=DDD":     "DDD",
		"https://www.youtube.com/playlist?list=PL1": "",
		"https://www.youtube.com/@channel":          "",
		"https://example.com/watch?v=EEE":           "",
		"not a url":                                 "",
		"":                                          "",
	}
	for in, want := range cases {
		if got := VideoID(in); got != want {
			t.Errorf("VideoID(%q) = %q, want %q", in, got, want)
		}
	}
}

func wanted(ids ...string) []WantedVideo {
	var w []WantedVideo
	for _, id := range ids {
		w = append(w, WantedVideo{VideoID: id, Title: "t-" + id})
	}
	return w
}

func current(ids ...string) []PlaylistEntry {
	var c []PlaylistEntry
	for _, id := range ids {
		c = append(c, PlaylistEntry{ItemID: "item-" + id, VideoID: id, Title: "t-" + id})
	}
	return c
}

// apply runs a plan against a playlist the way the API would, so each case can check
// the end state and not just the shape of the steps.
func apply(cur []PlaylistEntry, p PlaylistPlan) []string {
	list := append([]PlaylistEntry(nil), cur...)
	for _, r := range p.Remove {
		for i, e := range list {
			if e.ItemID == r.ItemID {
				list = append(list[:i], list[i+1:]...)
				break
			}
		}
	}
	for _, s := range p.Steps {
		switch s.Kind {
		case StepAdd:
			list = insertEntry(list, s.Position, PlaylistEntry{ItemID: "new-" + s.VideoID, VideoID: s.VideoID})
		case StepMove:
			for i, e := range list {
				if e.ItemID == s.ItemID {
					list = append(list[:i], list[i+1:]...)
					list = insertEntry(list, s.Position, e)
					break
				}
			}
		}
	}
	var out []string
	for _, e := range list {
		out = append(out, e.VideoID)
	}
	return out
}

func TestPlanPlaylistReachesTheWantedPlaylist(t *testing.T) {
	cases := []struct {
		name       string
		cur, want  []string
		wantRemove int
		wantSteps  int
	}{
		{"already equal", []string{"a", "b", "c"}, []string{"a", "b", "c"}, 0, 0},
		{"empty to full", nil, []string{"a", "b", "c"}, 0, 3},
		{"drop one", []string{"a", "b", "c"}, []string{"a", "c"}, 1, 0},
		{"drop all", []string{"a", "b"}, nil, 2, 0},
		{"new one at the front", []string{"a", "b"}, []string{"n", "a", "b"}, 0, 1},
		{"swap two", []string{"a", "b", "c"}, []string{"b", "a", "c"}, 0, 1},
		{"reverse", []string{"a", "b", "c", "d"}, []string{"d", "c", "b", "a"}, 0, 3},
		{"mixed", []string{"a", "x", "b", "y"}, []string{"b", "n", "a"}, 2, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cur := current(c.cur...)
			plan := PlanPlaylist(wanted(c.want...), cur)
			if len(plan.Remove) != c.wantRemove {
				t.Errorf("removes: want %d, got %+v", c.wantRemove, plan.Remove)
			}
			if len(plan.Steps) != c.wantSteps {
				t.Errorf("steps: want %d, got %+v", c.wantSteps, plan.Steps)
			}
			got := apply(cur, plan)
			if !reflect.DeepEqual(got, append([]string(nil), c.want...)) && !(len(got) == 0 && len(c.want) == 0) {
				t.Errorf("applying the plan gives %v, want %v", got, c.want)
			}
			if plan.Empty() != (c.wantRemove == 0 && c.wantSteps == 0) {
				t.Error("Empty() disagrees with the plan")
			}
		})
	}
}

func TestPlanPlaylistHandlesDuplicates(t *testing.T) {
	cur := []PlaylistEntry{
		{ItemID: "i1", VideoID: "a"},
		{ItemID: "i2", VideoID: "a"},
		{ItemID: "i3", VideoID: "b"},
	}
	plan := PlanPlaylist([]WantedVideo{{VideoID: "a"}, {VideoID: "a"}, {VideoID: "b"}, {VideoID: ""}}, cur)
	if len(plan.Remove) != 1 || plan.Remove[0].ItemID != "i2" {
		t.Errorf("the second copy of a is the only removal: %+v", plan.Remove)
	}
	if len(plan.Steps) != 0 {
		t.Errorf("order is already right: %+v", plan.Steps)
	}
}

func TestPlanPlaylistOnlyRemovesWhatItWasGiven(t *testing.T) {
	cur := current("a", "b")
	plan := PlanPlaylist(wanted("a"), cur)
	for _, r := range plan.Remove {
		if r.ItemID != "item-b" {
			t.Errorf("removed an item that was not in current: %+v", r)
		}
	}
}
