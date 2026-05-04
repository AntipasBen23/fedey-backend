package utils

import (
	"strings"
	"testing"
)

// ─── TruncateTweet edge cases ─────────────────────────────────────────────────

func TestTruncateTweet_SingleLongWord(t *testing.T) {
	// No spaces — falls back to hard cut at limit-1
	input := strings.Repeat("x", 300)
	got := TruncateTweet(input)
	if len([]rune(got)) > 280 {
		t.Errorf("single long word: result too long (%d runes)", len([]rune(got)))
	}
}

func TestTruncateTweet_281Chars(t *testing.T) {
	input := strings.Repeat("a", 281)
	got := TruncateTweet(input)
	if len([]rune(got)) > 280 {
		t.Errorf("281 chars should be truncated to <=280, got %d", len([]rune(got)))
	}
}

func TestTruncateTweet_PreservesWordBoundary(t *testing.T) {
	// Build a sentence that exceeds 280 but has clear word breaks
	input := strings.Repeat("hello ", 50) // 300 chars
	got := TruncateTweet(input)
	// Result should not cut in the middle of "hello"
	trimmed := strings.TrimSuffix(got, "…")
	if strings.HasSuffix(strings.TrimSpace(trimmed), "hell") ||
		strings.HasSuffix(strings.TrimSpace(trimmed), "hel") {
		t.Errorf("word boundary not respected: %q", got[max(0, len(got)-20):])
	}
}

// ─── SplitThread edge cases ───────────────────────────────────────────────────

func TestSplitThread_LargeThread(t *testing.T) {
	var lines []string
	for i := 1; i <= 10; i++ {
		lines = append(lines, strings.Repeat("x", 1)+"/"+strings.Repeat(" tweet content", 5))
	}
	script := strings.Join(lines, "\n")
	tweets := SplitThread(script)
	if len(tweets) != 10 {
		t.Errorf("expected 10 tweets, got %d", len(tweets))
	}
}

func TestSplitThread_OnlyBlankLines(t *testing.T) {
	tweets := SplitThread("\n\n\n\n")
	if len(tweets) != 0 {
		t.Errorf("expected 0 tweets from blank input, got %d", len(tweets))
	}
}

func TestSplitThread_TrimsTweets(t *testing.T) {
	script := "1/ Hello World"
	tweets := SplitThread(script)
	if len(tweets) != 1 {
		t.Fatalf("expected 1 tweet, got %d", len(tweets))
	}
	if tweets[0] != strings.TrimSpace(tweets[0]) {
		t.Errorf("tweet content should be trimmed, got: %q", tweets[0])
	}
}
