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
