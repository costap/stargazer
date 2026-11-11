package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// fakeAPI is an in-memory playlist behind the real HTTP shapes.
type fakeAPI struct {
	items    []fakeItem
	next     int
	pageSize int
	calls    []string
	failWith *struct {
		status int
		body   string
	}
}

type fakeItem struct{ ID, Video, Title string }

func (f *fakeAPI) handler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls = append(f.calls, r.Method+" "+r.URL.Path)
		if f.failWith != nil {
			w.WriteHeader(f.failWith.status)
			_, _ = w.Write([]byte(f.failWith.body))
			return
		}
		switch {
		case r.URL.Path == "/playlists" && r.Method == http.MethodPost:
			var b struct {
				Snippet struct{ Title string }
				Status  struct{ PrivacyStatus string }
			}
			_ = json.NewDecoder(r.Body).Decode(&b)
			if b.Status.PrivacyStatus != "private" {
				t.Errorf("a playlist must be created private, got %q", b.Status.PrivacyStatus)
			}
			_, _ = w.Write([]byte(`{"id":"PL123"}`))
		case r.URL.Path == "/playlistItems" && r.Method == http.MethodGet:
			start := 0
			if tok := r.URL.Query().Get("pageToken"); tok != "" {
				start, _ = strconv.Atoi(tok)
			}
			end := start + f.pageSize
			if f.pageSize == 0 || end > len(f.items) {
				end = len(f.items)
			}
			var rows []string
			// Position is deliberately reported reversed-friendly: the client must sort by it.
			for i := start; i < end; i++ {
				it := f.items[i]
				rows = append(rows, `{"id":"`+it.ID+`","snippet":{"position":`+strconv.Itoa(i)+`,"title":"`+it.Title+`","resourceId":{"videoId":"`+it.Video+`"}}}`)
			}
			out := `{"items":[` + strings.Join(rows, ",") + `]`
			if end < len(f.items) {
				out += `,"nextPageToken":"` + strconv.Itoa(end) + `"`
			}
			_, _ = w.Write([]byte(out + "}"))
		case r.URL.Path == "/playlistItems" && r.Method == http.MethodPost:
			var b struct {
				Snippet struct {
					PlaylistID string
					Position   int
					ResourceID struct{ VideoId string }
				}
			}
			_ = json.NewDecoder(r.Body).Decode(&b)
			f.next++
			it := fakeItem{ID: "item" + strconv.Itoa(f.next), Video: b.Snippet.ResourceID.VideoId}
			p := b.Snippet.Position
			f.items = append(f.items[:p], append([]fakeItem{it}, f.items[p:]...)...)
			_, _ = w.Write([]byte(`{}`))
		case r.URL.Path == "/playlistItems" && r.Method == http.MethodDelete:
			id := r.URL.Query().Get("id")
			for i, it := range f.items {
				if it.ID == id {
					f.items = append(f.items[:i], f.items[i+1:]...)
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":404,"message":"gone","errors":[{"reason":"playlistItemNotFound"}]}}`))
		case r.URL.Path == "/playlistItems" && r.Method == http.MethodPut:
			var b struct {
				ID      string
				Snippet struct{ Position int }
			}
			_ = json.NewDecoder(r.Body).Decode(&b)
			for i, it := range f.items {
				if it.ID == b.ID {
					f.items = append(f.items[:i], f.items[i+1:]...)
					p := b.Snippet.Position
					f.items = append(f.items[:p], append([]fakeItem{it}, f.items[p:]...)...)
					_, _ = w.Write([]byte(`{}`))
					return
				}
			}
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusTeapot)
		}
	})
}

func newClient(t *testing.T, f *fakeAPI) *Client {
	srv := httptest.NewServer(f.handler(t))
	t.Cleanup(srv.Close)
	return &Client{HTTP: srv.Client(), BaseURL: srv.URL}
}

func videos(f *fakeAPI) string {
	var s []string
	for _, it := range f.items {
		s = append(s, it.Video)
	}
	return strings.Join(s, ",")
}

func TestClientRoundTrip(t *testing.T) {
	ctx := context.Background()
	f := &fakeAPI{pageSize: 2}
	c := newClient(t, f)

	id, err := c.CreatePlaylist(ctx, "Stargazer queue", "kept by stargazer")
	if err != nil || id != "PL123" {
		t.Fatalf("create: %q %v", id, err)
	}
	for i, v := range []string{"a", "b", "c", "d", "e"} {
		if err := c.AddItem(ctx, id, v, i); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.AddItem(ctx, id, "x", 1); err != nil {
		t.Fatal(err)
	}
	if videos(f) != "a,x,b,c,d,e" {
		t.Fatalf("add at a position: %s", videos(f))
	}

	got, err := c.ListItems(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 6 || got[1].VideoID != "x" || got[5].VideoID != "e" {
		t.Fatalf("list must follow every page, in order: %+v", got)
	}

	if err := c.DeleteItem(ctx, got[1].ItemID); err != nil {
		t.Fatal(err)
	}
	if err := c.MoveItem(ctx, id, got[5].ItemID, "e", 0); err != nil {
		t.Fatal(err)
	}
	if videos(f) != "e,a,b,c,d" {
		t.Errorf("delete then move: %s", videos(f))
	}
}

func TestClientMapsAPIErrors(t *testing.T) {
	ctx := context.Background()
	f := &fakeAPI{failWith: &struct {
		status int
		body   string
	}{403, `{"error":{"code":403,"message":"quota used","errors":[{"reason":"quotaExceeded"}]}}`}}
	c := newClient(t, f)

	_, err := c.ListItems(ctx, "PL1")
	var ae *APIError
	if !errors.As(err, &ae) || !ae.QuotaExceeded() || ae.Status != 403 {
		t.Fatalf("want a quota APIError, got %v", err)
	}
	if !strings.Contains(err.Error(), "quotaExceeded") || !strings.Contains(err.Error(), "list playlist items") {
		t.Errorf("the message must name the operation and the reason: %v", err)
	}

	f.failWith = &struct {
		status int
		body   string
	}{404, `{"error":{"code":404,"message":"no such playlist","errors":[{"reason":"playlistNotFound"}]}}`}
	_, err = c.ListItems(ctx, "PL1")
	if !errors.As(err, &ae) || !ae.NotFound() || ae.QuotaExceeded() {
		t.Fatalf("want a not-found APIError, got %v", err)
	}
}

func TestClientNeverCallsAnythingThatDeletesAPlaylist(t *testing.T) {
	ctx := context.Background()
	f := &fakeAPI{}
	c := newClient(t, f)
	_, _ = c.CreatePlaylist(ctx, "t", "d")
	_ = c.AddItem(ctx, "PL123", "a", 0)
	_, _ = c.ListItems(ctx, "PL123")
	_ = c.DeleteItem(ctx, "item1")
	for _, call := range f.calls {
		if strings.HasPrefix(call, "DELETE /playlists") && !strings.HasPrefix(call, "DELETE /playlistItems") {
			t.Errorf("a playlist delete was attempted: %s", call)
		}
	}
}
