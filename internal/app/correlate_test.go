package app

import (
	"strings"
	"testing"

	"github.com/costap/stargazer/internal/domain"
)

func testADRs() []domain.ADR {
	mk := func(track string, n int, raw string) domain.ADR {
		return domain.ADR{Track: track, Number: n, RawID: raw, QualifiedID: track + "/" + raw}
	}
	return []domain.ADR{
		mk("2-product", 31, "0031"), // also exists in 3-platform
		mk("2-product", 99, "0099"), // only in product
		mk("3-platform", 31, "0031"),
		mk("3-platform", 40, "0040"), // only in platform
	}
}

func roadmapWith(refs ...string) *domain.RoadmapManifest {
	r := &domain.RoadmapManifest{}
	p := domain.Phase{ID: "phase-1"}
	for _, ref := range refs {
		ref := ref
		p.Needs = append(p.Needs, domain.PhaseNeed{Kind: domain.NeedKindADR, Ref: &ref})
	}
	r.Phases = []domain.Phase{p}
	return r
}

func TestCorrelation(t *testing.T) {
	tests := []struct {
		name      string
		ref       string
		wantBound string // qualified id the need resolves to, "" for none
	}{
		{"bare number in both tracks binds to nothing", "0031", ""},
		{"unpadded bare number in both tracks binds to nothing", "31", ""},
		{"ADR- prefix in both tracks binds to nothing", "ADR-0031", ""},
		{"bare number in one track binds to it", "0099", "2-product/0099"},
		{"bare number only in platform binds to platform", "40", "3-platform/0040"},
		{"ADR- prefix in one track binds", "ADR-0099", "2-product/0099"},
		{"qualified product binds exactly", "2-product/0031", "2-product/0031"},
		{"qualified without numeric prefix binds exactly", "product/0031", "2-product/0031"},
		{"qualified platform binds exactly", "3-platform/0031", "3-platform/0031"},
		{"unknown number binds to nothing", "0555", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			adrs := testADRs()
			rm := roadmapWith(tc.ref)
			correlateRoadmapWithADRs(rm, adrs)

			got := rm.Phases[0].Needs[0].ResolvedADR
			switch {
			case tc.wantBound == "" && got != nil:
				t.Errorf("ref %q must not bind, bound to %s", tc.ref, got.QualifiedID)
			case tc.wantBound != "" && (got == nil || got.QualifiedID != tc.wantBound):
				t.Errorf("ref %q: want %s, got %v", tc.ref, tc.wantBound, got)
			}

			tagged := 0
			for _, a := range adrs {
				if a.EpochID != "" {
					tagged++
					if tc.wantBound == "" || a.QualifiedID != tc.wantBound {
						t.Errorf("ref %q wrongly tagged %s with a phase", tc.ref, a.QualifiedID)
					}
				}
			}
			if tc.wantBound != "" && tagged != 1 {
				t.Errorf("ref %q: want exactly one ADR tagged, got %d", tc.ref, tagged)
			}
		})
	}
}

func TestCorrelationNoticesNameTheProblem(t *testing.T) {
	rm := roadmapWith("0031", "0555", "0099", "3-platform/0031")
	notices := correlateRoadmapWithADRs(rm, testADRs())

	if len(notices) != 2 {
		t.Fatalf("want a notice for the ambiguous and the unknown ref only, got %q", notices)
	}
	amb, none := notices[0], notices[1]
	for _, want := range []string{"phase-1", `"0031"`, "ambiguous", "2-product/0031", "3-platform/0031"} {
		if !contains(amb, want) {
			t.Errorf("ambiguity notice lacks %q: %s", want, amb)
		}
	}
	if !contains(none, `"0555"`) || !contains(none, "no ADR") {
		t.Errorf("unknown-ref notice should name the ref: %s", none)
	}
}

func TestCorrelationIgnoresNeedsWithoutAnADRRef(t *testing.T) {
	rm := &domain.RoadmapManifest{Phases: []domain.Phase{{ID: "p", Needs: []domain.PhaseNeed{
		{Kind: domain.NeedKindADR},                           // ref: null
		{Kind: domain.NeedKindDecision, Ref: strPtr("0031")}, // not an ADR need
	}}}}
	if n := correlateRoadmapWithADRs(rm, testADRs()); len(n) != 0 {
		t.Errorf("undrafted and non-ADR needs are not errors, got %q", n)
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
func strPtr(s string) *string     { return &s }
