package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// RequireAuth validates the JWT and confirms the user still exists in the DB.
// It reads the token from the httpOnly cookie "furci_access" first,
// then falls back to the Authorization: Bearer header (for admin panel / API clients).
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenStr string

		// Primary: httpOnly cookie (web app)
		if cookie, err := c.Cookie("furci_access"); err == nil && cookie != "" {
			tokenStr = cookie
		}

		// Fallback: Authorization header (admin panel, API clients, legacy)
		if tokenStr == "" {
			auth := c.GetHeader("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				tokenStr = strings.TrimPrefix(auth, "Bearer ")
			}
		}

		if tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{"code": "AUTH_REQUIRED", "message": "Authentication required."},
			})
			return
		}
		secret := os.Getenv("JWT_SECRET")

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		})

		// LOGIC UPDATE: Even if the token is expired, we want to know WHO it was
		// so we can check if they were deleted by an admin.
		if err != nil {
			// Try to parse without validation to get the UserID claims
			unverifiedToken, _, parseErr := new(jwt.Parser).ParseUnverified(tokenStr, jwt.MapClaims{})
			if parseErr == nil {
				if claims, ok := unverifiedToken.Claims.(jwt.MapClaims); ok {
					if sub, ok := claims["sub"].(float64); ok {
						var user models.User
						// If the user is missing (hard deleted) or soft-deleted, send the 'deleted' flag
						if database.DB.Unscoped().First(&user, uint(sub)).Error != nil || user.DeletedAt.Valid {
							c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
								"error":   gin.H{"code": "AUTH_REQUIRED", "message": "Account no longer exists."},
								"deleted": true,
							})
							return
						}
					}
				}
			}
		}

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{"code": "SESSION_EXPIRED", "message": "Invalid or expired session. Please log in again."},
			})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{"code": "SESSION_EXPIRED", "message": "Invalid token claims."},
			})
			return
		}

		// Expect "sub" = user ID (float64 in JWT), "email" string, "type" = "access"
		if claims["type"] != "access" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{"code": "SESSION_EXPIRED", "message": "Invalid token type."},
			})
			return
		}

		userID := uint(claims["sub"].(float64))
		email, _ := claims["email"].(string)

		// Confirm the user still exists — if deleted via admin, reject immediately
		var user models.User
		if database.DB.First(&user, userID).Error != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   gin.H{"code": "AUTH_REQUIRED", "message": "Account no longer exists."},
				"deleted": true,
			})
			return
		}

		c.Set("userID", userID)
		c.Set("userEmail", email)
		c.Next()
	}
}
