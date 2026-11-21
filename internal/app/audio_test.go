package app

import (
	"testing"

	"github.com/costap/stargazer/internal/domain"
)

func TestExtractAudioQueueTakesSignalsAndDigestLinksOnceEach(t *testing.T) {
	p := &domain.SignalsPayload{
		Signals: []domain.SignalItem{
			{ID: "0001", Title: "Talk A", URL: "https://www.youtube.com/watch?v=AAA", Category: "ai", WhyCaptured: "why"},
			{ID: "0002", Title: "Not video", URL: "https://example.com/post"},
			{ID: "0003", Title: "Talk A again", URL: "https://www.youtube.com/watch?v=AAA"},
			{ID: "0004", Title: "Short link", URL: "https://youtu.be/BBB"},
		},
		Digests: []domain.SignalDigest{{
			Slug:  "conf26",
			Title: "Conf 26",
			RawContent: "# Digest\n\n## Watch These First\n\n" +
				"1. **[Talk A](https://www.youtube.com/watch?v=AAA)**  \n   already a signal\n\n" +
				"2. **[Talk C](https://www.youtube.com/watch?v=CCC)**  \n   only in the digest\n\n" +
				"## Other\n\n[Talk D](https://www.youtube.com/watch?v=DDD)\n",
		}},
	}
	got := extractAudioQueue(p)

	urls := make([]string, len(got))
	for i, q := range got {
		urls[i] = q.URL
	}
	want := []string{
		"https://www.youtube.com/watch?v=AAA",
		"https://youtu.be/BBB",
		"https://www.youtube.com/watch?v=CCC",
	}
	if len(urls) != len(want) {
		t.Fatalf("want %v, got %v", want, urls)
	}
	for i := range want {
		if urls[i] != want[i] {
			t.Errorf("position %d: want %s, got %s", i, want[i], urls[i])
		}
	}
	if got[0].Source != "Signal Note" || got[2].Source != "conf26" {
		t.Errorf("sources: %+v", got)
	}
}
