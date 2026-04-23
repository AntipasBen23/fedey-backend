package utils

import (
	"fmt"
)

// RepurposeContent takes content from one platform and refactors it for another
func RepurposeContent(originalContent, fromPlatform, toPlatform, niche string) (string, error) {
	prompt := fmt.Sprintf(`
		You are a Cross-Platform Content Strategist.
		Take this successful post from %s and refactor it for %s.
		
		ORIGINAL CONTENT:
		"%s"

		TARGET PLATFORM CONTEXT (%s):
		- If LinkedIn: Make it professional, use paragraphs, add a "key takeaway" section, and ensure it sounds like an industry leader.
		- If Twitter/X: Make it punchy, use short sentences, and aim for high engagement/shares.
		- If Instagram/Carousel: Break it down into 5-7 clear "Slides" (Text only).

		NICHE: %s

		Return ONLY the refactored content.
	`, fromPlatform, toPlatform, originalContent, toPlatform, niche)

	return GenerateAIResponse(prompt)
}
