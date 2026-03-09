package handlers

import (
	"encoding/json"
	"net/http"
)

type hypothesis struct {
	ID         string  `json:"id"`
	Statement  string  `json:"statement"`
	Channel    string  `json:"channel"`
	Confidence float64 `json:"confidence"`
}

type experimentSnapshot struct {
	ID           string `json:"id"`
	HypothesisID string `json:"hypothesisId"`
	Metric       string `json:"metric"`
	Status       string `json:"status"`
}

type strategyRecommendation struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Detail      string `json:"detail"`
	ImpactScore int    `json:"impactScore"`
}

type strategySnapshotResponse struct {
	Hypotheses      []hypothesis             `json:"hypotheses"`
	Experiments     []experimentSnapshot     `json:"experiments"`
	Recommendations []strategyRecommendation `json:"recommendations"`
}

func StrategySnapshotV1(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	response := strategySnapshotResponse{
		Hypotheses: []hypothesis{
			{
				ID:         "hyp-01",
				Statement:  "Educational threads will outperform product-first posts this week",
				Channel:    "x",
				Confidence: 0.78,
			},
			{
				ID:         "hyp-02",
				Statement:  "Posting short carousel explainers at noon will improve saves",
				Channel:    "instagram",
				Confidence: 0.71,
			},
		},
		Experiments: []experimentSnapshot{
			{
				ID:           "exp-11",
				HypothesisID: "hyp-01",
				Metric:       "engagement_rate",
				Status:       "running",
			},
			{
				ID:           "exp-12",
				HypothesisID: "hyp-02",
				Metric:       "save_rate",
				Status:       "draft",
			},
		},
		Recommendations: []strategyRecommendation{
			{
				ID:          "rec-4",
				Title:       "Increase educational share",
				Detail:      "Accounts in your segment saw 1.9x higher reach with educational content this week.",
				ImpactScore: 89,
			},
			{
				ID:          "rec-5",
				Title:       "Compress publish window",
				Detail:      "Top variants peaked between 11:30 and 13:00 local time.",
				ImpactScore: 72,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}
