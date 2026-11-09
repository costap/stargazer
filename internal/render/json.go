package render

import (
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/costap/stargazer/internal/domain"
)

// JSONSchemaVersion is bumped on any breaking change to the document: a field
// removed, renamed or given a different meaning. Adding a field is not breaking.
const JSONSchemaVersion = 1

// Document is the machine-readable form of a DispatchBoard. Consumers read this
// instead of scraping ARCH_DISPATCH.md.
type Document struct {
	Schema           int        `json:"schema"`
	SyncedAt         string     `json:"synced_at"`
	ADRUnknown       bool       `json:"adr_unknown"`
	WIP              DocWIP     `json:"wip"`
	Launch           *DocLaunch `json:"launch"`
	Active           []DocADR   `json:"active"`
	Handoffs         []DocADR   `json:"handoffs"`
	DispatchedHidden int        `json:"dispatched_hidden"`
	Gaps             []DocGap   `json:"gaps"`
	Warnings         []string   `json:"warnings"`
}

type DocWIP struct {
	Limit  int `json:"limit"`
	Count  int `json:"count"`
	OverBy int `json:"over_by"`
}

type DocLaunch struct {
	Date string `json:"date"`
	Days int    `json:"days"`
}

type DocADR struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	File          string `json:"file"`
	Date          string `json:"date"`
	Status        string `json:"status"`
	StatusLine    string `json:"status_line"`
	Phase         string `json:"phase"`
	NextAction    string `json:"next_action"`
	NextActionSet bool   `json:"next_action_set"`
	Dispatched    bool   `json:"dispatched"`
	DispatchRef   string `json:"dispatch_ref"`
}

// DocGap is a roadmap need with no ADR behind it yet (ref: null).
type DocGap struct {
	Phase  string   `json:"phase"`
	Kind   string   `json:"kind"`
	Gist   string   `json:"gist"`
	Owner  string   `json:"owner"`
	Blocks []string `json:"blocks"`
}

// BuildDocument converts the board into its document. Slices are never nil, so a
// consumer sees [] rather than null.
func BuildDocument(board *domain.DispatchBoard) Document {
	doc := Document{
		Schema:           JSONSchemaVersion,
		SyncedAt:         board.SyncedAt.UTC().Format(time.RFC3339),
		ADRUnknown:       board.ADRUnknown,
		Active:           []DocADR{},
		Handoffs:         []DocADR{},
		DispatchedHidden: board.DispatchedHidden,
		Gaps:             []DocGap{},
		Warnings:         []string{},
	}

	count := board.ActiveWIPCount()
	doc.WIP = DocWIP{Limit: board.WIPLimit, Count: count}
	if count > board.WIPLimit {
		doc.WIP.OverBy = count - board.WIPLimit
	}

	if board.Roadmap != nil {
		if board.Roadmap.LaunchDate != "" {
			doc.Launch = &DocLaunch{Date: board.Roadmap.LaunchDate, Days: board.Roadmap.DaysUntilLaunch()}
		}
		for _, p := range board.Roadmap.Phases {
			for _, n := range p.Needs {
				if n.Ref != nil && *n.Ref != "" {
					continue
				}
				blocks := n.Blocks
				if blocks == nil {
					blocks = []string{}
				}
				doc.Gaps = append(doc.Gaps, DocGap{
					Phase: p.ID, Kind: string(n.Kind), Gist: strings.TrimSpace(n.Gist), Owner: n.Owner, Blocks: blocks,
				})
			}
		}
	}

	for _, a := range board.ActiveADRs {
		doc.Active = append(doc.Active, docADR(a))
	}
	for _, a := range board.Handoffs {
		doc.Handoffs = append(doc.Handoffs, docADR(a))
	}
	doc.Warnings = append(doc.Warnings, board.Warnings...)
	return doc
}

func docADR(a domain.ADR) DocADR {
	return DocADR{
		ID: a.QualifiedID, Title: a.Title, File: a.Filename, Date: a.Date,
		Status: string(a.Status), StatusLine: a.StatusLine, Phase: a.EpochID,
		NextAction: a.NextAction, NextActionSet: a.NextActionSet,
		Dispatched: a.IsDispatched(), DispatchRef: a.Dispatched,
	}
}

// WriteJSON writes the document, indented, followed by a newline.
func WriteJSON(w io.Writer, board *domain.DispatchBoard) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(BuildDocument(board))
}
