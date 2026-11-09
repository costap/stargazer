package domain

import "strings"

// DispatchState is the hand-maintained record of which Accepted ADRs have had their
// stories handed off. It is the only place that fact is stored: an ADR file is
// immutable once accepted, and the board is regenerated on every sync.
type DispatchState struct {
	// Dispatched maps a track-qualified ADR id (e.g. "2-product/0099") to a free-form
	// reference: a ticket key, or any note such as "done".
	Dispatched map[string]string `yaml:"dispatched"`
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
