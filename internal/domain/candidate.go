package domain

// Candidate is a decision worth an ADR that has not been drafted yet. Candidates are
// the queue the WIP limit exists to gate: a full pipeline means they wait.
type Candidate struct {
	// ID is the number from the filename (e.g. "011").
	ID    string `json:"id"`
	Title string `json:"title"`
	// Raised is the "Raised" date (YYYY-MM-DD), empty when absent.
	Raised string `json:"raised,omitempty"`
	// Source is the "Source" field, verbatim.
	Source string `json:"source,omitempty"`
}

// Candidates is the open candidate queue plus the gate state derived from it.
type Candidates struct {
	// Configured is false when no candidates directory is set; nothing is rendered then.
	Configured bool        `json:"configured"`
	Open       []Candidate `json:"open"`
}
