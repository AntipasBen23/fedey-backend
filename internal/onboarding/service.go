package onboarding

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	xplatform "github.com/AntipasBen23/fedey-backend/internal/platform/x"
	"github.com/AntipasBen23/fedey-backend/internal/xaccounts"
)

type Service struct {
	repository      Repository
	xClient         *xplatform.Client
	xAccountService *xaccounts.Service
}

func NewService(repository Repository, xClient *xplatform.Client, xAccountService *xaccounts.Service) *Service {
	return &Service{
		repository:      repository,
		xClient:         xClient,
		xAccountService: xAccountService,
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
		Audit: AuditReport{
			Status: "not_started",
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
	return session, nil
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
	if allRequiredAnswered(session.Questions) {
		session.Status = StatusReady
		session.UpdatedAt = time.Now().UTC()
		if err := s.repository.UpdateSession(ctx, session); err != nil {
			return Session{}, err
		}
	}
	return session, nil
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

	account, err := s.resolveXCredentials(ctx)
	if err == nil && s.xClient != nil {
		posts, err := s.xClient.FetchUserPostsWithToken(ctx, account.AccessToken, account.UserID, 15)
		if err == nil {
			report.ConnectedPlatforms = append(report.ConnectedPlatforms, "x")
			report.PostsReviewed = len(posts)
			report.ContentPatterns = deriveContentPatterns(posts)
			report.ReplyPatterns = deriveReplyPatterns(posts)
			report.Recommendations = deriveRecommendations(posts, session)
			report.Status = "completed"
		}
	}

	if len(report.ConnectedPlatforms) == 0 {
		report.Status = "waiting_for_connections"
		report.Recommendations = []string{
			"Connect an existing X account so the agent can learn from historical posts and replies.",
			"Add brand voice preferences or answer follow-up questions to sharpen the onboarding model.",
		}
	}

	session.Audit = report
	session.UpdatedAt = time.Now().UTC()
	if report.Status == "completed" && allRequiredAnswered(session.Questions) {
		session.Status = StatusReady
	}
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
	case strings.Contains(lower, "instagram"):
		return "instagram"
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

func deriveContentPatterns(posts []xplatform.UserPost) []string {
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
		fmt.Sprintf("Average post length is %d characters.", averageLength),
		fmt.Sprintf("%d of the last %d posts were replies, which shows how conversational the account already is.", replyCount, len(posts)),
	}
	if threadStarters > 0 {
		patterns = append(patterns, fmt.Sprintf("%d recent posts used multi-line structure, which suggests the audience tolerates thread-style formatting.", threadStarters))
	}
	return patterns
}

func deriveReplyPatterns(posts []xplatform.UserPost) []string {
	patterns := []string{}
	openings := make(map[string]int)
	for _, post := range posts {
		if post.ReplyToID == "" {
			continue
		}
		words := strings.Fields(post.Text)
		if len(words) == 0 {
			continue
		}
		openings[strings.ToLower(words[0])]++
	}
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
	for index, item := range counts {
		if index >= 2 {
			break
		}
		patterns = append(patterns, fmt.Sprintf("Replies often open with '%s', which is part of the existing response rhythm.", item.word))
	}
	if len(patterns) == 0 {
		patterns = append(patterns, "The account has limited recent public reply history, so the agent should rely on interview answers for reply tone.")
	}
	return patterns
}

func deriveRecommendations(posts []xplatform.UserPost, session Session) []string {
	recommendations := []string{
		fmt.Sprintf("Keep the %s-first structure in early drafts so the audience recognizes continuity.", session.PrimaryPlatform),
		"Preserve familiar hooks from the existing account for week one, then begin controlled improvements through experiments.",
	}
	if len(posts) >= 10 {
		recommendations = append(recommendations, "The account has enough recent material to clone structure before optimizing tone and timing.")
	}
	return recommendations
}

func (s *Service) resolveXCredentials(ctx context.Context) (xaccounts.Account, error) {
	if s.xAccountService == nil {
		return xaccounts.Account{}, ErrInvalidSessionInput
	}
	return s.xAccountService.GetActive(ctx)
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
