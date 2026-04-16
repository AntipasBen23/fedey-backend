// Package video provides FFmpeg-based template video assembly for Furci AI.
// It renders a sequence of text slides as a portrait MP4 (1080x1920) — no
// generative AI credits required.
package video

import (
	"context"
	"fmt"
	"image/color"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fogleman/gg"
	"golang.org/x/image/font/gofont/gobold"
)

const (
	SlideW    = 1080
	SlideH    = 1920
	CarouselW = 1080
	CarouselH = 1080
)

// ─── Types ────────────────────────────────────────────────────────────────────

// Slide is a single frame in the template video.
type Slide struct {
	Text        string  // main headline text
	SubText     string  // smaller supporting text (hashtags, CTA sub-line)
	Duration    float64 // seconds this slide stays on screen
	BgHex       string  // background hex colour without "#", e.g. "0d1117"
	BgVideoPath string  // optional: path to a downloaded stock footage clip
	FontSize    float64 // 0 → auto-sized based on text length
}

// CarouselSlide is a single designed image for a carousel post.
type CarouselSlide struct {
	Title      string // large headline (hook or point)
	Body       string // supporting copy (optional)
	SlideNum   int    // 1-based slide number
	TotalSlides int
	BgHex      string
}

// TemplateVideoSpec describes the full video to be assembled.
type TemplateVideoSpec struct {
	Slides     []Slide
	OutputPath string // where the final MP4 should be written
}

// ─── Font resolution ──────────────────────────────────────────────────────────

var (
	resolvedFont     string
	resolvedFontOnce sync.Once
)

// fontPath returns a usable TrueType font path.
// It checks common system locations, then falls back to writing the bundled
// Go bold font to a temp file so rendering always works.
func fontPath() string {
	resolvedFontOnce.Do(func() {
		candidates := []string{
			os.Getenv("FURCI_FONT_PATH"),
			// Alpine / Docker (ttf-dejavu package)
			"/usr/share/fonts/dejavu/DejaVuSans-Bold.ttf",
			// Debian/Ubuntu
			"/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
			"/usr/share/fonts/truetype/liberation/LiberationSans-Bold.ttf",
			// macOS
			"/System/Library/Fonts/Helvetica.ttc",
			// Windows
			"C:/Windows/Fonts/arialbd.ttf",
		}
		for _, p := range candidates {
			if p == "" {
				continue
			}
			if _, err := os.Stat(p); err == nil {
				resolvedFont = p
				log.Printf("[Video] Using system font: %s", p)
				return
			}
		}
		// Write the embedded Go bold font to a temp file as a universal fallback
		f, err := os.CreateTemp("", "furci-gobold-*.ttf")
		if err != nil {
			log.Printf("[Video] Could not create temp font file: %v", err)
			return
		}
		if _, err := f.Write(gobold.TTF); err != nil {
			log.Printf("[Video] Could not write temp font file: %v", err)
			f.Close()
			return
		}
		f.Close()
		resolvedFont = f.Name()
		log.Printf("[Video] Using bundled Go bold font at %s", resolvedFont)
	})
	return resolvedFont
}

// ─── Palette ─────────────────────────────────────────────────────────────────

// slideBackground returns a background colour for a given slide index.
// The palette cycles through dark professional tones.
var slideBg = []string{
	"0d1117", // near-black  (hook)
	"0f172a", // dark navy   (body 1)
	"1e1b4b", // deep indigo (body 2)
	"14243b", // dark slate  (body 3)
	"1a1a2e", // dark purple (body 4)
	"0d1117", // near-black  (CTA)
}

// accentColour is the thin top bar that gives slides a consistent brand feel
var accentRGB = [3]uint8{99, 102, 241} // indigo-500

// BgForIndex picks a background hex for slide i.
func BgForIndex(i int) string {
	return slideBg[i%len(slideBg)]
}

// ─── Entry point ─────────────────────────────────────────────────────────────

// BuildTemplateVideo renders all slides to PNG, converts each to a video clip,
// concatenates them, and writes the final MP4 to spec.OutputPath.
func BuildTemplateVideo(ctx context.Context, spec TemplateVideoSpec) error {
	tmpDir := filepath.Dir(spec.OutputPath)

	font := fontPath()
	if font == "" {
		return fmt.Errorf("no usable font found — set FURCI_FONT_PATH env var or install ttf-dejavu")
	}

	var clipPaths []string
	for i, slide := range spec.Slides {
		clipPath := filepath.Join(tmpDir, fmt.Sprintf("clip_%d.mp4", i))

		// Try video background; if it fails clear the path so we fall through to PNG.
		if slide.BgVideoPath != "" {
			if err := buildVideoSlide(ctx, slide, font, i, tmpDir, clipPath); err != nil {
				log.Printf("[Video] Video slide %d failed, falling back to solid bg: %v", i, err)
				slide.BgVideoPath = ""
			}
		}

		// Solid-colour PNG path — used when there is no stock clip (or it failed).
		if slide.BgVideoPath == "" {
			pngPath := filepath.Join(tmpDir, fmt.Sprintf("slide_%d.png", i))
			if err := renderSlide(slide, font, pngPath); err != nil {
				return fmt.Errorf("render slide %d: %w", i, err)
			}
			if err := pngToClip(ctx, pngPath, slide.Duration, clipPath); err != nil {
				return fmt.Errorf("encode clip %d: %w", i, err)
			}
			os.Remove(pngPath)
		}

		clipPaths = append(clipPaths, clipPath)
	}

	if err := concatClips(ctx, clipPaths, spec.OutputPath); err != nil {
		return fmt.Errorf("concat clips: %w", err)
	}

	for _, p := range clipPaths {
		os.Remove(p)
	}
	return nil
}

// ─── Slide rendering ─────────────────────────────────────────────────────────

func renderSlide(s Slide, font, outputPath string) error {
	dc := gg.NewContext(SlideW, SlideH)

	// ── Background ────────────────────────────────────────────────────────
	dc.SetColor(parseHex(s.BgHex, color.RGBA{13, 17, 23, 255}))
	dc.Clear()

	// ── Subtle bottom gradient (improves text legibility) ─────────────────
	for y := SlideH / 2; y < SlideH; y++ {
		alpha := float64(y-SlideH/2) / float64(SlideH/2) * 0.5
		dc.SetRGBA(0, 0, 0, alpha)
		dc.DrawLine(0, float64(y), float64(SlideW), float64(y))
		dc.Stroke()
	}

	// ── Top accent bar ────────────────────────────────────────────────────
	dc.SetColor(color.RGBA{accentRGB[0], accentRGB[1], accentRGB[2], 255})
	dc.DrawRectangle(0, 0, float64(SlideW), 10)
	dc.Fill()

	// ── Main text ─────────────────────────────────────────────────────────
	if s.Text != "" {
		fontSize := s.FontSize
		if fontSize == 0 {
			fontSize = autoFontSize(s.Text)
		}
		if err := dc.LoadFontFace(font, fontSize); err != nil {
			return fmt.Errorf("load font face (%.0fpx): %w", fontSize, err)
		}
		dc.SetColor(color.White)
		yCenter := float64(SlideH)/2 - 40
		if s.SubText != "" {
			yCenter = float64(SlideH)/2 - 80
		}
		dc.DrawStringWrapped(
			s.Text,
			float64(SlideW)/2, yCenter,
			0.5, 0.5,
			float64(SlideW)*0.85,
			1.5,
			gg.AlignCenter,
		)
	}

	// ── Sub text (hashtags / supporting line) ─────────────────────────────
	if s.SubText != "" {
		if err := dc.LoadFontFace(font, 40); err == nil {
			dc.SetRGBA(1, 1, 1, 0.65)
			dc.DrawStringWrapped(
				s.SubText,
				float64(SlideW)/2, float64(SlideH)*0.70,
				0.5, 0.5,
				float64(SlideW)*0.80,
				1.4,
				gg.AlignCenter,
			)
		}
	}

	// ── Brand watermark ───────────────────────────────────────────────────
	if err := dc.LoadFontFace(font, 28); err == nil {
		dc.SetRGBA(1, 1, 1, 0.30)
		dc.DrawStringAnchored("furciai.com", float64(SlideW)/2, float64(SlideH)-50, 0.5, 0.5)
	}

	return dc.SavePNG(outputPath)
}

// autoFontSize picks a font size that fits the text on one "screen worth" of
// space. Longer text = smaller font.
func autoFontSize(text string) float64 {
	switch {
	case len(text) <= 40:
		return 84
	case len(text) <= 80:
		return 72
	case len(text) <= 140:
		return 60
	case len(text) <= 220:
		return 50
	default:
		return 42
	}
}

// ─── FFmpeg operations ────────────────────────────────────────────────────────

// pngToClip converts a static PNG image to a video clip of the given duration.
func pngToClip(ctx context.Context, imgPath string, duration float64, outputPath string) error {
	args := []string{
		"-y",
		"-loop", "1",
		"-i", imgPath,
		"-t", fmt.Sprintf("%.2f", duration),
		"-r", "30",
		"-vf", "format=yuv420p",
		"-c:v", "libx264",
		"-preset", "fast",
		"-tune", "stillimage",
		outputPath,
	}
	return runFFmpeg(ctx, args)
}

// concatClips joins multiple MP4 clips into one seamless video.
func concatClips(ctx context.Context, clips []string, outputPath string) error {
	tmpDir := filepath.Dir(outputPath)
	concatFile := filepath.Join(tmpDir, "concat.txt")

	var sb strings.Builder
	for _, clip := range clips {
		// Use absolute paths and escape single quotes
		safe := strings.ReplaceAll(clip, "'", "'\\''")
		sb.WriteString(fmt.Sprintf("file '%s'\n", safe))
	}
	if err := os.WriteFile(concatFile, []byte(sb.String()), 0644); err != nil {
		return err
	}
	defer os.Remove(concatFile)

	args := []string{
		"-y",
		"-f", "concat",
		"-safe", "0",
		"-i", concatFile,
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		"-movflags", "+faststart",
		outputPath,
	}
	return runFFmpeg(ctx, args)
}

func runFFmpeg(ctx context.Context, args []string) error {
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg error: %w\noutput:\n%s", err, string(out))
	}
	return nil
}

// ─── Video background slide ───────────────────────────────────────────────────

// buildVideoSlide overlays text on a stock footage clip using FFmpeg's
// drawtext filter. Text is written to temp files to avoid filter-graph
// escaping problems with special characters.
func buildVideoSlide(ctx context.Context, s Slide, font string, index int, tmpDir, outputPath string) error {
	fontSize := s.FontSize
	if fontSize == 0 {
		fontSize = autoFontSize(s.Text)
	}

	// Pre-wrap text so lines aren't too wide on screen
	maxChars := 20
	if fontSize < 60 {
		maxChars = 28
	}
	mainText := wrapText(s.Text, maxChars)

	// Write text to temp files — sidesteps all FFmpeg filter escaping
	mainFile := filepath.Join(tmpDir, fmt.Sprintf("vtxt_main_%d.txt", index))
	if err := os.WriteFile(mainFile, []byte(mainText), 0644); err != nil {
		return err
	}
	defer os.Remove(mainFile)

	wmFile := filepath.Join(tmpDir, fmt.Sprintf("vtxt_wm_%d.txt", index))
	os.WriteFile(wmFile, []byte("furciai.com"), 0644)
	defer os.Remove(wmFile)

	// Escape the font path for FFmpeg filter (colons must be \:)
	escapedFont := strings.ReplaceAll(font, ":", "\\:")
	escapedMain := strings.ReplaceAll(mainFile, ":", "\\:")
	escapedWM := strings.ReplaceAll(wmFile, ":", "\\:")

	filters := []string{
		// Crop + scale to portrait
		"scale=1080:1920:force_original_aspect_ratio=increase",
		"crop=1080:1920",
		// Darken footage so white text is readable
		"eq=brightness=-0.30:saturation=0.75",
		// Semi-transparent dark band in the middle for extra contrast
		"drawbox=x=0:y=700:w=iw:h=520:color=black@0.45:t=fill",
		// Indigo accent bar at top
		"drawbox=x=0:y=0:w=iw:h=10:color=0x6366f1@1.0:t=fill",
		// Main text
		fmt.Sprintf(
			"drawtext=fontfile=%s:textfile=%s:fontsize=%.0f:fontcolor=white:"+
				"x=(w-tw)/2:y=(h-th)/2-30:line_spacing=18:"+
				"shadowcolor=black@0.8:shadowx=3:shadowy=3",
			escapedFont, escapedMain, fontSize,
		),
		// Brand watermark
		fmt.Sprintf(
			"drawtext=fontfile=%s:textfile=%s:fontsize=28:fontcolor=white@0.30:"+
				"x=(w-tw)/2:y=h-50",
			escapedFont, escapedWM,
		),
	}

	// Optional subtext
	if s.SubText != "" {
		subFile := filepath.Join(tmpDir, fmt.Sprintf("vtxt_sub_%d.txt", index))
		os.WriteFile(subFile, []byte(s.SubText), 0644)
		defer os.Remove(subFile)
		escapedSub := strings.ReplaceAll(subFile, ":", "\\:")
		filters = append(filters,
			fmt.Sprintf(
				"drawtext=fontfile=%s:textfile=%s:fontsize=38:fontcolor=white@0.70:"+
					"x=(w-tw)/2:y=h*0.72:line_spacing=14:"+
					"shadowcolor=black@0.6:shadowx=2:shadowy=2",
				escapedFont, escapedSub,
			),
		)
	}

	args := []string{
		"-y",
		"-i", s.BgVideoPath,
		"-t", fmt.Sprintf("%.2f", s.Duration),
		"-vf", strings.Join(filters, ","),
		"-r", "30",
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		"-preset", "fast",
		outputPath,
	}
	return runFFmpeg(ctx, args)
}

// wrapText breaks text into lines of at most maxChars characters,
// splitting at word boundaries.
func wrapText(text string, maxChars int) string {
	words := strings.Fields(text)
	var lines []string
	var current strings.Builder
	for _, word := range words {
		if current.Len() > 0 && current.Len()+1+len(word) > maxChars {
			lines = append(lines, current.String())
			current.Reset()
		}
		if current.Len() > 0 {
			current.WriteByte(' ')
		}
		current.WriteString(word)
	}
	if current.Len() > 0 {
		lines = append(lines, current.String())
	}
	return strings.Join(lines, "\n")
}

// ─── Carousel slide rendering ─────────────────────────────────────────────────

// RenderCarouselSlide renders a single 1080×1080 PNG carousel slide using gg.
// Returns the path to the saved PNG.
func RenderCarouselSlide(s CarouselSlide, outputPath string) error {
	font := fontPath()
	if font == "" {
		return fmt.Errorf("no font available")
	}

	dc := gg.NewContext(CarouselW, CarouselH)

	// Background
	dc.SetColor(parseHex(s.BgHex, color.RGBA{13, 17, 23, 255}))
	dc.Clear()

	// Gradient overlay — bottom quarter darkens slightly
	for y := CarouselH * 3 / 4; y < CarouselH; y++ {
		alpha := float64(y-CarouselH*3/4) / float64(CarouselH/4) * 0.4
		dc.SetRGBA(0, 0, 0, alpha)
		dc.DrawLine(0, float64(y), float64(CarouselW), float64(y))
		dc.Stroke()
	}

	// Top accent bar
	dc.SetColor(color.RGBA{99, 102, 241, 255})
	dc.DrawRectangle(0, 0, float64(CarouselW), 8)
	dc.Fill()

	// Slide counter  (e.g. "2 / 5")
	if s.TotalSlides > 0 {
		if err := dc.LoadFontFace(font, 28); err == nil {
			counter := fmt.Sprintf("%d / %d", s.SlideNum, s.TotalSlides)
			dc.SetRGBA(1, 1, 1, 0.45)
			dc.DrawStringAnchored(counter, float64(CarouselW)-40, 40, 1, 0.5)
		}
	}

	// Title text
	if s.Title != "" {
		fontSize := carouselFontSize(s.Title, s.Body != "")
		if err := dc.LoadFontFace(font, fontSize); err == nil {
			dc.SetColor(color.White)
			yPos := float64(CarouselH) / 2
			if s.Body != "" {
				yPos = float64(CarouselH) * 0.38
			}
			dc.DrawStringWrapped(
				s.Title,
				float64(CarouselW)/2, yPos,
				0.5, 0.5,
				float64(CarouselW)*0.85,
				1.45,
				gg.AlignCenter,
			)
		}
	}

	// Body text (supporting copy)
	if s.Body != "" {
		if err := dc.LoadFontFace(font, 38); err == nil {
			dc.SetRGBA(1, 1, 1, 0.75)
			dc.DrawStringWrapped(
				s.Body,
				float64(CarouselW)/2, float64(CarouselH)*0.70,
				0.5, 0.5,
				float64(CarouselW)*0.80,
				1.4,
				gg.AlignCenter,
			)
		}
	}

	// Brand watermark
	if err := dc.LoadFontFace(font, 24); err == nil {
		dc.SetRGBA(1, 1, 1, 0.28)
		dc.DrawStringAnchored("furciai.com", float64(CarouselW)/2, float64(CarouselH)-30, 0.5, 0.5)
	}

	return dc.SavePNG(outputPath)
}

func carouselFontSize(title string, hasBody bool) float64 {
	base := 72.0
	if hasBody {
		base = 62.0
	}
	switch {
	case len(title) <= 30:
		return base
	case len(title) <= 60:
		return base - 10
	case len(title) <= 100:
		return base - 20
	default:
		return base - 28
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// parseHex converts a 6-digit hex string (with or without #) to color.RGBA.
func parseHex(hex string, fallback color.RGBA) color.RGBA {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return fallback
	}
	var r, g, b uint8
	fmt.Sscanf(hex[0:2], "%x", &r)
	fmt.Sscanf(hex[2:4], "%x", &g)
	fmt.Sscanf(hex[4:6], "%x", &b)
	return color.RGBA{R: r, G: g, B: b, A: 255}
}
