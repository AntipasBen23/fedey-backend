package onboarding

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/AntipasBen23/fedey-backend/internal/brandmemory"
	"github.com/AntipasBen23/fedey-backend/internal/content"
	"github.com/AntipasBen23/fedey-backend/internal/linkedinaccounts"
	openai "github.com/AntipasBen23/fedey-backend/internal/llm/openai"
	"github.com/AntipasBen23/fedey-backend/internal/performance"
	linkedinplatform "github.com/AntipasBen23/fedey-backend/internal/platform/linkedin"
	xplatform "github.com/AntipasBen23/fedey-backend/internal/platform/x"
	"github.com/AntipasBen23/fedey-backend/internal/publishing"
	"github.com/AntipasBen23/fedey-backend/internal/xaccounts"
)

type Service struct {
	repository         Repository
	brandMemoryService *brandmemory.Service
	contentService     *content.Service
	performanceService *performance.Service
	publishingService  *publishing.Service
	activationWindows  []publishing.Window
	xClient            *xplatform.Client
	xAccountService    *xaccounts.Service
	linkedinClient     *linkedinplatform.Client
	linkedinService    *linkedinaccounts.Service
	chatResolver       chatResolver
}

type chatResolver interface {
	Configured() bool
	ResolveOnboardingChat(ctx context.Context, systemPrompt string, messages []openai.Message) (openai.OnboardingResolution, error)
}

func NewService(
	repository Repository,
	brandMemoryService *brandmemory.Service,
	contentService *content.Service,
	performanceService *performance.Service,
	publishingService *publishing.Service,
	activationWindows []publishing.Window,
	xClient *xplatform.Client,
	xAccountService *xaccounts.Service,
	linkedinClient *linkedinplatform.Client,
	linkedinService *linkedinaccounts.Service,
	chatResolver chatResolver,
) *Service {
	return &Service{
		repository:         repository,
		brandMemoryService: brandMemoryService,
		contentService:     contentService,
		performanceService: performanceService,
		publishingService:  publishingService,
		activationWindows:  append([]publishing.Window(nil), activationWindows...),
		xClient:            xClient,
		xAccountService:    xAccountService,
		linkedinClient:     linkedinClient,
		linkedinService:    linkedinService,
		chatResolver:       chatResolver,
	}
}

func (s *Service) List(ctx context.Context) ([]Session, error) {
	return s.repository.List(ctx)
}

func (s *Service) CreateSession(ctx context.Context, input CreateSessionInput) (Session, error) {
	if strings.TrimSpace(input.JobDescription) == "" || strings.TrimSpace(input.AccountMode) == "" {
		return Session{}, ErrInvalidSessionInput
	}

	now := time.Now().UTC()
	session := Session{
		ID:              "onb-" + uuid.NewString(),
		Title:           firstNonEmpty(strings.TrimSpace(input.Title), inferTitle(input.JobDescription, input.BrandName)),
		JobDescription:  strings.TrimSpace(input.JobDescription),
		AccountMode:     normalizeAccountMode(input.AccountMode),
		Objective:       firstNonEmpty(strings.TrimSpace(input.Objective), inferObjective(input.JobDescription)),
		PrimaryPlatform: firstNonEmpty(strings.TrimSpace(input.PrimaryPlatform), inferPlatform(input.JobDescription)),
		BrandName:       strings.TrimSpace(input.BrandName),
		Audience:        strings.TrimSpace(input.Audience),
		VoiceSummary:    inferVoice(input.JobDescription),
		Constraints:     normalizeStrings(input.Constraints),
		ReviewMode:      normalizeReviewMode(input.ReviewMode),
		ApprovalStatus:  initialApprovalStatus(input.ReviewMode),
		ChatMessages:    seedChatMessages(input.JobDescription, input.BrandName),
		Audit: AuditReport{
			Status: "not_started",
		},
		Activation: ActivationPlan{
			Status: "not_started",
		},
		History: []HistoryEntry{
			newHistoryEntry("system", "session_created", "Created onboarding session from hiring brief."),
		},
		Status:    StatusInterview,
		CreatedAt: now,
		UpdatedAt: now,
	}

	questions := buildQuestions(session)
	if session.AccountMode == "existing" {
		session.Status = StatusAuditReady
	}

	if err := s.repository.CreateSession(ctx, session); err != nil {
		return Session{}, err
	}
	for _, question := range questions {
		if err := s.repository.UpsertQuestion(ctx, question); err != nil {
			return Session{}, err
		}
	}
	session.Questions = questions
	if _, err := s.syncBrandMemory(ctx, session); err != nil {
		return Session{}, err
	}
	session.History = append(session.History, newHistoryEntry("system", "brand_memory_synced", "Synced initial brand memory from onboarding inputs."))
	if err := s.repository.UpdateSession(ctx, session); err != nil {
		return Session{}, err
	}
	return s.repository.GetSession(ctx, session.ID)
}

func (s *Service) Chat(ctx context.Context, input ChatInput) (Session, error) {
	if strings.TrimSpace(input.SessionID) == "" || strings.TrimSpace(input.Message) == "" {
		return Session{}, ErrInvalidSessionInput
	}
	if s.chatResolver == nil || !s.chatResolver.Configured() {
		return Session{}, ErrChatUnavailable
	}

	session, err := s.repository.GetSession(ctx, input.SessionID)
	if err != nil {
		return Session{}, err
	}

	now := time.Now().UTC()
	userMessage := ChatMessage{
		ID:        "chat-" + uuid.NewString(),
		Role:      "user",
		Content:   strings.TrimSpace(input.Message),
		CreatedAt: now,
	}
	session.ChatMessages = append(session.ChatMessages, userMessage)

	resolution, err := s.chatResolver.ResolveOnboardingChat(
		ctx,
		buildOnboardingSystemPrompt(session),
		buildChatTranscript(session),
	)
	if err != nil {
		return Session{}, err
	}

	for _, item := range resolution.ResolvedAnswers {
		if err := s.applyResolvedAnswer(ctx, &session, item.QuestionID, item.Answer); err != nil {
			return Session{}, err
		}
	}
	session.Questions, err = s.repository.ListQuestions(ctx, session.ID)
	if err != nil {
		return Session{}, err
	}

	if allRequiredAnswered(session.Questions) {
		if session.AccountMode == "existing" {
			session.Status = StatusAuditReady
		} else {
			session.Status = StatusReady
		}
	}

	session.ChatMessages = append(session.ChatMessages, ChatMessage{
		ID:        "chat-" + uuid.NewString(),
		Role:      "assistant",
		Content:   strings.TrimSpace(resolution.AssistantMessage),
		CreatedAt: time.Now().UTC(),
	})
	session.History = append(session.History,
		newHistoryEntry("hirer", "chat_message_received", "Sent an onboarding chat message."),
		newHistoryEntry("agent", "chat_response_generated", "Generated an onboarding chat reply and updated inferred answers."),
	)
	session.UpdatedAt = time.Now().UTC()
	if err := s.repository.UpdateSession(ctx, session); err != nil {
		return Session{}, err
	}
	if _, err := s.syncBrandMemory(ctx, session); err != nil {
		return Session{}, err
	}
	session, err = s.repository.GetSession(ctx, session.ID)
	if err != nil {
		return Session{}, err
	}
	session.History = append(session.History, newHistoryEntry("system", "brand_memory_synced", "Updated brand memory from onboarding chat."))
	session.UpdatedAt = time.Now().UTC()
	if err := s.repository.UpdateSession(ctx, session); err != nil {
		return Session{}, err
	}
	return s.repository.GetSession(ctx, session.ID)
}

func (s *Service) AnswerQuestion(ctx context.Context, input AnswerQuestionInput) (Session, error) {
	if strings.TrimSpace(input.SessionID) == "" || strings.TrimSpace(input.QuestionID) == "" || strings.TrimSpace(input.Answer) == "" {
		return Session{}, ErrInvalidSessionInput
	}

	question, err := s.repository.GetQuestion(ctx, input.SessionID, input.QuestionID)
	if err != nil {
		return Session{}, err
	}
	question.Answer = strings.TrimSpace(input.Answer)
	question.AnsweredAt = time.Now().UTC()
	if err := s.repository.UpsertQuestion(ctx, question); err != nil {
		return Session{}, err
	}

	session, err := s.repository.GetSession(ctx, input.SessionID)
	if err != nil {
		return Session{}, err
	}
	session.Questions, _ = s.repository.ListQuestions(ctx, session.ID)
	if allRequiredAnswered(session.Questions) && session.AccountMode == "new" {
		session.Status = StatusReady
	}
	session.History = append(session.History, newHistoryEntry("hirer", "question_answered", "Saved onboarding answer for "+question.Category+"."))
	session.UpdatedAt = time.Now().UTC()
	if err := s.repository.UpdateSession(ctx, session); err != nil {
		return Session{}, err
	}
	if _, err := s.syncBrandMemory(ctx, session); err != nil {
		return Session{}, err
	}
	session, err = s.repository.GetSession(ctx, session.ID)
	if err != nil {
		return Session{}, err
	}
	session.History = append(session.History, newHistoryEntry("system", "brand_memory_synced", "Updated brand memory from onboarding answers."))
	session.UpdatedAt = time.Now().UTC()
	if err := s.repository.UpdateSession(ctx, session); err != nil {
		return Session{}, err
	}
	return s.repository.GetSession(ctx, session.ID)
}

func (s *Service) applyResolvedAnswer(ctx context.Context, session *Session, questionID, answer string) error {
	if strings.TrimSpace(questionID) == "" || strings.TrimSpace(answer) == "" {
		return nil
	}

	question, err := s.repository.GetQuestion(ctx, session.ID, questionID)
	if err != nil {
		if err == ErrQuestionNotFound {
			return nil
		}
		return err
	}
	question.Answer = strings.TrimSpace(answer)
	question.AnsweredAt = time.Now().UTC()
	if err := s.repository.UpsertQuestion(ctx, question); err != nil {
		return err
	}
	session.History = append(session.History, newHistoryEntry("agent", "question_inferred_from_chat", "Captured an onboarding answer from the live chat for "+question.Category+"."))
	return nil
}

func (s *Service) UpdateReviewMode(ctx context.Context, input UpdateReviewModeInput) (Session, error) {
	if strings.TrimSpace(input.SessionID) == "" {
		return Session{}, ErrInvalidSessionInput
	}

	session, err := s.repository.GetSession(ctx, input.SessionID)
	if err != nil {
		return Session{}, err
	}
	session.ReviewMode = normalizeReviewMode(input.ReviewMode)
	if session.ReviewMode == "auto" {
		if session.Activation.Status == "generated" && !hasScheduledActivationDraft(session.Activation.Drafts) {
			session.Activation.Drafts, err = s.scheduleActivationDrafts(ctx, session, session.Activation.Drafts)
			if err != nil {
				return Session{}, err
			}
			session.Activation.Status = "scheduled"
		}
		session.ApprovalStatus = "not_required"
		if session.Activation.Status == "generated" || session.Activation.Status == "approved" || session.Activation.Status == "scheduled" {
			session.Status = StatusActivated
		}
	} else {
		if session.Activation.Status == "generated" && session.Status == StatusActivated {
			session.Status = StatusAwaitingApproval
		}
		if session.Activation.Status == "generated" {
			session.ApprovalStatus = "pending"
		} else {
			session.ApprovalStatus = "not_started"
		}
	}
	session.History = append(session.History, newHistoryEntry("hirer", "review_mode_updated", "Changed review mode to "+session.ReviewMode+"."))
	session.UpdatedAt = time.Now().UTC()
	if err := s.repository.UpdateSession(ctx, session); err != nil {
		return Session{}, err
	}
	return s.repository.GetSession(ctx, session.ID)
}

func (s *Service) RunAudit(ctx context.Context, sessionID string) (Session, error) {
	session, err := s.repository.GetSession(ctx, sessionID)
	if err != nil {
		return Session{}, err
	}
	if session.AccountMode != "existing" {
		return Session{}, ErrInvalidSessionInput
	}

	report := AuditReport{
		Status:    "in_progress",
		LastRunAt: time.Now().UTC(),
	}

	if s.performanceService != nil {
		_, _ = s.performanceService.SyncConnectedAccounts(ctx)
	}

	if account, err := s.resolveXCredentials(ctx); err == nil && s.xClient != nil {
		if posts, err := s.xClient.FetchUserPostsWithToken(ctx, account.AccessToken, account.UserID, 15); err == nil {
			report.ConnectedPlatforms = append(report.ConnectedPlatforms, "x")
			report.PostsReviewed += len(posts)
			report.ContentPatterns = append(report.ContentPatterns, deriveXContentPatterns(posts)...)
			report.ReplyPatterns = append(report.ReplyPatterns, deriveXReplyPatterns(posts)...)
			report.Recommendations = append(report.Recommendations, deriveXRecommendations(posts, session)...)
			report.PerformanceInsights = append(report.PerformanceInsights, deriveXPerformanceInsights(posts)...)
		}
	}

	if account, err := s.resolveLinkedInCredentials(ctx); err == nil && s.linkedinClient != nil {
		if posts, err := s.linkedinClient.ListAuthorPosts(ctx, account.AccessToken, account.AuthorURN, 10); err == nil {
			report.ConnectedPlatforms = append(report.ConnectedPlatforms, "linkedin")
			report.PostsReviewed += len(posts)
			report.ContentPatterns = append(report.ContentPatterns, deriveLinkedInContentPatterns(posts)...)
			replyPatterns, recommendations := s.deriveLinkedInConversationPatterns(ctx, account, posts)
			report.ReplyPatterns = append(report.ReplyPatterns, replyPatterns...)
			report.Recommendations = append(report.Recommendations, recommendations...)
			report.PerformanceInsights = append(report.PerformanceInsights, deriveLinkedInPerformanceInsights(ctx, account, posts, s.linkedinClient)...)
		}
	}

	if s.performanceService != nil {
		if insights, err := s.performanceService.Insights(ctx, "x"); err == nil {
			report.PerformanceInsights = append(report.PerformanceInsights, insights...)
		}
		if insights, err := s.performanceService.Insights(ctx, "linkedin"); err == nil {
			report.PerformanceInsights = append(report.PerformanceInsights, insights...)
		}
	}

	report.ConnectedPlatforms = uniqueStrings(report.ConnectedPlatforms)
	report.ContentPatterns = uniqueStrings(report.ContentPatterns)
	report.ReplyPatterns = uniqueStrings(report.ReplyPatterns)
	report.Recommendations = uniqueStrings(report.Recommendations)
	report.PerformanceInsights = uniqueStrings(report.PerformanceInsights)

	if len(report.ConnectedPlatforms) == 0 {
		report.Status = "waiting_for_connections"
		report.Recommendations = []string{
			"Connect an existing X or LinkedIn account so the agent can learn from historical posts and replies.",
			"Answer the follow-up questions so the onboarding model has enough context to build a brand profile.",
		}
	} else {
		report.Status = "completed"
	}

	session.Audit = report
	session.History = append(session.History, newHistoryEntry("system", "audit_completed", fmt.Sprintf("Completed onboarding audit across %s.", strings.Join(report.ConnectedPlatforms, ", "))))
	session.UpdatedAt = time.Now().UTC()
	if report.Status == "completed" && allRequiredAnswered(session.Questions) {
		session.Status = StatusReady
	}
	if err := s.repository.UpdateSession(ctx, session); err != nil {
		return Session{}, err
	}
	if _, err := s.syncBrandMemory(ctx, session); err != nil {
		return Session{}, err
	}
	return s.repository.GetSession(ctx, session.ID)
}

func (s *Service) Activate(ctx context.Context, sessionID string) (Session, error) {
	session, err := s.repository.GetSession(ctx, sessionID)
	if err != nil {
		return Session{}, err
	}
	if !allRequiredAnswered(session.Questions) {
		return Session{}, ErrInvalidSessionInput
	}

	session, err = s.syncBrandMemory(ctx, session)
	if err != nil {
		return Session{}, err
	}

	plan := session.Activation.WeekPlan
	if len(plan) == 0 {
		plan = buildWeekPlan(session)
	}
	drafts, err := s.syncActivationDrafts(ctx, session, plan)
	if err != nil {
		return Session{}, err
	}

	session.Activation = ActivationPlan{
		Status:          "generated",
		BrandMemorySync: true,
		WeekPlan:        plan,
		Drafts:          drafts,
		Summary:         buildActivationSummary(session),
		GeneratedAt:     time.Now().UTC(),
	}
	session.History = append(session.History, newHistoryEntry("system", "activation_generated", "Generated week-one plan and activation drafts for X and LinkedIn."))
	if session.ReviewMode == "manual" {
		session.ApprovalStatus = "pending"
		session.Status = StatusAwaitingApproval
	} else {
		session.Activation.Drafts, err = s.scheduleActivationDrafts(ctx, session, session.Activation.Drafts)
		if err != nil {
			return Session{}, err
		}
		session.Activation.Status = "scheduled"
		session.ApprovalStatus = "not_required"
		session.Status = StatusActivated
		session.History = append(session.History, newHistoryEntry("system", "activation_auto_scheduled", "Auto mode scheduled the first-week posts at performance-informed publish times."))
	}
	session.UpdatedAt = time.Now().UTC()
	if err := s.repository.UpdateSession(ctx, session); err != nil {
		return Session{}, err
	}
	return s.repository.GetSession(ctx, session.ID)
}

func (s *Service) ApproveActivation(ctx context.Context, sessionID string) (Session, error) {
	session, err := s.repository.GetSession(ctx, sessionID)
	if err != nil {
		return Session{}, err
	}
	if session.Activation.Status != "generated" {
		return Session{}, ErrInvalidSessionInput
	}

	scheduledDrafts, err := s.scheduleActivationDrafts(ctx, session, session.Activation.Drafts)
	if err != nil {
		return Session{}, err
	}

	session.ApprovalStatus = "approved"
	session.Status = StatusActivated
	session.Activation.Status = "approved"
	session.Activation.Drafts = scheduledDrafts
	session.History = append(session.History, newHistoryEntry("hirer", "activation_approved", "Approved activation and scheduled the first week of posts."))
	session.UpdatedAt = time.Now().UTC()
	if err := s.repository.UpdateSession(ctx, session); err != nil {
		return Session{}, err
	}
	return s.repository.GetSession(ctx, session.ID)
}

func (s *Service) UpdateActivationPlan(ctx context.Context, input UpdateActivationPlanInput) (Session, error) {
	if strings.TrimSpace(input.SessionID) == "" || len(input.WeekPlan) == 0 {
		return Session{}, ErrInvalidSessionInput
	}

	session, err := s.repository.GetSession(ctx, input.SessionID)
	if err != nil {
		return Session{}, err
	}
	if session.ApprovalStatus == "approved" || hasScheduledActivationDraft(session.Activation.Drafts) {
		return Session{}, ErrActivationLocked
	}
	if session.Activation.Status == "" || session.Activation.Status == "not_started" {
		return Session{}, ErrInvalidSessionInput
	}

	plan := normalizeActivationPlan(input.WeekPlan)
	if len(plan) == 0 {
		return Session{}, ErrInvalidSessionInput
	}

	drafts, err := s.syncActivationDrafts(ctx, session, plan)
	if err != nil {
		return Session{}, err
	}

	session.Activation.WeekPlan = plan
	session.Activation.Drafts = drafts
	session.Activation.Status = "generated"
	session.History = append(session.History, newHistoryEntry("hirer", "activation_plan_updated", "Updated the week-one activation plan before go-live."))
	session.UpdatedAt = time.Now().UTC()
	if session.ReviewMode == "manual" {
		session.ApprovalStatus = "pending"
		session.Status = StatusAwaitingApproval
	}
	if err := s.repository.UpdateSession(ctx, session); err != nil {
		return Session{}, err
	}
	return s.repository.GetSession(ctx, session.ID)
}

func (s *Service) UpdateActivationDrafts(ctx context.Context, input UpdateActivationDraftsInput) (Session, error) {
	if strings.TrimSpace(input.SessionID) == "" || len(input.Drafts) == 0 {
		return Session{}, ErrInvalidSessionInput
	}

	session, err := s.repository.GetSession(ctx, input.SessionID)
	if err != nil {
		return Session{}, err
	}
	if session.ApprovalStatus == "approved" || hasScheduledActivationDraft(session.Activation.Drafts) {
		return Session{}, ErrActivationLocked
	}

	currentByID := make(map[string]ActivationDraft, len(session.Activation.Drafts))
	for _, draft := range session.Activation.Drafts {
		currentByID[draft.DraftID] = draft
	}

	updatedDrafts := make([]ActivationDraft, 0, len(input.Drafts))
	for _, item := range input.Drafts {
		existing, ok := currentByID[item.DraftID]
		if !ok {
			continue
		}
		contentDraft, err := s.contentService.GetByID(ctx, existing.DraftID)
		if err != nil {
			return Session{}, err
		}

		channel := strings.ToLower(strings.TrimSpace(item.Channel))
		if channel != "x" && channel != "linkedin" {
			channel = existing.Channel
		}
		contentDraft.Channel = channel
		contentDraft.Hook = firstNonEmpty(strings.TrimSpace(item.Hook), contentDraft.Hook)
		contentDraft.Body = firstNonEmpty(strings.TrimSpace(item.Body), contentDraft.Body)
		contentDraft.Rationale = firstNonEmpty(strings.TrimSpace(item.Rationale), contentDraft.Rationale)
		if err := s.contentService.UpdateDraft(ctx, contentDraft); err != nil {
			return Session{}, err
		}

		existing.Channel = contentDraft.Channel
		existing.Hook = contentDraft.Hook
		existing.Body = contentDraft.Body
		existing.Rationale = contentDraft.Rationale
		updatedDrafts = append(updatedDrafts, existing)
	}

	if len(updatedDrafts) == 0 {
		return Session{}, ErrInvalidSessionInput
	}

	session.Activation.Drafts = mergeActivationDrafts(session.Activation.Drafts, updatedDrafts)
	session.History = append(session.History, newHistoryEntry("hirer", "activation_drafts_updated", "Edited activation draft copy before approval."))
	session.UpdatedAt = time.Now().UTC()
	if err := s.repository.UpdateSession(ctx, session); err != nil {
		return Session{}, err
	}
	return s.repository.GetSession(ctx, session.ID)
}

func buildQuestions(session Session) []Question {
	now := time.Now().UTC()
	prompts := inferQuestions(session)
	items := make([]Question, 0, len(prompts))
	for index, prompt := range prompts {
		items = append(items, Question{
			ID:        fmt.Sprintf("q-%d-%s", index+1, uuid.NewString()),
			SessionID: session.ID,
			Prompt:    prompt.Prompt,
			Category:  prompt.Category,
			Required:  prompt.Required,
			CreatedAt: now.Add(time.Duration(index) * time.Second),
		})
	}
	return items
}

type promptSpec struct {
	Prompt   string
	Category string
	Required bool
}

func inferQuestions(session Session) []promptSpec {
	items := []promptSpec{
		{Prompt: "Who exactly should the agent speak to, and what does that audience care about most?", Category: "audience", Required: session.Audience == ""},
		{Prompt: "What outcome matters most over the next 90 days: reach, qualified leads, authority, trust, or sales?", Category: "objective", Required: session.Objective == ""},
		{Prompt: "What should the agent never say or imply on your behalf?", Category: "guardrails", Required: len(session.Constraints) == 0},
	}

	if session.AccountMode == "new" {
		items = append(items,
			promptSpec{Prompt: "Which existing brands, creators, or founders feel closest to the style you want?", Category: "style_reference", Required: true},
			promptSpec{Prompt: "Do you want the voice to feel authoritative, warm, playful, contrarian, or technical?", Category: "voice", Required: session.VoiceSummary == ""},
		)
	} else {
		items = append(items,
			promptSpec{Prompt: "Which connected account should the agent study first for historical learning?", Category: "account_scope", Required: true},
			promptSpec{Prompt: "Should the agent preserve your current style closely, or deliberately improve and sharpen it?", Category: "improvement_mode", Required: true},
		)
	}
	return items
}

func seedChatMessages(jobDescription, brandName string) []ChatMessage {
	now := time.Now().UTC()
	subject := firstNonEmpty(strings.TrimSpace(brandName), "your brand")
	return []ChatMessage{
		{
			ID:        "chat-" + uuid.NewString(),
			Role:      "assistant",
			Content:   fmt.Sprintf("I’ve read the hiring brief for %s. I’m going to learn the brand, tighten the strategy, and only ask for details that materially improve execution.", subject),
			CreatedAt: now,
		},
	}
}

func buildOnboardingSystemPrompt(session Session) string {
	var pending []string
	for _, question := range session.Questions {
		if strings.TrimSpace(question.Answer) != "" {
			continue
		}
		pending = append(pending, fmt.Sprintf("- %s | %s", question.ID, question.Prompt))
	}

	if len(pending) == 0 {
		pending = append(pending, "- none; answer the user clearly and, if useful, explain what the agent will do next.")
	}

	return strings.TrimSpace(fmt.Sprintf(`
You are Fedey, an AI social media manager being hired like a real employee.

Your job in this chat:
1. reply naturally, clearly, and helpfully like a strong GPT-style agent
2. extract answers from the user's latest message only when the message actually answers one of the pending onboarding questions
3. if the user asks a direct question, answer it well before steering back to missing onboarding details
4. ask at most one next best follow-up question in your assistant_message
5. never invent facts that the hirer did not provide

Current onboarding context:
- title: %s
- brand: %s
- account mode: %s
- primary platform: %s
- objective: %s
- audience: %s
- review mode: %s
- voice summary: %s
- constraints: %s
- audit status: %s
- connected platforms: %s
- recent audit recommendations: %s

Pending questions:
%s

Return strict JSON with:
- assistant_message: string
- resolved_answers: array of objects with question_id and answer

Only include a resolved answer if the latest user message clearly answered that question.
`, session.Title, firstNonEmpty(session.BrandName, session.Title), session.AccountMode, session.PrimaryPlatform, firstNonEmpty(session.Objective, "unspecified"), firstNonEmpty(session.Audience, "unspecified"), session.ReviewMode, firstNonEmpty(session.VoiceSummary, "unspecified"), strings.Join(session.Constraints, ", "), session.Audit.Status, strings.Join(session.Audit.ConnectedPlatforms, ", "), strings.Join(session.Audit.Recommendations, "; "), strings.Join(pending, "\n")))
}

func buildChatTranscript(session Session) []openai.Message {
	items := make([]openai.Message, 0, len(session.ChatMessages)+1)
	for _, message := range session.ChatMessages {
		role := "assistant"
		if message.Role == "user" {
			role = "user"
		}
		items = append(items, openai.Message{
			Role:    role,
			Content: message.Content,
		})
	}
	return items
}

func allRequiredAnswered(questions []Question) bool {
	for _, question := range questions {
		if question.Required && strings.TrimSpace(question.Answer) == "" {
			return false
		}
	}
	return true
}

func normalizeAccountMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "existing":
		return "existing"
	default:
		return "new"
	}
}

func normalizeReviewMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "manual":
		return "manual"
	default:
		return "auto"
	}
}

func initialApprovalStatus(reviewMode string) string {
	if normalizeReviewMode(reviewMode) == "manual" {
		return "not_started"
	}
	return "not_required"
}

func inferTitle(description, brandName string) string {
	if strings.TrimSpace(brandName) != "" {
		return brandName + " Social Agent"
	}
	return truncate(description, 48)
}

func inferObjective(description string) string {
	lower := strings.ToLower(description)
	switch {
	case strings.Contains(lower, "lead"), strings.Contains(lower, "sale"):
		return "generate leads"
	case strings.Contains(lower, "trust"), strings.Contains(lower, "authority"):
		return "build authority"
	default:
		return "grow attention"
	}
}

func inferPlatform(description string) string {
	lower := strings.ToLower(description)
	switch {
	case strings.Contains(lower, "linkedin"):
		return "linkedin"
	default:
		return "x"
	}
}

func inferVoice(description string) string {
	lower := strings.ToLower(description)
	parts := make([]string, 0, 3)
	if strings.Contains(lower, "professional") || strings.Contains(lower, "founder") {
		parts = append(parts, "professional")
	}
	if strings.Contains(lower, "funny") || strings.Contains(lower, "humor") {
		parts = append(parts, "playful")
	}
	if strings.Contains(lower, "technical") || strings.Contains(lower, "developer") {
		parts = append(parts, "technical")
	}
	if len(parts) == 0 {
		return "clear, strategic, and human"
	}
	return strings.Join(parts, ", ")
}

func deriveXContentPatterns(posts []xplatform.UserPost) []string {
	if len(posts) == 0 {
		return nil
	}
	averageLength := 0
	replyCount := 0
	threadStarters := 0
	for _, post := range posts {
		averageLength += len(post.Text)
		if post.ReplyToID != "" {
			replyCount++
		}
		if strings.Contains(post.Text, "\n") {
			threadStarters++
		}
	}
	averageLength /= len(posts)
	patterns := []string{
		fmt.Sprintf("On X, average post length is %d characters.", averageLength),
		fmt.Sprintf("On X, %d of the last %d posts were replies, which shows how conversational the account already is.", replyCount, len(posts)),
	}
	if threadStarters > 0 {
		patterns = append(patterns, fmt.Sprintf("On X, %d recent posts used multi-line structure, which suggests the audience tolerates thread-style formatting.", threadStarters))
	}
	return patterns
}

func deriveXReplyPatterns(posts []xplatform.UserPost) []string {
	return deriveOpeningPatterns(
		posts,
		func(post xplatform.UserPost) bool { return post.ReplyToID != "" },
		func(post xplatform.UserPost) string { return post.Text },
		"X replies",
	)
}

func deriveXRecommendations(posts []xplatform.UserPost, session Session) []string {
	recommendations := []string{
		fmt.Sprintf("Keep the X structure familiar in early drafts so the audience recognizes continuity for %s.", session.BrandName),
	}
	if len(posts) >= 10 {
		recommendations = append(recommendations, "The X account has enough recent material to preserve structure first and optimize later.")
	}
	return recommendations
}

func deriveXPerformanceInsights(posts []xplatform.UserPost) []string {
	if len(posts) == 0 {
		return nil
	}
	totalLikes := 0
	totalReplies := 0
	totalQuotes := 0
	for _, post := range posts {
		totalLikes += post.LikeCount
		totalReplies += post.ReplyCount
		totalQuotes += post.QuoteCount
	}
	return []string{
		fmt.Sprintf("Historical X performance averages %d likes and %d replies per post across the recent sample.", totalLikes/max(1, len(posts)), totalReplies/max(1, len(posts))),
		fmt.Sprintf("Quote activity averages %d per post, which helps estimate how much opinion-led content resonates on X.", totalQuotes/max(1, len(posts))),
	}
}

func deriveLinkedInContentPatterns(posts []linkedinplatform.Post) []string {
	if len(posts) == 0 {
		return nil
	}
	averageLength := 0
	for _, post := range posts {
		averageLength += len(post.Commentary)
	}
	averageLength /= len(posts)
	return []string{
		fmt.Sprintf("On LinkedIn, average post length is %d characters.", averageLength),
		"LinkedIn history should keep a professional, insight-led structure unless onboarding answers suggest a sharper shift.",
	}
}

func deriveLinkedInPerformanceInsights(ctx context.Context, account linkedinaccounts.Account, posts []linkedinplatform.Post, client *linkedinplatform.Client) []string {
	if len(posts) == 0 || client == nil {
		return nil
	}

	totalComments := 0
	totalAuthoredReplies := 0
	for _, post := range posts {
		comments, err := client.ListComments(ctx, account.AccessToken, normalizeLinkedInPostURN(post.ID), 20)
		if err != nil {
			continue
		}
		totalComments += len(comments)
		for _, comment := range comments {
			if strings.EqualFold(comment.ActorURN, account.AuthorURN) {
				totalAuthoredReplies++
			}
		}
	}
	return []string{
		fmt.Sprintf("Recent LinkedIn posts average %d visible comments in the sampled history.", totalComments/max(1, len(posts))),
		fmt.Sprintf("Authored LinkedIn replies average %d per sampled post, which shows how actively the brand joins its own conversations.", totalAuthoredReplies/max(1, len(posts))),
	}
}

func (s *Service) deriveLinkedInConversationPatterns(ctx context.Context, account linkedinaccounts.Account, posts []linkedinplatform.Post) ([]string, []string) {
	var replyPatterns []string
	var recommendations []string
	for _, post := range posts {
		comments, err := s.linkedinClient.ListComments(ctx, account.AccessToken, normalizeLinkedInPostURN(post.ID), 10)
		if err != nil {
			continue
		}

		authoredReplies := 0
		externalComments := 0
		openings := []linkedinplatform.Comment{}
		for _, comment := range comments {
			if strings.EqualFold(comment.ActorURN, account.AuthorURN) {
				authoredReplies++
				openings = append(openings, comment)
				continue
			}
			externalComments++
		}
		if authoredReplies > 0 || externalComments > 0 {
			replyPatterns = append(replyPatterns, fmt.Sprintf("On LinkedIn, the post %s has %d audience comments and %d authored replies.", normalizeLinkedInPostURN(post.ID), externalComments, authoredReplies))
		}
		replyPatterns = append(replyPatterns, deriveCommentOpeningPatterns(openings, "LinkedIn replies")...)
	}

	if len(posts) > 0 {
		recommendations = append(recommendations, "Keep LinkedIn posts more polished and insight-heavy than X while preserving the same core positioning.")
	}
	return uniqueStrings(replyPatterns), uniqueStrings(recommendations)
}

func deriveOpeningPatterns[T any](items []T, predicate func(T) bool, text func(T) string, prefix string) []string {
	openings := make(map[string]int)
	for _, item := range items {
		if !predicate(item) {
			continue
		}
		words := strings.Fields(text(item))
		if len(words) == 0 {
			continue
		}
		openings[strings.ToLower(words[0])]++
	}
	return topOpeningPatterns(openings, prefix)
}

func deriveCommentOpeningPatterns(items []linkedinplatform.Comment, prefix string) []string {
	openings := make(map[string]int)
	for _, item := range items {
		words := strings.Fields(item.Message)
		if len(words) == 0 {
			continue
		}
		openings[strings.ToLower(words[0])]++
	}
	return topOpeningPatterns(openings, prefix)
}

func topOpeningPatterns(openings map[string]int, prefix string) []string {
	type openingCount struct {
		word  string
		count int
	}
	var counts []openingCount
	for word, count := range openings {
		counts = append(counts, openingCount{word: word, count: count})
	}
	slices.SortFunc(counts, func(left, right openingCount) int {
		return right.count - left.count
	})

	var patterns []string
	for index, item := range counts {
		if index >= 2 {
			break
		}
		patterns = append(patterns, fmt.Sprintf("%s often open with '%s'.", prefix, item.word))
	}
	if len(patterns) == 0 {
		patterns = append(patterns, fmt.Sprintf("%s have limited recent public history, so the agent should combine audit learning with onboarding answers.", prefix))
	}
	return patterns
}

func (s *Service) syncBrandMemory(ctx context.Context, session Session) (Session, error) {
	if s.brandMemoryService == nil {
		return session, nil
	}

	tone, audience, pillars, guardrails := deriveBrandMemoryFields(session)
	brandName := firstNonEmpty(session.BrandName, session.Title)
	if brandName == "" || tone == "" || audience == "" {
		return session, nil
	}

	profile, err := s.brandMemoryService.Upsert(ctx, brandmemory.UpsertInput{
		BrandName:  brandName,
		Tone:       tone,
		Audience:   audience,
		Pillars:    pillars,
		Guardrails: guardrails,
	})
	if err != nil {
		return session, err
	}

	session.BrandName = profile.BrandName
	session.Audience = profile.Audience
	session.VoiceSummary = profile.Tone
	session.Constraints = profile.Guardrails
	session.UpdatedAt = time.Now().UTC()
	if err := s.repository.UpdateSession(ctx, session); err != nil {
		return session, err
	}
	return session, nil
}

func deriveBrandMemoryFields(session Session) (string, string, []string, []string) {
	tone := firstNonEmpty(session.VoiceSummary, inferVoice(session.JobDescription))
	audience := firstNonEmpty(session.Audience, answerForCategory(session.Questions, "audience"))
	if audience == "" {
		audience = "Founders and operators who want strategic social growth"
	}
	guardrails := normalizeStrings(append([]string{}, session.Constraints...))
	if answer := answerForCategory(session.Questions, "guardrails"); answer != "" {
		guardrails = append(guardrails, splitByComma(answer)...)
	}
	guardrails = uniqueStrings(guardrails)
	pillars := inferPillars(session)
	return tone, audience, pillars, guardrails
}

func inferPillars(session Session) []string {
	var pillars []string
	if session.Objective != "" {
		pillars = append(pillars, session.Objective)
	}
	pillars = append(pillars, "x growth", "linkedin growth")
	if len(session.Audit.ContentPatterns) > 0 {
		pillars = append(pillars, truncate(session.Audit.ContentPatterns[0], 80))
	}
	if answer := answerForCategory(session.Questions, "style_reference"); answer != "" {
		pillars = append(pillars, truncate(answer, 80))
	}
	if len(pillars) == 0 {
		pillars = append(pillars, "authority building", "audience education")
	}
	return uniqueStrings(pillars)
}

func buildWeekPlan(session Session) []ActivationItem {
	focuses := []string{
		"brand-introduction insight",
		"authority-building lesson",
		"comment-driven conversation starter",
		"case-study style proof point",
		"weekly recap and stance",
	}
	formats := map[string]string{
		"x":        "thread or sharp multi-post sequence",
		"linkedin": "professional insight post",
	}
	hypothesis := buildActivationHypothesis(session)
	plan := make([]ActivationItem, 0, 5)
	for index := 0; index < 5; index++ {
		channel := "x"
		if index%2 == 1 {
			channel = "linkedin"
		}
		plan = append(plan, ActivationItem{
			Day:        fmt.Sprintf("Day %d", index+1),
			Channel:    channel,
			Focus:      focuses[index],
			Format:     formats[channel],
			Hypothesis: hypothesis,
		})
	}
	return plan
}

func buildActivationHypothesis(session Session) string {
	if len(session.Audit.Recommendations) > 0 {
		return session.Audit.Recommendations[0]
	}
	if session.Objective != "" {
		return "Consistent X and LinkedIn execution should improve " + session.Objective + " within the first week."
	}
	return "A coordinated X and LinkedIn presence should establish a recognizable voice within the first week."
}

func buildActivationSummary(session Session) string {
	return fmt.Sprintf(
		"Activation plan generated for %s. The agent will start with X and LinkedIn, preserve learned structural patterns, and test improvements against the goal to %s.",
		firstNonEmpty(session.BrandName, session.Title),
		firstNonEmpty(session.Objective, "grow attention"),
	)
}

func (s *Service) syncActivationDrafts(ctx context.Context, session Session, plan []ActivationItem) ([]ActivationDraft, error) {
	if s.contentService == nil {
		return nil, nil
	}

	newDrafts := make([]content.Draft, 0, len(plan))
	updatedDrafts := make([]content.Draft, 0, len(plan))
	summaries := make([]ActivationDraft, 0, len(plan))
	now := time.Now().UTC()
	for index, item := range plan {
		hook, body := buildActivationCopy(session, item)
		draft := content.Draft{
			ID:          draftIDForIndex(session.Activation.Drafts, index),
			Channel:     item.Channel,
			Hook:        hook,
			Body:        body,
			Rationale:   fmt.Sprintf("Generated from onboarding activation plan for %s.", item.Day),
			SourceTrend: "activation_plan",
			Status:      "draft",
			CreatedAt:   now.Add(time.Duration(index) * time.Minute),
		}
		if draft.ID == "" {
			draft.ID = "draft-" + uuid.NewString()
			newDrafts = append(newDrafts, draft)
		} else {
			updatedDrafts = append(updatedDrafts, draft)
		}
		summaries = append(summaries, ActivationDraft{
			DraftID:        draft.ID,
			Channel:        draft.Channel,
			Hook:           draft.Hook,
			Body:           draft.Body,
			Rationale:      draft.Rationale,
			ScheduleID:     scheduleIDForIndex(session.Activation.Drafts, index),
			ScheduleStatus: scheduleStatusForIndex(session.Activation.Drafts, index),
			ScheduledFor:   scheduledForForIndex(session.Activation.Drafts, index),
		})
	}

	if err := s.contentService.SaveBatch(ctx, newDrafts); err != nil {
		return nil, err
	}
	for _, draft := range updatedDrafts {
		if err := s.contentService.UpdateDraft(ctx, draft); err != nil {
			return nil, err
		}
	}
	return summaries, nil
}

func (s *Service) scheduleActivationDrafts(ctx context.Context, session Session, drafts []ActivationDraft) ([]ActivationDraft, error) {
	if s.publishingService == nil || len(drafts) == 0 {
		return drafts, nil
	}

	scheduledTimes := activationScheduleTimes(time.Now().UTC(), len(drafts), s.activationWindows)
	items := append([]ActivationDraft(nil), drafts...)
	for index := range items {
		if strings.TrimSpace(items[index].ScheduleID) != "" {
			continue
		}
		recommendedAt := scheduledTimes[index]
		if s.publishingService != nil {
			recommendedAt = s.publishingService.RecommendNextTime(ctx, items[index].Channel, recommendedAt.Add(-1*time.Minute), session.AccountMode)
		}

		schedule, err := s.publishingService.Create(ctx, publishing.CreateInput{
			DraftID:      items[index].DraftID,
			VariantLabel: "",
			Channel:      items[index].Channel,
			QueueProfile: session.AccountMode,
			ScheduledFor: recommendedAt,
		})
		if err != nil {
			return nil, err
		}
		items[index].ScheduleID = schedule.ID
		items[index].ScheduleStatus = string(schedule.Status)
		items[index].ScheduledFor = schedule.ScheduledFor
	}
	return items, nil
}

func buildActivationCopy(session Session, item ActivationItem) (string, string) {
	switch item.Channel {
	case "linkedin":
		return fmt.Sprintf("%s: %s", firstNonEmpty(session.BrandName, session.Title), strings.Title(item.Focus)),
			fmt.Sprintf(
				"This week we are building around %s. On LinkedIn, this piece focuses on %s for %s. The operating idea is simple: %s",
				firstNonEmpty(session.Objective, "audience growth"),
				item.Focus,
				firstNonEmpty(session.Audience, "the target audience"),
				item.Hypothesis,
			)
	default:
		return fmt.Sprintf("%s | %s", firstNonEmpty(session.BrandName, session.Title), strings.Title(item.Focus)),
			fmt.Sprintf(
				"Today on X we are leaning into %s.\n\nThe goal is to move %s while preserving the learned voice: %s.\n\nWorking hypothesis: %s",
				item.Focus,
				firstNonEmpty(session.Objective, "attention"),
				firstNonEmpty(session.VoiceSummary, "clear and strategic"),
				item.Hypothesis,
			)
	}
}

func normalizeActivationPlan(items []ActivationItem) []ActivationItem {
	plan := make([]ActivationItem, 0, len(items))
	for index, item := range items {
		channel := strings.ToLower(strings.TrimSpace(item.Channel))
		if channel != "x" && channel != "linkedin" {
			continue
		}
		day := strings.TrimSpace(item.Day)
		if day == "" {
			day = fmt.Sprintf("Day %d", index+1)
		}
		focus := strings.TrimSpace(item.Focus)
		format := strings.TrimSpace(item.Format)
		hypothesis := strings.TrimSpace(item.Hypothesis)
		if focus == "" || format == "" || hypothesis == "" {
			continue
		}
		plan = append(plan, ActivationItem{
			Day:        day,
			Channel:    channel,
			Focus:      focus,
			Format:     format,
			Hypothesis: hypothesis,
		})
	}
	return plan
}

func hasScheduledActivationDraft(drafts []ActivationDraft) bool {
	for _, draft := range drafts {
		if strings.TrimSpace(draft.ScheduleID) != "" {
			return true
		}
	}
	return false
}

func draftIDForIndex(drafts []ActivationDraft, index int) string {
	if index < len(drafts) {
		return drafts[index].DraftID
	}
	return ""
}

func scheduleIDForIndex(drafts []ActivationDraft, index int) string {
	if index < len(drafts) {
		return drafts[index].ScheduleID
	}
	return ""
}

func scheduleStatusForIndex(drafts []ActivationDraft, index int) string {
	if index < len(drafts) {
		return drafts[index].ScheduleStatus
	}
	return ""
}

func scheduledForForIndex(drafts []ActivationDraft, index int) time.Time {
	if index < len(drafts) {
		return drafts[index].ScheduledFor
	}
	return time.Time{}
}

func activationScheduleTimes(now time.Time, count int, windows []publishing.Window) []time.Time {
	defaultWindows := []publishing.Window{
		{Hour: 9, Minute: 0, Label: "09:00"},
		{Hour: 13, Minute: 0, Label: "13:00"},
	}
	if len(windows) == 0 {
		windows = defaultWindows
	}

	items := make([]time.Time, 0, count)
	base := nextBusinessDay(now)
	for index := 0; index < count; index++ {
		window := windows[index%len(windows)]
		day := base.AddDate(0, 0, index)
		day = nextBusinessDay(day)
		candidate := time.Date(day.Year(), day.Month(), day.Day(), window.Hour, window.Minute, 0, 0, time.UTC)
		if !candidate.After(now.UTC()) {
			nextDay := nextBusinessDay(day.AddDate(0, 0, 1))
			candidate = time.Date(nextDay.Year(), nextDay.Month(), nextDay.Day(), window.Hour, window.Minute, 0, 0, time.UTC)
		}
		items = append(items, candidate)
	}
	return items
}

func nextBusinessDay(from time.Time) time.Time {
	day := from.UTC()
	if !day.IsZero() && (day.Hour() > 20 || day.Hour() < 6) {
		day = day.AddDate(0, 0, 1)
	}
	for day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
		day = day.AddDate(0, 0, 1)
	}
	return day
}

func mergeActivationDrafts(current []ActivationDraft, updates []ActivationDraft) []ActivationDraft {
	updatedByID := make(map[string]ActivationDraft, len(updates))
	for _, item := range updates {
		updatedByID[item.DraftID] = item
	}

	items := make([]ActivationDraft, 0, len(current))
	for _, item := range current {
		if updated, ok := updatedByID[item.DraftID]; ok {
			items = append(items, updated)
			continue
		}
		items = append(items, item)
	}
	return items
}

func newHistoryEntry(actor, action, description string) HistoryEntry {
	return HistoryEntry{
		ID:          "hist-" + uuid.NewString(),
		Actor:       strings.TrimSpace(actor),
		Action:      strings.TrimSpace(action),
		Description: strings.TrimSpace(description),
		CreatedAt:   time.Now().UTC(),
	}
}

func (s *Service) resolveXCredentials(ctx context.Context) (xaccounts.Account, error) {
	if s.xAccountService == nil {
		return xaccounts.Account{}, ErrInvalidSessionInput
	}
	return s.xAccountService.GetActive(ctx)
}

func (s *Service) resolveLinkedInCredentials(ctx context.Context) (linkedinaccounts.Account, error) {
	if s.linkedinService == nil {
		return linkedinaccounts.Account{}, ErrInvalidSessionInput
	}
	return s.linkedinService.GetActive(ctx)
}

func answerForCategory(questions []Question, category string) string {
	for _, question := range questions {
		if question.Category == category && strings.TrimSpace(question.Answer) != "" {
			return question.Answer
		}
	}
	return ""
}

func normalizeStrings(values []string) []string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	items := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		items = append(items, trimmed)
	}
	return items
}

func splitByComma(value string) []string {
	parts := strings.Split(value, ",")
	return normalizeStrings(parts)
}

func normalizeLinkedInPostURN(value string) string {
	trimmed := strings.TrimSpace(value)
	if strings.HasPrefix(trimmed, "urn:") {
		return trimmed
	}
	return "urn:li:ugcPost:" + trimmed
}

func firstNonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func truncate(value string, limit int) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) <= limit {
		return trimmed
	}
	return strings.TrimSpace(trimmed[:limit]) + "..."
}

func max(left, right int) int {
	if left > right {
		return left
	}
	return right
}
