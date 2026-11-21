package domain

import (
	"sort"
	"strings"
)

// DispatchState is the hand-maintained record of which Accepted ADRs have had their
// stories handed off. It is the only place that fact is stored: an ADR file is
// immutable once accepted, and the board is regenerated on every sync.
type DispatchState struct {
	// Dispatched maps a track-qualified ADR id (e.g. "2-product/0099") to a free-form
	// reference: a ticket key, or any note such as "done".
	Dispatched map[string]string `yaml:"dispatched"`

	// Parked maps a track-qualified id of a Proposed ADR to the reason it is set aside
	// (e.g. "waiting on PoC"). Parked ADRs stay visible but do not count against the
	// WIP limit.
	Parked map[string]string `yaml:"parked"`

	// Heard lists talks you have listened to, by signal id (e.g. "0016") or YouTube
	// video id. They are left out of the playlist. The YouTube API cannot read watch
	// history, so this is the only record.
	Heard []string `yaml:"heard"`

	// Fronts are named pieces of work that need no ADR (an initiative, go-live
	// preparation). They have their own limit, separate from the ADR WIP.
	Fronts map[string]Front `yaml:"fronts"`
}

// Front is one non-ADR piece of work in flight, keyed by a short id in the state file.
type Front struct {
	ID    string `yaml:"-" json:"id"`
	Title string `yaml:"title" json:"title"`
	// Next is the one-line next action, hand-written.
	Next string `yaml:"next" json:"next"`
	// Ref points at where the work lives: a PR, an epic, a register area.
	Ref string `yaml:"ref" json:"ref,omitempty"`
}

// OrderedFronts returns the fronts sorted by id, with IDs filled in.
func (s *DispatchState) OrderedFronts() []Front {
	if s == nil {
		return nil
	}
	ids := make([]string, 0, len(s.Fronts))
	for id := range s.Fronts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]Front, 0, len(ids))
	for _, id := range ids {
		f := s.Fronts[id]
		f.ID = id
		out = append(out, f)
	}
	return out
}

// LookupParked returns the parked reason for an ADR by its qualified id.
func (s *DispatchState) LookupParked(qualifiedID string) (string, bool) {
	if s == nil {
		return "", false
	}
	for k, v := range s.Parked {
		if strings.EqualFold(strings.TrimSpace(k), qualifiedID) {
			return strings.TrimSpace(v), true
		}
	}
	return "", false
}

// Lookup returns the dispatch reference for an ADR by its qualified id, compared
// case-insensitively. Bare numbers are deliberately not accepted: the same number
// can exist in more than one track.
func (s *DispatchState) Lookup(qualifiedID string) (string, bool) {
	if s == nil {
		return "", false
	}
	for k, v := range s.Dispatched {
		if strings.EqualFold(strings.TrimSpace(k), qualifiedID) {
			return strings.TrimSpace(v), true
		}
	}
	return "", false
}

// IsHeard reports whether a talk is marked heard, by signal id or video id,
// compared case-insensitively for ids and exactly for video ids' case-insensitive form.
func (s *DispatchState) IsHeard(signalID, videoID string) bool {
	if s == nil {
		return false
	}
	for _, h := range s.Heard {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		if (signalID != "" && strings.EqualFold(h, signalID)) || (videoID != "" && strings.EqualFold(h, videoID)) {
			return true
		}
	}
	return false
}
