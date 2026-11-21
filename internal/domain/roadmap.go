package domain

import (
	"strings"
	"time"
)

type NeedKind string

const (
	NeedKindADR      NeedKind = "adr"
	NeedKindDecision NeedKind = "decision"
	NeedKindDesign   NeedKind = "design"
)

type PhaseNeed struct {
	ID     string   `yaml:"id"`     // names an outcome, so its ADRs and decisions can point at it
	For    string   `yaml:"for"`    // the outcome id this need serves
	Title  string   `yaml:"title"`  // short description shown on the board; the first sentence of Gist when unset
	Review string   `yaml:"review"` // who reviews without owning, e.g. "me"
	With   string   `yaml:"with"`   // who else must agree
	Kind   NeedKind `yaml:"kind"`
	Ref    *string  `yaml:"ref"`   // nil when ref: null (unassigned open decision)
	Gist   string   `yaml:"gist"`  // Short summary when ref is null
	Owner  string   `yaml:"owner"` // "me", "product", "devops", "<squad>"
	Squad  string   `yaml:"squad"` // e.g. "checkout"
	Note   string   `yaml:"note"`
	Blocks []string `yaml:"blocks"` // Blocked ticket/ADR refs e.g. [OPR-089]

	// Resolved pointers during scanning
	ResolvedADR *ADR `yaml:"-"`
}

func (n *PhaseNeed) IsMine() bool {
	return strings.EqualFold(n.Owner, "me")
}

// DisplayRef is what the board shows to identify a need: the outcome id when it
// has one (a doc path says where an outcome is tracked, not what it is), else
// the ref, else a marker that no ADR exists yet.
func (n *PhaseNeed) DisplayRef() string {
	if n.ID != "" {
		return n.ID
	}
	if n.Ref != nil && *n.Ref != "" {
		return *n.Ref
	}
	return "(no ADR yet)"
}

// ShortDesc is a one-line description of the need: Title when set, otherwise the
// first sentence of Gist, cut to max runes with an ellipsis. It never invents text.
func (n *PhaseNeed) ShortDesc(max int) string {
	s := strings.TrimSpace(n.Title)
	if s == "" {
		s = strings.Join(strings.Fields(n.Gist), " ")
		if i := strings.Index(s, ". "); i >= 0 {
			s = s[:i+1]
		}
	}
	r := []rune(s)
	if max > 1 && len(r) > max {
		return string(r[:max-1]) + "\u2026"
	}
	return s
}

// OwnerTag is the owner with the reviewer and co-decider, e.g. "TEAM, REVIEW: ME".
func (n *PhaseNeed) OwnerTag() string {
	tag := strings.ToUpper(n.Owner)
	if tag == "" {
		tag = "UNASSIGNED"
	}
	if n.With != "" {
		tag += ", WITH: " + strings.ToUpper(n.With)
	}
	if n.Review != "" {
		tag += ", REVIEW: " + strings.ToUpper(n.Review)
	}
	return tag
}

type Phase struct {
	ID             string      `yaml:"id"`
	Name           string      `yaml:"name"`
	Target         string      `yaml:"target"`
	Status         string      `yaml:"status"` // in-flight, horizon
	Scope          string      `yaml:"scope"`
	ReadinessQuery string      `yaml:"readiness_query"`
	Needs          []PhaseNeed `yaml:"needs"`
}

type RoadmapManifest struct {
	RoadmapAsOf string  `yaml:"roadmap_as_of"`
	LaunchDate  string  `yaml:"launch"` // "2026-09-15"
	Phases      []Phase `yaml:"phases"`
}

func (r *RoadmapManifest) DaysUntilLaunch() int {
	if r.LaunchDate == "" {
		return 0
	}
	t, err := time.Parse("2006-01-02", r.LaunchDate)
	if err != nil {
		return 0
	}
	diff := time.Until(t)
	return int(diff.Hours() / 24)
}
