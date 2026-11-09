package domain

import "strings"

// SignalsPayload represents the top-level signals.json export from V'Ger / Astrometrics
type SignalsPayload struct {
	GeneratedAt string         `json:"generatedAt"`
	Stardate    string         `json:"stardate"`
	Stats       SignalStats    `json:"stats"`
	Signals     []SignalItem   `json:"signals"`
	Digests     []SignalDigest `json:"digests"`
}

type SignalStats struct {
	TotalSignals   int            `json:"totalSignals"`
	TotalDigests   int            `json:"totalDigests"`
	StatusCounts   map[string]int `json:"statusCounts"`
	CategoryCounts map[string]int `json:"categoryCounts"`
}

type SignalItem struct {
	ID           string           `json:"id"`
	Title        string           `json:"title"`
	Date         string           `json:"date"`
	Stardate     string           `json:"stardate"`
	Source       string           `json:"source"`
	URL          string           `json:"url"`
	Category     string           `json:"category"`
	Status       string           `json:"status"` // e.g. "spotted"
	WhyCaptured  string           `json:"whyCaptured"`
	Links        []string         `json:"links"`
	Enrichment   SignalEnrichment `json:"enrichment"`
	RawContent   string           `json:"rawContent"`
	RelativePath string           `json:"relativePath"`
}

type SignalEnrichment struct {
	WhatItIs        string   `json:"whatItIs"`
	MaturityAndRisk string   `json:"maturityAndRisk"`
	Alternatives    []string `json:"alternatives"`
	StackFit        string   `json:"stackFit"`
	NextSteps       []string `json:"nextSteps"`
}

type SignalDigest struct {
	Slug             string         `json:"slug"`
	Title            string         `json:"title"`
	TalkCount        int            `json:"talkCount"`
	OverarchingTheme string         `json:"overarchingTheme"`
	Radar            map[string]int `json:"radar"`
	RawContent       string         `json:"rawContent"`
}

// Matches returns true if reference matches bare ID ("0015", "15") or keyword in title/category
func (s *SignalItem) Matches(ref string) bool {
	clean := strings.ToLower(strings.TrimSpace(ref))
	if clean == strings.ToLower(s.ID) || clean == strings.TrimLeft(strings.ToLower(s.ID), "0") {
		return true
	}
	return strings.Contains(strings.ToLower(s.Title), clean) || strings.EqualFold(s.Category, clean)
}

// ExtractAudioQueue returns conference talks tagged as YouTube links, sorted for offline/listening sessions
func (p *SignalsPayload) ExtractAudioQueue() []QueuedTalk {
	var talks []QueuedTalk
	for _, s := range p.Signals {
		if strings.Contains(s.URL, "youtube.com") || strings.Contains(s.URL, "youtu.be") {
			talks = append(talks, QueuedTalk{
				ID:       s.ID,
				Title:    s.Title,
				URL:      s.URL,
				Category: s.Category,
				Context:  s.WhyCaptured,
			})
		}
	}
	return talks
}
