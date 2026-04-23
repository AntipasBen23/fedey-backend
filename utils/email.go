package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// SendEmail sends an HTML email via the Resend API.
// Requires RESEND_API_KEY and optionally RESEND_FROM.
func SendEmail(to, subject, htmlBody string) error {
	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey == "" {
		log.Printf("[Email] RESEND_API_KEY not set — would have sent to %s: %s", to, subject)
		return nil // non-fatal in dev
	}

	from := os.Getenv("RESEND_FROM")
	if from == "" {
		from = "Furci.ai <onboarding@resend.dev>"
	}

	payload := map[string]interface{}{
		"from":    from,
		"to":      []string{to},
		"subject": subject,
		"html":    htmlBody,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Resend API error %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
