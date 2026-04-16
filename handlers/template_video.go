package handlers

// POST /v1/video/template
//
// Generates a template-style slide video from an existing ScheduledPost using
// FFmpeg — no generative AI credits required.
//
// Flow:
//  1. Load the post from the DB by postId.
//  2. Parse its content/script into a sequence of slides.
//  3. Render each slide as a PNG (dark background + white text) with fogleman/gg.
//  4. Encode each PNG to a short MP4 clip with FFmpeg.
//  5. Concatenate all clips into one portrait (1080×1920) MP4.
//  6. Upload to Cloudinary and persist the URL on the post.
//
// Returns { videoUrl, slides, message }.

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

// ─── Request type ─────────────────────────────────────────────────────────────

type TemplateVideoRequest struct {
	PostID uint `json:"postId"`
}

// ─── Handler ──────────────────────────────────────────────────────────────────

func GenerateTemplateVideoHandler(c *gin.Context) {
	var req TemplateVideoRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PostID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "postId is required"})
		return
	}

	if !utils.CloudinaryEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Cloudinary is not configured — cannot store the generated video.",
		})
		return
	}

	var post models.ScheduledPost
	if err := database.DB.First(&post, req.PostID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
		return
	}

	slides := buildSlidesFromPost(post)
	if len(slides) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not extract slides from post content"})
		return
	}

	// Work in a unique temp directory that is cleaned up after the request
	tmpDir, err := os.MkdirTemp("", "furci-tmplvid-*")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create temp workspace"})
		return
	}
	defer os.RemoveAll(tmpDir)

	outputPath := filepath.Join(tmpDir, "output.mp4")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	log.Printf("[TemplateVideo] Building %d slides for post %d (type=%s)", len(slides), post.ID, post.ContentType)

	if err := video.BuildTemplateVideo(ctx, video.TemplateVideoSpec{
		Slides:     slides,
		OutputPath: outputPath,
	}); err != nil {
		log.Printf("[TemplateVideo] Assembly failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Video assembly failed: " + err.Error()})
		return
	}

	log.Printf("[TemplateVideo] Assembly complete — uploading to Cloudinary")

	publicID := fmt.Sprintf("template_%d_%d", post.ID, time.Now().Unix())
	videoURL, uploadErr := utils.UploadVideo(ctx, outputPath, "furci/template-videos", publicID)
	if uploadErr != nil {
		log.Printf("[TemplateVideo] Cloudinary upload failed: %v", uploadErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Upload failed: " + uploadErr.Error()})
		return
	}

	log.Printf("[TemplateVideo] Uploaded: %s", videoURL)

	// Persist the URL on the post so the dashboard can display it
	database.DB.Model(&post).Update("video_url", videoURL)

	c.JSON(http.StatusOK, gin.H{
		"videoUrl": videoURL,
		"slides":   len(slides),
		"message":  fmt.Sprintf("Slide video with %d screens generated successfully.", len(slides)),
	})
}

// ─── Content → Slides ────────────────────────────────────────────────────────

// buildSlidesFromPost converts a ScheduledPost into a []video.Slide based on
// its content type.
func buildSlidesFromPost(post models.ScheduledPost) []video.Slide {
	switch post.ContentType {
	case "thread":
		return slidesFromThread(post)
	case "carousel":
		return slidesFromCarousel(post)
	default: // tweet, linkedin_post, video_script, or anything else
		return slidesFromContent(post)
	}
}

// slidesFromThread treats each tweet in the script as its own slide.
func slidesFromThread(post models.ScheduledPost) []video.Slide {
	src := post.Script
	if src == "" {
		src = post.Content
	}
	lines := extractPoints(src)
	var slides []video.Slide
	for i, line := range lines {
		if line == "" {
			continue
		}
		slides = append(slides, video.Slide{
			Text:     line,
			Duration: 4.0,
			BgHex:    video.BgForIndex(i),
		})
	}
	return slides
}

// slidesFromCarousel uses the SlidesJSON array — one element per slide.
func slidesFromCarousel(post models.ScheduledPost) []video.Slide {
	if post.SlidesJSON != "" {
		var texts []string
		if err := json.Unmarshal([]byte(post.SlidesJSON), &texts); err == nil && len(texts) > 0 {
			var slides []video.Slide
			for i, text := range texts {
				dur := 4.0
				if i == 0 || i == len(texts)-1 {
					dur = 3.0
				}
				slides = append(slides, video.Slide{
					Text:     text,
					Duration: dur,
					BgHex:    video.BgForIndex(i),
				})
			}
			return slides
		}
	}
	return slidesFromContent(post)
}

// slidesFromContent parses plain content/script into hook + body + CTA slides.
func slidesFromContent(post models.ScheduledPost) []video.Slide {
	src := post.Script
	if src == "" {
		src = post.Content
	}
	if src == "" {
		return nil
	}

	points := extractPoints(src)
	if len(points) == 0 {
		return nil
	}

	var slides []video.Slide

	// Hook slide
	slides = append(slides, video.Slide{
		Text:     points[0],
		Duration: 3.5,
		BgHex:    video.BgForIndex(0),
	})

	// Body slides — cap at 4 to keep the video under ~25 seconds
	body := points[1:]
	if len(body) > 4 {
		body = body[:4]
	}
	for i, pt := range body {
		if pt == "" {
			continue
		}
		slides = append(slides, video.Slide{
			Text:     pt,
			Duration: 4.0,
			BgHex:    video.BgForIndex(i + 1),
		})
	}

	// CTA slide
	cta := post.CTAText
	if cta == "" {
		cta = "Follow for more"
	}
	slides = append(slides, video.Slide{
		Text:     cta,
		SubText:  post.Hashtags,
		Duration: 3.0,
		BgHex:    video.BgForIndex(0),
	})

	return slides
}

// extractPoints splits text into individual meaningful points.
// Handles newline-separated lines, numbered lists ("1. ", "1/ "), and
// falls back to sentence splitting for single-block text.
func extractPoints(text string) []string {
	var points []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		// Strip leading list markers: "1.", "1/", "-", "•", "–"
		line = strings.TrimLeftFunc(line, func(r rune) bool {
			return r == '-' || r == '•' || r == '–' || r == '*'
		})
		line = strings.TrimSpace(line)
		// Strip "1/ " or "1. " prefixes
		for i, ch := range line {
			if ch == '/' || ch == '.' {
				prefix := strings.TrimSpace(line[:i])
				if isDigits(prefix) && prefix != "" {
					line = strings.TrimSpace(line[i+1:])
					break
				}
			}
		}
		if len(line) > 10 {
			points = append(points, line)
		}
	}

	// If we only got one big block, split by sentences
	if len(points) <= 1 && len(text) > 80 {
		points = nil
		for _, s := range strings.FieldsFunc(text, func(r rune) bool {
			return r == '.' || r == '!' || r == '?'
		}) {
			s = strings.TrimSpace(s)
			if len(s) > 10 {
				points = append(points, s)
			}
		}
	}
	return points
}

func isDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
