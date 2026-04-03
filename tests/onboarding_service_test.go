package tests

import (
	"context"
	"testing"

	"github.com/AntipasBen23/fedey-backend/internal/brandmemory"
	openai "github.com/AntipasBen23/fedey-backend/internal/llm/openai"
	"github.com/AntipasBen23/fedey-backend/internal/onboarding"
)

type fakeChatResolver struct {
	resolution openai.OnboardingResolution
}

func (f fakeChatResolver) Configured() bool {
	return true
}

func (f fakeChatResolver) ResolveOnboardingChat(_ context.Context, _ string, _ []openai.Message) (openai.OnboardingResolution, error) {
	return f.resolution, nil
}

func TestCreateSessionSeedsChatQuestionsAndBrandMemory(t *testing.T) {
	t.Parallel()

	repo := onboarding.NewMemoryRepository()
	brandService := brandmemory.NewService(brandmemory.NewMemoryRepository())
	service := onboarding.NewService(repo, brandService, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	session, err := service.CreateSession(context.Background(), onboarding.CreateSessionInput{
		Title:           "Founder Social Manager",
		JobDescription:  "Manage social media for our founder account and grow authority on X.",
		AccountMode:     "new",
		PrimaryPlatform: "x",
		BrandName:       "Fedey",
		ReviewMode:      "manual",
	})
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}

	if len(session.ChatMessages) == 0 {
		t.Fatalf("expected seeded chat messages")
	}
	if session.ChatMessages[0].Role != "assistant" {
		t.Fatalf("expected first chat message to come from assistant, got %q", session.ChatMessages[0].Role)
	}
	if len(session.Questions) == 0 {
		t.Fatalf("expected generated onboarding questions")
	}

	profile, err := brandService.Get(context.Background())
	if err != nil {
		t.Fatalf("brandService.Get returned error: %v", err)
	}
	if profile.BrandName != "Fedey" {
		t.Fatalf("expected brand memory to be synced with Fedey, got %q", profile.BrandName)
	}
}

func TestChatResolvesAnswersAndAppendsConversation(t *testing.T) {
	t.Parallel()

	repo := onboarding.NewMemoryRepository()
	brandService := brandmemory.NewService(brandmemory.NewMemoryRepository())
	resolver := fakeChatResolver{
		resolution: openai.OnboardingResolution{
			AssistantMessage: "Clear. I’ll use that as the primary audience and keep moving.",
			ResolvedAnswers: []openai.ResolvedQuestion{
				{QuestionID: "q-audience", Answer: "Founders and operators scaling service businesses"},
			},
		},
	}
	service := onboarding.NewService(repo, brandService, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, resolver)

	session := onboarding.Session{
		ID:              "onb-1",
		Title:           "Fedey Social",
		JobDescription:  "Help us grow on X and LinkedIn",
		AccountMode:     "new",
		Objective:       "authority",
		PrimaryPlatform: "x",
		BrandName:       "Fedey",
		Audience:        "",
		VoiceSummary:    "clear and strategic",
		ReviewMode:      "manual",
		ApprovalStatus:  "not_started",
		ChatMessages: []onboarding.ChatMessage{
			{ID: "m-1", Role: "assistant", Content: "Who exactly should I speak to?"},
		},
		Status: "interview",
	}
	if err := repo.CreateSession(context.Background(), session); err != nil {
		t.Fatalf("CreateSession on repo returned error: %v", err)
	}
	if err := repo.UpsertQuestion(context.Background(), onboarding.Question{
		ID:        "q-audience",
		SessionID: session.ID,
		Prompt:    "Who exactly should the agent speak to, and what does that audience care about most?",
		Category:  "audience",
		Required:  true,
	}); err != nil {
		t.Fatalf("UpsertQuestion returned error: %v", err)
	}

	updated, err := service.Chat(context.Background(), onboarding.ChatInput{
		SessionID: session.ID,
		Message:   "Speak to founders and operators scaling service businesses.",
	})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}

	question, err := repo.GetQuestion(context.Background(), session.ID, "q-audience")
	if err != nil {
		t.Fatalf("GetQuestion returned error: %v", err)
	}
	if question.Answer == "" {
		t.Fatalf("expected question answer to be inferred from chat")
	}
	if len(updated.ChatMessages) < 3 {
		t.Fatalf("expected user and assistant chat messages to be appended, got %d", len(updated.ChatMessages))
	}
	if updated.ChatMessages[len(updated.ChatMessages)-1].Role != "assistant" {
		t.Fatalf("expected last chat message to be assistant")
	}
}
