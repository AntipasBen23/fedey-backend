package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// ── Geo cache ─────────────────────────────────────────────────────────────────

type geoResult struct {
	Country string
	Region  string
	City    string
}

type geoCacheEntry struct {
	result    geoResult
	expiresAt time.Time
}

var (
	geoCache   = map[string]geoCacheEntry{}
	geoCacheMu sync.RWMutex
)

func lookupGeo(ip string) geoResult {
	if ip == "" || ip == "127.0.0.1" || ip == "::1" ||
		strings.HasPrefix(ip, "192.168.") || strings.HasPrefix(ip, "10.") {
		return geoResult{}
	}

	geoCacheMu.RLock()
	if entry, ok := geoCache[ip]; ok && time.Now().Before(entry.expiresAt) {
		geoCacheMu.RUnlock()
		return entry.result
	}
	geoCacheMu.RUnlock()

	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Get(fmt.Sprintf("https://ipwho.is/%s?fields=success,country,region,city", ip))
	if err != nil {
		return geoResult{}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var geo struct {
		Success bool   `json:"success"`
		Country string `json:"country"`
		Region  string `json:"region"`
		City    string `json:"city"`
	}
	if err := json.Unmarshal(body, &geo); err != nil || !geo.Success {
		return geoResult{}
	}

	result := geoResult{Country: geo.Country, Region: geo.Region, City: geo.City}

	geoCacheMu.Lock()
	if len(geoCache) >= 5000 {
		geoCache = map[string]geoCacheEntry{}
	}
	geoCache[ip] = geoCacheEntry{result: result, expiresAt: time.Now().Add(6 * time.Hour)}
	geoCacheMu.Unlock()

	return result
}

// ── IP extraction ─────────────────────────────────────────────────────────────

func clientIP(c *gin.Context) string {
	for _, header := range []string{"CF-Connecting-IP", "X-Forwarded-For", "X-Real-IP"} {
		value := strings.TrimSpace(c.GetHeader(header))
		if value == "" {
			continue
		}
		if header == "X-Forwarded-For" {
			parts := strings.Split(value, ",")
			value = strings.TrimSpace(parts[0])
		}
		if ip := net.ParseIP(value); ip != nil {
			return ip.String()
		}
	}
	return c.ClientIP()
}

// ── Visitor fingerprint ───────────────────────────────────────────────────────

func visitorKey(ip, ua string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(ip) + "|" + strings.TrimSpace(ua)))
	return hex.EncodeToString(sum[:])
}

// ── User identification from cookie ───────────────────────────────────────────

func identifyUser(cookieVal string) (*uint, string) {
	if cookieVal == "" {
		return nil, ""
	}
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "furci-default-secret-change-in-prod"
	}
	token, err := jwt.Parse(cookieVal, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, ""
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["type"] != "access" {
		return nil, ""
	}
	sub, ok := claims["sub"].(float64)
	if !ok {
		return nil, ""
	}
	id := uint(sub)
	email, _ := claims["email"].(string)
	return &id, email
}

// ── Handler ───────────────────────────────────────────────────────────────────

type trackRequest struct {
	Path      string `json:"path"`
	Referrer  string `json:"referrer"`
	UserAgent string `json:"user_agent"`
}

// POST /v1/track — public, fire-and-forget page view recording
func TrackPageViewHandler(c *gin.Context) {
	var req trackRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Path == "" {
		c.Status(http.StatusAccepted)
		return
	}
	if database.DB == nil {
		c.Status(http.StatusAccepted)
		return
	}

	// Extract everything from the live request BEFORE spawning the goroutine —
	// the gin.Context is invalid once the handler returns.
	ip := clientIP(c)
	ua := req.UserAgent
	if ua == "" {
		ua = c.GetHeader("User-Agent")
	}
	key := visitorKey(ip, ua)
	cookieVal, _ := c.Cookie("furci_access")
	path := req.Path
	referrer := req.Referrer

	c.Status(http.StatusAccepted)

	go func() {
		geo := lookupGeo(ip)
		userID, email := identifyUser(cookieVal)

		visit := models.PageVisit{
			VisitorKey: key,
			UserID:     userID,
			UserEmail:  email,
			Path:       path,
			Referrer:   referrer,
			Country:    geo.Country,
			Region:     geo.Region,
			City:       geo.City,
			UserAgent:  ua,
		}
		database.DB.Create(&visit)
	}()
}

func detectDevice(ua string) string {
	ua = strings.ToLower(ua)
	if strings.Contains(ua, "mobile") || strings.Contains(ua, "android") ||
		strings.Contains(ua, "iphone") || strings.Contains(ua, "ipod") {
		return "mobile"
	}
	if strings.Contains(ua, "tablet") || strings.Contains(ua, "ipad") {
		return "tablet"
	}
	return "desktop"
}
