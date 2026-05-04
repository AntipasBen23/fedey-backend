package utils

import (
	"strings"
	"testing"
)

// ─── TruncateTweet ───────────────────────────────────────────────────────────

func TestTruncateTweet_ShortPassesThrough(t *testing.T) {
	input := "Hello, world!"
	got := TruncateTweet(input)
	if got != input {
		t.Errorf("expected unchanged %q, got %q", input, got)
	}
}

func TestTruncateTweet_Exactly280(t *testing.T) {
	input := strings.Repeat("a", 280)
	got := TruncateTweet(input)
	if got != input {
		t.Errorf("expected unchanged string of 280 chars")
	}
}

func TestTruncateTweet_Over280IsTruncated(t *testing.T) {
	input := strings.Repeat("a", 300)
	got := TruncateTweet(input)
	if len([]rune(got)) > 280 {
		t.Errorf("result is %d runes, want <= 280", len([]rune(got)))
	}
}

func TestTruncateTweet_EndsWithEllipsis(t *testing.T) {
	// Build a string that is clearly over 280 chars with spaces so word-boundary truncation triggers
	input := strings.Repeat("word ", 70) // 350 chars
	got := TruncateTweet(input)
	if !strings.HasSuffix(got, "…") {
		t.Errorf("truncated tweet should end with ellipsis, got: %q", got[max(0, len(got)-5):])
	}
}

func TestTruncateTweet_Empty(t *testing.T) {
	got := TruncateTweet("")
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestTruncateTweet_UnicodeCountsAsOneRune(t *testing.T) {
	// Each emoji is 1 rune but >1 byte — make sure we count runes, not bytes
	input := strings.Repeat("🔥", 300)
	got := TruncateTweet(input)
	if len([]rune(got)) > 280 {
		t.Errorf("unicode truncation broken: %d runes", len([]rune(got)))
	}
}

// ─── SplitThread ─────────────────────────────────────────────────────────────

func TestSplitThread_Empty(t *testing.T) {
	tweets := SplitThread("")
	if len(tweets) != 0 {
		t.Errorf("expected 0 tweets, got %d", len(tweets))
	}
}

func TestSplitThread_SingleTweet(t *testing.T) {
	script := "1/ This is the first tweet"
	tweets := SplitThread(script)
	if len(tweets) != 1 {
		t.Fatalf("expected 1 tweet, got %d", len(tweets))
	}
	if tweets[0] != "1/ This is the first tweet" {
		t.Errorf("unexpected content: %q", tweets[0])
	}
}

func TestSplitThread_MultipleTweets(t *testing.T) {
	script := "1/ First tweet\n2/ Second tweet\n3/ Third tweet"
	tweets := SplitThread(script)
	if len(tweets) != 3 {
		t.Fatalf("expected 3 tweets, got %d: %v", len(tweets), tweets)
	}
}

func TestSplitThread_SkipsBlankLines(t *testing.T) {
	script := "1/ First tweet\n\n\n2/ Second tweet"
	tweets := SplitThread(script)
	if len(tweets) != 2 {
		t.Fatalf("expected 2 tweets, got %d", len(tweets))
	}
}

func TestSplitThread_MultiLineBody(t *testing.T) {
	script := "1/ First tweet\ncontinued on same tweet\n2/ Second tweet"
	tweets := SplitThread(script)
	if len(tweets) != 2 {
		t.Fatalf("expected 2 tweets, got %d", len(tweets))
	}
	if !strings.Contains(tweets[0], "continued on same tweet") {
		t.Errorf("continuation line should be part of tweet 1: %q", tweets[0])
	}
}

func TestSplitThread_NoNumberedLines(t *testing.T) {
	// Plain text with no number markers → treated as a single tweet
	script := "Just a normal post with no thread markers"
	tweets := SplitThread(script)
	if len(tweets) != 1 {
		t.Fatalf("expected 1 tweet for plain text, got %d", len(tweets))
	}
}

// max helper (Go 1.21+ has built-in, use a local one for compat)
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
