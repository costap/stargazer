package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/costap/stargazer/internal/domain"
)

// BuildDesiredPlaylist decides which videos the playlist should hold, in order.
//
// Signals come first, newest first, and only while their status is "spotted": a
// signal you have moved on from should not keep a slot. Digest "Watch These First"
// links follow in digest order. Talks listed under heard: in the state file are
// skipped, ids in skip (videos YouTube refused) are never wanted, a video appears once, and the list is cut to cap (cap <= 0 means no cut).
// The notices name digests that carry no such section, because a changed heading
// in the signals export would otherwise silently empty that half of the queue.
func BuildDesiredPlaylist(payload *domain.SignalsPayload, state *domain.DispatchState, cap int, skip []string) ([]domain.WantedVideo, []string) {
	if payload == nil {
		return nil, nil
	}
	var out []domain.WantedVideo
	var notices []string
	seen := map[string]bool{}
	for _, id := range skip {
		seen[id] = true // known unavailable: never wanted, and the cap fills from the next talk
	}

	add := func(signalID, rawURL, title string) {
		id := domain.VideoID(rawURL)
		if id == "" || seen[id] || state.IsHeard(signalID, id) {
			return
		}
		seen[id] = true
		out = append(out, domain.WantedVideo{VideoID: id, Title: title})
	}

	signals := append([]domain.SignalItem(nil), payload.Signals...)
	sort.SliceStable(signals, func(i, j int) bool {
		if signals[i].Date != signals[j].Date {
			return signals[i].Date > signals[j].Date
		}
		return signals[i].ID > signals[j].ID
	})
	for _, s := range signals {
		if !strings.EqualFold(strings.TrimSpace(s.Status), "spotted") {
			continue
		}
		add(s.ID, s.URL, s.Title)
	}

	for _, d := range payload.Digests {
		if !strings.Contains(d.RawContent, "## Watch These First") {
			notices = append(notices, fmt.Sprintf("digest %q has no \"## Watch These First\" section, so it adds nothing to the playlist", d.Slug))
			continue
		}
		for _, link := range extractWatchFirstLinks(d.RawContent) {
			add("", link.URL, link.Title)
		}
	}

	if cap > 0 && len(out) > cap {
		out = out[:cap]
	}
	return out, notices
}
