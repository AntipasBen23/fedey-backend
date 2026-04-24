package handlers

// POST /v1/carousel/design
//
// Generates a set of fully-designed carousel slide images (1080×1080 PNG)
// from an existing ScheduledPost using the video/assembler rendering engine.
//
// Flow:
//  1. Load the post from the DB by postId.
//  2. Parse its content/script/SlidesJSON into CarouselSlide structs.
//  3. Render each slide as a 1080×1080 PNG (dark bg + white text + branding).
//  4. Upload each PNG to Cloudinary.
//  5. Persist the URLs in post.ImageUrlsJson and return them.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/AntipasBen23/fedey-backend/utils"
	"github.com/AntipasBen23/fedey-backend/video"
	"github.com/gin-gonic/gin"
)

type CarouselDesignRequest struct {
	PostID uint `json:"postId"`
}

// GenerateCarouselDesignHandler renders and uploads designed carousel slides.
func GenerateCarouselDesignHandler(c *gin.Context) {
	var req CarouselDesignRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PostID == 0 {
		utils.APIError(c, http.StatusBadRequest, "VALIDATION_ERROR", "postId is required.")
		return
	}

	if !utils.CloudinaryEnabled() {
		utils.APIError(c, http.StatusServiceUnavailable, "SERVER_ERROR", "Cloudinary is not configured.")
		return
	}

	var post models.ScheduledPost
	if err := database.DB.First(&post, req.PostID).Error; err != nil {
		utils.APIError(c, http.StatusNotFound, "NOT_FOUND", "Post not found.")
		return
	}

	slides := carouselSlidesFromPost(post)
	if len(slides) == 0 {
		utils.APIError(c, http.StatusBadRequest, "VALIDATION_ERROR", "Could not extract slides from post content.")
		return
	}

	tmpDir, err := os.MkdirTemp("", "furci-carousel-*")
	if err != nil {
		utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", "Failed to create temp workspace.")
		return
	}
	defer os.RemoveAll(tmpDir)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	log.Printf("[CarouselDesign] Rendering %d slides for post %d", len(slides), post.ID)

	var imageURLs []string
	for i, slide := range slides {
		pngPath := filepath.Join(tmpDir, fmt.Sprintf("carousel_%d.png", i))

		if err := video.RenderCarouselSlide(slide, pngPath); err != nil {
			log.Printf("[CarouselDesign] Slide %d render failed: %v", i, err)
			utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", fmt.Sprintf("Slide %d render failed.", i+1))
			return
		}

		publicID := fmt.Sprintf("carousel_%d_slide%d_%d", post.ID, i+1, time.Now().Unix())
		url, uploadErr := utils.UploadImage(ctx, pngPath, "furci/carousel-designs", publicID)
		if uploadErr != nil {
			log.Printf("[CarouselDesign] Upload slide %d failed: %v", i, uploadErr)
			utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", fmt.Sprintf("Upload failed for slide %d.", i+1))
			return
		}

		imageURLs = append(imageURLs, url)
		log.Printf("[CarouselDesign] Slide %d uploaded: %s", i+1, url)
	}

	// Persist the URLs on the post
	urlsJSON, _ := json.Marshal(imageURLs)
	database.DB.Model(&post).Update("image_urls_json", string(urlsJSON))

	c.JSON(http.StatusOK, gin.H{
		"imageUrls": imageURLs,
		"slides":    len(imageURLs),
		"message":   fmt.Sprintf("%d designed carousel slides generated.", len(imageURLs)),
	})
}

// ─── Content → CarouselSlides ─────────────────────────────────────────────────

func carouselSlidesFromPost(post models.ScheduledPost) []video.CarouselSlide {
	// Prefer explicit SlidesJSON if present
	if post.SlidesJSON != "" {
		var texts []string
		if err := json.Unmarshal([]byte(post.SlidesJSON), &texts); err == nil && len(texts) > 0 {
			return textsToCarouselSlides(texts)
		}
	}

	// Fall back to parsing the script or content
	src := post.Script
	if src == "" {
		src = post.Content
	}
	points := extractPoints(src) // reuse helper from template_video.go
	if len(points) == 0 {
		return nil
	}
	return textsToCarouselSlides(points)
}

// textsToCarouselSlides converts a flat list of text strings into
// CarouselSlide structs with hook → body → CTA layout.
func textsToCarouselSlides(texts []string) []video.CarouselSlide {
	// Cap at 8 slides — most carousels are 5-8
	if len(texts) > 8 {
		texts = texts[:8]
	}
	total := len(texts)

	palette := []string{"0d1117", "0f172a", "1e1b4b", "14243b", "1a1a2e", "0d1117", "0f172a", "1e1b4b"}

	var slides []video.CarouselSlide
	for i, text := range texts {
		slide := video.CarouselSlide{
			SlideNum:    i + 1,
			TotalSlides: total,
			BgHex:       palette[i%len(palette)],
		}

		// Split text into title + body if it's long enough
		if len(text) > 80 {
			// First sentence → title, rest → body
			if idx := strings.IndexAny(text, ".!?"); idx > 20 && idx < len(text)-1 {
				slide.Title = strings.TrimSpace(text[:idx+1])
				slide.Body = strings.TrimSpace(text[idx+1:])
			} else {
				slide.Title = text
			}
		} else {
			slide.Title = text
		}

		slides = append(slides, slide)
	}
	return slides
}
