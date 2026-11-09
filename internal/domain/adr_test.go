package domain

import "testing"

func adr(track string, n int, raw string) ADR {
	return ADR{Track: track, Number: n, RawID: raw, QualifiedID: track + "/" + raw}
}

func TestMatches(t *testing.T) {
	prod := adr("2-product", 31, "0031")
	plt := adr("3-platform", 31, "0031")

	tests := []struct {
		name string
		adr  ADR
		ref  string
		want bool
	}{
		{"qualified exact", prod, "2-product/0031", true},
		{"qualified case-insensitive", prod, "2-PRODUCT/0031", true},
		{"qualified without the numeric prefix", prod, "product/0031", true},
		{"qualified with an unpadded number", prod, "product/31", true},
		{"qualified names the other track", plt, "2-product/0031", false},
		{"qualified, wrong number", prod, "2-product/0032", false},
		{"padded bare number", prod, "0031", true},
		{"unpadded bare number", prod, "31", true},
		{"ADR- prefix", prod, "ADR-0031", true},
		{"ADR- prefix unpadded", prod, "ADR-31", true},
		{"ADR- prefix lower case", prod, "adr-31", true},
		{"surrounding spaces", prod, "  0031 ", true},
		{"empty", prod, "", false},
		{"blank", prod, "   ", false},
		{"different number", prod, "0032", false},
		{"not a number", prod, "banana", false},
		{"unknown track", prod, "other/0031", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.adr.Matches(tc.ref); got != tc.want {
				t.Errorf("%s.Matches(%q) = %v, want %v", tc.adr.QualifiedID, tc.ref, got, tc.want)
			}
		})
	}
}

// A bare number matching in both tracks is why the caller, not Matches, decides
// whether a reference is ambiguous (SG-003). This pins the premise.
func TestBareNumberMatchesEveryTrackThatHasIt(t *testing.T) {
	prod := adr("2-product", 31, "0031")
	plt := adr("3-platform", 31, "0031")
	if !prod.Matches("0031") || !plt.Matches("0031") {
		t.Fatal("a bare number is expected to match in both tracks")
	}
	if !prod.Matches("2-product/0031") || plt.Matches("2-product/0031") {
		t.Fatal("a qualified reference must match one track only")
	}
}
