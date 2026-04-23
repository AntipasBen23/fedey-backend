package middleware

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// IPRateLimiter holds a map of several rate limiters, one per IP.
type IPRateLimiter struct {
	ips map[string]*rate.Limiter
	mu  *sync.RWMutex
	r   rate.Limit
	b   int
}

func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	return &IPRateLimiter{
		ips: make(map[string]*rate.Limiter),
		mu:  &sync.RWMutex{},
		r:   r,
		b:   b,
	}
}

func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	limiter, exists := i.ips[ip]
	if !exists {
		limiter = rate.NewLimiter(i.r, i.b)
		i.ips[ip] = limiter
	}

	return limiter
}

// GlobalLimiter limits total requests per IP
var globalLimiter = NewIPRateLimiter(5, 20) // 5 req/sec, burst of 20

func RateLimiter() gin.HandlerFunc {
	return func(c *gin.Context) {
		limiter := globalLimiter.GetLimiter(c.ClientIP())
		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many requests. Please slow down.",
			})
			return
		}
		c.Next()
	}
}

// AIActionLimiter is more strict for expensive AI endpoints
var aiLimiter = NewIPRateLimiter(0.2, 5) // 1 req every 5 seconds, burst of 5

func ExpensiveActionLimiter() gin.HandlerFunc {
	return func(c *gin.Context) {
		limiter := aiLimiter.GetLimiter(c.ClientIP())
		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "AI generation limit reached. Please wait a few seconds before trying again.",
			})
			return
		}
		c.Next()
	}
}
