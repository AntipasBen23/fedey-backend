package handlers

import "testing"

// ─── Content type validation (pure logic) ─────────────────────────────────────

func isValidContentType(ct string) bool {
	valid := map[string]bool{
		"tweet":          true,
		"thread":         true,
		"carousel":       true,
		"video_script":   true,
		"linkedin_post":  true,
	}
	return valid[ct]
}

func TestIsValidContentType_Tweet(t *testing.T) {
	if !isValidContentType("tweet") {
		t.Error("tweet should be a valid content type")
	}
}

func TestIsValidContentType_Thread(t *testing.T) {
	if !isValidContentType("thread") {
		t.Error("thread should be a valid content type")
	}
}

func TestIsValidContentType_Carousel(t *testing.T) {
	if !isValidContentType("carousel") {
		t.Error("carousel should be a valid content type")
	}
}

func TestIsValidContentType_VideoScript(t *testing.T) {
	if !isValidContentType("video_script") {
		t.Error("video_script should be a valid content type")
	}
}

func TestIsValidContentType_LinkedInPost(t *testing.T) {
	if !isValidContentType("linkedin_post") {
		t.Error("linkedin_post should be a valid content type")
	}
}

func TestIsValidContentType_Unknown(t *testing.T) {
	if isValidContentType("story") {
		t.Error("unknown type should not be valid")
	}
}

func TestIsValidContentType_Empty(t *testing.T) {
	if isValidContentType("") {
		t.Error("empty string should not be valid")
	}
}

// ─── Platform selection validation ────────────────────────────────────────────

func isValidPlatform(platform string) bool {
	valid := map[string]bool{
		"twitter":   true,
		"x":         true,
		"linkedin":  true,
		"instagram": true,
	}
	return valid[platform]
}

func TestIsValidPlatform_Twitter(t *testing.T) {
	if !isValidPlatform("twitter") {
		t.Error("twitter should be valid")
	}
}

func TestIsValidPlatform_X(t *testing.T) {
	if !isValidPlatform("x") {
		t.Error("x should be valid")
	}
}

func TestIsValidPlatform_LinkedIn(t *testing.T) {
	if !isValidPlatform("linkedin") {
		t.Error("linkedin should be valid")
	}
}

func TestIsValidPlatform_Instagram(t *testing.T) {
	if !isValidPlatform("instagram") {
		t.Error("instagram should be valid")
	}
}

func TestIsValidPlatform_Facebook(t *testing.T) {
	if isValidPlatform("facebook") {
		t.Error("facebook is not a supported platform")
	}
}

func TestIsValidPlatform_Empty(t *testing.T) {
	if isValidPlatform("") {
		t.Error("empty platform should not be valid")
	}
}
