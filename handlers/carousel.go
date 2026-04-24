package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/AntipasBen23/fedey-backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
)

// ─── Request / Response ──────────────────────────────────────────────────────

type CarouselImageRequest struct {
	VisualPrompts []string `json:"visualPrompts"` // One prompt per slide (or single cover prompt)
	CoverPrompt   string   `json:"coverPrompt"`   // Single prompt for the carousel cover slide
	Quality       string   `json:"quality"`       // "standard" (default) or "hd"
	Style         string   `json:"style"`         // "vivid" (default) or "natural"
}

type SlideImage struct {
	Index     int    `json:"index"`
	URL       string `json:"url"`       // Permanent Cloudinary URL (or temp DALL-E URL if Cloudinary not configured)
	Permanent bool   `json:"permanent"` // true = Cloudinary, false = expires in ~1h
	Prompt    string `json:"prompt"`
}

type CarouselImageResponse struct {
	Images  []SlideImage `json:"images"`
	Warning string       `json:"warning,omitempty"`
}

// ─── Handler ──────────────────────────────────────────────────────────────────

// POST /v1/carousel/images
// Generates DALL-E 3 images for carousel slides in parallel.
// If Cloudinary is configured, images are immediately uploaded and permanent URLs are returned.
func GenerateCarouselImagesHandler(c *gin.Context) {
	var req CarouselImageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.APIError(c, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request payload.")
		return
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", "API key missing.")
		return
	}

	// Build list of prompts to generate
	prompts := req.VisualPrompts
	if len(prompts) == 0 && req.CoverPrompt != "" {
		prompts = []string{req.CoverPrompt}
	}
	if len(prompts) == 0 {
		utils.APIError(c, http.StatusBadRequest, "VALIDATION_ERROR", "Provide visualPrompts array or coverPrompt.")
		return
	}

	// Cap at 8 slides to prevent runaway costs
	if len(prompts) > 8 {
		prompts = prompts[:8]
	}

	quality := openai.CreateImageQualityStandard
	if req.Quality == "hd" {
		quality = openai.CreateImageQualityHD
	}

	style := openai.CreateImageStyleVivid
	if req.Style == "natural" {
		style = openai.CreateImageStyleNatural
	}

	cldEnabled := utils.CloudinaryEnabled()
	client := openai.NewClient(apiKey)

	// Generate + upload all slides in parallel
	images := make([]SlideImage, len(prompts))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for i, prompt := range prompts {
		wg.Add(1)
		go func(idx int, p string) {
			defer wg.Done()

			fullPrompt := fmt.Sprintf(
				"Social media carousel slide design. %s. Clean, modern, high contrast, professional. No text overlays. Suitable as a background for a carousel post.",
				p,
			)

			dalleResp, err := client.CreateImage(
				context.Background(),
				openai.ImageRequest{
					Model:   openai.CreateImageModelDallE3,
					Prompt:  fullPrompt,
					N:       1,
					Size:    openai.CreateImageSize1024x1024,
					Quality: quality,
					Style:   style,
				},
			)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}

			dalleURL := dalleResp.Data[0].URL
			finalURL := dalleURL
			permanent := false

			// Immediately upload to Cloudinary if configured
			if cldEnabled {
				publicID := fmt.Sprintf("slide_%d", idx+1)
				uploaded, uploadErr := utils.UploadImage(
					context.Background(),
					dalleURL,
					"furci/carousel",
					publicID,
				)
				if uploadErr != nil {
					log.Printf("[Carousel] Cloudinary upload failed for slide %d: %v — using temp URL", idx+1, uploadErr)
				} else {
					finalURL = uploaded
					permanent = true
					log.Printf("[Carousel] Slide %d uploaded to Cloudinary: %s", idx+1, finalURL)
				}
			}

			mu.Lock()
			images[idx] = SlideImage{
				Index:     idx + 1,
				URL:       finalURL,
				Permanent: permanent,
				Prompt:    p,
			}
			mu.Unlock()
		}(i, prompt)
	}

	wg.Wait()

	if firstErr != nil && images[0].URL == "" {
		utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", "Failed to generate images.")
		return
	}

	var result []SlideImage
	for _, img := range images {
		if img.URL != "" {
			result = append(result, img)
		}
	}

	warning := ""
	if !cldEnabled {
		warning = "Cloudinary not configured — image URLs expire in ~1 hour. Add CLOUDINARY_CLOUD_NAME, CLOUDINARY_API_KEY, CLOUDINARY_API_SECRET to enable permanent storage."
	}

	c.JSON(http.StatusOK, CarouselImageResponse{
		Images:  result,
		Warning: warning,
	})
}
