package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"time"

	"github.com/costap/stargazer/internal/domain"
	"github.com/costap/stargazer/internal/scanner"
	"github.com/costap/stargazer/internal/youtube"
)

// PlaylistAPI is the part of the YouTube client the sync needs. The real client
// satisfies it; tests use a fake.
type PlaylistAPI interface {
	CreatePlaylist(ctx context.Context, title, description string) (string, error)
	ListItems(ctx context.Context, playlistID string) ([]domain.PlaylistEntry, error)
	AddItem(ctx context.Context, playlistID, videoID string, position int) error
	DeleteItem(ctx context.Context, itemID string) error
	MoveItem(ctx context.Context, playlistID, itemID, videoID string, position int) error
}

// PlaylistOptions are the flags of "playlist sync".
type PlaylistOptions struct {
	DryRun bool
}

// PlaylistResult says what a playlist sync did (or, in a dry run, would do).
type PlaylistResult struct {
	PlaylistID string
	Created    bool
	Wanted     int
	Plan       domain.PlaylistPlan
}

// SyncPlaylist makes the owned YouTube playlist match the audio queue. newAPI is
// called only when the network is actually needed, so a dry run before the first
// sync (nothing to compare with) needs no login.
func (e *Engine) SyncPlaylist(ctx context.Context, opts PlaylistOptions, newAPI func() (PlaylistAPI, error), out io.Writer) (PlaylistResult, error) {
	var res PlaylistResult
	if e.cfg.Signals.Source == "" {
		return res, errors.New("no signals source: set signals.source (or --signals) to the tech-signals export")
	}
	payload, err := scanner.LoadSignalsPayload(ctx, e.cfg.Signals.Source)
	if err != nil {
		return res, fmt.Errorf("signals: %w", err)
	}
	if payload == nil {
		return res, errors.New("no signals loaded")
	}

	var state *domain.DispatchState
	if e.cfg.State.File != "" {
		state, err = scanner.LoadDispatchState(e.cfg.State.File)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) && !e.cfg.State.Explicit {
				state = nil
			} else {
				return res, fmt.Errorf("state %s: %w", e.cfg.State.File, err)
			}
		}
	}

	rec, err := scanner.LoadPlaylistRecord(e.cfg.YouTube.PlaylistFile)
	if err != nil {
		return res, err
	}
	title := e.cfg.YouTube.Playlist.Title
	res.PlaylistID = rec.ID

	var api PlaylistAPI
	justCreated := false
	if rec.ID == "" {
		res.Created = true
		if opts.DryRun {
			desired, notices := BuildDesiredPlaylist(payload, state, e.cfg.YouTube.Playlist.Cap, rec.Unavailable)
			for _, n := range notices {
				fmt.Fprintf(out, "Notice: %s\n", n)
			}
			res.Wanted = len(desired)
			fmt.Fprintf(out, "Would create the private playlist %q.\n", title)
			res.Plan = domain.PlanPlaylist(desired, nil)
			printPlaylistPlan(out, res.Plan, len(desired))
			fmt.Fprintln(out, "Dry run: nothing was changed.")
			return res, nil
		}
		if api, err = newAPI(); err != nil {
			return res, err
		}
		id, err := api.CreatePlaylist(ctx, title, "Kept in step with the Stargazer audio queue. Edits made here are overwritten by the next sync.")
		if err != nil {
			return res, err
		}
		rec = scanner.PlaylistRecord{ID: id, Title: title}
		res.PlaylistID = id
		justCreated = true
		// Save at once: if a later step fails, the next run must find this playlist
		// and not create a second one.
		if err := scanner.SavePlaylistRecord(e.cfg.YouTube.PlaylistFile, rec); err != nil {
			return res, fmt.Errorf("the playlist %s was created but could not be recorded: %w", id, err)
		}
		fmt.Fprintf(out, "Created playlist %q (%s).\n", title, id)
	} else if api, err = newAPI(); err != nil {
		return res, err
	}

	// A video YouTube refuses is recorded and left out, and the plan is made again
	// from the playlist as it now stands, so positions stay right and the slot is
	// filled by the next talk. The bound only stops a runaway loop.
	const maxReplans = 10
	for attempt := 0; ; attempt++ {
		desired, notices := BuildDesiredPlaylist(payload, state, e.cfg.YouTube.Playlist.Cap, rec.Unavailable)
		if attempt == 0 {
			for _, n := range notices {
				fmt.Fprintf(out, "Notice: %s\n", n)
			}
		}
		res.Wanted = len(desired)

		var current []domain.PlaylistEntry
		if !(justCreated && attempt == 0) {
			if current, err = api.ListItems(ctx, rec.ID); err != nil {
				var ae *youtube.APIError
				if errors.As(err, &ae) && ae.NotFound() {
					return res, fmt.Errorf("playlist %s no longer exists on YouTube (deleted by hand?): remove %s to create a new one: %w", rec.ID, e.cfg.YouTube.PlaylistFile, err)
				}
				return res, err
			}
		}

		res.Plan = domain.PlanPlaylist(desired, current)
		if attempt == 0 || !res.Plan.Empty() {
			printPlaylistPlan(out, res.Plan, len(desired))
		}
		if opts.DryRun {
			fmt.Fprintln(out, "Dry run: nothing was changed.")
			return res, nil
		}
		if res.Plan.Empty() {
			fmt.Fprintln(out, "Playlist already matches the queue.")
			break
		}

		bad, err := applyPlaylistPlan(ctx, api, rec.ID, res.Plan)
		if err == nil {
			break
		}
		if bad == "" {
			return res, err
		}
		rec.Unavailable = append(rec.Unavailable, bad)
		if serr := scanner.SavePlaylistRecord(e.cfg.YouTube.PlaylistFile, rec); serr != nil {
			return res, serr
		}
		fmt.Fprintf(out, "Notice: YouTube says video %s is not available (deleted, private or blocked); it is skipped from now on and the next talk takes its place.\n", bad)
		if attempt+1 >= maxReplans {
			return res, fmt.Errorf("gave up after %d unavailable videos in one run; rerun to continue: %w", maxReplans, err)
		}
	}

	rec.LastSync = time.Now().UTC().Format(time.RFC3339)
	if err := scanner.SavePlaylistRecord(e.cfg.YouTube.PlaylistFile, rec); err != nil {
		return res, err
	}
	return res, nil
}

// applyPlaylistPlan runs the removals, then the steps in order. When YouTube refuses
// a video as unavailable it returns that video's id with the error, so the caller can
// record it and plan again; any other failure returns an empty id.
func applyPlaylistPlan(ctx context.Context, api PlaylistAPI, playlistID string, plan domain.PlaylistPlan) (string, error) {
	for _, r := range plan.Remove {
		if err := api.DeleteItem(ctx, r.ItemID); err != nil {
			// Already gone is the state we wanted. The listing can lag a delete
			// that just succeeded, so a second run would otherwise fail here.
			var ae *youtube.APIError
			if errors.As(err, &ae) && ae.ItemGone() {
				continue
			}
			return "", fmt.Errorf("removing %q: %w", r.Title, err)
		}
	}
	for _, s := range plan.Steps {
		var err error
		switch s.Kind {
		case domain.StepAdd:
			err = api.AddItem(ctx, playlistID, s.VideoID, s.Position)
		case domain.StepMove:
			err = api.MoveItem(ctx, playlistID, s.ItemID, s.VideoID, s.Position)
		}
		if err != nil {
			wrapped := fmt.Errorf("%s %q at %d (earlier steps were applied; rerun to continue): %w", s.Kind, s.Title, s.Position+1, err)
			var ae *youtube.APIError
			if s.Kind == domain.StepAdd && errors.As(err, &ae) && ae.VideoUnavailable() {
				return s.VideoID, wrapped
			}
			return "", wrapped
		}
	}
	return "", nil
}

func printPlaylistPlan(out io.Writer, p domain.PlaylistPlan, wanted int) {
	for _, r := range p.Remove {
		fmt.Fprintf(out, "- remove  %s\n", label(r.Title, r.VideoID))
	}
	for _, s := range p.Steps {
		verb := "+ add   "
		if s.Kind == domain.StepMove {
			verb = "~ move  "
		}
		fmt.Fprintf(out, "%s #%d %s\n", verb, s.Position+1, label(s.Title, s.VideoID))
	}
	fmt.Fprintf(out, "Queue holds %d talk(s); %d to remove, %d to add or move.\n", wanted, len(p.Remove), len(p.Steps))
}

func label(title, id string) string {
	if title == "" {
		return id
	}
	return fmt.Sprintf("%s (%s)", title, id)
}
