package domain

import (
	"net/url"
	"strings"
)

// VideoID returns the YouTube video id in a watch or short link, or "" when the
// URL is not a single-video YouTube link.
func VideoID(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	switch host {
	case "youtube.com", "m.youtube.com", "music.youtube.com":
		if u.Path == "/watch" {
			return u.Query().Get("v")
		}
	case "youtu.be":
		return strings.Trim(u.Path, "/")
	}
	return ""
}

// WantedVideo is one video the playlist should hold, in playlist order.
type WantedVideo struct {
	VideoID string
	Title   string
}

// PlaylistEntry is one item currently in the playlist. ItemID is YouTube's id for
// the playlist item (needed to delete or move it); VideoID is the video.
type PlaylistEntry struct {
	ItemID  string
	VideoID string
	Title   string
}

// StepKind says how a step changes the playlist.
type StepKind string

const (
	StepAdd  StepKind = "add"
	StepMove StepKind = "move"
)

// PlaylistStep is one ordered change: add a video at Position, or move an existing
// item to it. Steps are applied in order, after the removals.
type PlaylistStep struct {
	Kind     StepKind
	VideoID  string
	Title    string
	ItemID   string // set for moves
	Position int
}

// PlaylistPlan is what has to change to turn the current playlist into the wanted one.
type PlaylistPlan struct {
	// Remove lists items to delete: videos no longer wanted, and duplicate copies.
	Remove []PlaylistEntry
	// Steps adds missing videos and fixes order, applied in sequence.
	Steps []PlaylistStep
}

// Empty reports whether the playlist already matches.
func (p PlaylistPlan) Empty() bool { return len(p.Remove) == 0 && len(p.Steps) == 0 }

// PlanPlaylist computes the minimal ordered changes that make current equal to
// desired. It is pure: no network, no clock. A video wanted twice is wanted once;
// an item present twice keeps its first copy and the rest are removed. Only items
// passed in as current can ever be removed.
func PlanPlaylist(desired []WantedVideo, current []PlaylistEntry) PlaylistPlan {
	var plan PlaylistPlan

	want := make([]WantedVideo, 0, len(desired))
	wantSet := map[string]bool{}
	for _, d := range desired {
		if d.VideoID == "" || wantSet[d.VideoID] {
			continue
		}
		wantSet[d.VideoID] = true
		want = append(want, d)
	}

	// kept is the playlist after removals, in its current order.
	var kept []PlaylistEntry
	have := map[string]bool{}
	for _, c := range current {
		if wantSet[c.VideoID] && !have[c.VideoID] {
			have[c.VideoID] = true
			kept = append(kept, c)
			continue
		}
		plan.Remove = append(plan.Remove, c)
	}

	// Walk the target order, simulating the playlist so positions are right when applied.
	for i, w := range want {
		if i < len(kept) && kept[i].VideoID == w.VideoID {
			continue
		}
		at := -1
		for j, k := range kept {
			if k.VideoID == w.VideoID {
				at = j
				break
			}
		}
		if at < 0 {
			plan.Steps = append(plan.Steps, PlaylistStep{Kind: StepAdd, VideoID: w.VideoID, Title: w.Title, Position: i})
			kept = insertEntry(kept, i, PlaylistEntry{VideoID: w.VideoID, Title: w.Title})
			continue
		}
		entry := kept[at]
		kept = append(kept[:at], kept[at+1:]...)
		kept = insertEntry(kept, i, entry)
		plan.Steps = append(plan.Steps, PlaylistStep{Kind: StepMove, VideoID: w.VideoID, Title: w.Title, ItemID: entry.ItemID, Position: i})
	}
	return plan
}

func insertEntry(s []PlaylistEntry, i int, e PlaylistEntry) []PlaylistEntry {
	if i > len(s) {
		i = len(s)
	}
	s = append(s, PlaylistEntry{})
	copy(s[i+1:], s[i:])
	s[i] = e
	return s
}
