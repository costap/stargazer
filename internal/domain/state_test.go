package domain

import "testing"

func TestDispatchStateLookup(t *testing.T) {
	st := &DispatchState{Dispatched: map[string]string{"2-product/0099": " PROJ-1 ", "3-platform/0031": ""}}
	tests := []struct {
		id      string
		wantRef string
		wantOK  bool
	}{
		{"2-product/0099", "PROJ-1", true},
		{"2-PRODUCT/0099", "PROJ-1", true},
		{"3-platform/0031", "", true},
		{"0099", "", false},
		{"product/0099", "", false}, // the short track form is not accepted here
		{"2-product/0100", "", false},
	}
	for _, tc := range tests {
		ref, ok := st.Lookup(tc.id)
		if ref != tc.wantRef || ok != tc.wantOK {
			t.Errorf("Lookup(%q) = %q, %v; want %q, %v", tc.id, ref, ok, tc.wantRef, tc.wantOK)
		}
	}
	var nilState *DispatchState
	if _, ok := nilState.Lookup("2-product/0099"); ok {
		t.Error("a nil state has nothing dispatched")
	}
}

func TestFrontsAreOrderedByIDAndLimited(t *testing.T) {
	st := &DispatchState{Fronts: map[string]Front{
		"b-front": {Title: "B", Next: "do b"},
		"a-front": {Title: "A", Next: "do a", Ref: "PR 1"},
	}}
	got := st.OrderedFronts()
	if len(got) != 2 || got[0].ID != "a-front" || got[1].ID != "b-front" || got[0].Ref != "PR 1" {
		t.Errorf("fronts not ordered or ids not filled: %+v", got)
	}
	var none *DispatchState
	if none.OrderedFronts() != nil {
		t.Error("a missing state has no fronts")
	}

	b := &DispatchBoard{Fronts: got, FrontsLimit: 2}
	if b.FrontsOver() {
		t.Error("two fronts against a limit of two is not over")
	}
	b.Fronts = append(b.Fronts, Front{ID: "c"})
	if !b.FrontsOver() {
		t.Error("three fronts against a limit of two is over")
	}
	b.FrontsLimit = 0
	if b.FrontsOver() {
		t.Error("a limit of zero means unlimited")
	}
	b.FrontsLimit = 2
	b.ActiveADRs = make([]ADR, 4)
	b.WIPLimit = 3
	if !b.HasWIPViolation() {
		t.Error("fronts must not hide an ADR WIP violation")
	}
}

func TestCandidatesAreGatedWhenThePipelineIsFullOrUnknown(t *testing.T) {
	b := &DispatchBoard{WIPLimit: 3, ActiveADRs: make([]ADR, 2)}
	if b.CandidatesGated() || b.FreeSlots() != 1 {
		t.Errorf("2 of 3 leaves a free slot: gated=%v slots=%d", b.CandidatesGated(), b.FreeSlots())
	}
	b.ActiveADRs = make([]ADR, 3)
	if !b.CandidatesGated() || b.FreeSlots() != 0 {
		t.Error("3 of 3 must gate")
	}
	b.ActiveADRs = make([]ADR, 5)
	if !b.CandidatesGated() || b.FreeSlots() != 0 {
		t.Error("over the limit must gate, and free slots never go negative")
	}
	b.ActiveADRs = nil
	b.ADRUnknown = true
	if !b.CandidatesGated() {
		t.Error("unknown WIP is treated as full")
	}
}
