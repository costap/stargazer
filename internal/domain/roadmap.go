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

func (n *PhaseNeed) DisplayRef() string {
	if n.Ref != nil && *n.Ref != "" {
		return *n.Ref
	}
	return "DECISION (UNASSIGNED)"
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
