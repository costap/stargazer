package domain

import (
	"fmt"
	"strconv"
	"strings"
)

// ADRStatus represents the formal review state of an Architecture Decision Record.
type ADRStatus string

const (
	StatusProposed   ADRStatus = "Proposed"
	StatusAccepted   ADRStatus = "Accepted"
	StatusRejected   ADRStatus = "Rejected"
	StatusSuperseded ADRStatus = "Superseded"
)

// ADR represents an architectural decision record scanned from disk.
type ADR struct {
	// Track or directory namespace (e.g., "2-product", "3-platform", "product", "platform")
	Track string `json:"track"`

	// Numeric component extracted from filename (e.g., 99)
	Number int `json:"number"`

	// Original raw string identifier with zero-padding intact (e.g., "0099")
	RawID string `json:"raw_id"`

	// Fully qualified canonical identifier: "<track>/<raw_id>" (e.g., "2-product/0099")
	QualifiedID string `json:"qualified_id"`

	// Title extracted from markdown heading (# ...)
	Title string `json:"title"`

	// Physical file metadata
	Filename string `json:"filename"` // e.g., "0099-order-placement-idempotency.md"
	Path     string `json:"path"`     // Absolute or relative filepath on disk

	// Review & Execution lifecycle
	Status     ADRStatus `json:"status"`      // Proposed, Accepted, Rejected, Superseded
	NextAction string    `json:"next_action"` // Scratchpad preserved across sync runs
	LastCommit string    `json:"last_commit"` // Relative git freshness e.g. "2 days ago"

	// Date is the "Date: YYYY-MM-DD" line of the ADR, empty when absent.
	Date string `json:"date,omitempty"`

	// StatusLine is the first line under "## Status", verbatim (e.g. "Proposed - pending PoC").
	StatusLine string `json:"status_line,omitempty"`

	// NextActionSet is true when NextAction was written by hand, false when it is
	// one of the generated defaults.
	NextActionSet bool `json:"next_action_set"`

	// Dispatched is the reference recorded in the state file once the stories for
	// an Accepted ADR have been handed off (e.g. "PROJ-5301"); empty when not dispatched.
	Dispatched string `json:"dispatched,omitempty"`

	// Upstream alignment
	EpochID string `json:"epoch_id,omitempty"` // Correlated roadmap epoch (e.g., "phase-1-mvp")
}

// IsDispatched reports whether the ADR has been marked as handed off.
func (a *ADR) IsDispatched() bool { return a.Dispatched != "" }

// Matches checks whether an input reference matches this ADR.
// Supports:
//   - Fully qualified: "2-product/0099" or "product/0099"
//   - Padded numeric:  "0099"
//   - Integer string:  "99"
//   - ADR prefix:      "ADR-0099", "ADR-99"
func (a *ADR) Matches(ref string) bool {
	clean := strings.TrimSpace(ref)
	if clean == "" {
		return false
	}

	cleanUpper := strings.ToUpper(clean)
	trackUpper := strings.ToUpper(a.Track)
	normTrackUpper := strings.TrimLeft(trackUpper, "0123456789-_") // "2-PRODUCT" -> "PRODUCT"

	// 1. Exact track-qualified match: "2-PRODUCT/0099" or "PRODUCT/0099"
	if strings.EqualFold(clean, a.QualifiedID) {
		return true
	}
	if strings.HasPrefix(cleanUpper, trackUpper+"/") || strings.HasPrefix(cleanUpper, normTrackUpper+"/") {
		parts := strings.SplitN(cleanUpper, "/", 2)
		if len(parts) == 2 && a.matchesNumeric(parts[1]) {
			return true
		}
	}

	// 2. Unqualified match by number / ID (e.g. "0099", "99", "ADR-0099")
	return a.matchesNumeric(clean)
}

// matchesNumeric tests numeric equivalence against a raw string
func (a *ADR) matchesNumeric(val string) bool {
	v := strings.ToUpper(strings.TrimSpace(val))
	v = strings.TrimPrefix(v, "ADR-")
	v = strings.TrimPrefix(v, "ADR_")
	v = strings.TrimPrefix(v, "ADR")

	// Padded raw ID string comparison ("0099" == "0099")
	if v == strings.ToUpper(a.RawID) {
		return true
	}

	// Integer numerical value comparison (99 == 99)
	if n, err := strconv.Atoi(v); err == nil && n == a.Number {
		return true
	}

	return false
}

// DisplayName outputs a formatted label for markdown and CLI boards
func (a *ADR) DisplayName() string {
	if a.Track != "" {
		return fmt.Sprintf("%s/%s", a.Track, a.RawID)
	}
	return a.RawID
}
