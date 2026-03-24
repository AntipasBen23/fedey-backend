package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/AntipasBen23/fedey-backend/internal/brandmemory"
	"github.com/AntipasBen23/fedey-backend/internal/trends"
)

type hypothesis struct {
	ID         string  `json:"id"`
	Statement  string  `json:"statement"`
	Channel    string  `json:"channel"`
	Confidence float64 `json:"confidence"`
}

type strategyRecommendation struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Detail      string `json:"detail"`
	ImpactScore int    `json:"impactScore"`
}

type strategySnapshotResponse struct {
	Hypotheses      []hypothesis             `json:"hypotheses"`
	Recommendations []strategyRecommendation `json:"recommendations"`
}

type StrategyHandler struct {
	brandMemoryService *brandmemory.Service
	trendService       *trends.Service
}

func NewStrategyHandler(
	brandMemoryService *brandmemory.Service,
	trendService *trends.Service,
) *StrategyHandler {
	return &StrategyHandler{
		brandMemoryService: brandMemoryService,
		trendService:       trendService,
	}
}

func (h *StrategyHandler) GetSnapshot(w http.ResponseWriter, r *http.Request) {
	profile, err := h.brandMemoryService.Get(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load brand memory")
		return
	}

	signals, err := h.trendService.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load trends")
		return
	}

	response := strategySnapshotResponse{
		Hypotheses:      buildHypotheses(profile, signals),
		Recommendations: buildRecommendations(profile, signals),
	}

	writeJSON(w, http.StatusOK, response)
}

func buildHypotheses(profile brandmemory.Profile, signals []trends.Signal) []hypothesis {
	if len(signals) == 0 {
		return []hypothesis{
			{
				ID:         "hyp-default-1",
				Statement:  fmt.Sprintf("%s should post a brand-defining educational thread this week.", profile.BrandName),
				Channel:    "x",
				Confidence: 0.61,
			},
		}
	}

	limit := 2
	if len(signals) < limit {
		limit = len(signals)
	}

	items := make([]hypothesis, 0, limit)
	for index := 0; index < limit; index++ {
		signal := signals[index]
		channel := preferredChannel(signal.Source)
		items = append(items, hypothesis{
			ID: fmt.Sprintf("hyp-%d", index+1),
			Statement: fmt.Sprintf(
				"Use %s to connect '%s' to %s for %s.",
				channelLabel(channel),
				signal.Topic,
				firstPillar(profile.Pillars),
				audienceFragment(profile.Audience),
			),
			Channel:    channel,
			Confidence: hypothesisConfidence(signal.Relevance, signal.Velocity),
		})
	}

	return items
}

func buildRecommendations(profile brandmemory.Profile, signals []trends.Signal) []strategyRecommendation {
	if len(signals) == 0 {
		return []strategyRecommendation{
			{
				ID:          "rec-default-1",
				Title:       "Capture baseline demand",
				Detail:      "Trend ingestion is empty, so prioritize evergreen posts around your strongest pillar.",
				ImpactScore: 58,
			},
		}
	}

	topSignal := signals[0]
	recommendations := []strategyRecommendation{
		{
			ID:          "rec-1",
			Title:       "React while the signal is warm",
			Detail:      fmt.Sprintf("%s is surfacing on %s with high relevance. Publish a %s angle within the next cycle.", topSignal.Topic, strings.ToUpper(topSignal.Source), firstPillar(profile.Pillars)),
			ImpactScore: impactScore(topSignal),
		},
	}

	if len(profile.Guardrails) > 0 {
		recommendations = append(recommendations, strategyRecommendation{
			ID:          "rec-2",
			Title:       "Keep the brand constraints visible",
			Detail:      fmt.Sprintf("Any trend response should still respect guardrails like '%s'.", profile.Guardrails[0]),
			ImpactScore: 71,
		})
	}

	return recommendations
}

func preferredChannel(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "linkedin":
		return "linkedin"
	case "instagram":
		return "instagram"
	case "tiktok":
		return "tiktok"
	default:
		return "x"
	}
}

func channelLabel(channel string) string {
	switch channel {
	case "linkedin":
		return "a LinkedIn post"
	case "instagram":
		return "an Instagram carousel"
	case "tiktok":
		return "a TikTok short"
	default:
		return "an X thread"
	}
}

func firstPillar(pillars []string) string {
	if len(pillars) == 0 {
		return "your strongest positioning"
	}

	return pillars[0]
}

func audienceFragment(audience string) string {
	if strings.TrimSpace(audience) == "" {
		return "your audience"
	}

	return audience
}

func hypothesisConfidence(relevance float64, velocity int) float64 {
	confidence := relevance*0.75 + minFloat(float64(velocity)/100, 1)*0.25
	if confidence > 0.95 {
		return 0.95
	}
	if confidence < 0.3 {
		return 0.3
	}

	return confidence
}

func impactScore(signal trends.Signal) int {
	score := int(signal.Relevance*70) + minInt(signal.Velocity, 30)
	if score > 99 {
		return 99
	}
	if score < 40 {
		return 40
	}

	return score
}

func minFloat(left, right float64) float64 {
	if left < right {
		return left
	}
	return right
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
