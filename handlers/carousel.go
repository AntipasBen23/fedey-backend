package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"

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
	Index  int    `json:"index"`
	URL    string `json:"url"`
	Prompt string `json:"prompt"`
}

type CarouselImageResponse struct {
	Images  []SlideImage `json:"images"`
	Warning string       `json:"warning,omitempty"`
}

// ─── Handler ──────────────────────────────────────────────────────────────────

// POST /v1/carousel/images
// Generates DALL-E 3 images for carousel slides in parallel.
// Each URL is valid for ~1 hour — download and store on the frontend if persistence is needed.
func GenerateCarouselImagesHandler(c *gin.Context) {
	var req CarouselImageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "API Key missing."})
		return
	}

	// Build list of prompts to generate
	prompts := req.VisualPrompts
	if len(prompts) == 0 && req.CoverPrompt != "" {
		prompts = []string{req.CoverPrompt}
	}
	if len(prompts) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provide visualPrompts array or coverPrompt"})
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

	client := openai.NewClient(apiKey)

	// Generate all slides in parallel
	images := make([]SlideImage, len(prompts))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for i, prompt := range prompts {
		wg.Add(1)
		go func(idx int, p string) {
			defer wg.Done()

			// Add carousel design context to every prompt
			fullPrompt := fmt.Sprintf(
				"Social media carousel slide design. %s. Clean, modern, high contrast, professional. No text overlays. Suitable as a background for a carousel post.",
				p,
			)

			resp, err := client.CreateImage(
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

			mu.Lock()
			images[idx] = SlideImage{
				Index:  idx + 1,
				URL:    resp.Data[0].URL,
				Prompt: p,
			}
			mu.Unlock()
		}(i, prompt)
	}

	wg.Wait()

	if firstErr != nil && images[0].URL == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate images: " + firstErr.Error()})
		return
	}

	// Filter out any slots that failed (partial success is OK)
	var result []SlideImage
	for _, img := range images {
		if img.URL != "" {
			result = append(result, img)
		}
	}

	c.JSON(http.StatusOK, CarouselImageResponse{
		Images:  result,
		Warning: "Image URLs expire after ~1 hour. Download and store them if you need persistence.",
	})
}
