package server

import (
	"net/http"

	"github.com/AntipasBen23/fedey-backend/internal/automation"
	"github.com/AntipasBen23/fedey-backend/internal/brandmemory"
	"github.com/AntipasBen23/fedey-backend/internal/community"
	"github.com/AntipasBen23/fedey-backend/internal/content"
	"github.com/AntipasBen23/fedey-backend/internal/experiments"
	"github.com/AntipasBen23/fedey-backend/internal/linkedinaccounts"
	"github.com/AntipasBen23/fedey-backend/internal/publishing"
	"github.com/AntipasBen23/fedey-backend/internal/server/handlers"
	"github.com/AntipasBen23/fedey-backend/internal/trends"
	"github.com/AntipasBen23/fedey-backend/internal/xaccounts"
)

type Dependencies struct {
	ExperimentService  *experiments.Service
	BrandMemoryService *brandmemory.Service
	TrendService       *trends.Service
	ContentService     *content.Service
	PublishingService  *publishing.Service
	CommunityService   *community.Service
	AutomationService  *automation.Service
	XAccountService    *xaccounts.Service
	LinkedInService    *linkedinaccounts.Service
}

func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	registerRoutes(mux, deps)
	return mux
}

func registerRoutes(mux *http.ServeMux, deps Dependencies) {
	experimentsHandler := handlers.NewExperimentsHandler(deps.ExperimentService)
	analyticsHandler := handlers.NewAnalyticsHandler(deps.ExperimentService)
	brandMemoryHandler := handlers.NewBrandMemoryHandler(deps.BrandMemoryService)
	trendsHandler := handlers.NewTrendsHandler(deps.TrendService)
	strategyHandler := handlers.NewStrategyHandler(deps.BrandMemoryService, deps.TrendService)
	contentHandler := handlers.NewContentHandler(deps.ContentService)
	publishingHandler := handlers.NewPublishingHandler(deps.PublishingService)
	communityHandler := handlers.NewCommunityHandler(deps.CommunityService)
	automationHandler := handlers.NewAutomationHandler(deps.AutomationService)
	xAccountsHandler := handlers.NewXAccountsHandler(deps.XAccountService)
	linkedinAccountsHandler := handlers.NewLinkedInAccountsHandler(deps.LinkedInService)

	mux.HandleFunc("GET /healthz", handlers.Healthz)
	mux.HandleFunc("GET /v1/health", handlers.HealthV1)
	mux.HandleFunc("GET /v1/strategy/snapshot", strategyHandler.GetSnapshot)
	mux.HandleFunc("GET /v1/brand-memory", brandMemoryHandler.Get)
	mux.HandleFunc("PUT /v1/brand-memory", brandMemoryHandler.Upsert)
	mux.HandleFunc("GET /v1/trends", trendsHandler.List)
	mux.HandleFunc("POST /v1/trends", trendsHandler.Create)
	mux.HandleFunc("POST /v1/trends/ingest", trendsHandler.Ingest)
	mux.HandleFunc("GET /v1/content/drafts", contentHandler.ListDrafts)
	mux.HandleFunc("POST /v1/content/drafts/generate", contentHandler.GenerateDrafts)
	mux.HandleFunc("POST /v1/content/drafts/{id}/variants/generate", contentHandler.GenerateVariants)
	mux.HandleFunc("GET /v1/publishing/schedules", publishingHandler.ListSchedules)
	mux.HandleFunc("POST /v1/publishing/schedules", publishingHandler.CreateSchedule)
	mux.HandleFunc("PATCH /v1/publishing/schedules/{id}/publish", publishingHandler.MarkPublished)
	mux.HandleFunc("GET /v1/community/inbox", communityHandler.ListInbox)
	mux.HandleFunc("POST /v1/community/inbox", communityHandler.CreateInboxItem)
	mux.HandleFunc("POST /v1/community/inbox/sync/x", communityHandler.SyncXMentions)
	mux.HandleFunc("POST /v1/community/inbox/sync/linkedin", communityHandler.SyncLinkedInComments)
	mux.HandleFunc("POST /v1/community/inbox/{id}/draft-reply", communityHandler.DraftReply)
	mux.HandleFunc("PATCH /v1/community/inbox/{id}/reply", communityHandler.MarkReplied)
	mux.HandleFunc("GET /v1/automation/runs", automationHandler.ListRuns)
	mux.HandleFunc("GET /v1/automation/settings", automationHandler.GetSettings)
	mux.HandleFunc("POST /v1/automation/run", automationHandler.RunOnce)
	mux.HandleFunc("GET /v1/integrations/x/status", xAccountsHandler.Status)
	mux.HandleFunc("GET /v1/integrations/x/connect", xAccountsHandler.StartConnect)
	mux.HandleFunc("GET /v1/integrations/x/callback", xAccountsHandler.Callback)
	mux.HandleFunc("GET /v1/integrations/linkedin/status", linkedinAccountsHandler.Status)
	mux.HandleFunc("GET /v1/integrations/linkedin/connect", linkedinAccountsHandler.StartConnect)
	mux.HandleFunc("GET /v1/integrations/linkedin/callback", linkedinAccountsHandler.Callback)

	mux.HandleFunc("POST /v1/experiments", experimentsHandler.Create)
	mux.HandleFunc("GET /v1/experiments", experimentsHandler.List)
	mux.HandleFunc("PATCH /v1/experiments/{id}/status", experimentsHandler.PatchStatus)
	mux.HandleFunc("POST /v1/analytics/events", analyticsHandler.RecordEvent)
}
