package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/AntipasBen23/fedey-backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
)

// ─── Request / Response Types ────────────────────────────────────────────────

type ScriptRequest struct {
	Topic     string `json:"topic"`    // What is the content about
	Platform  string `json:"platform"` // twitter | linkedin | tiktok | instagram
	Format    string `json:"format"`   // video_script | thread | carousel
	ToneStyle string `json:"toneStyle"` // professional | casual | funny | educational | storytelling
	Duration  string `json:"duration"` // "30s" | "60s" | "90s" (for video only)
}

type ScriptScene struct {
	StartTime   string `json:"startTime"`
	EndTime     string `json:"endTime"`
	OnScreen    string `json:"onScreen"`    // What to show visually
	Voiceover   string `json:"voiceover"`   // What to say
	BRoll       string `json:"bRoll"`       // B-roll / visual suggestions
	TextOverlay string `json:"textOverlay"` // Text displayed on screen
}

type VideoScript struct {
	Title       string        `json:"title"`
	TotalTime   string        `json:"totalTime"`
	Hook        string        `json:"hook"`
	Caption     string        `json:"caption"`
	Scenes      []ScriptScene `json:"scenes"`
	CTAText     string        `json:"ctaText"`
	Hashtags    []string      `json:"hashtags"`
	VisualStyle string        `json:"visualStyle"` // e.g. "Talking head, casual home setup, warm light"
	MusicVibes  string        `json:"musicVibes"`  // e.g. "Upbeat lo-fi, no lyrics"
}

type CarouselSlide struct {
	SlideNumber int    `json:"slideNumber"`
	Headline    string `json:"headline"`
	Subtext     string `json:"subtext"`
	VisualNote  string `json:"visualNote"`  // What to put as background/visual
	DesignNote  string `json:"designNote"`  // Color/font/layout direction
}

type CarouselScript struct {
	Title       string          `json:"title"`
	Caption     string          `json:"caption"`
	Slides      []CarouselSlide `json:"slides"`
	CTAText     string          `json:"ctaText"`
	ColorScheme string          `json:"colorScheme"` // e.g. "Navy + white, bold sans-serif"
	Hashtags    []string        `json:"hashtags"`
}

type ThreadScript struct {
	Title    string   `json:"title"`
	Tweets   []string `json:"tweets"` // Each tweet numbered: "1/ Hook here..."
	CTAText  string   `json:"ctaText"`
	Hashtags []string `json:"hashtags"`
}

type ScriptResponse struct {
	Format   string          `json:"format"`
	Video    *VideoScript    `json:"video,omitempty"`
	Carousel *CarouselScript `json:"carousel,omitempty"`
	Thread   *ThreadScript   `json:"thread,omitempty"`
}

// ─── Prompts ──────────────────────────────────────────────────────────────────

const videoScriptPrompt = `You are Furci AI, a professional video scriptwriter and social media strategist.

Write a complete, production-ready %s video script for this topic: "%s"
Platform: %s
Tone/Style: %s

The script must be written exactly how a skilled human content creator would write it — punchy, clear, no filler words.

Structure the video with these scenes:
- [0-3s] HOOK: Something so unexpected or bold the viewer can't scroll past
- [3-20s] PROBLEM/CONTEXT: Set up why this matters. Relatable pain point or situation.
- [20-50s] VALUE/SOLUTION: Step-by-step insight, story, or demonstration. This is the meat.
- [50-end] CTA: One clear action. No more than one ask.

For each scene provide:
- startTime / endTime
- onScreen: Exactly what the viewer sees (camera angle, action, graphic, text)
- voiceover: Word-for-word what is said (write it conversationally, not like an essay)
- bRoll: Any cutaway visuals or B-roll suggestions
- textOverlay: Any on-screen text/captions to show

Also provide:
- hook: The very first line said on camera (this determines if they watch or scroll)
- caption: Full post caption ready to publish with line breaks
- ctaText: The CTA line spoken and shown
- hashtags: 4-5 niche hashtags
- visualStyle: Camera setup and aesthetic direction
- musicVibes: Background music style recommendation

Return ONLY valid JSON matching this exact schema (no markdown, no explanation):
{
  "title": "...",
  "totalTime": "...",
  "hook": "...",
  "caption": "...",
  "scenes": [
    {
      "startTime": "0s",
      "endTime": "3s",
      "onScreen": "...",
      "voiceover": "...",
      "bRoll": "...",
      "textOverlay": "..."
    }
  ],
  "ctaText": "...",
  "hashtags": ["#tag"],
  "visualStyle": "...",
  "musicVibes": "..."
}`

const carouselScriptPrompt = `You are Furci AI, a professional carousel designer and copywriter.

Design a complete carousel for this topic: "%s"
Platform: %s
Tone/Style: %s

Write it exactly how the best-performing LinkedIn/Instagram carousels work:
- Slide 1: Bold hook that makes someone STOP scrolling and swipe
- Slides 2-6: One clear insight, stat, or tip per slide. SHORT. Maximum 2-3 lines per slide.
- Slide 7 (optional extra value): "Bonus" insight or summary
- Last slide: Strong CTA + "Follow for more [topic]" prompt

For each slide provide:
- slideNumber
- headline: The main text (big, bold — this is what people read first)
- subtext: Supporting copy (optional — keep it short)
- visualNote: What image, icon, or graphic goes on this slide
- designNote: Color, font size, layout direction for this specific slide

Also provide:
- caption: Full post caption for when the carousel is posted
- colorScheme: Overall design direction (e.g., "Deep navy background, white bold text, orange accent")
- ctaText: The call to action on the final slide
- hashtags: 4-5 niche hashtags

Return ONLY valid JSON matching this exact schema (no markdown, no explanation):
{
  "title": "...",
  "caption": "...",
  "slides": [
    {
      "slideNumber": 1,
      "headline": "...",
      "subtext": "...",
      "visualNote": "...",
      "designNote": "..."
    }
  ],
  "ctaText": "...",
  "colorScheme": "...",
  "hashtags": ["#tag"]
}`

const threadScriptPrompt = `You are Furci AI, a professional Twitter/X thread writer.

Write a viral-worthy thread about: "%s"
Tone/Style: %s

Rules for a great thread:
- Tweet 1: The hook. Make a bold claim or ask a provocative question. No "Thread:" label — just the hook.
- Tweets 2-8: One insight, step, or story beat per tweet. Max 260 chars each. Short sentences. Line breaks for breathing room.
- Last tweet: CTA — ask a question, tell them to follow, or drive to a link.

Write every tweet in full, numbered (1/, 2/, etc.). Make each tweet work as a standalone so people can retweet any of them.

Return ONLY valid JSON matching this exact schema (no markdown, no explanation):
{
  "title": "...",
  "tweets": [
    "1/ Hook tweet here...",
    "2/ Second tweet here...",
    "..."
  ],
  "ctaText": "...",
  "hashtags": ["#tag"]
}`

// ─── Handler ──────────────────────────────────────────────────────────────────

func GenerateScriptHandler(c *gin.Context) {
	var req ScriptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.APIError(c, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request payload.")
		return
	}

	if req.Topic == "" || req.Format == "" {
		utils.APIError(c, http.StatusBadRequest, "MISSING_FIELDS", "topic and format are required.")
		return
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", "API key missing.")
		return
	}

	toneStyle := req.ToneStyle
	if toneStyle == "" {
		toneStyle = "professional but conversational"
	}
	platform := req.Platform
	if platform == "" {
		platform = "social media"
	}
	duration := req.Duration
	if duration == "" {
		duration = "60s"
	}

	client := openai.NewClient(apiKey)

	var prompt string
	switch req.Format {
	case "video_script":
		prompt = fmt.Sprintf(videoScriptPrompt, duration, req.Topic, platform, toneStyle)
	case "carousel":
		prompt = fmt.Sprintf(carouselScriptPrompt, req.Topic, platform, toneStyle)
	case "thread":
		prompt = fmt.Sprintf(threadScriptPrompt, req.Topic, toneStyle)
	default:
		utils.APIError(c, http.StatusBadRequest, "VALIDATION_ERROR", "format must be one of: video_script, carousel, thread.")
		return
	}

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4o,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
			ResponseFormat: &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONObject,
			},
		},
	)

	if err != nil {
		utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", "Failed to generate script.")
		return
	}

	result := ScriptResponse{Format: req.Format}
	raw := resp.Choices[0].Message.Content

	switch req.Format {
	case "video_script":
		var v VideoScript
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", "Failed to parse video script.")
			return
		}
		result.Video = &v
	case "carousel":
		var cr CarouselScript
		if err := json.Unmarshal([]byte(raw), &cr); err != nil {
			utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", "Failed to parse carousel script.")
			return
		}
		result.Carousel = &cr
	case "thread":
		var t ThreadScript
		if err := json.Unmarshal([]byte(raw), &t); err != nil {
			utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", "Failed to parse thread script.")
			return
		}
		result.Thread = &t
	}

	c.JSON(http.StatusOK, result)
}
