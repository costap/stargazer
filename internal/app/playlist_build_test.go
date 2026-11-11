package app

import (
	"strings"
	"testing"

	"github.com/costap/stargazer/internal/domain"
)

func payloadForPlaylist() *domain.SignalsPayload {
	return &domain.SignalsPayload{
		Signals: []domain.SignalItem{
			{ID: "0001", Title: "Old", Date: "2026-10-01", URL: "https://www.youtube.com/watch?v=OLD", Status: "spotted"},
			{ID: "0003", Title: "New", Date: "2026-11-02", URL: "https://www.youtube.com/watch?v=NEW", Status: "spotted"},
			{ID: "0002", Title: "Moved on", Date: "2026-10-20", URL: "https://www.youtube.com/watch?v=GONE", Status: "adopted"},
			{ID: "0004", Title: "Article", Date: "2026-11-01", URL: "https://example.com/post", Status: "spotted"},
		},
		Digests: []domain.SignalDigest{
			{Slug: "conf", RawContent: "## Watch These First\n\n1. **[Digest talk](https://www.youtube.com/watch?v=DIG)**  \n   why\n\n1. **[Old again](https://youtu.be/OLD)**\n\n## Other\n"},
			{Slug: "nosection", RawContent: "# Digest\n\nnothing here\n"},
		},
	}
}

func ids(w []domain.WantedVideo) string {
	var s []string
	for _, v := range w {
		s = append(s, v.VideoID)
	}
	return strings.Join(s, ",")
}

func TestBuildDesiredPlaylistOrdersNewestSignalsThenDigests(t *testing.T) {
	got, notices := BuildDesiredPlaylist(payloadForPlaylist(), nil, 0, nil)
	if ids(got) != "NEW,OLD,DIG" {
		t.Errorf("want NEW,OLD,DIG (newest signal first, a status that moved on skipped, a repeat once), got %s", ids(got))
	}
	if len(notices) != 1 || !strings.Contains(notices[0], "nosection") {
		t.Errorf("a digest with no Watch These First section must be named: %q", notices)
	}
}

func TestBuildDesiredPlaylistSkipsHeardBySignalIDOrVideoID(t *testing.T) {
	st := &domain.DispatchState{Heard: []string{"0003", "dig"}}
	got, _ := BuildDesiredPlaylist(payloadForPlaylist(), st, 0, nil)
	if ids(got) != "OLD" {
		t.Errorf("0003 (signal id) and DIG (video id, any case) are heard; got %s", ids(got))
	}
}

func TestBuildDesiredPlaylistCapsTheList(t *testing.T) {
	got, _ := BuildDesiredPlaylist(payloadForPlaylist(), nil, 2, nil)
	if ids(got) != "NEW,OLD" {
		t.Errorf("cap 2 keeps the first two, got %s", ids(got))
	}
	if g, _ := BuildDesiredPlaylist(nil, nil, 5, nil); g != nil {
		t.Error("no payload, no playlist")
	}
}

func TestBuildDesiredPlaylistLeavesOutUnavailableVideosAndBackfills(t *testing.T) {
	got, _ := BuildDesiredPlaylist(payloadForPlaylist(), nil, 2, []string{"NEW"})
	if ids(got) != "OLD,DIG" {
		t.Errorf("NEW is unavailable: want OLD,DIG (the cap fills from the next talk), got %s", ids(got))
	}
}
