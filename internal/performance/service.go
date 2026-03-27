package performance

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/AntipasBen23/fedey-backend/internal/linkedinaccounts"
	linkedinplatform "github.com/AntipasBen23/fedey-backend/internal/platform/linkedin"
	xplatform "github.com/AntipasBen23/fedey-backend/internal/platform/x"
	"github.com/AntipasBen23/fedey-backend/internal/xaccounts"
)

type Service struct {
	repository      Repository
	xClient         *xplatform.Client
	xAccountService *xaccounts.Service
	linkedinClient  *linkedinplatform.Client
	linkedinService *linkedinaccounts.Service
}

func NewService(
	repository Repository,
	xClient *xplatform.Client,
	xAccountService *xaccounts.Service,
	linkedinClient *linkedinplatform.Client,
	linkedinService *linkedinaccounts.Service,
) *Service {
	return &Service{
		repository:      repository,
		xClient:         xClient,
		xAccountService: xAccountService,
		linkedinClient:  linkedinClient,
		linkedinService: linkedinService,
	}
}

func (s *Service) SyncConnectedAccounts(ctx context.Context) (SyncResult, error) {
	result := SyncResult{}

	if count, err := s.syncX(ctx); err == nil {
		result.XSnapshots = count
	}
	if count, err := s.syncLinkedIn(ctx); err == nil {
		result.LinkedInSnapshots = count
	}

	return result, nil
}

func (s *Service) Insights(ctx context.Context, platform string) ([]string, error) {
	snapshots, err := s.repository.ListRecent(ctx, strings.ToLower(strings.TrimSpace(platform)), 24)
	if err != nil {
		return nil, err
	}
	if len(snapshots) == 0 {
		return nil, nil
	}

	latest := latestPerPost(snapshots)
	if len(latest) == 0 {
		return nil, nil
	}

	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "linkedin":
		return buildLinkedInInsights(latest), nil
	default:
		return buildXInsights(latest), nil
	}
}

func (s *Service) syncX(ctx context.Context) (int, error) {
	if s.xClient == nil || s.xAccountService == nil {
		return 0, fmt.Errorf("x performance sync unavailable")
	}

	account, err := s.xAccountService.GetActive(ctx)
	if err != nil {
		return 0, err
	}

	posts, err := s.xClient.FetchUserPostsWithToken(ctx, account.AccessToken, account.UserID, 20)
	if err != nil {
		return 0, err
	}

	now := time.Now().UTC()
	snapshots := make([]Snapshot, 0, len(posts))
	for _, post := range posts {
		snapshots = append(snapshots, Snapshot{
			ID:             "perf-" + uuid.NewString(),
			Platform:       "x",
			ExternalPostID: post.ID,
			AuthorRef:      account.Username,
			ContentPreview: preview(post.Text),
			LikeCount:      post.LikeCount,
			ReplyCount:     post.ReplyCount,
			QuoteCount:     post.QuoteCount,
			CapturedAt:     chooseCapturedAt(post.CreatedAt, now),
		})
	}

	if err := s.repository.SaveBatch(ctx, snapshots); err != nil {
		return 0, err
	}
	return len(snapshots), nil
}

func (s *Service) syncLinkedIn(ctx context.Context) (int, error) {
	if s.linkedinClient == nil || s.linkedinService == nil {
		return 0, fmt.Errorf("linkedin performance sync unavailable")
	}

	account, err := s.linkedinService.GetActive(ctx)
	if err != nil {
		return 0, err
	}

	posts, err := s.linkedinClient.ListAuthorPosts(ctx, account.AccessToken, account.AuthorURN, 12)
	if err != nil {
		return 0, err
	}

	now := time.Now().UTC()
	snapshots := make([]Snapshot, 0, len(posts))
	for _, post := range posts {
		comments, err := s.linkedinClient.ListComments(ctx, account.AccessToken, normalizeLinkedInPostURN(post.ID), 25)
		if err != nil {
			continue
		}

		authoredReplies := 0
		for _, comment := range comments {
			if strings.EqualFold(comment.ActorURN, account.AuthorURN) {
				authoredReplies++
			}
		}

		snapshots = append(snapshots, Snapshot{
			ID:             "perf-" + uuid.NewString(),
			Platform:       "linkedin",
			ExternalPostID: normalizeLinkedInPostURN(post.ID),
			AuthorRef:      account.DisplayName,
			ContentPreview: preview(post.Commentary),
			ReplyCount:     authoredReplies,
			CommentCount:   len(comments),
			CapturedAt:     chooseCapturedAt(post.CreatedAt, now),
		})
	}

	if err := s.repository.SaveBatch(ctx, snapshots); err != nil {
		return 0, err
	}
	return len(snapshots), nil
}

func latestPerPost(snapshots []Snapshot) []Snapshot {
	items := append([]Snapshot(nil), snapshots...)
	slices.SortFunc(items, func(left, right Snapshot) int {
		return right.CapturedAt.Compare(left.CapturedAt)
	})

	latest := make(map[string]Snapshot, len(items))
	for _, item := range items {
		key := strings.ToLower(item.Platform) + "|" + item.ExternalPostID
		if _, exists := latest[key]; exists {
			continue
		}
		latest[key] = item
	}

	result := make([]Snapshot, 0, len(latest))
	for _, item := range latest {
		result = append(result, item)
	}
	slices.SortFunc(result, func(left, right Snapshot) int {
		return right.CapturedAt.Compare(left.CapturedAt)
	})
	return result
}

func buildXInsights(snapshots []Snapshot) []string {
	totalLikes := 0
	totalReplies := 0
	totalQuotes := 0
	best := snapshots[0]
	bestScore := engagementScore(best)
	for _, item := range snapshots {
		totalLikes += item.LikeCount
		totalReplies += item.ReplyCount
		totalQuotes += item.QuoteCount
		if score := engagementScore(item); score > bestScore {
			best = item
			bestScore = score
		}
	}

	count := max(1, len(snapshots))
	return []string{
		fmt.Sprintf("Ongoing X memory now tracks %d recent posts with an average of %d likes, %d replies, and %d quotes.", len(snapshots), totalLikes/count, totalReplies/count, totalQuotes/count),
		fmt.Sprintf("The strongest tracked X post so far centers on: %s", preview(best.ContentPreview)),
	}
}

func buildLinkedInInsights(snapshots []Snapshot) []string {
	totalComments := 0
	totalReplies := 0
	best := snapshots[0]
	bestScore := best.CommentCount + best.ReplyCount
	for _, item := range snapshots {
		totalComments += item.CommentCount
		totalReplies += item.ReplyCount
		if score := item.CommentCount + item.ReplyCount; score > bestScore {
			best = item
			bestScore = score
		}
	}

	count := max(1, len(snapshots))
	return []string{
		fmt.Sprintf("Ongoing LinkedIn memory now tracks %d recent posts with an average of %d visible comments and %d authored replies.", len(snapshots), totalComments/count, totalReplies/count),
		fmt.Sprintf("The strongest tracked LinkedIn post so far centers on: %s", preview(best.ContentPreview)),
	}
}

func engagementScore(snapshot Snapshot) int {
	return snapshot.LikeCount + snapshot.ReplyCount + snapshot.QuoteCount
}

func normalizeLinkedInPostURN(value string) string {
	trimmed := strings.TrimSpace(value)
	if strings.HasPrefix(trimmed, "urn:") {
		return trimmed
	}
	return "urn:li:ugcPost:" + trimmed
}

func chooseCapturedAt(postCreatedAt, fallback time.Time) time.Time {
	if postCreatedAt.IsZero() {
		return fallback
	}
	if time.Since(postCreatedAt) < 15*time.Minute {
		return fallback
	}
	return fallback
}

func preview(value string) string {
	trimmed := strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if len(trimmed) <= 120 {
		return trimmed
	}
	return trimmed[:120] + "..."
}

func max(left, right int) int {
	if left > right {
		return left
	}
	return right
}
