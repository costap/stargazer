package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/costap/stargazer/internal/config"
	"github.com/costap/stargazer/internal/domain"
	"github.com/costap/stargazer/internal/render"
	"github.com/costap/stargazer/internal/scanner"
)

type SyncOptions struct {
	DryRun            bool
	IncludeAudioQueue bool

	// KeepGoing renders a degraded board when an input cannot be read, instead of
	// failing the sync. Without it, any unreadable ADR group or explicitly configured
	// roadmap is an error and nothing is written.
	KeepGoing bool

	// AllHandoffs lists every Accepted ADR in the hand-off section, dispatched or not.
	// By default only ADRs not yet marked dispatched are listed.
	AllHandoffs bool

	// JSON receives the machine-readable board when non-empty: "-" for the engine's
	// stdout, otherwise a file path. The markdown board is still written as usual.
	JSON string
}

// ErrDegraded is returned when inputs could not be read and KeepGoing is off.
var ErrDegraded = errors.New("inputs could not be read")

type Engine struct {
	cfg *config.Config

	// Stdout receives the JSON document for "-". Defaults to os.Stdout.
	Stdout io.Writer
}

func NewEngine(cfg *config.Config) *Engine {
	return &Engine{cfg: cfg, Stdout: os.Stdout}
}

// Sync executes the default sync workflow
func (e *Engine) Sync(ctx context.Context) error {
	return e.SyncWithOptions(ctx, SyncOptions{
		DryRun:            false,
		IncludeAudioQueue: true,
	})
}

// SyncWithOptions runs the end-to-end reconciliation pipeline
func (e *Engine) SyncWithOptions(ctx context.Context, opts SyncOptions) error {
	var warnings []string
	adrUnknown := false

	// 1. Ingest ADRs across all configured tracks (e.g. 2-product, 3-platform).
	// A group that cannot be read means the pipeline is unknown, not empty.
	adrs, err := scanner.ScanAllADRGroups(e.cfg.ADR.Groups)
	if err != nil {
		adrUnknown = true
		warnings = append(warnings, splitJoined(err)...)
	} else if len(adrs) == 0 {
		adrUnknown = true
		warnings = append(warnings, "no ADRs found in any configured group")
	}

	// 2. Ingest Roadmap Manifest (roadmap.yaml). Absent is normal while the path is
	// still the default; an explicitly configured roadmap that fails is a warning.
	var roadmap *domain.RoadmapManifest
	if e.cfg.Roadmap.File != "" {
		roadmap, err = scanner.LoadRoadmapManifest(e.cfg.Roadmap.File)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) && !e.cfg.Roadmap.Explicit {
				fmt.Fprintf(os.Stderr, "Notice: no roadmap file at %s\n", e.cfg.Roadmap.File)
			} else {
				warnings = append(warnings, fmt.Sprintf("roadmap %s: %v", e.cfg.Roadmap.File, err))
			}
		}
	}

	// 2b. Ingest the dispatch state: which Accepted ADRs already had their stories
	// handed off. Absent is normal while the path is still the default.
	var state *domain.DispatchState
	if e.cfg.State.File != "" {
		state, err = scanner.LoadDispatchState(e.cfg.State.File)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) && !e.cfg.State.Explicit {
				state = nil
			} else {
				warnings = append(warnings, fmt.Sprintf("state %s: %v", e.cfg.State.File, err))
			}
		}
	}

	if len(warnings) > 0 {
		for _, w := range warnings {
			fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
		}
		if !opts.KeepGoing {
			return fmt.Errorf("%w (%d); nothing written, rerun with --keep-going to render a degraded board", ErrDegraded, len(warnings))
		}
	}

	// 3. Ingest Signals Payload (from local JSON export or astrometrics.pfclabs.net)
	var signalsPayload *domain.SignalsPayload
	if e.cfg.Signals.Source != "" {
		signalsPayload, err = scanner.LoadSignalsPayload(ctx, e.cfg.Signals.Source)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Notice: could not load signals payload: %v\n", err)
		}
	}

	// 4. Correlate Roadmap Needs to Scanned ADRs
	if roadmap != nil && len(adrs) > 0 {
		for _, n := range correlateRoadmapWithADRs(roadmap, adrs) {
			fmt.Fprintf(os.Stderr, "Notice: %s\n", n)
		}
	}

	// 5. Preserve Manual "Next Actions" from previous ARCH_DISPATCH.md runs
	existingActions := render.ParseExistingDispatch(e.cfg.Dispatch.Output)
	applyPreservedActions(adrs, existingActions)

	// 5b. Mark Accepted ADRs the state file says are dispatched.
	for _, n := range applyDispatchState(adrs, state) {
		fmt.Fprintf(os.Stderr, "Notice: %s\n", n)
	}

	// 6. Build the Dispatch Board Aggregate
	board := buildDispatchBoard(roadmap, adrs, signalsPayload, e.cfg.ADR.WIPLimit, opts.IncludeAudioQueue, opts.AllHandoffs)
	board.Warnings = warnings
	board.ADRUnknown = adrUnknown

	// 7. Output: Write Markdown unless DryRun is active. When the ADRs were not read,
	// the existing file is left alone: its Next Action notes are the only copy of
	// that working memory, and a board with no pipeline would erase them on the next sync.
	if !opts.DryRun && !adrUnknown {
		if err := render.WriteMarkdownBoard(e.cfg.Dispatch.Output, board); err != nil {
			return fmt.Errorf("failed writing dispatch markdown board to %s: %w", e.cfg.Dispatch.Output, err)
		}
	} else if adrUnknown {
		fmt.Fprintf(os.Stderr, "Warning: %s was not rewritten, to keep its Next Actions\n", e.cfg.Dispatch.Output)
	}

	// 8. Output: the machine-readable board, when asked for. It is emitted even for a
	// degraded board, because a consumer needs the warnings and adr_unknown to know
	// what it cannot trust.
	if opts.JSON != "" {
		if err := e.writeJSON(opts.JSON, board); err != nil {
			return err
		}
	}

	// 9. Output: Render Terminal Summary unless silenced
	if !e.cfg.Dispatch.Silent {
		render.PrintTerminalSummary(board, e.cfg.Dispatch.Output, opts.DryRun)
	}

	return nil
}

// correlateRoadmapWithADRs binds each roadmap ADR need to the one ADR it names and
// tags that ADR with the phase. A reference that matches no ADR, or more than one
// (a bare number that exists in both tracks), binds to nothing: guessing would tag
// the wrong decision with a phase. It returns one notice per such reference.
func correlateRoadmapWithADRs(roadmap *domain.RoadmapManifest, adrs []domain.ADR) []string {
	var notices []string
	for pIdx := range roadmap.Phases {
		phase := &roadmap.Phases[pIdx]
		for nIdx := range phase.Needs {
			need := &phase.Needs[nIdx]
			if need.Kind != domain.NeedKindADR || need.Ref == nil {
				continue
			}

			targetRef := *need.Ref
			var matches []int
			for i := range adrs {
				if adrs[i].Matches(targetRef) {
					matches = append(matches, i)
				}
			}

			switch len(matches) {
			case 1:
				need.ResolvedADR = &adrs[matches[0]]
				adrs[matches[0]].EpochID = phase.ID
			case 0:
				notices = append(notices, fmt.Sprintf("roadmap %s: ref %q matches no ADR", phase.ID, targetRef))
			default:
				ids := make([]string, len(matches))
				for k, i := range matches {
					ids[k] = adrs[i].QualifiedID
				}
				notices = append(notices, fmt.Sprintf("roadmap %s: ref %q is ambiguous (%s); qualify it with a track", phase.ID, targetRef, strings.Join(ids, ", ")))
			}
		}
	}
	return notices
}

// The generated Next Actions. A board that has been synced once carries them in the
// file, so reading them back must not count as the architect having written them.
const (
	defaultProposedAction = "Review RFC comments; drive consensus with lead engineers"
	defaultAcceptedAction = "Run story-gen skill; review Gherkin contracts with pod PM"
)

func applyPreservedActions(adrs []domain.ADR, preserved map[string]string) {
	for i := range adrs {
		key := adrs[i].QualifiedID
		if action, exists := preserved[key]; exists && action != "" {
			adrs[i].NextAction = action
		} else if rawAction, exists := preserved[adrs[i].RawID]; exists && rawAction != "" {
			adrs[i].NextAction = rawAction
		} else {
			if adrs[i].Status == domain.StatusProposed {
				adrs[i].NextAction = defaultProposedAction
			} else if adrs[i].Status == domain.StatusAccepted {
				adrs[i].NextAction = defaultAcceptedAction
			}
		}
		adrs[i].NextActionSet = adrs[i].NextAction != "" &&
			adrs[i].NextAction != defaultProposedAction &&
			adrs[i].NextAction != defaultAcceptedAction
	}
}

// applyDispatchState marks Accepted ADRs as dispatched from the state file and returns
// a notice for every entry that matches no Accepted ADR (a typo, a renamed track, or
// an ADR that is not Accepted yet). A bare number never matches: it can name two ADRs.
func applyDispatchState(adrs []domain.ADR, state *domain.DispatchState) []string {
	if state == nil {
		return nil
	}
	used := map[string]bool{}
	for i := range adrs {
		if adrs[i].Status != domain.StatusAccepted {
			continue
		}
		ref, ok := state.Lookup(adrs[i].QualifiedID)
		if !ok {
			continue
		}
		if ref == "" {
			ref = "yes"
		}
		adrs[i].Dispatched = ref
		used[strings.ToLower(adrs[i].QualifiedID)] = true
	}

	var notices []string
	keys := make([]string, 0, len(state.Dispatched))
	for k := range state.Dispatched {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !used[strings.ToLower(strings.TrimSpace(k))] {
			notices = append(notices, fmt.Sprintf("state: %q matches no Accepted ADR (use the track-qualified id, e.g. 2-product/0099)", k))
		}
	}
	return notices
}

func (e *Engine) writeJSON(dest string, board *domain.DispatchBoard) error {
	if dest == "-" {
		if err := render.WriteJSON(e.Stdout, board); err != nil {
			return fmt.Errorf("failed writing JSON board: %w", err)
		}
		return nil
	}
	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed writing JSON board to %s: %w", dest, err)
	}
	defer f.Close()
	if err := render.WriteJSON(f, board); err != nil {
		return fmt.Errorf("failed writing JSON board to %s: %w", dest, err)
	}
	return f.Close()
}

func buildDispatchBoard(
	roadmap *domain.RoadmapManifest,
	adrs []domain.ADR,
	signalsPayload *domain.SignalsPayload,
	wipLimit int,
	includeAudio bool,
	allHandoffs bool,
) *domain.DispatchBoard {
	board := &domain.DispatchBoard{
		SyncedAt: time.Now(),
		WIPLimit: wipLimit,
		Roadmap:  roadmap,
	}

	for _, item := range adrs {
		switch item.Status {
		case domain.StatusProposed:
			board.ActiveADRs = append(board.ActiveADRs, item)
		case domain.StatusAccepted:
			if item.IsDispatched() && !allHandoffs {
				board.DispatchedHidden++
				continue
			}
			board.Handoffs = append(board.Handoffs, item)
		}
	}

	if signalsPayload != nil {
		board.Signals = signalsPayload.Signals
		board.Digests = signalsPayload.Digests

		if includeAudio {
			board.AudioQueue = extractAudioQueue(signalsPayload)
		}
	}

	return board
}

func extractAudioQueue(payload *domain.SignalsPayload) []domain.QueuedTalk {
	var queue []domain.QueuedTalk
	seen := make(map[string]bool)

	for _, s := range payload.Signals {
		if isYouTube(s.URL) && !seen[s.URL] {
			seen[s.URL] = true
			queue = append(queue, domain.QueuedTalk{
				ID:       s.ID,
				Title:    s.Title,
				URL:      s.URL,
				Category: s.Category,
				Context:  s.WhyCaptured,
				Source:   "Signal Note",
			})
		}
	}

	for _, d := range payload.Digests {
		for _, link := range extractWatchFirstLinks(d.RawContent) {
			if isYouTube(link.URL) && !seen[link.URL] {
				seen[link.URL] = true
				queue = append(queue, domain.QueuedTalk{
					ID:       d.Slug,
					Title:    link.Title,
					URL:      link.URL,
					Category: "conference",
					Context:  fmt.Sprintf("From %s: %s", d.Title, link.Context),
					Source:   d.Slug,
				})
			}
		}
	}

	return queue
}

func isYouTube(u string) bool {
	return strings.Contains(u, "youtube.com/watch") || strings.Contains(u, "youtu.be/")
}

type watchFirstItem struct {
	Title   string
	URL     string
	Context string
}

func extractWatchFirstLinks(raw string) []watchFirstItem {
	var items []watchFirstItem
	lines := strings.Split(raw, "\n")
	inWatchFirst := false

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		if strings.HasPrefix(line, "## Watch These First") {
			inWatchFirst = true
			continue
		}
		if inWatchFirst && strings.HasPrefix(line, "## ") {
			break
		}

		if inWatchFirst && strings.Contains(line, "[http") && strings.Contains(line, "](") {
			startTitle := strings.Index(line, "[")
			endTitle := strings.Index(line, "](")
			endURL := strings.Index(line, ")")

			if startTitle != -1 && endTitle != -1 && endURL != -1 && endURL > endTitle {
				title := strings.TrimPrefix(line[startTitle+1:endTitle], "**")
				title = strings.TrimSuffix(title, "**")
				u := line[endTitle+2 : endURL]

				contextText := ""
				if i+1 < len(lines) {
					contextText = strings.TrimSpace(lines[i+1])
				}

				items = append(items, watchFirstItem{
					Title:   title,
					URL:     u,
					Context: contextText,
				})
			}
		}
	}
	return items
}

// splitJoined unpacks an errors.Join result into one message per error.
func splitJoined(err error) []string {
	if j, ok := err.(interface{ Unwrap() []error }); ok {
		var out []string
		for _, e := range j.Unwrap() {
			out = append(out, e.Error())
		}
		return out
	}
	return []string{err.Error()}
}
