package content

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/AntipasBen23/fedey-backend/internal/brandmemory"
	"github.com/AntipasBen23/fedey-backend/internal/experiments"
	"github.com/AntipasBen23/fedey-backend/internal/trends"
)

var ErrInvalidVariantRequest = errors.New("invalid content variant generation request")

type Service struct {
	repository         Repository
	brandMemoryService *brandmemory.Service
	trendService       *trends.Service
	experimentService  *experiments.Service
}

func NewService(
	repository Repository,
	brandMemoryService *brandmemory.Service,
	trendService *trends.Service,
	experimentService *experiments.Service,
) *Service {
	return &Service{
		repository:         repository,
		brandMemoryService: brandMemoryService,
		trendService:       trendService,
		experimentService:  experimentService,
	}
}

func (s *Service) List(ctx context.Context) ([]Draft, error) {
	return s.repository.List(ctx)
}

func (s *Service) GetByID(ctx context.Context, draftID string) (Draft, error) {
	if strings.TrimSpace(draftID) == "" {
		return Draft{}, ErrInvalidVariantRequest
	}

	return s.repository.GetByID(ctx, strings.TrimSpace(draftID))
}

func (s *Service) Generate(ctx context.Context) ([]Draft, error) {
	profile, err := s.brandMemoryService.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("get brand memory: %w", err)
	}

	signals, err := s.trendService.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list trend signals: %w", err)
	}

	drafts := buildDrafts(profile, signals)
	if err := s.repository.SaveBatch(ctx, drafts); err != nil {
		return nil, err
	}

	return drafts, nil
}

func (s *Service) GenerateVariants(ctx context.Context, draftID string) (Draft, error) {
	if strings.TrimSpace(draftID) == "" {
		return Draft{}, ErrInvalidVariantRequest
	}

	draft, err := s.repository.GetByID(ctx, strings.TrimSpace(draftID))
	if err != nil {
		return Draft{}, err
	}

	if draft.ExperimentID != "" && len(draft.Variants) > 0 {
		return draft, nil
	}

	experiment, err := s.experimentService.Create(ctx, experiments.CreateInput{
		HypothesisID: "content-" + draft.ID,
		Metric:       "engagement_rate",
	})
	if err != nil {
		return Draft{}, fmt.Errorf("create linked experiment: %w", err)
	}

	variants := buildVariants(draft)
	draft.ExperimentID = experiment.ID
	draft.Variants = variants
	draft.Status = "variant_ready"

	if err := s.repository.Update(ctx, draft); err != nil {
		return Draft{}, err
	}
	if err := s.repository.SaveVariants(ctx, experiment.ID, variants); err != nil {
		return Draft{}, err
	}

	return draft, nil
}

func (s *Service) SaveBatch(ctx context.Context, drafts []Draft) error {
	if len(drafts) == 0 {
		return nil
	}

	return s.repository.SaveBatch(ctx, drafts)
}

func (s *Service) UpdateDraft(ctx context.Context, draft Draft) error {
	if strings.TrimSpace(draft.ID) == "" {
		return ErrInvalidVariantRequest
	}

	return s.repository.Update(ctx, draft)
}

func buildDrafts(profile brandmemory.Profile, signals []trends.Signal) []Draft {
	now := time.Now().UTC()
	if len(signals) == 0 {
		return []Draft{
			{
				ID:          "draft-" + uuid.NewString(),
				Channel:     "x",
				Hook:        fmt.Sprintf("%s is not just a tool idea. It is an operator for growth.", profile.BrandName),
				Body:        fmt.Sprintf("Most teams still treat AI like a caption helper. We are building %s to behave like a social media manager that observes, plans, experiments, and learns.", profile.BrandName),
				Rationale:   "Evergreen draft generated because no trend signals are available.",
				SourceTrend: "evergreen",
				Status:      "draft",
				CreatedAt:   now,
			},
		}
	}

	limit := 3
	if len(signals) < limit {
		limit = len(signals)
	}

	channels := []string{"x", "linkedin", "instagram"}
	drafts := make([]Draft, 0, limit)
	for index := 0; index < limit; index++ {
		signal := signals[index]
		channel := channels[index%len(channels)]

		drafts = append(drafts, Draft{
			ID:          "draft-" + uuid.NewString(),
			Channel:     channel,
			Hook:        fmt.Sprintf("Trend signal: %s is opening a content angle for %s.", signal.Topic, profile.BrandName),
			Body:        buildBody(profile, signal, channel),
			Rationale:   fmt.Sprintf("Generated from %s with relevance %.0f%% and velocity %d.", signal.Source, signal.Relevance*100, signal.Velocity),
			SourceTrend: signal.Topic,
			Status:      "draft",
			CreatedAt:   now.Add(time.Duration(index) * time.Minute),
		})
	}

	return drafts
}

func buildBody(profile brandmemory.Profile, signal trends.Signal, channel string) string {
	switch channel {
	case "linkedin":
		return fmt.Sprintf(
			"Teams are starting to ask a sharper question: what does it look like when %s is handled by an AI operator instead of a content assistant? We think the answer is a system that can observe %s, turn that into experiments, and learn what the audience actually responds to.",
			firstValue(profile.Pillars, "social growth"),
			signal.Topic,
		)
	case "instagram":
		return fmt.Sprintf(
			"Carousel idea:\n1. %s is trending\n2. Why it matters to %s\n3. The experiment we would run this week\n4. The guardrail: %s",
			signal.Topic,
			profile.Audience,
			firstValue(profile.Guardrails, "Stay useful, not spammy."),
		)
	default:
		return fmt.Sprintf(
			"People are talking about %s.\n\nHere is the interesting part: the real opportunity is not just posting about it. It is building a system that can test angles around it, measure response, and compound what works for %s.",
			signal.Topic,
			profile.BrandName,
		)
	}
}

func firstValue(items []string, fallback string) string {
	if len(items) == 0 {
		return fallback
	}

	return items[0]
}

func buildVariants(draft Draft) []Variant {
	return []Variant{
		{
			Label: "A",
			Hook:  draft.Hook,
			Body:  draft.Body,
			Angle: "Baseline explanatory angle",
		},
		{
			Label: "B",
			Hook:  "What most teams miss about this trend",
			Body:  draft.Body + "\n\nThe test here is whether a sharper contrarian opening earns more engagement.",
			Angle: "Contrarian hook",
		},
		{
			Label: "C",
			Hook:  "A practical playbook for acting on this signal",
			Body:  draft.Body + "\n\nThis version leans into operator steps and implementation detail.",
			Angle: "Tactical playbook hook",
		},
	}
}
