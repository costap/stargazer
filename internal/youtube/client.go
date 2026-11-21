// Package youtube is a small client for the parts of the YouTube Data API v3 that
// keep one playlist in step with a list of videos. It uses plain net/http; the caller
// supplies an http.Client that already carries OAuth credentials.
package youtube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"

	"github.com/costap/stargazer/internal/domain"
)

const defaultBase = "https://www.googleapis.com/youtube/v3"

// Client talks to the API. BaseURL is overridable so tests can point it at a fake.
type Client struct {
	HTTP    *http.Client
	BaseURL string
}

// APIError is a non-2xx answer from the API, with Google's reason code when present.
type APIError struct {
	Status  int
	Reason  string
	Message string
	Op      string
}

func (e *APIError) Error() string {
	r := e.Reason
	if r == "" {
		r = http.StatusText(e.Status)
	}
	return fmt.Sprintf("youtube %s: %d %s: %s", e.Op, e.Status, r, e.Message)
}

// QuotaExceeded reports whether the daily API allowance is used up.
func (e *APIError) QuotaExceeded() bool {
	return e.Reason == "quotaExceeded" || e.Reason == "dailyLimitExceeded" || e.Reason == "rateLimitExceeded"
}

// VideoUnavailable reports that YouTube refused a video itself (deleted, private or
// blocked), as opposed to a problem with the playlist or the login.
func (e *APIError) VideoUnavailable() bool {
	return e.Reason == "videoNotFound" || e.Reason == "videoNotAvailable"
}

// ItemGone reports that the playlist item to delete no longer exists, as happens
// when it was removed a moment ago and the list still showed it, or removed by hand.
func (e *APIError) ItemGone() bool { return e.Reason == "playlistItemNotFound" }

// NotFound reports whether the playlist (or item) does not exist.
func (e *APIError) NotFound() bool { return e.Status == http.StatusNotFound }

func (c *Client) base() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return defaultBase
}

func (c *Client) do(ctx context.Context, op, method, path string, q url.Values, body any, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	u := c.base() + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rdr)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("youtube %s: %w", op, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return parseError(op, resp.StatusCode, data)
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("youtube %s: unreadable answer: %w", op, err)
		}
	}
	return nil
}

func parseError(op string, status int, data []byte) error {
	var e struct {
		Error struct {
			Message string `json:"message"`
			Errors  []struct {
				Reason string `json:"reason"`
			} `json:"errors"`
		} `json:"error"`
	}
	_ = json.Unmarshal(data, &e)
	ae := &APIError{Status: status, Op: op, Message: e.Error.Message}
	if len(e.Error.Errors) > 0 {
		ae.Reason = e.Error.Errors[0].Reason
	}
	if ae.Message == "" {
		ae.Message = string(bytes.TrimSpace(data))
	}
	return ae
}

type resourceID struct {
	Kind    string `json:"kind"`
	VideoID string `json:"videoId"`
}

type itemSnippet struct {
	PlaylistID string     `json:"playlistId,omitempty"`
	Position   *int       `json:"position,omitempty"`
	ResourceID resourceID `json:"resourceId"`
	Title      string     `json:"title,omitempty"`
}

// CreatePlaylist makes a private playlist and returns its id.
func (c *Client) CreatePlaylist(ctx context.Context, title, description string) (string, error) {
	body := map[string]any{
		"snippet": map[string]string{"title": title, "description": description},
		"status":  map[string]string{"privacyStatus": "private"},
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := c.do(ctx, "create playlist", http.MethodPost, "/playlists", url.Values{"part": {"snippet,status"}}, body, &out); err != nil {
		return "", err
	}
	if out.ID == "" {
		return "", fmt.Errorf("youtube create playlist: the answer carried no playlist id")
	}
	return out.ID, nil
}

// ListItems returns every item in the playlist, in playlist order, following pages.
func (c *Client) ListItems(ctx context.Context, playlistID string) ([]domain.PlaylistEntry, error) {
	type row struct {
		entry domain.PlaylistEntry
		pos   int
	}
	var rows []row
	token := ""
	for {
		q := url.Values{"part": {"snippet"}, "playlistId": {playlistID}, "maxResults": {"50"}}
		if token != "" {
			q.Set("pageToken", token)
		}
		var out struct {
			NextPageToken string `json:"nextPageToken"`
			Items         []struct {
				ID      string      `json:"id"`
				Snippet itemSnippet `json:"snippet"`
			} `json:"items"`
		}
		if err := c.do(ctx, "list playlist items", http.MethodGet, "/playlistItems", q, nil, &out); err != nil {
			return nil, err
		}
		for _, it := range out.Items {
			pos := len(rows)
			if it.Snippet.Position != nil {
				pos = *it.Snippet.Position
			}
			rows = append(rows, row{
				entry: domain.PlaylistEntry{ItemID: it.ID, VideoID: it.Snippet.ResourceID.VideoID, Title: it.Snippet.Title},
				pos:   pos,
			})
		}
		if out.NextPageToken == "" {
			break
		}
		token = out.NextPageToken
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].pos < rows[j].pos })
	entries := make([]domain.PlaylistEntry, len(rows))
	for i, r := range rows {
		entries[i] = r.entry
	}
	return entries, nil
}

// AddItem inserts a video at the given position.
func (c *Client) AddItem(ctx context.Context, playlistID, videoID string, position int) error {
	pos := position
	body := map[string]any{"snippet": itemSnippet{
		PlaylistID: playlistID, Position: &pos, ResourceID: resourceID{Kind: "youtube#video", VideoID: videoID},
	}}
	return c.do(ctx, "add item", http.MethodPost, "/playlistItems", url.Values{"part": {"snippet"}}, body, nil)
}

// DeleteItem removes one playlist item by its item id. It never deletes a playlist.
func (c *Client) DeleteItem(ctx context.Context, itemID string) error {
	return c.do(ctx, "delete item", http.MethodDelete, "/playlistItems", url.Values{"id": {itemID}}, nil, nil)
}

// MoveItem moves an existing item to a position.
func (c *Client) MoveItem(ctx context.Context, playlistID, itemID, videoID string, position int) error {
	pos := position
	body := map[string]any{"id": itemID, "snippet": itemSnippet{
		PlaylistID: playlistID, Position: &pos, ResourceID: resourceID{Kind: "youtube#video", VideoID: videoID},
	}}
	return c.do(ctx, "move item", http.MethodPut, "/playlistItems", url.Values{"part": {"snippet"}}, body, nil)
}
