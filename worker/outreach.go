package worker

import (
	"fmt"
	"log"
	"time"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/AntipasBen23/fedey-backend/utils"
)

// RunOnboardingOutreach scans for users who started onboarding but never finished,
// and sends them a single friendly re-engagement email. It will not re-send to the
// same user within 14 days.
func RunOnboardingOutreach() {
	if database.DB == nil {
		return
	}

	log.Println("[Outreach] Scanning for incomplete onboarding users...")

	cutoff := time.Now().Add(-48 * time.Hour) // must have registered at least 48h ago
	resendWindow := time.Now().Add(-14 * 24 * time.Hour)

	var users []models.User
	database.DB.
		Where("is_verified = true").
		Where("last_onboarding_step != '' AND last_onboarding_step != 'completed'").
		Where("created_at <= ?", cutoff).
		Find(&users)

	for _, user := range users {
		// Skip if we already sent them an outreach email in the last 14 days
		var recent int64
		database.DB.Model(&models.OnboardingOutreach{}).
			Where("user_id = ? AND sent_at >= ?", user.ID, resendWindow).
			Count(&recent)
		if recent > 0 {
			continue
		}

		if err := sendOutreachToUser(user); err != nil {
			log.Printf("[Outreach] Failed to send to %s: %v", user.Email, err)
		}
	}
}

// SendOutreachToUser sends a re-engagement email to a single user and logs it.
// Returns an error if the user is not eligible or the send fails.
func SendOutreachToUser(userID uint) error {
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		return fmt.Errorf("user not found")
	}
	if !user.IsVerified || user.LastOnboardingStep == "" || user.LastOnboardingStep == "completed" {
		return fmt.Errorf("user is not eligible for outreach")
	}
	return sendOutreachToUser(user)
}

func sendOutreachToUser(user models.User) error {
	body := buildOutreachEmail(user.Name)
	subject := "We noticed you started setting up your Furci AI account"

	if err := utils.SendEmail(user.Email, subject, body); err != nil {
		return err
	}

	database.DB.Create(&models.OnboardingOutreach{
		UserID:    user.ID,
		UserEmail: user.Email,
		UserName:  user.Name,
		StoppedAt: user.LastOnboardingStep,
		SentAt:    time.Now(),
	})

	log.Printf("[Outreach] Re-engagement email sent to %s (stopped at %s)", user.Email, user.LastOnboardingStep)
	return nil
}

func buildOutreachEmail(name string) string {
	firstName := name
	return fmt.Sprintf(`
<div style="font-family:Georgia,serif;max-width:560px;margin:auto;padding:2.5rem;background:#ffffff;color:#1a1a1a;line-height:1.8">
  <p style="margin:0 0 1.2rem">Hi %s,</p>

  <p style="margin:0 0 1.2rem">We noticed you started setting up your Furci AI account a couple of days ago but did not get a chance to finish.</p>

  <p style="margin:0 0 1.2rem">We want to make sure you have everything you need. If you ran into any issues during setup, or if something was unclear, we would love to hear from you. Just reply to this email and we will sort it out together.</p>

  <p style="margin:0 0 1.2rem">If life simply got in the way, no worries at all. Your account is still here waiting for you. You can pick up right where you left off anytime.</p>

  <p style="margin:0 0 2rem">
    <a href="https://furciai.com" style="display:inline-block;background:#093f67;color:#ffffff;padding:0.75rem 2rem;border-radius:6px;text-decoration:none;font-weight:600;font-size:0.95rem">Continue your setup</a>
  </p>

  <p style="margin:0 0 1.2rem">Looking forward to having you fully onboard.</p>

  <p style="margin:0">Warm regards,<br>The Furci AI Team</p>
</div>`, firstName)
}
