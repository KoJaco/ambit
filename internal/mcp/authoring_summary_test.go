package mcp

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSeedSummaryKeepsLongTranscript(t *testing.T) {
	exact := strings.Repeat("a", summaryTranscriptMax)
	if got := seedSummary(3, exact); strings.HasSuffix(got, "...") || !strings.HasSuffix(got, exact) {
		t.Fatalf("transcript at the limit must be kept whole")
	}
	long := strings.Repeat("é", summaryTranscriptMax+10)
	got := seedSummary(3, long)
	if !utf8.ValidString(got) || !strings.HasSuffix(got, "...") {
		t.Fatalf("over-limit summary invalid or not marked: %q", got)
	}
	if n := strings.Count(got, "é"); n != summaryTranscriptMax {
		t.Fatalf("kept %d runes, want %d", n, summaryTranscriptMax)
	}
	if !strings.HasPrefix(got, "3 operation(s) from transcript: ") {
		t.Fatalf("prefix: %q", got)
	}
}
