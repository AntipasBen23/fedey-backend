package handlers

import "testing"

// ─── impactScore edge cases ───────────────────────────────────────────────────

func TestImpactScore_ExactlyAt5000Impressions(t *testing.T) {
	// 5000 is NOT > 5000, so no high-impression bonus
	// engRate=1 → base=12; no bonus → 12
	got := impactScore(1.0, 5000)
	if got != 12 {
		t.Errorf("exactly 5000 impressions: expected 12, got %d", got)
	}
}

func TestImpactScore_ExactlyAt1000Impressions(t *testing.T) {
	// 1000 is NOT > 1000, so no mid-impression bonus
	got := impactScore(1.0, 1000)
	if got != 12 {
		t.Errorf("exactly 1000 impressions: expected 12, got %d", got)
	}
}

func TestImpactScore_JustOver5000(t *testing.T) {
	// engRate=1 → base=12; >5000 → +15 = 27
	got := impactScore(1.0, 5001)
	if got != 27 {
		t.Errorf("5001 impressions: expected 27, got %d", got)
	}
}

func TestImpactScore_JustOver1000(t *testing.T) {
	got := impactScore(1.0, 1001)
	if got != 20 {
		t.Errorf("1001 impressions: expected 20, got %d", got)
	}
}

func TestImpactScore_HighEngLowImpressions(t *testing.T) {
	// engRate=8 → base=96; no impression bonus → 96
	got := impactScore(8.0, 0)
	if got != 96 {
		t.Errorf("high eng, no impressions: expected 96, got %d", got)
	}
}

func TestImpactScore_JustBelow100Cap(t *testing.T) {
	// engRate=7 → base=84; >5000 → +15 = 99
	got := impactScore(7.0, 6000)
	if got != 99 {
		t.Errorf("expected 99, got %d", got)
	}
}

// ─── generateInsight boundary coverage ───────────────────────────────────────

func TestGenerateInsight_JustBelow5(t *testing.T) {
	// 4.99 should NOT hit the >= 5 branch
	o := AnalyticsOverview{PostsAnalyzed: 1, AvgEngRate: 4.99}
	got := generateInsight(o)
	want := "Solid engagement. Try more threads or carousels — they typically outperform single tweets."
	if got != want {
		t.Errorf("4.99 engRate: got %q, want %q", got, want)
	}
}

func TestGenerateInsight_JustBelow2(t *testing.T) {
	// 1.99 with enough impressions → default
	o := AnalyticsOverview{PostsAnalyzed: 1, AvgEngRate: 1.99, TotalImpressions: 600}
	got := generateInsight(o)
	want := "Your best posts get the most replies. Aim to end every post with a direct question."
	if got != want {
		t.Errorf("1.99 engRate: got %q, want %q", got, want)
	}
}

func TestGenerateInsight_ExactlyAt500Impressions(t *testing.T) {
	// 500 impressions with low eng — low impression branch triggers on < 500, so 500 goes to default
	o := AnalyticsOverview{PostsAnalyzed: 1, AvgEngRate: 0.5, TotalImpressions: 500}
	got := generateInsight(o)
	want := "Your best posts get the most replies. Aim to end every post with a direct question."
	if got != want {
		t.Errorf("exactly 500 impressions: got %q, want %q", got, want)
	}
}

func TestGenerateInsight_ZeroImpressions(t *testing.T) {
	o := AnalyticsOverview{PostsAnalyzed: 3, AvgEngRate: 0, TotalImpressions: 0}
	got := generateInsight(o)
	want := "Impressions are low. Posting daily for the next 7 days is the fastest way to grow reach."
	if got != want {
		t.Errorf("zero impressions: got %q, want %q", got, want)
	}
}
