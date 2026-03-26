package trends

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/AntipasBen23/fedey-backend/internal/brandmemory"
)

type LiveIngestInput struct {
	Source    string `json:"source"`
	Query     string `json:"query"`
	Subreddit string `json:"subreddit"`
	Limit     int    `json:"limit"`
}

type SourceSpec struct {
	Source    string `json:"source"`
	Query     string `json:"query,omitempty"`
	Subreddit string `json:"subreddit,omitempty"`
	Limit     int    `json:"limit"`
}

func (s *Service) IngestLive(ctx context.Context, input LiveIngestInput) ([]Signal, error) {
	source := strings.ToLower(strings.TrimSpace(input.Source))
	if source == "" {
		return nil, ErrInvalidSignalInput
	}

	limit := input.Limit
	if limit <= 0 || limit > 10 {
		limit = 5
	}

	var fetched []Signal
	var err error
	switch source {
	case "reddit":
		fetched, err = fetchRedditSignals(ctx, strings.TrimSpace(input.Subreddit), limit)
	case "google_news":
		fetched, err = fetchGoogleNewsSignals(ctx, strings.TrimSpace(input.Query), limit)
	default:
		return nil, ErrInvalidSignalInput
	}
	if err != nil {
		return nil, err
	}

	return s.storeUnique(ctx, fetched)
}

func (s *Service) IngestDefaults(ctx context.Context, profile brandmemory.Profile) ([]Signal, error) {
	specs := buildDefaultSourceSpecs(profile)
	created := make([]Signal, 0, len(specs)*2)
	for _, spec := range specs {
		items, err := s.IngestLive(ctx, LiveIngestInput(spec))
		if err != nil {
			continue
		}
		created = append(created, items...)
	}

	return created, nil
}

func buildDefaultSourceSpecs(profile brandmemory.Profile) []SourceSpec {
	query := strings.TrimSpace(profile.BrandName)
	if query == "" && len(profile.Pillars) > 0 {
		query = profile.Pillars[0]
	}
	if query == "" {
		query = "ai marketing automation"
	}

	specs := []SourceSpec{
		{Source: "google_news", Query: query, Limit: 4},
		{Source: "reddit", Subreddit: "artificial", Limit: 4},
	}
	if len(profile.Pillars) > 0 {
		specs = append(specs, SourceSpec{Source: "google_news", Query: profile.Pillars[0], Limit: 4})
	}

	return specs
}

func (s *Service) storeUnique(ctx context.Context, signals []Signal) ([]Signal, error) {
	existing, err := s.repository.List(ctx)
	if err != nil {
		return nil, err
	}

	known := make(map[string]struct{}, len(existing))
	for _, item := range existing {
		key := strings.ToLower(strings.TrimSpace(item.Source + "|" + item.Topic))
		known[key] = struct{}{}
	}

	created := make([]Signal, 0, len(signals))
	for _, signal := range signals {
		key := strings.ToLower(strings.TrimSpace(signal.Source + "|" + signal.Topic))
		if _, exists := known[key]; exists {
			continue
		}

		item, err := s.Create(ctx, CreateInput{
			Topic:     signal.Topic,
			Source:    signal.Source,
			Angle:     signal.Angle,
			Velocity:  signal.Velocity,
			Relevance: signal.Relevance,
		})
		if err != nil {
			return created, err
		}
		known[key] = struct{}{}
		created = append(created, item)
	}

	return created, nil
}

func fetchRedditSignals(ctx context.Context, subreddit string, limit int) ([]Signal, error) {
	if subreddit == "" {
		subreddit = "artificial"
	}

	endpoint := fmt.Sprintf("https://www.reddit.com/r/%s/hot.json?limit=%d&raw_json=1", url.PathEscape(subreddit), limit)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build reddit request: %w", err)
	}
	request.Header.Set("User-Agent", "fedey-trend-ingestor/1.0")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("perform reddit request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("reddit trend fetch failed: status=%d", response.StatusCode)
	}

	var payload struct {
		Data struct {
			Children []struct {
				Data struct {
					Title       string  `json:"title"`
					SelfText    string  `json:"selftext"`
					Score       int     `json:"score"`
					NumComments int     `json:"num_comments"`
					CreatedUTC  float64 `json:"created_utc"`
				} `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode reddit response: %w", err)
	}

	signals := make([]Signal, 0, len(payload.Data.Children))
	for _, child := range payload.Data.Children {
		angle := strings.TrimSpace(child.Data.SelfText)
		if angle == "" {
			angle = "Live Reddit discussion is gaining traction around this topic."
		}
		relevance := 0.6
		if child.Data.Score > 100 {
			relevance = 0.8
		}
		signals = append(signals, Signal{
			Topic:      strings.TrimSpace(child.Data.Title),
			Source:     "reddit",
			Angle:      truncate(angle, 220),
			Velocity:   child.Data.Score + child.Data.NumComments,
			Relevance:  relevance,
			ObservedAt: time.Now().UTC(),
		})
	}

	return signals, nil
}

func fetchGoogleNewsSignals(ctx context.Context, query string, limit int) ([]Signal, error) {
	if query == "" {
		query = "ai marketing"
	}

	endpoint := "https://news.google.com/rss/search?q=" + url.QueryEscape(query)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build google news request: %w", err)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("perform google news request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("google news trend fetch failed: status=%d", response.StatusCode)
	}

	var feed struct {
		Channel struct {
			Items []struct {
				Title       string `xml:"title"`
				Description string `xml:"description"`
				PubDate     string `xml:"pubDate"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	if err := xml.NewDecoder(response.Body).Decode(&feed); err != nil {
		return nil, fmt.Errorf("decode google news response: %w", err)
	}

	if limit > len(feed.Channel.Items) {
		limit = len(feed.Channel.Items)
	}

	signals := make([]Signal, 0, limit)
	for _, item := range feed.Channel.Items[:limit] {
		relevance := 0.72
		if publishedAt, err := time.Parse(time.RFC1123Z, item.PubDate); err == nil && time.Since(publishedAt) < 24*time.Hour {
			relevance = 0.84
		}

		signals = append(signals, Signal{
			Topic:      strings.TrimSpace(item.Title),
			Source:     "google_news",
			Angle:      truncate(stripTags(item.Description), 220),
			Velocity:   100 - len(signals)*7,
			Relevance:  relevance,
			ObservedAt: time.Now().UTC(),
		})
	}

	return signals, nil
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}

	return strings.TrimSpace(value[:limit]) + "..."
}

func stripTags(value string) string {
	replacer := strings.NewReplacer("<b>", "", "</b>", "", "<br>", " ", "<br/>", " ", "&nbsp;", " ")
	return strings.TrimSpace(replacer.Replace(value))
}
