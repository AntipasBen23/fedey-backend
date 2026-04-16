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
	SlideW = 1080
	SlideH = 1920
)

// ─── Types ────────────────────────────────────────────────────────────────────

// Slide is a single frame in the template video.
type Slide struct {
	Text     string  // main headline text
	SubText  string  // smaller supporting text (hashtags, CTA sub-line)
	Duration float64 // seconds this slide stays on screen
	BgHex    string  // background hex colour without "#", e.g. "0d1117"
	FontSize float64 // 0 → auto-sized based on text length
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
		pngPath := filepath.Join(tmpDir, fmt.Sprintf("slide_%d.png", i))
		clipPath := filepath.Join(tmpDir, fmt.Sprintf("clip_%d.mp4", i))

		if err := renderSlide(slide, font, pngPath); err != nil {
			return fmt.Errorf("render slide %d: %w", i, err)
		}
		if err := pngToClip(ctx, pngPath, slide.Duration, clipPath); err != nil {
			return fmt.Errorf("encode clip %d: %w", i, err)
		}
		os.Remove(pngPath) // PNG no longer needed
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
