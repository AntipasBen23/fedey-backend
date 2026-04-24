package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/AntipasBen23/fedey-backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// ─── httpOnly cookie helpers ──────────────────────────────────────────────────

// cookieAttrs returns security attributes for auth cookies.
// Production: SameSite=Lax; Secure — api.furciai.com and furciai.com share the same
// registered domain so cookies are same-site and SameSite=Lax is sufficient.
// Development (localhost): SameSite=Lax over plain HTTP (no Secure flag).
func cookieAttrs() string {
	isProduction := os.Getenv("GIN_MODE") == "release" ||
		os.Getenv("RAILWAY_ENVIRONMENT") != "" ||
		os.Getenv("COOKIE_SECURE") == "true"
	if isProduction {
		return "; HttpOnly; Secure; SameSite=Lax"
	}
	return "; HttpOnly; SameSite=Lax"
}

// setAuthCookies writes the access + refresh tokens as httpOnly cookies on the response.
func setAuthCookies(c *gin.Context, access, refresh string, refreshExp time.Time) {
	attrs := cookieAttrs()
	maxAgeRefresh := int(time.Until(refreshExp).Seconds())
	c.Writer.Header().Add("Set-Cookie", fmt.Sprintf("furci_access=%s; Path=/; Max-Age=3600%s", access, attrs))
	// Max-Age matches the refresh token TTL so the cookie lives exactly as long as the token
	c.Writer.Header().Add("Set-Cookie", fmt.Sprintf("furci_refresh=%s; Path=/; Max-Age=%d%s", refresh, maxAgeRefresh, attrs))
}

// clearAuthCookies expires both auth cookies immediately, logging the user out browser-side.
func clearAuthCookies(c *gin.Context) {
	attrs := cookieAttrs()
	c.Writer.Header().Add("Set-Cookie", fmt.Sprintf("furci_access=; Path=/; Max-Age=0%s", attrs))
	c.Writer.Header().Add("Set-Cookie", fmt.Sprintf("furci_refresh=; Path=/; Max-Age=0%s", attrs))
}

// userJSON returns the standard user payload for auth responses.
func userJSON(u models.User) gin.H {
	return gin.H{
		"id":                 u.ID,
		"name":               u.Name,
		"username":           u.Username,
		"email":              u.Email,
		"plan":               u.Plan,
		"jobDescription":     u.JobDescription,
		"platformContext":    u.PlatformContext,
		"lastOnboardingStep": u.LastOnboardingStep,
	}
}

// ─── Password validation ──────────────────────────────────────────────────────

type passwordError struct{ msg string }

func (e passwordError) Error() string { return e.msg }

func validatePassword(pw string) error {
	if len(pw) < 8 {
		return passwordError{"Password must be at least 8 characters."}
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range pw {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case strings.ContainsRune("!@#$%^&*()_+-=[]{}|;':\",./<>?", r):
			hasSpecial = true
		}
	}
	if !hasUpper {
		return passwordError{"Password must contain at least one uppercase letter."}
	}
	if !hasLower {
		return passwordError{"Password must contain at least one lowercase letter."}
	}
	if !hasDigit {
		return passwordError{"Password must contain at least one number."}
	}
	if !hasSpecial {
		return passwordError{"Password must contain at least one special character (!@#$%^&* etc.)."}
	}
	return nil
}

// ─── JWT helpers ──────────────────────────────────────────────────────────────

func jwtSecret() []byte {
	return []byte(os.Getenv("JWT_SECRET"))
}

func refreshSecret() []byte {
	return []byte(os.Getenv("JWT_REFRESH_SECRET"))
}

func issueAccessToken(user models.User) (string, error) {
	claims := jwt.MapClaims{
		"sub":   user.ID,
		"email": user.Email,
		"name":  user.Name,
		"plan":  user.Plan,
		"type":  "access",
		"exp":   time.Now().Add(15 * time.Minute).Unix(),
		"iat":   time.Now().Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(jwtSecret())
}

func issueRefreshToken(userID uint, rememberMe bool) (string, time.Time, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, err
	}
	token := hex.EncodeToString(raw)

	ttl := 30 * 24 * time.Hour
	if rememberMe {
		ttl = 90 * 24 * time.Hour
	}
	exp := time.Now().Add(ttl)

	rt := models.RefreshToken{UserID: userID, Token: token, ExpiresAt: exp}
	if err := database.DB.Create(&rt).Error; err != nil {
		return "", time.Time{}, err
	}
	return token, exp, nil
}

func sendVerificationEmail(email, name, code string) {
	body := fmt.Sprintf(`
<div style="font-family:sans-serif;max-width:480px;margin:auto;padding:2rem">
  <h2 style="color:#4f46e5">Welcome to Furci.ai, %s!</h2>
  <p>Your email verification code is:</p>
  <div style="font-size:2.5rem;font-weight:900;letter-spacing:0.25em;color:#111;background:#f3f4f6;border-radius:12px;padding:1rem;text-align:center">%s</div>
  <p style="color:#6b7280;font-size:0.9rem">This code expires in 15 minutes. If you didn't sign up for Furci.ai, you can safely ignore this email.</p>
</div>`, name, code)
	if err := utils.SendEmail(email, "Your Furci.ai verification code", body); err != nil {
		log.Printf("[Email] Failed to send verification to %s: %v", email, err)
	}
}

func sendPasswordResetEmail(email, name, token string) {
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "https://furci.ai"
	}
	link := frontendURL + "/reset-password?token=" + token
	body := fmt.Sprintf(`
<div style="font-family:sans-serif;max-width:480px;margin:auto;padding:2rem">
  <h2 style="color:#4f46e5">Reset your Furci.ai password</h2>
  <p>Hi %s, we received a request to reset your password.</p>
  <a href="%s" style="display:inline-block;background:#4f46e5;color:#fff;padding:0.75rem 2rem;border-radius:8px;text-decoration:none;font-weight:700;margin:1rem 0">Reset Password</a>
  <p style="color:#6b7280;font-size:0.9rem">This link expires in 1 hour. If you didn't request this, you can safely ignore this email.</p>
</div>`, name, link)
	if err := utils.SendEmail(email, "Reset your Furci.ai password", body); err != nil {
		log.Printf("[Email] Failed to send password reset to %s: %v", email, err)
	}
}

// ─── Rate limiting (in-memory, per email / per user ID) ──────────────────────

var (
	loginAttemptsMu sync.Mutex
	loginAttempts   = map[string]int{}
	loginLockedAt   = map[string]time.Time{}

	// Email verify: max 10 wrong attempts per userID per 15 minutes
	verifyAttemptsMu sync.Mutex
	verifyAttempts   = map[uint]int{}
	verifyLockedAt   = map[uint]time.Time{}

	// Forgot password: max 3 requests per email per hour
	forgotAttemptsMu sync.Mutex
	forgotAttempts   = map[string]int{}
	forgotLockedAt   = map[string]time.Time{}
)

func checkVerifyRateLimit(userID uint) error {
	verifyAttemptsMu.Lock()
	defer verifyAttemptsMu.Unlock()
	if lockedAt, ok := verifyLockedAt[userID]; ok {
		if time.Since(lockedAt) < 15*time.Minute {
			return fmt.Errorf("too many incorrect attempts. Please request a new code.")
		}
		delete(verifyLockedAt, userID)
		delete(verifyAttempts, userID)
	}
	return nil
}

func recordFailedVerify(userID uint) {
	verifyAttemptsMu.Lock()
	defer verifyAttemptsMu.Unlock()
	verifyAttempts[userID]++
	if verifyAttempts[userID] >= 10 {
		verifyLockedAt[userID] = time.Now()
		delete(verifyAttempts, userID)
	}
}

func checkForgotRateLimit(email string) error {
	forgotAttemptsMu.Lock()
	defer forgotAttemptsMu.Unlock()
	if lockedAt, ok := forgotLockedAt[email]; ok {
		if time.Since(lockedAt) < time.Hour {
			return fmt.Errorf("too many reset requests. Please try again in an hour.")
		}
		delete(forgotLockedAt, email)
		delete(forgotAttempts, email)
	}
	forgotAttempts[email]++
	if forgotAttempts[email] >= 3 {
		forgotLockedAt[email] = time.Now()
		delete(forgotAttempts, email)
	}
	return nil
}

const maxLoginAttempts = 5
const lockoutDuration = 10 * time.Minute

func checkLoginRateLimit(email string) error {
	loginAttemptsMu.Lock()
	defer loginAttemptsMu.Unlock()

	if lockedAt, ok := loginLockedAt[email]; ok {
		if time.Since(lockedAt) < lockoutDuration {
			remaining := lockoutDuration - time.Since(lockedAt)
			return fmt.Errorf("too many failed attempts. Please try again in %d minutes.", int(remaining.Minutes())+1)
		}
		delete(loginLockedAt, email)
		delete(loginAttempts, email)
	}
	return nil
}

func recordFailedLogin(email string) {
	loginAttemptsMu.Lock()
	defer loginAttemptsMu.Unlock()
	loginAttempts[email]++
	if loginAttempts[email] >= maxLoginAttempts {
		loginLockedAt[email] = time.Now()
		delete(loginAttempts, email)
	}
}

func clearLoginAttempts(email string) {
	loginAttemptsMu.Lock()
	defer loginAttemptsMu.Unlock()
	delete(loginAttempts, email)
	delete(loginLockedAt, email)
}

// ─── POST /v1/user/register ───────────────────────────────────────────────────

type RegisterRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func RegisterHandler(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.APIError(c, http.StatusBadRequest, "MISSING_FIELDS", "Name, email, and password are required.")
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Name = strings.TrimSpace(req.Name)

	// Validate email format
	if !regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`).MatchString(req.Email) {
		utils.APIError(c, http.StatusBadRequest, "VALIDATION_ERROR", "Please enter a valid email address.")
		return
	}

	// Validate password strength
	if err := validatePassword(req.Password); err != nil {
		utils.APIError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	// Check if email is taken — use Unscoped() to also find soft-deleted records,
	// which would otherwise cause a duplicate key error on INSERT.
	var existing models.User
	if database.DB.Unscoped().Where("email = ?", req.Email).First(&existing).Error == nil {
		if existing.DeletedAt.Valid {
			// Admin deleted this account — permanently erase the tombstone so the
			// email is freed and a fresh registration can proceed normally.
			database.DB.Unscoped().Delete(&existing)
			// Fall through to create a fresh account below.
		} else if !existing.IsVerified {
			resendVerificationCode(existing)
			// Return the same shape as a fresh registration so the UI can show the verify form.
			c.JSON(http.StatusCreated, gin.H{
				"message":           "Check your email for a 6-digit verification code.",
				"userId":            existing.ID,
				"needsVerification": true,
			})
			return
		} else {
			// Verified active account — return same shape as success (no enumeration).
			c.JSON(http.StatusOK, gin.H{"message": "Check your email for a 6-digit verification code."})
			return
		}
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		utils.APIError(c, http.StatusInternalServerError, "REGISTRATION_FAILED", "Registration failed. Please try again.")
		return
	}

	user := models.User{
		Name:         req.Name,
		Email:        req.Email,
		PasswordHash: string(hash),
		Plan:         "free",
	}
	if err := database.DB.Create(&user).Error; err != nil {
		utils.APIError(c, http.StatusInternalServerError, "REGISTRATION_FAILED", "Registration failed. Please try again.")
		return
	}

	// Send verification email
	resendVerificationCode(user)

	c.JSON(http.StatusCreated, gin.H{
		"message": "Account created! Check your email for a 6-digit verification code.",
		"userId":  user.ID,
	})
}

// ─── POST /v1/user/verify-email ───────────────────────────────────────────────

type VerifyEmailRequest struct {
	UserID uint   `json:"userId" binding:"required"`
	Code   string `json:"code" binding:"required"`
}

func VerifyEmailHandler(c *gin.Context) {
	var req VerifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.APIError(c, http.StatusBadRequest, "MISSING_FIELDS", "User ID and verification code are required.")
		return
	}

	if err := checkVerifyRateLimit(req.UserID); err != nil {
		utils.APIError(c, http.StatusTooManyRequests, "RATE_LIMITED", err.Error())
		return
	}

	var v models.EmailVerification
	err := database.DB.Where("user_id = ? AND code = ? AND used = false AND expires_at > ?", req.UserID, strings.TrimSpace(req.Code), time.Now()).
		Order("created_at desc").First(&v).Error
	if err != nil {
		recordFailedVerify(req.UserID)
		utils.APIError(c, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid or expired code. Request a new one.")
		return
	}

	database.DB.Model(&v).Update("used", true)
	database.DB.Model(&models.User{}).Where("id = ?", req.UserID).Update("is_verified", true)

	var user models.User
	database.DB.First(&user, req.UserID)

	access, err := issueAccessToken(user)
	if err != nil {
		utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", "Verification succeeded but login failed. Please log in manually.")
		return
	}
	refresh, exp, err := issueRefreshToken(user.ID, false)
	if err != nil {
		utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", "Verification succeeded but session failed. Please log in manually.")
		return
	}

	setAuthCookies(c, access, refresh, exp)
	c.JSON(http.StatusOK, gin.H{
		"message": "Email verified successfully. Welcome to Furci!",
		"user":    userJSON(user),
	})
}

// ─── POST /v1/user/resend-code ────────────────────────────────────────────────

type ResendCodeRequest struct {
	UserID uint `json:"userId" binding:"required"`
}

func ResendCodeHandler(c *gin.Context) {
	var req ResendCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.APIError(c, http.StatusBadRequest, "MISSING_FIELDS", "User ID is required.")
		return
	}

	var user models.User
	if database.DB.First(&user, req.UserID).Error != nil || user.IsVerified {
		// Return the same OK response whether the user doesn't exist or is already verified —
		// no information disclosed to the caller.
		c.JSON(http.StatusOK, gin.H{"message": "If there is a pending verification, a new code has been sent."})
		return
	}

	resendVerificationCode(user)
	c.JSON(http.StatusOK, gin.H{"message": "If there is a pending verification, a new code has been sent."})
}

func resendVerificationCode(user models.User) {
	// Invalidate old codes
	database.DB.Model(&models.EmailVerification{}).Where("user_id = ? AND used = false", user.ID).Update("used", true)

	// Generate 6-digit code
	n, _ := rand.Int(rand.Reader, big.NewInt(900000))
	code := fmt.Sprintf("%06d", n.Int64()+100000)

	v := models.EmailVerification{
		UserID:    user.ID,
		Code:      code,
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}
	database.DB.Create(&v)
	go sendVerificationEmail(user.Email, user.Name, code)
}

// ─── POST /v1/user/login ──────────────────────────────────────────────────────

type LoginRequest struct {
	Email      string `json:"email" binding:"required"`
	Password   string `json:"password" binding:"required"`
	RememberMe bool   `json:"rememberMe"`
}

func LoginHandler(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.APIError(c, http.StatusBadRequest, "MISSING_FIELDS", "Email and password are required.")
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	if err := checkLoginRateLimit(req.Email); err != nil {
		utils.APIError(c, http.StatusTooManyRequests, "RATE_LIMITED", err.Error())
		return
	}

	var user models.User
	if database.DB.Where("email = ?", req.Email).First(&user).Error != nil {
		recordFailedLogin(req.Email)
		utils.APIError(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password.")
		return
	}

	if user.PasswordHash == "" {
		// Don't reveal that this account uses Google — just fail generically.
		recordFailedLogin(req.Email)
		utils.APIError(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password.")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		recordFailedLogin(req.Email)
		utils.APIError(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password.")
		return
	}

	if !user.IsVerified {
		resendVerificationCode(user)
		c.JSON(http.StatusForbidden, gin.H{
			"error": gin.H{
				"code":    "UNVERIFIED_EMAIL",
				"message": "Your email isn't verified yet. We've sent a new code.",
			},
			"needsVerification": true,
			"userId":            user.ID,
		})
		return
	}

	clearLoginAttempts(req.Email)

	// Record last login time
	now := time.Now()
	database.DB.Model(&user).Update("last_login_at", now)

	access, err := issueAccessToken(user)
	if err != nil {
		utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", "Login failed. Please try again.")
		return
	}
	refresh, exp, err := issueRefreshToken(user.ID, req.RememberMe)
	if err != nil {
		utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", "Login failed. Please try again.")
		return
	}

	setAuthCookies(c, access, refresh, exp)
	c.JSON(http.StatusOK, gin.H{"user": userJSON(user)})
}

// ─── POST /v1/user/refresh ────────────────────────────────────────────────────

type RefreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

func RefreshTokenHandler(c *gin.Context) {
	// Read refresh token from httpOnly cookie (primary) or JSON body (fallback for older clients)
	refreshTokenStr, _ := c.Cookie("furci_refresh")
	if refreshTokenStr == "" {
		var req RefreshRequest
		if c.ShouldBindJSON(&req) == nil {
			refreshTokenStr = req.RefreshToken
		}
	}
	if refreshTokenStr == "" {
		utils.APIError(c, http.StatusUnauthorized, "SESSION_EXPIRED", "Session expired. Please log in again.")
		return
	}

	var rt models.RefreshToken
	if database.DB.Where("token = ? AND expires_at > ?", refreshTokenStr, time.Now()).First(&rt).Error != nil {
		clearAuthCookies(c)
		utils.APIError(c, http.StatusUnauthorized, "SESSION_EXPIRED", "Session expired. Please log in again.")
		return
	}

	var user models.User
	if database.DB.First(&user, rt.UserID).Error != nil {
		clearAuthCookies(c)
		utils.APIError(c, http.StatusUnauthorized, "SESSION_EXPIRED", "Session expired. Please log in again.")
		return
	}

	// Rotate refresh token
	database.DB.Delete(&rt)
	rememberMe := time.Until(rt.ExpiresAt) > 8*24*time.Hour // was a 30-day token
	newRefresh, exp, err := issueRefreshToken(user.ID, rememberMe)
	if err != nil {
		utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", "Session renewal failed.")
		return
	}

	access, err := issueAccessToken(user)
	if err != nil {
		utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", "Session renewal failed.")
		return
	}

	setAuthCookies(c, access, newRefresh, exp)
	c.JSON(http.StatusOK, gin.H{"user": userJSON(user)})
}

// ─── POST /v1/user/google ─────────────────────────────────────────────────────

type GoogleAuthRequest struct {
	IDToken    string `json:"idToken" binding:"required"`
	RememberMe bool   `json:"rememberMe"`
}

func GoogleAuthHandler(c *gin.Context) {
	var req GoogleAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.APIError(c, http.StatusBadRequest, "MISSING_FIELDS", "Google ID token is required.")
		return
	}

	// Verify the Google ID token with Google's tokeninfo endpoint
	resp, err := http.Get("https://oauth2.googleapis.com/tokeninfo?id_token=" + req.IDToken)
	if err != nil || resp.StatusCode != http.StatusOK {
		utils.APIError(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Google sign-in failed. Please try again.")
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var gData struct {
		Sub           string `json:"sub"`   // Google user ID
		Email         string `json:"email"`
		Name          string `json:"name"`
		EmailVerified string `json:"email_verified"`
		Aud           string `json:"aud"` // must match our client ID
	}
	if err := json.Unmarshal(body, &gData); err != nil || gData.Email == "" {
		utils.APIError(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Could not verify Google account.")
		return
	}

	// Optionally verify audience
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	if clientID != "" && gData.Aud != clientID {
		utils.APIError(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Google sign-in client mismatch.")
		return
	}

	gData.Email = strings.ToLower(strings.TrimSpace(gData.Email))

	// Find or create user — also check soft-deleted records to avoid duplicate-key errors
	var user models.User
	result := database.DB.Where("email = ?", gData.Email).First(&user)
	if result.Error != nil {
		// If soft-deleted, restore as a fresh user — wipe onboarding so they start from scratch
		var deleted models.User
		if database.DB.Unscoped().Where("email = ?", gData.Email).First(&deleted).Error == nil {
			if err := database.DB.Unscoped().Model(&deleted).Updates(map[string]interface{}{
				"deleted_at":           nil,
				"google_id":            gData.Sub,
				"is_verified":          true,
				"name":                 gData.Name,
				"plan":                 "free",
				"last_onboarding_step": "",
				"job_description":      "",
				"platform_context":     "",
			}).Error; err != nil {
				utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", "Sign-in failed. Please try again.")
				return
			}
			user = deleted
			user.LastOnboardingStep = ""
		} else {
			// Truly new user via Google
			user = models.User{
				Name:       gData.Name,
				Email:      gData.Email,
				GoogleID:   gData.Sub,
				IsVerified: true,
				Plan:       "free",
			}
			if err := database.DB.Create(&user).Error; err != nil {
				utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", "Sign-in failed. Please try again.")
				return
			}
		}
	} else if user.GoogleID == "" {
		// Existing email-password user — link Google to their account
		database.DB.Model(&user).Updates(map[string]interface{}{"google_id": gData.Sub, "is_verified": true})
	}

	// Update last login timestamp
	now := time.Now()
	database.DB.Model(&user).Update("last_login_at", now)
	user.LastLoginAt = &now

	access, err := issueAccessToken(user)
	if err != nil {
		utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", "Sign-in failed.")
		return
	}
	refresh, exp, err := issueRefreshToken(user.ID, req.RememberMe)
	if err != nil {
		utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", "Sign-in failed.")
		return
	}

	setAuthCookies(c, access, refresh, exp)
	c.JSON(http.StatusOK, gin.H{"user": userJSON(user)})
}

// ─── POST /v1/user/forgot-password ───────────────────────────────────────────

type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required"`
}

func ForgotPasswordHandler(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.APIError(c, http.StatusBadRequest, "MISSING_FIELDS", "Email is required.")
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	if err := checkForgotRateLimit(req.Email); err != nil {
		utils.APIError(c, http.StatusTooManyRequests, "RATE_LIMITED", err.Error())
		return
	}

	// Always respond OK to prevent email enumeration
	var user models.User
	if database.DB.Where("email = ?", req.Email).First(&user).Error != nil {
		c.JSON(http.StatusOK, gin.H{"message": "If that email is registered, you'll receive a reset link shortly."})
		return
	}

	// Invalidate previous tokens
	database.DB.Model(&models.PasswordResetToken{}).Where("user_id = ? AND used = false", user.ID).Update("used", true)

	// Generate secure random token
	raw := make([]byte, 32)
	rand.Read(raw)
	token := hex.EncodeToString(raw)

	prt := models.PasswordResetToken{
		UserID:    user.ID,
		Token:     token,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	database.DB.Create(&prt)
	go sendPasswordResetEmail(user.Email, user.Name, token)

	c.JSON(http.StatusOK, gin.H{"message": "If that email is registered, you'll receive a reset link shortly."})
}

// ─── POST /v1/user/reset-password ────────────────────────────────────────────

type ResetPasswordRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func ResetPasswordHandler(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.APIError(c, http.StatusBadRequest, "MISSING_FIELDS", "Token and new password are required.")
		return
	}

	if err := validatePassword(req.Password); err != nil {
		utils.APIError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	var prt models.PasswordResetToken
	if database.DB.Where("token = ? AND used = false AND expires_at > ?", req.Token, time.Now()).First(&prt).Error != nil {
		utils.APIError(c, http.StatusBadRequest, "SESSION_EXPIRED", "This reset link is invalid or has expired. Please request a new one.")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", "Password reset failed. Please try again.")
		return
	}

	database.DB.Model(&models.User{}).Where("id = ?", prt.UserID).Update("password_hash", string(hash))
	database.DB.Model(&prt).Update("used", true)
	// Revoke all refresh tokens for this user
	database.DB.Where("user_id = ?", prt.UserID).Delete(&models.RefreshToken{})

	c.JSON(http.StatusOK, gin.H{"message": "Password updated successfully. You can now log in with your new password."})
}

// ─── POST /v1/user/logout ─────────────────────────────────────────────────────

type LogoutRequest struct {
	RefreshToken string `json:"refreshToken"`
}

func LogoutHandler(c *gin.Context) {
	// Try cookie first (primary), fall back to JSON body (backwards compat)
	refreshTokenStr, _ := c.Cookie("furci_refresh")
	if refreshTokenStr == "" {
		var req LogoutRequest
		c.ShouldBindJSON(&req)
		refreshTokenStr = req.RefreshToken
	}
	if refreshTokenStr != "" {
		database.DB.Where("token = ?", refreshTokenStr).Delete(&models.RefreshToken{})
	}
	clearAuthCookies(c)
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully."})
}

// ─── GET /v1/user/me ──────────────────────────────────────────────────────────

func GetMeHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	var user models.User
	if database.DB.First(&user, userID).Error != nil {
		utils.APIError(c, http.StatusUnauthorized, "SESSION_EXPIRED", "Session expired. Please log in again.")
		return
	}
	c.JSON(http.StatusOK, userJSON(user))
}

// ─── PATCH /v1/user/username ─────────────────────────────────────────────────

type SetUsernameRequest struct {
	Username string `json:"username" binding:"required"`
}

func SetUsernameHandler(c *gin.Context) {
	userID, _ := c.Get("userID")
	var req SetUsernameRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Username) == "" {
		utils.APIError(c, http.StatusBadRequest, "MISSING_FIELDS", "Username is required.")
		return
	}
	username := strings.ToLower(strings.TrimSpace(req.Username))
	// validate: only alphanumeric + underscore, 3-20 chars
	matched, _ := regexp.MatchString(`^[a-z0-9_]{3,20}$`, username)
	if !matched {
		utils.APIError(c, http.StatusBadRequest, "VALIDATION_ERROR", "Username must be 3-20 characters, letters/numbers/underscores only.")
		return
	}
	// check uniqueness
	var existing models.User
	if database.DB.Where("username = ? AND id != ?", username, userID).First(&existing).Error == nil {
		utils.APIError(c, http.StatusConflict, "CONFLICT", "That username is already taken.")
		return
	}
	if err := database.DB.Model(&models.User{}).Where("id = ?", userID).Update("username", username).Error; err != nil {
		utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", "Failed to save username.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"username": username})
}

// ─── PATCH /v1/user/onboarding ────────────────────────────────────────────────

type UpdateOnboardingRequest struct {
	JobDescription     *string `json:"jobDescription"`
	PlatformContext    *string `json:"platformContext"`
	LastOnboardingStep *string `json:"lastOnboardingStep"`
}

func UpdateOnboardingHandler(c *gin.Context) {
	userID, _ := c.Get("userID")

	var req UpdateOnboardingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.APIError(c, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request.")
		return
	}

	updates := map[string]interface{}{}
	if req.JobDescription != nil {
		updates["job_description"] = *req.JobDescription
	}
	if req.PlatformContext != nil {
		updates["platform_context"] = *req.PlatformContext
	}
	if req.LastOnboardingStep != nil {
		updates["last_onboarding_step"] = *req.LastOnboardingStep
	}

	if len(updates) > 0 {
		database.DB.Model(&models.User{}).Where("id = ?", userID).Updates(updates)
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
