package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/costap/stargazer/internal/config"
	"github.com/costap/stargazer/internal/domain"
	"github.com/costap/stargazer/internal/youtube"
)

// memPlaylist is an in-memory YouTube: the same contract, no network.
type memPlaylist struct {
	items   []domain.PlaylistEntry
	created int
	deletes int
	adds    int
	moves   int
	n       int
}

func (m *memPlaylist) CreatePlaylist(_ context.Context, _, _ string) (string, error) {
	m.created++
	return "PLX", nil
}
func (m *memPlaylist) ListItems(_ context.Context, _ string) ([]domain.PlaylistEntry, error) {
	return append([]domain.PlaylistEntry(nil), m.items...), nil
}
func (m *memPlaylist) AddItem(_ context.Context, _, vid string, pos int) error {
	m.adds++
	m.n++
	e := domain.PlaylistEntry{ItemID: "i" + strconv.Itoa(m.n), VideoID: vid}
	m.items = append(m.items[:pos], append([]domain.PlaylistEntry{e}, m.items[pos:]...)...)
	return nil
}
func (m *memPlaylist) DeleteItem(_ context.Context, id string) error {
	m.deletes++
	for i, e := range m.items {
		if e.ItemID == id {
			m.items = append(m.items[:i], m.items[i+1:]...)
		}
	}
	return nil
}
func (m *memPlaylist) MoveItem(_ context.Context, _, id, _ string, pos int) error {
	m.moves++
	for i, e := range m.items {
		if e.ItemID == id {
			m.items = append(m.items[:i], m.items[i+1:]...)
			m.items = append(m.items[:pos], append([]domain.PlaylistEntry{e}, m.items[pos:]...)...)
		}
	}
	return nil
}

func (m *memPlaylist) videos() string {
	var s []string
	for _, e := range m.items {
		s = append(s, e.VideoID)
	}
	return strings.Join(s, ",")
}

func playlistEngine(t *testing.T, heard string) (*Engine, string, string) {
	t.Helper()
	dir := t.TempDir()
	sig := filepath.Join(dir, "signals.json")
	payload := domain.SignalsPayload{Signals: []domain.SignalItem{
		{ID: "0001", Title: "One", Date: "2026-10-01", URL: "https://www.youtube.com/watch?v=V1", Status: "spotted"},
		{ID: "0002", Title: "Two", Date: "2026-10-02", URL: "https://www.youtube.com/watch?v=V2", Status: "spotted"},
		{ID: "0003", Title: "Three", Date: "2026-10-03", URL: "https://www.youtube.com/watch?v=V3", Status: "spotted"},
	}}
	b, _ := json.Marshal(payload)
	if err := os.WriteFile(sig, b, 0o600); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(dir, "state.yaml")
	if err := os.WriteFile(statePath, []byte("heard: ["+heard+"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var cfg config.Config
	cfg.Signals.Source = sig
	cfg.State.File = statePath
	cfg.State.Explicit = true
	cfg.YouTube.PlaylistFile = filepath.Join(dir, "yt.json")
	cfg.YouTube.Playlist.Title = "Stargazer queue"
	cfg.YouTube.Playlist.Cap = 25
	return NewEngine(&cfg), statePath, dir
}

func TestPlaylistSyncCreatesFillsAndThenDoesNothing(t *testing.T) {
	e, _, _ := playlistEngine(t, "")
	api := &memPlaylist{}
	get := func() (PlaylistAPI, error) { return api, nil }
	var out bytes.Buffer

	if _, err := e.SyncPlaylist(context.Background(), PlaylistOptions{}, get, &out); err != nil {
		t.Fatal(err)
	}
	if api.created != 1 || api.videos() != "V3,V2,V1" {
		t.Fatalf("first run: created=%d playlist=%s\n%s", api.created, api.videos(), out.String())
	}

	api.adds, api.deletes, api.moves = 0, 0, 0
	out.Reset()
	if _, err := e.SyncPlaylist(context.Background(), PlaylistOptions{}, get, &out); err != nil {
		t.Fatal(err)
	}
	if api.created != 1 || api.adds+api.deletes+api.moves != 0 {
		t.Errorf("a second run with nothing changed must do nothing: %+v\n%s", api, out.String())
	}
	if !strings.Contains(out.String(), "already matches") {
		t.Errorf("say so: %s", out.String())
	}
}

func TestPlaylistSyncDropsWhatYouMarkHeard(t *testing.T) {
	e, statePath, _ := playlistEngine(t, "")
	api := &memPlaylist{}
	get := func() (PlaylistAPI, error) { return api, nil }
	var out bytes.Buffer
	if _, err := e.SyncPlaylist(context.Background(), PlaylistOptions{}, get, &out); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(statePath, []byte("heard: ['0003']\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if _, err := e.SyncPlaylist(context.Background(), PlaylistOptions{}, get, &out); err != nil {
		t.Fatal(err)
	}
	if api.videos() != "V2,V1" || api.deletes != 1 {
		t.Errorf("0003 heard: want V2,V1 with one delete, got %s deletes=%d", api.videos(), api.deletes)
	}
}

func TestPlaylistDryRunChangesNothingAndNeedsNoLoginBeforeTheFirstSync(t *testing.T) {
	e, _, dir := playlistEngine(t, "")
	called := false
	get := func() (PlaylistAPI, error) { called = true; return nil, os.ErrPermission }
	var out bytes.Buffer

	if _, err := e.SyncPlaylist(context.Background(), PlaylistOptions{DryRun: true}, get, &out); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("a dry run before the first sync has nothing to compare with, so it must not log in")
	}
	if _, err := os.Stat(filepath.Join(dir, "yt.json")); err == nil {
		t.Error("a dry run must not write the playlist record")
	}
	if !strings.Contains(out.String(), "Would create") || !strings.Contains(out.String(), "Dry run") {
		t.Errorf("a dry run says what it would do: %s", out.String())
	}

	// With a recorded playlist, a dry run reads it but writes nothing.
	api := &memPlaylist{}
	if _, err := e.SyncPlaylist(context.Background(), PlaylistOptions{}, func() (PlaylistAPI, error) { return api, nil }, &out); err != nil {
		t.Fatal(err)
	}
	api.items = api.items[:1] // someone removed two items by hand
	before := api.videos()
	api.adds, api.deletes, api.moves = 0, 0, 0
	if _, err := e.SyncPlaylist(context.Background(), PlaylistOptions{DryRun: true}, func() (PlaylistAPI, error) { return api, nil }, &out); err != nil {
		t.Fatal(err)
	}
	if api.videos() != before || api.adds+api.deletes+api.moves != 0 {
		t.Errorf("dry run changed the playlist: %s %+v", api.videos(), api)
	}
}

func TestPlaylistSyncNamesAPlaylistDeletedByHand(t *testing.T) {
	e, _, dir := playlistEngine(t, "")
	if err := os.WriteFile(filepath.Join(dir, "yt.json"), []byte(`{"id":"GONE"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	get := func() (PlaylistAPI, error) { return &notFoundAPI{}, nil }
	_, err := e.SyncPlaylist(context.Background(), PlaylistOptions{}, get, &out)
	if err == nil || !strings.Contains(err.Error(), "no longer exists") || !strings.Contains(err.Error(), "yt.json") {
		t.Errorf("want a message naming the record to remove, got %v", err)
	}
}

type notFoundAPI struct{ memPlaylist }

func (*notFoundAPI) ListItems(context.Context, string) ([]domain.PlaylistEntry, error) {
	return nil, &youtube.APIError{Status: 404, Reason: "playlistNotFound", Op: "list"}
}

func TestPlaylistSyncRefusesWithoutSignals(t *testing.T) {
	var cfg config.Config
	_, err := NewEngine(&cfg).SyncPlaylist(context.Background(), PlaylistOptions{}, nil, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "signals") {
		t.Errorf("want a signals hint, got %v", err)
	}
}

// deadVideoAPI refuses one video as YouTube does for a deleted or private one.
type deadVideoAPI struct {
	memPlaylist
	dead     string
	deadTrys int
}

func (d *deadVideoAPI) AddItem(ctx context.Context, pid, vid string, pos int) error {
	if vid == d.dead {
		d.deadTrys++
		return &youtube.APIError{Status: 404, Reason: "videoNotFound", Op: "add item", Message: "Video not found."}
	}
	return d.memPlaylist.AddItem(ctx, pid, vid, pos)
}

func TestPlaylistSyncSkipsAnUnavailableVideoRemembersItAndBackfills(t *testing.T) {
	e, _, dir := playlistEngine(t, "")
	e.cfg.YouTube.Playlist.Cap = 2 // wants V3,V2; V3 is dead, so V1 must take the slot
	api := &deadVideoAPI{dead: "V3"}
	get := func() (PlaylistAPI, error) { return api, nil }
	var out bytes.Buffer

	if _, err := e.SyncPlaylist(context.Background(), PlaylistOptions{}, get, &out); err != nil {
		t.Fatalf("one dead video must not fail the sync: %v\n%s", err, out.String())
	}
	if api.videos() != "V2,V1" {
		t.Errorf("want V2,V1 (V3 skipped, V1 backfilled), got %s\n%s", api.videos(), out.String())
	}
	if !strings.Contains(out.String(), "V3 is not available") {
		t.Errorf("name the skipped video: %s", out.String())
	}
	rec, _ := os.ReadFile(filepath.Join(dir, "yt.json"))
	if !strings.Contains(string(rec), `"V3"`) {
		t.Errorf("the dead video must be recorded: %s", rec)
	}

	// The next run must not try V3 again.
	api.deadTrys = 0
	out.Reset()
	if _, err := e.SyncPlaylist(context.Background(), PlaylistOptions{}, get, &out); err != nil {
		t.Fatal(err)
	}
	if api.deadTrys != 0 {
		t.Errorf("a recorded unavailable video must never be retried (%d tries)", api.deadTrys)
	}
	if !strings.Contains(out.String(), "already matches") {
		t.Errorf("second run should be a no-op: %s", out.String())
	}
}

func TestPlaylistSyncStillFailsOnOtherErrors(t *testing.T) {
	e, _, _ := playlistEngine(t, "")
	api := &quotaAPI{}
	_, err := e.SyncPlaylist(context.Background(), PlaylistOptions{}, func() (PlaylistAPI, error) { return api, nil }, &bytes.Buffer{})
	var ae *youtube.APIError
	if !errors.As(err, &ae) || !ae.QuotaExceeded() {
		t.Errorf("a quota error must stop the sync loudly, got %v", err)
	}
}

type quotaAPI struct{ memPlaylist }

func (*quotaAPI) AddItem(context.Context, string, string, int) error {
	return &youtube.APIError{Status: 403, Reason: "quotaExceeded", Op: "add item"}
}

// goneAPI refuses to delete an item that is already gone, as YouTube does when the
// listing lags a delete that just succeeded.
type goneAPI struct{ memPlaylist }

func (g *goneAPI) DeleteItem(context.Context, string) error {
	return &youtube.APIError{Status: 404, Reason: "playlistItemNotFound", Op: "delete item", Message: "Playlist item not found."}
}

func TestRemoveOfAnAlreadyGoneItemIsNotAFailure(t *testing.T) {
	api := &goneAPI{}
	plan := domain.PlaylistPlan{Remove: []domain.PlaylistEntry{{ItemID: "i1", VideoID: "v1", Title: "Heard talk"}}}
	if _, err := applyPlaylistPlan(context.Background(), api, "PLX", plan); err != nil {
		t.Fatalf("an item already removed must not fail the sync: %v", err)
	}
}

func TestOtherDeleteFailuresStillFail(t *testing.T) {
	api := &quotaDeleteAPI{}
	plan := domain.PlaylistPlan{Remove: []domain.PlaylistEntry{{ItemID: "i1", VideoID: "v1", Title: "Heard talk"}}}
	if _, err := applyPlaylistPlan(context.Background(), api, "PLX", plan); err == nil {
		t.Fatal("a quota error on delete must still fail the sync")
	}
}

type quotaDeleteAPI struct{ memPlaylist }

func (*quotaDeleteAPI) DeleteItem(context.Context, string) error {
	return &youtube.APIError{Status: 403, Reason: "quotaExceeded", Op: "delete item"}
}
