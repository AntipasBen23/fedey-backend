package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/AntipasBen23/fedey-backend/utils"
	"github.com/gin-gonic/gin"
	openai "github.com/sashabaranov/go-openai"
)

// Runway ML API base URL (Gen-3 Alpha)
const runwayBaseURL = "https://api.dev.runwayml.com/v1"

// ─── Request / Response Types ────────────────────────────────────────────────

type VideoGenerateRequest struct {
	// Text prompt describing the video scene (used for text-to-video)
	PromptText string `json:"promptText"`
	// Optional: a publicly accessible image URL to use as the first frame (image-to-video)
	PromptImageURL string `json:"promptImageUrl,omitempty"`
	// 5 or 10 seconds. Default: 5
	Duration int `json:"duration,omitempty"`
	// "1280:720" (landscape) | "720:1280" (portrait/Reels) | "1104:832" (square-ish). Default: portrait
	Ratio string `json:"ratio,omitempty"`
	// Watermark the video? Default: false
	Watermark bool `json:"watermark,omitempty"`
}

type VideoTaskResponse struct {
	TaskID    string `json:"taskId"`
	Status    string `json:"status"`  // PENDING | RUNNING | SUCCEEDED | FAILED
	VideoURL  string `json:"videoUrl,omitempty"`
	Message   string `json:"message,omitempty"`
}

// ─── Internal Runway types ───────────────────────────────────────────────────

// runwayImageInput is the format Runway API expects for promptImage entries
type runwayImageInput struct {
	URI      string `json:"uri"`
	Position string `json:"position"` // "first" | "last"
}

type runwayCreateRequest struct {
	Model        string             `json:"model"`
	PromptText   string             `json:"promptText,omitempty"`
	PromptImage  []runwayImageInput `json:"promptImage"`
	Duration     int                `json:"duration"`
	Ratio        string             `json:"ratio"`
	Watermark    bool               `json:"watermark"`
}

type runwayTaskStatus struct {
	ID      string   `json:"id"`
	Status  string   `json:"status"`
	Output  []string `json:"output"`
	Failure string   `json:"failure,omitempty"`
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func runwayRequest(method, path string, body interface{}) ([]byte, int, error) {
	apiKey := os.Getenv("RUNWAY_API_KEY")
	if apiKey == "" {
		return nil, 0, fmt.Errorf("RUNWAY_API_KEY not set")
	}

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, runwayBaseURL+path, reqBody)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Runway-Version", "2024-11-06")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	return data, resp.StatusCode, err
}

// generateStartingFrame uses DALL-E 3 to create a starting frame image for the video.
// This is required by gen4_turbo which needs an input image to animate.
func generateStartingFrame(promptText string) (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY not set")
	}

	client := openai.NewClient(apiKey)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Build a visual prompt for the starting frame
	visualPrompt := fmt.Sprintf(
		"Cinematic still frame, professional photography. %s. "+
			"High quality, dramatic lighting, suitable as a video thumbnail. "+
			"No text, no watermarks.",
		promptText,
	)
	if len(visualPrompt) > 900 {
		visualPrompt = visualPrompt[:900]
	}

	resp, err := client.CreateImage(ctx, openai.ImageRequest{
		Model:          openai.CreateImageModelDallE3,
		Prompt:         visualPrompt,
		N:              1,
		Size:           openai.CreateImageSize1024x1792, // Portrait — matches 768:1280 ratio
		Quality:        openai.CreateImageQualityStandard,
		ResponseFormat: openai.CreateImageResponseFormatURL,
	})
	if err != nil {
		return "", fmt.Errorf("DALL-E error: %w", err)
	}
	if len(resp.Data) == 0 {
		return "", fmt.Errorf("DALL-E returned no images")
	}

	imageURL := resp.Data[0].URL

	// Upload to Cloudinary immediately so the URL doesn't expire before Runway fetches it
	if utils.CloudinaryEnabled() {
		permanent, uploadErr := utils.UploadImage(context.Background(), imageURL, "furci/video-frames", "")
		if uploadErr == nil {
			return permanent, nil
		}
		log.Printf("[Video] Cloudinary upload of starting frame failed, using DALL-E URL directly: %v", uploadErr)
	}

	return imageURL, nil
}

// ─── Handlers ────────────────────────────────────────────────────────────────

// POST /v1/video/generate
// Kicks off an async Runway ML video generation task.
// gen4_turbo requires a starting frame image — if none is provided, one is
// auto-generated with DALL-E 3 from the prompt text.
// Returns a taskId — poll GET /v1/video/status/:taskId for the result.
func GenerateVideoHandler(c *gin.Context) {
	var req VideoGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	if req.PromptText == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "promptText is required"})
		return
	}

	if os.Getenv("RUNWAY_API_KEY") == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Video generation is not configured. Set RUNWAY_API_KEY to enable this feature.",
		})
		return
	}

	duration := req.Duration
	if duration != 5 && duration != 10 {
		duration = 5
	}

	ratio := req.Ratio
	if ratio == "" {
		ratio = "720:1280" // Portrait (Reels/TikTok-friendly default)
	}

	// gen4_turbo is an image-to-video model — a starting frame is required.
	// Auto-generate one with DALL-E 3 if the caller didn't provide one.
	imageURL := req.PromptImageURL
	if imageURL == "" {
		log.Printf("[Video] No starting frame provided — generating one with DALL-E 3")
		generated, err := generateStartingFrame(req.PromptText)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Could not generate starting frame: " + err.Error(),
			})
			return
		}
		imageURL = generated
		log.Printf("[Video] Starting frame generated: %s", imageURL[:min(60, len(imageURL))])
	}

	payload := runwayCreateRequest{
		Model:      "gen4_turbo",
		PromptText: req.PromptText,
		PromptImage: []runwayImageInput{
			{URI: imageURL, Position: "first"},
		},
		Duration:  duration,
		Ratio:     ratio,
		Watermark: req.Watermark,
	}

	// Log the exact payload being sent so we can diagnose Runway rejections
	payloadBytes, _ := json.Marshal(payload)
	log.Printf("[Video] Sending to Runway: %s", string(payloadBytes))

	data, status, err := runwayRequest("POST", "/image_to_video", payload)
	if err != nil {
		log.Printf("[Video] Runway request error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Runway API error: " + err.Error()})
		return
	}
	log.Printf("[Video] Runway response status=%d body=%s", status, string(data))
	if status >= 400 {
		// Return the full Runway error to the frontend so it's visible in the browser
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "Runway error (" + fmt.Sprintf("%d", status) + "): " + string(data),
			"detail": string(data),
		})
		return
	}

	var task runwayTaskStatus
	if err := json.Unmarshal(data, &task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse Runway response"})
		return
	}

	c.JSON(http.StatusAccepted, VideoTaskResponse{
		TaskID:  task.ID,
		Status:  task.Status,
		Message: "Video generation started. Poll GET /v1/video/status/" + task.ID + " for progress.",
	})
}

// GET /v1/video/status/:taskId
// Polls Runway ML for the status of a video generation task.
// When Status == "SUCCEEDED", VideoURL is populated.
func GetVideoStatusHandler(c *gin.Context) {
	taskID := c.Param("taskId")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "taskId is required"})
		return
	}

	if os.Getenv("RUNWAY_API_KEY") == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Video generation not configured."})
		return
	}

	data, status, err := runwayRequest("GET", "/tasks/"+taskID, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Runway API error: " + err.Error()})
		return
	}
	if status >= 400 {
		c.JSON(status, gin.H{"error": "Failed to fetch task", "detail": string(data)})
		return
	}

	var task runwayTaskStatus
	if err := json.Unmarshal(data, &task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse Runway response"})
		return
	}

	resp := VideoTaskResponse{
		TaskID: task.ID,
		Status: task.Status,
	}

	if task.Status == "SUCCEEDED" && len(task.Output) > 0 {
		runwayURL := task.Output[0]
		finalURL := runwayURL
		permanent := false

		// Auto-upload to Cloudinary if configured
		if utils.CloudinaryEnabled() {
			uploaded, uploadErr := utils.UploadVideo(
				context.Background(),
				runwayURL,
				"furci/videos",
				"video_"+taskID,
			)
			if uploadErr != nil {
				log.Printf("[Video] Cloudinary upload failed for task %s: %v — using Runway URL", taskID, uploadErr)
			} else {
				finalURL = uploaded
				permanent = true
				log.Printf("[Video] Video uploaded to Cloudinary: %s", finalURL)
			}
		}

		resp.VideoURL = finalURL
		if permanent {
			resp.Message = "Video is ready and saved permanently to Cloudinary."
		} else {
			resp.Message = "Video is ready. Note: Runway URLs are temporary — configure Cloudinary for permanent storage."
		}
	} else if task.Status == "FAILED" {
		resp.Message = "Video generation failed: " + task.Failure
	} else {
		resp.Message = "Still processing — check back in a few seconds."
	}

	c.JSON(http.StatusOK, resp)
}

// DELETE /v1/video/task/:taskId
// Cancels an in-progress Runway task.
func CancelVideoTaskHandler(c *gin.Context) {
	taskID := c.Param("taskId")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "taskId is required"})
		return
	}

	data, status, err := runwayRequest("DELETE", "/tasks/"+taskID, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if status >= 400 {
		c.JSON(status, gin.H{"error": string(data)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task cancelled."})
}
