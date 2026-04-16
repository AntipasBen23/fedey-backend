package video

// Pexels stock footage integration.
// FetchStockClip searches Pexels Videos for a clip matching a query and
// downloads only the first few seconds via FFmpeg — avoids pulling full files.
//
// Requires PEXELS_API_KEY env var (free at pexels.com/api).

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// ─── Pexels API types ─────────────────────────────────────────────────────────

type pexelsVideoResult struct {
	TotalResults int             `json:"total_results"`
	Videos       []pexelsVideo   `json:"videos"`
}

type pexelsVideo struct {
	ID         int                `json:"id"`
	Duration   int                `json:"duration"`
	VideoFiles []pexelsVideoFile  `json:"video_files"`
}

type pexelsVideoFile struct {
	ID       int    `json:"id"`
	Quality  string `json:"quality"`         // "hd" | "sd" | "uhd"
	FileType string `json:"file_type"`        // "video/mp4"
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Link     string `json:"link"`
}

// ─── Public API ───────────────────────────────────────────────────────────────

// PexelsEnabled returns true when a Pexels API key is configured.
func PexelsEnabled() bool {
	return os.Getenv("PEXELS_API_KEY") != ""
}

// FetchStockClip finds a Pexels video matching query and downloads a clip
// of up to maxSeconds to destPath. Falls back gracefully on any error.
func FetchStockClip(ctx context.Context, query, destPath string, maxSeconds float64) error {
	apiKey := os.Getenv("PEXELS_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("PEXELS_API_KEY not set")
	}

	link, err := searchPexels(ctx, apiKey, query)
	if err != nil {
		return fmt.Errorf("pexels search: %w", err)
	}

	return downloadClipSegment(ctx, link, destPath, maxSeconds)
}

// SearchQuery derives a short, search-engine-friendly query from slide text.
// It strips stop-words and takes the first 3 meaningful keywords.
func SearchQuery(text string) string {
	stop := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "your": true, "our": true,
		"is": true, "are": true, "was": true, "be": true, "that": true,
		"this": true, "it": true, "how": true, "why": true, "what": true,
		"you": true, "we": true, "they": true, "i": true, "my": true,
		"will": true, "can": true, "from": true, "by": true, "not": true,
	}

	words := strings.Fields(text)
	var keywords []string
	for _, w := range words {
		w = strings.ToLower(strings.Trim(w, ".,!?:;\"'👉🏽🔥✅❌•–—"))
		if len(w) > 2 && !stop[w] {
			keywords = append(keywords, w)
		}
		if len(keywords) >= 3 {
			break
		}
	}
	if len(keywords) == 0 {
		return "professional business"
	}
	return strings.Join(keywords, " ")
}

// ─── Internal ─────────────────────────────────────────────────────────────────

func searchPexels(ctx context.Context, apiKey, query string) (string, error) {
	endpoint := fmt.Sprintf(
		"https://api.pexels.com/videos/search?query=%s&orientation=portrait&size=medium&per_page=5",
		url.QueryEscape(query),
	)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Pexels returned HTTP %d", resp.StatusCode)
	}

	var result pexelsVideoResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Videos) == 0 {
		// Try a broader fallback query (first word only)
		if spaceIdx := strings.Index(query, " "); spaceIdx > 0 {
			return searchPexels(ctx, apiKey, query[:spaceIdx])
		}
		return "", fmt.Errorf("no Pexels results for %q", query)
	}

	return bestVideoLink(result.Videos), nil
}

// bestVideoLink picks the best MP4 link from a set of Pexels videos.
// Priority: portrait HD > portrait SD > landscape HD > any MP4.
func bestVideoLink(videos []pexelsVideo) string {
	type candidate struct {
		link     string
		priority int // higher = better
	}
	var best candidate

	for _, vid := range videos {
		for _, f := range vid.VideoFiles {
			if f.FileType != "video/mp4" || f.Link == "" {
				continue
			}
			p := 0
			if f.Height >= f.Width { // portrait
				p += 10
			}
			switch f.Quality {
			case "hd":
				p += 3
			case "sd":
				p += 1
			}
			// Prefer medium resolution — not too small, not too heavy
			if f.Width >= 720 && f.Width <= 1920 {
				p += 2
			}
			if p > best.priority {
				best = candidate{f.Link, p}
			}
		}
	}
	return best.link
}

// downloadClipSegment fetches and trims a remote video to maxSeconds using FFmpeg.
// Only the data needed to produce maxSeconds of output is buffered.
func downloadClipSegment(ctx context.Context, videoURL, destPath string, maxSeconds float64) error {
	args := []string{
		"-y",
		"-i", videoURL,
		"-t", fmt.Sprintf("%.2f", maxSeconds),
		"-vf", "format=yuv420p",
		"-c:v", "libx264",
		"-preset", "fast",
		"-an", // strip audio — we don't use it in the video slides
		destPath,
	}
	return runFFmpeg(ctx, args)
}
