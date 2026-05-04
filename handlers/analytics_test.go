package handlers

import "testing"

// ─── impactScore ─────────────────────────────────────────────────────────────

func TestImpactScore_ZeroEngNoImpressions(t *testing.T) {
	got := impactScore(0, 0)
	if got != 10 {
		t.Errorf("expected minimum 10, got %d", got)
	}
}

func TestImpactScore_NeverExceeds100(t *testing.T) {
	got := impactScore(100, 100000)
	if got > 100 {
		t.Errorf("impact score capped at 100, got %d", got)
	}
	if got != 100 {
		t.Errorf("expected 100 for very high engagement, got %d", got)
	}
}

func TestImpactScore_NeverBelow10(t *testing.T) {
	got := impactScore(0, 0)
	if got < 10 {
		t.Errorf("impact score floored at 10, got %d", got)
	}
}

func TestImpactScore_HighImpressionsBonus(t *testing.T) {
	// engRate=1 → base = int(1*12) = 12; impressions > 5000 → +15 → 27
	got := impactScore(1.0, 6000)
	if got != 27 {
		t.Errorf("expected 27 with >5000 impressions bonus, got %d", got)
	}
}

func TestImpactScore_MidImpressionsBonus(t *testing.T) {
	// engRate=1 → base=12; impressions > 1000 → +8 → 20
	got := impactScore(1.0, 2000)
	if got != 20 {
		t.Errorf("expected 20 with >1000 impressions bonus, got %d", got)
	}
}

func TestImpactScore_LowImpressionsNoBonus(t *testing.T) {
	// engRate=1 → base=12; impressions <= 1000 → no bonus → 12
	got := impactScore(1.0, 500)
	if got != 12 {
		t.Errorf("expected 12 with no impressions bonus, got %d", got)
	}
}

// ─── generateInsight ─────────────────────────────────────────────────────────

func TestGenerateInsight_NoPostsAnalyzed(t *testing.T) {
	o := AnalyticsOverview{PostsAnalyzed: 0}
	got := generateInsight(o)
	want := "Post your first piece of content to start tracking performance."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGenerateInsight_HighEngagementRate(t *testing.T) {
	o := AnalyticsOverview{PostsAnalyzed: 5, AvgEngRate: 6.0}
	got := generateInsight(o)
	want := "Your engagement rate is above average — keep the posting frequency consistent."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGenerateInsight_SolidEngagementRate(t *testing.T) {
	o := AnalyticsOverview{PostsAnalyzed: 5, AvgEngRate: 3.0}
	got := generateInsight(o)
	want := "Solid engagement. Try more threads or carousels — they typically outperform single tweets."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGenerateInsight_LowImpressions(t *testing.T) {
	o := AnalyticsOverview{PostsAnalyzed: 5, AvgEngRate: 1.0, TotalImpressions: 200}
	got := generateInsight(o)
	want := "Impressions are low. Posting daily for the next 7 days is the fastest way to grow reach."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGenerateInsight_Default(t *testing.T) {
	o := AnalyticsOverview{PostsAnalyzed: 5, AvgEngRate: 1.0, TotalImpressions: 1000}
	got := generateInsight(o)
	want := "Your best posts get the most replies. Aim to end every post with a direct question."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGenerateInsight_ExactlyAtBoundary5(t *testing.T) {
	// AvgEngRate == 5.0 should hit the >= 5 branch
	o := AnalyticsOverview{PostsAnalyzed: 1, AvgEngRate: 5.0}
	got := generateInsight(o)
	want := "Your engagement rate is above average — keep the posting frequency consistent."
	if got != want {
		t.Errorf("boundary 5.0: got %q, want %q", got, want)
	}
}

func TestGenerateInsight_ExactlyAtBoundary2(t *testing.T) {
	// AvgEngRate == 2.0 should hit the >= 2 branch
	o := AnalyticsOverview{PostsAnalyzed: 1, AvgEngRate: 2.0}
	got := generateInsight(o)
	want := "Solid engagement. Try more threads or carousels — they typically outperform single tweets."
	if got != want {
		t.Errorf("boundary 2.0: got %q, want %q", got, want)
	}
}
