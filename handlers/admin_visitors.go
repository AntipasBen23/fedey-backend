package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/AntipasBen23/fedey-backend/utils"
	"github.com/gin-gonic/gin"
)

// VisitorSession is the aggregated row returned by the sessions query.
type VisitorSession struct {
	VisitorKey string     `json:"visitorKey"`
	UserID     *uint      `json:"userId"`
	UserEmail  string     `json:"userEmail"`
	Country    string     `json:"country"`
	Region     string     `json:"region"`
	LatestPath string     `json:"latestPath"`
	FirstSeen  time.Time  `json:"firstSeen"`
	LastSeen   time.Time  `json:"lastSeen"`
	PageViews  int64      `json:"pageViews"`
}

// VisitorDetail is the full profile for one visitor.
type VisitorDetail struct {
	VisitorKey string             `json:"visitorKey"`
	UserID     *uint              `json:"userId"`
	UserEmail  string             `json:"userEmail"`
	Country    string             `json:"country"`
	Region     string             `json:"region"`
	City       string             `json:"city"`
	FirstSeen  time.Time          `json:"firstSeen"`
	LastSeen   time.Time          `json:"lastSeen"`
	PageViews  int64              `json:"pageViews"`
	Trail      []models.PageVisit `json:"trail"`
}

// GET /v1/admin/visitors?search=&country=&limit=50&offset=0
func AdminVisitorSessionsHandler(c *gin.Context) {
	search := c.Query("search")
	country := c.Query("country")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	searchParam := "%%"
	if search != "" {
		searchParam = "%" + search + "%"
	}

	type row struct {
		VisitorKey string     `gorm:"column:visitor_key"`
		UserID     *uint      `gorm:"column:user_id"`
		UserEmail  string     `gorm:"column:user_email"`
		Country    string     `gorm:"column:country"`
		Region     string     `gorm:"column:region"`
		LatestPath string     `gorm:"column:latest_path"`
		FirstSeen  time.Time  `gorm:"column:first_seen"`
		LastSeen   time.Time  `gorm:"column:last_seen"`
		PageViews  int64      `gorm:"column:page_views"`
	}

	var rows []row
	database.DB.Raw(`
		WITH filtered AS (
			SELECT *
			FROM page_visits
			WHERE (? = '%%' OR user_email ILIKE ? OR path ILIKE ? OR country ILIKE ?)
			  AND (? = '' OR country = ?)
		),
		ranked AS (
			SELECT *,
				ROW_NUMBER() OVER (PARTITION BY visitor_key ORDER BY created_at DESC) AS rn
			FROM filtered
		),
		agg AS (
			SELECT
				visitor_key,
				MIN(created_at)             AS first_seen,
				MAX(created_at)             AS last_seen,
				COUNT(*)                    AS page_views,
				MAX(NULLIF(user_email, '')) AS best_email,
				MAX(user_id)                AS best_user_id
			FROM filtered
			GROUP BY visitor_key
		)
		SELECT
			r.visitor_key,
			agg.best_user_id                                   AS user_id,
			COALESCE(agg.best_email, '')                       AS user_email,
			COALESCE(NULLIF(r.country, ''), 'Unknown')         AS country,
			COALESCE(NULLIF(r.region, ''), '')                 AS region,
			COALESCE(NULLIF(r.path, ''), '/')                  AS latest_path,
			agg.first_seen,
			agg.last_seen,
			agg.page_views
		FROM ranked r
		JOIN agg ON agg.visitor_key = r.visitor_key
		WHERE r.rn = 1
		ORDER BY agg.last_seen DESC
		LIMIT ? OFFSET ?
	`, searchParam, searchParam, searchParam, searchParam, country, country, limit, offset).Scan(&rows)

	sessions := make([]VisitorSession, len(rows))
	for i, r := range rows {
		sessions[i] = VisitorSession{
			VisitorKey: r.VisitorKey,
			UserID:     r.UserID,
			UserEmail:  r.UserEmail,
			Country:    r.Country,
			Region:     r.Region,
			LatestPath: r.LatestPath,
			FirstSeen:  r.FirstSeen,
			LastSeen:   r.LastSeen,
			PageViews:  r.PageViews,
		}
	}

	c.JSON(http.StatusOK, gin.H{"sessions": sessions, "total": len(sessions)})
}

// GET /v1/admin/visitors/:key
func AdminVisitorDetailHandler(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		utils.APIError(c, http.StatusBadRequest, "MISSING_FIELDS", "Visitor key required.")
		return
	}

	type aggRow struct {
		UserID    *uint     `gorm:"column:user_id"`
		UserEmail string    `gorm:"column:user_email"`
		Country   string    `gorm:"column:country"`
		Region    string    `gorm:"column:region"`
		City      string    `gorm:"column:city"`
		FirstSeen time.Time `gorm:"column:first_seen"`
		LastSeen  time.Time `gorm:"column:last_seen"`
		PageViews int64     `gorm:"column:page_views"`
	}

	var agg aggRow
	database.DB.Raw(`
		SELECT
			MAX(user_id)                AS user_id,
			COALESCE(MAX(NULLIF(user_email, '')), '') AS user_email,
			COALESCE(NULLIF(MAX(country), ''), 'Unknown') AS country,
			COALESCE(NULLIF(MAX(region), ''), '')  AS region,
			COALESCE(NULLIF(MAX(city), ''), '')    AS city,
			MIN(created_at)             AS first_seen,
			MAX(created_at)             AS last_seen,
			COUNT(*)                    AS page_views
		FROM page_visits
		WHERE visitor_key = ?
	`, key).Scan(&agg)

	var trail []models.PageVisit
	database.DB.Where("visitor_key = ?", key).
		Order("created_at DESC").
		Limit(200).
		Find(&trail)

	detail := VisitorDetail{
		VisitorKey: key,
		UserID:     agg.UserID,
		UserEmail:  agg.UserEmail,
		Country:    agg.Country,
		Region:     agg.Region,
		City:       agg.City,
		FirstSeen:  agg.FirstSeen,
		LastSeen:   agg.LastSeen,
		PageViews:  agg.PageViews,
		Trail:      trail,
	}

	c.JSON(http.StatusOK, gin.H{"visitor": detail})
}
