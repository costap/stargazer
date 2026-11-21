package domain

import (
	"time"
)

// QueuedTalk represents a prioritized video or conference talk extracted
// from Astrometrics signals and playlist digests for offline listening.
type QueuedTalk struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Category string `json:"category"`
	Context  string `json:"context"` // Explains why it was captured or which digest curated it
	Source   string `json:"source"`  // e.g. "Signal Note", "StrimziCon2026", "argocon2026"
}

// DispatchBoard is the aggregate root representing the current operational
// architecture state across upstream roadmap epochs, active design fronts,
// squad hand-offs, candidate radar PoCs, and offline audio ingestion.
type DispatchBoard struct {
	// SyncedAt records the timestamp when the board was reconciled.
	SyncedAt time.Time `json:"synced_at"`

	// WIPLimit defines the maximum number of concurrent proposed ADRs / design fronts
	// that can be active before triggering cognitive concurrency saturation warnings.
	WIPLimit int `json:"wip_limit"`

	// Roadmap represents the hand-authored upstream delivery milestones (phases and needs).
	Roadmap *RoadmapManifest `json:"roadmap,omitempty"`

	// ActiveADRs contains all scanned ADRs currently in "Proposed" review state.
	ActiveADRs []ADR `json:"active_adrs"`

	// Handoffs contains all scanned ADRs in "Accepted" state awaiting ticket slicing.
	Handoffs []ADR `json:"handoffs"`

	// ParkedADRs are Proposed ADRs set aside in the state file. They are not WIP.
	ParkedADRs []ADR `json:"parked_adrs,omitempty"`

	// Fronts are non-ADR pieces of work in flight (state file), counted against
	// FrontsLimit and not against WIPLimit.
	Fronts      []Front `json:"fronts,omitempty"`
	FrontsLimit int     `json:"fronts_limit"`

	// Candidates is the open ADR candidate queue, oldest first (nil if not configured).
	Candidates *Candidates `json:"candidates,omitempty"`

	// DispatchedHidden counts Accepted ADRs left out of Handoffs because they are
	// marked dispatched. Zero when every Accepted ADR is listed.
	DispatchedHidden int `json:"dispatched_hidden,omitempty"`

	// Signals contains candidate technologies from V'Ger / Astrometrics (e.g. status: spotted).
	Signals []SignalItem `json:"signals,omitempty"`

	// Digests contains playlist digests from conferences (GoogleCloudNext, StrimziCon, IstioCon, etc.).
	Digests []SignalDigest `json:"digests,omitempty"`

	// AudioQueue holds extracted YouTube conference talks for off-desk intake (walks, transit).
	AudioQueue []QueuedTalk `json:"audio_queue,omitempty"`

	// Warnings lists inputs that could not be read. A board with warnings is degraded:
	// what it shows may be incomplete, and renderers must say so.
	Warnings []string `json:"warnings,omitempty"`

	// ADRUnknown is true when the ADR sources could not be read, so the pipeline and
	// WIP are unknown. An empty ActiveADRs then means "not read", never "none".
	ADRUnknown bool `json:"adr_unknown,omitempty"`
}

// NewDispatchBoard creates an initialized DispatchBoard instance with defaults.
func NewDispatchBoard(
	roadmap *RoadmapManifest,
	adrs []ADR,
	signals []SignalItem,
	digests []SignalDigest,
	wipLimit int,
) *DispatchBoard {
	board := &DispatchBoard{
		SyncedAt: time.Now(),
		WIPLimit: wipLimit,
		Roadmap:  roadmap,
		Signals:  signals,
		Digests:  digests,
	}

	for _, item := range adrs {
		switch item.Status {
		case StatusProposed:
			if item.Parked != "" {
				board.ParkedADRs = append(board.ParkedADRs, item)
				continue
			}
			board.ActiveADRs = append(board.ActiveADRs, item)
		case StatusAccepted:
			board.Handoffs = append(board.Handoffs, item)
		}
	}

	return board
}

// Degraded reports whether any input could not be read.
func (b *DispatchBoard) Degraded() bool {
	return len(b.Warnings) > 0
}

// WIP is defined once, here: Work in progress is the Proposed ADRs that are not parked. A parked ADR (state file) is still listed but
// costs no slot. Accepted ADRs, candidates and roadmap needs are not WIP.

// FrontsOver reports whether more non-ADR fronts are in flight than the fronts limit.
// A limit of zero or less means fronts are not limited.
func (b *DispatchBoard) FrontsOver() bool {
	return b.FrontsLimit > 0 && len(b.Fronts) > b.FrontsLimit
}

// FreeSlots is how many more Proposed ADRs fit under the WIP limit, never negative.
func (b *DispatchBoard) FreeSlots() int {
	if n := b.WIPLimit - len(b.ActiveADRs); n > 0 {
		return n
	}
	return 0
}

// CandidatesGated reports whether open candidates must wait: the pipeline has no
// free slot, or its state is unknown. A gated queue is never offered as work.
func (b *DispatchBoard) CandidatesGated() bool {
	return b.ADRUnknown || b.FreeSlots() == 0
}

// HasWIPViolation returns true if active proposed ADRs exceed the configured WIP limit.
func (b *DispatchBoard) HasWIPViolation() bool {
	return len(b.ActiveADRs) > b.WIPLimit
}

// ActiveWIPCount returns the WIP: Proposed ADRs that are not parked.
func (b *DispatchBoard) ActiveWIPCount() int {
	return len(b.ActiveADRs)
}

// ReadyHandoffCount returns the number of accepted ADRs ready for story generation.
func (b *DispatchBoard) ReadyHandoffCount() int {
	n := 0
	for i := range b.Handoffs {
		if !b.Handoffs[i].IsDispatched() {
			n++
		}
	}
	return n
}

// CriticalPathCount returns the number of roadmap needs explicitly assigned to "owner: me".
func (b *DispatchBoard) CriticalPathCount() int {
	if b.Roadmap == nil {
		return 0
	}
	count := 0
	for _, phase := range b.Roadmap.Phases {
		for _, need := range phase.Needs {
			if need.IsMine() {
				count++
			}
		}
	}
	return count
}

// BlockedDownstreamCount returns the number of roadmap needs owned by external teams (product, devops, squads).
func (b *DispatchBoard) BlockedDownstreamCount() int {
	if b.Roadmap == nil {
		return 0
	}
	count := 0
	for _, phase := range b.Roadmap.Phases {
		for _, need := range phase.Needs {
			if !need.IsMine() {
				count++
			}
		}
	}
	return count
}
