package handlers

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/AntipasBen23/fedey-backend/worker"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func TestDailyReportHandler(c *gin.Context) {
	go worker.SendDailyReports()
	c.JSON(http.StatusOK, gin.H{"message": "Daily report cycle triggered in background."})
}

func TestStrategistHandler(c *gin.Context) {
	go worker.RunStrategicOptimization()
	c.JSON(http.StatusOK, gin.H{"message": "Strategic optimization cycle triggered in background."})
}

// ─── Admin brute-force protection (mirrors user login rate limiting) ──────────

var (
	adminLoginMu       sync.Mutex
	adminLoginAttempts = map[string]int{}
	adminLoginLockedAt = map[string]time.Time{}
)

const adminMaxAttempts = 3              // stricter than user login
const adminLockoutDuration = 30 * time.Minute

func checkAdminLoginRateLimit(email string) error {
	adminLoginMu.Lock()
	defer adminLoginMu.Unlock()
	if lockedAt, ok := adminLoginLockedAt[email]; ok {
		if time.Since(lockedAt) < adminLockoutDuration {
			remaining := adminLockoutDuration - time.Since(lockedAt)
			return fmt.Errorf("too many failed attempts. Try again in %d minutes.", int(remaining.Minutes())+1)
		}
		delete(adminLoginLockedAt, email)
		delete(adminLoginAttempts, email)
	}
	return nil
}

func recordAdminFailedLogin(email string) {
	adminLoginMu.Lock()
	defer adminLoginMu.Unlock()
	adminLoginAttempts[email]++
	if adminLoginAttempts[email] >= adminMaxAttempts {
		adminLoginLockedAt[email] = time.Now()
		delete(adminLoginAttempts, email)
	}
}

func clearAdminLoginAttempts(email string) {
	adminLoginMu.Lock()
	defer adminLoginMu.Unlock()
	delete(adminLoginAttempts, email)
	delete(adminLoginLockedAt, email)
}

// ─── Admin JWT ────────────────────────────────────────────────────────────────

func adminSecret() []byte {
	return []byte(os.Getenv("ADMIN_JWT_SECRET"))
}

func issueAdminToken(userID uint, email string) (string, error) {
	claims := jwt.MapClaims{
		"sub":   userID,
		"email": email,
		"role":  "admin",
		"type":  "admin",
		"exp":   time.Now().Add(8 * time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(adminSecret())
}

// ─── POST /v1/admin/setup ────────────────────────────────────────────────────
// One-time endpoint to create the admin account. Locked behind ADMIN_SECRET.
// Once the account exists, this endpoint becomes a no-op (returns 409).

type AdminSetupRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
	Name     string `json:"name"`
}

func AdminSetupHandler(c *gin.Context) {
	var req AdminSetupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email and password are required."})
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Name == "" {
		req.Name = "Admin"
	}

	// If an admin already exists, block — setup is one-time only.
	var existing models.User
	if database.DB.Where("email = ?", req.Email).First(&existing).Error == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Admin account already exists. Use the login endpoint."})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Setup failed."})
		return
	}

	user := models.User{
		Name:         req.Name,
		Email:        req.Email,
		PasswordHash: string(hash),
		IsVerified:   true, // admin account is pre-verified
		Plan:         "pro",
	}
	if err := database.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create admin account."})
		return
	}

	token, _ := issueAdminToken(user.ID, user.Email)
	c.JSON(http.StatusCreated, gin.H{
		"message": "Admin account created successfully.",
		"token":   token,
		"admin":   gin.H{"id": user.ID, "name": user.Name, "email": user.Email},
	})
}

// ─── POST /v1/admin/login ────────────────────────────────────────────────────

type AdminLoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func AdminLoginHandler(c *gin.Context) {
	var req AdminLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email and password are required."})
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	if err := checkAdminLoginRateLimit(req.Email); err != nil {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if database.DB.Where("email = ?", req.Email).First(&user).Error != nil {
		recordAdminFailedLogin(req.Email)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials."})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		recordAdminFailedLogin(req.Email)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials."})
		return
	}

	clearAdminLoginAttempts(req.Email)

	token, err := issueAdminToken(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Login failed."})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"admin": gin.H{"id": user.ID, "name": user.Name, "email": user.Email},
	})
}

// ─── GET /v1/admin/stats ─────────────────────────────────────────────────────

func AdminStatsHandler(c *gin.Context) {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	weekStart := todayStart.AddDate(0, 0, -7)

	var totalUsers, newToday, newThisWeek, verifiedUsers, freeUsers, proUsers, googleUsers int64
	database.DB.Model(&models.User{}).Count(&totalUsers)
	database.DB.Model(&models.User{}).Where("created_at >= ?", todayStart).Count(&newToday)
	database.DB.Model(&models.User{}).Where("created_at >= ?", weekStart).Count(&newThisWeek)
	database.DB.Model(&models.User{}).Where("is_verified = true").Count(&verifiedUsers)
	database.DB.Model(&models.User{}).Where("plan = 'free'").Count(&freeUsers)
	database.DB.Model(&models.User{}).Where("plan = 'pro'").Count(&proUsers)
	database.DB.Model(&models.User{}).Where("google_id != ''").Count(&googleUsers)

	var totalViews, viewsToday, viewsThisWeek, uniqueSessions int64
	database.DB.Model(&models.PageView{}).Count(&totalViews)
	database.DB.Model(&models.PageView{}).Where("created_at >= ?", todayStart).Count(&viewsToday)
	database.DB.Model(&models.PageView{}).Where("created_at >= ?", weekStart).Count(&viewsThisWeek)
	database.DB.Model(&models.PageView{}).Where("created_at >= ?", weekStart).
		Distinct("session_id").Count(&uniqueSessions)

	var totalPosts, postsThisWeek int64
	database.DB.Model(&models.ScheduledPost{}).Count(&totalPosts)
	database.DB.Model(&models.ScheduledPost{}).Where("created_at >= ?", weekStart).Count(&postsThisWeek)

	var totalSyncs int64
	database.DB.Model(&models.PostAnalytics{}).Count(&totalSyncs)

	c.JSON(http.StatusOK, gin.H{
		"users": gin.H{
			"total":    totalUsers,
			"newToday": newToday,
			"newWeek":  newThisWeek,
			"verified": verifiedUsers,
			"free":     freeUsers,
			"pro":      proUsers,
			"google":   googleUsers,
		},
		"pageViews": gin.H{
			"total":          totalViews,
			"today":          viewsToday,
			"thisWeek":       viewsThisWeek,
			"uniqueSessions": uniqueSessions,
		},
		"content": gin.H{
			"totalPosts":    totalPosts,
			"postsThisWeek": postsThisWeek,
			"totalSyncs":    totalSyncs,
		},
	})
}

// ─── GET /v1/admin/users ─────────────────────────────────────────────────────

func AdminListUsersHandler(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit := 20
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit
	search := strings.TrimSpace(c.Query("search"))

	query := database.DB.Model(&models.User{})
	if search != "" {
		like := "%" + search + "%"
		query = query.Where("name ILIKE ? OR email ILIKE ?", like, like)
	}

	var total int64
	query.Count(&total)

	var users []models.User
	query.Order("created_at desc").Limit(limit).Offset(offset).Find(&users)

	type UserRow struct {
		ID           uint       `json:"id"`
		Name         string     `json:"name"`
		Email        string     `json:"email"`
		Plan         string     `json:"plan"`
		IsVerified   bool       `json:"isVerified"`
		SignupMethod string     `json:"signupMethod"`
		CreatedAt    time.Time  `json:"createdAt"`
		LastLoginAt  *time.Time `json:"lastLoginAt"`
	}

	rows := make([]UserRow, 0, len(users))
	for _, u := range users {
		method := "email"
		if u.GoogleID != "" {
			method = "google"
		}
		rows = append(rows, UserRow{
			ID:           u.ID,
			Name:         u.Name,
			Email:        u.Email,
			Plan:         u.Plan,
			IsVerified:   u.IsVerified,
			SignupMethod: method,
			CreatedAt:    u.CreatedAt,
			LastLoginAt:  u.LastLoginAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"users": rows,
		"total": total,
		"page":  page,
		"pages": (int(total) + limit - 1) / limit,
	})
}

// ─── DELETE /v1/admin/users/:id ──────────────────────────────────────────────

func AdminDeleteUserHandler(c *gin.Context) {
	id := c.Param("id")
	var user models.User
	if database.DB.First(&user, id).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found."})
		return
	}

	// Cascade: delete refresh tokens + verifications
	database.DB.Where("user_id = ?", user.ID).Delete(&models.RefreshToken{})
	database.DB.Where("user_id = ?", user.ID).Delete(&models.EmailVerification{})
	database.DB.Where("user_id = ?", user.ID).Delete(&models.PasswordResetToken{})

	// Hard-delete all user content
	database.DB.Unscoped().Where("user_id = ?", user.ID).Delete(&models.SocialAccount{})
	database.DB.Unscoped().Where("user_id = ?", user.ID).Delete(&models.ContentCalendar{})
	database.DB.Unscoped().Where("user_id = ?", user.ID).Delete(&models.ScheduledPost{})
	database.DB.Unscoped().Where("user_id = ?", user.ID).Delete(&models.UserStrategy{})
	database.DB.Unscoped().Where("user_id = ?", user.ID).Delete(&models.EngagementEvent{})
	database.DB.Unscoped().Where("user_id = ?", user.ID).Delete(&models.FollowerSnapshot{})
	database.DB.Unscoped().Where("user_id = ?", user.ID).Delete(&models.PostAnalytics{})

	database.DB.Delete(&user)

	c.JSON(http.StatusOK, gin.H{"message": "User deleted."})
}

// ─── PATCH /v1/admin/users/:id/plan ─────────────────────────────────────────

type UpdatePlanRequest struct {
	Plan string `json:"plan" binding:"required"`
}

func AdminUpdateUserPlanHandler(c *gin.Context) {
	id := c.Param("id")
	var req UpdatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil || (req.Plan != "free" && req.Plan != "pro") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Plan must be 'free' or 'pro'."})
		return
	}
	if database.DB.Model(&models.User{}).Where("id = ?", id).Update("plan", req.Plan).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Plan updated."})
}

// ─── GET /v1/admin/visitors/timeline ─────────────────────────────────────────

func AdminVisitorTimelineHandler(c *gin.Context) {
	// Last 30 days, one row per day
	type DayCount struct {
		Day   string `json:"day"`
		Views int    `json:"views"`
	}
	var rows []DayCount
	database.DB.Raw(`
		SELECT TO_CHAR(created_at, 'Mon DD') AS day, COUNT(*) AS views
		FROM page_views
		WHERE created_at >= NOW() - INTERVAL '30 days'
		GROUP BY TO_CHAR(created_at, 'Mon DD'), DATE_TRUNC('day', created_at)
		ORDER BY DATE_TRUNC('day', created_at) ASC
	`).Scan(&rows)
	c.JSON(http.StatusOK, gin.H{"timeline": rows})
}

// ─── GET /v1/admin/visitors/top-pages ────────────────────────────────────────

func AdminTopPagesHandler(c *gin.Context) {
	type PageCount struct {
		Path  string `json:"path"`
		Views int    `json:"views"`
	}
	var rows []PageCount
	database.DB.Raw(`
		SELECT path, COUNT(*) AS views
		FROM page_views
		WHERE created_at >= NOW() - INTERVAL '30 days'
		GROUP BY path
		ORDER BY views DESC
		LIMIT 15
	`).Scan(&rows)
	c.JSON(http.StatusOK, gin.H{"pages": rows})
}

// ─── GET /v1/admin/visitors/devices ──────────────────────────────────────────

func AdminDevicesHandler(c *gin.Context) {
	type DeviceCount struct {
		Device string `json:"device"`
		Count  int    `json:"count"`
	}
	var rows []DeviceCount
	database.DB.Raw(`
		SELECT device_type AS device, COUNT(*) AS count
		FROM page_views
		WHERE created_at >= NOW() - INTERVAL '30 days'
		GROUP BY device_type
		ORDER BY count DESC
	`).Scan(&rows)
	c.JSON(http.StatusOK, gin.H{"devices": rows})
}

// ─── GET /v1/admin/activity ───────────────────────────────────────────────────

func AdminActivityHandler(c *gin.Context) {
	// Recent 50 scheduled posts
	var posts []models.ScheduledPost
	database.DB.Order("created_at desc").Limit(50).Find(&posts)

	type ActivityItem struct {
		Type      string    `json:"type"`
		Detail    string    `json:"detail"`
		Platform  string    `json:"platform"`
		Status    string    `json:"status"`
		CreatedAt time.Time `json:"createdAt"`
	}

	items := make([]ActivityItem, 0, len(posts))
	for _, p := range posts {
		snippet := p.Content
		if len(snippet) > 80 {
			snippet = snippet[:80] + "…"
		}
		items = append(items, ActivityItem{
			Type:      "post",
			Detail:    snippet,
			Platform:  p.Platform,
			Status:    p.Status,
			CreatedAt: p.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{"activity": items})
}
